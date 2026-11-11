package jobcmd

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/job/jobtest"
	"github.com/gridconsole/grid-plugins/internal/where"
)

// hnCodeRun runs one call through Run and returns its exit, its stdout as the one
// JSON object it must be, and the raw stdout.
func hnCodeRun(t *testing.T, e Env, args ...string) (int, map[string]json.RawMessage, string) {
	t.Helper()
	out, errs := &bytes.Buffer{}, &bytes.Buffer{}
	e.Stdout, e.Stderr = out, errs
	exit := Run(args, e)
	dec := json.NewDecoder(bytes.NewReader(out.Bytes()))
	var obj map[string]json.RawMessage
	if err := dec.Decode(&obj); err != nil {
		t.Fatalf("%v: exit %d, stdout %q is not a JSON object: %v", args, exit, out, err)
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		t.Fatalf("%v: stdout holds more than one JSON value: %q", args, out)
	}
	return exit, obj, out.String()
}

// hnCodeString reads one string field of the object, "" when it is absent.
func hnCodeString(t *testing.T, obj map[string]json.RawMessage, key string) string {
	t.Helper()
	var s string
	if raw, ok := obj[key]; ok {
		if err := json.Unmarshal(raw, &s); err != nil {
			t.Fatalf("%s is not a string: %s", key, raw)
		}
	}
	return s
}

// hnCodeWant runs the call and asserts its exit and its top-level code; an empty
// code means the key must be absent.
func hnCodeWant(t *testing.T, e Env, exit int, code string, args ...string) map[string]json.RawMessage {
	t.Helper()
	got, obj, out := hnCodeRun(t, e, args...)
	if got != exit {
		t.Fatalf("%v: exit %d, want %d; stdout %s", args, got, exit, out)
	}
	_, has := obj["code"]
	switch {
	case code == "" && has:
		t.Fatalf("%v: an unclassified refusal has a code: %s", args, out)
	case code != "" && hnCodeString(t, obj, "code") != code:
		t.Fatalf("%v: code %s, want %q; stdout %s", args, obj["code"], code, out)
	}
	return obj
}

// hnCodeExample is the job fixture plus a registered project example with no tasks;
// the call runs from the example folder, so T-99 is looked up in example.
func hnCodeExample(t *testing.T) (Env, string) {
	t.Helper()
	e, _, _ := fixture(t)
	folder := t.TempDir()
	for _, dir := range []string{folder, filepath.Join(e.GridHome, "projects", "example", "tasks")} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	record := "---\nfolders:\n  - {machine: " + jobtest.Machine + ", path: " + folder + "}\n---\n"
	if err := os.WriteFile(filepath.Join(e.GridHome, "projects", "example", "project.md"), []byte(record), 0o644); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(folder, "brief.md")
	if err := os.WriteFile(file, []byte("parity\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	e.Cwd = folder
	e.Resolver.Machine = jobtest.Machine
	return e, file
}

func TestHnCodeUsage(t *testing.T) {
	e, _, _ := fixture(t)
	obj := hnCodeWant(t, e, 2, "invalid_argument", "render", "T-01", "--json")
	if got := hnCodeString(t, obj, "error"); got != "render requires <task> <item>" {
		t.Fatalf("error %q, want the base text", got)
	}
	if string(obj["ok"]) != "false" || string(obj["findings"]) != "[]" {
		t.Fatalf("present fields changed: ok %s findings %s", obj["ok"], obj["findings"])
	}
	hnCodeWant(t, e, 2, "invalid_argument", "verify", "--json")
	obj = hnCodeWant(t, e, 2, "invalid_argument", "trace", "brain:project/a", "brain:project/b", "--json")
	if string(obj["rows"]) != "[]" || !strings.Contains(string(obj["problems"]), "trace requires exactly one pointer") {
		t.Fatalf("trace lost rows or problems: rows %s problems %s", obj["rows"], obj["problems"])
	}
}

func TestHnCodeLookup(t *testing.T) {
	e, file := hnCodeExample(t)
	const notFound = "task T-99 not found in project example"
	for _, args := range [][]string{{"show", "T-99", "J-1", "--json"}, {"lint", file, "--task", "T-99", "--json"}} {
		obj := hnCodeWant(t, e, 1, "no_such_task", args...)
		if got := hnCodeString(t, obj, "error"); got != notFound {
			t.Fatalf("%v: error %q, want %q", args, got, notFound)
		}
	}
	absent := []string{"show", "T-99", "J-1", "--project", "absent", "--json"}
	hnCodeWant(t, e, 1, "no_such_project", absent...)
	guest := e
	guest.Resolver.Caller = where.Caller{Role: "guest", Member: "gus"}
	hnCodeWant(t, guest, 1, "no_such_task", absent...)
	obj := hnCodeWant(t, e, traceExitNotFound, "no_such_project", "trace", "brain:project/a", "--project", "absent", "--json")
	if string(obj["rows"]) != "[]" || !strings.Contains(string(obj["problems"]), "project not found") {
		t.Fatalf("trace lost rows or problems: rows %s problems %s", obj["rows"], obj["problems"])
	}
}

func TestHnCodeUnclassified(t *testing.T) {
	e, _, _ := fixture(t)
	obj := hnCodeWant(t, e, 1, "", "verify", "J-9", "--task", "T-01", "--json")
	if hnCodeString(t, obj, "error") == "" {
		t.Fatalf("the verify refusal lost its error: %v", obj)
	}
	// No Grid home is a Trace() error that is not errTraceProjectNotFound.
	noHome := e
	noHome.GridHome = ""
	obj = hnCodeWant(t, noHome, traceExitFailed, "", "trace", "brain:project/a", "--json")
	if !strings.Contains(string(obj["problems"]), "cannot find the Grid home") {
		t.Fatalf("trace problems %s", obj["problems"])
	}
}

func TestHnCodeWalk(t *testing.T) {
	e, file := hnCodeExample(t)
	bad := map[string][]string{
		"lint":   {"lint", file, "--task", "T-99", "--json"},
		"render": {"render", "T-01", "--json"},
		"show":   {"show", "T-99", "J-1", "--json"},
		"verify": {"verify", "--json"},
		"trace":  {"trace", "brain:project/a", "--project", "absent", "--json"},
	}
	for _, verb := range []string{"lint", "render", "show", "verify", "trace"} {
		exit, obj, out := hnCodeRun(t, e, bad[verb]...)
		if exit == 0 || hnCodeString(t, obj, "code") == "" {
			t.Errorf("%s: exit %d, stdout lacks a top-level code: %s", verb, exit, out)
		}
	}
	// A successful call of each verb that has one on the fixture prints no code.
	// verify has none here: a passing verify runs the Job's command.
	ok, _, _ := fixture(t)
	exit, obj, out := hnCodeRun(t, ok, "render", "T-01", "1", "--json")
	if exit != 0 {
		t.Fatalf("render: exit %d: %s", exit, out)
	}
	path := hnCodeString(t, obj, "path")
	for _, args := range [][]string{
		{"render", "T-01", "1", "--json"},
		{"lint", path, "--json"},
		{"show", "T-01", "J-1", "--json"},
		{"trace", "brain:project/export-format", "--json"},
	} {
		exit, _, out := hnCodeRun(t, ok, args...)
		if exit != 0 || strings.Contains(out, `"code"`) {
			t.Errorf("%v: exit %d; a success must print no code: %s", args, exit, out)
		}
	}
}
