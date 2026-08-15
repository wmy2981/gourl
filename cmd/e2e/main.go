// Command e2e runs the gourl server for Playwright end-to-end tests: the
// SQLite database is in-memory and Redis is replaced by miniredis, so no
// external services are needed. It is never built into the production image.
package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"

	"github.com/wmy2981/gourl/internal/api"
	"github.com/wmy2981/gourl/internal/config"
	"github.com/wmy2981/gourl/internal/counter"
	"github.com/wmy2981/gourl/internal/logx"
	"github.com/wmy2981/gourl/internal/store"
)

const (
	adminPassword = "e2e-password"
	sessionSecret = "e2e-secret"
	flushInterval = 500 * time.Millisecond
)

func main() {
	logx.Init()
	_ = os.Setenv("LOG_LEVEL", "warning") // keep e2e output readable

	port := os.Getenv("E2E_PORT")
	if port == "" {
		port = "8099"
	}

	// The config must live at a writable path: the settings page PUTs config
	// changes back to it, and an empty path breaks the atomic write-back.
	cfgPath := os.Getenv("E2E_CONFIG")
	if cfgPath == "" {
		tmp, err := os.CreateTemp("", "gourl-e2e-config-*.yaml")
		if err != nil {
			slog.Error("config temp file failed", "error", err)
			os.Exit(1)
		}
		cfgPath = tmp.Name()
		tmp.Close()
		defer os.Remove(cfgPath)
	}
	cfg, err := config.NewManager(cfgPath)
	if err != nil {
		slog.Error("config load failed", "path", cfgPath, "error", err)
		os.Exit(1)
	}
	st, err := store.Open(":memory:")
	if err != nil {
		slog.Error("store open failed", "error", err)
		os.Exit(1)
	}
	defer st.Close()

	mr := miniredis.NewMiniRedis()
	if err := mr.Start(); err != nil {
		slog.Error("miniredis start failed", "error", err)
		os.Exit(1)
	}
	defer mr.Close()
	ctr := counter.NewFromClient(redis.NewClient(&redis.Options{Addr: mr.Addr()}))

	// NewServer reads these from the environment.
	_ = os.Setenv("ADMIN_PASSWORD", adminPassword)
	_ = os.Setenv("SESSION_SECRET", sessionSecret)
	assetsDir, err := os.MkdirTemp("", "gourl-e2e-assets-")
	if err != nil {
		slog.Error("assets dir failed", "error", err)
		os.Exit(1)
	}
	_ = os.Setenv("ASSETS_DIR", assetsDir)
	defer os.RemoveAll(assetsDir)

	// Fast flush so click-count assertions do not wait 30s.
	flushCtx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go counter.NewFlusher(st, ctr, flushInterval).Run(flushCtx)

	addr := "127.0.0.1:" + port
	slog.Info("e2e server listening", "addr", "http://"+addr, "admin_password", adminPassword)
	if err := http.ListenAndServe(addr, api.NewServer(st, cfg, ctr).Handler()); err != nil {
		slog.Error("http server failed", "error", err)
		os.Exit(1)
	}
}
