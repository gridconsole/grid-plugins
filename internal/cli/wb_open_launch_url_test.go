package cli

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// wbOpenCode is a launch code as the app writes it: 43 base64url characters.
const wbOpenCode = "q7-Lm_2xZ9pR4tV8wK0jH3nB6cF1yD5sG-aE_uJ0iOk"

// wbOpenBase is the address the app writes to app-url, without its trailing slash.
const wbOpenBase = "http://127.0.0.1:8484"

// wbOpenHome is the open fixture with app-url in the Grid home and no AppStart, the
// launch-code file the app writes (one line, mode 0600) when withCode, and a browser
// that records the address it was handed. route is the task page's hash route.
func wbOpenHome(t *testing.T, withCode bool) (h *harness, route string, opened *[]string) {
	t.Helper()
	h, project, id := openFixture(t, true)
	h.write(filepath.Join(h.gridHome, "app-url"), wbOpenBase+"/\n")
	if withCode {
		if err := os.WriteFile(filepath.Join(h.gridHome, "launch-code"), []byte(wbOpenCode+"\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	opened = &[]string{}
	h.env.OpenURL = func(u string) error { *opened = append(*opened, u); return nil }
	return h, "#/t/" + project + "/" + id + "/overview", opened
}

// wbOpenJSONURL runs grid open T-01 --json and returns its one field, url.
func wbOpenJSONURL(t *testing.T, h *harness) string {
	t.Helper()
	if code := h.run(h.home, "open", "T-01", "--json"); code != 0 {
		t.Fatalf("open --json: exit %d, stderr %q", code, h.errb)
	}
	var got map[string]string
	if err := json.Unmarshal(h.out.Bytes(), &got); err != nil || len(got) != 1 {
		t.Fatalf("json: %q %v", h.out, err)
	}
	return got["url"]
}

// wbOpenBrowse runs grid open T-01 and returns the one address the browser was handed.
func wbOpenBrowse(t *testing.T, h *harness, opened *[]string) string {
	t.Helper()
	*opened = nil
	if code := h.run(h.home, "open", "T-01"); code != 0 {
		t.Fatalf("open: exit %d, stderr %q", code, h.errb)
	}
	if len(*opened) != 1 {
		t.Fatalf("opened %q, want one address", *opened)
	}
	return (*opened)[0]
}

// wbOpenNoCodeOnStderr fails when the launch code shows on the stderr of a run.
func wbOpenNoCodeOnStderr(t *testing.T, run string, h *harness) {
	t.Helper()
	if strings.Contains(h.errb.String(), wbOpenCode) {
		t.Errorf("%s: stderr %q holds the launch code", run, h.errb)
	}
}

func TestWbOpenJSONCarriesTheCode(t *testing.T) {
	h, route, opened := wbOpenHome(t, true)
	want := wbOpenBase + "/api/launch?code=" + wbOpenCode + route
	if got := wbOpenJSONURL(t, h); got != want {
		t.Fatalf("--json url %q, want %q", got, want)
	}
	if got := wbOpenBrowse(t, h, opened); got != want {
		t.Fatalf("browser got %q, want %q", got, want)
	}
}

func TestWbOpenNoCodeFileOpensPlainURL(t *testing.T) {
	h, route, opened := wbOpenHome(t, false)
	plain := wbOpenBase + "/" + route
	if got := wbOpenJSONURL(t, h); got != plain {
		t.Fatalf("--json url %q, want %q", got, plain)
	}
	if got := wbOpenBrowse(t, h, opened); got != plain {
		t.Fatalf("browser got %q, want %q", got, plain)
	}

	// GRID_APP_URL names an app this machine does not run: it never gets this home's code.
	h, route, opened = wbOpenHome(t, true)
	t.Setenv("GRID_APP_URL", "https://grid.example.test")
	plain = "https://grid.example.test/" + route
	if got := wbOpenJSONURL(t, h); got != plain {
		t.Fatalf("GRID_APP_URL --json url %q, want %q", got, plain)
	}
	if got := wbOpenBrowse(t, h, opened); got != plain {
		t.Fatalf("GRID_APP_URL browser got %q, want %q", got, plain)
	}
}

func TestWbOpenCodeNotOnStderr(t *testing.T) {
	// Row 1's runs: the code is in the address handed on, and never on stderr.
	h, route, opened := wbOpenHome(t, true)
	want := wbOpenBase + "/api/launch?code=" + wbOpenCode + route
	if got := wbOpenJSONURL(t, h); got != want {
		t.Fatalf("--json url %q, want %q", got, want)
	}
	wbOpenNoCodeOnStderr(t, "--json", h)
	if got := wbOpenBrowse(t, h, opened); got != want {
		t.Fatalf("browser got %q, want %q", got, want)
	}
	wbOpenNoCodeOnStderr(t, "browser", h)

	// No browser: the address goes to stdout for the person to open; stderr says only why.
	h.env.OpenURL = func(string) error { return errors.New("no display") }
	if code := h.run(h.home, "open", "T-01"); code != 0 {
		t.Fatalf("open with no browser: exit %d, stderr %q", code, h.errb)
	}
	if got := strings.TrimSpace(h.out.String()); got != want {
		t.Fatalf("no browser: stdout %q, want %q", got, want)
	}
	if !strings.Contains(h.errb.String(), "no browser was opened") {
		t.Errorf("no browser: stderr %q does not say why", h.errb)
	}
	wbOpenNoCodeOnStderr(t, "no browser", h)

	// Row 2's runs with a code file present: GRID_APP_URL set.
	h, _, opened = wbOpenHome(t, true)
	t.Setenv("GRID_APP_URL", "https://grid.example.test")
	if got := wbOpenJSONURL(t, h); strings.Contains(got, wbOpenCode) {
		t.Fatalf("GRID_APP_URL --json url %q holds the code", got)
	}
	wbOpenNoCodeOnStderr(t, "GRID_APP_URL --json", h)
	wbOpenBrowse(t, h, opened)
	wbOpenNoCodeOnStderr(t, "GRID_APP_URL browser", h)
}
