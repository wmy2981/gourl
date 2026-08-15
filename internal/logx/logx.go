// Package logx configures the process-wide structured logger (log/slog)
// with four levels: debug, info, warning, error.
//
// Configuration (environment variables):
//   - LOG_LEVEL: debug | info | warning | error (default: info)
//   - LOG_FORMAT: json for structured JSON output, anything else = text
//
// Logs go to stderr so stdout stays clean for anything piping it.
package logx

import (
	"log/slog"
	"os"
	"strings"
)

// Init configures slog as the default logger. Call once at startup.
func Init() {
	level := parseLevel(os.Getenv("LOG_LEVEL"))
	opts := &slog.HandlerOptions{Level: level}

	var handler slog.Handler = slog.NewTextHandler(os.Stderr, opts)
	if strings.EqualFold(os.Getenv("LOG_FORMAT"), "json") {
		handler = slog.NewJSONHandler(os.Stderr, opts)
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
