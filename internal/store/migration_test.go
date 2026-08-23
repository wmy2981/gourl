package store

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
)

// TestMigrationV3AssignsIDsInCreatedAtOrder upgrades a v2 database (links
// without id/deleted) and checks that existing links get ids in created_at
// order and fresh creates continue the sequence.
func TestMigrationV3AssignsIDsInCreatedAtOrder(t *testing.T) {
	path := filepath.Join(t.TempDir(), "old.db")
	db, err := sql.Open("sqlite", "file:"+path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`CREATE TABLE schema_version (version INTEGER NOT NULL)`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(migrations[0]); err != nil { // v1
		t.Fatalf("v1: %v", err)
	}
	if _, err := db.Exec(migrations[1]); err != nil { // v2
		t.Fatalf("v2: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO schema_version (version) VALUES (2)`); err != nil {
		t.Fatal(err)
	}
	// Insert in reverse created_at order to prove ordering, not insertion.
	if _, err := db.Exec(
		`INSERT INTO links (code, url, title, description, expires_at, click_count, created_at, updated_at)
		 VALUES ('newer', 'https://x/1', '', '', 0, 0, 2000, 2000)`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(
		`INSERT INTO links (code, url, title, description, expires_at, click_count, created_at, updated_at)
		 VALUES ('older', 'https://x/2', '', '', 0, 0, 1000, 1000)`); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	s, err := Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer s.Close()
	ctx := context.Background()

	older, err := s.GetLink(ctx, "older")
	if err != nil {
		t.Fatalf("get older: %v", err)
	}
	if older.ID != 1 {
		t.Errorf("older id = %d, want 1 (created_at first)", older.ID)
	}
	newer, err := s.GetLink(ctx, "newer")
	if err != nil {
		t.Fatalf("get newer: %v", err)
	}
	if newer.ID != 2 {
		t.Errorf("newer id = %d, want 2", newer.ID)
	}

	// A fresh create continues from the migrated max.
	third := &Link{Code: "third", URL: "https://x/3", CreatedAt: 3000, UpdatedAt: 3000}
	if err := s.CreateLink(ctx, third); err != nil {
		t.Fatalf("create: %v", err)
	}
	if third.ID != 3 {
		t.Errorf("fresh id = %d, want 3", third.ID)
	}
}

// TestMigrationV5OrphansDailyClicks: daily rows whose link was hard-deleted
// before v5 migrate with a NULL link_id and keep feeding the global totals.
func TestMigrationV5OrphansDailyClicks(t *testing.T) {
	path := filepath.Join(t.TempDir(), "old.db")
	db, err := sql.Open("sqlite", "file:"+path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`CREATE TABLE schema_version (version INTEGER NOT NULL)`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(migrations[0]); err != nil { // v1
		t.Fatal(err)
	}
	if _, err := db.Exec(migrations[1]); err != nil { // v2
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO schema_version (version) VALUES (2)`); err != nil {
		t.Fatal(err)
	}
	// One live link with clicks, one orphan's clicks (its link is gone).
	if _, err := db.Exec(
		`INSERT INTO links (code, url, title, description, expires_at, click_count, created_at, updated_at)
		 VALUES ('alive', 'https://x/1', '', '', 0, 0, 1000, 1000)`); err != nil {
		t.Fatal(err)
	}
	for _, d := range []string{"('alive', '2026-08-15', 4)", "('ghost', '2026-08-15', 6)"} {
		if _, err := db.Exec(`INSERT INTO daily_clicks (code, date, count) VALUES ` + d); err != nil {
			t.Fatal(err)
		}
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	s, err := Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer s.Close()
	ctx := context.Background()

	// The orphan row keeps a NULL link_id and the totals sum everything.
	var orphanLinkID *int64
	if err := s.db.QueryRowContext(ctx,
		`SELECT link_id FROM daily_clicks WHERE code = 'ghost'`).Scan(&orphanLinkID); err != nil {
		t.Fatal(err)
	}
	if orphanLinkID != nil {
		t.Errorf("orphan link_id = %v, want NULL", *orphanLinkID)
	}
	_, total, _, err := s.StatsOverview(ctx, "2026-08-01")
	if err != nil {
		t.Fatal(err)
	}
	if total != 10 {
		t.Errorf("global total = %d, want 10 (orphan history kept)", total)
	}
}

// TestMigrationV7DropsLegacyUABlocks: a v6 database still carries the dead
// ua_blocks table (UA blocks live in config.yaml since long); v7 drops it.
// A fresh database never has it in the first place.
func TestMigrationV7DropsLegacyUABlocks(t *testing.T) {
	path := filepath.Join(t.TempDir(), "old.db")
	db, err := sql.Open("sqlite", "file:"+path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`CREATE TABLE schema_version (version INTEGER NOT NULL)`); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 6; i++ {
		if _, err := db.Exec(migrations[i]); err != nil {
			t.Fatalf("v%d: %v", i+1, err)
		}
	}
	if _, err := db.Exec(`INSERT INTO schema_version (version) VALUES (6)`); err != nil {
		t.Fatal(err)
	}
	// The v1..v6 chain no longer creates ua_blocks; emulate the legacy table
	// a pre-v7 database would carry, with one row of stale data.
	if _, err := db.Exec(
		`CREATE TABLE ua_blocks (id INTEGER PRIMARY KEY AUTOINCREMENT, pattern TEXT NOT NULL UNIQUE, created_at INTEGER NOT NULL)`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO ua_blocks (pattern, created_at) VALUES ('Googlebot', 1)`); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	s, err := Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer s.Close()
	var n int
	if err := s.db.QueryRow(
		`SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = 'ua_blocks'`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Error("ua_blocks table survived migration v7")
	}

	// A fresh database goes straight to the final schema without the table.
	fresh, err := Open(filepath.Join(t.TempDir(), "new.db"))
	if err != nil {
		t.Fatalf("open fresh: %v", err)
	}
	defer fresh.Close()
	if err := fresh.db.QueryRow(
		`SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = 'ua_blocks'`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Error("fresh database created the legacy ua_blocks table")
	}
}
