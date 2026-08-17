package logx

import (
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/natefinch/lumberjack.v2"
)

func TestParseLevel(t *testing.T) {
	cases := map[string]slog.Level{
		"debug":   slog.LevelDebug,
		"DEBUG":   slog.LevelDebug,
		"info":    slog.LevelInfo,
		"":        slog.LevelInfo,
		"garbage": slog.LevelInfo,
		"warning": slog.LevelWarn,
		"warn":    slog.LevelWarn,
		"error":   slog.LevelError,
	}
	for in, want := range cases {
		if got := ParseLevel(in); got != want {
			t.Errorf("ParseLevel(%q) = %v, want %v", in, got, want)
		}
	}
}

// TestSetLevelChangesRuntimeLevel verifies SetLevel takes effect immediately
// (the settings page swaps log_level without a restart).
func TestSetLevelChangesRuntimeLevel(t *testing.T) {
	Init(slog.LevelError)
	t.Cleanup(Close)

	ch, cancel := Subscribe(8)
	defer cancel()
	// Below the current level: nothing reaches subscribers.
	slog.Debug("hidden")
	slog.Info("also hidden")

	SetLevel(slog.LevelDebug)
	slog.Debug("visible now")
	rec := <-ch
	if rec.Message != "visible now" {
		t.Fatalf("expected debug record after SetLevel, got %+v", rec)
	}
}

// TestInitMirrorsToLogDir verifies that LOG_DIR makes Init mirror every log
// line into a rotating file in that directory (stderr keeps receiving it too).
func TestInitMirrorsToLogDir(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("LOG_DIR", dir)
	Init(slog.LevelDebug)
	t.Cleanup(Close)

	slog.Info("hello log file", "k", "v")

	raw, err := os.ReadFile(filepath.Join(dir, "gourl.log"))
	if err != nil {
		t.Fatalf("read log file: %v", err)
	}
	if !strings.Contains(string(raw), "hello log file") {
		t.Fatalf("log file content = %q, want the Info line", raw)
	}
}

// setFanout installs a fanout handler wrapping a null text handler so the
// subscribe tests do not stack nested fanoutHandlers via Init.
func setFanout() {
	slog.SetDefault(slog.New(&fanoutHandler{next: slog.NewTextHandler(os.Stderr, nil)}))
}

func TestParseJSONLine(t *testing.T) {
	rec, ok := parseJSONLine(`{"time":"2026-08-16T10:00:00+08:00","level":"INFO","msg":"link created","code":"abc"}`)
	if !ok {
		t.Fatal("expected JSON line to parse")
	}
	if rec.Level != "info" || rec.Message != "link created" {
		t.Fatalf("unexpected record: %+v", rec)
	}
	if rec.Attrs["code"] != "abc" {
		t.Fatalf("expected code attr, got %+v", rec.Attrs)
	}
	if rec.Time.IsZero() {
		t.Fatal("expected parsed time")
	}
}

func TestParseTextLine(t *testing.T) {
	rec := parseTextLine(`time=2026-08-16T10:00:00.000+08:00 level=WARN msg="ua blocked" code=abc user_agent=bot`)
	if rec.Level != "warn" || rec.Message != "ua blocked" {
		t.Fatalf("unexpected record: %+v", rec)
	}
	if rec.Attrs["code"] != "abc" || rec.Attrs["user_agent"] != "bot" {
		t.Fatalf("expected attrs, got %+v", rec.Attrs)
	}
}

func TestParseTextLineUnparseable(t *testing.T) {
	rec := parseTextLine("garbage line")
	if rec.Message != "garbage line" || rec.Level != "" {
		t.Fatalf("expected bare message, got %+v", rec)
	}
}

func TestParseTextLineAttrsWithSpaces(t *testing.T) {
	rec := parseTextLine(`time=2026-08-16T10:00:00Z level=INFO msg="some message" error="two words"`)
	if rec.Message != "some message" {
		t.Fatalf("expected message, got %q", rec.Message)
	}
	if rec.Attrs["error"] != "two words" {
		t.Fatalf("expected quoted attr, got %+v", rec.Attrs)
	}
}

func TestSubscribeReceivesRecords(t *testing.T) {
	setFanout()
	ch, cancel := Subscribe(16)
	defer cancel()

	slog.Info("hello", "code", "abc")
	rec := <-ch
	if rec.Message != "hello" || rec.Level != "info" {
		t.Fatalf("unexpected record: %+v", rec)
	}
	if rec.Attrs["code"] != "abc" {
		t.Fatalf("expected attr, got %+v", rec.Attrs)
	}
}

func TestSubscribeCancelClosesChannel(t *testing.T) {
	setFanout()
	ch, cancel := Subscribe(4)
	cancel()
	if _, ok := <-ch; ok {
		t.Fatal("expected channel to be closed after cancel")
	}
}

func TestReadHistoryNewestFirst(t *testing.T) {
	path := filepath.Join(t.TempDir(), "gourl.log")
	lines := []string{
		`time=2026-08-16T10:00:00Z level=INFO msg=first`,
		`time=2026-08-16T10:00:01Z level=INFO msg=second`,
		`time=2026-08-16T10:00:02Z level=INFO msg=third`,
		``,
	}
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")), 0o644); err != nil {
		t.Fatal(err)
	}
	fileSink = &lumberjack.Logger{Filename: path}
	defer func() { fileSink = nil }()

	recs, err := ReadHistory(0, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(recs) != 3 {
		t.Fatalf("expected 3 records, got %d", len(recs))
	}
	if recs[0].Message != "third" || recs[2].Message != "first" {
		t.Fatalf("expected newest-first, got %+v", recs)
	}

	recs, err = ReadHistory(2, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(recs) != 2 {
		t.Fatalf("expected 2 records with limit+offset, got %d", len(recs))
	}
	if recs[0].Message != "second" || recs[1].Message != "first" {
		t.Fatalf("expected offset skip, got %+v", recs)
	}
}

func TestReadHistoryWithoutLogDir(t *testing.T) {
	fileSink = nil
	recs, err := ReadHistory(0, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(recs) != 0 {
		t.Fatalf("expected empty history, got %d", len(recs))
	}
}
