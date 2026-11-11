package routine

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/gridhome"
)

// urBackend plans one timer file per routine and one command per phase.
type urBackend struct{ home string }

func (b urBackend) Plan(sp TimerSpec) (TimerPlan, error) {
	return TimerPlan{
		Label:       sp.label(),
		Project:     sp.Project,
		Name:        sp.Name,
		Files:       []TimerFile{{Path: filepath.Join(b.home, "timers", sp.Name+".timer"), Content: marker(sp.label()) + "\n"}},
		Install:     []OSCommand{{Program: "install"}},
		Remove:      []OSCommand{{Program: "remove"}},
		AfterRemove: []OSCommand{{Program: "after"}},
	}, nil
}

// urRunner records every call and fails only for the program named in failOn.
type urRunner struct {
	calls  []string
	failOn string
}

func (r *urRunner) Run(_ context.Context, c OSCommand) error {
	r.calls = append(r.calls, c.Program)
	if c.Program == r.failOn {
		return errors.New("ur " + c.Program)
	}
	return nil
}

type urFixture struct {
	s      Scheduler
	store  Store
	runner *urRunner
	// stateDir is the folder of ownership records; statePath is daily's record.
	stateDir, statePath, timerPath string
}

// urSetup builds the scheduler. With install it schedules daily, then forgets the setup's calls.
func urSetup(t *testing.T, install bool) urFixture {
	t.Helper()
	home := t.TempDir()
	gh := gridhome.Real(filepath.Join(home, "gh"))
	if err := os.MkdirAll(gh, 0o700); err != nil {
		t.Fatal(err)
	}
	store := Store{GridHome: gh}
	if _, err := store.Add("daily", "1h", "grid version"); err != nil {
		t.Fatal(err)
	}
	runner := &urRunner{}
	s := Scheduler{Home: home, GridHome: store.GridHome, Binary: filepath.Join(home, "grid"), Backend: urBackend{home: home}, Runner: runner}
	statePath, err := s.statePath(timerLabel(store.GridHome, "", "daily"))
	if err != nil {
		t.Fatal(err)
	}
	if install {
		if err := s.Schedule(context.Background(), store); err != nil {
			t.Fatal(err)
		}
		runner.calls = nil
	}
	return urFixture{s: s, store: store, runner: runner, stateDir: filepath.Dir(statePath), statePath: statePath, timerPath: filepath.Join(home, "timers", "daily.timer")}
}

func urWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func urExists(path string) bool {
	_, err := os.Lstat(path)
	return err == nil
}

func urWantErr(t *testing.T, err error, want string) {
	t.Helper()
	if err == nil || err.Error() != want {
		t.Fatalf("error = %v, want %q", err, want)
	}
}

func urWantCalls(t *testing.T, got []string, want ...string) {
	t.Helper()
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("runner calls = %v, want %v", got, want)
	}
}

func urWantSyntax(t *testing.T, err error) {
	t.Helper()
	var syn *json.SyntaxError
	if !errors.As(err, &syn) {
		t.Fatalf("error = %v (%T), want *json.SyntaxError", err, err)
	}
}

func TestUnscheduleReadDirFailure(t *testing.T) {
	f := urSetup(t, false)
	if err := os.MkdirAll(filepath.Dir(f.stateDir), 0o700); err != nil {
		t.Fatal(err)
	}
	urWrite(t, f.stateDir, "not a folder")
	if err := f.s.Unschedule(context.Background(), "", ""); err == nil {
		t.Fatal("Unschedule with a file in place of the state folder returned nil")
	}
}

func TestUnscheduleSkipsOtherEntries(t *testing.T) {
	f := urSetup(t, true)
	readme := filepath.Join(f.stateDir, "README")
	other := filepath.Join(f.stateDir, LabelPrefix+"ur-other.json")
	urWrite(t, readme, "keep")
	urWrite(t, other, `{"label":"x","project":"other","name":"daily"}`)
	if err := f.s.Unschedule(context.Background(), "", ""); err != nil {
		t.Fatal(err)
	}
	urWantCalls(t, f.runner.calls, "remove", "after")
	if !urExists(readme) || !urExists(other) {
		t.Fatal("README or the other-project record was removed")
	}
	if urExists(f.statePath) {
		t.Fatal("daily's state file is still there")
	}
}

func TestUnscheduleBadRecord(t *testing.T) {
	f := urSetup(t, true)
	urWrite(t, f.statePath, "{")
	urWantSyntax(t, f.s.Unschedule(context.Background(), "", ""))
}

func TestUnscheduleNamelessRecord(t *testing.T) {
	f := urSetup(t, true)
	urWrite(t, f.statePath, `{"label":"`+timerLabel(f.store.GridHome, "", "daily")+`","project":"","name":""}`)
	urWantErr(t, f.s.Unschedule(context.Background(), "", ""), "timer manifest has no routine name")
}

func TestUnscheduleUnreadableRecord(t *testing.T) {
	f := urSetup(t, false)
	if err := os.MkdirAll(filepath.Join(f.stateDir, LabelPrefix+"ur-dir.json"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := f.s.Unschedule(context.Background(), "", ""); err == nil {
		t.Fatal("Unschedule with a folder in place of a record returned nil")
	}
	urWantCalls(t, f.runner.calls)
}

func TestUnscheduleRefusesForeignTimerFile(t *testing.T) {
	const want = "refusing timer file not created by grid routine: daily.timer"
	f := urSetup(t, true)
	urWrite(t, f.timerPath, "foreign")
	urWantErr(t, f.s.Unschedule(context.Background(), "", "daily"), want)
	urWantErr(t, f.s.Unschedule(context.Background(), "", ""), want)
	if !urExists(f.statePath) || !urExists(f.timerPath) {
		t.Fatal("state file or foreign file was removed")
	}
	b, err := os.ReadFile(f.timerPath)
	if err != nil || string(b) != "foreign" {
		t.Fatalf("foreign file = %q, %v", b, err)
	}
	urWantCalls(t, f.runner.calls)
}

func TestRemoveRefusals(t *testing.T) {
	f := urSetup(t, true)
	urWantErr(t, f.s.Unschedule(context.Background(), "", "bad name"), "invalid routine or project name")
	urWrite(t, f.statePath, "{")
	urWantSyntax(t, f.s.Unschedule(context.Background(), "", "daily"))
	urWantCalls(t, f.runner.calls)
}

func TestRemoveAfterRemoveFailureKeepsStateForRetry(t *testing.T) {
	f := urSetup(t, true)
	f.runner.failOn = "after"
	urWantErr(t, f.s.Unschedule(context.Background(), "", "daily"), "ur after")
	urWantCalls(t, f.runner.calls, "remove", "after")
	if urExists(f.timerPath) {
		t.Fatal("timer file is still there")
	}
	if !urExists(f.statePath) {
		t.Fatal("state file is gone; a retry could not finish")
	}
	f.runner.failOn = ""
	if err := f.s.Unschedule(context.Background(), "", "daily"); err != nil {
		t.Fatal(err)
	}
	if urExists(f.statePath) {
		t.Fatal("state file is still there after the retry")
	}
}

func TestRemoveRoutineRefusals(t *testing.T) {
	f := urSetup(t, true)
	ctx := context.Background()
	urWantErr(t, f.s.RemoveRoutine(ctx, f.store, "bad name"), "invalid routine or project name")
	urWrite(t, f.timerPath, "foreign")
	urWantErr(t, f.s.RemoveRoutine(ctx, f.store, "daily"), "refusing timer file not created by grid routine: daily.timer")
	if _, err := f.store.Get("daily"); err != nil {
		t.Fatalf("routine was removed: %v", err)
	}
}

func TestPreviewRefusals(t *testing.T) {
	f := urSetup(t, false)
	urWantPlans := func(plans []TimerPlan, err error) {
		t.Helper()
		if err != nil || len(plans) != 0 {
			t.Fatalf("plans = %v, err = %v, want none and nil", plans, err)
		}
	}

	_, err := f.s.Preview(Store{GridHome: f.store.GridHome, Project: "bad name"}, false)
	urWantErr(t, err, "project level needs a valid project name")

	rel := f.s
	rel.Home = "rel"
	_, err = rel.Preview(f.store, false)
	urWantErr(t, err, "timer home, Grid home and executable must be absolute")

	e, err := f.store.Get("daily")
	if err != nil {
		t.Fatal(err)
	}
	const other = "01ARZ3NDEKTSV4RRFFQ69G5FAV"
	if e.Machine == other {
		t.Fatal("fixture machine equals the other machine")
	}
	err = f.store.brain().EditConfig(f.store.level(), "routines", "daily", func(old []byte) ([]byte, error) {
		return []byte(strings.Replace(string(old), "machine: "+e.Machine, "machine: "+other, 1)), nil
	})
	if err != nil {
		t.Fatal(err)
	}
	urWantPlans(f.s.Preview(f.store, false))
}
