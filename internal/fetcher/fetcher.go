// Package fetcher retrieves page titles and descriptions from user-supplied
// URLs. Any reachable host is allowed (internal networks included); each hop
// is still checked to be an absolute http(s) URL.
package fetcher

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"golang.org/x/net/html"
)

const (
	defaultTimeout        = 5 * time.Second
	defaultMaxBody        = 1 << 20 // 1 MiB
	defaultMaxRedirects   = 5
	defaultTitleLimit     = 200
	defaultDescriptionMax = 500
	browserUA             = "Mozilla/5.0 (compatible; gourl/0.1 +https://github.com/wmy2981/gourl)"
)

// Fetcher downloads and parses the title and description of a page.
type Fetcher struct {
	client         *http.Client
	titleLimit     int
	descriptionMax int
}

// New creates a Fetcher with default limits.
func New() *Fetcher {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	f := &Fetcher{
		titleLimit:     defaultTitleLimit,
		descriptionMax: defaultDescriptionMax,
	}
	f.client = &http.Client{
		Timeout:   defaultTimeout,
		Transport: transport,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= defaultMaxRedirects {
				return errors.New("too many redirects")
			}
			if err := validateFetchURL(req.URL.String()); err != nil {
				return fmt.Errorf("redirect rejected: %w", err)
			}
			return nil
		},
	}
	return f
}

// Fetch returns the page title and meta description of rawURL. Fetching is
// lenient on purpose: internal services often answer with odd status codes
// or content types yet still carry a <title>, so any response body is
// parsed (bounded by the size cap) and "no title" is the result, not an
// error. Failures that do return an error are network/URL problems the
// caller cannot fix here.
func (f *Fetcher) Fetch(ctx context.Context, rawURL string) (title, description string, err error) {
	slog.Debug("fetching title", "url", rawURL)
	if err := validateFetchURL(rawURL); err != nil {
		return "", "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return "", "", fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("User-Agent", browserUA)
	req.Header.Set("Accept", "text/html")

	resp, err := f.client.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, defaultMaxBody+1))
	if err != nil {
		return "", "", fmt.Errorf("read body: %w", err)
	}
	if len(body) > defaultMaxBody {
		return "", "", errors.New("response body too large")
	}

	doc, err := html.Parse(bytes.NewReader(body))
	if err != nil {
		return "", "", fmt.Errorf("parse html: %w", err)
	}
	t, d := extractMeta(doc)
	t = truncate(clean(t), f.titleLimit)
	d = truncate(clean(d), f.descriptionMax)
	slog.Debug("title fetched", "url", rawURL, "status", resp.StatusCode, "title_len", len(t), "description_len", len(d))
	return t, d, nil
}

// extractMeta walks the tree for a title and a description. The title falls
// back in order: <title>, then og:title/twitter:title, then the first <h1> —
// many internal devices serve pages without a <title> tag yet still show a
// heading worth keeping. The description comes from the first
// <meta name="description">.
func extractMeta(root *html.Node) (title, description string) {
	var firstH1 string
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode {
			switch n.Data {
			case "title":
				if title == "" {
					title = textContent(n)
				}
			case "h1":
				if firstH1 == "" {
					firstH1 = textContent(n)
				}
			case "meta":
				var name, content string
				for _, a := range n.Attr {
					switch a.Key {
					case "name", "property":
						name = a.Val
					case "content":
						content = a.Val
					}
				}
				switch {
				case description == "" && strings.EqualFold(name, "description"):
					description = content
				case title == "" && (strings.EqualFold(name, "og:title") || strings.EqualFold(name, "twitter:title")):
					title = content
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(root)
	if title == "" && firstH1 != "" {
		title = firstH1
	}
	return title, description
}

// textContent joins every text node under an element. Recursive on purpose:
// device pages often wrap their heading text in links or spans
// (<h1><a>Device</a></h1>), and the direct-children version returned "" for
// those — silently killing the h1 fallback.
func textContent(n *html.Node) string {
	var b strings.Builder
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			if c.Type == html.TextNode {
				b.WriteString(c.Data)
			} else if c.Type == html.ElementNode && c.Data != "script" && c.Data != "style" {
				walk(c)
			}
		}
	}
	walk(n)
	return b.String()
}

// clean trims and collapses whitespace.
func clean(s string) string { return strings.Join(strings.Fields(s), " ") }

// truncate cuts s to n runes (n <= 0 means no limit).
func truncate(s string, n int) string {
	if n <= 0 || len([]rune(s)) <= n {
		return s
	}
	return string([]rune(s)[:n])
}
