package cli

import (
	"fmt"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/fslock"
)

// vqRunRefusal is the whole of stderr when grid migrate --apply stops for a running app.
const vqRunRefusal = "grid migrate: The Grid app is running on ~/.grid. Quit the Grid app, then run grid migrate --apply again.\n"

// vqRunStatusJSON is what the app's worker writes to worker-status.json (the shape the
// worker package reads).
const vqRunStatusJSON = `{"pid":4242,"version":"0.9.1-test","started_at":"2026-11-08T09:00:00.123456789Z","uptime_seconds":61,"address":"127.0.0.1:43111"}`

// vqRunHarness is the harness with no GRID_APP_URL: appBaseURL reads the process
// environment first, so a live address there would stand for a running app.
func vqRunHarness(t *testing.T) *harness {
	t.Helper()
	t.Setenv("GRID_APP_URL", "")
	return newHarness(t)
}

// vqRunLegacy writes the in-place layout of TestMigrateInPlaceDefault.
func vqRunLegacy(h *harness) {
	h.t.Helper()
	h.write(filepath.Join(h.gridHome, "legacy", "projects", "example", "project.md"), "---\nfolder: /example/work/project\n---\n")
}

// vqRunHold creates worker.lock in the Grid home and holds it from a second handle,
// as the app's worker does while it runs. The handle closes when the test ends.
func vqRunHold(h *harness) {
	h.t.Helper()
	f, err := fslock.OpenNoFollow(filepath.Join(h.gridHome, "worker.lock"), os.O_RDWR|os.O_CREATE, 0o600)
	if err != nil {
		h.t.Fatal(err)
	}
	if err := fslock.TryLockExclusive(f); err != nil {
		f.Close()
		h.t.Fatal(err)
	}
	h.t.Cleanup(func() { f.Close() })
}

// vqRunLiveURL starts a server and returns its address; it closes with the test.
func vqRunLiveURL(t *testing.T) string {
	t.Helper()
	srv := httptest.NewServer(http.NotFoundHandler())
	t.Cleanup(srv.Close)
	return srv.URL
}

// vqRunDeadURL returns the address of a server that is already closed.
func vqRunDeadURL(t *testing.T) string {
	t.Helper()
	srv := httptest.NewServer(http.NotFoundHandler())
	url := srv.URL
	srv.Close()
	return url
}

// vqRunSnapshot records every path under root with its kind and, for a file, its bytes.
func vqRunSnapshot(t *testing.T, root string) map[string]string {
	t.Helper()
	snap := map[string]string{}
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		switch {
		case d.Type()&fs.ModeSymlink != 0:
			target, err := os.Readlink(path)
			if err != nil {
				return err
			}
			snap[rel] = "link:" + target
		case d.IsDir():
			snap[rel] = "dir"
		case d.Type().IsRegular():
			b, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			snap[rel] = "file:" + string(b)
		default:
			snap[rel] = "other:" + d.Type().String()
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return snap
}

// vqRunDiff lists the paths that were added, removed or changed between two snapshots.
func vqRunDiff(before, after map[string]string) []string {
	var out []string
	for p, b := range before {
		if a, ok := after[p]; !ok {
			out = append(out, "removed "+p)
		} else if a != b {
			out = append(out, "changed "+p)
		}
	}
	for p := range after {
		if _, ok := before[p]; !ok {
			out = append(out, "added "+p)
		}
	}
	sort.Strings(out)
	return out
}

// vqRunSaw is what a run printed, for a failure message.
func vqRunSaw(h *harness, code int) string {
	return fmt.Sprintf("exit %d\nstdout:\n%s\nstderr:\n%s", code, h.out, h.errb)
}

// vqRunWantRefused checks a run that stopped: exit 1, the one-line refusal on stderr,
// the plan on stdout, and nothing under the root changed since the snapshot.
func vqRunWantRefused(t *testing.T, h *harness, before map[string]string, code int) {
	t.Helper()
	if code != 1 || h.errb.String() != vqRunRefusal {
		t.Fatalf("want exit 1 and stderr %q\n%s", vqRunRefusal, vqRunSaw(h, code))
	}
	if !strings.Contains(h.out.String(), "0 conflicts, 0 unmapped") {
		t.Fatalf("the plan was not printed before the refusal\n%s", vqRunSaw(h, code))
	}
	if d := vqRunDiff(before, vqRunSnapshot(t, h.root)); len(d) > 0 {
		t.Fatalf("the refused run changed the tree (backup, journal or swap): %v\n%s", d, vqRunSaw(h, code))
	}
}

// vqRunWantApplied checks a run that migrated the in-place legacy layout.
func vqRunWantApplied(t *testing.T, h *harness, code int) {
	t.Helper()
	if code != 0 || !strings.Contains(h.out.String(), "migrate: applied\n") {
		t.Fatalf("want exit 0 and migrate: applied\n%s", vqRunSaw(h, code))
	}
	if _, err := os.Stat(filepath.Join(h.gridHome, "projects", "example", "project.md")); err != nil {
		t.Fatalf("applied, but the project is missing: %v", err)
	}
}

// The worker holds worker.lock: --apply stops before any backup or swap.
func TestVqRunHeldWorkerLockRefusesApply(t *testing.T) {
	h := vqRunHarness(t)
	vqRunLegacy(h)
	vqRunHold(h)
	before := vqRunSnapshot(t, h.root)
	code := h.run(h.root, "migrate", "--apply")
	vqRunWantRefused(t, h, before, code)
}

// The server answers at the address in app-url: --apply stops, with no worker.lock.
func TestVqRunListeningAppURLRefusesApply(t *testing.T) {
	h := vqRunHarness(t)
	vqRunLegacy(h)
	h.write(filepath.Join(h.gridHome, AppURLFile), vqRunLiveURL(t)+"\n")
	before := vqRunSnapshot(t, h.root)
	code := h.run(h.root, "migrate", "--apply")
	vqRunWantRefused(t, h, before, code)
}

// app-url names a server that is gone: nothing runs, --apply goes ahead.
func TestVqRunDeadAppURLApplies(t *testing.T) {
	h := vqRunHarness(t)
	vqRunLegacy(h)
	h.write(filepath.Join(h.gridHome, AppURLFile), vqRunDeadURL(t)+"\n")
	code := h.run(h.root, "migrate", "--apply")
	vqRunWantApplied(t, h, code)
}

// A worker.lock no handle holds, with the status file the worker left (presence
// "stale"), is not a running app.
func TestVqRunStaleWorkerLockApplies(t *testing.T) {
	h := vqRunHarness(t)
	vqRunLegacy(h)
	h.write(filepath.Join(h.gridHome, "worker.lock"), "")
	h.write(filepath.Join(h.gridHome, "worker-status.json"), vqRunStatusJSON)
	code := h.run(h.root, "migrate", "--apply")
	vqRunWantApplied(t, h, code)
}

// --dry-run, and a run with no flag, show the plan and change nothing, app or not.
func TestVqRunDryRunWithHeldLock(t *testing.T) {
	h := vqRunHarness(t)
	vqRunLegacy(h)
	vqRunHold(h)
	h.write(filepath.Join(h.gridHome, AppURLFile), vqRunLiveURL(t)+"\n")
	before := vqRunSnapshot(t, h.root)
	for _, args := range [][]string{{"migrate", "--dry-run"}, {"migrate"}} {
		code := h.run(h.root, args...)
		if code != 0 || h.errb.Len() != 0 || !strings.Contains(h.out.String(), "0 conflicts, 0 unmapped") {
			t.Fatalf("%v: want exit 0, the plan, empty stderr\n%s", args, vqRunSaw(h, code))
		}
		if d := vqRunDiff(before, vqRunSnapshot(t, h.root)); len(d) > 0 {
			t.Fatalf("%v changed the tree: %v", args, d)
		}
	}
}

// A fresh home the app's worker started in holds only worker.lock and
// worker-status.json: there is nothing to migrate, running worker or not.
func TestVqRunWorkerFilesOnlyHomeHasNothingToMigrate(t *testing.T) {
	h := vqRunHarness(t)
	vqRunHold(h)
	h.write(filepath.Join(h.gridHome, "worker-status.json"), vqRunStatusJSON)
	before := vqRunSnapshot(t, h.root)
	for _, args := range [][]string{{"migrate"}, {"migrate", "--apply"}} {
		code := h.run(h.root, args...)
		if code != 0 || h.out.String() != "migrate: nothing to migrate\n" || h.errb.Len() != 0 {
			t.Fatalf("%v:\n%s", args, vqRunSaw(h, code))
		}
		if d := vqRunDiff(before, vqRunSnapshot(t, h.root)); len(d) > 0 {
			t.Fatalf("%v changed the tree: %v", args, d)
		}
	}
}

// --from an old folder swaps the Grid home just the same: a fresh home with the
// worker running in it is refused.
func TestVqRunFromFolderRefusesWhileWorkerRuns(t *testing.T) {
	h := vqRunHarness(t)
	h.write(filepath.Join(h.root, "old", "projects", "example", "project.md"), "---\nfolder: /example/work/project\n---\n")
	vqRunHold(h)
	h.write(filepath.Join(h.gridHome, "worker-status.json"), vqRunStatusJSON)
	before := vqRunSnapshot(t, h.root)
	code := h.run(h.root, "migrate", "--apply", "--from", "old")
	vqRunWantRefused(t, h, before, code)
}

// The home label in the refusal is the Grid home as ~ shows it, whatever the folder is called.
func TestVqRunRefusalNamesTheHomeByTilde(t *testing.T) {
	h := vqRunHarness(t)
	h.env.GridHome = filepath.Join(h.home, "elsewhere", "grid")
	h.mkdir(h.env.GridHome)
	h.gridHome = h.env.GridHome
	vqRunLegacy(h)
	vqRunHold(h)
	code := h.run(h.root, "migrate", "--apply")
	want := "grid migrate: The Grid app is running on ~/elsewhere/grid. Quit the Grid app, then run grid migrate --apply again.\n"
	if code != 1 || h.errb.String() != want {
		t.Fatalf("want exit 1 and stderr %q\n%s", want, vqRunSaw(h, code))
	}
}
