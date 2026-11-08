package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/gridhome"
	"github.com/gridconsole/grid-plugins/internal/project"
)

func TestLandDryRunJSON(t *testing.T) {
	home := t.TempDir()
	if err := project.Write(home, project.Record{Name: "sample", ID: "example"}); err != nil {
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
	var out, stderr bytes.Buffer
	env := Env{GridHome: home, Cwd: gridhome.Real(home), Stdout: &out, Stderr: &stderr}
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
