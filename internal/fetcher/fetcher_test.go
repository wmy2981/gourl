package fetcher

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// The fetcher reaches any host — internal networks included — so the
// loopback test servers below fetch directly without extra options.
func TestFetchExtractsTitleAndDescription(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte(`<!DOCTYPE html>
<html><head>
<title>   Example  &amp;  Title   </title>
<meta name="description" content="A   long    description here">
<meta name="keywords" content="ignored">
</head><body>content</body></html>`))
	}))
	defer srv.Close()

	f := New()
	title, desc, err := f.Fetch(context.Background(), srv.URL)
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if title != "Example & Title" {
		t.Errorf("title = %q, want %q", title, "Example & Title")
	}
	if desc != "A long description here" {
		t.Errorf("description = %q", desc)
	}
}

func TestFetchRejectsNonHTML(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"x":1}`))
	}))
	defer srv.Close()

	f := New()
	if _, _, err := f.Fetch(context.Background(), srv.URL); err == nil {
		t.Fatal("expected error for non-html content")
	}
}

func TestFetchRejectsOversizeBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(strings.Repeat("a", defaultMaxBody+1024)))
	}))
	defer srv.Close()

	f := New()
	if _, _, err := f.Fetch(context.Background(), srv.URL); err == nil {
		t.Fatal("expected error for oversize body")
	}
}

func TestFetchFollowsRedirectsWithValidation(t *testing.T) {
	final := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte("<title>Final</title>"))
	}))
	defer final.Close()

	hop := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, final.URL, http.StatusFound)
	}))
	defer hop.Close()

	f := New()
	title, _, err := f.Fetch(context.Background(), hop.URL)
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if title != "Final" {
		t.Errorf("title = %q, want Final", title)
	}
}

func TestFetchRejectsTooManyRedirects(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, r.URL.String(), http.StatusFound) // infinite self-redirect
	}))
	defer srv.Close()

	f := New()
	if _, _, err := f.Fetch(context.Background(), srv.URL); err == nil {
		t.Fatal("expected error for redirect loop")
	}
}

// The old SSRF tests (loopback/private refusal) are replaced by this: private
// and loopback addresses are fetchable — title fetching must support internal
// networks end to end.
func TestFetchReachesPrivateAddresses(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte("<title>internal</title>"))
	}))
	defer srv.Close()

	f := New()
	title, _, err := f.Fetch(context.Background(), srv.URL) // 127.0.0.1 loopback
	if err != nil {
		t.Fatalf("Fetch loopback: %v", err)
	}
	if title != "internal" {
		t.Errorf("title = %q, want internal", title)
	}
}

func TestFetchRejectsNonHTTP(t *testing.T) {
	f := New()
	for _, u := range []string{"ftp://example.com/x", "file:///etc/passwd", "not-a-url"} {
		if _, _, err := f.Fetch(context.Background(), u); err == nil {
			t.Errorf("expected error for %s", u)
		}
	}
}
