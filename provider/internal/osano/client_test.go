package osano

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync/atomic"
	"testing"
	"time"
)

func TestNewClientDefaults(t *testing.T) {
	u, _ := url.Parse("https://api.example.com")
	c := NewClient(u, "x-api-key", "test-key")

	if c.maxRetries != defaultMaxRetries {
		t.Fatalf("expected maxRetries %d, got %d", defaultMaxRetries, c.maxRetries)
	}
	if c.initialBackoff != defaultInitialBackoff {
		t.Fatalf("expected initialBackoff %v, got %v", defaultInitialBackoff, c.initialBackoff)
	}
	if c.headerName != "x-api-key" {
		t.Fatalf("expected headerName x-api-key, got %q", c.headerName)
	}
	if c.apiKey != "test-key" {
		t.Fatalf("expected apiKey test-key, got %q", c.apiKey)
	}
}

func TestNewClientOptions(t *testing.T) {
	u, _ := url.Parse("https://api.example.com")
	c := NewClient(u, "x-api-key", "test-key",
		WithMaxRetries(5),
		WithInitialBackoff(2*time.Second),
	)

	if c.maxRetries != 5 {
		t.Fatalf("expected maxRetries 5, got %d", c.maxRetries)
	}
	if c.initialBackoff != 2*time.Second {
		t.Fatalf("expected initialBackoff 2s, got %v", c.initialBackoff)
	}
}

func TestShouldRetry(t *testing.T) {
	cases := []struct {
		status int
		want   bool
	}{
		{200, false},
		{201, false},
		{204, false},
		{400, false},
		{401, false},
		{403, false},
		{404, false},
		{422, false},
		{429, true},
		{500, true},
		{502, true},
		{503, true},
		{504, true},
	}
	for _, tc := range cases {
		got := shouldRetry(tc.status)
		if got != tc.want {
			t.Errorf("shouldRetry(%d) = %v, want %v", tc.status, got, tc.want)
		}
	}
}

func TestRetryAfterDelay(t *testing.T) {
	cases := []struct {
		name      string
		value     string
		wantDelay time.Duration
		wantOK    bool
	}{
		{"empty", "", 0, false},
		{"seconds", "5", 5 * time.Second, true},
		{"zero seconds", "0", 0, true},
		{"negative seconds", "-1", 0, false},
		{"with spaces", " 5 ", 5 * time.Second, true},
		{"invalid string", "abc", 0, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			delay, ok := retryAfterDelay(tc.value)
			if ok != tc.wantOK {
				t.Fatalf("retryAfterDelay(%q) ok = %v, want %v", tc.value, ok, tc.wantOK)
			}
			if delay != tc.wantDelay {
				t.Fatalf("retryAfterDelay(%q) delay = %v, want %v", tc.value, delay, tc.wantDelay)
			}
		})
	}
}

func TestRetryDelay(t *testing.T) {
	u, _ := url.Parse("https://api.example.com")
	c := NewClient(u, "x-api-key", "key", WithInitialBackoff(100*time.Millisecond))

	t.Run("exponential backoff", func(t *testing.T) {
		d0 := c.retryDelay(nil, 0)
		d1 := c.retryDelay(nil, 1)
		d2 := c.retryDelay(nil, 2)

		if d0 != 100*time.Millisecond {
			t.Fatalf("attempt 0: expected 100ms, got %v", d0)
		}
		if d1 != 200*time.Millisecond {
			t.Fatalf("attempt 1: expected 200ms, got %v", d1)
		}
		if d2 != 400*time.Millisecond {
			t.Fatalf("attempt 2: expected 400ms, got %v", d2)
		}
	})

	t.Run("retry-after header overrides backoff", func(t *testing.T) {
		resp := &http.Response{Header: http.Header{}}
		resp.Header.Set("Retry-After", "3")

		d := c.retryDelay(resp, 0)
		if d != 3*time.Second {
			t.Fatalf("expected 3s from Retry-After, got %v", d)
		}
	})

	t.Run("negative attempt clamped to zero", func(t *testing.T) {
		d := c.retryDelay(nil, -1)
		if d != 100*time.Millisecond {
			t.Fatalf("expected 100ms for negative attempt, got %v", d)
		}
	})
}

func TestSleepWithContext(t *testing.T) {
	t.Run("normal sleep", func(t *testing.T) {
		err := sleepWithContext(context.Background(), time.Millisecond)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("zero duration", func(t *testing.T) {
		err := sleepWithContext(context.Background(), 0)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("negative duration", func(t *testing.T) {
		err := sleepWithContext(context.Background(), -time.Second)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("cancelled context", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		err := sleepWithContext(ctx, time.Hour)
		if err != context.Canceled {
			t.Fatalf("expected context.Canceled, got: %v", err)
		}
	})
}

func TestDoJSONGetSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Errorf("expected GET, got %s", r.Method)
		}
		if r.Header.Get("x-api-key") != "test-key" {
			t.Errorf("expected x-api-key=test-key, got %q", r.Header.Get("x-api-key"))
		}
		if r.Header.Get("accept") != "application/json" {
			t.Errorf("expected accept=application/json, got %q", r.Header.Get("accept"))
		}
		// GET should not have content-type
		if ct := r.Header.Get("content-type"); ct != "" {
			t.Errorf("GET should not have content-type, got %q", ct)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"id": "123"})
	}))
	defer srv.Close()

	u, _ := url.Parse(srv.URL)
	c := NewClient(u, "x-api-key", "test-key")

	var out map[string]string
	err := c.DoJSON(context.Background(), "GET", "/test", nil, nil, &out)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out["id"] != "123" {
		t.Fatalf("expected id=123, got %v", out)
	}
}

func TestDoJSONPostSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.Header.Get("content-type") != "application/json" {
			t.Errorf("expected content-type=application/json, got %q", r.Header.Get("content-type"))
		}

		var body map[string]string
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("failed to decode request body: %v", err)
		}
		if body["name"] != "test" {
			t.Errorf("expected name=test, got %q", body["name"])
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"id": "456"})
	}))
	defer srv.Close()

	u, _ := url.Parse(srv.URL)
	c := NewClient(u, "x-api-key", "test-key")

	var out map[string]string
	err := c.DoJSON(context.Background(), "POST", "/items", nil, map[string]string{"name": "test"}, &out)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out["id"] != "456" {
		t.Fatalf("expected id=456, got %v", out)
	}
}

func TestDoJSONErrorNonRetryable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"error":"not found"}`))
	}))
	defer srv.Close()

	u, _ := url.Parse(srv.URL)
	c := NewClient(u, "x-api-key", "test-key", WithMaxRetries(3))

	err := c.DoJSON(context.Background(), "GET", "/missing", nil, nil, nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	httpErr, ok := err.(*HTTPError)
	if !ok {
		t.Fatalf("expected *HTTPError, got %T", err)
	}
	if httpErr.StatusCode != 404 {
		t.Fatalf("expected status 404, got %d", httpErr.StatusCode)
	}
}

func TestDoJSONRetrySuccess(t *testing.T) {
	var attempts atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := attempts.Add(1)
		if n <= 2 {
			w.WriteHeader(http.StatusTooManyRequests)
			w.Write([]byte(`{"error":"rate limited"}`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}))
	defer srv.Close()

	u, _ := url.Parse(srv.URL)
	c := NewClient(u, "x-api-key", "test-key", WithMaxRetries(3), WithInitialBackoff(time.Millisecond))

	var out map[string]string
	err := c.DoJSON(context.Background(), "GET", "/test", nil, nil, &out)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := attempts.Load(); got != 3 {
		t.Fatalf("expected 3 attempts, got %d", got)
	}
	if out["status"] != "ok" {
		t.Fatalf("expected status=ok, got %v", out)
	}
}

func TestDoJSONRetryExhausted(t *testing.T) {
	var attempts atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("server error"))
	}))
	defer srv.Close()

	u, _ := url.Parse(srv.URL)
	c := NewClient(u, "x-api-key", "test-key", WithMaxRetries(2), WithInitialBackoff(time.Millisecond))

	err := c.DoJSON(context.Background(), "GET", "/test", nil, nil, nil)
	if err == nil {
		t.Fatal("expected error after retries exhausted")
	}
	httpErr, ok := err.(*HTTPError)
	if !ok {
		t.Fatalf("expected *HTTPError, got %T", err)
	}
	if httpErr.StatusCode != 500 {
		t.Fatalf("expected status 500, got %d", httpErr.StatusCode)
	}
	// 1 initial + 2 retries = 3 attempts
	if got := attempts.Load(); got != 3 {
		t.Fatalf("expected 3 attempts, got %d", got)
	}
}

func TestDoJSONPostBodyResentOnRetry(t *testing.T) {
	var attempts atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify the request body is present on every attempt.
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("failed to read body: %v", err)
		}
		var parsed map[string]string
		if err := json.Unmarshal(body, &parsed); err != nil {
			t.Errorf("failed to parse body on attempt %d: %v", attempts.Load()+1, err)
		}
		if parsed["key"] != "value" {
			t.Errorf("expected key=value on attempt %d, got %v", attempts.Load()+1, parsed)
		}

		n := attempts.Add(1)
		if n == 1 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"done": "true"})
	}))
	defer srv.Close()

	u, _ := url.Parse(srv.URL)
	c := NewClient(u, "x-api-key", "test-key", WithMaxRetries(2), WithInitialBackoff(time.Millisecond))

	var out map[string]string
	err := c.DoJSON(context.Background(), "POST", "/work", nil, map[string]string{"key": "value"}, &out)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := attempts.Load(); got != 2 {
		t.Fatalf("expected 2 attempts, got %d", got)
	}
}

func TestDoJSONDeleteNilOutput(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "DELETE" {
			t.Errorf("expected DELETE, got %s", r.Method)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	u, _ := url.Parse(srv.URL)
	c := NewClient(u, "x-api-key", "test-key")

	err := c.DoJSON(context.Background(), "DELETE", "/items/123", nil, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDoJSONEmptyResponseBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		// Empty body
	}))
	defer srv.Close()

	u, _ := url.Parse(srv.URL)
	c := NewClient(u, "x-api-key", "test-key")

	var out map[string]string
	err := c.DoJSON(context.Background(), "GET", "/empty", nil, nil, &out)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// out should remain nil/zero-value since body was empty
	if out != nil {
		t.Fatalf("expected nil output for empty body, got %v", out)
	}
}

func TestDoJSONQueryParams(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("page") != "2" {
			t.Errorf("expected query page=2, got %q", r.URL.Query().Get("page"))
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"page": "2"})
	}))
	defer srv.Close()

	u, _ := url.Parse(srv.URL)
	c := NewClient(u, "x-api-key", "test-key")

	q := url.Values{"page": {"2"}}
	var out map[string]string
	err := c.DoJSON(context.Background(), "GET", "/items", q, nil, &out)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out["page"] != "2" {
		t.Fatalf("expected page=2, got %v", out)
	}
}

func TestDoJSONContextCancelled(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()

	u, _ := url.Parse(srv.URL)
	c := NewClient(u, "x-api-key", "test-key", WithMaxRetries(5), WithInitialBackoff(time.Second))

	ctx, cancel := context.WithCancel(context.Background())
	// Cancel immediately so the retry sleep is interrupted.
	go func() {
		time.Sleep(10 * time.Millisecond)
		cancel()
	}()

	err := c.DoJSON(ctx, "GET", "/slow", nil, nil, nil)
	if err == nil {
		t.Fatal("expected error from cancelled context")
	}
}

func TestDoJSONNoApiKey(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("x-api-key") != "" {
			t.Errorf("expected no x-api-key header, got %q", r.Header.Get("x-api-key"))
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"ok": "true"})
	}))
	defer srv.Close()

	u, _ := url.Parse(srv.URL)
	c := NewClient(u, "x-api-key", "")

	var out map[string]string
	err := c.DoJSON(context.Background(), "GET", "/public", nil, nil, &out)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDoJSONRetryWithRetryAfterHeader(t *testing.T) {
	var attempts atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := attempts.Add(1)
		if n == 1 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"ok": "true"})
	}))
	defer srv.Close()

	u, _ := url.Parse(srv.URL)
	c := NewClient(u, "x-api-key", "test-key", WithMaxRetries(2), WithInitialBackoff(time.Second))

	start := time.Now()
	var out map[string]string
	err := c.DoJSON(context.Background(), "GET", "/test", nil, nil, &out)
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Retry-After: 0 should mean immediate retry, well under the 1s initial backoff.
	if elapsed > 500*time.Millisecond {
		t.Fatalf("expected fast retry with Retry-After: 0, took %v", elapsed)
	}
}
