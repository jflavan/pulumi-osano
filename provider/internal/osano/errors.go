//nolint:goheader // Source-file header normalization is still in progress during alpha.
package osano

import (
	"errors"
	"fmt"
)

// HTTPError reports a non-success Osano API response.
type HTTPError struct {
	StatusCode int
	Body       string
}

func (e *HTTPError) Error() string {
	if e.Body == "" {
		return fmt.Sprintf("osano api error: status=%d", e.StatusCode)
	}
	return fmt.Sprintf("osano api error: status=%d body=%s", e.StatusCode, e.Body)
}

// IsHTTPStatus reports whether err wraps an HTTPError with statusCode.
func IsHTTPStatus(err error, statusCode int) bool {
	var httpErr *HTTPError
	return errors.As(err, &httpErr) && httpErr.StatusCode == statusCode
}
