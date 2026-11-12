package cli

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/routine"
)

// vuRmRunner records every OS command as "program args..." and fails the ones whose
// text contains failOn.
type vuRmRunner struct {
	calls  []string
	failOn string
}

func (r *vuRmRunner) Run(_ context.Context, c routine.OSCommand) error {
	line := c.Program + " " + strings.Join(c.Args, " ")
	r.calls = append(r.calls, line)
	if r.failOn != "" && strings.Contains(line, r.failOn) {
		return errors.New("vu os refused: " + line)
	}
	return nil
}

type vuRmFixture struct {
	h      *harness
	runner *vuRmRunner
	binary string
}

// vuRmSetup registers project alpha, schedules the named routines of alpha and one
// routine of beta with the real systemd planner and a recording runner, and wires the
// same pair into the Env seam of grid project remove. The setup's own calls are forgotten.
func vuRmSetup(t *testing.T, alphaRoutines ...string) vuRmFixture {
	t.Helper()
	h := ynSpNew(t)
	h.write(filepath.Join(h.gridHome, "machine"), "01ARZ3NDEKTSV4RRFFQ69G5FAV\n")
	folder := filepath.Join(h.root, "work", "alpha")
	h.mkdir(folder)
	if code := h.run(folder, "project", "add", folder, "--name", "alpha"); code != 0 {
		t.Fatalf("project add exit %d: %s", code, h.errb)
	}
	runner := &vuRmRunner{}
	f := vuRmFixture{h: h, runner: runner, binary: filepath.Join(h.home, "bin", "grid")}
	for project, names := range map[string][]string{"alpha": alphaRoutines, "beta": {"weekly"}} {
		store := routine.Store{GridHome: h.gridHome, Project: project}
		for _, name := range names {
			if _, err := store.Add(name, "1h", "grid version"); err != nil {
				t.Fatal(err)
			}
		}
		if err := f.scheduler().Schedule(context.Background(), store); err != nil {
			t.Fatal(err)
		}
	}
	runner.calls = nil
	h.env.timerOS = func() (routine.Backend, routine.OSRunner) { return routine.Systemd{}, runner }
	return f
}

func (f vuRmFixture) scheduler() routine.Scheduler {
	return routine.Scheduler{Home: f.h.home, GridHome: f.h.gridHome, Binary: f.binary, Backend: routine.Systemd{}, Runner: f.runner}
}

// removeCalls is what the OS scheduler is asked for when one timer comes down.
func (f vuRmFixture) removeCalls(t *testing.T, project, name string) []string {
	t.Helper()
	p, err := routine.Systemd{}.Plan(routine.TimerSpec{Home: f.h.home, GridHome: f.h.gridHome, Project: project, Binary: f.binary, Name: name})
	if err != nil {
		t.Fatal(err)
	}
	var calls []string
	for _, c := range append(p.Remove, p.AfterRemove...) {
		calls = append(calls, c.Program+" "+strings.Join(c.Args, " "))
	}
	return calls
}

// manifests lists the ownership records of timers on this machine.
func (f vuRmFixture) manifests(t *testing.T) []string {
	t.Helper()
	entries, err := os.ReadDir(filepath.Join(f.h.gridHome, "local", "routine-timers"))
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	return names
}

func (f vuRmFixture) wantCalls(t *testing.T, want ...string) {
	t.Helper()
	if strings.Join(f.runner.calls, "\n") != strings.Join(want, "\n") {
		t.Fatalf("OS calls =\n%s\nwant\n%s", strings.Join(f.runner.calls, "\n"), strings.Join(want, "\n"))
	}
}

func vuRmDecode(t *testing.T, raw []byte) (unscheduled []string, folders []any) {
	t.Helper()
	var answer struct {
		Name        string   `json:"name"`
		Folders     []any    `json:"folders"`
		Unscheduled []string `json:"unscheduled"`
	}
	if err := json.Unmarshal(raw, &answer); err != nil {
		t.Fatalf("answer is not JSON: %v\n%s", err, raw)
	}
	if answer.Name != "alpha" {
		t.Fatalf("answer names %q, want alpha: %s", answer.Name, raw)
	}
	return answer.Unscheduled, answer.Folders
}

func TestVuRmRemoveTakesDownTheProjectsTimer(t *testing.T) {
	f := vuRmSetup(t, "daily")
	if len(f.manifests(t)) != 2 {
		t.Fatalf("setup manifests = %v, want alpha's and beta's", f.manifests(t))
	}
	if code := f.h.run(f.h.root, "project", "remove", "alpha", "--json"); code != 0 {
		t.Fatalf("exit %d: %s", code, f.h.errb)
	}
	out := f.h.out.Bytes()
	if err := contract.Validate("cmd-project-remove", out); err != nil {
		t.Fatalf("cmd-project-remove rejects the answer: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), `"unscheduled":["daily"]`) {
		t.Fatalf("answer has no \"unscheduled\":[\"daily\"]: %s", out)
	}
	unscheduled, folders := vuRmDecode(t, out)
	if len(unscheduled) != 1 || len(folders) != 0 {
		t.Fatalf("unscheduled = %v, folders = %v", unscheduled, folders)
	}
	f.wantCalls(t, f.removeCalls(t, "alpha", "daily")...)
	if got := f.manifests(t); len(got) != 1 {
		t.Fatalf("manifests after remove = %v, want only beta's", got)
	}
	// The definition stays in the project's brain config, so a restore can schedule it again.
	if _, err := (routine.Store{GridHome: f.h.gridHome, Project: "alpha"}).Get("daily"); err != nil {
		t.Fatalf("alpha's routine definition is gone: %v", err)
	}
	if strings.Contains(f.h.errb.String(), "grid:") {
		t.Fatalf("stderr: %s", f.h.errb)
	}
}

func TestVuRmRemoveWithoutTimersAnswersEmptyList(t *testing.T) {
	f := vuRmSetup(t)
	if code := f.h.run(f.h.root, "project", "remove", "alpha", "--json"); code != 0 {
		t.Fatalf("exit %d: %s", code, f.h.errb)
	}
	out := f.h.out.Bytes()
	if err := contract.Validate("cmd-project-remove", out); err != nil {
		t.Fatalf("cmd-project-remove rejects the answer: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), `"unscheduled":[]`) {
		t.Fatalf("answer has no \"unscheduled\":[]: %s", out)
	}
	f.wantCalls(t)
	if got := f.manifests(t); len(got) != 1 {
		t.Fatalf("beta's manifest is gone: %v", got)
	}
}

func TestVuRmRemoveTextListsEachTimer(t *testing.T) {
	f := vuRmSetup(t, "daily", "nightly")
	if code := f.h.run(f.h.root, "project", "remove", "alpha"); code != 0 {
		t.Fatalf("exit %d: %s", code, f.h.errb)
	}
	out := f.h.out.String()
	if !strings.HasPrefix(out, "project: alpha\nid: ") || !strings.HasSuffix(out, "\nunscheduled: daily\nunscheduled: nightly\n") {
		t.Fatalf("plain answer = %q", out)
	}
}

func TestVuRmRemoveKeepsTheProjectRemovedWhenTheOSRefuses(t *testing.T) {
	f := vuRmSetup(t, "daily")
	f.runner.failOn = "disable"
	if code := f.h.run(f.h.root, "project", "remove", "alpha", "--json"); code != 1 {
		t.Fatalf("exit %d, want 1: out %s err %s", code, f.h.out, f.h.errb)
	}
	stderr := f.h.errb.String()
	if !strings.Contains(stderr, "daily") || !strings.Contains(stderr, "alpha") || !strings.Contains(stderr, "vu os refused") {
		t.Fatalf("stderr does not name the project, the routine and the cause: %q", stderr)
	}
	if got, _ := vuRmDecode(t, f.h.out.Bytes()); len(got) != 0 {
		t.Fatalf("answer lists %v as down; none is", got)
	}

	if code := f.h.run(f.h.root, "project", "list", "--json"); code != 0 {
		t.Fatalf("list exit %d: %s", code, f.h.errb)
	}
	var list []struct {
		Name    string `json:"name"`
		Folders []any  `json:"folders"`
	}
	if err := json.Unmarshal(f.h.out.Bytes(), &list); err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].Name != "alpha" || len(list[0].Folders) != 0 {
		t.Fatalf("project list = %s, want alpha still listed with no folder on this machine", f.h.out)
	}

	// The manifest is kept, so running remove's unschedule again finishes the job.
	f.runner.failOn, f.runner.calls = "", nil
	names, err := f.scheduler().UnscheduleProject(context.Background(), "alpha")
	if err != nil || len(names) != 1 || names[0] != "daily" {
		t.Fatalf("retry = %v, %v", names, err)
	}
}
