package store

import (
	"context"
	"fmt"
)

// ListUABlocks returns all UA block patterns, oldest first.
func (s *Store) ListUABlocks(ctx context.Context) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT pattern FROM ua_blocks ORDER BY id`)
	if err != nil {
		return nil, fmt.Errorf("list ua blocks: %w", err)
	}
	defer rows.Close()
	var patterns []string
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err != nil {
			return nil, fmt.Errorf("scan ua block: %w", err)
		}
		patterns = append(patterns, p)
	}
	return patterns, rows.Err()
}

// CreateUABlock adds a pattern. Returns ErrTaken on duplicates.
func (s *Store) CreateUABlock(ctx context.Context, pattern string, now int64) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO ua_blocks (pattern, created_at) VALUES (?, ?)`, pattern, now)
	if isConstraint(err) {
		return ErrTaken
	}
	if err != nil {
		return fmt.Errorf("create ua block: %w", err)
	}
	return nil
}

// DeleteUABlock removes a pattern by id. Returns ErrNotFound if absent.
func (s *Store) DeleteUABlock(ctx context.Context, id int64) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM ua_blocks WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete ua block: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}
