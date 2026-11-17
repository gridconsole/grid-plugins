package machinefolder_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/machinefolder"
)

// A feed with no task id has no name to put under tasks/, so with an active folder
// its state.json stays where it was: no error that stops the fold, and no file in the
// machine folder that two such feeds would share.
func TestStateFileWithoutATaskIDStaysInTheFeedFolder(t *testing.T) {
	machine := filepath.Join(t.TempDir(), "machine")
	feedDir := filepath.Join(t.TempDir(), "feed")
	if err := os.MkdirAll(feedDir, 0o700); err != nil {
		t.Fatal(err)
	}
	f := machinefolder.Folder{Path: machine, Source: machinefolder.SourceEnv}
	if !f.Active() {
		t.Fatal("folder is not Active")
	}
	path, err := f.StateFile(feedDir, "")
	if err != nil {
		t.Fatalf("StateFile with no task id: %v", err)
	}
	if want := filepath.Join(feedDir, "state.json"); path != want {
		t.Fatalf("StateFile = %q, want %q", path, want)
	}
	if err := f.WriteStateFile(path, []byte(`{"jobs":{}}`)); err != nil {
		t.Fatalf("WriteStateFile outside the machine folder: %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil || string(got) != `{"jobs":{}}` {
		t.Fatalf("state.json = %q, %v", got, err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if mode := info.Mode().Perm(); mode != 0o600 {
		t.Errorf("state.json mode = %o, want 600 as before", mode)
	}
	if _, err := os.Stat(machine); err == nil {
		t.Errorf("the machine folder %s was made for a feed that has no task id", machine)
	}
}
