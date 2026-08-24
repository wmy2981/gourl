package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/wmy2981/gourl/internal/store"
)

// enableSQLConsole flips the config switch (the same path a `gourl db
// console on` + reload would take in production).
func enableSQLConsole(t *testing.T, s *Server) {
	t.Helper()
	upd := s.cfg.Get()
	upd.SQLConsoleEnabled = true
	if err := s.cfg.Update(upd); err != nil {
		t.Fatalf("enable sql console: %v", err)
	}
}

func postSQL(t *testing.T, s *Server, sql string) *httptest.ResponseRecorder {
	t.Helper()
	return do(t, s, http.MethodPost, "/api/v1/db", map[string]string{"sql": sql})
}

func decodeConsoleResponse(t *testing.T, rec *httptest.ResponseRecorder) dbConsoleResponse {
	t.Helper()
	var resp dbConsoleResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return resp
}

// TestSQLConsoleDisabledByDefault: the endpoint exists but refuses with the
// stable error code until the config switch is turned on.
func TestSQLConsoleDisabledByDefault(t *testing.T) {
	s, _ := newTestServer(t)
	rec := postSQL(t, s, "SELECT 1")
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", rec.Code)
	}
	var errResp errorBody
	if err := json.NewDecoder(rec.Body).Decode(&errResp); err != nil {
		t.Fatal(err)
	}
	if errResp.Error.Code != "sql_console_disabled" {
		t.Errorf("error code = %q, want sql_console_disabled", errResp.Error.Code)
	}
}

// TestSQLConsoleRequiresAuth: without a session or token the request is
// rejected before the console switch is even consulted.
func TestSQLConsoleRequiresAuth(t *testing.T) {
	s, _ := newTestServer(t)
	enableSQLConsole(t, s)
	rec := doWith(t, s, http.MethodPost, "/api/v1/db", map[string]string{"sql": "SELECT 1"}, nil, "")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
}

// TestSQLConsoleSelect: a plain query returns columns and typed rows.
func TestSQLConsoleSelect(t *testing.T) {
	s, _ := newTestServer(t)
	enableSQLConsole(t, s)
	createLink(t, s, "abc", "https://example.com")
	rec := postSQL(t, s, "SELECT code, url FROM links WHERE deleted = 0 ORDER BY id")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body %s", rec.Code, rec.Body.String())
	}
	resp := decodeConsoleResponse(t, rec)
	if len(resp.Results) != 1 || resp.Results[0].SQL != "SELECT code, url FROM links WHERE deleted = 0 ORDER BY id" {
		t.Fatalf("unexpected results: %+v", resp.Results)
	}
	r0 := resp.Results[0]
	if len(r0.Columns) != 2 || r0.Columns[0] != "code" || r0.Columns[1] != "url" {
		t.Errorf("columns = %v", r0.Columns)
	}
	if len(r0.Rows) != 1 || r0.Rows[0][0] != "abc" || r0.Rows[0][1] != "https://example.com" {
		t.Errorf("rows = %v", r0.Rows)
	}
}

// TestSQLConsoleMultiStatementTransactionRollback: a batch whose last
// statement fails leaves no partial writes behind.
func TestSQLConsoleMultiStatementTransactionRollback(t *testing.T) {
	s, _ := newTestServer(t)
	enableSQLConsole(t, s)
	rec := postSQL(t, s, "INSERT INTO links (code, url, title, created_at, updated_at) VALUES ('x1', 'https://x.com', '', 1, 2); INSERT INTO links (code, url, title, created_at, updated_at) VALUES ('x2', 'https://y.com', '', 1, 2); SELECT nope")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body %s", rec.Code, rec.Body.String())
	}
	var n int
	if err := s.store.SQL().QueryRow(`SELECT COUNT(*) FROM links WHERE code IN ('x1','x2')`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Errorf("rolled-back batch left %d rows, want 0", n)
	}
}

// TestSQLConsoleCommitAndCacheInvalidation: a committed write is visible and
// drops the GetLink cache — a cached entry taken before the SQL write must
// not survive it.
func TestSQLConsoleCommitAndCacheInvalidation(t *testing.T) {
	s, _ := newTestServer(t)
	enableSQLConsole(t, s)
	createLink(t, s, "old", "https://before.com")
	// Populate the cache and learn the row id from a fresh read.
	got0, err := s.store.GetLink(context.Background(), "old")
	if err != nil {
		t.Fatal(err)
	}
	rec := postSQL(t, s, "UPDATE links SET url = 'https://after.com' WHERE id = "+itoa64(got0.ID))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body %s", rec.Code, rec.Body.String())
	}
	got, err := s.store.GetLink(context.Background(), "old")
	if err != nil {
		t.Fatal(err)
	}
	if got.URL != "https://after.com" {
		t.Errorf("cached url = %q, want https://after.com — cache was not invalidated", got.URL)
	}
}

// TestSQLConsoleForbiddenStatements: DDL, transaction control and writable
// pragmas are refused with stable codes; nothing executes.
func TestSQLConsoleForbiddenStatements(t *testing.T) {
	cases := []struct{ name, sql string }{
		{"create table", "CREATE TABLE evil (id INTEGER)"},
		{"drop table", "DROP TABLE links"},
		{"alter table", "ALTER TABLE links ADD COLUMN x"},
		{"attach", "ATTACH DATABASE 'file:x' AS x"},
		{"begin", "BEGIN"},
		{"commit", "COMMIT"},
		{"vacuum", "VACUUM"},
		{"writable pragma", "PRAGMA journal_mode = DELETE"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s, _ := newTestServer(t)
			enableSQLConsole(t, s)
			rec := postSQL(t, s, tc.sql)
			if rec.Code != http.StatusForbidden {
				t.Fatalf("status = %d, want 403, body %s", rec.Code, rec.Body.String())
			}
			var errResp errorBody
			if err := json.NewDecoder(rec.Body).Decode(&errResp); err != nil {
				t.Fatal(err)
			}
			if errResp.Error.Code != "statement_forbidden" {
				t.Errorf("error code = %q, want statement_forbidden", errResp.Error.Code)
			}
		})
	}
}

// TestSQLConsoleReadOnlyPragmaAllowed: reporting pragmas pass.
func TestSQLConsoleReadOnlyPragmaAllowed(t *testing.T) {
	s, _ := newTestServer(t)
	enableSQLConsole(t, s)
	for _, sql := range []string{"PRAGMA integrity_check", "pragma table_info(links)"} {
		rec := postSQL(t, s, sql)
		if rec.Code != http.StatusOK {
			t.Errorf("%s: status = %d, body %s", sql, rec.Code, rec.Body.String())
		}
	}
}

// TestSQLConsoleLimits: empty input, oversized bodies, too many statements.
func TestSQLConsoleLimits(t *testing.T) {
	s, _ := newTestServer(t)
	enableSQLConsole(t, s)

	if rec := postSQL(t, s, "   "); rec.Code != http.StatusBadRequest {
		t.Errorf("empty script status = %d, want 400", rec.Code)
	}
	if rec := postSQL(t, s, strings.Repeat("SELECT 1;", maxSQLStatements+1)); rec.Code != http.StatusBadRequest {
		t.Errorf("statement flood status = %d, want 400", rec.Code)
	}

	huge := `{"sql":"` + strings.Repeat("a", maxSQLBodyBytes) + `"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/db", strings.NewReader(huge))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(testSession)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("oversized body status = %d, want 400", rec.Code)
	}
}

// TestSQLConsoleRowsAffected: DML reports its affected-row count through the
// Exec path (the query path returns an empty set with 0 on modernc).
func TestSQLConsoleRowsAffected(t *testing.T) {
	s, _ := newTestServer(t)
	enableSQLConsole(t, s)
	createLink(t, s, "ra1", "https://ra.com")
	rec := postSQL(t, s, "UPDATE links SET url = 'https://ra2.com' WHERE code = 'ra1'; DELETE FROM links WHERE code = 'ra1'")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body %s", rec.Code, rec.Body.String())
	}
	resp := decodeConsoleResponse(t, rec)
	if len(resp.Results) != 2 {
		t.Fatalf("results = %d, want 2", len(resp.Results))
	}
	if resp.Results[0].RowsAffected != 1 || resp.Results[1].RowsAffected != 1 {
		t.Errorf("rows_affected = %d/%d, want 1/1", resp.Results[0].RowsAffected, resp.Results[1].RowsAffected)
	}
}

// TestSplitSQLStatements: semicolons inside strings and comments do not
// split; trailing semicolons and empties are dropped.
func TestSplitSQLStatements(t *testing.T) {
	cases := []struct {
		name, in string
		want     []string
	}{
		{"simple", "SELECT 1; SELECT 2;", []string{"SELECT 1", "SELECT 2"}},
		{"semicolon in string", "INSERT INTO t VALUES ('a;b'); SELECT 2", []string{"INSERT INTO t VALUES ('a;b')", "SELECT 2"}},
		{"escaped quote", "INSERT INTO t VALUES ('it''s; fine')", []string{"INSERT INTO t VALUES ('it''s; fine')"}},
		{"line comment", "SELECT 1 -- hi; there\n; SELECT 2", []string{"SELECT 1 -- hi; there", "SELECT 2"}},
		{"block comment", "SELECT /* a;b;c */ 1; SELECT 2", []string{"SELECT /* a;b;c */ 1", "SELECT 2"}},
		{"empty parts", "; ; ;", nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := splitSQLStatements(tc.in)
			if len(got) != len(tc.want) {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Errorf("stmt[%d] = %q, want %q", i, got[i], tc.want[i])
				}
			}
		})
	}
}

// TestSQLConsoleSetupModeStillLocked: setup_required wins over the console,
// matching every other management endpoint.
func TestSQLConsoleSetupModeStillLocked(t *testing.T) {
	s, _ := newTestServer(t)
	enterSetupMode(t, s)
	enableSQLConsole(t, s)
	rec := postSQL(t, s, "SELECT 1")
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403 setup_required", rec.Code)
	}
	var errResp errorBody
	if err := json.NewDecoder(rec.Body).Decode(&errResp); err != nil {
		t.Fatal(err)
	}
	if errResp.Error.Code != "setup_required" {
		t.Errorf("error code = %q, want setup_required", errResp.Error.Code)
	}
}

// TestInvalidateCacheContract pins the store-side helper used by the console.
func TestInvalidateCacheContract(t *testing.T) {
	dir := t.TempDir()
	st, err := store.Open(dir + "/test.db")
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	l := &store.Link{Code: "cc", URL: "https://c.com", CreatedAt: 1, UpdatedAt: 1}
	if err := st.CreateLink(context.Background(), l); err != nil {
		t.Fatal(err)
	}
	if _, err := st.GetLink(context.Background(), "cc"); err != nil {
		t.Fatal(err)
	}
	st.InvalidateCache()
	if _, err := st.GetLink(context.Background(), "cc"); err != nil {
		t.Errorf("get after invalidate: %v", err)
	}
}

func itoa64(n int64) string {
	return fmt.Sprintf("%d", n)
}
