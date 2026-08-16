// Package logx configures the process-wide structured logger (log/slog)
// with four levels: debug, info, warning, error.
//
// Configuration (environment variables):
//   - LOG_LEVEL: debug | info | warning | error (default: info)
//   - LOG_FORMAT: json for structured JSON output, anything else = text
//   - LOG_DIR: optional directory for a rotating file mirror of the logs
//     (e.g. a directory on the mounted data volume so logs persist across
//     container restarts). Files are size-rotated by lumberjack.
//
// Logs go to stderr so stdout stays clean for anything piping it.
package logx

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/natefinch/lumberjack.v2"
)

// fileSink is the optional rotating file writer; kept so Close can release
// the handle (tests and graceful shutdowns).
var fileSink *lumberjack.Logger

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
	slog.SetDefault(slog.New(handler))

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
