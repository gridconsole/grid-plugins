package routine

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gridconsole/grid-plugins/internal/gridhome"
)

func TestRoutineWorkflowResolutionAndSnapshot(t *testing.T) {
	home := gridhome.Real(t.TempDir())
	s := Store{GridHome: home, Project: "example"}
	writeRoutineWorkflow(t, Store{GridHome: home}, "routine_budget: 50")
	e, err := s.Add("daily", "1h", "signals", policyForTest(t, map[string]string{"routine_max_turns": "3"}))
	if err != nil || e.Limits.Budget != 1 {
		t.Fatalf("project inherited home policy: %+v %v", e.Limits, err)
	}
	writeRoutineWorkflow(t, s, "routine_timeout: 20m\nroutine_max_turns: 20\nroutine_budget: 2\nunrelated: retained")
	path := filepath.Join(home, "projects", "example", "workflow.md")
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	entries, err := s.List()
	want := Limits{Timeout: "20m", MaxTurns: 3, Budget: 2}
	if err != nil || len(entries) != 1 || entries[0].Limits != want {
		t.Fatalf("list policy: %+v %v", entries, err)
	}
	calls := 0
	runner := executorFunc(func(ctx context.Context, s Store, e Entry) error {
		if e.Limits != want {
			t.Fatalf("run policy: %+v", e.Limits)
		}
		// A concurrent workflow edit after the guard must not change this run.
		writeRoutineWorkflow(t, s, "routine_timeout: 2h\nroutine_max_turns: 100\nroutine_budget: 50")
		p := ProcessExecutor{
			withTimeout: func(ctx context.Context, d time.Duration) (context.Context, context.CancelFunc) {
				if d != 20*time.Minute {
					t.Fatalf("executor timeout = %s", d)
				}
				return context.WithCancel(ctx)
			},
			runCommand: func(cmd *exec.Cmd) error {
				calls++
				args := strings.Join(cmd.Args, " ")
				if !strings.Contains(args, "--max-budget-usd 2 --max-turns 3") {
					t.Fatalf("executor did not use approved snapshot: %s", args)
				}
				return nil
			},
		}
		if err := p.Execute(ctx, s, e); err != nil {
			return err
		}
		return os.WriteFile(path, before, 0600)
	})
	if _, err := s.Execute(context.Background(), "daily", false, runner, nil); err != nil || calls != 1 {
		t.Fatalf("execute: %v, calls %d", err, calls)
	}
	raw, err := s.brain().ReadConfig(s.level(), "routines", "daily")
	if err != nil || strings.Contains(string(raw), "routine_timeout") || strings.Contains(string(raw), "routine_budget") {
		t.Fatalf("inherited values persisted: %v", err)
	}
	after, err := os.ReadFile(path)
	if err != nil || string(after) != string(before) {
		t.Fatalf("workflow changed: %v", err)
	}
}

func TestRoutineInvalidWorkflowRefusesBeforeExecution(t *testing.T) {
	for _, front := range []string{
		"routine_budget: null", "routine_max_turns: 101", "routine_timeout: 1s",
		"routine_budget: 1\nroutine_budget: 2", "routine_budget: &budget 2", "routine_budget: [",
	} {
		t.Run(front, func(t *testing.T) {
			s := Store{GridHome: gridhome.Real(t.TempDir())}
			if _, err := s.Add("daily", "1h", "signals", policyForTest(t, map[string]string{"routine_budget": "1"})); err != nil {
				t.Fatal(err)
			}
			before, err := s.brain().ReadConfig(s.level(), "routines", "daily")
			if err != nil {
				t.Fatal(err)
			}
			writeRoutineWorkflow(t, s, front)
			for _, due := range []bool{false, true} {
				_, err := s.Execute(context.Background(), "daily", due, executorFunc(func(context.Context, Store, Entry) error {
					t.Fatal("invalid workflow reached executor")
					return nil
				}), nil)
				if err == nil || strings.Contains(err.Error(), s.GridHome) {
					t.Fatalf("unsafe workflow refusal: %v", err)
				}
			}
			after, err := s.brain().ReadConfig(s.level(), "routines", "daily")
			if err != nil || string(after) != string(before) {
				t.Fatalf("refusal changed routine: %v", err)
			}
		})
	}
}

func TestRoutineWorkflowUnsafeFileRefused(t *testing.T) {
	for _, kind := range []string{"directory", "symlink"} {
		t.Run(kind, func(t *testing.T) {
			s := Store{GridHome: gridhome.Real(t.TempDir())}
			if _, err := s.Add("daily", "1h", "signals"); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(s.GridHome, "workflow.md")
			if kind == "directory" {
				if err := os.Mkdir(path, 0700); err != nil {
					t.Fatal(err)
				}
			} else {
				if err := os.Symlink(filepath.Join(s.GridHome, "missing"), path); err != nil {
					t.Skip("symlinks unavailable")
				}
			}
			if _, err := s.Get("daily"); err == nil || err.Error() != "cannot read routine workflow policy" {
				t.Fatalf("unsafe file refusal: %v", err)
			}
		})
	}
}

func TestRoutineFingerprintRejectsInvalidPolicy(t *testing.T) {
	e := Entry{Run: "grid version", Every: "1h", LimitPolicy: policyForTest(t, map[string]string{"routine_budget": "1"})}
	e.RoutineBudget.Value = "51"
	var limitErr *LimitError
	if _, err := e.Fingerprint(); !errors.As(err, &limitErr) {
		t.Fatalf("invalid fingerprint: %v", err)
	}
}
