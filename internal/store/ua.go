package store

import (
	"context"
	"fmt"
)

// UABlock is a UA block pattern record.
type UABlock struct {
	ID        int64
	Pattern   string
	CreatedAt int64
}

// ListUABlocks returns all UA block patterns, oldest first.
func (s *Store) ListUABlocks(ctx context.Context) ([]UABlock, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, pattern, created_at FROM ua_blocks ORDER BY id`)
	if err != nil {
		return nil, fmt.Errorf("list ua blocks: %w", err)
	}
	defer rows.Close()
	var blocks []UABlock
	for rows.Next() {
		var b UABlock
		if err := rows.Scan(&b.ID, &b.Pattern, &b.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan ua block: %w", err)
		}
		blocks = append(blocks, b)
	}
	return blocks, rows.Err()
}

// CreateUABlock adds a pattern, returning its id. Returns ErrTaken on
// duplicates.
func (s *Store) CreateUABlock(ctx context.Context, pattern string, now int64) (int64, error) {
	res, err := s.db.ExecContext(ctx,
		`INSERT INTO ua_blocks (pattern, created_at) VALUES (?, ?)`, pattern, now)
	if isConstraint(err) {
		return 0, ErrTaken
	}
	if err != nil {
		return 0, fmt.Errorf("create ua block: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}
	return id, nil
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
