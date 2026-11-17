package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/brain"
	"github.com/gridconsole/grid-plugins/internal/gridconfig"
)

// pbVrHome makes a home folder whose Grid config has one workspace, with the data
// folder under it and a brain folder only when brainKey is set.
func pbVrHome(t *testing.T, brainKey bool) (home, data, vault string) {
	t.Helper()
	home = t.TempDir()
	for _, key := range []string{"GRID_HOME", "GRID_PROJECTS", "USERPROFILE", "APPDATA", "LOCALAPPDATA"} {
		t.Setenv(key, "")
	}
	t.Setenv("HOME", home)
	data = filepath.Join(home, "data")
	vault = filepath.Join(home, "vault")
	if err := os.MkdirAll(data, 0o700); err != nil {
		t.Fatal(err)
	}
	ws := gridconfig.Workspace{Name: "w", Data: data, Member: "Sam"}
	if brainKey {
		ws.Brain = vault
	}
	cfg := gridconfig.Config{Current: "w", Workspaces: []gridconfig.Workspace{ws}}
	if err := gridconfig.Write(gridconfig.Path(os.Getenv, home), cfg); err != nil {
		t.Fatal(err)
	}
	return home, data, vault
}

// Through the binary's own environment, a config with the brain key lists the
// vault and shows a note by its path id.
func TestPbVrBinaryListsTheVaultOfTheConfig(t *testing.T) {
	_, _, vault := pbVrHome(t, true)
	for rel, text := range map[string]string{
		"Ideas/Café plan.md": "Plan the café with [[Plain]].\n",
		"Plain.md":           "Plain.\n",
		".obsidian/x.md":     "hidden\n",
	} {
		p := filepath.Join(vault, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	out, code := pcRun(t, "brain", "list", "--json", "--level", "home")
	if code != 0 {
		t.Fatalf("list exit %d: %s", code, out)
	}
	var l brain.Listing
	if err := json.Unmarshal([]byte(out), &l); err != nil {
		t.Fatalf("%v: %s", err, out)
	}
	if l.Root != vault || len(l.Notes) != 2 {
		t.Fatalf("listing: %s", out)
	}
	if out, code := pcRun(t, "brain", "show", "brain:home/Ideas/Café plan", "--json"); code != 0 || !strings.Contains(out, `"Ideas/Café plan.md"`) {
		t.Fatalf("show exit %d: %s", code, out)
	}
}

// With the key absent the same command lists the old knowledge/ layout.
func TestPbVrBinaryListsAnOldBrainWithTheKeyAbsent(t *testing.T) {
	_, data, _ := pbVrHome(t, false)
	if out, code := pcRun(t, "--grid-home", data, "brain", "propose", "--level", "home", "--name", "old-note", "--topic", "decisions",
		"--description", "Before a merge.", "--summary", "Run the checks.", "--source", "job:T-01/J-01", "--body", "Run the checks.\n"); code != 0 {
		t.Fatalf("propose: %d %s", code, out)
	}
	if out, code := pcRun(t, "--grid-home", data, "brain", "accept", "old-note", "--level", "home"); code != 0 {
		t.Fatalf("accept: %d %s", code, out)
	}
	out, code := pcRun(t, "brain", "list", "--json", "--level", "home")
	if code != 0 {
		t.Fatalf("list exit %d: %s", code, out)
	}
	var l brain.Listing
	if err := json.Unmarshal([]byte(out), &l); err != nil || len(l.Notes) != 1 {
		t.Fatalf("%v: %s", err, out)
	}
	if r := l.Notes[0]; r.ID != "brain:home/old-note" || r.Path != "knowledge/decisions/old-note.md" || l.Root != data {
		t.Fatalf("row %+v root %q", r, l.Root)
	}
}
