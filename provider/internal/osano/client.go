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
    "time"
)

type Client struct {
    baseURL *url.URL
    apiKey  string

    headerName string

    http *http.Client
}

func NewClient(baseURL *url.URL, headerName, apiKey string) *Client {
    return &Client{
        baseURL:     baseURL,
        apiKey:      apiKey,
        headerName:  headerName,
        http:        &http.Client{Timeout: 30 * time.Second},
    }
}

func (c *Client) DoJSON(ctx context.Context, method, pth string, query url.Values, in any, out any) error {
    // Build URL
    u := *c.baseURL
    u.Path = path.Join(c.baseURL.Path, pth)
    u.RawQuery = query.Encode()

    var body io.Reader
    if in != nil {
        b, err := json.Marshal(in)
        if err != nil {
            return fmt.Errorf("marshal request: %w", err)
        }
        body = bytes.NewReader(b)
    }

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
    defer resp.Body.Close()

    b, err := io.ReadAll(resp.Body)
    if err != nil {
        return fmt.Errorf("read response: %w", err)
    }

    if resp.StatusCode < 200 || resp.StatusCode >= 300 {
        return &HTTPError{StatusCode: resp.StatusCode, Body: string(b)}
    }

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
