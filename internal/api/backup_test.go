package api

import (
	"context"
	"net/http"
	"testing"
)

// TestBackupOnEdits: manual edits, renames and batch-import updates each
// append a backup snapshot; a no-op PATCH backs nothing up.
func TestBackupOnEdits(t *testing.T) {
	s, _ := newTestServer(t)
	ctx := context.Background()

	if _, err := s.store.CountBackups(ctx); err != nil {
		t.Fatal(err)
	}

	// Create.
	rec := do(t, s, http.MethodPost, "/api/v1/links", map[string]any{
		"url": "https://example.com/a", "code": "abc",
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("create status = %d body %s", rec.Code, rec.Body.String())
	}

	// Plain edit backs up once.
	rec = do(t, s, http.MethodPatch, "/api/v1/links/abc", map[string]any{
		"url": "https://example.com/b",
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("edit status = %d body %s", rec.Code, rec.Body.String())
	}
	if n, _ := s.store.CountBackups(ctx); n != 1 {
		t.Fatalf("backups after edit = %d, want 1", n)
	}

	// Rename backs up the old code too.
	rec = do(t, s, http.MethodPatch, "/api/v1/links/abc", map[string]any{
		"code": "def",
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("rename status = %d body %s", rec.Code, rec.Body.String())
	}
	if n, _ := s.store.CountBackups(ctx); n != 2 {
		t.Fatalf("backups after rename = %d, want 2", n)
	}

	// A no-op PATCH (nothing but unchanged fields) must not back up.
	rec = do(t, s, http.MethodPatch, "/api/v1/links/def", map[string]any{})
	if rec.Code != http.StatusOK {
		t.Fatalf("no-op patch status = %d body %s", rec.Code, rec.Body.String())
	}
	if n, _ := s.store.CountBackups(ctx); n != 2 {
		t.Fatalf("backups after no-op = %d, want 2", n)
	}

	// Batch import with conflict=update backs up the overwritten row.
	rec = do(t, s, http.MethodPost, "/api/v1/links/batch", map[string]any{
		"conflict": "update",
		"items": []map[string]any{
			{"url": "https://example.com/c", "code": "def"},
		},
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("batch status = %d body %s", rec.Code, rec.Body.String())
	}
	if n, _ := s.store.CountBackups(ctx); n != 3 {
		t.Fatalf("backups after batch update = %d, want 3", n)
	}
}
