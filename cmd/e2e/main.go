// Command e2e runs the gourl server for Playwright end-to-end tests: the
// SQLite database is in-memory and Redis is replaced by miniredis, so no
// external services are needed. It is never built into the production image.
package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"

	"github.com/wmy2981/gourl/internal/api"
	"github.com/wmy2981/gourl/internal/config"
	"github.com/wmy2981/gourl/internal/counter"
	"github.com/wmy2981/gourl/internal/store"
)

const (
	adminPassword = "e2e-password"
	sessionSecret = "e2e-secret"
	flushInterval = 500 * time.Millisecond
)

func main() {
	port := os.Getenv("E2E_PORT")
	if port == "" {
		port = "8099"
	}

	cfg, err := config.NewManager(os.Getenv("E2E_CONFIG"))
	if err != nil {
		log.Fatalf("config: %v", err)
	}
	st, err := store.Open(":memory:")
	if err != nil {
		log.Fatalf("store: %v", err)
	}
	defer st.Close()

	mr := miniredis.NewMiniRedis()
	if err := mr.Start(); err != nil {
		log.Fatalf("miniredis: %v", err)
	}
	defer mr.Close()
	ctr := counter.NewFromClient(redis.NewClient(&redis.Options{Addr: mr.Addr()}))

	// NewServer reads these from the environment.
	_ = os.Setenv("ADMIN_PASSWORD", adminPassword)
	_ = os.Setenv("SESSION_SECRET", sessionSecret)
	assetsDir, err := os.MkdirTemp("", "gourl-e2e-assets-")
	if err != nil {
		log.Fatal(err)
	}
	_ = os.Setenv("ASSETS_DIR", assetsDir)
	defer os.RemoveAll(assetsDir)

	// Fast flush so click-count assertions do not wait 30s.
	flushCtx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go counter.NewFlusher(st, ctr, flushInterval).Run(flushCtx)

	addr := "127.0.0.1:" + port
	log.Printf("e2e server listening on http://%s (admin password: %s)", addr, adminPassword)
	if err := http.ListenAndServe(addr, api.NewServer(st, cfg, ctr).Handler()); err != nil {
		log.Fatal(err)
	}
}
