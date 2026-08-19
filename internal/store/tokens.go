package store

import (
	"context"
	"fmt"
	"log/slog"

	"golang.org/x/crypto/bcrypt"
)

// Token is an API token record. Token holds the bcrypt hash; TokenPrefix is
// the first 8 characters of the plaintext, kept so the UI preview stays
// readable after hashing.
type Token struct {
	ID          int64
	Token       string
	TokenPrefix string
	Note        string
	CreatedAt   int64
}

// CreateToken stores a token as a bcrypt hash, returning its id. The
// plaintext is never persisted. Keys stay permanently taken: an identical
// plaintext already present (soft-deleted rows included) fails with
// ErrTaken, mirroring the old UNIQUE constraint on plaintext keys.
func (s *Store) CreateToken(ctx context.Context, token, note string, now int64) (int64, error) {
	// bcrypt hashes cannot be checked by SQL equality, so every row is
	// compared — token counts are tiny and creation is rare.
	rows, err := s.db.QueryContext(ctx, `SELECT token FROM api_tokens`)
	if err != nil {
		return 0, fmt.Errorf("create token: %w", err)
	}
	for rows.Next() {
		var stored string
		if err := rows.Scan(&stored); err != nil {
			rows.Close()
			return 0, fmt.Errorf("create token: %w", err)
		}
		if bcrypt.CompareHashAndPassword([]byte(stored), []byte(token)) == nil {
			rows.Close()
			return 0, ErrTaken
		}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return 0, fmt.Errorf("create token: %w", err)
	}
	// Close the read set before writing: the in-memory test store pins a
	// single connection, and SQLite cannot run a write while a query is
	// still open on the same connection.
	rows.Close()

	hash, err := bcrypt.GenerateFromPassword([]byte(token), bcrypt.DefaultCost)
	if err != nil {
		return 0, fmt.Errorf("create token: %w", err)
	}
	prefix := token
	if len(prefix) > 8 {
		prefix = prefix[:8]
	}
	res, err := s.db.ExecContext(ctx,
		`INSERT INTO api_tokens (token, token_prefix, note, created_at) VALUES (?, ?, ?, ?)`,
		string(hash), prefix, note, now)
	if err != nil {
		return 0, fmt.Errorf("create token: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}
	slog.Debug("store: token created", "id", id)
	return id, nil
}

// GetToken returns the token record matching the plaintext. bcrypt hashes
// cannot be matched by SQL equality, so every active row is compared (token
// counts are tiny). Soft-deleted tokens no longer match (revocation takes
// effect immediately). Returns ErrNotFound if absent.
func (s *Store) GetToken(ctx context.Context, token string) (*Token, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, token, token_prefix, note, created_at FROM api_tokens WHERE deleted = 0`)
	if err != nil {
		return nil, fmt.Errorf("get token: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var t Token
		if err := rows.Scan(&t.ID, &t.Token, &t.TokenPrefix, &t.Note, &t.CreatedAt); err != nil {
			return nil, fmt.Errorf("get token: %w", err)
		}
		if bcrypt.CompareHashAndPassword([]byte(t.Token), []byte(token)) == nil {
			return &t, nil
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("get token: %w", err)
	}
	return nil, ErrNotFound
}

// ListTokens returns all tokens, newest first. Soft-deleted tokens are
// excluded (the UI never shows them).
func (s *Store) ListTokens(ctx context.Context) ([]Token, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, token, token_prefix, note, created_at FROM api_tokens WHERE deleted = 0 ORDER BY id DESC`)
	if err != nil {
		return nil, fmt.Errorf("list tokens: %w", err)
	}
	defer rows.Close()
	var tokens []Token
	for rows.Next() {
		var t Token
		if err := rows.Scan(&t.ID, &t.Token, &t.TokenPrefix, &t.Note, &t.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan token: %w", err)
		}
		tokens = append(tokens, t)
	}
	return tokens, rows.Err()
}

// MigrateTokenHashes converts legacy plaintext token rows (stored before the
// bcrypt switch) in place: each gets a bcrypt hash plus the plaintext prefix
// used by the UI preview. Already-hashed rows are left alone; the query is
// idempotent. Called once at startup, right after migrations.
func (s *Store) MigrateTokenHashes(ctx context.Context) error {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, token FROM api_tokens
		 WHERE token NOT LIKE '$2a$%' AND token NOT LIKE '$2b$%' AND token NOT LIKE '$2c$%'`)
	if err != nil {
		return fmt.Errorf("migrate token hashes: %w", err)
	}
	type legacy struct {
		id    int64
		plain string
	}
	var pending []legacy
	for rows.Next() {
		var l legacy
		if err := rows.Scan(&l.id, &l.plain); err != nil {
			rows.Close()
			return fmt.Errorf("migrate token hashes: %w", err)
		}
		pending = append(pending, l)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return fmt.Errorf("migrate token hashes: %w", err)
	}
	// Close the read set before writing (see CreateToken — the in-memory test
	// store pins a single connection, and SQLite cannot write over an open
	// query on it).
	rows.Close()

	for _, l := range pending {
		hash, err := bcrypt.GenerateFromPassword([]byte(l.plain), bcrypt.DefaultCost)
		if err != nil {
			return fmt.Errorf("migrate token hashes: %w", err)
		}
		prefix := l.plain
		if len(prefix) > 8 {
			prefix = prefix[:8]
		}
		if _, err := s.db.ExecContext(ctx,
			`UPDATE api_tokens SET token = ?, token_prefix = ? WHERE id = ?`,
			string(hash), prefix, l.id); err != nil {
			return fmt.Errorf("migrate token hashes: %w", err)
		}
		slog.Debug("store: token hash migrated", "id", l.id)
	}
	return nil
}

// RevokeAllTokens soft-deletes every active token (gourl reset api),
// returning how many were revoked. Same semantics as DeleteToken.
func (s *Store) RevokeAllTokens(ctx context.Context) (int64, error) {
	res, err := s.db.ExecContext(ctx,
		`UPDATE api_tokens SET deleted = 1 WHERE deleted = 0`)
	if err != nil {
		return 0, fmt.Errorf("revoke all tokens: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return 0, err
	}
	slog.Debug("store: all tokens revoked", "count", n)
	return n, nil
}

// DeleteToken soft-deletes a token (deleted = 1), keeping the row for
// exports. The key stays permanently taken: CreateToken's plaintext check
// still refuses it. Returns ErrNotFound if absent.
func (s *Store) DeleteToken(ctx context.Context, id int64) error {
	res, err := s.db.ExecContext(ctx,
		`UPDATE api_tokens SET deleted = 1 WHERE id = ? AND deleted = 0`, id)
	if err != nil {
		return fmt.Errorf("delete token: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		slog.Debug("store: delete token failed", "id", id, "error", ErrNotFound)
		return ErrNotFound
	}
	slog.Debug("store: token deleted", "id", id)
	return nil
}
