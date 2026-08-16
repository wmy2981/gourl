package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/wmy2981/gourl/internal/api"
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
	logx.Init()

	// Log the build identity first: dev images embed "VERSION (sha7)", so
	// even a startup failure (bad config, unwritable data dir, …) can be
	// traced to the exact build.
	slog.Info("gourl version", "version", version.Version)

	cfgPath := envOr("CONFIG_PATH", "config.yaml")
	cfg, err := config.NewManager(cfgPath)
	if err != nil {
		slog.Error("config load failed", "path", cfgPath, "error", err)
		os.Exit(1)
	}

	dbPath := envOr("DB_PATH", "data/gourl.db")
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
		"auth_enabled", os.Getenv("ADMIN_PASSWORD") != "",
	)
	if err := http.ListenAndServe(addr, api.NewServer(st, cfg, ctr).Handler()); err != nil {
		slog.Error("http server failed", "error", err)
		os.Exit(1)
	}
}
