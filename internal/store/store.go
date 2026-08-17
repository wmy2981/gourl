// Package store persists links and related records (daily clicks, UA blocks,
// API tokens) in SQLite. Business configuration lives in config.yaml, not here.
package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

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
	Query    string // substring match on code/url/title/description
	Page     int    // 1-based
	PageSize int    // default 20, max 100
	Sort     string // created_at (default) | clicks | code
	Order    string // desc (default) | asc
	// Expires filters by expiry state: "" or "all" = no filter, "expired" =
	// past expiry, "active" = never expires or still valid. Now is the clock
	// used for the comparison (injectable for tests).
	Expires string
	Now     int64
}

// Store wraps the SQLite connection plus the in-memory link lookup cache.
type Store struct {
	db    *sql.DB
	cache *linkCache
}

// Open opens (creating if needed) the SQLite database at path and runs
// migrations. Pass ":memory:" for tests.
func Open(path string) (*Store, error) {
	dsn := "file:" + path + "?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	if path == ":memory:" {
		// An in-memory database lives per-connection; pin a single connection
		// so every goroutine (e.g. the async meta workers) sees the same data.
		db.SetMaxOpenConns(1)
	}
	s := &Store{db: db, cache: newLinkCache()}
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
	// v2: query indexes for list ordering, expiry cleanup and daily stats.
	`CREATE INDEX IF NOT EXISTS idx_links_created_at ON links(created_at);
	CREATE INDEX IF NOT EXISTS idx_links_expires_at ON links(expires_at);
	CREATE INDEX IF NOT EXISTS idx_daily_clicks_date ON daily_clicks(date);`,
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
	s.cache.set(l.Code, l)
	return nil
}

// CreateLinks inserts many links in a single transaction (batch imports).
// The returned slice mirrors CreateLink's semantics per item: ErrTaken for
// codes that already exist. A non-constraint error aborts the whole batch.
func (s *Store) CreateLinks(ctx context.Context, links []Link) ([]error, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	errs := make([]error, len(links))
	for i := range links {
		_, err := tx.ExecContext(ctx,
			`INSERT INTO links (`+linkColumns+`) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			links[i].Code, links[i].URL, links[i].Title, links[i].Description,
			links[i].ExpiresAt, links[i].ClickCount, links[i].CreatedAt, links[i].UpdatedAt)
		if isConstraint(err) {
			errs[i] = ErrTaken
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("create links: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	for i, err := range errs {
		if err == nil {
			s.cache.set(links[i].Code, &links[i])
		}
	}
	return errs, nil
}

// UpdateMeta refreshes a link's title/description after an async meta fetch.
func (s *Store) UpdateMeta(ctx context.Context, code, title, description string, updatedAt int64) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE links SET title = ?, description = ?, updated_at = ? WHERE code = ?`,
		title, description, updatedAt, code)
	if err != nil {
		return fmt.Errorf("update meta: %w", err)
	}
	s.cache.del(code)
	return nil
}

// GetLink fetches a link by code, returning ErrNotFound if absent. Results
// are served from the in-memory cache when fresh; every write invalidates the
// affected entry.
func (s *Store) GetLink(ctx context.Context, code string) (*Link, error) {
	if l := s.cache.get(code); l != nil {
		return l, nil
	}
	row := s.db.QueryRowContext(ctx, `SELECT `+linkColumns+` FROM links WHERE code = ?`, code)
	l, err := scanLink(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get link: %w", err)
	}
	s.cache.set(code, l)
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
	s.cache.del(l.Code)
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
	if err := tx.Commit(); err != nil {
		return err
	}
	s.cache.del(oldCode)
	s.cache.del(newCode)
	return nil
}

// DeleteLink removes a link row. Its daily click records are deliberately
// kept: the dashboard totals and trend chart count history even for links
// that no longer exist. Returns ErrNotFound if the code was absent.
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
	if err := tx.Commit(); err != nil {
		return err
	}
	s.cache.del(code)
	return nil
}

// DeleteLinks removes many links in one transaction, returning how many rows
// were actually deleted (absent codes are simply skipped). Daily click
// records are deliberately kept, as in DeleteLink.
func (s *Store) DeleteLinks(ctx context.Context, codes []string) (int64, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	var deleted int64
	for _, code := range codes {
		res, err := tx.ExecContext(ctx, `DELETE FROM links WHERE code = ?`, code)
		if err != nil {
			return 0, fmt.Errorf("delete links: %w", err)
		}
		n, err := res.RowsAffected()
		if err != nil {
			return 0, err
		}
		deleted += n
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	for _, code := range codes {
		s.cache.del(code)
	}
	return deleted, nil
}

// CountExpired returns the number of links past their expiry (expires_at in
// the past, never counting 0 = never expires).
func (s *Store) CountExpired(ctx context.Context, now int64) (int64, error) {
	var n int64
	err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM links WHERE expires_at > 0 AND expires_at < ?`, now).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("count expired: %w", err)
	}
	return n, nil
}

// DeleteExpired removes every expired link in one transaction and returns
// how many were deleted.
func (s *Store) DeleteExpired(ctx context.Context, now int64) (int64, error) {
	res, err := s.db.ExecContext(ctx,
		`DELETE FROM links WHERE expires_at > 0 AND expires_at < ?`, now)
	if err != nil {
		return 0, fmt.Errorf("delete expired: %w", err)
	}
	// The sweep touches unknown codes; drop the whole cache to be safe.
	s.cache.clear()
	return res.RowsAffected()
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

	var conds []string
	var args []any
	if opts.Query != "" {
		// Parens keep AND-joined expiry filters from binding inside the ORs.
		conds = append(conds, `(code LIKE ? OR url LIKE ? OR title LIKE ? OR description LIKE ?)`)
		like := "%" + opts.Query + "%"
		args = []any{like, like, like, like}
	}
	switch opts.Expires {
	case "expired":
		conds = append(conds, `expires_at > 0 AND expires_at < ?`)
		args = append(args, opts.Now)
	case "active":
		conds = append(conds, `(expires_at = 0 OR expires_at >= ?)`)
		args = append(args, opts.Now)
	}
	where := ""
	if len(conds) > 0 {
		where = ` WHERE ` + strings.Join(conds, ` AND `)
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
