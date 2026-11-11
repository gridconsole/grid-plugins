package migrate

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gridconsole/grid-plugins/internal/gridhome"
	"github.com/gridconsole/grid-plugins/internal/project"
	"github.com/gridconsole/grid-plugins/internal/testenv"
)

// The crash tests run Apply in a copy of the test binary that exits, without running
// a single defer, in the hook after a chosen step: what a killed process leaves.
// TestCrashHelper is that copy's entry point.
func TestCrashHelper(t *testing.T) {
	step := os.Getenv("GRID_CRASH_STEP")
	if step == "" {
		t.Skip("entry point of the crash tests")
	}
	afterStep = func(s string) {
		if s == step {
			os.Exit(crashExit)
		}
	}
	if os.Getenv("GRID_CRASH_OPERATION") == "rollback" {
		if _, err := Rollback(os.Getenv("GRID_CRASH_HOME")); err != nil {
			t.Fatal(err)
		}
		return
	}
	// Scan reads no process variable: this copy passes CLAUDE_PLUGIN_DATA the way
	// grid migrate does, from the environment the test gave it.
	var opts []Option
	if data := os.Getenv("CLAUDE_PLUGIN_DATA"); data != "" {
		opts = append(opts, WithPluginData(data))
	}
	p, err := Scan(os.Getenv("GRID_CRASH_FROM"), os.Getenv("GRID_CRASH_HOME"), opts...)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := p.Apply(); err != nil {
		t.Fatal(err)
	}
}

const crashExit = 77

// crashAt runs a migration that dies right after step.
func crashAt(t *testing.T, step, from, home string, env ...string) {
	t.Helper()
	cmd := exec.Command(os.Args[0], "-test.run=^TestCrashHelper$", "-test.count=1")
	cmd.Env = append(testenv.Temp(t, os.Environ()), "GRID_CRASH_STEP="+step, "GRID_CRASH_FROM="+from, "GRID_CRASH_HOME="+home)
	cmd.Env = append(cmd.Env, env...)
	out, err := cmd.CombinedOutput()
	var ee *exec.ExitError
	if !errors.As(err, &ee) || ee.ExitCode() != crashExit {
		t.Fatalf("the run did not stop after %q: %v\n%s", step, err, out)
	}
}

var applySteps = []string{"locked", "journal-preparing", "backed-up", "staged", "journal-publishing", "displaced", "published", "receipt-written", "previous-removed", "done"}

// crashLayout builds a source and a home in root, with a fixed machine id so that the
// result of an interrupted migration can be compared with a clean one.
type crashLayout struct {
	name  string
	build func(t *testing.T, root string) (from, home string)
}

func crashLayouts() []crashLayout {
	const machine = "01MACHINEMACHINEMACHINE00\n"
	return []crashLayout{
		{"old home in place", func(t *testing.T, root string) (string, string) {
			from, _ := fixtureIn(t, root)
			remainingFixture(t, from)
			put(t, filepath.Join(from, "machine"), machine)
			return from, from
		}},
		{"separate home that holds a machine file", func(t *testing.T, root string) (string, string) {
			from, home := fixtureIn(t, root)
			put(t, filepath.Join(home, "machine"), machine)
			return from, home
		}},
		{"home does not exist yet", func(t *testing.T, root string) (string, string) {
			from, home := fixtureIn(t, root)
			put(t, filepath.Join(from, "machine"), machine)
			return from, home
		}},
	}
}

// migrated is the home as a clean migration leaves it, without the log, which names
// the backup. Paths in the files are the same in every run because the root is.
func migrated(t *testing.T, root, home string) map[string]string {
	t.Helper()
	return without(viewAs(t, home, strings.NewReplacer(root, "<ROOT>")), "migration.log")
}

func cleanRun(t *testing.T, l crashLayout, root string) map[string]string {
	t.Helper()
	wipe(t, root)
	from, home := l.build(t, root)
	if _, err := scan(t, from, home).Apply(); err != nil {
		t.Fatal(err)
	}
	return migrated(t, root, home)
}

func TestCrashAtEveryStepThenRerun(t *testing.T) {
	for _, l := range crashLayouts() {
		t.Run(l.name, func(t *testing.T) {
			t.Parallel()
			root := gridhome.Real(t.TempDir())
			want := cleanRun(t, l, root)
			for _, step := range applySteps {
				t.Run(step, func(t *testing.T) {
					wipe(t, root)
					from, home := l.build(t, root)
					hadHome := exists(home)
					if step == "displaced" && !hadHome {
						t.Skip("there is no old home to set aside")
					}
					var oldHome map[string]string
					if hadHome {
						oldHome = view(t, home)
					}
					crashAt(t, step, from, home)
					checkOneHomeSurvives(t, step, home, hadHome, oldHome)

					// A dry run says what it found and changes nothing.
					state := snapshotState(t, root)
					p := scan(t, from, home)
					if journaled := step != "locked" && step != "done"; journaled != (p.Interrupted != "") {
						t.Fatalf("interrupted run noticed = %v after a stop at %q", p.Interrupted != "", step)
					}
					if p.Interrupted != "" {
						var b strings.Builder
						if err := p.WriteText(&b); err != nil || !strings.Contains(b.String(), "interrupted") {
							t.Fatalf("plan text: %q %v", b.String(), err)
						}
					}
					if got := snapshotState(t, root); got != state {
						t.Fatalf("a dry run changed the disk:\n%s\n%s", state, got)
					}

					first := finishAfterCrash(t, from, home)
					if journaled := step != "locked" && step != "done"; journaled && (first.Recovered == "" || first.Rerun == first.Changed) {
						t.Fatalf("recovery after %q: %+v", step, first)
					}
					if got := migrated(t, root, home); !sameView(got, want) {
						t.Fatalf("the home differs from a clean migration:\nwant %v\n got %v", want, got)
					}
					for _, s := range siblings(t, home) {
						if strings.Contains(s, ".migrate-") && !strings.HasSuffix(s, ".migrate-lock") {
							t.Errorf("left behind: %s", s)
						}
					}
					entries, _ := os.ReadDir(home + "-backup")
					if len(entries) == 0 {
						t.Fatal("no backup")
					}
					for _, e := range entries {
						if !exists(filepath.Join(home+"-backup", e.Name(), "manifest.json")) {
							t.Errorf("incomplete backup kept: %s", e.Name())
						}
					}
				})
			}
		})
	}
}

// finishAfterCrash is what a person does: run migrate until it has nothing left to do.
// A run that only undid the earlier one is followed by one that migrates. It returns
// the result of the first run.
func finishAfterCrash(t *testing.T, from, home string, opts ...Option) (first Result) {
	t.Helper()
	for i := 0; i < 3; i++ {
		p, err := Scan(from, home, opts...)
		if err != nil {
			t.Fatalf("scan after the crash: %v", err)
		}
		if p.Current {
			return first
		}
		res, err := p.Apply()
		if err != nil {
			t.Fatalf("apply after the crash: %v", err)
		}
		if i == 0 {
			first = res
		}
	}
	t.Fatal("migrate never finished")
	return first
}

// checkOneHomeSurvives is the promise: after a crash the old home or the new home is
// in place. The one gap is between the two renames, when the old home sits beside the
// new one under the names the journal gives.
func checkOneHomeSurvives(t *testing.T, step, home string, hadHome bool, oldHome map[string]string) {
	t.Helper()
	publishedSteps := map[string]bool{"published": true, "receipt-written": true, "previous-removed": true, "done": true}
	switch {
	case !exists(home):
		if hadHome && step != "displaced" || !hadHome && publishedSteps[step] {
			t.Fatalf("no home after a stop at %q", step)
		}
		if step == "displaced" && (len(globOne(home+".migrate-previous-*")) != 1 || len(globOne(home+".migrate-stage-*")) != 1) {
			t.Fatal("the swap was cut but the old and new homes are not both beside it")
		}
	case !exists(filepath.Join(home, "VERSION")):
		if hadHome && !sameView(oldHome, view(t, home)) {
			t.Fatalf("the old home changed at %q", step)
		}
	default:
		if read(t, home, "VERSION") != "1\n" || exists(filepath.Join(home, "alpha/projects")) {
			t.Fatal("the new home is not complete")
		}
	}
}

func globOne(pattern string) []string {
	m, _ := filepath.Glob(pattern)
	return m
}

// snapshotState describes the folder around the home, for "a dry run changes nothing".
func snapshotState(t *testing.T, dir string) string {
	t.Helper()
	var b strings.Builder
	filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		info, _ := d.Info()
		rel, _ := filepath.Rel(dir, path)
		b.WriteString(rel + " " + info.Mode().String() + " " + info.ModTime().String() + "\n")
		return nil
	})
	return b.String()
}

// A crash inside the recovery is recovered from too.
func TestCrashDuringRecovery(t *testing.T) {
	for _, l := range crashLayouts()[:2] {
		root := gridhome.Real(t.TempDir())
		want := cleanRun(t, l, root)
		t.Run(l.name, func(t *testing.T) {
			wipe(t, root)
			from, home := l.build(t, root)
			crashAt(t, "displaced", from, home)
			crashAt(t, "recover-renamed", from, home)
			if first := finishAfterCrash(t, from, home); !first.Changed || first.Recovered == "" {
				t.Fatalf("finish: %+v", first)
			}
			if got := migrated(t, root, home); !sameView(got, want) {
				t.Fatal("the home differs from a clean migration")
			}
		})
	}
}

// With the staged copy gone and the old home set aside, recovery puts the old home
// back, and a later run migrates it.
func TestRecoveryPutsTheOldHomeBack(t *testing.T) {
	l := crashLayouts()[1]
	root := gridhome.Real(t.TempDir())
	want := cleanRun(t, l, root)
	wipe(t, root)
	from, home := l.build(t, root)
	old := view(t, home)
	crashAt(t, "displaced", from, home)
	if err := os.RemoveAll(globOne(home + ".migrate-stage-*")[0]); err != nil {
		t.Fatal(err)
	}
	p := scan(t, from, home)
	res, err := p.Apply()
	if err != nil || !res.Rerun || res.Changed {
		t.Fatalf("recovery: %+v %v", res, err)
	}
	if !sameView(old, view(t, home)) {
		t.Fatal("the old home is not back as it was")
	}
	finishAfterCrash(t, from, home)
	if got := migrated(t, root, home); !sameView(got, want) {
		t.Fatal("the home differs from a clean migration")
	}
}

// A machine id made at apply and lost with a staged copy is made again, once.
func TestMachineIDAfterCrashBeforePublish(t *testing.T) {
	from, home := fixture(t)
	crashAt(t, "staged", from, home)
	finishAfterCrash(t, from, home)
	machine := gridhome.Machine(home)
	if !ulidRE.MatchString(machine) {
		t.Fatalf("machine %q", machine)
	}
	recs, err := project.Load(home)
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range recs {
		if len(r.FoldersOn(machine)) != 1 {
			t.Errorf("%s: folders %+v do not match machine %s", r.Name, r.Folders, machine)
		}
	}
}

func TestInterruptedRunIsReportedNotRunByAScan(t *testing.T) {
	from, home := fixture(t)
	put(t, filepath.Join(home, "machine"), "01MACHINEMACHINEMACHINE00\n")
	crashAt(t, "displaced", from, home)
	p, err := Scan(from, home)
	if err != nil || p.Interrupted == "" {
		t.Fatalf("scan: %+v %v", p, err)
	}
	if exists(home) {
		t.Fatal("a scan restored the home")
	}
}

func TestUnexplainedLeftoversAreNotGuessedAt(t *testing.T) {
	from, home := fixture(t)
	put(t, home+".migrate-journal", "not json")
	if _, err := Scan(from, home); err == nil || !strings.Contains(err.Error(), "journal") {
		t.Fatalf("scan: %v", err)
	}
	os.Remove(home + ".migrate-journal")
	if err := os.MkdirAll(home+".migrate-previous-abc", 0700); err != nil {
		t.Fatal(err)
	}
	if _, err := Scan(from, home); err == nil || !strings.Contains(err.Error(), "by hand") {
		t.Fatalf("scan: %v", err)
	}
	// A journal whose folders are in a state no step explains stops the run and
	// changes nothing.
	os.Remove(home + ".migrate-previous-abc")
	j := journal{Version: 1, Phase: phasePublishing, Source: from, Home: home, Backup: home + "-backup/x", Stage: home + ".migrate-stage-x", Previous: home + ".migrate-previous-x", HadHome: true}
	if err := writeJournal(j); err != nil {
		t.Fatal(err)
	}
	before := snapshotState(t, home)
	if _, err := Scan(from, home); err == nil || !strings.Contains(err.Error(), "Nothing was changed") {
		t.Fatalf("scan: %v", err)
	}
	if snapshotState(t, home) != before {
		t.Fatal("the disk changed")
	}
}

func TestLockOwnership(t *testing.T) {
	from, home := fixture(t)
	l, err := acquireLock(lockPath(home))
	if err != nil {
		t.Fatal(err)
	}
	defer l.release()
	// A second migrate says who holds the lock instead of waiting for it.
	done := make(chan error, 1)
	go func() {
		_, err := Scan(from, home)
		if err == nil {
			p, _ := Scan(from, home)
			_, err = p.Apply()
		}
		done <- err
	}()
	select {
	case err := <-done:
		if err == nil || !strings.Contains(err.Error(), "another grid migrate is running") {
			t.Fatalf("second migrate: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("a second migrate waited for the lock")
	}
	if _, err := Rollback(home); err == nil || !strings.Contains(err.Error(), "another grid migrate is running") {
		t.Fatalf("rollback: %v", err)
	}
	l.release()
	l, err = acquireLock(lockPath(home))
	if err != nil {
		t.Fatalf("after release: %v", err)
	}
	l.release()
}
