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
	if _, err := s.CreateUABlock(ctx, "Googlebot", 1); err != nil {
		t.Fatalf("CreateUABlock: %v", err)
	}
	if _, err := s.CreateUABlock(ctx, "curl", 2); err != nil {
		t.Fatalf("CreateUABlock: %v", err)
	}
	blocks, err := s.ListUABlocks(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(blocks) != 2 || blocks[0].Pattern != "Googlebot" || blocks[1].Pattern != "curl" {
		t.Errorf("blocks = %v", blocks)
	}

	// Duplicate pattern.
	if _, err := s.CreateUABlock(ctx, "curl", 3); !errors.Is(err, ErrTaken) {
		t.Errorf("duplicate: %v, want ErrTaken", err)
	}

	// Delete by id 1.
	if err := s.DeleteUABlock(ctx, 1); err != nil {
		t.Fatalf("DeleteUABlock: %v", err)
	}
	blocks, _ = s.ListUABlocks(ctx)
	if len(blocks) != 1 || blocks[0].Pattern != "curl" {
		t.Errorf("after delete: %v", blocks)
	}
	if err := s.DeleteUABlock(ctx, 999); !errors.Is(err, ErrNotFound) {
		t.Errorf("delete missing: %v, want ErrNotFound", err)
	}
}
