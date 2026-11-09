package routine

import (
	"bytes"
	"context"
	"errors"
	"os/exec"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/gridconsole/grid-plugins/internal/brain"
	"github.com/gridconsole/grid-plugins/internal/gridhome"
)

func policyForTest(t *testing.T, values map[string]string) LimitPolicy {
	t.Helper()
	var p LimitPolicy
	for key, value := range values {
		if err := p.SetText(key, value); err != nil {
			t.Fatal(err)
		}
	}
	return p
}

func TestProcessRoutineLimits(t *testing.T) {
	for _, explicit := range []bool{false, true} {
		t.Run(map[bool]string{false: "defaults", true: "overrides"}[explicit], func(t *testing.T) {
			e := Entry{Run: "signals"}
			wantTimeout := 10 * time.Minute
			wantBudget, wantTurns := "1", "10"
			if explicit {
				e.LimitPolicy = policyForTest(t, map[string]string{"routine_timeout": "2h", "routine_max_turns": "3", "routine_budget": "0.25"})
				wantTimeout, wantBudget, wantTurns = 2*time.Hour, "0.25", "3"
			}
			want := []string{"claude", "-p", "Run the grid:signals skill once. Work only in the Grid data folder. The project is example.", "--allowedTools", "Read,Glob,Grep,Bash(grid brain:*),Bash(grid signal:*)", "--max-budget-usd", wantBudget, "--max-turns", wantTurns, "--permission-mode", "dontAsk"}
			calls := 0
			p := ProcessExecutor{
				withTimeout: func(ctx context.Context, d time.Duration) (context.Context, context.CancelFunc) {
					if d != wantTimeout {
						t.Fatalf("timeout %s, want %s", d, wantTimeout)
					}
					return context.WithCancel(ctx)
				},
				runCommand: func(cmd *exec.Cmd) error {
					calls++
					if !reflect.DeepEqual(cmd.Args, want) {
						t.Fatalf("argv: %q, want %q", cmd.Args, want)
					}
					return nil
				},
			}
			if err := p.Execute(context.Background(), Store{Project: "example"}, e); err != nil || calls != 1 {
				t.Fatalf("execute: %v, calls %d", err, calls)
			}
		})
	}
}

func TestGridRoutineTimeoutAtInjectedClock(t *testing.T) {
	e := Entry{Run: `grid example "two words"`, LimitPolicy: policyForTest(t, map[string]string{"routine_timeout": "1m", "routine_max_turns": "3", "routine_budget": "0.25"})}
	var commandContext context.Context
	var advanceClock context.CancelFunc
	p := ProcessExecutor{
		GridBinary: "grid",
		withTimeout: func(ctx context.Context, d time.Duration) (context.Context, context.CancelFunc) {
			if d != time.Minute {
				t.Fatalf("timeout = %s, want 1m", d)
			}
			commandContext, advanceClock = context.WithCancel(ctx)
			return commandContext, advanceClock
		},
		runCommand: func(cmd *exec.Cmd) error {
			if want := []string{"grid", "example", "two words"}; !reflect.DeepEqual(cmd.Args, want) {
				t.Fatalf("grid got agent flags: %q", cmd.Args)
			}
			if commandContext.Err() != nil {
				t.Fatal("cancelled before clock advance")
			}
			advanceClock()
			<-commandContext.Done()
			return commandContext.Err()
		},
	}
	if err := p.Execute(context.Background(), Store{}, e); !errors.Is(err, context.Canceled) || !strings.Contains(err.Error(), "timed out or was cancelled") {
		t.Fatalf("cancellation: %v", err)
	}
}

func TestRoutinePolicyRoundTripAndRefusal(t *testing.T) {
	for _, project := range []string{"", "example"} {
		s := Store{GridHome: gridhome.Real(t.TempDir()), Project: project}
		policy := policyForTest(t, map[string]string{"routine_timeout": "1m", "routine_max_turns": "3", "routine_budget": "0.25"})
		e, err := s.Add("daily", "1h", "signals", policy)
		if err != nil {
			t.Fatal(err)
		}
		e.Body = "Keep these routine notes."
		e.LastRun = time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
		e.Outcome = "success"
		if err := s.save(e); err != nil {
			t.Fatal(err)
		}
		runner := executorFunc(func(_ context.Context, _ Store, got Entry) error {
			if got.Limits.Timeout != "1m" || got.Limits.MaxTurns != 3 || got.Limits.Budget != 0.25 {
				t.Fatalf("executor limits: %+v", got.Limits)
			}
			return nil
		})
		if _, err := s.Execute(context.Background(), e.Name, false, runner, nil); err != nil {
			t.Fatal(err)
		}
		got, err := s.Get(e.Name)
		if err != nil || strings.TrimSpace(got.Body) != e.Body || got.RoutineMaxTurns.Value != "3" {
			t.Fatalf("round trip lost fields/body: %+v %v", got, err)
		}
		var before []byte
		if err := s.brain().EditConfig(s.level(), "routines", e.Name, func(old []byte) ([]byte, error) {
			before = bytes.Replace(old, []byte("routine_timeout: 1m"), []byte("routine_timeout: 1s"), 1)
			if bytes.Equal(before, old) {
				t.Fatal("did not inject invalid timeout")
			}
			return before, nil
		}); err != nil {
			t.Fatal(err)
		}
		_, err = s.Execute(context.Background(), e.Name, false, executorFunc(func(context.Context, Store, Entry) error { t.Fatal("invalid policy reached executor"); return nil }), func(Entry, error) error { t.Fatal("invalid policy emitted failure"); return nil })
		var invalid *LimitError
		if !errors.As(err, &invalid) || invalid.Key != "routine_timeout" {
			t.Fatalf("refusal: %v", err)
		}
		after, err := s.brain().ReadConfig(s.level(), "routines", e.Name)
		if err != nil || !bytes.Equal(before, after) {
			t.Fatalf("refusal changed last_run: %v", err)
		}
	}
}

func TestRoutineLimitBoundariesAndTypes(t *testing.T) {
	for _, tc := range []struct {
		key, value string
		valid      bool
	}{
		{"routine_timeout", "1m", true}, {"routine_timeout", "2h", true}, {"routine_timeout", "1m30s", true},
		{"routine_timeout", "59.999s", false}, {"routine_timeout", "2h1ns", false}, {"routine_timeout", `""`, false},
		{"routine_max_turns", "1", true}, {"routine_max_turns", "100", true}, {"routine_max_turns", "1.0", false},
		{"routine_max_turns", "99999999999999999999999999", false}, {"routine_max_turns", "{}", false},
		{"routine_budget", "0.000001", true}, {"routine_budget", "50", true}, {"routine_budget", "50.0001", false}, {"routine_budget", "50.000000000000000001", false},
		{"routine_budget", "049", true}, {"routine_budget", "060", false}, {"routine_budget", "0x10", false}, {"routine_budget", "false", false}, {"routine_budget", "[]", false},
	} {
		t.Run(tc.key+"_"+tc.value, func(t *testing.T) {
			data := []byte("---\nname: daily\nevery: 1h\nrun: signals\nmachine: 01ARZ3NDEKTSV4RRFFQ69G5FAV\n" + tc.key + ": " + tc.value + "\n---\n")
			_, err := decode("daily", data)
			if (err == nil) != tc.valid {
				t.Fatalf("decode: %v, valid %v", err, tc.valid)
			}
			if !tc.valid && err.Error() != "invalid "+tc.key {
				t.Fatalf("unsafe refusal: %v", err)
			}
		})
	}
}

func TestRoutineOmittedLimitsStayOmitted(t *testing.T) {
	s := Store{GridHome: gridhome.Real(t.TempDir())}
	e, err := s.Add("daily", "1h", "signals")
	if err != nil {
		t.Fatal(err)
	}
	data, err := brain.EncodeConfig(e, "")
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"routine_timeout", "routine_max_turns", "routine_budget", "limits:"} {
		if bytes.Contains(data, []byte(key)) {
			t.Fatalf("default persisted as override: %s", data)
		}
	}
}

func TestRoutineAddRejectsYAMLDecorations(t *testing.T) {
	for _, text := range []string{"&limit 3", "!!int 3", "*limit"} {
		var policy LimitPolicy
		if err := policy.SetText("routine_max_turns", text); err == nil || err.Error() != "invalid routine_max_turns" {
			t.Fatalf("accepted YAML decoration: %v", err)
		}
	}
}
