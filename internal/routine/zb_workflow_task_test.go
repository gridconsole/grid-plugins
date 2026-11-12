package routine

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os/exec"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/gridconsole/grid-plugins/internal/gridhome"
)

// zbFlowStore is a project store whose clock the test moves.
func zbFlowStore(t *testing.T, project string) (Store, *time.Time) {
	t.Helper()
	home := gridhome.Real(t.TempDir())
	now := time.Date(2026, 1, 2, 3, 4, 0, 0, time.UTC)
	s := Store{GridHome: filepath.Join(home, ".grid"), Project: project, Now: func() time.Time { return now }}
	return s, &now
}

// zbFlowExecutor starts no process: its runCommand records the argv and writes
// what `grid task new --json` prints, or fails as a non-zero exit does.
type zbFlowExecutor struct {
	args   [][]string
	print  string
	failed error
}

func (z *zbFlowExecutor) process() ProcessExecutor {
	return ProcessExecutor{GridBinary: "/opt/grid/bin/grid", Home: "/home/person", Getenv: func(string) string { return "" },
		runCommand: func(cmd *exec.Cmd) error {
			z.args = append(z.args, slices.Clone(cmd.Args[1:]))
			if z.failed != nil {
				return z.failed
			}
			_, err := io.WriteString(cmd.Stdout, z.print)
			return err
		}}
}

func zbFlowSignals(count *int) FailureSignal {
	return func(Entry, error) error { *count++; return nil }
}

func TestZbFlowRunStartsTaskAndRecordsItsID(t *testing.T) {
	store, _ := zbFlowStore(t, "alpha")
	if _, err := store.AddWith("nightly", "1h", "", AddOptions{Workflow: "bugfix"}); err != nil {
		t.Fatal(err)
	}
	z := &zbFlowExecutor{print: `{"id":"T-07","path":"/p"}` + "\n"}
	signals := 0
	result, err := store.Execute(context.Background(), "nightly", false, z.process(), zbFlowSignals(&signals))
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"task", "new", "nightly", "--workflow", "bugfix", "--project", "alpha", "--json"}
	if len(z.args) != 1 || !slices.Equal(z.args[0], want) {
		t.Fatalf("grid args %q, want %q", z.args, want)
	}
	got, err := store.Get("nightly")
	if err != nil || got.LastTask != "T-07" || got.Outcome != "success" || result.LastTask != "T-07" || signals != 0 {
		t.Fatalf("stored %+v result %+v signals %d: %v", got, result.Entry, signals, err)
	}

	// A non-zero exit, then an exit that prints no id: failed, the old task kept, one signal each.
	for i, z := range []*zbFlowExecutor{{failed: errors.New("exit status 1")}, {print: "no json here\n"}} {
		if _, err := store.Execute(context.Background(), "nightly", false, z.process(), zbFlowSignals(&signals)); err == nil {
			t.Fatalf("case %d: failed run returned no error", i)
		}
		got, err := store.Get("nightly")
		if err != nil || got.Outcome != "failed" || got.LastTask != "T-07" || signals != i+1 {
			t.Fatalf("case %d: stored %+v signals %d: %v", i, got, signals, err)
		}
	}
}

func TestZbFlowHomeRoutineStartsPersonalTask(t *testing.T) {
	store, _ := zbFlowStore(t, "")
	if _, err := store.AddWith("weekly", "1h", "", AddOptions{Workflow: "bugfix"}); err != nil {
		t.Fatal(err)
	}
	z := &zbFlowExecutor{print: `{"id":"T-01","path":"/p"}`}
	if _, err := store.Execute(context.Background(), "weekly", false, z.process(), nil); err != nil {
		t.Fatal(err)
	}
	want := []string{"task", "new", "weekly", "--workflow", "bugfix", "--project", "personal", "--json"}
	if len(z.args) != 1 || !slices.Equal(z.args[0], want) {
		t.Fatalf("grid args %q, want %q", z.args, want)
	}
}

func TestZbFlowAddRefusesRunAndWorkflowTogether(t *testing.T) {
	store, _ := zbFlowStore(t, "alpha")
	if _, err := store.AddWith("both", "1h", "grid version", AddOptions{Workflow: "bugfix"}); err == nil {
		t.Fatal("a routine with both run and workflow was added")
	}
	if _, err := store.AddWith("neither", "1h", "", AddOptions{}); err == nil {
		t.Fatal("a routine with neither run nor workflow was added")
	}
}

func TestZbFlowFingerprintChangesOnlyWithWorkflow(t *testing.T) {
	plain := Entry{Name: "daily", Every: "1h", Run: "grid version"}
	sum := sha256.Sum256([]byte("grid version" + "\x00" + "1h"))
	got, err := plain.Fingerprint()
	if err != nil || got != hex.EncodeToString(sum[:]) {
		t.Fatalf("fingerprint of a run routine %s, want %s: %v", got, hex.EncodeToString(sum[:]), err)
	}
	flow := Entry{Name: "daily", Every: "1h", Workflow: "bugfix"}
	flowSum := sha256.Sum256([]byte("\x00" + "1h"))
	other := flow
	other.Workflow = "other"
	a, errA := flow.Fingerprint()
	b, errB := other.Fingerprint()
	if errA != nil || errB != nil || a == got || a == hex.EncodeToString(flowSum[:]) || a == b {
		t.Fatalf("workflow fingerprints %s %s: %v %v", a, b, errA, errB)
	}
}

func TestZbFlowReplaceKeepsHistoryAndNeedsScheduleAgain(t *testing.T) {
	store, now := zbFlowStore(t, "alpha")
	if _, err := store.AddWith("nightly", "1h", "", AddOptions{Workflow: "bugfix"}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.AddWith("nightly", "1h", "", AddOptions{Workflow: "other"}); err == nil {
		t.Fatal("add without replace rewrote an existing routine")
	}
	s := Scheduler{Home: filepath.Dir(store.GridHome), GridHome: store.GridHome, Binary: "/opt/grid/bin/grid", Backend: Systemd{}, Runner: &fakeOS{}}
	if err := s.Schedule(context.Background(), store); err != nil {
		t.Fatal(err)
	}
	z := &zbFlowExecutor{print: `{"id":"T-07","path":"/p"}`}
	if _, err := store.Execute(context.Background(), "nightly", true, z.process(), nil); err != nil {
		t.Fatalf("scheduled run: %v", err)
	}
	before, err := store.Get("nightly")
	if err != nil {
		t.Fatal(err)
	}
	*now = now.Add(2 * time.Hour)
	replaced, err := store.AddWith("nightly", "1h", "", AddOptions{Workflow: "other", Replace: true})
	if err != nil {
		t.Fatal(err)
	}
	got, err := store.Get("nightly")
	if err != nil || got.Workflow != "other" || got.LastTask != "T-07" || got.Outcome != "success" || !got.LastRun.Equal(before.LastRun) || replaced.LastTask != "T-07" {
		t.Fatalf("replaced %+v, before %+v: %v", got, before, err)
	}
	signals := 0
	_, err = store.Execute(context.Background(), "nightly", true, z.process(), zbFlowSignals(&signals))
	if !errors.Is(err, ErrScheduleChanged) || len(z.args) != 1 || signals != 1 {
		t.Fatalf("timer run after replace: %v, runs %d, signals %d", err, len(z.args), signals)
	}
	if err := s.ScheduleOne(context.Background(), store, "nightly"); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Execute(context.Background(), "nightly", true, z.process(), nil); err != nil || len(z.args) != 2 {
		t.Fatalf("timer run after schedule again: %v, runs %d", err, len(z.args))
	}
}
