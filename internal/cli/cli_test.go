package cli

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	_ "modernc.org/sqlite"

	"github.com/wmy2981/gourl/internal/store"
)

// captureStdout runs f and returns what it printed plus its exit code.
func captureStdout(t *testing.T, f func() int) (string, int) {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w
	code := f()
	w.Close()
	os.Stdout = old
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		t.Fatal(err)
	}
	return buf.String(), code
}

// TestConfirmRequiresTerminal: without a terminal and without -y the
// operation is refused; -y skips the prompt entirely.
func TestConfirmRequiresTerminal(t *testing.T) {
	if err := confirm(false, "proceed?"); err == nil {
		t.Fatal("expected refusal without a terminal and without -y")
	}
	if err := confirm(true, "proceed?"); err != nil {
		t.Fatalf("confirm(true) should skip the prompt: %v", err)
	}
}

// TestExportDBFormat: the CLI export must mirror scripts/db-export.mts —
// nullable columns stay null, deleted becomes a boolean, b_id becomes "b-N".
func TestExportDBFormat(t *testing.T) {
	dir := t.TempDir()
	db, err := sql.Open("sqlite", "file:"+filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	// Schema equivalent to migrations v1..v5 for the four exported tables.
	for _, ddl := range []string{
		`CREATE TABLE links (id INTEGER PRIMARY KEY AUTOINCREMENT, code TEXT NOT NULL UNIQUE, url TEXT NOT NULL,
			title TEXT, description TEXT, expires_at INTEGER NOT NULL DEFAULT 0, click_count INTEGER NOT NULL DEFAULT 0,
			created_at INTEGER NOT NULL, updated_at INTEGER NOT NULL, deleted INTEGER NOT NULL DEFAULT 0)`,
		`CREATE TABLE api_tokens (id INTEGER PRIMARY KEY AUTOINCREMENT, token TEXT NOT NULL UNIQUE,
			note TEXT NOT NULL DEFAULT '', created_at INTEGER NOT NULL, deleted INTEGER NOT NULL DEFAULT 0)`,
		`CREATE TABLE daily_clicks (link_id INTEGER, code TEXT NOT NULL, date TEXT NOT NULL, count INTEGER NOT NULL)`,
		`CREATE TABLE backups (b_id INTEGER PRIMARY KEY, link_id INTEGER NOT NULL, code TEXT NOT NULL, url TEXT NOT NULL,
			title TEXT, description TEXT, expires_at INTEGER NOT NULL DEFAULT 0, click_count INTEGER NOT NULL DEFAULT 0,
			created_at INTEGER NOT NULL, updated_at INTEGER NOT NULL, backed_at INTEGER NOT NULL)`,
	} {
		if _, err := db.Exec(ddl); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := db.Exec(`INSERT INTO links (code, url, title, created_at, updated_at, deleted) VALUES ('abc', 'https://e.com', NULL, 1, 1, 0)`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO links (code, url, title, created_at, updated_at, deleted) VALUES ('del', 'https://d.com', 'gone', 2, 2, 1)`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO api_tokens (token, note, created_at, deleted) VALUES ('tok-1', 'ci', 3, 0)`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO daily_clicks (link_id, code, date, count) VALUES (1, 'abc', '2026-08-18', 5)`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO daily_clicks (link_id, code, date, count) VALUES (NULL, 'orphan', '2025-01-01', 2)`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO backups (b_id, link_id, code, url, title, created_at, updated_at, backed_at) VALUES (1, 1, 'abc', 'https://e.com', 'old', 1, 1, 1)`); err != nil {
		t.Fatal(err)
	}

	out := filepath.Join(dir, "out")
	if err := os.MkdirAll(out, 0o755); err != nil {
		t.Fatal(err)
	}
	nLinks, nTokens, nDaily, nBackups, err := exportDB(db, out)
	if err != nil {
		t.Fatal(err)
	}
	if nLinks != 2 || nTokens != 1 || nDaily != 2 || nBackups != 1 {
		t.Fatalf("counts: links=%d tokens=%d daily=%d backups=%d", nLinks, nTokens, nDaily, nBackups)
	}

	// links.json: newest first (created_at DESC), deleted as boolean, NULL
	// title stays null.
	var links []exportLink
	readJSON(t, filepath.Join(out, "links.json"), &links)
	if len(links) != 2 {
		t.Fatalf("links rows = %d, want 2", len(links))
	}
	if !links[0].Deleted || links[0].Title == nil || *links[0].Title != "gone" {
		t.Errorf("newest link should be the deleted one: %+v", links[0])
	}
	if links[1].Deleted || links[1].Title != nil {
		t.Errorf("second link: deleted=%v title=%v, want false/nil", links[1].Deleted, links[1].Title)
	}

	// daily-clicks.json: the NULL link_id orphan stays null.
	var dailies []exportDailyClick
	readJSON(t, filepath.Join(out, "daily-clicks.json"), &dailies)
	foundOrphan := false
	for _, d := range dailies {
		if d.Code == "orphan" {
			foundOrphan = true
			if d.LinkID != nil {
				t.Errorf("orphan daily click link_id = %v, want null", d.LinkID)
			}
		}
	}
	if !foundOrphan {
		t.Error("orphan daily click row missing")
	}

	// backups.json: b_id renders as "b-N".
	var backups []exportBackup
	readJSON(t, filepath.Join(out, "backups.json"), &backups)
	if len(backups) != 1 || backups[0].BID != "b-1" {
		t.Errorf("backups = %+v, want a single b-1", backups)
	}

	// tokens.json carries the full token value.
	var tokens []exportToken
	readJSON(t, filepath.Join(out, "tokens.json"), &tokens)
	if len(tokens) != 1 || tokens[0].Token != "tok-1" {
		t.Errorf("tokens = %+v", tokens)
	}
}

func readJSON(t *testing.T, path string, v any) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, v); err != nil {
		t.Fatal(err)
	}
}

// writeConfig creates a temp config file and points CONFIG_PATH at it.
func writeConfig(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CONFIG_PATH", path)
	return path
}

func TestConfigShowHidesPasswordHash(t *testing.T) {
	writeConfig(t, "site:\n  name: test\npassword_hash: $2a$10$secret\n")
	out, code := captureStdout(t, func() int { return cmdConfig([]string{"show"}) })
	if code != 0 {
		t.Fatalf("config show exit = %d, out %s", code, out)
	}
	if strings.Contains(out, "$2a$10$secret") {
		t.Error("config show must not print the password hash")
	}
	if !strings.Contains(out, "name: test") {
		t.Errorf("config show should include the config: %s", out)
	}
}

func TestResetPasswordClearsHash(t *testing.T) {
	writeConfig(t, "site:\n  name: test\npassword_hash: $2a$10$secret\n")
	code := cmdReset([]string{"-y", "password"})
	if code != 0 {
		t.Fatalf("reset password exit = %d", code)
	}
	data, err := os.ReadFile(cfgPath())
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "$2a$10$secret") {
		t.Error("reset password must remove the hash from the config file")
	}
}

func TestWebUIToggle(t *testing.T) {
	writeConfig(t, "site:\n  name: test\n")
	code := cmdWebUI([]string{"-y", "off"})
	if code != 0 {
		t.Fatalf("webui off exit = %d", code)
	}
	data, _ := os.ReadFile(cfgPath())
	if !strings.Contains(string(data), "webui_enabled: false") {
		t.Errorf("webui off did not persist: %s", data)
	}
	code = cmdWebUI([]string{"-y", "on"})
	if code != 0 {
		t.Fatalf("webui on exit = %d", code)
	}
	data, _ = os.ReadFile(cfgPath())
	if !strings.Contains(string(data), "webui_enabled: true") {
		t.Errorf("webui on did not persist: %s", data)
	}
	// Bad usage.
	if code := cmdWebUI(nil); code != 2 {
		t.Errorf("webui without args exit = %d, want 2", code)
	}
}

func TestStatusAndHealth(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("CONFIG_PATH", filepath.Join(dir, "config.yaml"))
	t.Setenv("DB_PATH", filepath.Join(dir, "data", "gourl.db"))
	t.Setenv("REDIS_ADDR", "127.0.0.1:1") // unreachable
	// Ensure no database file exists (status must not create one).
	out, code := captureStdout(t, func() int { return cmdStatus() })
	if code != 0 {
		t.Errorf("status exit = %d, out %s", code, out)
	}
	if !strings.Contains(out, "gourl ") || !strings.Contains(out, "missing") {
		t.Errorf("status output unexpected: %s", out)
	}
	if _, err := os.Stat(dbPath()); !os.IsNotExist(err) {
		t.Error("status must not create the database file")
	}
	_, code = captureStdout(t, func() int { return cmdHealth() })
	if code != 1 {
		t.Errorf("health exit = %d, want 1 (redis down)", code)
	}
}

// TestSetupCodeCommand: `gourl setup-code` prints the persisted bootstrap
// code and fails (exit 1) when the server is not in setup mode.
func TestSetupCodeCommand(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("DB_PATH", filepath.Join(dir, "gourl.db"))
	if code := cmdSetupCode(); code != 1 {
		t.Errorf("setup-code without a code file exit = %d, want 1", code)
	}
	if err := os.WriteFile(filepath.Join(dir, "setup.code"), []byte("Ab3xYz9k"), 0o600); err != nil {
		t.Fatal(err)
	}
	out, code := captureStdout(t, func() int { return cmdSetupCode() })
	if code != 0 || out != "Ab3xYz9k" {
		t.Errorf("setup-code exit = %d, out %q, want 0 and the code", code, out)
	}
}

func TestVersionPrintsVersion(t *testing.T) {
	out, code := captureStdout(t, func() int { return Main([]string{"version"}) })
	if code != 0 || strings.TrimSpace(out) == "" {
		t.Errorf("version exit = %d, out %q", code, out)
	}
}

// TestResetApiRevokesTokens exercises the whole path a container would run:
// a real sqlite file, a token seeded through the store, then cmdReset, then a
// fresh store proving the token no longer authenticates.
func TestResetApiRevokesTokens(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "gourl.db")
	t.Setenv("DB_PATH", path)

	st, err := store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.CreateToken(context.Background(), "tok-cli", "ci", 1); err != nil {
		st.Close()
		t.Fatal(err)
	}
	st.Close()

	code := cmdReset([]string{"-y", "api"})
	if code != 0 {
		t.Fatalf("reset api exit = %d", code)
	}

	st2, err := store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer st2.Close()
	if _, err := st2.GetToken(context.Background(), "tok-cli"); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("token after reset api = %v, want ErrNotFound", err)
	}
}
