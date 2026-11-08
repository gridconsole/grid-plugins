package cli

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestVersionCheckCLI(t *testing.T) {
	hits := 0
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		fmt.Fprint(w, `{"version":"99.0.0","contract":"2","app_min":"2.0.0"}`)
	}))
	defer s.Close()
	t.Setenv("GRID_RELEASE_MANIFEST_URL", s.URL)
	t.Setenv("GRID_APP_VERSION", "0.0.1")
	t.Setenv("GRID_SESSION_ID", "example-session")
	h := newHarness(t)
	if code := h.run(h.tmp, "version", "--check", "--json"); code != 0 {
		t.Fatalf("%d: %s", code, h.errb)
	}
	var r struct {
		Check struct {
			Status string `json:"status"`
			Cached bool   `json:"cached"`
		} `json:"check"`
		Warning string `json:"warning"`
	}
	if err := json.Unmarshal(h.out.Bytes(), &r); err != nil {
		t.Fatal(err)
	}
	if r.Check.Status != "update available" || r.Warning == "" || hits != 1 {
		t.Fatalf("%s hits=%d", h.out, hits)
	}
	if code := h.run(h.tmp, "version", "--check"); code != 0 || hits != 1 || strings.Contains(h.out.String(), "Update the app.") {
		t.Fatalf("%d: %s hits=%d", code, h.out, hits)
	}
	if code := h.run(h.tmp, "version", "--check", "--manifest-url", "file:///manifest"); code != 0 || !strings.Contains(h.out.String(), "unavailable") || hits != 1 {
		t.Fatalf("%d: %s", code, h.out)
	}
	if code := h.run(h.tmp, "version", "--check", "--timeout", "0s"); code != 2 {
		t.Fatalf("%d: %s", code, h.errb)
	}
}
