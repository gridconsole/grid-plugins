package hook

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync/atomic"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/hookio"
)

// A permission request no guard decides is left to Claude Code's own prompt and
// deny rules: nothing is sent to GRID_URL and no decision is printed.
func TestPermissionRequestIsNotForwardedToGrid(t *testing.T) {
	f := eventSetup(t)
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		_, _ = w.Write([]byte(`{"behavior":"allow"}`))
	}))
	defer server.Close()
	f.env["GRID_URL"] = server.URL
	in := hookio.Input{ToolName: "Read", ToolInput: map[string]any{"file_path": filepath.Join(filepath.Dir(f.home), "outside.md")}}
	out, log, code := f.run(t, "PermissionRequest", in)
	if code != 0 || out != "" || log != "" || calls.Load() != 0 {
		t.Fatalf("forwarded: code=%d out=%q log=%q calls=%d", code, out, log, calls.Load())
	}
}
