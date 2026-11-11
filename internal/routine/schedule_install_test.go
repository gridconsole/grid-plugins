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

// siBackend renders a fixed plan so the install checks can be driven without a
// host scheduler.
type siBackend struct {
	home   string
	logDir string
	path   string
}

func (b siBackend) Plan(sp TimerSpec) (TimerPlan, error) {
	path := b.path
	if path == "" {
		path = filepath.Join(b.home, "timers", sp.Name+".timer")
	}
	return TimerPlan{
		Label:       sp.label(),
		Project:     sp.Project,
		Name:        sp.Name,
		LogDir:      b.logDir,
		Files:       []TimerFile{{Path: path, Content: marker(sp.label()) + "\n"}},
		Install:     []OSCommand{{Program: "install"}},
		Remove:      []OSCommand{{Program: "remove"}},
		AfterRemove: []OSCommand{{Program: "after"}},
	}, nil
}

// siSetup builds a home, a store holding the routine "daily" and a Scheduler
// that has not run yet.
func siSetup(t *testing.T, b siBackend) (Scheduler, Store, *fakeOS) {
	t.Helper()
	home := t.TempDir()
	store := Store{GridHome: gridhome.Real(filepath.Join(home, "gh"))}
	if err := os.MkdirAll(store.GridHome, 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Add("daily", "1h", "grid version"); err != nil {
		t.Fatal(err)
	}
	b.home = home
	fake := &fakeOS{}
	s := Scheduler{Home: home, GridHome: store.GridHome, Binary: filepath.Join(home, "grid"), Backend: b, Runner: fake}
	return s, store, fake
}

func siWant(t *testing.T, err error, want string) {
	t.Helper()
	if err == nil || err.Error() != want {
		t.Fatalf("error: %v, want %q", err, want)
	}
}

func siCalls(calls []OSCommand) []string {
	names := []string{}
	for _, c := range calls {
		names = append(names, c.Program)
	}
	return names
}

func siState(t *testing.T, s Scheduler, store Store) string {
	t.Helper()
	path, err := s.statePath(timerLabel(store.GridHome, "", "daily"))
	if err != nil {
		t.Fatal(err)
	}
	return path
}

func TestSIExecOSRunner(t *testing.T) {
	ctx := context.Background()
	if err := (ExecOSRunner{}).Run(ctx, OSCommand{Program: os.Args[0], Args: []string{"-test.run=^$"}}); err != nil {
		t.Fatalf("run test binary: %v", err)
	}
	err := ExecOSRunner{}.Run(ctx, OSCommand{Program: "si-no-such-program"})
	if err == nil || !strings.HasPrefix(err.Error(), "si-no-such-program: exec: ") {
		t.Fatalf("missing program: %v", err)
	}
}

func TestSIPlanRefusals(t *testing.T) {
	s, _, _ := siSetup(t, siBackend{})
	for _, tc := range []struct {
		name string
		edit func(*Scheduler)
		task string
		want string
	}{
		{"name", func(*Scheduler) {}, "bad name", "invalid routine or project name"},
		{"backend", func(s *Scheduler) { s.Backend = nil }, "daily", "timer backend and OS runner are required"},
		{"relative", func(s *Scheduler) { s.Home = "rel" }, "daily", "timer home, Grid home and executable must be absolute"},
		{"newline", func(s *Scheduler) { s.Path = "a\nb" }, "daily", "timer paths must be one line"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := s
			tc.edit(&c)
			_, err := c.plan("", tc.task)
			siWant(t, err, tc.want)
		})
	}
}

func TestSICheckFilesOutsideHome(t *testing.T) {
	s, store, fake := siSetup(t, siBackend{})
	s.Backend = siBackend{home: s.Home, path: filepath.Join(filepath.Dir(s.Home), "si-elsewhere", "x.timer")}
	siWant(t, s.Schedule(context.Background(), store), "timer file is outside the home folder")
	if len(fake.calls) != 0 {
		t.Fatalf("runner calls: %v", fake.calls)
	}
}

func TestSICheckFilesNotRegular(t *testing.T) {
	s, store, fake := siSetup(t, siBackend{})
	if err := os.MkdirAll(filepath.Join(s.Home, "timers", "daily.timer"), 0o700); err != nil {
		t.Fatal(err)
	}
	siWant(t, s.Schedule(context.Background(), store), "timer file is not a small regular file")
	if len(fake.calls) != 0 {
		t.Fatalf("runner calls: %v", fake.calls)
	}
}

func TestSIReadStateRefusals(t *testing.T) {
	s, store, fake := siSetup(t, siBackend{})
	path := siState(t, s, store)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}
	err := s.Schedule(context.Background(), store)
	var syntax *json.SyntaxError
	if !errors.As(err, &syntax) {
		t.Fatalf("bad record: %v, want *json.SyntaxError", err)
	}
	if err := os.WriteFile(path, []byte(`{"label":"other","project":"","name":"daily"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	siWant(t, s.Schedule(context.Background(), store), "timer ownership record does not match")
	if len(fake.calls) != 0 {
		t.Fatalf("runner calls: %v", fake.calls)
	}
}

func TestSILogFolderFailure(t *testing.T) {
	s, store, fake := siSetup(t, siBackend{})
	file := filepath.Join(s.Home, "si-logfile")
	if err := os.WriteFile(file, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	s.Backend = siBackend{home: s.Home, logDir: filepath.Join(file, "x")}
	err := s.Schedule(context.Background(), store)
	if err == nil || !strings.HasPrefix(err.Error(), "mkdir ") {
		t.Fatalf("log folder: %v, want prefix %q", err, "mkdir ")
	}
	if _, err := os.Stat(siState(t, s, store)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("state file written: %v", err)
	}
	if len(fake.calls) != 0 {
		t.Fatalf("runner calls: %v", fake.calls)
	}
}

func TestSIAnotherMachine(t *testing.T) {
	s, store, fake := siSetup(t, siBackend{})
	path := filepath.Join(store.GridHome, "knowledge", "routines", "daily.md")
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	old := "machine: " + gridhome.Machine(store.GridHome)
	if !strings.Contains(string(b), old) {
		t.Fatalf("entry has no %q:\n%s", old, b)
	}
	edited := strings.Replace(string(b), old, "machine: 01ARZ3NDEKTSV4RRFFQ69G5FAV", 1)
	if err := os.WriteFile(path, []byte(edited), 0o600); err != nil {
		t.Fatal(err)
	}
	siWant(t, s.install(context.Background(), store, "daily"), "routine belongs to another machine")
	if err := s.Schedule(context.Background(), store); err != nil {
		t.Fatalf("Schedule: %v", err)
	}
	if len(fake.calls) != 0 {
		t.Fatalf("runner calls: %v", fake.calls)
	}
}

func TestSIInstallRefusals(t *testing.T) {
	ctx := context.Background()
	s, store, _ := siSetup(t, siBackend{})
	siWant(t, s.install(ctx, store, "bad name"), "invalid routine or project name")
	if err := s.install(ctx, store, "missing"); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("missing routine: %v, want os.ErrNotExist", err)
	}
	s.Home = "rel"
	siWant(t, s.install(ctx, store, "daily"), "timer home, Grid home and executable must be absolute")
}

func TestSIScheduleListFailure(t *testing.T) {
	s, store, _ := siSetup(t, siBackend{})
	err := s.Schedule(context.Background(), Store{GridHome: store.GridHome, Project: "bad name"})
	siWant(t, err, "project level needs a valid project name")
}

func TestSIChangedTimerFile(t *testing.T) {
	ctx := context.Background()
	s, store, fake := siSetup(t, siBackend{})
	label := timerLabel(store.GridHome, "", "daily")
	file := filepath.Join(s.Home, "timers", "daily.timer")
	clean := marker(label) + "\n"
	edited := marker(label) + " edited\n"
	read := func() string {
		t.Helper()
		b, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		return string(b)
	}
	if err := s.Schedule(ctx, store); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file, []byte(edited), 0o600); err != nil {
		t.Fatal(err)
	}
	fake.calls = nil
	if err := s.Schedule(ctx, store); err != nil {
		t.Fatalf("reinstall: %v", err)
	}
	if got := siCalls(fake.calls); len(got) != 2 || got[0] != "remove" || got[1] != "install" {
		t.Fatalf("reinstall calls: %v, want [remove install]", got)
	}
	if got := read(); got != clean {
		t.Fatalf("timer file: %q, want %q", got, clean)
	}
	if err := os.WriteFile(file, []byte(edited), 0o600); err != nil {
		t.Fatal(err)
	}
	fake.calls = nil
	fake.fail = true
	siWant(t, s.Schedule(ctx, store), "OS failure")
	if got := siCalls(fake.calls); len(got) != 1 || got[0] != "remove" {
		t.Fatalf("failed reinstall calls: %v, want [remove]", got)
	}
	if got := read(); got != edited {
		t.Fatalf("timer file: %q, want the edited text %q", got, edited)
	}
}
