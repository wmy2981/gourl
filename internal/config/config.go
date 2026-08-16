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
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"gopkg.in/yaml.v3"
)

// Site holds single-language site information.
type Site struct {
	Name        string `yaml:"name" json:"name"`
	Title       string `yaml:"title" json:"title"`
	Keywords    string `yaml:"keywords" json:"keywords"`
	Description string `yaml:"description" json:"description"`
	Header      string `yaml:"header" json:"header"`
	Footer      string `yaml:"footer" json:"footer"`
}

// Config is the YAML business configuration.
type Config struct {
	Site            Site     `yaml:"site" json:"site"`
	ShortCodeLength int      `yaml:"short_code_length" json:"short_code_length"`
	BaseURL         string   `yaml:"base_url" json:"base_url"`
	ExtraBaseURLs   []string `yaml:"extra_base_urls" json:"extra_base_urls"`
	ReservedCodes   []string `yaml:"reserved_codes" json:"reserved_codes"`
	UABlocks        []string `yaml:"ua_blocks" json:"ua_blocks"`
	Icon            string   `yaml:"icon" json:"icon"`
}

// Default returns a usable default configuration.
func Default() *Config {
	return &Config{
		Site:            Site{Name: "gourl", Title: "gourl - Short Links"},
		ShortCodeLength: 6,
	}
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

// Validate checks constraints. An empty name falls back to "gourl".
func (c *Config) Validate() error {
	if c.ShortCodeLength < 4 || c.ShortCodeLength > 32 {
		return fmt.Errorf("short_code_length must be between 4 and 32, got %d", c.ShortCodeLength)
	}
	if c.Site.Name == "" {
		c.Site.Name = "gourl"
	}
	if c.BaseURL != "" && !isAbsoluteHTTPURL(c.BaseURL) {
		return fmt.Errorf("base_url must be an absolute http(s) URL")
	}
	for _, u := range c.ExtraBaseURLs {
		if !isAbsoluteHTTPURL(u) {
			return fmt.Errorf("extra_base_url %q must be an absolute http(s) URL", u)
		}
	}
	for _, r := range c.ReservedCodes {
		if err := validReservedCode(r); err != nil {
			return err
		}
	}
	return nil
}

func isAbsoluteHTTPURL(s string) bool {
	u, err := url.Parse(s)
	return err == nil && u.IsAbs() && (u.Scheme == "http" || u.Scheme == "https") && u.Host != ""
}

// validReservedCode accepts a non-empty reserved prefix of url-safe characters.
func validReservedCode(s string) error {
	if s == "" {
		return fmt.Errorf("reserved_codes entries must not be empty")
	}
	if strings.ContainsAny(s, "/") {
		return fmt.Errorf("reserved_codes entry %q must be a single segment (no '/')", s)
	}
	for _, r := range s {
		if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '_') {
			return fmt.Errorf("reserved_codes entry %q contains invalid character %q", s, r)
		}
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
	if cp.ExtraBaseURLs == nil {
		cp.ExtraBaseURLs = []string{}
	}
	if cp.ReservedCodes == nil {
		cp.ReservedCodes = []string{}
	}
	if cp.UABlocks == nil {
		cp.UABlocks = []string{}
	}
	return &cp
}

// Update validates the new config, writes it back atomically to disk, and
// hot-swaps it into memory. On write failure the in-memory config is unchanged.
func (m *Manager) Update(c *Config) error {
	if err := c.Validate(); err != nil {
		return err
	}
	data, err := yaml.Marshal(c)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	if err := atomicWrite(m.path, data); err != nil {
		return err
	}
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
