package api

import (
	"io/fs"
	"net/http"
	"strings"
	"testing"

	"github.com/wmy2981/gourl/internal/webui"
)

// TestSPAAssetsAreServed verifies the embedded build artifacts are reachable
// through /assets/, which the admin SPA depends on to boot.
func TestSPAAssetsAreServed(t *testing.T) {
	s, _ := newTestServer(t)
	dist := webui.Dist()

	// The SPA shell is served at /admin.
	rec := get(t, s, "/admin", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("/admin status = %d, want 200", rec.Code)
	}

	// Every hashed asset in the embed must resolve over HTTP.
	var found int
	fs.WalkDir(dist, "assets", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		rec := get(t, s, "/assets/"+strings.TrimPrefix(path, "assets/"), nil)
		if rec.Code != http.StatusOK {
			t.Errorf("GET /assets/%s = %d, want 200", path, rec.Code)
		}
		found++
		return nil
	})
	if found == 0 {
		t.Fatal("embed contains no assets to serve")
	}
}

// TestCustomIconTakesPrecedenceOverEmbedded confirms uploaded icons keep
// their /assets/custom-icon.* route.
func TestCustomIconTakesPrecedenceOverEmbedded(t *testing.T) {
	s, _ := newTestServer(t)
	rec := uploadIconRequest(t, s, "custom-icon.svg", "<svg xmlns='http://www.w3.org/2000/svg'/>")
	if rec.Code != http.StatusOK {
		t.Fatalf("upload icon status = %d", rec.Code)
	}
	rec = get(t, s, "/assets/custom-icon.svg", nil)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "<svg") {
		t.Errorf("custom icon not served: status %d body %q", rec.Code, rec.Body.String())
	}
}
