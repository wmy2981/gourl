package api

import (
	"bufio"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/wmy2981/gourl/internal/logx"
)

func TestLogHistoryNotConfigured(t *testing.T) {
	logx.Close() // ensure no LOG_DIR from other tests
	s, _ := newTestServer(t)
	rec := do(t, s, http.MethodGet, "/api/v1/logs", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("history status = %d, body %s", rec.Code, rec.Body.String())
	}
	var body struct {
		Available bool          `json:"available"`
		Records   []logx.Record `json:"records"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Available {
		t.Fatal("expected available=false without LOG_DIR")
	}
	if len(body.Records) != 0 {
		t.Fatalf("expected empty records, got %d", len(body.Records))
	}
}

func TestLogHistoryReadsFile(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("LOG_DIR", dir)
	logx.Init(slog.LevelDebug)
	t.Cleanup(logx.Close)

	slog.Info("history marker", "code", "abc")

	s, _ := newTestServer(t)
	rec := do(t, s, http.MethodGet, "/api/v1/logs?limit=10", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("history status = %d, body %s", rec.Code, rec.Body.String())
	}
	var body struct {
		Available bool          `json:"available"`
		Records   []logx.Record `json:"records"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if !body.Available {
		t.Fatal("expected available=true with LOG_DIR")
	}
	if len(body.Records) == 0 {
		t.Fatal("expected records from the log file")
	}
	if !strings.Contains(body.Records[0].Message, "history marker") {
		t.Fatalf("expected the marker record, got %+v", body.Records[0])
	}
}

func TestLogHistoryRequiresAuth(t *testing.T) {
	logx.Close()
	s, _ := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/logs", nil)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 without auth, got %d", rec.Code)
	}
}

func TestWriteSSEEvent(t *testing.T) {
	rec := httptest.NewRecorder()
	writeSSEEvent(rec, http.NewResponseController(rec), logx.Record{
		Level:   "info",
		Message: "hello",
		Attrs:   map[string]any{"code": "abc"},
	})
	body := rec.Body.String()
	if !strings.HasPrefix(body, "event: log\ndata: ") || !strings.HasSuffix(body, "\n\n") {
		t.Fatalf("unexpected SSE frame: %q", body)
	}
	if !strings.Contains(body, `"message":"hello"`) || !strings.Contains(body, `"code":"abc"`) {
		t.Fatalf("SSE frame missing payload: %q", body)
	}
}

func TestLogStreamLive(t *testing.T) {
	logx.Close()
	s, _ := newTestServer(t)
	ts := httptest.NewServer(s.Handler())
	defer ts.Close()

	req, err := http.NewRequest(http.MethodGet, ts.URL+"/api/v1/logs/stream", nil)
	if err != nil {
		t.Fatal(err)
	}
	if testSession != nil {
		req.AddCookie(testSession)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "text/event-stream") {
		t.Fatalf("expected event-stream content type, got %q", ct)
	}

	// The subscription is established right after the handler writes headers;
	// give it a beat so the first record is not broadcast before it attaches.
	time.Sleep(100 * time.Millisecond)
	slog.Info("stream marker message")

	scanner := bufio.NewScanner(resp.Body)
	lines := make(chan string, 32)
	go func() {
		for scanner.Scan() {
			lines <- scanner.Text()
		}
		close(lines)
	}()
	deadline := time.After(5 * time.Second)
	got := ""
	for got == "" {
		select {
		case line, ok := <-lines:
			if !ok {
				if err := scanner.Err(); err != nil {
					t.Fatalf("stream read error: %v", err)
				}
				t.Fatal("stream closed before a log record arrived")
			}
			if strings.HasPrefix(line, "data: ") {
				got = line
			}
		case <-deadline:
			t.Fatal("timed out waiting for a streamed log record")
		}
	}
	if !strings.Contains(got, "stream marker message") {
		t.Fatalf("expected the marker record on the stream, got %q", got)
	}
}
