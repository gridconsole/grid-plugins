package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/gridhome"
	"github.com/gridconsole/grid-plugins/internal/project"
)

func landFixture(t *testing.T) (Env, string) {
	t.Helper()
	home := t.TempDir()
	if err := project.Write(home, project.Record{Name: "sample", ID: "example", Folders: []project.Folder{{Machine: "machine-example", Path: gridhome.Real(home)}}}); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(gridhome.TasksDir(home, "sample"), "T-01-example")
	for path, text := range map[string]string{
		filepath.Join(dir, "task.md"):   "---\nid: T-01\ntitle: Example\nproject: sample\nstate: open\ncreated: 2026-11-08T00:00:00Z\n---\n",
		filepath.Join(dir, "intent.md"): "Example\n",
		filepath.Join(dir, "plan.md"):   "# Plan\n\n- [ ] 1 Land the change\n  jobs: []\n",
		filepath.Join(home, "projects", "sample", "knowledge", "deploy", "lands.md"): "---\nlands:\n  target: local\n  branch: main\n  command: printf landed > outcome\n  needs-approve: true\n---\n",
	} {
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(text), 0600); err != nil {
			t.Fatal(err)
		}
	}
	return Env{GridHome: home, Machine: "machine-example", Cwd: gridhome.Real(home)}, dir
}

func TestLandDryRunJSON(t *testing.T) {
	env, dir := landFixture(t)
	var out, stderr bytes.Buffer
	env.Stdout, env.Stderr = &out, &stderr
	if code := Run([]string{"land", "T-01", "--dry-run", "--json"}, env); code != 0 {
		t.Fatalf("code %d: %s", code, &stderr)
	}
	golden, err := os.ReadFile("testdata/land-dry-run.json")
	if err != nil {
		t.Fatal(err)
	}
	if out.String() != string(golden) {
		t.Fatalf("got %s", &out)
	}
	if _, err := os.Stat(filepath.Join(dir, ".land.lock")); !os.IsNotExist(err) {
		t.Fatal("dry run wrote a lock")
	}
}

func TestLandRejectsChangedRuleWithExitThree(t *testing.T) {
	env, dir := landFixture(t)
	b, _ := os.ReadFile(filepath.Join(dir, "task.md"))
	if err := os.WriteFile(filepath.Join(dir, "task.md"), []byte(strings.Replace(string(b), "state: open", "state: waiting", 1)), 0600); err != nil {
		t.Fatal(err)
	}
	var out, stderr bytes.Buffer
	env.Stdout, env.Stderr = &out, &stderr
	if code := Run([]string{"approve", "T-01"}, env); code != 0 {
		t.Fatalf("approve: %d %s", code, &stderr)
	}
	path := filepath.Join(env.GridHome, "projects", "sample", "knowledge", "deploy", "lands.md")
	b, _ = os.ReadFile(path)
	if err := os.WriteFile(path, []byte(strings.Replace(string(b), "target: local", "target: production", 1)), 0600); err != nil {
		t.Fatal(err)
	}
	if code := Run([]string{"land", "T-01"}, env); code != 3 {
		t.Fatalf("land: %d %s", code, &stderr)
	}
	if !strings.Contains(stderr.String(), "another plan or lands rule") {
		t.Fatalf("wrong rejection: %s", &stderr)
	}
	if _, err := os.Stat(filepath.Join(env.Cwd, "outcome")); !os.IsNotExist(err) {
		t.Fatal("rejected land executed")
	}
}
