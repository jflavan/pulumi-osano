package osano

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync/atomic"
	"testing"
	"time"
)

func TestNewClientDefaults(t *testing.T) {
	t.Parallel()

	baseURL, _ := url.Parse("https://api.example.com")
	client := NewClient(baseURL, "x-api-key", "test-key")

	if client.maxRetries != defaultMaxRetries {
		t.Fatalf("expected maxRetries %d, got %d", defaultMaxRetries, client.maxRetries)
	}
	if client.initialBackoff != defaultInitialBackoff {
		t.Fatalf("expected initialBackoff %v, got %v", defaultInitialBackoff, client.initialBackoff)
	}
	if client.headerName != "x-api-key" {
		t.Fatalf("expected headerName x-api-key, got %q", client.headerName)
	}
	if client.apiKey != "test-key" {
		t.Fatalf("expected apiKey test-key, got %q", client.apiKey)
	}
}

func TestNewClientOptions(t *testing.T) {
	t.Parallel()

	baseURL, _ := url.Parse("https://api.example.com")
	client := NewClient(
		baseURL,
		"x-api-key",
		"test-key",
		WithMaxRetries(5),
		WithInitialBackoff(2*time.Second),
	)

	if client.maxRetries != 5 {
		t.Fatalf("expected maxRetries 5, got %d", client.maxRetries)
	}
	if client.initialBackoff != 2*time.Second {
		t.Fatalf("expected initialBackoff 2s, got %v", client.initialBackoff)
	}
}

func TestNewClientUsesProvidedHTTPClient(t *testing.T) {
	t.Parallel()

	baseURL, err := url.Parse("https://api.example.com")
	if err != nil {
		t.Fatal(err)
	}
	httpClient := &http.Client{Timeout: 17 * time.Second}
	client := NewClient(baseURL, "x-osano-api-key", "key", WithHTTPClient(httpClient))
	if client.http != httpClient {
		t.Fatal("expected NewClient to retain the provided HTTP client")
	}
}

func TestShouldRetry(t *testing.T) {
	t.Parallel()

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
		t.Run(http.StatusText(tc.status), func(t *testing.T) {
			t.Parallel()

			if got := shouldRetry(tc.status); got != tc.want {
				t.Fatalf("shouldRetry(%d) = %v, want %v", tc.status, got, tc.want)
			}
		})
	}
}

func TestRetryAfterDelay(t *testing.T) {
	t.Parallel()

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
			t.Parallel()

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

func TestRetryAfterDelayHTTPDate(t *testing.T) {
	t.Parallel()

	delay, ok := retryAfterDelay("Wed, 21 Oct 2015 07:28:00 GMT")
	if !ok {
		t.Fatal("expected HTTP-date Retry-After to be accepted")
	}
	if delay != 0 {
		t.Fatalf("expected a past HTTP-date Retry-After delay of 0, got %s", delay)
	}
}

func TestRetryDelay(t *testing.T) {
	t.Parallel()

	baseURL, _ := url.Parse("https://api.example.com")
	client := NewClient(baseURL, "x-api-key", "key", WithInitialBackoff(100*time.Millisecond))

	t.Run("exponential backoff", func(t *testing.T) {
		d0 := client.retryDelay(nil, 0)
		d1 := client.retryDelay(nil, 1)
		d2 := client.retryDelay(nil, 2)

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

		if delay := client.retryDelay(resp, 0); delay != 3*time.Second {
			t.Fatalf("expected 3s from Retry-After, got %v", delay)
		}
	})

	t.Run("negative attempt clamped to zero", func(t *testing.T) {
		if delay := client.retryDelay(nil, -1); delay != 100*time.Millisecond {
			t.Fatalf("expected 100ms for negative attempt, got %v", delay)
		}
	})
}

func TestSleepWithContext(t *testing.T) {
	t.Parallel()

	t.Run("normal sleep", func(t *testing.T) {
		if err := sleepWithContext(context.Background(), time.Millisecond); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("zero duration", func(t *testing.T) {
		if err := sleepWithContext(context.Background(), 0); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("negative duration", func(t *testing.T) {
		if err := sleepWithContext(context.Background(), -time.Second); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("cancelled context", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		if err := sleepWithContext(ctx, time.Hour); err != context.Canceled {
			t.Fatalf("expected context.Canceled, got %v", err)
		}
	})
}

func TestDoJSONGetSuccess(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", r.Method)
		}
		if r.Header.Get("x-api-key") != "test-key" {
			t.Fatalf("expected x-api-key=test-key, got %q", r.Header.Get("x-api-key"))
		}
		if r.Header.Get("accept") != "application/json" {
			t.Fatalf("expected accept=application/json, got %q", r.Header.Get("accept"))
		}
		if ct := r.Header.Get("content-type"); ct != "" {
			t.Fatalf("GET should not have content-type, got %q", ct)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"id": "123"})
	}))
	defer server.Close()

	baseURL, _ := url.Parse(server.URL)
	client := NewClient(baseURL, "x-api-key", "test-key")

	var out map[string]string
	if err := client.DoJSON(context.Background(), http.MethodGet, "/test", nil, nil, &out); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out["id"] != "123" {
		t.Fatalf("expected id=123, got %v", out)
	}
}

func TestDoJSONSendsUserAgent(t *testing.T) {
	t.Parallel()

	var got atomic.Value
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got.Store(r.Header.Get("User-Agent"))
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{})
	}))
	defer server.Close()

	baseURL, _ := url.Parse(server.URL)
	client := NewClient(baseURL, "x-api-key", "test-key", WithUserAgent("pulumi-osano/1.2.3"))
	if err := client.DoJSON(context.Background(), http.MethodGet, "/test", nil, nil, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ua, _ := got.Load().(string); ua != "pulumi-osano/1.2.3" {
		t.Fatalf("expected User-Agent pulumi-osano/1.2.3, got %q", ua)
	}
}

func TestDoJSONPostSuccess(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", r.Method)
		}
		if r.Header.Get("content-type") != "application/json" {
			t.Fatalf("expected content-type=application/json, got %q", r.Header.Get("content-type"))
		}

		var body map[string]string
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("failed to decode request body: %v", err)
		}
		if body["name"] != "test" {
			t.Fatalf("expected name=test, got %q", body["name"])
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"id": "456"})
	}))
	defer server.Close()

	baseURL, _ := url.Parse(server.URL)
	client := NewClient(baseURL, "x-api-key", "test-key")

	var out map[string]string
	if err := client.DoJSON(
		context.Background(),
		http.MethodPost,
		"/items",
		nil,
		map[string]string{"name": "test"},
		&out,
	); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out["id"] != "456" {
		t.Fatalf("expected id=456, got %v", out)
	}
}

func TestDoJSONErrorNonRetryable(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":"not found"}`))
	}))
	defer server.Close()

	baseURL, _ := url.Parse(server.URL)
	client := NewClient(baseURL, "x-api-key", "test-key", WithMaxRetries(3))

	err := client.DoJSON(context.Background(), http.MethodGet, "/missing", nil, nil, nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	httpErr, ok := err.(*HTTPError)
	if !ok {
		t.Fatalf("expected *HTTPError, got %T", err)
	}
	if httpErr.StatusCode != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d", httpErr.StatusCode)
	}
}

func TestDoJSONRetrySuccess(t *testing.T) {
	t.Parallel()

	var attempts atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		count := attempts.Add(1)
		if count <= 2 {
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`{"error":"rate limited"}`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}))
	defer server.Close()

	baseURL, _ := url.Parse(server.URL)
	client := NewClient(baseURL, "x-api-key", "test-key", WithMaxRetries(3), WithInitialBackoff(time.Millisecond))

	var out map[string]string
	if err := client.DoJSON(context.Background(), http.MethodGet, "/test", nil, nil, &out); err != nil {
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
	t.Parallel()

	var attempts atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		attempts.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("server error"))
	}))
	defer server.Close()

	baseURL, _ := url.Parse(server.URL)
	client := NewClient(baseURL, "x-api-key", "test-key", WithMaxRetries(2), WithInitialBackoff(time.Millisecond))

	err := client.DoJSON(context.Background(), http.MethodGet, "/test", nil, nil, nil)
	if err == nil {
		t.Fatal("expected error after retries exhausted")
	}
	httpErr, ok := err.(*HTTPError)
	if !ok {
		t.Fatalf("expected *HTTPError, got %T", err)
	}
	if httpErr.StatusCode != http.StatusInternalServerError {
		t.Fatalf("expected status 500, got %d", httpErr.StatusCode)
	}
	if got := attempts.Load(); got != 3 {
		t.Fatalf("expected 3 attempts, got %d", got)
	}
}

func TestDoJSONPreservesEscapedPathSegments(t *testing.T) {
	t.Parallel()

	var gotRawPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotRawPath = r.URL.EscapedPath()
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	baseURL, _ := url.Parse(server.URL + "/root")
	client := NewClient(baseURL, "x-api-key", "test-key")

	pth := "/v1/configs/" + url.PathEscape("a/b c")
	if err := client.DoJSON(context.Background(), http.MethodGet, pth, nil, nil, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if want := "/root/v1/configs/a%2Fb%20c"; gotRawPath != want {
		t.Fatalf("expected request path %q, got %q", want, gotRawPath)
	}
}

func TestDoJSONPostRetryPolicy(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name         string
		status       int
		writeRetries bool
		wantAttempts int32
	}{
		{"502 is not replayed", http.StatusBadGateway, false, 1},
		{"500 is not replayed", http.StatusInternalServerError, false, 1},
		{"504 is not replayed", http.StatusGatewayTimeout, false, 1},
		{"429 is retried", http.StatusTooManyRequests, false, 3},
		{"503 is retried", http.StatusServiceUnavailable, false, 3},
		{"502 is retried when write retries are allowed", http.StatusBadGateway, true, 3},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var attempts atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				attempts.Add(1)
				w.WriteHeader(tc.status)
			}))
			defer server.Close()

			baseURL, _ := url.Parse(server.URL)
			client := NewClient(
				baseURL, "x-api-key", "test-key", WithMaxRetries(2), WithInitialBackoff(time.Millisecond),
			)
			ctx := context.Background()
			if tc.writeRetries {
				ctx = WithWriteRetries(ctx)
			}

			err := client.DoJSON(ctx, http.MethodPost, "/items", nil, map[string]string{"name": "x"}, nil)
			if !IsHTTPStatus(err, tc.status) {
				t.Fatalf("expected final HTTP %d error, got %v", tc.status, err)
			}
			if got := attempts.Load(); got != tc.wantAttempts {
				t.Fatalf("expected %d attempts, got %d", tc.wantAttempts, got)
			}
		})
	}
}

func TestRetryDelayCapsRetryAfter(t *testing.T) {
	t.Parallel()

	baseURL, _ := url.Parse("https://api.example.com")
	client := NewClient(baseURL, "x-api-key", "key")
	resp := &http.Response{Header: http.Header{}}
	resp.Header.Set("Retry-After", "3600")

	if delay := client.retryDelay(resp, 0); delay != maxRetryAfterDelay {
		t.Fatalf("expected Retry-After to be capped at %s, got %s", maxRetryAfterDelay, delay)
	}
}

func TestDoJSONRetriesTransportErrorsForGetOnly(t *testing.T) {
	t.Parallel()

	newFlakyServer := func(attempts *atomic.Int32) *httptest.Server {
		return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			if attempts.Add(1) == 1 {
				conn, _, err := w.(http.Hijacker).Hijack()
				if err == nil {
					_ = conn.Close()
				}
				return
			}
			w.WriteHeader(http.StatusNoContent)
		}))
	}

	t.Run("GET retries after a dropped connection", func(t *testing.T) {
		t.Parallel()
		var attempts atomic.Int32
		server := newFlakyServer(&attempts)
		defer server.Close()
		baseURL, _ := url.Parse(server.URL)
		client := NewClient(baseURL, "x-api-key", "key", WithInitialBackoff(time.Millisecond))
		if err := client.DoJSON(context.Background(), http.MethodGet, "/poll", nil, nil, nil); err != nil {
			t.Fatalf("expected GET to recover, got %v", err)
		}
		if got := attempts.Load(); got != 2 {
			t.Fatalf("expected 2 attempts, got %d", got)
		}
	})

	t.Run("POST does not replay after a dropped connection", func(t *testing.T) {
		t.Parallel()
		var attempts atomic.Int32
		server := newFlakyServer(&attempts)
		defer server.Close()
		baseURL, _ := url.Parse(server.URL)
		client := NewClient(baseURL, "x-api-key", "key", WithInitialBackoff(time.Millisecond))
		err := client.DoJSON(context.Background(), http.MethodPost, "/create", nil, map[string]string{"a": "b"}, nil)
		if err == nil {
			t.Fatal("expected POST transport error")
		}
		if got := attempts.Load(); got != 1 {
			t.Fatalf("expected a single POST attempt, got %d", got)
		}
	})
}
