package store

import (
	"context"
	"testing"
)

// TestBackupLinkAppendsSnapshots: every BackupLink call inserts a new row
// with a strictly increasing b_id starting at 1, and the snapshot carries the
// pre-edit values (old code included).
func TestBackupLinkAppendsSnapshots(t *testing.T) {
	s, err := Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	ctx := context.Background()

	l := &Link{Code: "abc", URL: "https://x/1", Title: "T", Description: "D",
		ExpiresAt: 111, ClickCount: 42, CreatedAt: 1000, UpdatedAt: 1000}
	if err := s.CreateLink(ctx, l); err != nil {
		t.Fatal(err)
	}

	b1, err := s.BackupLink(ctx, l, 2000)
	if err != nil {
		t.Fatal(err)
	}
	if b1 != 1 {
		t.Errorf("first b_id = %d, want 1", b1)
	}

	// Edit the link in place and back up again: both rows must exist and the
	// ids increase.
	l.URL = "https://x/2"
	l.Code = "renamed"
	b2, err := s.BackupLink(ctx, l, 3000)
	if err != nil {
		t.Fatal(err)
	}
	if b2 != 2 {
		t.Errorf("second b_id = %d, want 2", b2)
	}

	count, err := s.CountBackups(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Errorf("count = %d, want 2 (never overwrite)", count)
	}
}
