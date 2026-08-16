// Package logx configures the process-wide structured logger (log/slog)
// with four levels: debug, info, warning, error.
//
// Configuration (environment variables):
//   - LOG_LEVEL: debug | info | warning | error (default: info)
//   - LOG_FORMAT: json for structured JSON output, anything else = text
//   - LOG_DIR: optional directory for a rotating file mirror of the logs
//     (e.g. a directory on the mounted volume so logs persist across
//     container restarts). Files are size-rotated by lumberjack.
//
// Logs go to stderr so stdout stays clean for anything piping it.
//
// Every record is also fanned out to subscribers (the admin log page's SSE
// stream), and the mirrored file — when LOG_DIR is configured — backs the
// history endpoint.
package logx

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"gopkg.in/natefinch/lumberjack.v2"
)

// Record is the structured form of a log line as delivered to subscribers
// (live stream) and read back from the mirrored file (history). Level is
// lowercased (debug/info/warn/error) for both paths.
type Record struct {
	Time    time.Time      `json:"time"`
	Level   string         `json:"level"`
	Message string         `json:"message"`
	Attrs   map[string]any `json:"attrs,omitempty"`
}

// fileSink is the optional rotating file writer; kept so Close can release
// the handle (tests and graceful shutdowns).
var fileSink *lumberjack.Logger

// HistoryEnabled reports whether the mirrored log file backs the history
// endpoint (i.e. LOG_DIR is configured).
func HistoryEnabled() bool { return fileSink != nil }

// Close releases the log file handle if LOG_DIR was configured. Safe to call
// any number of times; the OS reclaims it on exit otherwise.
func Close() {
	if fileSink != nil {
		fileSink.Close()
		fileSink = nil
	}
}

// Init configures slog as the default logger. Call once at startup.
func Init() {
	level := parseLevel(os.Getenv("LOG_LEVEL"))
	opts := &slog.HandlerOptions{Level: level}

	// Mirror to a rotating file on the mounted volume when LOG_DIR is set;
	// stderr stays in the chain so docker logs still sees everything.
	var w io.Writer = os.Stderr
	if dir := os.Getenv("LOG_DIR"); dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			slog.Warn("log dir unavailable, stderr only", "dir", dir, "err", err)
		} else {
			fileSink = &lumberjack.Logger{
				Filename:   filepath.Join(dir, "gourl.log"),
				MaxSize:    10, // MB per file
				MaxBackups: 5,
				MaxAge:     30, // days
				Compress:   true,
			}
			w = io.MultiWriter(os.Stderr, fileSink)
		}
	}

	var handler slog.Handler = slog.NewTextHandler(w, opts)
	if strings.EqualFold(os.Getenv("LOG_FORMAT"), "json") {
		handler = slog.NewJSONHandler(w, opts)
	}
	// Fan every record out to live subscribers before the configured sink.
	slog.SetDefault(slog.New(&fanoutHandler{next: handler}))

	slog.Debug("logging initialized", "level", level.String())
}

func parseLevel(s string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "debug":
		return slog.LevelDebug
	case "warning", "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

/* ---------- live fan-out ---------- */

// fanoutHandler forwards records to the configured handler and, in parallel,
// delivers a structured copy to every live subscriber.
type fanoutHandler struct {
	next slog.Handler
}

func (h *fanoutHandler) Enabled(ctx context.Context, l slog.Level) bool {
	return h.next.Enabled(ctx, l)
}

func (h *fanoutHandler) Handle(ctx context.Context, r slog.Record) error {
	rec := Record{Time: r.Time, Level: strings.ToLower(r.Level.String()), Message: r.Message}
	var attrs map[string]any
	r.Attrs(func(a slog.Attr) bool {
		if a.Key == "" {
			return true
		}
		if attrs == nil {
			attrs = make(map[string]any)
		}
		collectAttr(attrs, "", a)
		return true
	})
	rec.Attrs = attrs
	broadcast(rec)
	return h.next.Handle(ctx, r)
}

func (h *fanoutHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &fanoutHandler{next: h.next.WithAttrs(attrs)}
}

func (h *fanoutHandler) WithGroup(name string) slog.Handler {
	return &fanoutHandler{next: h.next.WithGroup(name)}
}

// collectAttr flattens an attr (and any nested groups) into dst.
func collectAttr(dst map[string]any, prefix string, a slog.Attr) {
	key := a.Key
	if prefix != "" {
		key = prefix + "." + a.Key
	}
	if a.Value.Kind() == slog.KindGroup {
		for _, g := range a.Value.Group() {
			collectAttr(dst, key, g)
		}
		return
	}
	dst[key] = a.Value.Any()
}

var (
	fanMu sync.RWMutex
	fans  = make(map[chan Record]struct{})
)

// Subscribe returns a buffered channel that receives every subsequent log
// record, plus a cancel func that stops delivery and closes the channel.
// A subscriber that fails to drain fast enough is dropped rather than
// blocking the logging hot path.
func Subscribe(buffer int) (<-chan Record, func()) {
	ch := make(chan Record, buffer)
	fanMu.Lock()
	fans[ch] = struct{}{}
	fanMu.Unlock()
	return ch, func() {
		fanMu.Lock()
		if _, ok := fans[ch]; ok {
			delete(fans, ch)
			close(ch)
		}
		fanMu.Unlock()
	}
}

func broadcast(r Record) {
	fanMu.RLock()
	defer fanMu.RUnlock()
	for ch := range fans {
		select {
		case ch <- r:
		default: // slow subscriber: drop rather than block logging
		}
	}
}

/* ---------- history from the mirrored file ---------- */

// HistoryLimit caps a single history page.
const HistoryLimit = 1000

// ReadHistory returns up to limit records from the mirrored log file, newest
// first, skipping the most recent offset records. It is empty when LOG_DIR
// is not configured or the file does not exist yet. Lines are parsed
// leniently — both the text and JSON handler output are understood, and
// unparseable lines surface as bare messages with an empty level.
func ReadHistory(limit, offset int) ([]Record, error) {
	if fileSink == nil {
		return []Record{}, nil
	}
	data, err := os.ReadFile(fileSink.Filename)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return []Record{}, nil
		}
		return nil, err
	}
	lines := strings.Split(string(data), "\n")
	if n := len(lines); n > 0 && lines[n-1] == "" {
		lines = lines[:n-1]
	}
	if limit <= 0 {
		limit = 200
	}
	if limit > HistoryLimit {
		limit = HistoryLimit
	}
	if offset < 0 {
		offset = 0
	}
	out := make([]Record, 0, limit)
	for i := len(lines) - 1 - offset; i >= 0 && len(out) < limit; i-- {
		if lines[i] == "" {
			continue
		}
		out = append(out, parseLine(lines[i]))
	}
	return out, nil
}

// parseLine turns one file line into a Record. JSON lines (LOG_FORMAT=json)
// parse structurally; text lines yield time/level/msg plus loose attr pairs;
// anything else becomes a bare message.
func parseLine(line string) Record {
	if strings.HasPrefix(line, "{") {
		if rec, ok := parseJSONLine(line); ok {
			return rec
		}
	}
	return parseTextLine(line)
}

func parseJSONLine(line string) (Record, bool) {
	var m map[string]any
	if err := json.Unmarshal([]byte(line), &m); err != nil {
		return Record{}, false
	}
	rec := Record{
		Level:   strings.ToLower(fmt.Sprint(m["level"])),
		Message: fmt.Sprint(m["msg"]),
	}
	if t, ok := m["time"].(string); ok {
		rec.Time = parseTime(t)
	}
	delete(m, "time")
	delete(m, "level")
	delete(m, "msg")
	if len(m) > 0 {
		rec.Attrs = m
	}
	return rec, true
}

// textLineRe matches the slog text handler prefix: time=... level=... msg=...
var textLineRe = regexp.MustCompile(`^time=(\S+)\s+level=(\S+)\s+msg=(.*)$`)

// attrPairRe finds key=value pairs in an attrs tail (quoted values keep
// their spaces).
var attrPairRe = regexp.MustCompile(`(\w+)=("(?:[^"\\]|\\.)*"|\S+)`)

func parseTextLine(line string) Record {
	m := textLineRe.FindStringSubmatch(line)
	if m == nil {
		return Record{Message: line}
	}
	msg, tail := splitMsgAttrs(m[3])
	rec := Record{
		Time:    parseTime(m[1]),
		Level:   strings.ToLower(m[2]),
		Message: unquote(msg),
	}
	if attrs := parseAttrs(tail); len(attrs) > 0 {
		rec.Attrs = attrs
	}
	return rec
}

// unquote unwraps a quoted slog text value (escapes included).
func unquote(s string) string {
	if strings.HasPrefix(s, `"`) && strings.HasSuffix(s, `"`) {
		if u, err := strconv.Unquote(s); err == nil {
			return u
		}
	}
	return s
}

// splitMsgAttrs separates the message from the trailing key=value attrs of a
// text-format log line. A bare `key=` token (no space in the key) marks the
// attrs start; quoted values may contain spaces.
func splitMsgAttrs(s string) (msg, tail string) {
	for i := 1; i < len(s); i++ {
		if s[i] != ' ' || i+1 >= len(s) {
			continue
		}
		j := i + 1
		for j < len(s) && s[j] != ' ' && s[j] != '=' {
			j++
		}
		if j < len(s) && s[j] == '=' {
			return s[:i], s[i+1:]
		}
	}
	return s, ""
}

func parseAttrs(s string) map[string]any {
	if s == "" {
		return nil
	}
	matches := attrPairRe.FindAllStringSubmatch(s, -1)
	if len(matches) == 0 {
		return nil
	}
	out := make(map[string]any, len(matches))
	for _, m := range matches {
		out[m[1]] = unquote(m[2])
	}
	return out
}

func parseTime(s string) time.Time {
	t, err := time.Parse(time.RFC3339Nano, s)
	if err == nil {
		return t
	}
	return time.Time{}
}
