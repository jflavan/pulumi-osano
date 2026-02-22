package osano

import "fmt"

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
