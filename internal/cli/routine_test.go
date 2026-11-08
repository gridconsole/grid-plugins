package cli

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gridconsole/grid-plugins/internal/brain"
	"github.com/gridconsole/grid-plugins/internal/routine"
)

type routineOS struct{}

func (routineOS) Run(context.Context, routine.OSCommand) error { return nil }

func TestRoutineChangedTimerRecordsOneSignal(t *testing.T) {
	for _, backend := range []routine.Backend{routine.Systemd{}, routine.Launchd{UID: 501}, routine.Schtasks{}} {
		for _, lastRun := range []time.Time{{}, time.Date(2026, 1, 2, 3, 4, 0, 0, time.UTC)} {
			h := newHarness(t)
			store := routine.Store{GridHome: h.gridHome}
			e, err := store.Add("daily", "1h", "grid version")
			if err != nil {
				t.Fatal(err)
			}
			scheduler := routine.Scheduler{Home: h.home, GridHome: h.gridHome, Binary: filepath.Join(h.root, "grid"), Backend: backend, Runner: routineOS{}}
			if err := scheduler.Schedule(context.Background(), store); err != nil {
				t.Fatal(err)
			}
			e.Run, e.LastRun = "grid brain index", lastRun
			if !lastRun.IsZero() {
				e.Outcome = "success"
			}
			if err := (brain.Store{GridHome: h.gridHome}).EditConfig("home", "routines", e.Name, func([]byte) ([]byte, error) {
				return brain.EncodeConfig(e, "")
			}); err != nil {
				t.Fatal(err)
			}
			h.env.Executable = func() (string, error) { return filepath.Join(h.root, "missing-grid"), nil }
			for i := 0; i < 2; i++ {
				if code := h.run(h.root, "routine", "run", "daily", "--due", "--json"); code != 1 || !strings.Contains(h.errb.String(), "grid routine schedule") {
					t.Fatalf("refusal: exit %d: %s", code, h.errb)
				}
				got, err := store.Get(e.Name)
				if err != nil || !got.LastRun.Equal(e.LastRun) || got.Outcome != e.Outcome {
					t.Fatalf("refusal changed run state: %+v %v", got, err)
				}
			}
			notes := filepath.Join(h.gridHome, "projects", "personal", "notes")
			files, err := os.ReadDir(notes)
			if err != nil || len(files) != 1 {
				t.Fatalf("expected one Personal note: %v %v", files, err)
			}
			body, err := os.ReadFile(filepath.Join(notes, files[0].Name()))
			if err != nil || !strings.Contains(string(body), "grid routine schedule") {
				t.Fatalf("note lacks approval instruction: %s %v", body, err)
			}
		}
	}
}

func TestRoutineCLI(t *testing.T) {
	h := newHarness(t)
	for _, tc := range []struct {
		args     []string
		code     int
		contains string
	}{
		{[]string{"routine", "list", "--json"}, 0, "[]"},
		{[]string{"routine", "add", "daily", "--every", "1h", "--run", "grid version", "--json"}, 0, `"name":"daily"`},
		{[]string{"routine", "list"}, 0, "every: 1h"},
		{[]string{"routine", "add", "daily", "--every", "1h", "--run", "grid version"}, 1, ""},
		{[]string{"routine", "add", "bad", "--every", "bad", "--run", "signals"}, 1, ""},
		{[]string{"routine", "add", "missing-flags"}, 1, ""},
		{[]string{"routine", "run"}, 2, ""},
		{[]string{"routine", "list", "extra"}, 2, ""},
		{[]string{"routine", "missing"}, 2, ""},
		{[]string{"routine", "remove", "daily"}, 0, "result: remove"},
		{[]string{"routine", "list", "--json"}, 0, "[]"},
		{[]string{"worker", "list", "--json"}, 0, "[]"},
		{[]string{"worker", "start"}, 2, ""},
	} {
		if code := h.run(h.root, tc.args...); code != tc.code {
			t.Fatalf("%v: exit %d: %s", tc.args, code, h.errb)
		}
		if tc.contains != "" && !strings.Contains(h.out.String(), tc.contains) {
			t.Fatalf("%v: %s", tc.args, h.out)
		}
	}
}

func TestRoutineRefusalAfterFailedRunKeepsItsOwnNote(t *testing.T) {
	h := newHarness(t)
	store := routine.Store{GridHome: h.gridHome}
	if _, err := store.Add("daily", "1h", "grid version"); err != nil {
		t.Fatal(err)
	}
	scheduler := routine.Scheduler{Home: h.home, GridHome: h.gridHome, Binary: filepath.Join(h.root, "grid"), Backend: routine.Systemd{}, Runner: routineOS{}}
	if err := scheduler.Schedule(context.Background(), store); err != nil {
		t.Fatal(err)
	}
	h.env.Executable = func() (string, error) { return filepath.Join(h.root, "missing-grid"), nil }
	if code := h.run(h.root, "routine", "run", "daily", "--due", "--json"); code != 1 {
		t.Fatalf("failing run: exit %d: %s", code, h.errb)
	}
	failed, err := store.Get("daily")
	if err != nil || failed.Outcome != "failed" || failed.LastRun.IsZero() {
		t.Fatalf("run did not fail: %+v %v", failed, err)
	}
	failed.Run = "grid brain index"
	if err := (brain.Store{GridHome: h.gridHome}).EditConfig("home", "routines", failed.Name, func([]byte) ([]byte, error) {
		return brain.EncodeConfig(failed, "")
	}); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		h.errb.Reset()
		if code := h.run(h.root, "routine", "run", "daily", "--due", "--json"); code != 1 || !strings.Contains(h.errb.String(), "grid routine schedule") {
			t.Fatalf("refusal: exit %d: %s", code, h.errb)
		}
	}
	notes := filepath.Join(h.gridHome, "projects", "personal", "notes")
	files, err := os.ReadDir(notes)
	if err != nil || len(files) != 2 {
		t.Fatalf("expected the failure note and one refusal note: %v %v", files, err)
	}
	var refusals, failures int
	for _, f := range files {
		body, err := os.ReadFile(filepath.Join(notes, f.Name()))
		if err != nil {
			t.Fatal(err)
		}
		switch {
		case strings.Contains(string(body), "grid routine schedule"):
			refusals++
		case strings.Contains(string(body), "Run grid routine list"):
			failures++
		}
	}
	if refusals != 1 || failures != 1 {
		t.Fatalf("notes: %d refusal, %d failure", refusals, failures)
	}
}

func TestRoutineHelpExplainsEntryLock(t *testing.T) {
	h := newHarness(t)
	for _, args := range [][]string{{"routine", "--help"}, {"routine", "run", "--help"}} {
		if code := h.run(h.root, args...); code != 0 {
			t.Fatalf("help: %d %s", code, h.errb)
		}
		text := h.out.String() + h.errb.String()
		if !strings.Contains(text, "entry lock until execution finishes") || !strings.Contains(text, "waits for it") {
			t.Fatalf("missing lock help: %s", text)
		}
	}
}

func TestRoutineScheduleRejectsGridHomeWhitespace(t *testing.T) {
	h := newHarness(t)
	h.env.GridHome = h.gridHome + " "
	if code := h.run(h.root, "routine", "add", "daily", "--every", "1h", "--run", "grid version"); code != 0 {
		t.Fatal(h.errb.String())
	}
	if code := h.run(h.root, "routine", "schedule"); code != 1 || !strings.Contains(h.errb.String(), "Grid home must not have leading or trailing whitespace") {
		t.Fatalf("schedule: exit %d: %s", code, h.errb)
	}
}

func TestRoutineRemoveAndUnscheduleWorkWithGridHomeWhitespace(t *testing.T) {
	h := newHarness(t)
	h.env.GridHome = h.gridHome + " "
	if code := h.run(h.root, "routine", "add", "daily", "--every", "1h", "--run", "grid version"); code != 0 {
		t.Fatal(h.errb.String())
	}
	h.errb.Reset()
	if code := h.run(h.root, "routine", "unschedule"); code != 0 {
		t.Fatalf("unschedule: exit %d: %s", code, h.errb)
	}
	if code := h.run(h.root, "routine", "remove", "daily"); code != 0 {
		t.Fatalf("remove: exit %d: %s", code, h.errb)
	}
	h.out.Reset()
	if code := h.run(h.root, "routine", "list", "--json"); code != 0 || strings.TrimSpace(h.out.String()) != "[]" {
		t.Fatalf("routine remains: exit %d: %s %s", code, h.out, h.errb)
	}
}

func TestRoutineProjectScope(t *testing.T) {
	h := newHarness(t)
	folder := filepath.Join(h.root, "work", "example")
	h.mkdir(folder)
	if code := h.run(folder, "project", "add"); code != 0 {
		t.Fatal(h.errb.String())
	}
	for _, project := range []string{"", "example"} {
		args := []string{"routine", "add", "same-name", "--every", "*/5 * * * *", "--run", "signals"}
		if project != "" {
			args = append(args, "--project", project)
		}
		if code := h.run(folder, args...); code != 0 {
			t.Fatal(h.errb.String())
		}
	}
	if code := h.run(folder, "routine", "remove", "same-name", "--project", "example"); code != 0 {
		t.Fatal(h.errb.String())
	}
	if code := h.run(folder, "routine", "list", "--json"); code != 0 {
		t.Fatal(h.errb.String())
	}
	var entries []routine.Entry
	if err := json.Unmarshal(h.out.Bytes(), &entries); err != nil || len(entries) != 1 {
		t.Fatalf("home lost: %s %v", h.out, err)
	}
}

func TestRoutineFailureRecordsSignal(t *testing.T) {
	h := newHarness(t)
	if code := h.run(h.root, "routine", "add", "failing", "--every", "1h", "--run", "grid version"); code != 0 {
		t.Fatal(h.errb.String())
	}
	h.env.Executable = func() (string, error) { return filepath.Join(h.root, "missing-grid"), nil }
	if code := h.run(h.root, "routine", "run", "failing", "--json"); code != 1 {
		t.Fatalf("exit %d: %s", code, h.errb)
	}
	var result routine.RunResult
	if err := json.Unmarshal(h.out.Bytes(), &result); err != nil || result.Outcome != "failed" || result.LastRun.IsZero() {
		t.Fatalf("result: %s %v", h.out, err)
	}
	files, err := os.ReadDir(filepath.Join(h.gridHome, "projects", "personal", "notes"))
	if err != nil || len(files) != 1 {
		t.Fatalf("home failure signal: %v %v: %s", files, err, h.errb)
	}
	if code := h.run(h.root, "routine", "run", "failing", "--due", "--json"); code != 0 {
		t.Fatal(h.errb.String())
	}
	if err := json.Unmarshal(h.out.Bytes(), &result); err != nil || !result.Skipped {
		t.Fatalf("guard: %s %v", h.out, err)
	}
}

func TestRoutineFailureMatchesPreL42Signal(t *testing.T) {
	h := newHarness(t)
	// This fixture retains the pre-L42 source ID and second-precision record
	// timestamps. A retry must find it without rewriting or reopening it.
	const id = "S-6fc2ee1fb06253bcdfabe9c06b3a17a161952020a1d6d1a963447fadccac5834"
	const oldNote = `---
id: ` + id + `
source: routine
source_id: example/daily:2026-01-02T03:04:05.123456789Z
title: "Routine failed: daily"
status: dismissed
captured: "2026-01-02T03:04:06Z"
updated: "2026-01-02T03:05:00Z"
tier: external
body_kind: data
---
Previously recorded failure.
`
	dir := filepath.Join(h.gridHome, "projects", "personal", "notes")
	path := filepath.Join(dir, id+".md")
	h.write(path, oldNote)
	entry := routine.Entry{Name: "daily", LastRun: time.Date(2026, 1, 2, 3, 4, 5, 123456789, time.UTC)}
	if err := routineFailureSignal(h.env, "example")(entry, errors.New("failed")); err != nil {
		t.Fatal(err)
	}
	files, err := os.ReadDir(dir)
	if err != nil || len(files) != 1 || files[0].Name() != id+".md" {
		t.Fatalf("retry did not match old signal: %v %v", files, err)
	}
	data, err := os.ReadFile(path)
	if err != nil || string(data) != oldNote {
		t.Fatalf("retry changed old signal: %s %v", data, err)
	}
}
