package cli

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

const (
	akOpenNoCodeLine  = "grid open: no launch code was found; the page opens without one.\n"
	akOpenNoAddrLine  = "grid open: the app's address is not known. Start the app, then run grid open again."
	akOpenNoTaskError = "give a task id or folder, or run inside a task folder"
)

// akOpenOutside makes a plain folder beside the home, in no project and in no task.
func akOpenOutside(h *harness) string {
	dir := h.root + "/elsewhere"
	h.mkdir(dir)
	return dir
}

func TestAkOpenHomeOutsideProject(t *testing.T) {
	h, _, opened := wbOpenHome(t, true)
	outside := akOpenOutside(h)
	if code := h.run(outside, "open"); code != 0 {
		t.Fatalf("open outside a project: exit %d, stderr %q", code, h.errb)
	}
	want := wbOpenBase + "/api/launch?code=" + wbOpenCode + "#/"
	if len(*opened) != 1 || (*opened)[0] != want {
		t.Fatalf("browser got %q, want %q", *opened, want)
	}
	if strings.Contains(h.errb.String(), akOpenNoTaskError) {
		t.Errorf("stderr %q still asks for a task", h.errb)
	}

	// Without the launch-code file the page opens plain and stderr says so.
	h, _, opened = wbOpenHome(t, false)
	outside = akOpenOutside(h)
	if code := h.run(outside, "open"); code != 0 {
		t.Fatalf("open with no code: exit %d, stderr %q", code, h.errb)
	}
	plain := wbOpenBase + "/#/"
	if len(*opened) != 1 || (*opened)[0] != plain {
		t.Fatalf("browser got %q, want %q", *opened, plain)
	}
	if got := h.errb.String(); got != akOpenNoCodeLine {
		t.Fatalf("stderr %q, want %q", got, akOpenNoCodeLine)
	}
}

func TestAkOpenHomeInProjectFolder(t *testing.T) {
	// The project folder of the open fixture, which holds a task but is not inside it.
	h, _, opened := wbOpenHome(t, true)
	if code := h.run(h.home, "open"); code != 0 {
		t.Fatalf("open in project folder: exit %d, stderr %q", code, h.errb)
	}
	want := wbOpenBase + "/api/launch?code=" + wbOpenCode + "#/"
	if len(*opened) != 1 || (*opened)[0] != want {
		t.Fatalf("browser got %q, want %q (not the project board or a task)", *opened, want)
	}
}

func TestAkOpenHomeJSON(t *testing.T) {
	h, _, opened := wbOpenHome(t, false)
	if code := h.run(h.home, "open", "--json"); code != 0 {
		t.Fatalf("open --json: exit %d, stderr %q", code, h.errb)
	}
	var got map[string]string
	if err := json.Unmarshal(h.out.Bytes(), &got); err != nil || len(got) != 1 {
		t.Fatalf("json: %q %v", h.out, err)
	}
	if u := got["url"]; !strings.HasSuffix(u, "/#/") || u != wbOpenBase+"/#/" {
		t.Fatalf("url %q, want %s", u, wbOpenBase+"/#/")
	}
	if len(*opened) != 1 || (*opened)[0] != wbOpenBase+"/#/" {
		t.Fatalf("browser got %q", *opened)
	}

	// No app-url and no GRID_APP_URL: the route alone, and the line that says why.
	h, _ = akOpenBare(t)
	if code := h.run(h.home, "open", "--json"); code != 0 {
		t.Fatalf("open --json with no address: exit %d, stderr %q", code, h.errb)
	}
	if got := strings.TrimSpace(h.out.String()); got != `{"url":"#/"}` {
		t.Fatalf("stdout %q, want {\"url\":\"#/\"}", got)
	}
	// As for a task route, --json returns before the plain-mode stderr line (open.go:160-164).
	if h.errb.Len() != 0 {
		t.Fatalf("--json stderr %q, want empty", h.errb)
	}
	if code := h.run(h.home, "open"); code != 0 {
		t.Fatalf("open with no address: exit %d, stderr %q", code, h.errb)
	}
	if got := strings.TrimSpace(h.out.String()); got != "#/" {
		t.Fatalf("stdout %q, want #/", got)
	}
	if !strings.Contains(h.errb.String(), akOpenNoAddrLine) {
		t.Fatalf("stderr %q lacks %q", h.errb, akOpenNoAddrLine)
	}
}

// akOpenBare is the open fixture with no app address and a browser that must not be called.
func akOpenBare(t *testing.T) (*harness, string) {
	t.Helper()
	h, _, _ := openFixture(t, true)
	h.env.OpenURL = func(u string) error { t.Fatalf("opened %s with no app address", u); return nil }
	return h, h.home
}

func TestAkOpenTaskStillOpensTask(t *testing.T) {
	h, project, id, folder := openFixtureAt(t, true)
	h.write(filepath.Join(h.gridHome, AppURLFile), wbOpenBase+"/\n")
	opened := &[]string{}
	h.env.OpenURL = func(u string) error { *opened = append(*opened, u); return nil }
	if code := h.run(folder, "open"); code != 0 {
		t.Fatalf("open inside the task: exit %d, stderr %q", code, h.errb)
	}
	want := wbOpenBase + "/#/t/" + project + "/" + id + "/overview"
	if len(*opened) != 1 || (*opened)[0] != want {
		t.Fatalf("browser got %q, want %q", *opened, want)
	}

	*opened = nil
	if code := h.run(h.home, "open", "T-99"); code == 0 {
		t.Fatalf("open T-99 exited 0: %s", h.out)
	}
	if len(*opened) != 0 {
		t.Fatalf("open T-99 opened %q", *opened)
	}
}
