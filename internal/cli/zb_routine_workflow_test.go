package cli

import (
	"encoding/json"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// zbFlowStep is one grid call and what it must exit with and print.
type zbFlowStep struct {
	args     []string
	code     int
	stdout   string
	stderr   string
	notInOut string
}

func TestZbFlowRoutineAddWorkflowAndReplace(t *testing.T) {
	h := newHarness(t)
	folder := filepath.Join(h.root, "work", "alpha")
	h.mkdir(folder)
	if code := h.run(folder, "project", "add"); code != 0 {
		t.Fatal(h.errb.String())
	}
	p := []string{"--project", "alpha"}
	with := func(args ...string) []string { return append(args, p...) }
	for _, step := range []zbFlowStep{
		{args: with("routine", "add", "nightly", "--every", "1h", "--workflow", "bugfix", "--json"), stdout: `"workflow":"bugfix"`},
		{args: with("routine", "list", "--json"), stdout: `"workflow":"bugfix"`},
		{args: with("routine", "list"), stdout: "workflow: bugfix"},
		{args: with("routine", "add", "x", "--every", "1h", "--run", "grid version", "--workflow", "bugfix"), code: 2},
		{args: with("routine", "add", "y", "--every", "1h"), code: 2},
		{args: with("routine", "add", "nightly", "--every", "1h", "--workflow", "other"), code: 1, stderr: "already exists"},
		{args: with("routine", "list", "--json"), stdout: `"workflow":"bugfix"`, notInOut: `"workflow":"other"`},
		{args: with("routine", "add", "nightly", "--every", "1h", "--workflow", "other", "--replace", "--json"), stdout: `"workflow":"other"`},
		{args: with("routine", "list", "--json"), stdout: `"workflow":"other"`, notInOut: `"workflow":"bugfix"`},
	} {
		code := h.run(folder, step.args...)
		if code != step.code || !strings.Contains(h.out.String(), step.stdout) || !strings.Contains(h.errb.String(), step.stderr) ||
			(step.notInOut != "" && strings.Contains(h.out.String(), step.notInOut)) {
			t.Fatalf("%q: exit %d, want %d\nstdout: %s\nstderr: %s", step.args, code, step.code, h.out, h.errb)
		}
	}
	for _, name := range []string{"x", "y"} {
		if strings.Contains(h.out.String(), `"name":"`+name+`"`) {
			t.Fatalf("refused routine %s was stored: %s", name, h.out)
		}
	}
}

func TestZbFlowRunRoutineListsNoNewKeys(t *testing.T) {
	h := newHarness(t)
	if code := h.run(h.root, "routine", "add", "daily", "--every", "1h", "--run", "grid version"); code != 0 {
		t.Fatal(h.errb.String())
	}
	if code := h.run(h.root, "routine", "list", "--json"); code != 0 {
		t.Fatal(h.errb.String())
	}
	var entries []map[string]json.RawMessage
	if err := json.Unmarshal(h.out.Bytes(), &entries); err != nil || len(entries) != 1 {
		t.Fatalf("list: %s %v", h.out, err)
	}
	var keys []string
	for key := range entries[0] {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	// The keys a never-run --run routine lists on base.
	want := []string{"every", "limits", "machine", "name", "next", "run", "scheduled"}
	if !slices.Equal(keys, want) {
		t.Fatalf("keys %q, want %q", keys, want)
	}
	if code := h.run(h.root, "routine", "list"); code != 0 || strings.Contains(h.out.String(), "workflow:") || strings.Contains(h.out.String(), "last_task:") || !strings.Contains(h.out.String(), "run: grid version\n") {
		t.Fatalf("text list: %s", h.out)
	}
}
