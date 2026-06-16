package notifications

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestShoutrrrNotifier_Send_GenericWebhook(t *testing.T) {
	type recorder struct {
		mu       sync.Mutex
		method   string
		body     string
		received bool
	}
	rec := &recorder{}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		rec.mu.Lock()
		rec.method = r.Method
		rec.body = string(body)
		rec.received = true
		rec.mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	url := "generic+" + srv.URL + "?messagekey=message&titlekey=title&template=json"

	notifier := NewShoutrrrNotifier()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := notifier.Send(ctx, url, "Test Title", "Hello World"); err != nil {
		t.Fatalf("Send returned error: %v", err)
	}

	rec.mu.Lock()
	defer rec.mu.Unlock()

	if !rec.received {
		t.Fatal("webhook server received no request")
	}
	if rec.method != http.MethodPost {
		t.Errorf("expected POST, got %s", rec.method)
	}

	var payload map[string]string
	if err := json.Unmarshal([]byte(rec.body), &payload); err != nil {
		t.Fatalf("body is not valid JSON: %v\nbody: %s", err, rec.body)
	}
	if payload["message"] != "Hello World" {
		t.Errorf("expected message %q, got %q", "Hello World", payload["message"])
	}
	if payload["title"] != "Test Title" {
		t.Errorf("expected title %q, got %q", "Test Title", payload["title"])
	}
}

func TestShoutrrrNotifier_Send_EmptyURL(t *testing.T) {
	notifier := NewShoutrrrNotifier()
	err := notifier.Send(context.Background(), "", "title", "msg")
	if err == nil {
		t.Fatal("expected error for empty URL, got nil")
	}
}

func TestShoutrrrNotifier_Send_InvalidURL(t *testing.T) {
	notifier := NewShoutrrrNotifier()
	err := notifier.Send(context.Background(), "not-a-shoutrrr-url", "title", "msg")
	if err == nil {
		t.Fatal("expected error for invalid URL, got nil")
	}
	if !strings.Contains(err.Error(), "shoutrrr") {
		t.Errorf("expected error to mention shoutrrr, got: %v", err)
	}
}

// TestShoutrrrNotifier_Send_RealService sends a real notification to the URL in
// SHOUTRRR_TEST_URL. Skipped when the env var is empty. Useful for manual
// validation of URL formats during integration testing.
//
// Example:
//
//	SHOUTRRR_TEST_URL="discord://TOKEN@WEBHOOK_ID" go test ./notifications/ -run RealService -v
func TestShoutrrrNotifier_Send_RealService(t *testing.T) {
	url := os.Getenv("SHOUTRRR_TEST_URL")
	if url == "" {
		t.Skip("SHOUTRRR_TEST_URL not set, skipping real-service test")
	}

	notifier := NewShoutrrrNotifier()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := notifier.Send(ctx, url, "Watchflare test", "Hello from the Shoutrrr integration test"); err != nil {
		t.Fatalf("Send to real service failed: %v", err)
	}
	t.Log("notification sent successfully")
}
