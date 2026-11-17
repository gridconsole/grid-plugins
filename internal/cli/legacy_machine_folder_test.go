package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/gridhome"
)

// oldAppMachineFiles is what the older desktop app kept in ~/.grid/machine, with dummy contents.
var oldAppMachineFiles = map[string]string{
	"config.json": "{\"dummy\":true}\n", "host.lock": "4242\n", "licence": "dummy licence\n",
	"licence.moved": "dummy moved\n", "machine-key": "dummy key\n", "machine-token": "dummy token\n",
}

// legacyMachineHome puts the older app's machine folder where the machine id file belongs.
func legacyMachineHome(t *testing.T, h *harness) string {
	t.Helper()
	dir := filepath.Join(h.gridHome, "machine")
	for name, text := range oldAppMachineFiles {
		h.write(filepath.Join(dir, name), text)
	}
	return dir
}

func wantOldAppFolder(t *testing.T, dir string) {
	t.Helper()
	for name, want := range oldAppMachineFiles {
		if got, err := os.ReadFile(filepath.Join(dir, name)); err != nil || string(got) != want {
			t.Errorf("%s/%s: %q, %v; want %q", filepath.Base(dir), name, got, err, want)
		}
	}
}

func machineIDIn(t *testing.T, gridHome string) string {
	t.Helper()
	st, err := os.Lstat(filepath.Join(gridHome, "machine"))
	if err != nil || !st.Mode().IsRegular() {
		t.Fatalf("machine is not an id file: %v, %v", st, err)
	}
	b, _ := os.ReadFile(filepath.Join(gridHome, "machine"))
	id := strings.TrimSpace(string(b))
	if !gridhome.ValidID(id) {
		t.Fatalf("machine holds %q, not a ULID", b)
	}
	return id
}

func TestProjectAddMovesTheOldAppMachineFolderAside(t *testing.T) {
	h := newHarness(t)
	legacyMachineHome(t, h)
	first := filepath.Join(h.root, "work", "first")
	second := filepath.Join(h.root, "work", "second")
	h.mkdir(first, second)

	if code := h.run(first, "project", "add", first, "--name", "X", "--json"); code != 0 {
		t.Fatalf("project add exit %d: %s", code, h.errb)
	}
	var added struct{ Name string }
	if err := json.Unmarshal(h.out.Bytes(), &added); err != nil || added.Name != "X" {
		t.Fatalf("project add answered %q: %v", h.out, err)
	}
	id := machineIDIn(t, h.gridHome)
	wantOldAppFolder(t, filepath.Join(h.gridHome, "machine.old-app"))

	// A second run changes nothing: same id, no second copy of the folder.
	if code := h.run(second, "project", "add", second, "--name", "Y", "--json"); code != 0 {
		t.Fatalf("second project add exit %d: %s", code, h.errb)
	}
	if again := machineIDIn(t, h.gridHome); again != id {
		t.Errorf("machine id changed from %s to %s", id, again)
	}
	wantOldAppFolder(t, filepath.Join(h.gridHome, "machine.old-app"))
	if _, err := os.Lstat(filepath.Join(h.gridHome, "machine.old-app-2")); err == nil {
		t.Error("the second run moved something aside again")
	}
}

func TestProjectAddKeepsAPersonsEditOfTheMachineFile(t *testing.T) {
	h := newHarness(t)
	h.write(filepath.Join(h.gridHome, "machine"), "my own notes\n")
	folder := filepath.Join(h.root, "work", "first")
	h.mkdir(folder)

	if code := h.run(folder, "project", "add", folder, "--name", "X", "--json"); code == 0 {
		t.Fatalf("project add over a hand-edited machine file succeeded: %s", h.out)
	}
	if got, _ := os.ReadFile(filepath.Join(h.gridHome, "machine")); string(got) != "my own notes\n" {
		t.Errorf("a person's edit was replaced: %q", got)
	}
	if _, err := os.Lstat(filepath.Join(h.gridHome, "machine.old-app")); err == nil {
		t.Error("a regular file was moved aside")
	}
}

func TestTaskNewMovesTheOldAppMachineFolderAside(t *testing.T) {
	h := newHarness(t)
	h.env.Machine = ""
	for _, key := range []string{"GRID_LEASE", "GRID_LEASE_EPOCH", "GRID_WAKE", "GRID_WORKER", "GRID_FEED_TASK", "GRID_JOB", "GRID_ATTEMPT"} {
		t.Setenv(key, "")
	}
	t.Setenv("GRID_MEMBER", "member")
	t.Setenv("GRID_ROLE", "owner")
	legacyMachineHome(t, h)

	if code := h.run(h.home, "task", "new", "Use the old home", "--json"); code != 0 {
		t.Fatalf("task new exit %d: %s", code, h.errb)
	}
	machineIDIn(t, h.gridHome)
	wantOldAppFolder(t, filepath.Join(h.gridHome, "machine.old-app"))
}
