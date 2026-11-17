//go:build unix

package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/machinefolder"
)

// Row 1 and 3: grid open under a machine folder takes its start lock and writes its
// program logs in launcher/<login>/, and leaves nothing of them in the Grid home.
func TestOpenStartLockAndLogsGoToTheMachineFolder(t *testing.T) {
	f := newStartFixture(t)
	machine := filepath.Join(t.TempDir(), "machine")
	f.env.MachineFolder = machinefolder.Folder{Path: machine, Source: machinefolder.SourceEnv, SharedGroup: "grid"}
	f.env.Login = "alice"
	f.env.OpenURL = func(string) error { return nil }
	if code := f.run(f.home, "open", "T-01"); code != 0 {
		t.Fatalf("open: %d %s", code, f.errb)
	}
	if w, s := f.counts(); w != 1 || s != 1 {
		t.Fatalf("starts: worker %d, server %d, want 1 and 1", w, s)
	}
	for _, name := range []string{"lock", "grid-server.log", "grid-worker.log"} {
		if _, err := os.Stat(filepath.Join(machine, "launcher", "alice", name)); err != nil {
			t.Errorf("launcher/alice/%s: %v", name, err)
		}
	}
	for _, name := range []string{"open-start.lock", "logs"} {
		if _, err := os.Lstat(filepath.Join(f.gridHome, name)); err == nil {
			t.Errorf("%s was made in the Grid home although the machine folder is on", name)
		}
	}
}

// Row 2: with no machine folder the lock and the logs are where they were.
func TestOpenStartLockAndLogsStayInTheGridHomeWithoutAMachineFolder(t *testing.T) {
	f := newStartFixture(t)
	f.env.OpenURL = func(string) error { return nil }
	if code := f.run(f.home, "open", "T-01"); code != 0 {
		t.Fatalf("open: %d %s", code, f.errb)
	}
	for _, name := range []string{OpenStartLock, filepath.Join("logs", "grid-server.log"), filepath.Join("logs", "grid-worker.log")} {
		st, err := os.Stat(filepath.Join(f.gridHome, name))
		if err != nil || st.Mode().Perm() != 0o600 {
			t.Errorf("%s: %v %v, want a 0600 file", name, st, err)
		}
	}
}
