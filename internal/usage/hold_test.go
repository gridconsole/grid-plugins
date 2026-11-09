package usage

import (
	"math"
	"os"
	"path/filepath"
	"testing"
)

// holdStore posts the spent amounts first, then sets the budget, so no notice
// fires and the test reads only the hold.
func holdStore(t *testing.T, spent Input, budget Change) Store {
	t.Helper()
	s := testStore(t)
	spent.Job = "lead"
	if _, err := s.Post(spent); err != nil {
		t.Fatal(err)
	}
	budget.Budget = true
	if _, err := s.Configure(budget); err != nil {
		t.Fatal(err)
	}
	return s
}

func tokens(n uint64) *uint64 { return &n }

func TestHoldAtTheBudget(t *testing.T) {
	for _, tc := range []struct {
		name   string
		spent  Input
		budget Change
		hold   bool
		reason string
	}{
		{"dollars at the budget", Input{Dollars: number(10)}, Change{Dollars: []float64{10}}, true, "dollars: 10 spent of a budget of 10"},
		{"dollars just under", Input{Dollars: number(9.99)}, Change{Dollars: []float64{10}}, false, ""},
		{"minutes at the budget", Input{Minutes: number(60)}, Change{Minutes: []float64{60}}, true, "minutes: 60 spent of a budget of 60"},
		{"minutes just under", Input{Minutes: number(59.5)}, Change{Minutes: []float64{60}}, false, ""},
		{"tokens at the budget", Input{Tokens: tokens(1000)}, Change{BudgetTokens: []uint64{1000}}, true, "tokens: 1000 spent of a budget of 1000"},
		{"tokens just under", Input{Tokens: tokens(999)}, Change{BudgetTokens: []uint64{1000}}, false, ""},
		{"over the budget", Input{Dollars: number(12.5)}, Change{Dollars: []float64{10}}, true, "dollars: 12.5 spent of a budget of 10"},
		{"decimal sum equals the budget", Input{Dollars: number(0.1)}, Change{Dollars: []float64{0.1}}, true, "dollars: 0.1 spent of a budget of 0.1"},
		{"any unit holds", Input{Dollars: number(1), Minutes: number(60)}, Change{Dollars: []float64{10}, Minutes: []float64{60}}, true, "minutes: 60 spent of a budget of 60"},
		{"every unit at the limit is named", Input{Dollars: number(10), Minutes: number(60)}, Change{Dollars: []float64{10}, Minutes: []float64{60}}, true, "dollars: 10 spent of a budget of 10; minutes: 60 spent of a budget of 60"},
		{"a unit without a budget is not judged", Input{Dollars: number(500)}, Change{Minutes: []float64{60}}, false, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := holdStore(t, tc.spent, tc.budget).Hold()
			if err != nil {
				t.Fatal(err)
			}
			if got.Hold != tc.hold || got.Reason != tc.reason {
				t.Fatalf("got %+v, want hold %v reason %q", got, tc.hold, tc.reason)
			}
		})
	}
}

func TestHoldDecimalPostsSumToTheBudget(t *testing.T) {
	s := testStore(t)
	for _, d := range []float64{0.1, 0.2} {
		if _, err := s.Post(Input{Job: "lead", Dollars: number(d)}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := s.Configure(Change{Budget: true, Dollars: []float64{0.3}}); err != nil {
		t.Fatal(err)
	}
	got, err := s.Hold()
	if err != nil || !got.Hold {
		t.Fatalf("got %+v, %v", got, err)
	}
}

func TestHoldWithoutABudgetNeverHolds(t *testing.T) {
	always := func(HoldSummary) (bool, string) { return true, "always" }
	s := testStore(t)
	s.HoldPolicy = always
	if _, err := s.Post(Input{Job: "lead", Dollars: number(500), Minutes: number(500), Tokens: tokens(500)}); err != nil {
		t.Fatal(err)
	}
	if got, err := s.Hold(); err != nil || got.Hold {
		t.Fatalf("no budget: %+v, %v", got, err)
	}
	// A task with no usage data at all, and a cleared budget, also never hold.
	fresh := testStore(t)
	fresh.HoldPolicy = always
	if got, err := fresh.Hold(); err != nil || got.Hold {
		t.Fatalf("no data: %+v, %v", got, err)
	}
	for _, dollars := range [][]float64{{1}, {}} {
		if _, err := s.Configure(Change{Budget: true, Dollars: dollars}); err != nil {
			t.Fatal(err)
		}
	}
	if got, err := s.Hold(); err != nil || got.Hold {
		t.Fatalf("cleared: %+v, %v", got, err)
	}
}

func TestHoldUsesTheWorkspaceTokenDefault(t *testing.T) {
	s := testStore(t)
	s.GridHome = t.TempDir()
	if err := os.WriteFile(filepath.Join(s.GridHome, "workspace.md"), []byte("---\ntokens_per_task: 100\n---\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Post(Input{Job: "lead", Tokens: tokens(99)}); err != nil {
		t.Fatal(err)
	}
	if got, err := s.Hold(); err != nil || got.Hold {
		t.Fatalf("under: %+v, %v", got, err)
	}
	if _, err := s.Post(Input{Job: "lead", Tokens: tokens(1)}); err != nil {
		t.Fatal(err)
	}
	if got, err := s.Hold(); err != nil || !got.Hold {
		t.Fatalf("at: %+v, %v", got, err)
	}
}

func TestHoldPolicySeam(t *testing.T) {
	at := func(percent float64) HoldPolicy { return HoldSettings{AtPercent: percent}.Judge }
	for _, tc := range []struct {
		name   string
		policy HoldPolicy
		spent  float64
		hold   bool
	}{
		{"80 percent holds at 8 of 10", at(80), 8, true},
		{"80 percent allows under 8 of 10", at(80), 7.99, false},
		{"80 percent holds past it", at(80), 10, true},
		{"100 percent allows 8 of 10", at(100), 8, false},
		{"a policy that never holds lets 10 of 10 through", func(HoldSummary) (bool, string) { return false, "" }, 10, false},
		{"the default policy holds 10 of 10", nil, 10, true},
		{"zero percent means the default", at(0), 9, false},
		{"negative percent means the default", at(-5), 9, false},
		{"not a number means the default", at(math.NaN()), 9, false},
		{"infinity means the default", at(math.Inf(1)), 9, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := holdStore(t, Input{Dollars: number(tc.spent)}, Change{Dollars: []float64{10}})
			s.HoldPolicy = tc.policy
			got, err := s.Hold()
			if err != nil || got.Hold != tc.hold {
				t.Fatalf("got %+v, %v, want hold %v", got, err, tc.hold)
			}
		})
	}
}

func TestHoldPolicySeesTheSpendAndTheBudget(t *testing.T) {
	s := holdStore(t, Input{Dollars: number(3), Minutes: number(4), Tokens: tokens(5)}, Change{Dollars: []float64{10}})
	var seen HoldSummary
	s.HoldPolicy = func(in HoldSummary) (bool, string) { seen = in; return true, "custom reason" }
	got, err := s.Hold()
	if err != nil || !got.Hold || got.Reason != "custom reason" {
		t.Fatalf("got %+v, %v", got, err)
	}
	if seen.Totals.Dollars != 3 || seen.Totals.Minutes != 4 || seen.Totals.Tokens != 5 {
		t.Fatalf("spent: %+v", seen.Totals)
	}
	if seen.Budget.Dollars == nil || *seen.Budget.Dollars != 10 || seen.Budget.Minutes != nil || seen.Budget.Tokens != nil {
		t.Fatalf("budget: %+v", seen.Budget)
	}
}

func TestHoldToolsAreTheOnesThatStartAnAgent(t *testing.T) {
	for tool, want := range map[string]bool{"Agent": true, "Task": true, "Read": false, "Bash": false, "Write": false, "": false, "agent": false} {
		if HoldsTool(tool) != want {
			t.Errorf("HoldsTool(%q) = %v, want %v", tool, !want, want)
		}
	}
}

func TestHoldReasonNamesThePercentWhenNotTheDefault(t *testing.T) {
	s := holdStore(t, Input{Dollars: number(8)}, Change{Dollars: []float64{10}})
	s.HoldPolicy = HoldSettings{AtPercent: 80}.Judge
	got, err := s.Hold()
	if err != nil || got.Reason != "dollars: 8 spent of a budget of 10 (hold at 80%)" {
		t.Fatalf("got %+v, %v", got, err)
	}
}

func TestHoldUnreadableOrMalformedDataDoesNotHold(t *testing.T) {
	t.Run("a directory where the usage log is", func(t *testing.T) {
		s := holdStore(t, Input{Dollars: number(10)}, Change{Dollars: []float64{10}})
		log := filepath.Join(s.TaskPath, "usage", "usage.jsonl")
		if err := os.Remove(log); err != nil {
			t.Fatal(err)
		}
		if err := os.Mkdir(log, 0o700); err != nil {
			t.Fatal(err)
		}
		got, err := s.Hold()
		if err == nil || got.Hold {
			t.Fatalf("got %+v, %v", got, err)
		}
	})
	t.Run("a torn usage line", func(t *testing.T) {
		s := holdStore(t, Input{Dollars: number(10)}, Change{Dollars: []float64{10}})
		f, err := os.OpenFile(filepath.Join(s.TaskPath, "usage", "usage.jsonl"), os.O_APPEND|os.O_WRONLY, 0)
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		if _, err := f.WriteString("not json\n"); err != nil {
			t.Fatal(err)
		}
		got, err := s.Hold()
		if err == nil || got.Hold {
			t.Fatalf("got %+v, %v", got, err)
		}
	})
	for _, tc := range []struct{ name, budget string }{
		{"a negative dollars budget", `{"dollars":-1}`},
		{"a text dollars budget", `{"dollars":"ten"}`},
		{"a zero minutes budget", `{"minutes":0}`},
		{"a fractional tokens budget", `{"tokens":1.5}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := holdStore(t, Input{Dollars: number(10)}, Change{Dollars: []float64{10}})
			data := `{"v":2,"budget":` + tc.budget + `,"baseline":{"dollars":0,"minutes":0,"tokens":0},"fired":{}}`
			if err := os.WriteFile(filepath.Join(s.TaskPath, "usage", "thresholds.json"), []byte(data), 0o600); err != nil {
				t.Fatal(err)
			}
			got, err := s.Hold()
			if err == nil || got.Hold {
				t.Fatalf("got %+v, %v", got, err)
			}
		})
	}
}

func TestHoldRequiresATask(t *testing.T) {
	if got, err := (Store{}).Hold(); err == nil || got.Hold {
		t.Fatalf("got %+v, %v", got, err)
	}
}
