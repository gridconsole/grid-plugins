package cli

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/project"
)

// grRepoHarness is a temp home with project alpha registered. HOME, GRID_HOME,
// CLAUDE_CONFIG_DIR and CODEX_HOME all point inside the test's temp folder.
func grRepoHarness(t *testing.T) *harness {
	t.Helper()
	h := newHarness(t)
	t.Setenv("HOME", h.home)
	t.Setenv("GRID_HOME", h.gridHome)
	t.Setenv("CLAUDE_CONFIG_DIR", filepath.Join(h.home, ".claude"))
	t.Setenv("CODEX_HOME", filepath.Join(h.home, ".codex"))
	h.record(project.Record{Name: "alpha", ID: "01ALPHA"})
	return h
}

func grRepoPath(h *harness) string {
	return filepath.Join(h.gridHome, "projects", "alpha", "repos.json")
}

// grRepoRaw is the file as bytes; ok is false when it does not exist.
func grRepoRaw(t *testing.T, h *harness) (string, bool) {
	t.Helper()
	b, err := os.ReadFile(grRepoPath(h))
	if os.IsNotExist(err) {
		return "", false
	}
	if err != nil {
		t.Fatal(err)
	}
	return string(b), true
}

func grRepoSeed(t *testing.T, h *harness, text string) {
	t.Helper()
	h.write(grRepoPath(h), text)
}

// grRepoItems decodes the file as the app does (projectsettings jsonItems): one
// object, items an array, nothing after it.
func grRepoItems(t *testing.T, h *harness) []map[string]any {
	t.Helper()
	raw, ok := grRepoRaw(t, h)
	if !ok {
		t.Fatal("repos.json was not written")
	}
	var doc struct {
		Items []map[string]any `json:"items"`
	}
	dec := json.NewDecoder(bytes.NewReader([]byte(raw)))
	if err := dec.Decode(&doc); err != nil {
		t.Fatalf("repos.json: %v: %s", err, raw)
	}
	if err := dec.Decode(new(any)); err != io.EOF {
		t.Fatalf("repos.json has more after the first object: %v: %s", err, raw)
	}
	if doc.Items == nil {
		t.Fatalf("repos.json items is not an array: %s", raw)
	}
	return doc.Items
}

func TestGrProjectRepoAddWritesTheItem(t *testing.T) {
	h := grRepoHarness(t)
	if _, ok := grRepoRaw(t, h); ok {
		t.Fatal("repos.json exists before the first add")
	}
	if code := h.run(h.tmp, "project", "repo", "add", "alpha", "--where", "/tmp/x/app", "--remote", "git@h:a.git", "--branch", "main", "--json"); code != 0 {
		t.Fatalf("add: exit %d: %s", code, h.errb)
	}
	var answer struct {
		State string         `json:"state"`
		Item  map[string]any `json:"item"`
	}
	if err := json.Unmarshal(h.out.Bytes(), &answer); err != nil {
		t.Fatalf("add answer: %v: %s", err, h.out)
	}
	want := map[string]any{"id": "app", "where": "/tmp/x/app", "remote": "git@h:a.git", "branch": "main"}
	if answer.State != "applied" || !reflect.DeepEqual(answer.Item, want) {
		t.Fatalf("add answer = %s, want state applied and item %v", h.out, want)
	}
	if items := grRepoItems(t, h); !reflect.DeepEqual(items, []map[string]any{want}) {
		t.Fatalf("repos.json items = %v, want [%v]", items, want)
	}
	if raw, _ := grRepoRaw(t, h); raw != `{"items":[{"id":"app","where":"/tmp/x/app","remote":"git@h:a.git","branch":"main"}]}`+"\n" {
		t.Fatalf("repos.json bytes = %q", raw)
	}
	// A trailing separator does not make the id empty, and --id wins over where.
	if code := h.run(h.tmp, "project", "repo", "add", "alpha", "--where", "/tmp/x/lib/", "--json"); code != 0 {
		t.Fatalf("add lib: exit %d: %s", code, h.errb)
	}
	if code := h.run(h.tmp, "project", "repo", "add", "alpha", "--id", "docs", "--where", "/tmp/x/site"); code != 0 {
		t.Fatalf("add docs: exit %d: %s", code, h.errb)
	}
	var ids []any
	for _, it := range grRepoItems(t, h) {
		ids = append(ids, it["id"])
	}
	if !reflect.DeepEqual(ids, []any{"app", "lib", "docs"}) {
		t.Fatalf("ids = %v, want app, lib, docs", ids)
	}
}

func TestGrProjectRepoEditKeepsOtherKeys(t *testing.T) {
	h := grRepoHarness(t)
	grRepoSeed(t, h, `{"items":[{"id":"app","where":"/w/app","remote":"r","branch":"main","note":"keep <me> & \"this\""},{"id":"lib","where":"/w/lib","remote":"","branch":"","note":{"a":[1,2]}}]}`+"\n")
	if code := h.run(h.tmp, "project", "repo", "edit", "alpha", "--id", "app", "--branch", "dev", "--json"); code != 0 {
		t.Fatalf("edit: exit %d: %s", code, h.errb)
	}
	want := `{"items":[{"id":"app","where":"/w/app","remote":"r","branch":"dev","note":"keep <me> & \"this\""},{"id":"lib","where":"/w/lib","remote":"","branch":"","note":{"a":[1,2]}}]}` + "\n"
	if raw, _ := grRepoRaw(t, h); raw != want {
		t.Fatalf("after edit:\n got %q\nwant %q", raw, want)
	}
	var answer struct {
		State string         `json:"state"`
		Item  map[string]any `json:"item"`
	}
	if err := json.Unmarshal(h.out.Bytes(), &answer); err != nil || answer.State != "applied" || answer.Item["note"] != `keep <me> & "this"` || answer.Item["branch"] != "dev" {
		t.Fatalf("edit answer = %s (%v)", h.out, err)
	}
	// An empty value that is given clears the field; the id is not renamed.
	if code := h.run(h.tmp, "project", "repo", "edit", "alpha", "--id", "app", "--remote", ""); code != 0 {
		t.Fatalf("clear remote: exit %d: %s", code, h.errb)
	}
	want = `{"items":[{"id":"app","where":"/w/app","remote":"","branch":"dev","note":"keep <me> & \"this\""},{"id":"lib","where":"/w/lib","remote":"","branch":"","note":{"a":[1,2]}}]}` + "\n"
	if raw, _ := grRepoRaw(t, h); raw != want {
		t.Fatalf("after clearing remote:\n got %q\nwant %q", raw, want)
	}
}

func TestGrProjectRepoRemoveTakesOnlyThatItem(t *testing.T) {
	h := grRepoHarness(t)
	grRepoSeed(t, h, `{"items":[{"id":"app","where":"/w/app","remote":"","branch":"","note":"x"},{"id":"lib","where":"/w/lib","remote":"","branch":"","note":"y"}]}`+"\n")
	if code := h.run(h.tmp, "project", "repo", "remove", "alpha", "--id", "app", "--json"); code != 0 {
		t.Fatalf("remove: exit %d: %s", code, h.errb)
	}
	if got := h.out.String(); got != `{"state":"applied"}`+"\n" {
		t.Fatalf("remove answer = %q", got)
	}
	if raw, _ := grRepoRaw(t, h); raw != `{"items":[{"id":"lib","where":"/w/lib","remote":"","branch":"","note":"y"}]}`+"\n" {
		t.Fatalf("after remove: %q", raw)
	}
	if code := h.run(h.tmp, "project", "repo", "remove", "alpha", "--id", "lib"); code != 0 {
		t.Fatalf("remove last: exit %d: %s", code, h.errb)
	}
	if items := grRepoItems(t, h); len(items) != 0 {
		t.Fatalf("items after the last remove = %v", items)
	}
}

func TestGrProjectRepoRefusalsLeaveTheFileAlone(t *testing.T) {
	seed := `{"items":[{"id":"app","where":"/w/app","remote":"","branch":"","note":"x"}]}` + "\n"
	for _, tc := range []struct {
		name string
		seed string
		args []string
		exit int
	}{
		{"add with a taken id", seed, []string{"add", "alpha", "--id", "app", "--where", "/w/other"}, 1},
		{"add whose default id is taken", seed, []string{"add", "alpha", "--where", "/elsewhere/app"}, 1},
		{"edit of an unknown id", seed, []string{"edit", "alpha", "--id", "nope", "--branch", "x"}, 1},
		{"remove of an unknown id", seed, []string{"remove", "alpha", "--id", "nope"}, 1},
		{"edit that changes nothing", seed, []string{"edit", "alpha", "--id", "app"}, 2},
		{"edit to an empty where", seed, []string{"edit", "alpha", "--id", "app", "--where", ""}, 2},
		{"add without where", seed, []string{"add", "alpha", "--id", "x"}, 2},
		{"remove with a field flag", seed, []string{"remove", "alpha", "--id", "app", "--branch", "x"}, 2},
		{"unknown verb", seed, []string{"from", "alpha", "--id", "app"}, 2},
		{"missing project operand", seed, []string{"add", "--where", "/w/x"}, 2},
		{"a file that is not {items:[...]}", `{"items":{}}` + "\n", []string{"add", "alpha", "--where", "/w/x"}, 1},
		{"a file with text after the object", `{"items":[]} x` + "\n", []string{"add", "alpha", "--where", "/w/x"}, 1},
		{"a file that is empty", "", []string{"add", "alpha", "--where", "/w/x"}, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := grRepoHarness(t)
			grRepoSeed(t, h, tc.seed)
			if code := h.run(h.tmp, append([]string{"project", "repo"}, tc.args...)...); code != tc.exit {
				t.Fatalf("exit %d, want %d: stdout %q stderr %q", code, tc.exit, h.out, h.errb)
			}
			if h.errb.Len() == 0 {
				t.Fatal("a refusal says why on stderr")
			}
			if raw, _ := grRepoRaw(t, h); raw != tc.seed {
				t.Fatalf("repos.json changed: %q, was %q", raw, tc.seed)
			}
		})
	}
}

func TestGrProjectRepoGuestIsRefused(t *testing.T) {
	for _, role := range []string{"guest", "grid-guest"} {
		t.Run(role, func(t *testing.T) {
			h := grRepoHarness(t)
			h.env.Getenv = func(k string) string { return map[string]string{"GRID_ROLE": role, "GRID_MEMBER": "gus"}[k] }
			for _, args := range [][]string{
				{"add", "alpha", "--where", "/w/x"},
				{"edit", "alpha", "--id", "x", "--branch", "b"},
				{"remove", "alpha", "--id", "x"},
			} {
				if code := h.run(h.tmp, append([]string{"project", "repo"}, args...)...); code != 1 {
					t.Fatalf("%v: exit %d, want 1: %s", args, code, h.errb)
				}
				if got := h.errb.String(); got != "grid: "+guestSettingsRefusal+"\n" {
					t.Fatalf("%v: stderr %q", args, got)
				}
				if _, ok := grRepoRaw(t, h); ok {
					t.Fatalf("%v: a guest wrote repos.json", args)
				}
			}
		})
	}
}

func TestGrProjectRepoUnknownProjectCode(t *testing.T) {
	for _, args := range [][]string{
		{"add", "absent", "--where", "/w/x"},
		{"edit", "absent", "--id", "x", "--branch", "b"},
		{"remove", "absent", "--id", "x"},
	} {
		h := grRepoHarness(t)
		base := append([]string{"project", "repo"}, args...)
		if code := h.run(h.tmp, base...); code != 1 || h.out.Len() != 0 {
			t.Fatalf("%v: exit %d stdout %q, want 1 and no stdout: %s", args, code, h.out, h.errb)
		}
		plain := h.errb.String()
		if code := h.run(h.tmp, append(base, "--json")...); code != 1 {
			t.Fatalf("%v --json: exit %d: %s", args, code, h.errb)
		}
		if h.errb.String() != plain {
			t.Fatalf("%v: stderr with --json %q differs from %q", args, h.errb, plain)
		}
		assertErrorOutput(t, h.out.String(), h.errb.String(), CodeNoSuchProject)
		if _, err := os.Stat(filepath.Join(h.gridHome, "projects", "absent")); !os.IsNotExist(err) {
			t.Fatalf("%v: the unknown project got a folder: %v", args, err)
		}
	}
}

func TestGrProjectRepoFindsTheProjectById(t *testing.T) {
	h := grRepoHarness(t)
	if code := h.run(h.tmp, "project", "repo", "add", "01ALPHA", "--where", "/w/app"); code != 0 {
		t.Fatalf("exit %d: %s", code, h.errb)
	}
	if items := grRepoItems(t, h); len(items) != 1 || items[0]["id"] != "app" {
		t.Fatalf("items = %v", items)
	}
}
