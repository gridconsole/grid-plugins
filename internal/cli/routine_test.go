package cli

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gridconsole/grid-plugins/internal/routine"
)

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
