package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadMissingFileReturnsDefaults(t *testing.T) {
	cfg, err := Load(filepath.Join(t.TempDir(), "nope.yaml"))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.ShortCodeLength != 6 {
		t.Errorf("default short_code_length = %d, want 6", cfg.ShortCodeLength)
	}
	if cfg.Site.Name != "gourl" {
		t.Errorf("default site name = %q, want gourl", cfg.Site.Name)
	}
}

func TestLoadAndValidate(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	content := `
site:
  name: My Shortener
  title: My Title
short_code_length: 8
base_url: "https://s.example.com"
extra_base_urls:
  - "https://s2.example.com"
reserved_codes:
  - "foo"
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Site.Name != "My Shortener" || cfg.ShortCodeLength != 8 || cfg.BaseURL != "https://s.example.com" {
		t.Errorf("unexpected config: %+v", cfg)
	}
	if len(cfg.ExtraBaseURLs) != 1 || cfg.ExtraBaseURLs[0] != "https://s2.example.com" {
		t.Errorf("unexpected extra base urls: %v", cfg.ExtraBaseURLs)
	}
}

func TestValidateRejectsBadConfigs(t *testing.T) {
	cases := []struct {
		name string
		mut  func(*Config)
	}{
		{"short code length too small", func(c *Config) { c.ShortCodeLength = 3 }},
		{"short code length too large", func(c *Config) { c.ShortCodeLength = 33 }},
		{"base url not absolute", func(c *Config) { c.BaseURL = "s.example.com" }},
		{"base url wrong scheme", func(c *Config) { c.BaseURL = "ftp://s.example.com" }},
		{"extra base url invalid", func(c *Config) { c.ExtraBaseURLs = []string{"not-a-url"} }},
		{"reserved code invalid char", func(c *Config) { c.ReservedCodes = []string{"a b"} }},
		{"reserved code empty segment", func(c *Config) { c.ReservedCodes = []string{"foo//bar"} }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := Default()
			tc.mut(c)
			if err := c.Validate(); err == nil {
				t.Fatal("expected validation error, got nil")
			}
		})
	}
}

func TestValidateLogLevel(t *testing.T) {
	for _, good := range []string{"", "debug", "info", "warning", "warn", "error"} {
		c := Default()
		c.LogLevel = good
		if err := c.Validate(); err != nil {
			t.Errorf("Validate(log_level %q) = %v", good, err)
		}
	}
	// Empty falls back to info.
	c := Default()
	c.LogLevel = ""
	if err := c.Validate(); err != nil || c.LogLevel != "info" {
		t.Errorf("empty log_level should fall back to info: %v, %q", err, c.LogLevel)
	}
	c = Default()
	c.LogLevel = "verbose"
	if err := c.Validate(); err == nil {
		t.Error("Validate(log_level verbose) should fail")
	}
}

func TestValidateIPBlocks(t *testing.T) {
	for _, bad := range []string{"", "999.1.2.3", "192.168.1", "not-an-ip", "192.168.*.x", "a.b.c.d"} {
		c := Default()
		c.IPBlocks = []string{bad}
		if err := c.Validate(); err == nil {
			t.Errorf("Validate(ip_blocks %q) should fail", bad)
		}
	}
	c := Default()
	c.IPBlocks = []string{"192.168.1.1", "10.0.0.0/8", "192.168.*.*", "2001:db8::1"}
	if err := c.Validate(); err != nil {
		t.Errorf("Validate(valid ip_blocks) = %v", err)
	}
}

func TestValidateAcceptsChineseAndMultiSegmentReservedCodes(t *testing.T) {
	c := Default()
	c.ReservedCodes = []string{"中文", "帮助/指南", "foo/bar", "short"}
	if err := c.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
}

func TestValidateEmptyNameFallsBack(t *testing.T) {
	c := Default()
	c.Site.Name = ""
	if err := c.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if c.Site.Name != "gourl" {
		t.Errorf("name = %q, want gourl", c.Site.Name)
	}
}

func TestGetNormalizesNilSlices(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	m, err := NewManager(path) // defaults: no base urls, no reserved codes
	if err != nil {
		t.Fatal(err)
	}
	cfg := m.Get()
	if cfg.ExtraBaseURLs == nil || cfg.ReservedCodes == nil {
		t.Fatalf("slice fields must be empty arrays, got %#v / %#v", cfg.ExtraBaseURLs, cfg.ReservedCodes)
	}
}

func TestManagerUpdateWritesBackAndHotSwaps(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}
	m, err := NewManager(path)
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	updated := m.Get()
	updated.Site.Name = "Renamed"
	updated.ShortCodeLength = 10
	if err := m.Update(updated); err != nil {
		t.Fatalf("Update: %v", err)
	}

	// Hot swap visible immediately.
	if got := m.Get().Site.Name; got != "Renamed" {
		t.Errorf("in-memory name = %q, want Renamed", got)
	}

	// Persisted and reloadable.
	reloaded, err := Load(path)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if reloaded.Site.Name != "Renamed" || reloaded.ShortCodeLength != 10 {
		t.Errorf("reloaded config = %+v", reloaded)
	}
}

func TestManagerUpdateRejectsInvalidAndKeepsOld(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	m, err := NewManager(path)
	if err != nil {
		t.Fatal(err)
	}
	bad := m.Get()
	bad.BaseURL = "not-a-url"
	if err := m.Update(bad); err == nil {
		t.Fatal("expected error for invalid update")
	}
	if m.Get().BaseURL != "" {
		t.Errorf("config changed after rejected update: %+v", m.Get())
	}
}
