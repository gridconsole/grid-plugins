//go:build !windows

package doctor

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/fslock"
	"github.com/gridconsole/grid-plugins/internal/version"
)

// vsDocHoldWorker writes worker-status.json with the given version and holds
// worker.lock from a second handle, as the app's worker does.
func vsDocHoldWorker(t *testing.T, gridHome, status string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(gridHome, "worker-status.json"), []byte(status), 0o600); err != nil {
		t.Fatal(err)
	}
	lock := filepath.Join(gridHome, "worker.lock")
	if err := os.WriteFile(lock, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	f, err := fslock.OpenNoFollow(lock, os.O_RDONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { fslock.Unlock(f); f.Close() })
	if err := fslock.TryLockExclusive(f); err != nil {
		t.Fatal(err)
	}
}

func TestVsDocWorkerOld(t *testing.T) {
	o := vsDocOptions(t)
	vsDocHoldWorker(t, o.GridHome, `{"pid":1,"version":"0.0.9"}`)
	rep := Run(o)
	c := vsDocWant(t, rep, "worker")
	want := version.AppWarning("", "", "0.0.9")
	if want == "" {
		t.Fatal("0.0.9 is not older than AppMin")
	}
	if c.OK || rep.OK || c.Detail != want {
		t.Fatalf("got %+v, want detail %q", c, want)
	}
	if c.Fix != "Update the Grid app, then start it again." {
		t.Errorf("fix = %q", c.Fix)
	}
}

func TestVsDocWorkerCurrent(t *testing.T) {
	for _, v := range []string{version.AppMin, "9.9.9"} {
		o := vsDocOptions(t)
		vsDocHoldWorker(t, o.GridHome, `{"pid":1,"version":"`+v+`"}`)
		rep := Run(o)
		c := vsDocWant(t, rep, "worker")
		if !c.OK || c.Detail != "running, version "+v || !rep.OK {
			t.Fatalf("version %s: %+v ok=%v", v, c, rep.OK)
		}
	}
}

func TestVsDocWorkerNoLockNoCheck(t *testing.T) {
	o := vsDocOptions(t)
	if c, ok := vsDocCheck(Run(o), "worker"); ok {
		t.Fatalf("worker check with no lock held: %+v", c)
	}
}
