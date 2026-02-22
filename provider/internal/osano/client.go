package osano

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"time"
)

type Client struct {
	baseURL *url.URL
	apiKey  string

	headerName string

	http *http.Client

	maxRetries     int
	initialBackoff time.Duration
}

type ClientOption func(*Client)

const (
	defaultMaxRetries     = 3
	defaultInitialBackoff = time.Second
)

func WithMaxRetries(maxRetries int) ClientOption {
	return func(c *Client) {
		c.maxRetries = maxRetries
	}
}

func WithInitialBackoff(backoff time.Duration) ClientOption {
	return func(c *Client) {
		c.initialBackoff = backoff
	}
}

func NewClient(baseURL *url.URL, headerName, apiKey string, opts ...ClientOption) *Client {
	c := &Client{
		baseURL:        baseURL,
		apiKey:         apiKey,
		headerName:     headerName,
		http:           &http.Client{Timeout: 30 * time.Second},
		maxRetries:     defaultMaxRetries,
		initialBackoff: defaultInitialBackoff,
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

func (c *Client) DoJSON(ctx context.Context, method, pth string, query url.Values, in any, out any) error {
	// Build URL
	u := *c.baseURL
	u.Path = path.Join(c.baseURL.Path, pth)
	u.RawQuery = query.Encode()

	var bodyBytes []byte
	var body io.Reader
	if in != nil {
		b, err := json.Marshal(in)
		if err != nil {
			return fmt.Errorf("marshal request: %w", err)
		}
		bodyBytes = b
		body = bytes.NewReader(b)
	}

	maxRetries := c.maxRetries
	if maxRetries < 0 {
		maxRetries = 0
	}

	for attempt := 0; ; attempt++ {
		req, err := http.NewRequestWithContext(ctx, method, u.String(), body)
		if err != nil {
			return fmt.Errorf("new request: %w", err)
		}
		req.Header.Set("accept", "application/json")
		if in != nil {
			req.Header.Set("content-type", "application/json")
		}
		if c.apiKey != "" {
			req.Header.Set(c.headerName, c.apiKey)
		}

		resp, err := c.http.Do(req)
		if err != nil {
			return fmt.Errorf("do request: %w", err)
		}

		b, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return fmt.Errorf("read response: %w", err)
		}

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			if out == nil {
				return nil
			}
			if len(bytes.TrimSpace(b)) == 0 {
				return nil
			}
			if err := json.Unmarshal(b, out); err != nil {
				return fmt.Errorf("unmarshal response: %w", err)
			}
			return nil
		}

		if shouldRetry(resp.StatusCode) && attempt < maxRetries {
			delay := c.retryDelay(resp, attempt)
			if err := sleepWithContext(ctx, delay); err != nil {
				return err
			}
			if len(bodyBytes) > 0 {
				body = bytes.NewReader(bodyBytes)
			} else {
				body = nil
			}
			continue
		}

		return &HTTPError{StatusCode: resp.StatusCode, Body: string(b)}
	}
}

func shouldRetry(statusCode int) bool {
	return statusCode == http.StatusTooManyRequests || statusCode >= 500
}

func (c *Client) retryDelay(resp *http.Response, attempt int) time.Duration {
	if resp != nil {
		if delay, ok := retryAfterDelay(resp.Header.Get("Retry-After")); ok {
			return delay
		}
	}

	backoff := c.initialBackoff
	if backoff <= 0 {
		backoff = defaultInitialBackoff
	}
	if attempt < 0 {
		attempt = 0
	}
	return backoff * time.Duration(1<<attempt)
}

func retryAfterDelay(value string) (time.Duration, bool) {
	if value == "" {
		return 0, false
	}

	if seconds, err := strconv.Atoi(strings.TrimSpace(value)); err == nil {
		if seconds < 0 {
			return 0, false
		}
		return time.Duration(seconds) * time.Second, true
	}

	if t, err := http.ParseTime(value); err == nil {
		delay := time.Until(t)
		if delay < 0 {
			return 0, true
		}
		return delay, true
	}

	return 0, false
}

func sleepWithContext(ctx context.Context, delay time.Duration) error {
	if delay <= 0 {
		return nil
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
