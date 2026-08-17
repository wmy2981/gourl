package store

import (
	"context"
	"fmt"
	"log/slog"
)

// Backup is one immutable snapshot of a link, taken before an edit. b_id is
// the global 1-based counter exposed as "b-1, b-2, …" by exports; link_id is
// the edited link's id, code its code at snapshot time (renames keep the old
// code here).
type Backup struct {
	BID         int64
	LinkID      int64
	Code        string
	URL         string
	Title       string
	Description string
	ExpiresAt   int64
	ClickCount  int64
	CreatedAt   int64
	UpdatedAt   int64
	BackedAt    int64
}

// BackupLink appends a snapshot of l (called before mutating it). Every call
// inserts a new row — snapshots are never overwritten. Returns the new b_id.
func (s *Store) BackupLink(ctx context.Context, l *Link, backedAt int64) (int64, error) {
	var bID int64
	if err := s.db.QueryRowContext(ctx, `SELECT COALESCE(MAX(b_id), 0) + 1 FROM backups`).Scan(&bID); err != nil {
		return 0, fmt.Errorf("next backup id: %w", err)
	}
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO backups (b_id, link_id, code, url, title, description, expires_at, click_count, created_at, updated_at, backed_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		bID, l.ID, l.Code, l.URL, l.Title, l.Description, l.ExpiresAt,
		l.ClickCount, l.CreatedAt, l.UpdatedAt, backedAt)
	if err != nil {
		slog.Debug("store: backup link failed", "code", l.Code, "error", err)
		return 0, fmt.Errorf("backup link: %w", err)
	}
	slog.Debug("store: link backed up", "code", l.Code, "link_id", l.ID, "b_id", bID)
	return bID, nil
}

// CountBackups returns the number of stored snapshots.
func (s *Store) CountBackups(ctx context.Context) (int64, error) {
	var n int64
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM backups`).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("count backups: %w", err)
	}
	return n, nil
}
