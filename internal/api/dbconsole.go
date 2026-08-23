package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// The SQL console endpoint (POST /api/v1/db) runs admin-authored SQL against
// the SQLite database inside a single transaction. It is gated on
// config.sql_console_enabled (default off, toggled only through the config
// file + `gourl reload`) and the standard requireAuth middleware.

const (
	maxSQLBodyBytes    = 1 << 20 // 1 MiB request body cap
	maxSQLStatements   = 50      // per-request statement cap
	maxLoggedSQLLength = 500     // statement text truncation in business logs
)

// dbConsoleRequest is the POST body.
type dbConsoleRequest struct {
	SQL string `json:"sql"`
}

// dbStatementResult is one entry of the per-statement results array.
type dbStatementResult struct {
	SQL          string   `json:"sql"`
	Columns      []string `json:"columns"`
	Rows         [][]any  `json:"rows"`
	RowsAffected int64    `json:"rows_affected"`
}

// dbConsoleResponse carries every statement's result plus the batch duration.
type dbConsoleResponse struct {
	Results    []dbStatementResult `json:"results"`
	DurationMS int64               `json:"duration_ms"`
}

// sqlForbidden lists the statement-leading keywords the console rejects:
// schema changes, transaction control (the endpoint wraps everything in its
// own transaction) and file/behavior-level statements. Matched
// case-insensitively on the first word.
var sqlForbidden = map[string]string{
	"create":    "schema changes are not allowed",
	"alter":     "schema changes are not allowed",
	"drop":      "schema changes are not allowed",
	"attach":    "attaching databases is not allowed",
	"detach":    "attaching databases is not allowed",
	"begin":     "transactions are managed by the endpoint",
	"commit":    "transactions are managed by the endpoint",
	"end":       "transactions are managed by the endpoint",
	"rollback":  "transactions are managed by the endpoint",
	"savepoint": "transactions are managed by the endpoint",
	"release":   "transactions are managed by the endpoint",
	"vacuum":    "VACUUM cannot run inside a transaction",
}

// readOnlyPragmas are the PRAGMA statements that only report state; every
// other PRAGMA can change database behavior and is refused.
var readOnlyPragmas = map[string]bool{
	"table_info":        true,
	"table_xinfo":       true,
	"index_list":        true,
	"index_info":        true,
	"table_list":        true,
	"database_list":     true,
	"schema_version":    true,
	"user_version":      true,
	"encoding":          true,
	"page_count":        true,
	"page_size":         true,
	"freelist_count":    true,
	"integrity_check":   true,
	"quick_check":       true,
	"foreign_key_check": true,
	"foreign_key_list":  true,
	"compile_options":   true,
	"function_list":     true,
	"module_list":       true,
	"pragma_list":       true,
	"journal_mode":      false, // reading it is fine but it is also writable — refuse to keep the rule simple
	"max_page_count":    false,
	"cache_size":        false,
	"busy_timeout":      false,
}

// splitSQLStatements splits a script into statements on semicolons that sit
// outside single/double-quoted strings and SQL comments (-- line and /* */
// block). Trailing empties are dropped.
func splitSQLStatements(script string) []string {
	var stmts []string
	var b strings.Builder
	inSingle, inDouble, inLine, inBlock := false, false, false, false
	for i := 0; i < len(script); i++ {
		c := script[i]
		switch {
		case inLine:
			if c == '\n' {
				inLine = false
			}
			b.WriteByte(c)
		case inBlock:
			if c == '*' && i+1 < len(script) && script[i+1] == '/' {
				b.WriteString("*/")
				i++
				inBlock = false
			} else {
				b.WriteByte(c)
			}
		case inSingle:
			b.WriteByte(c)
			if c == '\'' {
				if i+1 < len(script) && script[i+1] == '\'' {
					b.WriteByte('\'')
					i++
				} else {
					inSingle = false
				}
			}
		case inDouble:
			b.WriteByte(c)
			if c == '"' && (i+1 >= len(script) || script[i+1] != '"') {
				inDouble = false
			} else if c == '"' {
				b.WriteByte('"')
				i++
			}
		default:
			switch {
			case c == '\'':
				inSingle = true
				b.WriteByte(c)
			case c == '"':
				inDouble = true
				b.WriteByte(c)
			case c == '-' && i+1 < len(script) && script[i+1] == '-':
				inLine = true
				b.WriteString("--")
				i++
			case c == '/' && i+1 < len(script) && script[i+1] == '*':
				inBlock = true
				b.WriteString("/*")
				i++
			case c == ';':
				if s := strings.TrimSpace(b.String()); s != "" {
					stmts = append(stmts, s)
				}
				b.Reset()
			default:
				b.WriteByte(c)
			}
		}
	}
	if s := strings.TrimSpace(b.String()); s != "" {
		stmts = append(stmts, s)
	}
	return stmts
}

// firstKeyword returns the lowercased first word of the statement.
func firstKeyword(stmt string) string {
	s := strings.TrimLeft(strings.TrimSpace(stmt), " \t\r\n(")
	for i := 0; i < len(s); i++ {
		c := s[i]
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c == '_') {
			return strings.ToLower(s[:i])
		}
	}
	return strings.ToLower(s)
}

// validateStatement returns a stable error code + message for forbidden or
// malformed statements.
func validateStatement(stmt string) (code, message string) {
	kw := firstKeyword(stmt)
	if reason, bad := sqlForbidden[kw]; bad {
		return "statement_forbidden", fmt.Sprintf("%q is not allowed: %s", kw, reason)
	}
	if kw == "pragma" {
		name := pragmaName(stmt)
		if !readOnlyPragmas[name] {
			return "statement_forbidden", fmt.Sprintf("PRAGMA %q is not allowed (read-only pragmas only)", name)
		}
	}
	return "", ""
}

// pragmaName extracts the identifier right after PRAGMA.
func pragmaName(stmt string) string {
	s := strings.TrimSpace(stmt)
	s = s[len("pragma"):]
	s = strings.TrimLeft(s, " \t=()")
	var name strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '_' {
			name.WriteByte(c)
			continue
		}
		break
	}
	return strings.ToLower(name.String())
}

// hasWriteStatement reports whether any statement mutates data, deciding the
// cache invalidation after commit.
func hasWriteStatement(stmts []string) bool {
	for _, stmt := range stmts {
		switch firstKeyword(stmt) {
		case "insert", "update", "delete", "replace":
			return true
		}
	}
	return false
}

// dbConsole handles POST /api/v1/db.
func (s *Server) dbConsole(w http.ResponseWriter, r *http.Request) {
	if !s.cfg.Get().SQLConsoleEnabled {
		writeError(w, http.StatusForbidden, "sql_console_disabled", "the SQL console is disabled; enable sql_console_enabled in the config file and run `gourl reload`")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxSQLBodyBytes)
	var req dbConsoleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "invalid JSON body or body over 1 MiB")
		return
	}
	stmts := splitSQLStatements(req.SQL)
	if len(stmts) == 0 {
		writeError(w, http.StatusBadRequest, "invalid_request", "no SQL statement provided")
		return
	}
	if len(stmts) > maxSQLStatements {
		writeError(w, http.StatusBadRequest, "too_many_statements", fmt.Sprintf("at most %d statements per request, got %d", maxSQLStatements, len(stmts)))
		return
	}
	for _, stmt := range stmts {
		if code, msg := validateStatement(stmt); code != "" {
			logWarn(r, "sql console rejected a statement", "reason", msg)
			writeError(w, http.StatusForbidden, code, msg)
			return
		}
	}

	start := time.Now()
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	results, err := s.runSQLBatch(ctx, stmts)
	duration := time.Since(start).Milliseconds()
	if err != nil {
		if errors.Is(err, ctx.Err()) {
			writeError(w, http.StatusRequestTimeout, "sql_timeout", "the batch exceeded the 30s execution window and was rolled back")
			return
		}
		logWarn(r, "sql console batch failed and rolled back", "error", err.Error())
		writeError(w, http.StatusBadRequest, "sql_error", err.Error())
		return
	}

	if hasWriteStatement(stmts) {
		s.store.InvalidateCache()
	}
	logInfo(r, "sql console executed",
		"statements", len(stmts),
		"sql", truncateSQL(req.SQL),
		"duration_ms", duration,
	)
	writeJSON(w, http.StatusOK, dbConsoleResponse{Results: results, DurationMS: duration})
}

// runSQLBatch executes every statement inside one transaction: any failure
// rolls the whole batch back and surfaces the SQLite error message.
func (s *Server) runSQLBatch(ctx context.Context, stmts []string) ([]dbStatementResult, error) {
	db := s.store.SQL()
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback()

	results := make([]dbStatementResult, 0, len(stmts))
	for _, stmt := range stmts {
		res := dbStatementResult{SQL: stmt}
		rows, qErr := tx.QueryContext(ctx, stmt)
		if qErr != nil {
			// Statements that return no rows (INSERT/UPDATE/…) may surface as
			// query errors on some drivers; fall back to Exec semantics when
			// the error indicates no result set.
			if execRes, execErr := tx.ExecContext(ctx, stmt); execErr == nil {
				n, _ := execRes.RowsAffected()
				res.RowsAffected = n
				res.Columns = []string{}
				res.Rows = [][]any{}
				results = append(results, res)
				continue
			}
			return nil, qErr
		}
		cols, scanErr := collectRows(rows)
		rows.Close()
		if scanErr != nil {
			return nil, scanErr
		}
		res.Columns = cols.columns
		res.Rows = cols.rows
		res.RowsAffected = cols.rowsAffected
		results = append(results, res)
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}
	return results, nil
}

// rowSet is the collected form of a SELECT result.
type rowSet struct {
	columns      []string
	rows         [][]any
	rowsAffected int64
}

// collectRows drains a result set into JSON-friendly values.
func collectRows(rows *sql.Rows) (rowSet, error) {
	cols, err := rows.Columns()
	if err != nil {
		return rowSet{}, err
	}
	out := rowSet{columns: cols, rows: [][]any{}}
	for rows.Next() {
		vals := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return rowSet{}, err
		}
		for i, v := range vals {
			if b, ok := v.([]byte); ok {
				vals[i] = string(b)
			}
		}
		out.rows = append(out.rows, vals)
	}
	if err := rows.Err(); err != nil {
		return rowSet{}, err
	}
	return out, nil
}

// truncateSQL caps logged statement text so huge batches stay readable.
func truncateSQL(s string) string {
	if len(s) > maxLoggedSQLLength {
		return s[:maxLoggedSQLLength] + "…"
	}
	return s
}
