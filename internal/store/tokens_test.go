package store

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestTokenLifecycle(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	id, err := s.CreateToken(ctx, "tok-abc", "ci", 1)
	if err != nil {
		t.Fatal(err)
	}
	if id <= 0 {
		t.Errorf("token id = %d, want positive", id)
	}

	got, err := s.GetToken(ctx, "tok-abc")
	if err != nil {
		t.Fatalf("GetToken: %v", err)
	}
	if got.Note != "ci" || got.ID != id {
		t.Errorf("unexpected token: %+v", got)
	}
	// The stored value is a bcrypt hash, never the plaintext; the prefix
	// keeps the UI preview readable.
	if got.Token == "tok-abc" || !strings.HasPrefix(got.Token, "$2a$") {
		t.Errorf("stored token = %q, want a bcrypt hash", got.Token)
	}
	if got.TokenPrefix != "tok-abc" {
		t.Errorf("token prefix = %q, want %q", got.TokenPrefix, "tok-abc")
	}

	if err := s.DeleteToken(ctx, id); err != nil {
		t.Fatalf("DeleteToken: %v", err)
	}
	if _, err := s.GetToken(ctx, "tok-abc"); !errors.Is(err, ErrNotFound) {
		t.Errorf("revoked token must not match, got %v", err)
	}
	// DeleteToken on a revoked token is ErrNotFound.
	if err := s.DeleteToken(ctx, id); !errors.Is(err, ErrNotFound) {
		t.Errorf("double delete = %v, want ErrNotFound", err)
	}
}

// RevokeAllTokens backs `gourl reset api`: every active token is soft-deleted
// in one pass, keys stay permanently taken, revoked ones are untouched.
func TestRevokeAllTokens(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	s.CreateToken(ctx, "tok-1", "one", 1)
	s.CreateToken(ctx, "tok-2", "two", 2)
	s.CreateToken(ctx, "tok-3", "three", 3)
	// Pre-revoke one to prove it is not double-counted.
	if err := s.DeleteToken(ctx, 2); err != nil {
		t.Fatal(err)
	}

	n, err := s.RevokeAllTokens(ctx)
	if err != nil {
		t.Fatalf("RevokeAllTokens: %v", err)
	}
	if n != 2 {
		t.Errorf("revoked = %d, want 2", n)
	}

	tokens, err := s.ListTokens(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(tokens) != 0 {
		t.Errorf("active tokens after reset = %d, want 0", len(tokens))
	}
	for _, tok := range []string{"tok-1", "tok-3"} {
		if _, err := s.GetToken(ctx, tok); !errors.Is(err, ErrNotFound) {
			t.Errorf("token %s must be revoked, got %v", tok, err)
		}
	}
	// Keys stay permanently taken: CreateToken's plaintext check (hashes
	// cannot share a UNIQUE constraint) refuses the revoked key.
	if _, err := s.CreateToken(ctx, "tok-1", "again", 4); !errors.Is(err, ErrTaken) {
		t.Errorf("reusing a revoked key = %v, want ErrTaken", err)
	}

	// A second pass revokes nothing.
	n, err = s.RevokeAllTokens(ctx)
	if err != nil || n != 0 {
		t.Errorf("second revoke = %d, %v; want 0, nil", n, err)
	}
}

// TestMigrateTokenHashes converts legacy plaintext rows (pre-bcrypt) in
// place; hashed rows are untouched and lookups keep working either way.
func TestMigrateTokenHashes(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	// A legacy plaintext row, as stored before the bcrypt switch.
	if _, err := s.db.ExecContext(ctx,
		`INSERT INTO api_tokens (token, note, created_at) VALUES ('legacy-plain-1', 'legacy', 1)`); err != nil {
		t.Fatal(err)
	}
	// A modern hashed row stays untouched.
	if _, err := s.CreateToken(ctx, "modern-tok", "modern", 2); err != nil {
		t.Fatal(err)
	}

	if err := s.MigrateTokenHashes(ctx); err != nil {
		t.Fatalf("MigrateTokenHashes: %v", err)
	}

	got, err := s.GetToken(ctx, "legacy-plain-1")
	if err != nil {
		t.Fatalf("legacy token must still match after migration: %v", err)
	}
	if got.Token == "legacy-plain-1" || !strings.HasPrefix(got.Token, "$2a$") {
		t.Errorf("migrated token = %q, want a bcrypt hash", got.Token)
	}
	if got.TokenPrefix != "legacy-p" {
		t.Errorf("migrated prefix = %q, want %q", got.TokenPrefix, "legacy-p")
	}

	// Idempotent: a second pass changes nothing and lookups still work.
	if err := s.MigrateTokenHashes(ctx); err != nil {
		t.Fatalf("second MigrateTokenHashes: %v", err)
	}
	if _, err := s.GetToken(ctx, "legacy-plain-1"); err != nil {
		t.Fatalf("legacy token must still match after a second migration: %v", err)
	}
}
