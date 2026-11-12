package cli

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/contract"
)

// ykTwEnv sets the environment TestContractCommandsApp uses: no worker marker,
// a person actor and the owner role, which task delete and restore require.
func ykTwEnv(t *testing.T, h *harness) {
	t.Helper()
	for _, key := range strings.Fields("GRID_AGENT GRID_WORKER_ID GRID_WORKER GRID_LEASE GRID_LEASE_EPOCH GRID_WAKE GRID_FEED_TASK GRID_JOB GRID_ATTEMPT GRID_SESSION_ID CLAUDE_SESSION_ID GRID_OWNER GRID_APP_VERSION GRID_RELEASE_MANIFEST_URL") {
		t.Setenv(key, "")
	}
	for _, key := range []string{"GRID_MEMBER", "GRID_ACTOR", "GRID_WHO"} {
		t.Setenv(key, "member")
	}
	t.Setenv("GRID_ROLE", "owner")
	h.env.Machine = feedMachine
}

// ykTwRun runs one command that must exit 0 and returns a copy of its stdout.
func ykTwRun(t *testing.T, h *harness, args ...string) []byte {
	t.Helper()
	if code := h.run(h.home, args...); code != 0 {
		t.Fatalf("%s: exit %d: %s", strings.Join(args, " "), code, h.errb)
	}
	return append([]byte(nil), h.out.Bytes()...)
}

// ykTwObject decodes one stdout line into its fields.
func ykTwObject(t *testing.T, out []byte) map[string]any {
	t.Helper()
	var fields map[string]any
	if err := json.Unmarshal(out, &fields); err != nil {
		t.Fatalf("stdout is not one JSON object: %v\n%s", err, out)
	}
	return fields
}

// ykTwRenamed returns a copy of the stdout with one field under another name.
func ykTwRenamed(t *testing.T, out []byte, field string) []byte {
	t.Helper()
	fields := ykTwObject(t, out)
	value, ok := fields[field]
	if !ok {
		t.Fatalf("stdout has no %q field: %s", field, out)
	}
	delete(fields, field)
	fields[field+"_renamed"] = value
	b, err := json.Marshal(fields)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// ykTwString reads one string field the app reads from the stdout.
func ykTwString(t *testing.T, out []byte, field string) string {
	t.Helper()
	s, ok := ykTwObject(t, out)[field].(string)
	if !ok || s == "" {
		t.Fatalf("%q is not a non-empty string: %s", field, out)
	}
	return s
}

// ykTwSchemaFields lists the fields a schema declares, so a field the plugin
// writes and the schema lacks cannot pass by luck of additionalProperties.
func ykTwSchemaFields(t *testing.T, name string) (props, required map[string]bool) {
	t.Helper()
	data, err := contract.Schema(name)
	if err != nil {
		t.Fatal(err)
	}
	var schema struct {
		Properties map[string]json.RawMessage `json:"properties"`
		Required   []string                   `json:"required"`
	}
	if err := json.Unmarshal(data, &schema); err != nil {
		t.Fatal(err)
	}
	props, required = map[string]bool{}, map[string]bool{}
	for k := range schema.Properties {
		props[k] = true
	}
	for _, k := range schema.Required {
		required[k] = true
	}
	return props, required
}

// The app reads task new's id (newtask.go) and delete's project, id and folder
// (trash.go) from stdout; each real stdout must pass its schema, and renaming a
// field the app reads must fail it.
func TestYkTaskWriteSchemas(t *testing.T) {
	h := newHarness(t)
	ykTwEnv(t, h)

	created := ykTwRun(t, h, "task", "new", "Example task", "--json")
	var deleted, restored []byte
	id := ""

	t.Run("task new", func(t *testing.T) {
		id = ykTwString(t, created, "id")
		ykTwString(t, created, "path")
		if err := contract.Validate("cmd-task-new", created); err != nil {
			t.Fatalf("%v\n%s", err, created)
		}
		props, required := ykTwSchemaFields(t, "cmd-task-new")
		for field := range ykTwObject(t, created) {
			if !props[field] || !required[field] {
				t.Errorf("task new writes %q and cmd-task-new does not list it as required", field)
			}
		}
		for field := range required {
			if _, ok := ykTwObject(t, created)[field]; !ok {
				t.Errorf("cmd-task-new requires %q and task new does not write it", field)
			}
		}
	})

	t.Run("task delete", func(t *testing.T) {
		deleted = ykTwRun(t, h, "task", "delete", id, "--json")
		if err := contract.Validate("cmd-task-trash", deleted); err != nil {
			t.Fatalf("%v\n%s", err, deleted)
		}
		if got := ykTwString(t, deleted, "id"); got != id {
			t.Errorf("delete id = %q, want %q", got, id)
		}
		ykTwString(t, deleted, "project")
		ykTwString(t, deleted, "folder")
	})

	t.Run("task restore", func(t *testing.T) {
		project := ykTwString(t, deleted, "project")
		restored = ykTwRun(t, h, "task", "restore", id, "--project", project, "--json")
		if err := contract.Validate("cmd-task-trash", restored); err != nil {
			t.Fatalf("%v\n%s", err, restored)
		}
		if got := ykTwString(t, restored, "id"); got != id {
			t.Errorf("restore id = %q, want %q", got, id)
		}
		props, required := ykTwSchemaFields(t, "cmd-task-trash")
		for _, out := range [][]byte{deleted, restored} {
			for field := range ykTwObject(t, out) {
				if !props[field] || !required[field] {
					t.Errorf("task trash writes %q and cmd-task-trash does not list it as required", field)
				}
			}
			for field := range required {
				if _, ok := ykTwObject(t, out)[field]; !ok {
					t.Errorf("cmd-task-trash requires %q and the task move does not write it", field)
				}
			}
		}
	})

	t.Run("required holds the fields the app reads", func(t *testing.T) {
		cases := []struct {
			schema string
			out    []byte
			fields []string
		}{
			{"cmd-task-new", created, []string{"id", "path"}},
			{"cmd-task-trash", deleted, []string{"project", "id", "folder"}},
			{"cmd-task-trash", restored, []string{"project", "id", "folder"}},
		}
		for _, c := range cases {
			for _, field := range c.fields {
				if c.out == nil {
					t.Fatalf("no stdout to rename %q in; an earlier step failed", field)
				}
				// The unrenamed stdout must pass first, or a failing rename proves nothing.
				if err := contract.Validate(c.schema, c.out); err != nil {
					t.Fatalf("%s rejects the real stdout: %v\n%s", c.schema, err, c.out)
				}
				if err := contract.Validate(c.schema, ykTwRenamed(t, c.out, field)); err == nil {
					t.Errorf("%s accepts stdout without %q: %s", c.schema, field, c.out)
				}
			}
		}
	})
}
