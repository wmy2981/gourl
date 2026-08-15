package fetcher

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func newTestFetcher(t *testing.T, allowPrivate bool) *Fetcher {
	t.Helper()
	return New(Options{AllowPrivateIPs: allowPrivate})
}

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

	f := newTestFetcher(t, true)
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

	f := newTestFetcher(t, true)
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

	f := newTestFetcher(t, true)
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

	f := newTestFetcher(t, true)
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

	f := newTestFetcher(t, true)
	if _, _, err := f.Fetch(context.Background(), srv.URL); err == nil {
		t.Fatal("expected error for redirect loop")
	}
}

func TestSSRFBlocksLoopback(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("<title>secret</title>"))
	}))
	defer srv.Close()

	// Default fetcher (protection on) must refuse the loopback address.
	f := newTestFetcher(t, false)
	if _, _, err := f.Fetch(context.Background(), srv.URL); err == nil {
		t.Fatal("expected SSRF rejection of loopback address")
	}
}

func TestSSRFBlocksPrivateRanges(t *testing.T) {
	f := newTestFetcher(t, false)
	for _, u := range []string{
		"http://10.0.0.1/",
		"http://172.16.0.1/",
		"http://192.168.1.1/",
		"http://169.254.169.254/latest/meta-data/",
		"http://127.0.0.1:8080/",
		"http://[::1]/",
	} {
		if _, _, err := f.Fetch(context.Background(), u); err == nil {
			t.Errorf("expected SSRF rejection for %s", u)
		}
	}
}

func TestFetchRejectsNonHTTP(t *testing.T) {
	f := newTestFetcher(t, true)
	for _, u := range []string{"ftp://example.com/x", "file:///etc/passwd", "not-a-url"} {
		if _, _, err := f.Fetch(context.Background(), u); err == nil {
			t.Errorf("expected error for %s", u)
		}
	}
}
