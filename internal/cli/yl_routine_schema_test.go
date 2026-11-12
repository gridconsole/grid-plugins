package cli

import (
	"bytes"
	"encoding/json"
	"maps"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/gridconsole/grid-plugins/internal/brain"
	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/routine"
)

// The machine id a routine is stamped with is random per Grid home; fixtures hold this one.
const ylRtMachine = "01ARZ3NDEKTSV4RRFFQ69G5FAW"

// The keys grid routine list and add always write; the app's Routines page reads
// name, every, run, scheduled and next of them (app internal/slices/routines/http.go:246-252).
var ylRtRequired = []string{"name", "every", "run", "machine", "limits", "scheduled", "next"}

func ylRtFixturePath(schema, kind, name string) string {
	return filepath.Join("..", "..", "contract", "fixtures", schema, kind, name+".json")
}

// ylRtRun runs grid in cwd, wants exit 0 and returns a copy of stdout.
func ylRtRun(t *testing.T, h *harness, cwd string, args ...string) []byte {
	t.Helper()
	if code := h.run(cwd, args...); code != 0 {
		t.Fatalf("%s: exit %d: %s", strings.Join(args, " "), code, h.errb)
	}
	return bytes.Clone(h.out.Bytes())
}

// ylRtNormal decodes stdout and sets the random machine id of every object to the fixture's.
func ylRtNormal(t *testing.T, out []byte) any {
	t.Helper()
	var value any
	if err := json.Unmarshal(out, &value); err != nil {
		t.Fatalf("stdout is not one JSON value: %v: %s", err, out)
	}
	objects := []any{value}
	if list, ok := value.([]any); ok {
		objects = list
	}
	for _, object := range objects {
		if m, ok := object.(map[string]any); ok {
			if _, has := m["machine"]; has {
				m["machine"] = ylRtMachine
			}
		}
	}
	return value
}

// ylRtCheck validates what the command wrote against its schema and holds it equal to the
// valid fixture of that name, so a fixture is output the plugin writes and not an invention.
func ylRtCheck(t *testing.T, schema, name string, out []byte) {
	t.Helper()
	if err := contract.Validate(schema, out); err != nil {
		t.Fatalf("%s: %v: %s", schema, err, out)
	}
	got := ylRtNormal(t, out)
	path := ylRtFixturePath(schema, "valid", name)
	if os.Getenv("UPDATE_CONTRACT_COMMAND_FIXTURES") == "1" {
		contractCommandWrite(t, path, got)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("%v; output was: %s", err, out)
	}
	var want any
	if err := json.Unmarshal(data, &want); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("%s output differs from fixture %s:\n got: %s\nwant: %s", schema, path, out, data)
	}
}

// ylRtHarness makes a registered project as the app reads it: routines of one project.
func ylRtHarness(t *testing.T) (*harness, string) {
	t.Helper()
	h := newHarness(t)
	t.Setenv("GRID_ROLE", "owner")
	folder := filepath.Join(h.root, "work", "alpha")
	h.mkdir(folder)
	ylRtRun(t, h, folder, "project", "add")
	return h, folder
}

func TestYlRoutineListAddSchemas(t *testing.T) {
	h, folder := ylRtHarness(t)
	p := []string{"--project", "alpha"}
	run := func(args ...string) []byte { return ylRtRun(t, h, folder, append(args, p...)...) }

	empty := run("routine", "list", "--json")
	if strings.TrimSpace(string(empty)) != "[]" {
		t.Fatalf("an empty project lists %q, not []", empty)
	}
	ylRtCheck(t, "cmd-routine-list", "basic", empty)

	ylRtCheck(t, "cmd-routine-add", "basic", run("routine", "add", "morning", "--every", "1h", "--run", "grid version", "--json"))
	ylRtCheck(t, "cmd-routine-add", "workflow", run("routine", "add", "nightly", "--every", "30 2 * * *", "--workflow", "bugfix",
		"--routine-timeout", "20m", "--routine-max-turns", "25", "--routine-budget", "2.5", "--json"))
	// Two routines, none run yet: valid, and what the Routines page shows before a first run.
	first := run("routine", "list", "--json")
	if err := contract.Validate("cmd-routine-list", first); err != nil {
		t.Fatalf("%v: %s", err, first)
	}

	// A last run without the scheduler: the routine file as a finished run leaves it, written
	// the way routine_test.go writes one. routine run, schedule and unschedule are not called.
	store := routine.Store{GridHome: h.gridHome, Project: "alpha"}
	entry, err := store.Get("nightly")
	if err != nil {
		t.Fatal(err)
	}
	entry.LastRun, entry.Outcome, entry.LastTask = time.Date(2026, 11, 10, 2, 30, 0, 0, time.UTC), "success", "T-07"
	if err := (brain.Store{GridHome: h.gridHome, Project: "alpha"}).EditConfig("project", "routines", "nightly", func([]byte) ([]byte, error) {
		return brain.EncodeConfig(entry, "")
	}); err != nil {
		t.Fatal(err)
	}
	// --replace keeps last_run, outcome and last_task, and add lists them.
	replaced := run("routine", "add", "nightly", "--every", "30 2 * * *", "--workflow", "bugfix",
		"--routine-timeout", "20m", "--routine-max-turns", "25", "--routine-budget", "2.5", "--replace", "--json")
	ylRtCheck(t, "cmd-routine-add", "replaced", replaced)
	var kept struct {
		LastRun  string `json:"last_run"`
		Outcome  string `json:"outcome"`
		LastTask string `json:"last_task"`
	}
	if err := json.Unmarshal(replaced, &kept); err != nil || kept.LastRun != "2026-11-10T02:30:00Z" || kept.Outcome != "success" || kept.LastTask != "T-07" {
		t.Fatalf("replace dropped the last run: %+v %v", kept, err)
	}
	// Two routines, one with a last run; neither is scheduled, so next is null.
	populated := run("routine", "list", "--json")
	ylRtCheck(t, "cmd-routine-list", "populated", populated)
	var rows []map[string]any
	if err := json.Unmarshal(populated, &rows); err != nil || len(rows) != 2 {
		t.Fatalf("expected two routines: %v: %s", err, populated)
	}
	if _, ok := rows[0]["last_run"]; ok || rows[1]["last_run"] != "2026-11-10T02:30:00Z" {
		t.Fatalf("expected a last run on nightly only: %s", populated)
	}
	for _, row := range rows {
		if row["scheduled"] != false || row["next"] != nil {
			t.Fatalf("unscheduled routine lists scheduled/next wrongly: %v", row)
		}
	}
}

// ylRtRenames checks that renaming or dropping a key the app reads, or giving one the wrong type, is refused.
func ylRtRenames(t *testing.T, entry map[string]any, schema string, wrap func(map[string]any) any) {
	t.Helper()
	for _, key := range ylRtRequired {
		for _, how := range []string{"renamed", "dropped"} {
			copied := maps.Clone(entry)
			value := copied[key]
			delete(copied, key)
			if how == "renamed" {
				copied[key+"_x"] = value
			}
			b, err := json.Marshal(wrap(copied))
			if err != nil {
				t.Fatal(err)
			}
			if contract.Validate(schema, b) == nil {
				t.Errorf("%s accepts an entry with %s %s", schema, key, how)
			}
		}
	}
	// A field of the wrong type is refused too.
	for key, bad := range map[string]any{"every": 60, "scheduled": "false", "next": false, "last_run": "yesterday", "outcome": "done", "limits": "10m"} {
		copied := maps.Clone(entry)
		copied[key] = bad
		b, _ := json.Marshal(wrap(copied))
		if contract.Validate(schema, b) == nil {
			t.Errorf("%s accepts %s = %v", schema, key, bad)
		}
	}
}

func TestYlRoutineSchemasKeepAppFields(t *testing.T) {
	var rows []map[string]any
	data, err := os.ReadFile(ylRtFixturePath("cmd-routine-list", "valid", "populated"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &rows); err != nil || len(rows) != 2 {
		t.Fatalf("populated list fixture: %v", err)
	}
	// The row with a last run holds every key the plugin can write.
	entry := rows[1]
	for _, key := range append([]string{"workflow", "last_run", "outcome", "last_task"}, ylRtRequired...) {
		if _, ok := entry[key]; !ok {
			t.Fatalf("populated fixture lacks %s", key)
		}
	}
	ylRtRenames(t, entry, "cmd-routine-list", func(m map[string]any) any { return []any{m, rows[0]} })
	ylRtRenames(t, entry, "cmd-routine-add", func(m map[string]any) any { return m })

	// The top level is an array for list and an object for add; the other shape is refused.
	object, _ := json.Marshal(entry)
	if err := contract.Validate("cmd-routine-list", object); err == nil {
		t.Error("cmd-routine-list accepts a top-level object")
	}
	array, _ := json.Marshal([]any{entry})
	if err := contract.Validate("cmd-routine-add", array); err == nil {
		t.Error("cmd-routine-add accepts a top-level array")
	}
	// The limits object lists every limit and nothing else.
	for _, change := range []func(map[string]any){
		func(l map[string]any) { delete(l, "routine_budget") },
		func(l map[string]any) { l["routine_cost"] = 1 },
	} {
		copied := maps.Clone(entry)
		limits := maps.Clone(copied["limits"].(map[string]any))
		change(limits)
		copied["limits"] = limits
		b, _ := json.Marshal(copied)
		if contract.Validate("cmd-routine-add", b) == nil {
			t.Errorf("cmd-routine-add accepts limits %v", limits)
		}
	}
}

// The two schemas describe one entry; a field added to one and not the other fails here.
func TestYlRoutineListAndAddDescribeOneEntry(t *testing.T) {
	read := func(name string) map[string]any {
		data, err := contract.Schema(name)
		if err != nil {
			t.Fatal(err)
		}
		var schema map[string]any
		if err := json.Unmarshal(data, &schema); err != nil {
			t.Fatal(err)
		}
		return schema
	}
	list, add := read("cmd-routine-list"), read("cmd-routine-add")
	item, _ := list["items"].(map[string]any)
	for _, schema := range []map[string]any{item, add} {
		if schema == nil {
			t.Fatal("schema has no entry object")
		}
	}
	if !reflect.DeepEqual(item, ylRtWithout(add, "$schema", "$id", "title", "description")) {
		t.Errorf("list items and add object differ:\nlist: %v\nadd:  %v", item, add)
	}
	required, _ := item["required"].([]any)
	var got []string
	for _, key := range required {
		got = append(got, key.(string))
	}
	slices.Sort(got)
	want := slices.Sorted(slices.Values(ylRtRequired))
	if !slices.Equal(got, want) {
		t.Errorf("required %v, want %v", got, want)
	}
}

func ylRtWithout(m map[string]any, keys ...string) map[string]any {
	out := maps.Clone(m)
	for _, key := range keys {
		delete(out, key)
	}
	return out
}
