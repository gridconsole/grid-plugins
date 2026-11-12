package cli

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/brain"
	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/project"
)

// These tests run grid brain search, changes, write and drop with --json on a
// harness home that holds one project brain, and check each stdout against its
// cmd-brain-* schema. The app reads search hits (path, description, summary) and
// changes rows from this stdout; for write and drop it keeps only the exit code.

// yjBrOut is the stdout of every verb in one scenario: an empty project brain,
// then two notes written, a search that finds both, one note dropped.
type yjBrOut struct {
	searchNone, searchTwo   []byte
	changesEmpty, changesAt []byte
	writeFirst, writeSecond []byte
	drop                    []byte
}

func yjBrNote(name, description string) string {
	b, err := brain.Encode(brain.Note{Name: name, Topic: "decisions", Type: "reference",
		Description: description, Summary: "What the " + name + " note says.",
		Source: "person:member", Level: "project", Status: "accepted", Origin: brain.Origin{Kind: "session", ID: "s0"},
		Links: []brain.Link{}, Body: "# " + name + "\nThe " + name + " body.\n"})
	if err != nil {
		panic(err)
	}
	return string(b)
}

func yjBrScenario(t *testing.T) yjBrOut {
	t.Helper()
	h := newHarness(t)
	for _, key := range strings.Fields("GRID_AGENT GRID_WORKER_ID GRID_WORKER GRID_LEASE GRID_LEASE_EPOCH GRID_WAKE GRID_FEED_TASK GRID_JOB GRID_ATTEMPT GRID_SESSION_ID CLAUDE_SESSION_ID GRID_OWNER") {
		t.Setenv(key, "")
	}
	for _, key := range []string{"GRID_MEMBER", "GRID_ACTOR", "GRID_WHO"} {
		t.Setenv(key, "member")
	}
	t.Setenv("GRID_ROLE", "owner")
	cwd := filepath.Join(h.root, "example")
	h.mkdir(cwd)
	h.record(project.Record{Name: "example", Folders: []project.Folder{{Machine: h.env.Machine, Path: cwd}}})
	run := func(args ...string) []byte {
		t.Helper()
		if code := h.run(cwd, args...); code != 0 {
			t.Fatalf("%s: exit %d: %s", strings.Join(args, " "), code, h.errb)
		}
		out := append([]byte(nil), h.out.Bytes()...)
		t.Logf("%s -> %s", strings.Join(args, " "), out)
		return out
	}
	rev := func(out []byte) string {
		t.Helper()
		var got struct{ Rev string }
		if err := json.Unmarshal(out, &got); err != nil || got.Rev == "" {
			t.Fatalf("no rev in %s: %v", out, err)
		}
		return got.Rev
	}
	write := func(name, description string) []byte {
		file := filepath.Join(h.root, name+".md")
		h.write(file, yjBrNote(name, description))
		return run("brain", "write", "decisions/"+name, "--expect", "none", "--file", file, "--project", "example", "--json")
	}
	var o yjBrOut
	o.changesEmpty = run("brain", "changes", "--level", "project", "--project", "example", "--json")
	o.searchNone = run("brain", "search", "--project", "example", "--limit", "20", "--json", "--", "ferry schedule")
	o.writeFirst = write("ferry", "Read before changing the ferry schedule.")
	o.writeSecond = write("harbour", "Read before changing the ferry harbour.")
	o.searchTwo = run("brain", "search", "--project", "example", "--limit", "20", "--json", "--", "ferry")
	o.drop = run("brain", "drop", "decisions/harbour", "--expect", rev(o.writeSecond), "--project", "example", "--json")
	o.changesAt = run("brain", "changes", "--level", "project", "--project", "example", "--json")
	return o
}

// yjBrRename renames key from to to in a copy of the stdout: in the top-level
// object when list is "", else in every element of the array list.
func yjBrRename(t *testing.T, raw []byte, list, from, to string) []byte {
	t.Helper()
	var top map[string]any
	if err := json.Unmarshal(raw, &top); err != nil {
		t.Fatal(err)
	}
	rename := func(m map[string]any) {
		v, ok := m[from]
		if !ok {
			t.Fatalf("no %q in %s", from, raw)
		}
		delete(m, from)
		m[to] = v
	}
	if list == "" {
		rename(top)
	} else {
		items, _ := top[list].([]any)
		if len(items) == 0 {
			t.Fatalf("no %s rows in %s", list, raw)
		}
		for _, item := range items {
			rename(item.(map[string]any))
		}
	}
	out, err := json.Marshal(top)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func yjBrValid(t *testing.T, schema string, out []byte) {
	t.Helper()
	if err := contract.Validate(schema, out); err != nil {
		t.Fatalf("%s: %v\n%s", schema, err, out)
	}
}

func yjBrRows(t *testing.T, out []byte, list string, want int) {
	t.Helper()
	var top map[string]json.RawMessage
	var rows []json.RawMessage
	if err := json.Unmarshal(out, &top); err == nil {
		err = json.Unmarshal(top[list], &rows)
	}
	if len(rows) != want {
		t.Fatalf("%s: want %d rows, got %d in %s", list, want, len(rows), out)
	}
}

// yjBrRenamed checks that the stdout passes as written and fails once the app's
// field is renamed, so a failure is the missing field and not a missing schema.
func yjBrRenamed(t *testing.T, schema string, out []byte, list, from, to string) {
	t.Helper()
	yjBrValid(t, schema, out)
	if err := contract.Validate(schema, yjBrRename(t, out, list, from, to)); err == nil {
		t.Fatalf("%s accepts stdout with %s renamed to %s", schema, from, to)
	}
}

func TestYjBrainSearch(t *testing.T) {
	o := yjBrScenario(t)
	t.Run("no-hit", func(t *testing.T) { yjBrValid(t, "cmd-brain-search", o.searchNone) })
	t.Run("two-hits", func(t *testing.T) {
		yjBrRows(t, o.searchTwo, "hits", 2)
		yjBrValid(t, "cmd-brain-search", o.searchTwo)
	})
	t.Run("renamed-summary", func(t *testing.T) {
		yjBrRenamed(t, "cmd-brain-search", o.searchTwo, "hits", "summary", "snippet")
	})
}

func TestYjBrainChanges(t *testing.T) {
	o := yjBrScenario(t)
	t.Run("empty", func(t *testing.T) { yjBrValid(t, "cmd-brain-changes", o.changesEmpty) })
	t.Run("after-write", func(t *testing.T) {
		yjBrRows(t, o.changesAt, "changes", 3)
		yjBrValid(t, "cmd-brain-changes", o.changesAt)
	})
	t.Run("renamed-origin-id", func(t *testing.T) {
		yjBrRenamed(t, "cmd-brain-changes", o.changesAt, "changes", "origin_id", "origin")
	})
}

func TestYjBrainWrite(t *testing.T) {
	o := yjBrScenario(t)
	t.Run("new-note", func(t *testing.T) {
		yjBrValid(t, "cmd-brain-write", o.writeFirst)
		yjBrValid(t, "cmd-brain-write", o.writeSecond)
	})
	t.Run("renamed-rev", func(t *testing.T) {
		yjBrRenamed(t, "cmd-brain-write", o.writeFirst, "", "rev", "etag")
	})
}

func TestYjBrainDrop(t *testing.T) {
	o := yjBrScenario(t)
	t.Run("dropped", func(t *testing.T) { yjBrValid(t, "cmd-brain-drop", o.drop) })
	// The plugin writes status, version and rev; "state" is the field of the
	// app's own HTTP answer (hy_knowledge_drop.go hyKdAnswer), not of this stdout.
	t.Run("renamed-status", func(t *testing.T) {
		yjBrRenamed(t, "cmd-brain-drop", o.drop, "", "status", "state")
	})
	t.Run("renamed-rev", func(t *testing.T) {
		yjBrRenamed(t, "cmd-brain-drop", o.drop, "", "rev", "etag")
	})
}
