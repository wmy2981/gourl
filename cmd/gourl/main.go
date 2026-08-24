package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/wmy2981/gourl/internal/api"
	"github.com/wmy2981/gourl/internal/cli"
	"github.com/wmy2981/gourl/internal/config"
	"github.com/wmy2981/gourl/internal/counter"
	"github.com/wmy2981/gourl/internal/logx"
	"github.com/wmy2981/gourl/internal/store"
	"github.com/wmy2981/gourl/internal/version"
)

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func main() {
	// Any first argument is an administrative subcommand (gourl reset db,
	// gourl status, …); no arguments start the HTTP server.
	if len(os.Args) > 1 {
		os.Exit(cli.Main(os.Args[1:]))
	}
	runServer()
}

func runServer() {
	// Default level until the config loads; the configured log_level is
	// applied right after (and hot-applied on settings saves).
	logx.Init(slog.LevelInfo)

	// Log the build identity first: dev images embed "VERSION (sha7)", so
	// even a startup failure (bad config, unwritable data dir, …) can be
	// traced to the exact build.
	slog.Info("gourl version", "version", version.Version)

	// Defaults point at ./config and ./data (the container mounts these as
	// /app/config and /app/data); both directories are created here so the
	// config write-back and the database always have a home.
	cfgPath := envOr("CONFIG_PATH", "./config/config.yaml")
	if dir := filepath.Dir(cfgPath); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			slog.Error("create config dir failed", "dir", dir, "error", err)
			os.Exit(1)
		}
	}
	cfg, err := config.NewManager(cfgPath)
	if err != nil {
		slog.Error("config load failed", "path", cfgPath, "error", err)
		os.Exit(1)
	}
	logx.SetLevel(logx.ParseLevel(cfg.Get().LogLevel))
	slog.Info("log level applied", "level", cfg.Get().LogLevel)

	// SIGHUP (sent by CLI config commands via `gourl reload` semantics)
	// re-reads the config file so edits made in another process apply
	// without a restart. A broken file keeps the previous config.
	reloadCfg := func() {
		if err := cfg.Reload(); err != nil {
			slog.Warn("config reload failed; keeping the previous config", "path", cfgPath, "error", err)
			return
		}
		level := cfg.Get().LogLevel
		logx.SetLevel(logx.ParseLevel(level))
		slog.Info("config reloaded", "path", cfgPath, "level", level)
	}
	sigHup := make(chan os.Signal, 1)
	signal.Notify(sigHup, syscall.SIGHUP)
	defer signal.Stop(sigHup)
	go func() {
		for range sigHup {
			reloadCfg()
		}
	}()

	dbPath := envOr("DB_PATH", "./data/gourl.db")
	if dir := filepath.Dir(dbPath); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			slog.Error("create data dir failed", "dir", dir, "error", err)
			os.Exit(1)
		}
	}
	st, err := store.Open(dbPath)
	if err != nil {
		slog.Error("store open failed", "path", dbPath, "error", err)
		os.Exit(1)
	}
	defer st.Close()

	redisAddr := envOr("REDIS_ADDR", "localhost:6379")
	ctr := counter.New(redisAddr)
	if err := ctr.Ping(context.Background()); err != nil {
		slog.Warn("redis unavailable; clicks will not be counted", "addr", redisAddr, "error", err)
	}

	// Flush buffered click counts to SQLite every 30s.
	flushCtx, flushCancel := context.WithCancel(context.Background())
	defer flushCancel()
	go counter.NewFlusher(st, ctr, 30*time.Second).Run(flushCtx)

	addr := ":" + envOr("PORT", "8080")
	slog.Info("gourl started",
		"version", version.Version,
		"addr", addr,
		"config", cfgPath,
		"db", dbPath,
		"redis", redisAddr,
		"auth_enabled", cfg.Get().PasswordHash != "",
	)
	if err := http.ListenAndServe(addr, api.NewServer(st, cfg, ctr).Handler()); err != nil {
		slog.Error("http server failed", "error", err)
		os.Exit(1)
	}
}
