package config

import (
	"errors"
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
		{"short code length too small", func(c *Config) { c.ShortCodeLength = 1 }},
		{"short code length too large", func(c *Config) { c.ShortCodeLength = 65 }},
		{"base url not absolute", func(c *Config) { c.BaseURL = "s.example.com" }},
		{"base url wrong scheme", func(c *Config) { c.BaseURL = "ftp://s.example.com" }},
		{"extra base url invalid", func(c *Config) { c.ExtraBaseURLs = []string{"not-a-url"} }},
		{"reserved code invalid char", func(c *Config) { c.ReservedCodes = []string{"a b"} }},
		{"reserved code empty segment", func(c *Config) { c.ReservedCodes = []string{"foo//bar"} }},
		{"session ttl below minimum", func(c *Config) { c.SessionTTLMinutes = 4 }},
		{"session ttl negative", func(c *Config) { c.SessionTTLMinutes = -1 }},
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

// TestValidateErrorCodes: every validation rule reports its stable
// machine-readable code (and interpolation params where applicable).
func TestValidateErrorCodes(t *testing.T) {
	cases := []struct {
		name   string
		mut    func(*Config)
		code   string
		params map[string]any
	}{
		{"short code length too small", func(c *Config) { c.ShortCodeLength = 1 },
			"short_code_length_range", map[string]any{"min": 2, "max": 64, "got": 1}},
		{"short code length too large", func(c *Config) { c.ShortCodeLength = 65 },
			"short_code_length_range", map[string]any{"min": 2, "max": 64, "got": 65}},
		{"base url not absolute", func(c *Config) { c.BaseURL = "s.example.com" }, "invalid_base_url", nil},
		{"extra base url invalid", func(c *Config) { c.ExtraBaseURLs = []string{"not-a-url"} },
			"invalid_extra_base_url", map[string]any{"url": "not-a-url"}},
		{"reserved code invalid char", func(c *Config) { c.ReservedCodes = []string{"a b"} },
			"invalid_reserved_code", map[string]any{"entry": "a b"}},
		{"reserved code empty", func(c *Config) { c.ReservedCodes = []string{""} },
			"invalid_reserved_code", map[string]any{"entry": ""}},
		{"ip block invalid", func(c *Config) { c.IPBlocks = []string{"not-an-ip"} },
			"invalid_ip_block", map[string]any{"entry": "not-an-ip"}},
		{"ip block empty", func(c *Config) { c.IPBlocks = []string{""} },
			"invalid_ip_block", map[string]any{"entry": ""}},
		{"login rate negative", func(c *Config) { c.LoginRateMaxAttempts = -1 },
			"negative_login_rate", nil},
		{"login rate lock negative", func(c *Config) { c.LoginRateLockSeconds = -1 },
			"negative_login_rate", nil},
		{"link rate negative", func(c *Config) { c.LinkRatePerSecond = -1 },
			"negative_link_rate", nil},
		{"session ttl below minimum", func(c *Config) { c.SessionTTLMinutes = 3 },
			"invalid_session_ttl", map[string]any{"got": 3}},
		{"log level unknown", func(c *Config) { c.LogLevel = "verbose" },
			"invalid_log_level", map[string]any{"got": "verbose"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := Default()
			tc.mut(c)
			err := c.Validate()
			var ve *ValidationError
			if !errors.As(err, &ve) {
				t.Fatalf("Validate error = %v, want *ValidationError", err)
			}
			if ve.Code != tc.code {
				t.Errorf("code = %q, want %q", ve.Code, tc.code)
			}
			if tc.params == nil {
				if len(ve.Params) != 0 {
					t.Errorf("params = %v, want none", ve.Params)
				}
			} else {
				for k, want := range tc.params {
					if got := ve.Params[k]; got != want {
						t.Errorf("params[%q] = %v, want %v", k, got, want)
					}
				}
			}
			if ve.Message == "" {
				t.Error("Message must stay non-empty for unknown-code fallbacks")
			}
		})
	}
}

func TestValidateSessionTTL(t *testing.T) {
	for _, good := range []int{0, 5, 30, 10080} {
		c := Default()
		c.SessionTTLMinutes = good
		if err := c.Validate(); err != nil {
			t.Errorf("Validate(session_ttl_minutes %d) = %v", good, err)
		}
	}
	for _, bad := range []int{-5, 1, 4} {
		c := Default()
		c.SessionTTLMinutes = bad
		if err := c.Validate(); err == nil {
			t.Errorf("Validate(session_ttl_minutes %d) should fail", bad)
		}
	}
}

func TestLoadKeepsSessionTTLDefaultForOldFiles(t *testing.T) {
	// A config file written before session_ttl_minutes existed must keep the
	// 7-day default instead of falling into the 0 = never expire branch.
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte("site:\n  name: legacy\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.SessionTTLMinutes != 10080 {
		t.Errorf("session_ttl_minutes = %d, want the 7-day default 10080", cfg.SessionTTLMinutes)
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

// TestManagerReloadPicksUpExternalEdits: another process (the CLI) edits the
// file behind the manager's back; Reload hot-swaps it in. A broken edit
// keeps the previous config.
func TestManagerReloadPicksUpExternalEdits(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	m, err := NewManager(path)
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	if m.Get().Site.Name == "Renamed" {
		t.Fatal("test precondition: default name must differ from Renamed")
	}

	if err := os.WriteFile(path, []byte("site:\n  name: Renamed\nwebui_enabled: false\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := m.Reload(); err != nil {
		t.Fatalf("Reload: %v", err)
	}
	cfg := m.Get()
	if cfg.Site.Name != "Renamed" || cfg.WebUIEnabled {
		t.Errorf("reloaded config = name %q webui %v, want Renamed/false", cfg.Site.Name, cfg.WebUIEnabled)
	}

	if err := os.WriteFile(path, []byte("site:\n  name: Broken\nshort_code_length: 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := m.Reload(); err == nil {
		t.Error("expected error reloading an invalid config file")
	}
	cfg = m.Get()
	if cfg.Site.Name != "Renamed" || cfg.WebUIEnabled {
		t.Errorf("failed reload changed the config: name %q webui %v", cfg.Site.Name, cfg.WebUIEnabled)
	}
}
