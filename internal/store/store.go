// Package store persists links and related records (daily clicks, UA blocks,
// API tokens) in SQLite. Business configuration lives in config.yaml, not here.
package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	sqlite3 "modernc.org/sqlite"
	sqlite3lib "modernc.org/sqlite/lib"
)

// Sentinel errors usable with errors.Is.
var (
	ErrNotFound = errors.New("not found")
	ErrTaken    = errors.New("code already taken")
)

// Link is a short link record.
type Link struct {
	Code        string
	URL         string
	Title       string
	Description string
	ExpiresAt   int64 // unix seconds, 0 = never expires
	ClickCount  int64
	CreatedAt   int64
	UpdatedAt   int64
}

// ListOptions controls listing.
type ListOptions struct {
	Query    string // substring match on code/url/title
	Page     int    // 1-based
	PageSize int    // default 20, max 100
	Sort     string // created_at (default) | clicks | code
	Order    string // desc (default) | asc
}

// Store wraps the SQLite connection.
type Store struct {
	db *sql.DB
}

// Open opens (creating if needed) the SQLite database at path and runs
// migrations. Pass ":memory:" for tests.
func Open(path string) (*Store, error) {
	dsn := "file:" + path + "?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	s := &Store{db: db}
	if err := s.migrate(context.Background()); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}

// Close closes the database.
func (s *Store) Close() error { return s.db.Close() }

// Ping verifies database connectivity.
func (s *Store) Ping(ctx context.Context) error { return s.db.PingContext(ctx) }

// migrations is an ordered list of schema migrations; index i applies after
// version i.
var migrations = []string{
	// v1: initial schema.
	`CREATE TABLE links (
		code        TEXT PRIMARY KEY,
		url         TEXT NOT NULL,
		title       TEXT NOT NULL DEFAULT '',
		description TEXT NOT NULL DEFAULT '',
		expires_at  INTEGER NOT NULL DEFAULT 0,
		click_count INTEGER NOT NULL DEFAULT 0,
		created_at  INTEGER NOT NULL,
		updated_at  INTEGER NOT NULL
	);
	CREATE TABLE daily_clicks (
		code  TEXT NOT NULL,
		date  TEXT NOT NULL,
		count INTEGER NOT NULL DEFAULT 0,
		PRIMARY KEY (code, date)
	);
	CREATE TABLE ua_blocks (
		id         INTEGER PRIMARY KEY AUTOINCREMENT,
		pattern    TEXT NOT NULL UNIQUE,
		created_at INTEGER NOT NULL
	);
	CREATE TABLE api_tokens (
		id         INTEGER PRIMARY KEY AUTOINCREMENT,
		token      TEXT NOT NULL UNIQUE,
		note       TEXT NOT NULL DEFAULT '',
		created_at INTEGER NOT NULL
	);`,
}

// migrate applies pending migrations inside a transaction each, recording the
// applied version in schema_version.
func (s *Store) migrate(ctx context.Context) error {
	if _, err := s.db.ExecContext(ctx,
		`CREATE TABLE IF NOT EXISTS schema_version (version INTEGER NOT NULL)`); err != nil {
		return fmt.Errorf("create schema_version: %w", err)
	}
	var current int
	err := s.db.QueryRowContext(ctx, `SELECT COALESCE(MAX(version), 0) FROM schema_version`).Scan(&current)
	if err != nil {
		return fmt.Errorf("read schema version: %w", err)
	}
	for i := current; i < len(migrations); i++ {
		tx, err := s.db.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, migrations[i]); err != nil {
			tx.Rollback()
			return fmt.Errorf("apply migration %d: %w", i+1, err)
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO schema_version (version) VALUES (?)`, i+1); err != nil {
			tx.Rollback()
			return fmt.Errorf("record migration %d: %w", i+1, err)
		}
		if err := tx.Commit(); err != nil {
			return err
		}
	}
	return nil
}

const linkColumns = `code, url, title, description, expires_at, click_count, created_at, updated_at`

func scanLink(row interface{ Scan(...any) error }) (*Link, error) {
	var l Link
	if err := row.Scan(&l.Code, &l.URL, &l.Title, &l.Description,
		&l.ExpiresAt, &l.ClickCount, &l.CreatedAt, &l.UpdatedAt); err != nil {
		return nil, err
	}
	return &l, nil
}

// CreateLink inserts a link. Returns ErrTaken if the code already exists.
func (s *Store) CreateLink(ctx context.Context, l *Link) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO links (`+linkColumns+`) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		l.Code, l.URL, l.Title, l.Description, l.ExpiresAt, l.ClickCount, l.CreatedAt, l.UpdatedAt)
	if isConstraint(err) {
		return ErrTaken
	}
	if err != nil {
		return fmt.Errorf("create link: %w", err)
	}
	return nil
}

// GetLink fetches a link by code, returning ErrNotFound if absent.
func (s *Store) GetLink(ctx context.Context, code string) (*Link, error) {
	row := s.db.QueryRowContext(ctx, `SELECT `+linkColumns+` FROM links WHERE code = ?`, code)
	l, err := scanLink(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get link: %w", err)
	}
	return l, nil
}

// UpdateLink updates mutable fields of a link. Returns ErrNotFound if absent.
// The code is not changeable through this method.
func (s *Store) UpdateLink(ctx context.Context, l *Link) error {
	res, err := s.db.ExecContext(ctx,
		`UPDATE links SET url = ?, title = ?, description = ?, expires_at = ?, updated_at = ? WHERE code = ?`,
		l.URL, l.Title, l.Description, l.ExpiresAt, l.UpdatedAt, l.Code)
	if err != nil {
		return fmt.Errorf("update link: %w", err)
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

// RenameLink changes a link's code. Returns ErrNotFound or ErrTaken.
func (s *Store) RenameLink(ctx context.Context, oldCode, newCode string, now int64) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	res, err := tx.ExecContext(ctx,
		`UPDATE links SET code = ?, updated_at = ? WHERE code = ?`, newCode, now, oldCode)
	if err != nil {
		if isConstraint(err) {
			return ErrTaken
		}
		return fmt.Errorf("rename link: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	if _, err := tx.ExecContext(ctx, `UPDATE daily_clicks SET code = ? WHERE code = ?`, newCode, oldCode); err != nil {
		return fmt.Errorf("rename daily clicks: %w", err)
	}
	return tx.Commit()
}

// DeleteLink removes a link and its daily click records. Returns ErrNotFound
// if the code was absent.
func (s *Store) DeleteLink(ctx context.Context, code string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	res, err := tx.ExecContext(ctx, `DELETE FROM links WHERE code = ?`, code)
	if err != nil {
		return fmt.Errorf("delete link: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM daily_clicks WHERE code = ?`, code); err != nil {
		return fmt.Errorf("delete daily clicks: %w", err)
	}
	return tx.Commit()
}

// ListLinks returns links matching the options plus the total count of matches.
func (s *Store) ListLinks(ctx context.Context, opts ListOptions) ([]Link, int, error) {
	if opts.Page < 1 {
		opts.Page = 1
	}
	if opts.PageSize < 1 {
		opts.PageSize = 20
	}
	if opts.PageSize > 100 {
		opts.PageSize = 100
	}
	sortCol := map[string]string{"created_at": "created_at", "clicks": "click_count", "code": "code"}[opts.Sort]
	if sortCol == "" {
		sortCol = "created_at"
	}
	order := "DESC"
	if opts.Order == "asc" {
		order = "ASC"
	}

	var where string
	var args []any
	if opts.Query != "" {
		where = ` WHERE code LIKE ? OR url LIKE ? OR title LIKE ?`
		like := "%" + opts.Query + "%"
		args = []any{like, like, like}
	}

	var total int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM links`+where, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count links: %w", err)
	}

	rows, err := s.db.QueryContext(ctx,
		`SELECT `+linkColumns+` FROM links`+where+` ORDER BY `+sortCol+` `+order+`, rowid `+order+` LIMIT ? OFFSET ?`,
		append(args, opts.PageSize, (opts.Page-1)*opts.PageSize)...)
	if err != nil {
		return nil, 0, fmt.Errorf("list links: %w", err)
	}
	defer rows.Close()

	var links []Link
	for rows.Next() {
		l, err := scanLink(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("scan link: %w", err)
		}
		links = append(links, *l)
	}
	return links, total, rows.Err()
}

// ListAllLinks returns every link, newest first (no pagination; used by
// exports).
func (s *Store) ListAllLinks(ctx context.Context) ([]Link, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT `+linkColumns+` FROM links ORDER BY created_at DESC, rowid DESC`)
	if err != nil {
		return nil, fmt.Errorf("list all links: %w", err)
	}
	defer rows.Close()
	var links []Link
	for rows.Next() {
		l, err := scanLink(rows)
		if err != nil {
			return nil, fmt.Errorf("scan link: %w", err)
		}
		links = append(links, *l)
	}
	return links, rows.Err()
}

// isConstraint reports whether err is a SQLite uniqueness constraint violation
// (primary key or unique index).
func isConstraint(err error) bool {
	var se *sqlite3.Error
	if !errors.As(err, &se) {
		return false
	}
	return se.Code() == sqlite3lib.SQLITE_CONSTRAINT_PRIMARYKEY ||
		se.Code() == sqlite3lib.SQLITE_CONSTRAINT_UNIQUE
}
