package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
)

// Token is an API token record.
type Token struct {
	ID        int64
	Token     string
	Note      string
	CreatedAt int64
}

// CreateToken stores a token, returning its id.
func (s *Store) CreateToken(ctx context.Context, token, note string, now int64) (int64, error) {
	res, err := s.db.ExecContext(ctx,
		`INSERT INTO api_tokens (token, note, created_at) VALUES (?, ?, ?)`, token, note, now)
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

// GetToken returns the token record. Soft-deleted tokens no longer match
// (revocation takes effect immediately). Returns ErrNotFound if absent.
func (s *Store) GetToken(ctx context.Context, token string) (*Token, error) {
	var t Token
	err := s.db.QueryRowContext(ctx,
		`SELECT id, token, note, created_at FROM api_tokens WHERE token = ? AND deleted = 0`, token).
		Scan(&t.ID, &t.Token, &t.Note, &t.CreatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get token: %w", err)
	}
	return &t, nil
}

// ListTokens returns all tokens, newest first. Soft-deleted tokens are
// excluded (the UI never shows them).
func (s *Store) ListTokens(ctx context.Context) ([]Token, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, token, note, created_at FROM api_tokens WHERE deleted = 0 ORDER BY id DESC`)
	if err != nil {
		return nil, fmt.Errorf("list tokens: %w", err)
	}
	defer rows.Close()
	var tokens []Token
	for rows.Next() {
		var t Token
		if err := rows.Scan(&t.ID, &t.Token, &t.Note, &t.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan token: %w", err)
		}
		tokens = append(tokens, t)
	}
	return tokens, rows.Err()
}

// DeleteToken soft-deletes a token (deleted = 1), keeping the row for
// exports. The key stays permanently taken: the UNIQUE constraint still
// applies, so a new token cannot reuse it. Returns ErrNotFound if absent.
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
