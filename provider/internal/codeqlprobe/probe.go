// Package codeqlprobe is a deliberate vulnerability used only to verify that the CodeQL gate blocks a
// pull request. It lives on a throwaway branch and must never be merged.
package codeqlprobe

import (
	"net/http"
	"os/exec"
)

// Handler runs a request parameter as a shell command (command injection, on purpose).
func Handler(w http.ResponseWriter, r *http.Request) {
	out, _ := exec.Command("sh", "-c", r.URL.Query().Get("cmd")).Output() //nolint:gosec // deliberate
	_, _ = w.Write(out)
}
