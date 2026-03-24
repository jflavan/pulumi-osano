package osano

import "testing"

func TestHTTPErrorWithBody(t *testing.T) {
	t.Parallel()

	err := &HTTPError{StatusCode: 404, Body: `{"error":"not found"}`}
	want := `osano api error: status=404 body={"error":"not found"}`
	if err.Error() != want {
		t.Fatalf("got %q, want %q", err.Error(), want)
	}
}

func TestHTTPErrorWithoutBody(t *testing.T) {
	t.Parallel()

	err := &HTTPError{StatusCode: 500}
	want := "osano api error: status=500"
	if err.Error() != want {
		t.Fatalf("got %q, want %q", err.Error(), want)
	}
}
