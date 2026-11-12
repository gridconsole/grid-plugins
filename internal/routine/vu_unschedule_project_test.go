package routine

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/gridhome"
)

// vuUnsBackend is urBackend with one timer file per project and routine, and the
// project and routine in the remove commands, so a test sees whose timer was touched.
type vuUnsBackend struct{ home string }

func (b vuUnsBackend) Plan(sp TimerSpec) (TimerPlan, error) {
	p, err := urBackend{home: b.home}.Plan(sp)
	if err != nil {
		return p, err
	}
	p.Files[0].Path = filepath.Join(b.home, "timers", sp.Project+"-"+sp.Name+".timer")
	p.Remove = []OSCommand{{Program: "remove", Args: []string{sp.Project, sp.Name}}}
	p.AfterRemove = []OSCommand{{Program: "after", Args: []string{sp.Project, sp.Name}}}
	return p, nil
}

// vuUnsRunner records "program project name" for every call and fails the program in failOn.
type vuUnsRunner struct {
	calls  []string
	failOn string
}

func (r *vuUnsRunner) Run(_ context.Context, c OSCommand) error {
	r.calls = append(r.calls, c.Program+" "+strings.Join(c.Args, " "))
	if c.Program == r.failOn {
		return errors.New("vu " + c.Program)
	}
	return nil
}

type vuUnsFixture struct {
	s      Scheduler
	runner *vuUnsRunner
	home   string
}

// vuUnsSetup schedules the routines of each project (name -> routine names) and then
// forgets the setup's OS calls.
func vuUnsSetup(t *testing.T, projects map[string][]string) vuUnsFixture {
	t.Helper()
	home := gridhome.Real(t.TempDir())
	gh := gridhome.Real(filepath.Join(home, "gh"))
	if err := os.MkdirAll(gh, 0o700); err != nil {
		t.Fatal(err)
	}
	runner := &vuUnsRunner{}
	s := Scheduler{Home: home, GridHome: gh, Binary: filepath.Join(home, "grid"), Backend: vuUnsBackend{home: home}, Runner: runner}
	for project, names := range projects {
		store := Store{GridHome: gh, Project: project}
		for _, name := range names {
			if _, err := store.Add(name, "1h", "grid version"); err != nil {
				t.Fatal(err)
			}
		}
		if err := s.Schedule(context.Background(), store); err != nil {
			t.Fatal(err)
		}
	}
	runner.calls = nil
	return vuUnsFixture{s: s, runner: runner, home: home}
}

func (f vuUnsFixture) manifest(t *testing.T, project, name string) string {
	t.Helper()
	p, err := f.s.statePath(timerLabel(f.s.GridHome, project, name))
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func (f vuUnsFixture) timer(project, name string) string {
	return filepath.Join(f.home, "timers", project+"-"+name+".timer")
}

func vuUnsWantNames(t *testing.T, got []string, err error, want ...string) {
	t.Helper()
	if err != nil {
		t.Fatalf("UnscheduleProject error = %v", err)
	}
	if got == nil || strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("names = %#v, want %#v", got, want)
	}
}

func vuUnsSorted(in []string) []string {
	out := append([]string{}, in...)
	sort.Strings(out)
	return out
}

func TestVuUnsProjectTakesDownOnlyItsOwnTimers(t *testing.T) {
	ctx := context.Background()
	f := vuUnsSetup(t, map[string][]string{"alpha": {"nightly", "daily"}, "beta": {"weekly"}})
	for _, p := range [][2]string{{"alpha", "daily"}, {"alpha", "nightly"}, {"beta", "weekly"}} {
		if !urExists(f.manifest(t, p[0], p[1])) || !urExists(f.timer(p[0], p[1])) {
			t.Fatalf("setup left no manifest or timer file for %v", p)
		}
	}

	names, err := f.s.UnscheduleProject(ctx, "alpha")
	vuUnsWantNames(t, names, err, "daily", "nightly")

	want := []string{"after alpha daily", "after alpha nightly", "remove alpha daily", "remove alpha nightly"}
	if got := vuUnsSorted(f.runner.calls); strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("runner calls = %v, want %v", got, want)
	}
	for _, name := range []string{"daily", "nightly"} {
		if urExists(f.manifest(t, "alpha", name)) || urExists(f.timer("alpha", name)) {
			t.Fatalf("alpha %s still has a manifest or timer file", name)
		}
	}
	if !urExists(f.manifest(t, "beta", "weekly")) || !urExists(f.timer("beta", "weekly")) {
		t.Fatal("beta's manifest or timer file was removed")
	}

	f.runner.calls = nil
	names, err = f.s.UnscheduleProject(ctx, "alpha")
	vuUnsWantNames(t, names, err)
	urWantCalls(t, f.runner.calls)
	if !urExists(f.manifest(t, "beta", "weekly")) {
		t.Fatal("the second call removed beta's manifest")
	}
}

func TestVuUnsProjectWithoutTimerFolder(t *testing.T) {
	f := vuUnsSetup(t, nil)
	names, err := f.s.UnscheduleProject(context.Background(), "alpha")
	vuUnsWantNames(t, names, err)
	urWantCalls(t, f.runner.calls)
}

func TestVuUnsProjectFailureNamesTheRoutineAndKeepsItsManifest(t *testing.T) {
	ctx := context.Background()
	f := vuUnsSetup(t, map[string][]string{"alpha": {"daily"}})
	f.runner.failOn = "remove"
	names, err := f.s.UnscheduleProject(ctx, "alpha")
	var te *TimerError
	if !errors.As(err, &te) || te.Name != "daily" || err.Error() != "vu remove" {
		t.Fatalf("error = %v (%T), want a TimerError for daily with the OS error's text", err, err)
	}
	if len(names) != 0 {
		t.Fatalf("names = %v, want none: the timer is not down", names)
	}
	if !urExists(f.manifest(t, "alpha", "daily")) {
		t.Fatal("manifest is gone; a retry could not finish")
	}

	f.runner.failOn = ""
	names, err = f.s.UnscheduleProject(ctx, "alpha")
	vuUnsWantNames(t, names, err, "daily")
	if urExists(f.manifest(t, "alpha", "daily")) {
		t.Fatal("manifest is still there after the retry")
	}
}

func TestVuUnsProjectDoesNotCountAManifestItCannotOwn(t *testing.T) {
	f := vuUnsSetup(t, map[string][]string{"alpha": {"daily"}})
	// A record that names alpha's routine but sits under a label this scheduler does not
	// compute: remove finds no record of its own and does nothing.
	path := f.manifest(t, "alpha", "daily")
	stray := filepath.Join(filepath.Dir(path), LabelPrefix+"vu-stray.json")
	urWrite(t, stray, `{"label":"x","project":"alpha","name":"stray"}`)
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	names, err := f.s.UnscheduleProject(context.Background(), "alpha")
	vuUnsWantNames(t, names, err)
	urWantCalls(t, f.runner.calls)
	if !urExists(stray) {
		t.Fatal("the stray record was removed")
	}
}

func TestVuUnsProjectReturnsWhatWasDownBeforeAFailure(t *testing.T) {
	ctx := context.Background()
	f := vuUnsSetup(t, map[string][]string{"alpha": {"daily", "nightly"}})
	// Break one of the two records; the other is taken down whichever comes first, and
	// the broken one stops the walk with its own error.
	urWrite(t, f.manifest(t, "alpha", "nightly"), "{")
	names, err := f.s.UnscheduleProject(ctx, "alpha")
	urWantSyntax(t, err)
	if len(names) > 1 || (len(names) == 1 && names[0] != "daily") {
		t.Fatalf("names = %v, want at most daily", names)
	}
}

func TestVuUnsUnscheduleNamelessStillWalksTheProject(t *testing.T) {
	f := vuUnsSetup(t, map[string][]string{"alpha": {"daily"}, "beta": {"weekly"}})
	if err := f.s.Unschedule(context.Background(), "alpha", ""); err != nil {
		t.Fatal(err)
	}
	urWantCalls(t, f.runner.calls, "remove alpha daily", "after alpha daily")
	if urExists(f.manifest(t, "alpha", "daily")) || !urExists(f.manifest(t, "beta", "weekly")) {
		t.Fatal("Unschedule with no name did not take down alpha only")
	}
	f.runner.failOn = "remove"
	err := f.s.Unschedule(context.Background(), "beta", "")
	urWantErr(t, err, "vu remove")
	var te *TimerError
	if errors.As(err, &te) {
		t.Fatalf("Unschedule returns the wrapper %T; its error must stay the OS error", err)
	}
}
