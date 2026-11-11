package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/project"
)

func goUnsetFixture(t *testing.T) (*harness, string) {
	t.Helper()
	h := newHarness(t)
	for _, k := range []string{"HOME", "CLAUDE_CONFIG_DIR", "CODEX_HOME", "GRID_HOME"} {
		t.Setenv(k, t.TempDir())
	}
	repo := filepath.Join(h.root, "repo")
	h.mkdir(repo)
	h.record(project.Record{Name: "example", ID: "example-id", Folders: []project.Folder{{Machine: h.env.Machine, Path: repo}}})
	return h, repo
}

func goUnsetRun(t *testing.T, h *harness, cwd string, args ...string) int {
	t.Helper()
	h.out.Reset()
	h.errb.Reset()
	return h.run(cwd, args...)
}

func goUnsetRead(t *testing.T, path string) []byte {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	return b
}

func TestGoAgentUnsetInherits(t *testing.T) {
	h, repo := goUnsetFixture(t)
	home := filepath.Join(h.gridHome, "settings.json")
	proj := filepath.Join(h.gridHome, "projects", "example", "settings.json")
	for _, args := range [][]string{
		{"agent", "set", "grid-builder", "--model", "haiku", "--effort", "low"},
		{"agent", "set", "grid-builder", "--project", "example", "--model", "opus", "--effort", "high"},
	} {
		if code := goUnsetRun(t, h, repo, args...); code != 0 {
			t.Fatalf("%v: %d %s", args, code, h.errb)
		}
	}
	homeBefore := goUnsetRead(t, home)
	if code := goUnsetRun(t, h, repo, "agent", "unset", "grid-builder", "--project", "example", "--json"); code != 0 {
		t.Fatalf("unset: %d %s", code, h.errb)
	}
	var got map[string]string
	if err := json.Unmarshal(h.out.Bytes(), &got); err != nil || len(got) != 3 || got["kind"] != "agent" || got["name"] != "grid-builder" || got["project"] != "example" {
		t.Fatalf("json %q err %v", h.out, err)
	}
	if strings.Contains(string(goUnsetRead(t, proj)), "grid-builder") {
		t.Fatalf("project file still names the agent: %s", goUnsetRead(t, proj))
	}
	if !bytes.Equal(homeBefore, goUnsetRead(t, home)) {
		t.Fatal("home file changed")
	}
	if code := goUnsetRun(t, h, repo, "agent", "list", "--json"); code != 0 {
		t.Fatalf("list: %d %s", code, h.errb)
	}
	if want := `"grid-builder":{"model":"haiku","effort":"low"}`; !strings.Contains(h.out.String(), want) {
		t.Fatalf("list %s lacks %s", h.out, want)
	}
}

func TestGoAgentUnsetNoEntryAndUsage(t *testing.T) {
	h, repo := goUnsetFixture(t)
	home := filepath.Join(h.gridHome, "settings.json")
	if code := goUnsetRun(t, h, repo, "agent", "set", "grid-builder", "--model", "opus"); code != 0 {
		t.Fatal(h.errb)
	}
	before := goUnsetRead(t, home)
	proj := filepath.Join(h.gridHome, "projects", "example", "settings.json")
	if code := goUnsetRun(t, h, repo, "agent", "unset", "grid-builder", "--project", "example"); code != 0 {
		t.Fatalf("no-entry unset: %d %s", code, h.errb)
	}
	if _, err := os.Stat(proj); !os.IsNotExist(err) {
		t.Fatalf("no-entry unset created %s: %v", proj, err)
	}
	if code := goUnsetRun(t, h, repo, "agent", "unset", "grid-scout"); code != 0 {
		t.Fatalf("no-entry unset at home: %d %s", code, h.errb)
	}
	if !bytes.Equal(before, goUnsetRead(t, home)) {
		t.Fatal("no-entry unset changed bytes")
	}
	if code := goUnsetRun(t, h, repo, "agent", "unsets", "grid-builder"); code != 2 || !strings.Contains(h.errb.String(), `unknown command "unsets"`) {
		t.Fatalf("unknown op: %d %s", code, h.errb)
	}
	if code := goUnsetRun(t, h, repo, "agent", "unset", "grid-builder", "--model", "opus"); code == 0 {
		t.Fatal("unset accepted --model")
	}
}

func TestGoAgentUnsetGuestRefused(t *testing.T) {
	h, cwd := settingsAccessFixture(t, "guest")
	path := filepath.Join(h.gridHome, "projects", "alpha", "settings.json")
	h.write(path, "{\"agents\":{\"grid-builder\":{\"model\":\"opus\"}}}\n")
	before := settingsAccessTree(t, h.gridHome)
	for _, args := range [][]string{
		{"agent", "unset", "grid-builder", "--project", "alpha"},
		{"agent", "unset", "grid-builder"},
	} {
		if code := goUnsetRun(t, h, cwd, args...); code != 1 || h.out.Len() != 0 {
			t.Errorf("%v: exit %d stdout %q stderr %q", args, code, h.out, h.errb)
		}
	}
	if got := settingsAccessTree(t, h.gridHome); len(got) != len(before) || got[path] != before[path] {
		t.Error("refused unset changed Grid home")
	}
}
