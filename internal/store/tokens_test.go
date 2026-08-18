package store

import (
	"context"
	"errors"
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
	// Keys stay permanently taken: the UNIQUE constraint still applies, so a
	// new token cannot reuse the revoked key.
	if _, err := s.CreateToken(ctx, "tok-1", "again", 4); err == nil {
		t.Error("reusing a revoked key should fail the UNIQUE constraint")
	}

	// A second pass revokes nothing.
	n, err = s.RevokeAllTokens(ctx)
	if err != nil || n != 0 {
		t.Errorf("second revoke = %d, %v; want 0, nil", n, err)
	}
}
