package cli

import (
	"archive/zip"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/doctor"
)

func TestDoctorBundleCLI(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { requests.Add(1) }))
	defer server.Close()
	t.Setenv("HTTP_PROXY", server.URL)
	t.Setenv("HTTPS_PROXY", server.URL)
	t.Setenv("GRID_RELEASE_MANIFEST_URL", server.URL)
	h := newHarness(t)
	// Unhealthy installations must still produce usable support bundles.
	if code := h.run(h.tmp, "doctor", "--bundle", "--out", "support.zip", "--json"); code != 0 {
		t.Fatalf("exit %d: %s", code, h.errb)
	}
	var result struct {
		Path   string        `json:"path"`
		Doctor doctor.Report `json:"doctor"`
	}
	if err := json.Unmarshal(h.out.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.Doctor.OK || result.Path != filepath.Join(h.tmp, "support.zip") {
		t.Fatalf("%+v", result)
	}
	archive, err := zip.OpenReader(result.Path)
	if err != nil {
		t.Fatal(err)
	}
	archive.Close()
	if requests.Load() != 0 {
		t.Fatal("bundle made a network request")
	}
	if code := h.run(h.tmp, "doctor", "--bundle"); code != 0 {
		t.Fatalf("exit %d: %s", code, h.errb)
	}
	if _, err = os.Stat(strings.TrimSpace(h.out.String())); err != nil {
		t.Fatal(err)
	}
	if code := h.run(h.tmp, "doctor", "--out", "bad.zip"); code != 2 {
		t.Fatalf("unexpected --out exit %d", code)
	}
}
