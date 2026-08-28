// Package config loads and manages the YAML business configuration.
//
// The config file holds business settings (site info, short code length,
// base URLs, reserved codes, icon). Runtime and secrets live in environment
// variables, never here. The Manager supports hot-reload: updates validate,
// are written back atomically, and take effect without a restart.
package config

import (
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/wmy2981/gourl/internal/shortcode"
	"gopkg.in/yaml.v3"
)

// Site holds single-language site information.
type Site struct {
	Name        string `yaml:"name" json:"name"`
	Title       string `yaml:"title" json:"title"`
	Description string `yaml:"description" json:"description"`
}

// Config is the YAML business configuration.
type Config struct {
	Site            Site     `yaml:"site" json:"site"`
	ShortCodeLength int      `yaml:"short_code_length" json:"short_code_length"`
	BaseURL         string   `yaml:"base_url" json:"base_url"`
	ExtraBaseURLs   []string `yaml:"extra_base_urls" json:"extra_base_urls"`
	ReservedCodes   []string `yaml:"reserved_codes" json:"reserved_codes"`
	UABlocks        []string `yaml:"ua_blocks" json:"ua_blocks"`
	IPBlocks        []string `yaml:"ip_blocks" json:"ip_blocks"`
	Icon            string   `yaml:"icon" json:"icon"`
	// LoginRateMaxAttempts / LoginRateLockSeconds limit failed logins per IP:
	// N failures in a row lock the address for the window. 0 disables.
	LoginRateMaxAttempts int `yaml:"login_rate_max_attempts" json:"login_rate_max_attempts"`
	LoginRateLockSeconds int `yaml:"login_rate_lock_seconds" json:"login_rate_lock_seconds"`
	// SessionTTLMinutes is how long an admin session stays valid. 0 means
	// sessions never expire; the minimum is 5 minutes. Changing it only
	// affects newly issued sessions — revoking existing ones is a session
	// epoch bump (gourl reset sessions / password change).
	SessionTTLMinutes int `yaml:"session_ttl_minutes" json:"session_ttl_minutes"`
	// LinkRatePerSecond caps short-link redirects across all codes (a shared
	// token bucket). 0 disables.
	LinkRatePerSecond int `yaml:"link_rate_per_second" json:"link_rate_per_second"`
	// PasswordHash is the bcrypt hash of the admin password, set through the
	// setup flow (or migrated from the legacy ADMIN_PASSWORD env var). It is
	// never exposed to the frontend.
	PasswordHash string `yaml:"password_hash" json:"-"`
	// SessionEpoch invalidates every issued session at once when bumped
	// (gourl reset sessions, password change). Kept out of the JSON contract;
	// updateConfig carries it over so a plain PUT never resets it.
	SessionEpoch int64 `yaml:"session_epoch" json:"-"`
	// WebUIEnabled gates the admin console (/admin) via `gourl webui on/off`.
	// Swagger /docs is unaffected. Kept out of the JSON contract; updateConfig
	// carries it over so a plain PUT never disables it.
	WebUIEnabled bool `yaml:"webui_enabled" json:"-"`
	// SQLConsoleEnabled gates POST /api/v1/db (the raw SQL console). Default
	// off; like webui_enabled it is a file-only field, toggled through the
	// config file + `gourl reload`, never exposed to the JSON contract.
	SQLConsoleEnabled bool `yaml:"sql_console_enabled" json:"-"`
	// LogLevel is the process-wide log verbosity (debug/info/warning/error),
	// applied at startup and hot-applied on every config save.
	LogLevel string `yaml:"log_level" json:"log_level"`
	// HardDelete makes every link deletion physically remove the row instead
	// of soft-deleting it. Daily click history is always kept; API tokens are
	// unaffected (their keys stay permanently taken).
	HardDelete bool `yaml:"hard_delete" json:"hard_delete"`
	// BackupOnEdit controls whether edits snapshot the pre-edit state into
	// the backups table (manual edits and batch conflict=update). Default
	// true; turning it off stops new snapshots but keeps existing ones.
	BackupOnEdit *bool `yaml:"backup_on_edit" json:"backup_on_edit"`
}

// Default returns a usable default configuration.
func Default() *Config {
	return &Config{
		Site:                 Site{Name: "gourl", Title: "gourl - Short Links", Description: "Lightweight self-hosted URL shortener."},
		ShortCodeLength:      4,
		LoginRateMaxAttempts: 10,
		LoginRateLockSeconds: 300,
		SessionTTLMinutes:    10080, // 7 days, matching the pre-config default
		LinkRatePerSecond:    100,
		WebUIEnabled:         true,
		LogLevel:             "info",
	}
}

// BackupOnEditEnabled reports the effective backup_on_edit value: backups
// are on unless explicitly turned off (a missing YAML key keeps them on).
func (c *Config) BackupOnEditEnabled() bool {
	return c.BackupOnEdit == nil || *c.BackupOnEdit
}

// Load reads the YAML file at path; a missing file yields the defaults.
func Load(path string) (*Config, error) {
	cfg := Default()
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return cfg, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parse config %s: %w", path, err)
	}
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config %s: %w", path, err)
	}
	return cfg, nil
}

// ValidationError is the typed validation failure: a stable machine-readable
// Code (surfaced to API clients for i18n mapping), a human-readable English
// Message, and optional Params interpolated into translated texts.
type ValidationError struct {
	Code    string
	Message string
	Params  map[string]any
}

func (e *ValidationError) Error() string { return e.Message }

// Validate checks constraints. An empty name falls back to "gourl".
// Failures come back as *ValidationError with one stable code per rule so
// clients can translate them.
func (c *Config) Validate() error {
	if c.ShortCodeLength < 2 || c.ShortCodeLength > 64 {
		return &ValidationError{Code: "short_code_length_range",
			Message: fmt.Sprintf("short_code_length must be between 2 and 64, got %d", c.ShortCodeLength),
			Params:  map[string]any{"min": 2, "max": 64, "got": c.ShortCodeLength},
		}
	}
	if c.Site.Name == "" {
		c.Site.Name = "gourl"
	}
	if c.BaseURL != "" && !isAbsoluteHTTPURL(c.BaseURL) {
		return &ValidationError{Code: "invalid_base_url",
			Message: "base_url must be an absolute http(s) URL",
		}
	}
	for _, u := range c.ExtraBaseURLs {
		if !isAbsoluteHTTPURL(u) {
			return &ValidationError{Code: "invalid_extra_base_url",
				Message: fmt.Sprintf("extra_base_url %q must be an absolute http(s) URL", u),
				Params:  map[string]any{"url": u},
			}
		}
	}
	for _, r := range c.ReservedCodes {
		if err := validReservedCode(r); err != nil {
			return &ValidationError{Code: "invalid_reserved_code",
				Message: err.Error(),
				Params:  map[string]any{"entry": r},
			}
		}
	}
	for _, b := range c.IPBlocks {
		if err := validIPBlock(b); err != nil {
			return &ValidationError{Code: "invalid_ip_block",
				Message: err.Error(),
				Params:  map[string]any{"entry": b},
			}
		}
	}
	if c.LoginRateMaxAttempts < 0 || c.LoginRateLockSeconds < 0 {
		return &ValidationError{Code: "negative_login_rate",
			Message: "login rate limits must not be negative (0 disables)",
		}
	}
	if c.SessionTTLMinutes != 0 && c.SessionTTLMinutes < 5 {
		return &ValidationError{Code: "invalid_session_ttl",
			Message: fmt.Sprintf("session_ttl_minutes must be 0 (never expire) or at least 5, got %d", c.SessionTTLMinutes),
			Params:  map[string]any{"got": c.SessionTTLMinutes},
		}
	}
	if c.LinkRatePerSecond < 0 {
		return &ValidationError{Code: "negative_link_rate",
			Message: "link_rate_per_second must not be negative (0 disables)",
		}
	}
	switch c.LogLevel {
	case "":
		c.LogLevel = "info"
	case "debug", "info", "warning", "warn", "error":
	default:
		return &ValidationError{Code: "invalid_log_level",
			Message: fmt.Sprintf("log_level must be debug, info, warning or error, got %q", c.LogLevel),
			Params:  map[string]any{"got": c.LogLevel},
		}
	}
	return nil
}

// validIPBlock accepts a single IP, a CIDR network, or an IPv4 dotted-quad
// rule with "*" segments (e.g. 192.168.*.*).
func validIPBlock(s string) error {
	if s == "" {
		return fmt.Errorf("ip_blocks entries must not be empty")
	}
	if _, _, err := net.ParseCIDR(s); err == nil {
		return nil
	}
	if net.ParseIP(s) != nil {
		return nil
	}
	if strings.Contains(s, "*") {
		parts := strings.Split(s, ".")
		if len(parts) == 4 {
			for _, p := range parts {
				if p == "*" {
					continue
				}
				n, err := strconv.Atoi(p)
				if err != nil || n < 0 || n > 255 {
					return fmt.Errorf("invalid ip_blocks entry %q", s)
				}
			}
			return nil
		}
	}
	return fmt.Errorf("invalid ip_blocks entry %q: want IP, CIDR or dotted-quad with '*' segments", s)
}

func isAbsoluteHTTPURL(s string) bool {
	u, err := url.Parse(s)
	return err == nil && u.IsAbs() && (u.Scheme == "http" || u.Scheme == "https") && u.Host != ""
}

// validReservedCode accepts a non-empty reserved entry. Entries are matched
// against short codes, so they must be valid code shapes too: url-safe
// characters (ASCII + CJK), non-empty segments, at most MaxSegments levels.
func validReservedCode(s string) error {
	if s == "" {
		return fmt.Errorf("reserved_codes entries must not be empty")
	}
	if err := shortcode.Validate(s); err != nil {
		return fmt.Errorf("invalid reserved_codes entry %q: %w", s, err)
	}
	return nil
}

// Manager owns the live config, supports concurrent reads and hot updates.
type Manager struct {
	mu   sync.RWMutex
	cfg  *Config
	path string
}

// NewManager loads the config file at path and returns a Manager.
func NewManager(path string) (*Manager, error) {
	cfg, err := Load(path)
	if err != nil {
		return nil, err
	}
	return &Manager{cfg: cfg, path: path}, nil
}

// Get returns a copy of the current config. Slice fields are normalized to
// empty (not nil) slices so JSON never emits null — the frontend relies on
// them being arrays.
func (m *Manager) Get() *Config {
	m.mu.RLock()
	defer m.mu.RUnlock()
	cp := *m.cfg
	cp.ExtraBaseURLs = append([]string(nil), m.cfg.ExtraBaseURLs...)
	cp.ReservedCodes = append([]string(nil), m.cfg.ReservedCodes...)
	cp.UABlocks = append([]string(nil), m.cfg.UABlocks...)
	cp.IPBlocks = append([]string(nil), m.cfg.IPBlocks...)
	if cp.ExtraBaseURLs == nil {
		cp.ExtraBaseURLs = []string{}
	}
	if cp.ReservedCodes == nil {
		cp.ReservedCodes = []string{}
	}
	if cp.UABlocks == nil {
		cp.UABlocks = []string{}
	}
	if cp.IPBlocks == nil {
		cp.IPBlocks = []string{}
	}
	return &cp
}

// Reload re-reads the config file and hot-swaps it into memory. On a parse
// or validation failure the in-memory config is unchanged and the error is
// returned — the CLI edits the file in a separate process, so a broken edit
// must never take the running server's config down.
func (m *Manager) Reload() error {
	cfg, err := Load(m.path)
	if err != nil {
		return err
	}
	m.mu.Lock()
	m.cfg = cfg
	m.mu.Unlock()
	return nil
}

// Update validates the new config, writes it back atomically to disk, and
// hot-swaps it into memory. On write failure the in-memory config is unchanged.
// A nil BackupOnEdit (key absent from the YAML/JSON) is normalized to an
// explicit true so a partial PUT cannot silently disable backups.
func (m *Manager) Update(c *Config) error {
	if c.BackupOnEdit == nil {
		on := true
		c.BackupOnEdit = &on
	}
	if err := c.Validate(); err != nil {
		return err
	}
	data, err := yaml.Marshal(c)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	if err := atomicWrite(m.path, data); err != nil {
		slog.Debug("config write failed", "path", m.path, "error", err)
		return err
	}
	slog.Debug("config written", "path", m.path)
	m.mu.Lock()
	m.cfg = c
	m.mu.Unlock()
	return nil
}

// atomicWrite writes data to path via a temp file and rename, so readers
// never observe a partially written config.
func atomicWrite(path string, data []byte) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".config-*.tmp")
	if err != nil {
		return fmt.Errorf("create temp config: %w", err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName) // no-op after successful rename
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return fmt.Errorf("write temp config: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp config: %w", err)
	}
	// os.Rename does not overwrite an existing file on Windows.
	if _, err := os.Stat(path); err == nil {
		if err := os.Remove(path); err != nil {
			return fmt.Errorf("remove old config: %w", err)
		}
	}
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("rename config into place: %w", err)
	}
	return nil
}
