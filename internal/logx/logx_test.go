package logx

import (
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
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
		if got := parseLevel(in); got != want {
			t.Errorf("parseLevel(%q) = %v, want %v", in, got, want)
		}
	}
}

// TestInitMirrorsToLogDir verifies that LOG_DIR makes Init mirror every log
// line into a rotating file in that directory (stderr keeps receiving it too).
func TestInitMirrorsToLogDir(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("LOG_DIR", dir)
	t.Setenv("LOG_LEVEL", "debug")
	Init()
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
