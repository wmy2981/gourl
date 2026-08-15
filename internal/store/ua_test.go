package store

import (
	"context"
	"errors"
	"testing"
)

func TestUABlockCRUD(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	if _, err := s.ListUABlocks(ctx); err != nil {
		t.Fatalf("ListUABlocks on empty: %v", err)
	}
	if err := s.CreateUABlock(ctx, "Googlebot", 1); err != nil {
		t.Fatalf("CreateUABlock: %v", err)
	}
	if err := s.CreateUABlock(ctx, "curl", 2); err != nil {
		t.Fatalf("CreateUABlock: %v", err)
	}
	patterns, err := s.ListUABlocks(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(patterns) != 2 || patterns[0] != "Googlebot" || patterns[1] != "curl" {
		t.Errorf("patterns = %v", patterns)
	}

	// Duplicate pattern.
	if err := s.CreateUABlock(ctx, "curl", 3); !errors.Is(err, ErrTaken) {
		t.Errorf("duplicate: %v, want ErrTaken", err)
	}

	// Delete by id 1.
	if err := s.DeleteUABlock(ctx, 1); err != nil {
		t.Fatalf("DeleteUABlock: %v", err)
	}
	patterns, _ = s.ListUABlocks(ctx)
	if len(patterns) != 1 || patterns[0] != "curl" {
		t.Errorf("after delete: %v", patterns)
	}
	if err := s.DeleteUABlock(ctx, 999); !errors.Is(err, ErrNotFound) {
		t.Errorf("delete missing: %v, want ErrNotFound", err)
	}
}
