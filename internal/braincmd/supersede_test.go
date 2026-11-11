package braincmd

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/brain"
)

// Tests for grid brain supersede, through the same entry point the CLI uses.

const svReason = "moved to v2 pipeline"

// svNotes proposes and accepts one home note per name.
func (h *harness) svNotes(t *testing.T, names ...string) {
	t.Helper()
	for _, name := range names {
		h.propose(t, "--name", name)
		if code := h.run("accept", name, "--level", "home"); code != 0 {
			t.Fatalf("accept %s: %d %s", name, code, &h.err)
		}
	}
}

// svTree reads every file under the Grid home but the lock file, so a refusal
// can be shown to have written nothing.
func svTree(t *testing.T, h *harness) map[string]string {
	t.Helper()
	tree := map[string]string{}
	err := filepath.WalkDir(h.env.GridHome, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || strings.Contains(d.Name(), "lock") {
			return err
		}
		rel, _ := filepath.Rel(h.env.GridHome, path)
		b, err := os.ReadFile(path)
		tree[filepath.ToSlash(rel)] = string(b)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return tree
}

func (h *harness) svVersions(t *testing.T, name string) int {
	t.Helper()
	dir := filepath.Join(h.env.GridHome, "knowledge", "versions", "decisions", name)
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	return len(entries)
}

func (h *harness) svRead(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(h.env.GridHome, "knowledge", "decisions", name+".md"))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// The command from the brief: before the verb it was an unknown command, exit 2.
func TestSupersedeMarksTheOldNoteThroughRun(t *testing.T) {
	h := newHarness(t)
	h.svNotes(t, "deploy", "deploy-v2")
	oldBefore := h.svRead(t, "deploy")

	code := h.run("supersede", "deploy", "--by", "deploy-v2", "--reason", svReason)
	if code != 0 || strings.Contains(h.err.String(), "unknown command") {
		t.Fatalf("exit %d: %s", code, &h.err)
	}
	if got := h.out.String(); got != "brain:home/deploy superseded by brain:home/deploy-v2 (v2, v2)\n" {
		t.Fatalf("text output: %q", got)
	}

	// The old file is still there with the field, and the rest unchanged.
	oldAfter := h.svRead(t, "deploy")
	want, err := brain.Parse([]byte(oldBefore))
	if err != nil {
		t.Fatal(err)
	}
	want.SupersededBy = "brain:home/deploy-v2"
	if got, err := brain.Parse([]byte(oldAfter)); err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("old note: %v\n got %#v\nwant %#v", err, got, want)
	}
	if !strings.Contains(oldAfter, "superseded_by: brain:home/deploy-v2") {
		t.Fatalf("old file lacks the field:\n%s", oldAfter)
	}
	// The new note has one supersedes link carrying the reason.
	got, err := brain.Parse([]byte(h.svRead(t, "deploy-v2")))
	if err != nil || len(got.Links) != 1 || got.Links[0].Rel != "supersedes" || got.Links[0].To != "brain:home/deploy" || got.Links[0].Why != svReason {
		t.Fatalf("new note links: %+v, %v", got.Links, err)
	}
	// One new version for each file.
	if o, n := h.svVersions(t, "deploy"), h.svVersions(t, "deploy-v2"); o != 2 || n != 2 {
		t.Fatalf("versions: old %d, new %d, want 2 and 2", o, n)
	}
	// changes --json shows the old note superseded, with the reason.
	changes := h.changes(t).Changes
	found := false
	for _, c := range changes {
		if c.Status == "superseded" && c.Name == "deploy" && c.Version == 2 && strings.Contains(c.Reason, svReason) {
			found = true
		}
	}
	if !found || changes[0].Status != "linked" && changes[0].Status != "superseded" {
		t.Fatalf("changes: %+v", changes)
	}
	// Search leaves the old note out; show still reads it.
	if code := h.run("search", "checks", "--level", "home", "--json"); code != 0 {
		t.Fatalf("search: %d %s", code, &h.err)
	}
	var found2 struct{ Hits []brain.Hit }
	if err := json.Unmarshal(h.out.Bytes(), &found2); err != nil || len(found2.Hits) != 1 || found2.Hits[0].ID != "brain:home/deploy-v2" {
		t.Fatalf("search: %v %+v", err, found2.Hits)
	}
	if code := h.run("show", "brain:home/deploy", "--json"); code != 0 || !strings.Contains(h.out.String(), `"superseded_by":"brain:home/deploy-v2"`) {
		t.Fatalf("show: %d %s %s", code, &h.out, &h.err)
	}
}

func TestSupersedeJSONAndRepeat(t *testing.T) {
	h := newHarness(t)
	h.svNotes(t, "deploy", "deploy-v2")
	if code := h.run("supersede", "deploy", "--by", "deploy-v2", "--reason", svReason, "--level", "home", "--json"); code != 0 {
		t.Fatalf("exit %d: %s", code, &h.err)
	}
	var out struct {
		Old      string
		New      string
		Versions struct{ Old, New int }
		Wrote    bool
	}
	check := func(wrote bool) {
		t.Helper()
		var raw map[string]json.RawMessage
		if err := json.Unmarshal(h.out.Bytes(), &raw); err != nil || len(raw) != 4 {
			t.Fatalf("json keys: %v %s", err, &h.out)
		}
		if err := json.Unmarshal(h.out.Bytes(), &out); err != nil {
			t.Fatal(err)
		}
		if out.Old != "brain:home/deploy" || out.New != "brain:home/deploy-v2" || out.Versions.Old != 2 || out.Versions.New != 2 || out.Wrote != wrote {
			t.Fatalf("json: %+v (%s)", out, &h.out)
		}
	}
	check(true)
	before := svTree(t, h)

	// The same command again: exit 0, "already superseded", no byte written.
	if code := h.run("supersede", "deploy", "--by", "deploy-v2", "--reason", svReason, "--level", "home"); code != 0 {
		t.Fatalf("repeat exit %d: %s", code, &h.err)
	}
	if got := h.out.String(); !strings.HasPrefix(got, "already superseded") || strings.Count(got, "\n") != 1 {
		t.Fatalf("repeat output: %q", got)
	}
	if after := svTree(t, h); !reflect.DeepEqual(before, after) {
		t.Fatal("the repeat wrote something")
	}
	if code := h.run("supersede", "deploy", "--by", "deploy-v2", "--reason", svReason, "--level", "home", "--json"); code != 0 {
		t.Fatalf("repeat json exit %d: %s", code, &h.err)
	}
	check(false)
	if o, n := h.svVersions(t, "deploy"), h.svVersions(t, "deploy-v2"); o != 2 || n != 2 {
		t.Fatalf("versions after the repeat: old %d, new %d", o, n)
	}
}

func TestSupersedeRefusalsExitAndWriteNothing(t *testing.T) {
	h := newHarness(t)
	h.svNotes(t, "deploy", "deploy-v2", "deploy-v3", "other")
	// deploy-v3 replaces deploy-v2 first, so deploy-v2 is superseded.
	if code := h.run("supersede", "deploy-v2", "--by", "deploy-v3", "--reason", svReason, "--level", "home"); code != 0 {
		t.Fatalf("setup: %d %s", code, &h.err)
	}
	before := svTree(t, h)
	for _, c := range []struct {
		name string
		role string
		args []string
		code int
		want string
	}{
		{"unknown old", "", []string{"missing", "--by", "deploy", "--reason", svReason}, 1, "not found"},
		{"unknown new", "", []string{"deploy", "--by", "missing", "--reason", svReason}, 1, "not found"},
		{"same note", "", []string{"deploy", "--by", "deploy", "--reason", svReason}, 1, "cannot supersede itself"},
		{"new note superseded", "", []string{"other", "--by", "deploy-v2", "--reason", svReason}, 1, "itself superseded"},
		{"old note superseded by another", "", []string{"deploy-v2", "--by", "other", "--reason", svReason}, 1, "already superseded by brain:home/deploy-v3"},
		{"project old, home new", "", []string{"brain:project/deploy", "--by", "brain:home/other", "--reason", svReason}, 1, "different levels"},
		{"home old, project new", "", []string{"brain:home/deploy", "--by", "brain:project/other", "--reason", svReason}, 1, "different levels"},
		{"empty reason", "", []string{"deploy", "--by", "other", "--reason", ""}, 1, "reason must"},
		{"one-word reason", "", []string{"deploy", "--by", "other", "--reason", "superseded"}, 1, "reason must"},
		{"short reason", "", []string{"deploy", "--by", "other", "--reason", "too old"}, 1, "reason must"},
		{"role refused", "guest", []string{"deploy", "--by", "other", "--reason", svReason}, 3, "not allowed"},
		{"no --by", "", []string{"deploy", "--reason", svReason}, 2, "--by"},
		{"no --reason", "", []string{"deploy", "--by", "other"}, 2, "--reason"},
		{"no old note", "", []string{"--by", "other", "--reason", svReason}, 2, "<old-note>"},
		{"two old notes", "", []string{"deploy", "other", "--by", "deploy-v2", "--reason", svReason}, 2, "<old-note>"},
		{"level disagrees with an id", "", []string{"brain:project/deploy", "--by", "other", "--reason", svReason, "--level", "home"}, 2, "disagrees"},
	} {
		t.Run(c.name, func(t *testing.T) {
			h.env.Role = c.role
			defer func() { h.env.Role = "" }()
			args := append([]string{"supersede"}, c.args...)
			if c.name != "home old, project new" && c.name != "project old, home new" && !strings.Contains(strings.Join(c.args, " "), "--level") {
				args = append(args, "--level", "home")
			}
			if code := h.run(args...); code != c.code || !strings.Contains(h.err.String(), c.want) || h.out.Len() != 0 {
				t.Fatalf("exit %d, want %d; stderr %q (want %q), stdout %q", code, c.code, &h.err, c.want, &h.out)
			}
			if after := svTree(t, h); !reflect.DeepEqual(before, after) {
				t.Fatal("a refusal wrote something")
			}
		})
	}
}
