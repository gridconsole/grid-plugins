package cli

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/gridhome"
	"github.com/gridconsole/grid-plugins/internal/testgit"
)

// The workflows.md the first `workflow write --rev none` stores: a default
// template with every step kind and the optional rows, and a second one.
const yiWfFirst = `# Workflows

## Social post
applies when: project example, or label social
approvers: sam, robin
repeat: Mondays at 09:00
review rounds: 2

| # | step | kind | who | on reject, back to |
|---|---|---|---|---|
| 1 | Write the posts together | make-together | you and grid-designer |  |
| 2 | Check the copy | check | grid-runner | Write the posts together |
| 3 | Robin approves the posts | approve | robin | Write the posts together |
| 4 | Post on schedule | schedule | the scheduler |  |

## Rate change
applies when: the title has rates
only for project: example
approvers: sam
lead time: a week ahead
batch: One task holds the whole week
review rounds: 3
research: first

| # | step | kind | who | runs | sizes | on reject, back to |
|---|---|---|---|---|---|---|
| 1 | Accept the plan | ask | grid-planner asks, you answer |  | N L |  |
| 2 | Do the work | run | grid-builder | /grid:worker | all |  |
| 3 | Approve the result | approve | sam |  | all | Do the work |
| 4 | Ship | schedule | the scheduler |  | all |  |
`

// The text of the second project write: one template fewer.
const yiWfSecond = `# Workflows

## Rate change
applies when: the title has rates
approvers: sam
review rounds: 2

| # | step | kind | who | on reject, back to |
|---|---|---|---|---|
| 1 | Do the work | run | grid-builder |  |
| 2 | Approve the result | approve | sam | Do the work |
`

// yiWfWorkspace is the workspace write: a workspace file may hold the default template.
const yiWfWorkspace = `# Workflows

## Accept, approve, ship
applies when: nothing else matches
approvers: sam, alex
review rounds: 2

| # | step | kind | who | on reject, back to |
|---|---|---|---|---|
| 1 | Do the work | run | grid-builder |  |
| 2 | Approve the result | approve | sam | Do the work |
`

// yiWfBroken is appended to the stored file by hand; write refuses it, show lists it.
const yiWfBroken = `
## Broken
applies when: label broken
approvers: sam
review rounds: 2

| # | step | kind | who | on reject, back to |
|---|---|---|---|---|
| 1 | Do it | dance | grid-builder |  |
`

// yiWfFrontMatter is a stored file with application policy front matter, written by hand.
const yiWfFrontMatter = "---\nrefusal_message: none\nproject_document_write_roles: [owner]\n---\n" + yiWfSecond

// yiWfAnswers are the stdout of each captured command, by fixture name.
type yiWfAnswers struct {
	show  map[string][]byte // cmd-workflow-show
	write map[string][]byte // cmd-workflow-write
}

// yiWfCollect runs the plugin's own writers in a registered project and returns the
// stdout of `workflow show` and `workflow write` under their fixture names.
func yiWfCollect(t *testing.T) yiWfAnswers {
	t.Helper()
	h := newHarness(t)
	for _, key := range strings.Fields("GRID_AGENT GRID_WORKER_ID GRID_WORKER GRID_LEASE GRID_LEASE_EPOCH GRID_WAKE GRID_FEED_TASK GRID_JOB GRID_ATTEMPT GRID_SESSION_ID CLAUDE_SESSION_ID GRID_OWNER") {
		t.Setenv(key, "")
	}
	t.Setenv("GRID_ROLE", "owner")
	got := yiWfAnswers{show: map[string][]byte{}, write: map[string][]byte{}}
	run := func(cwd string, args ...string) []byte {
		t.Helper()
		if code := h.run(cwd, args...); code != 0 {
			t.Fatalf("%s: exit %d: %s", strings.Join(args, " "), code, h.errb)
		}
		return append([]byte(nil), h.out.Bytes()...)
	}
	checkout := filepath.Join(h.root, "checkout")
	h.mkdir(checkout)
	testgit.Init(t, checkout, "https://example.com/org/example.git", true)
	run(checkout, "project", "add", "--name", "example")
	h.env.Machine = gridhome.Machine(h.gridHome)

	got.show["no-workflows"] = run(checkout, "workflow", "show", "--project", "example", "--json")

	input := filepath.Join(h.root, "workflows-input.md")
	h.write(input, yiWfFirst)
	first := run(checkout, "workflow", "write", "--project", "example", "--rev", "none", "--file", input, "--json")
	got.write["project-new"] = first
	got.show["project"] = run(checkout, "workflow", "show", "--project", "example", "--json")
	got.show["project-policy"] = run(checkout, "workflow", "show", "--project", "example", "--json", "--policy")

	var wrote struct{ File, Rev string }
	if err := json.Unmarshal(first, &wrote); err != nil {
		t.Fatal(err)
	}
	h.write(input, yiWfSecond)
	got.write["project-replace"] = run(checkout, "workflow", "write", "--project", "example", "--rev", wrote.Rev, "--file", input, "--json")

	// The stored file of the project: the one workflows.md under the Grid home.
	var stored []string
	err := filepath.WalkDir(h.gridHome, func(path string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() && d.Name() == "workflows.md" {
			stored = append(stored, path)
		}
		return err
	})
	if err != nil || len(stored) != 1 {
		t.Fatalf("stored workflows.md files %v, err %v", stored, err)
	}
	if err := os.WriteFile(stored[0], []byte(yiWfSecond+yiWfBroken), 0o644); err != nil {
		t.Fatal(err)
	}
	got.show["project-errors"] = run(checkout, "workflow", "show", "--project", "example", "--json")
	if err := os.WriteFile(stored[0], []byte(yiWfFrontMatter), 0o644); err != nil {
		t.Fatal(err)
	}
	got.show["project-policy-front-matter"] = run(checkout, "workflow", "show", "--project", "example", "--json", "--policy")

	h.write(input, yiWfWorkspace)
	got.write["workspace"] = run(checkout, "workflow", "write", "--rev", "none", "--file", input, "--json")
	return got
}

// yiWfDecode parses one captured stdout the way the app does: a single JSON value.
func yiWfDecode(t *testing.T, raw []byte) any {
	t.Helper()
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		t.Fatalf("stdout is not JSON: %v\n%s", err, raw)
	}
	return v
}

// yiWfRename returns a copy of doc with the key at path renamed (the last element of
// path is the key; earlier elements are object keys or array indexes taken as 0).
func yiWfRename(t *testing.T, doc any, path ...string) []byte {
	t.Helper()
	raw, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	var copyOf any
	if err := json.Unmarshal(raw, &copyOf); err != nil {
		t.Fatal(err)
	}
	// object is the first element when the node is a list.
	object := func(node any) map[string]any {
		if list, ok := node.([]any); ok {
			if len(list) == 0 {
				t.Fatalf("%v: a list on the path is empty", path)
			}
			node = list[0]
		}
		m, ok := node.(map[string]any)
		if !ok {
			t.Fatalf("%v: not an object on the path", path)
		}
		return m
	}
	cur := object(copyOf)
	for _, step := range path[:len(path)-1] {
		cur = object(cur[step])
	}
	key := path[len(path)-1]
	value, present := cur[key]
	if !present {
		t.Fatalf("%v: %q is not a key of the captured output", path, key)
	}
	cur[key+"_renamed"] = value
	delete(cur, key)
	out, err := json.Marshal(copyOf)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func yiWfFixturePath(schema, kind, name string) string {
	return filepath.Join("..", "..", "contract", "fixtures", schema, kind, name+".json")
}

func yiWfWriteFixture(t *testing.T, path string, v any) {
	t.Helper()
	contractCommandWrite(t, path, v)
}

// TestYiWorkflowShowSchema: show --json with no workflows.md, with one, with a broken
// section, and with --policy all pass cmd-workflow-show, and the fixtures are the
// captured answers. UPDATE_YI_WORKFLOW_FIXTURES=1 rewrites them.
func TestYiWorkflowShowSchema(t *testing.T) {
	got := yiWfCollect(t)
	update := os.Getenv("UPDATE_YI_WORKFLOW_FIXTURES") == "1"
	for name, raw := range got.show {
		t.Run(name, func(t *testing.T) {
			if err := contract.Validate("cmd-workflow-show", raw); err != nil {
				t.Fatalf("%v\n%s", err, raw)
			}
			doc := yiWfDecode(t, raw)
			_, hasPolicy := doc.(map[string]any)["policy"]
			if want := strings.Contains(name, "policy"); hasPolicy != want {
				t.Fatalf("policy present %v, want %v (only --policy answers carry it)", hasPolicy, want)
			}
			path := yiWfFixturePath("cmd-workflow-show", "valid", name)
			if update {
				yiWfWriteFixture(t, path, doc)
			}
			fixture, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(yiWfDecode(t, fixture), doc) {
				t.Fatalf("fixture %s is not the captured answer; run with UPDATE_YI_WORKFLOW_FIXTURES=1\n%s", path, raw)
			}
		})
	}
	// The populated fixtures must carry what the app reads, not only empty lists.
	populated := yiWfDecode(t, got.show["project"]).(map[string]any)
	if templates, _ := populated["templates"].([]any); len(templates) != 2 {
		t.Fatalf("project answer has %d templates, want 2", len(templates))
	}
	if errs, _ := yiWfDecode(t, got.show["project-errors"]).(map[string]any)["errors"].([]any); len(errs) == 0 {
		t.Fatal("the broken section is not listed in errors")
	}
	if update {
		doc := yiWfDecode(t, got.show["project"])
		yiWfWriteFixture(t, yiWfFixturePath("cmd-workflow-show", "invalid", "missing-steps"), yiWfDecode(t, yiWfRename(t, doc, "templates", "steps")))
		yiWfWriteFixture(t, yiWfFixturePath("cmd-workflow-show", "invalid", "missing-errors"), yiWfDecode(t, yiWfRename(t, doc, "errors")))
	}
}

// TestYiWorkflowWriteSchema: write --json with --rev none (project and workspace) and
// with a stored rev passes cmd-workflow-write.
func TestYiWorkflowWriteSchema(t *testing.T) {
	got := yiWfCollect(t)
	update := os.Getenv("UPDATE_YI_WORKFLOW_FIXTURES") == "1"
	for name, raw := range got.write {
		t.Run(name, func(t *testing.T) {
			if err := contract.Validate("cmd-workflow-write", raw); err != nil {
				t.Fatalf("%v\n%s", err, raw)
			}
			doc := yiWfDecode(t, raw)
			path := yiWfFixturePath("cmd-workflow-write", "valid", name)
			if update {
				yiWfWriteFixture(t, path, doc)
			}
			fixture, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(yiWfDecode(t, fixture), doc) {
				t.Fatalf("fixture %s is not the captured answer; run with UPDATE_YI_WORKFLOW_FIXTURES=1\n%s", path, raw)
			}
		})
	}
	if update {
		doc := yiWfDecode(t, got.write["project-new"])
		yiWfWriteFixture(t, yiWfFixturePath("cmd-workflow-write", "invalid", "missing-rev"), yiWfDecode(t, yiWfRename(t, doc, "rev")))
		doc.(map[string]any)["rev"] = "not-a-revision"
		yiWfWriteFixture(t, yiWfFixturePath("cmd-workflow-write", "invalid", "bad-rev"), doc)
	}
}

// TestYiWorkflowRequiredFields: a field the app decodes, renamed in a copy of the
// captured stdout, makes Validate fail. The unrenamed stdout passes first.
func TestYiWorkflowRequiredFields(t *testing.T) {
	got := yiWfCollect(t)
	show := yiWfDecode(t, got.show["project"])
	policy := yiWfDecode(t, got.show["project-policy"])
	failed := yiWfDecode(t, got.show["project-errors"])
	write := yiWfDecode(t, got.write["project-new"])
	cases := []struct {
		schema string
		doc    any
		path   []string
	}{
		// internal/kernel/worker/judge.go:430-447 and taskview/view_steps.go:51-70 (app 8f9b806d).
		{"cmd-workflow-show", show, []string{"templates"}},
		{"cmd-workflow-show", show, []string{"errors"}},
		{"cmd-workflow-show", show, []string{"templates", "steps"}},
		{"cmd-workflow-show", show, []string{"templates", "name"}},
		{"cmd-workflow-show", show, []string{"templates", "default"}},
		{"cmd-workflow-show", show, []string{"templates", "rounds"}},
		{"cmd-workflow-show", show, []string{"templates", "file"}},
		{"cmd-workflow-show", show, []string{"templates", "overrides"}},
		{"cmd-workflow-show", show, []string{"templates", "openTasks"}},
		{"cmd-workflow-show", show, []string{"templates", "applies"}},
		{"cmd-workflow-show", show, []string{"templates", "steps", "name"}},
		{"cmd-workflow-show", show, []string{"templates", "steps", "kind"}},
		{"cmd-workflow-show", show, []string{"templates", "steps", "agent"}},
		{"cmd-workflow-show", show, []string{"templates", "steps", "sizes"}},
		{"cmd-workflow-show", failed, []string{"errors", "section"}},
		{"cmd-workflow-show", failed, []string{"errors", "message"}},
		{"cmd-workflow-show", show, []string{"files"}},
		// cmd/grid-server/write_policy.go:139-143 and :275 (the envelope has files, templates, errors, policy).
		{"cmd-workflow-show", policy, []string{"policy"}},
		{"cmd-workflow-show", policy, []string{"policy", "project"}},
		{"cmd-workflow-show", policy, []string{"policy", "keys"}},
		{"cmd-workflow-show", policy, []string{"policy", "keys", "refusal_message"}},
		{"cmd-workflow-show", policy, []string{"policy", "keys", "inbox_act_roles"}},
		{"cmd-workflow-show", policy, []string{"policy", "keys", "refusal_message", "value"}},
		{"cmd-workflow-show", policy, []string{"policy", "keys", "refusal_message", "source"}},
		// internal/slices/workflows/write.go:138-148.
		{"cmd-workflow-write", write, []string{"rev"}},
		{"cmd-workflow-write", write, []string{"file"}},
	}
	for _, c := range cases {
		t.Run(c.schema+"/"+strings.Join(c.path, "."), func(t *testing.T) {
			if raw, err := json.Marshal(c.doc); err != nil {
				t.Fatal(err)
			} else if err := contract.Validate(c.schema, raw); err != nil {
				t.Fatalf("the unrenamed stdout must pass: %v", err)
			}
			if err := contract.Validate(c.schema, yiWfRename(t, c.doc, c.path...)); err == nil {
				t.Fatalf("%s accepts stdout with %v renamed", c.schema, c.path)
			}
		})
	}
}
