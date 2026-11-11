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

// rsStore returns a store holding one routine "daily", the entry Add returned,
// and the path of its file.
func rsStore(t *testing.T) (Store, Entry, string) {
	t.Helper()
	s := Store{GridHome: gridhome.Real(t.TempDir())}
	e, err := s.Add("daily", "24h", "grid version")
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(s.GridHome, "knowledge", "routines", "daily.md")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("routine file missing: %v", err)
	}
	return s, e, path
}

func rsWant(t *testing.T, err error, want string) {
	t.Helper()
	if err == nil || err.Error() != want {
		t.Fatalf("got %v, want %q", err, want)
	}
}

func rsNames(t *testing.T, s Store) []string {
	t.Helper()
	entries, err := s.List()
	if err != nil {
		t.Fatal(err)
	}
	names := []string{}
	for _, e := range entries {
		names = append(names, e.Name)
	}
	return names
}

func TestRSValidateRefusals(t *testing.T) {
	_, base, _ := rsStore(t)
	if err := base.Validate(); err != nil {
		t.Fatalf("base entry must validate: %v", err)
	}
	cases := []struct {
		name   string
		change func(*Entry)
		want   string
	}{
		{"name", func(e *Entry) { e.Name = "bad name" }, "routine name must be a plain name"},
		{"machine", func(e *Entry) { e.Machine = "x" }, "routine needs an owner machine ULID"},
		{"every", func(e *Entry) { e.Every = "soon" }, "every must be a duration or five-field numeric cron expression"},
		{"run", func(e *Entry) { e.Run = "" }, "run is required"},
		{"outcome", func(e *Entry) { e.Outcome = "maybe" }, "invalid routine outcome"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			e := base
			c.change(&e)
			rsWant(t, e.Validate(), c.want)
		})
	}
}

func TestRSDecodeRefusals(t *testing.T) {
	s, _, path := rsStore(t)
	other := filepath.Join(filepath.Dir(path), "other.md")
	if err := os.WriteFile(other, []byte("---\nname: [\n---\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Get("other"); err == nil || !strings.HasPrefix(err.Error(), "front matter: ") {
		t.Fatalf("got %v, want prefix %q", err, "front matter: ")
	}
	_, err := Store{GridHome: s.GridHome, Project: "bad name"}.List()
	rsWant(t, err, "project level needs a valid project name")
}

func TestRSNameMismatch(t *testing.T) {
	s, _, path := rsStore(t)
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(filepath.Dir(path), "other.md"), b, 0o600); err != nil {
		t.Fatal(err)
	}
	_, err = s.Get("other")
	rsWant(t, err, "routine name does not match its file")
	_, err = s.List()
	rsWant(t, err, "routine other: routine name does not match its file")
}

func TestRSRemoveLockRefusal(t *testing.T) {
	s, _, _ := rsStore(t)
	rsWant(t, s.Remove("bad name"), "invalid routine or project name")
}

func TestRSAddRefusals(t *testing.T) {
	s, _, _ := rsStore(t)
	_, err := s.Add("bad name", "1h", "grid version")
	rsWant(t, err, "routine name must be a plain name")
	_, err = s.Add("y", "1h", "")
	rsWant(t, err, "run is required")
	if got := rsNames(t, s); len(got) != 1 || got[0] != "daily" {
		t.Fatalf("List after refused adds = %v, want [daily]", got)
	}
}

func TestRSExecuteLockRefusal(t *testing.T) {
	s, _, _ := rsStore(t)
	_, err := s.Execute(context.Background(), "bad name", false, nil, nil)
	rsWant(t, err, "invalid routine or project name")
}

func TestRSExecuteOtherMachine(t *testing.T) {
	s, _, path := rsStore(t)
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	own := "machine: " + gridhome.Machine(s.GridHome)
	if !strings.Contains(string(b), own) {
		t.Fatalf("routine file lacks %q", own)
	}
	edited := strings.Replace(string(b), own, "machine: 01ARZ3NDEKTSV4RRFFQ69G5FAV", 1)
	if err := os.WriteFile(path, []byte(edited), 0o600); err != nil {
		t.Fatal(err)
	}
	r, err := s.Execute(context.Background(), "daily", false, nil, nil)
	rsWant(t, err, "routine belongs to another machine")
	if r.Name != "daily" {
		t.Fatalf("result name = %q, want daily", r.Name)
	}
}

func TestRSExecuteNeedsExecutor(t *testing.T) {
	s, _, _ := rsStore(t)
	_, err := s.Execute(context.Background(), "daily", false, nil, nil)
	rsWant(t, err, "routine executor is required")
	e, err := s.Get("daily")
	if err != nil {
		t.Fatal(err)
	}
	if !e.LastRun.IsZero() || e.Outcome != "" {
		t.Fatalf("refused run was recorded: last_run %v outcome %q", e.LastRun, e.Outcome)
	}
}

func TestRSExecuteDisappeared(t *testing.T) {
	s, _, _ := rsStore(t)
	runner := executorFunc(func(_ context.Context, st Store, en Entry) error { return st.remove(en.Name) })
	_, err := s.Execute(context.Background(), "daily", false, runner, nil)
	if err == nil || !strings.Contains(err.Error(), "routine disappeared during run") {
		t.Fatalf("got %v, want it to contain %q", err, "routine disappeared during run")
	}
	if _, err := s.Get("daily"); err == nil {
		t.Fatal("routine still readable after it was removed")
	}
}

func TestRSExecuteBadTimerState(t *testing.T) {
	s, _, _ := rsStore(t)
	state, err := Scheduler{GridHome: s.GridHome}.statePath(timerLabel(s.GridHome, "", "daily"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(state), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(state, []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}
	ran := false
	runner := executorFunc(func(context.Context, Store, Entry) error { ran = true; return nil })

	_, err = s.Execute(context.Background(), "daily", true, runner, nil)
	var syn *json.SyntaxError
	if !errors.As(err, &syn) {
		t.Fatalf("got %v, want a *json.SyntaxError", err)
	}

	signal := errors.New("rs signal")
	calls := 0
	_, err = s.Execute(context.Background(), "daily", true, runner, func(Entry, error) error { calls++; return signal })
	if !errors.As(err, &syn) || !errors.Is(err, signal) {
		t.Fatalf("got %v, want both the syntax error and %q", err, signal)
	}
	if calls != 1 {
		t.Fatalf("onFailure called %d times, want 1", calls)
	}
	if ran {
		t.Fatal("runner ran despite unreadable timer state")
	}
}
