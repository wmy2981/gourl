package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/wmy2981/gourl/internal/api"
	"github.com/wmy2981/gourl/internal/config"
	"github.com/wmy2981/gourl/internal/counter"
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
	cfg, err := config.NewManager(envOr("CONFIG_PATH", "config.yaml"))
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	dbPath := envOr("DB_PATH", "data/gourl.db")
	if dir := filepath.Dir(dbPath); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			log.Fatalf("create data dir: %v", err)
		}
	}
	st, err := store.Open(dbPath)
	if err != nil {
		log.Fatalf("store: %v", err)
	}
	defer st.Close()

	redisAddr := envOr("REDIS_ADDR", "localhost:6379")
	ctr := counter.New(redisAddr)
	if err := ctr.Ping(context.Background()); err != nil {
		log.Printf("redis unavailable at %s: %v (clicks will not be counted)", redisAddr, err)
	}

	addr := ":" + envOr("PORT", "8080")
	log.Printf("gourl %s listening on %s", version.Version, addr)
	if err := http.ListenAndServe(addr, api.NewServer(st, cfg, ctr).Handler()); err != nil {
		log.Fatal(err)
	}
}
