package workflow

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/where"
)

var allPolicyRoles = []string{"owner", "member", "local", "agent"}

func policyHome(t *testing.T, workspace string, projects map[string]string) string {
	t.Helper()
	home := t.TempDir()
	putWorkflowFixture(t, filepath.Join(home, "workflows.md"), workspace)
	for name, text := range projects {
		putWorkflowFixture(t, filepath.Join(home, "projects", name, "project.md"), "---\nid: id-"+name+"\n---\n")
		putWorkflowFixture(t, filepath.Join(home, "projects", name, "workflows.md"), text)
	}
	return home
}

func readPolicy(t *testing.T, home string) []PolicyEntry {
	t.Helper()
	got, err := ReadPolicy(where.Resolver{GridHome: home}, "")
	if err != nil {
		t.Fatalf("ReadPolicy: %v", err)
	}
	return got.Policy
}

func wantDefaultKeys(t *testing.T, keys map[string]PolicyValue) {
	t.Helper()
	acts := map[string][]string{}
	for _, act := range inboxActs {
		acts[act] = allPolicyRoles
	}
	if len(inboxActs) != 7 {
		t.Fatalf("inboxActs = %v", inboxActs)
	}
	want := map[string]PolicyValue{
		"refusal_message":              {Value: "plugin", Source: "default"},
		"inbox_act_roles":              {Value: acts, Source: "default"},
		"project_document_write_roles": {Value: []string{"owner", "member"}, Source: "default"},
		"task_undo_roles":              {Value: []string{"owner", "member"}, Source: "default"},
	}
	if !reflect.DeepEqual(keys, want) {
		t.Fatalf("keys = %#v\nwant %#v", keys, want)
	}
}

// Row 1.
func TestReadPolicyDefaults(t *testing.T) {
	home := policyHome(t, "# Workflows\n", nil)
	got, err := ReadPolicy(where.Resolver{GridHome: home}, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Policy) != 1 || got.Policy[0].Project != nil {
		t.Fatalf("policy = %+v", got.Policy)
	}
	wantDefaultKeys(t, got.Policy[0].Keys)
	legacy, err := Read(where.Resolver{GridHome: home}, "")
	if err != nil || len(legacy.Policy) != 0 || len(legacy.Files) != 1 {
		t.Fatalf("Read = %+v %v", legacy, err)
	}
}

const workspaceFront = "---\ntask_undo_roles: [owner]\ninbox_act_roles:\n  note: [owner]\n---\n# Workflows\n"

func checkWorkspaceOverride(t *testing.T, keys map[string]PolicyValue, from string) {
	t.Helper()
	wantUndo := PolicyValue{Value: []string{"owner"}, Source: "workflow", From: from}
	wantActs := PolicyValue{Value: map[string][]string{"note": {"owner"}}, Source: "workflow", From: from}
	if !reflect.DeepEqual(keys["task_undo_roles"], wantUndo) {
		t.Fatalf("task_undo_roles = %#v want %#v", keys["task_undo_roles"], wantUndo)
	}
	if !reflect.DeepEqual(keys["inbox_act_roles"], wantActs) {
		t.Fatalf("inbox_act_roles = %#v want %#v", keys["inbox_act_roles"], wantActs)
	}
}

// Row 2.
func TestReadPolicyWorkspaceBaseAndProjectOverride(t *testing.T) {
	home := policyHome(t, workspaceFront, map[string]string{
		"alpha": "# Alpha\n",
		"beta":  "---\nproject_document_write_roles: [owner]\n---\n# Beta\n",
		"gamma": "# Gamma\n",
	})
	policy := readPolicy(t, home)
	if len(policy) != 4 || policy[0].Project != nil {
		t.Fatalf("policy = %+v", policy)
	}
	checkWorkspaceOverride(t, policy[0].Keys, "workspace")
	for i, name := range []string{"alpha", "beta", "gamma"} {
		e := policy[i+1]
		if e.Project == nil || *e.Project != name {
			t.Fatalf("entry %d project = %v want %s", i+1, e.Project, name)
		}
		checkWorkspaceOverride(t, e.Keys, "workspace")
		if e.Keys["refusal_message"].Source != "default" {
			t.Fatalf("%s refusal_message = %+v", name, e.Keys["refusal_message"])
		}
	}
	docs := func(i int) PolicyValue { return policy[i].Keys["project_document_write_roles"] }
	if want := (PolicyValue{Value: []string{"owner"}, Source: "workflow", From: "project"}); !reflect.DeepEqual(docs(2), want) {
		t.Fatalf("beta docs = %#v want %#v", docs(2), want)
	}
	for _, i := range []int{0, 1, 3} {
		if want := (PolicyValue{Value: []string{"owner", "member"}, Source: "default"}); !reflect.DeepEqual(docs(i), want) {
			t.Fatalf("entry %d docs = %#v want %#v", i, docs(i), want)
		}
	}
}

// Row 3. Finding for the owner: overriding inbox_act_roles replaces the whole
// map, so the acts not named drop out instead of staying at their default.
func TestReadPolicyInboxActsReplacedAsAWhole(t *testing.T) {
	home := policyHome(t, "---\ninbox_act_roles:\n  snooze: [owner]\n  wake: [agent, local]\n---\n", nil)
	policy := readPolicy(t, home)
	want := PolicyValue{
		Value:  map[string][]string{"snooze": {"owner"}, "wake": {"agent", "local"}},
		Source: "workflow", From: "workspace",
	}
	if got := policy[0].Keys["inbox_act_roles"]; !reflect.DeepEqual(got, want) {
		t.Fatalf("inbox_act_roles = %#v want %#v", got, want)
	}
	acts := policy[0].Keys["inbox_act_roles"].Value.(map[string][]string)
	for _, act := range []string{"turn", "add", "note", "dismiss", "reopen"} {
		if _, ok := acts[act]; ok {
			t.Fatalf("act %s is present; overriding one act keeps the others at default", act)
		}
	}
}

// Row 4.
func TestReadPolicyInvalidFailsWholeAnswer(t *testing.T) {
	cases := []struct {
		name, front, key string
	}{
		{"inbox not a map", "inbox_act_roles: [owner]", "inbox_act_roles"},
		{"inbox unknown act", "inbox_act_roles:\n  shout: [owner]", "inbox_act_roles"},
		{"inbox role outside set", "inbox_act_roles:\n  note: [guest]", "inbox_act_roles"},
		{"inbox roles not a list", "inbox_act_roles:\n  note: owner", "inbox_act_roles"},
		{"inbox role not a string", "inbox_act_roles:\n  note: [1]", "inbox_act_roles"},
		{"document roles not a list", "project_document_write_roles: owner", "project_document_write_roles"},
		{"document role local", "project_document_write_roles: [local]", "project_document_write_roles"},
		{"document role agent", "project_document_write_roles: [owner, agent]", "project_document_write_roles"},
		{"undo roles not a list", "task_undo_roles: {a: b}", "task_undo_roles"},
		{"undo role guest", "task_undo_roles: [guest]", "task_undo_roles"},
		{"undo role local", "task_undo_roles: [local]", "task_undo_roles"},
		{"front matter not closed", "task_undo_roles: [owner]", "front matter"},
		{"front matter bad yaml", "task_undo_roles: [owner\nx: : :", "front matter"},
	}
	for _, level := range []string{"workspace", "project"} {
		for _, tc := range cases {
			t.Run(level+"/"+tc.name, func(t *testing.T) {
				text := "---\n" + tc.front + "\n---\n"
				if tc.key == "front matter" && strings.Contains(tc.name, "not closed") {
					text = "---\n" + tc.front + "\n"
				}
				home := policyHome(t, "# Workflows\n", map[string]string{"alpha": "# Alpha\n"})
				path := filepath.Join(home, "workflows.md")
				if level == "project" {
					path = filepath.Join(home, "projects", "alpha", "workflows.md")
				}
				putWorkflowFixture(t, path, text)
				got, err := ReadPolicy(where.Resolver{GridHome: home}, "")
				want := "invalid " + tc.key + " in " + level + " workflow"
				if !errors.Is(err, ErrPolicy) {
					t.Fatalf("err = %v, want ErrPolicy", err)
				}
				if err.Error() != want {
					t.Fatalf("err = %q want %q", err, want)
				}
				if len(got.Files) != 0 || len(got.Policy) != 0 || len(got.Templates) != 0 || len(got.Errors) != 0 {
					t.Fatalf("answer not zero: %+v", got)
				}
			})
		}
	}
}

// Row 4: a file that cannot be read is a front matter problem, not defaults.
func TestReadPolicyUnreadableFileFailsAsFrontMatter(t *testing.T) {
	home := policyHome(t, "# Workflows\n", map[string]string{"alpha": "# Alpha\n"})
	path := filepath.Join(home, "projects", "alpha", "workflows.md")
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(path, 0o755); err != nil {
		t.Fatal(err)
	}
	got, err := ReadPolicy(where.Resolver{GridHome: home}, "")
	if !errors.Is(err, ErrPolicy) || err.Error() != "invalid front matter in project workflow" {
		t.Fatalf("err = %v", err)
	}
	if len(got.Files) != 0 || len(got.Policy) != 0 {
		t.Fatalf("answer not zero: %+v", got)
	}
}

// Row 5.
func TestReadPolicyCRLFFrontMatter(t *testing.T) {
	crlf := strings.ReplaceAll(workspaceFront, "\n", "\r\n")
	lf := readPolicy(t, policyHome(t, workspaceFront, nil))
	got := readPolicy(t, policyHome(t, crlf, nil))
	checkWorkspaceOverride(t, got[0].Keys, "workspace")
	if !reflect.DeepEqual(lf, got) {
		t.Fatalf("CRLF policy %+v differs from LF %+v", got, lf)
	}
}

// Row 6.
func TestProblemErrorKeepsCause(t *testing.T) {
	section := "Web"
	p := Problem{File: "alpha/workflows.md", Section: &section, Message: "cannot read"}
	want := "the workflow cannot be resolved: " + p.String()

	plain := problemError(p)
	if plain.Error() != want || errors.Unwrap(plain) != nil {
		t.Fatalf("no cause: %q unwrap=%v", plain, errors.Unwrap(plain))
	}

	cause := errors.New("disk gone")
	p.cause = cause
	err := problemError(p)
	if err.Error() != want || !errors.Is(err, cause) || errors.Unwrap(err) != cause {
		t.Fatalf("with cause: %q is=%v", err, errors.Is(err, cause))
	}
}
