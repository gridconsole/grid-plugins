package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/workflow"
)

type shownPolicyValue struct {
	Value  any    `json:"value"`
	Source string `json:"source"`
	From   string `json:"from,omitempty"`
}
type shownPolicy struct {
	Project *string                     `json:"project"`
	Keys    map[string]shownPolicyValue `json:"keys"`
}

func showPolicy(t *testing.T, h *harness, selector string) []shownPolicy {
	t.Helper()
	args := []string{"workflow", "show", "--json", "--policy"}
	if selector != "" {
		args = append(args, "--project", selector)
	}
	if code := h.run(h.root, args...); code != 0 || h.errb.Len() != 0 {
		t.Fatalf("policy exit=%d stderr=%q", code, h.errb.String())
	}
	var envelope struct {
		Files  []workflow.File `json:"files"`
		Policy []shownPolicy   `json:"policy"`
	}
	if err := json.Unmarshal(h.out.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	if len(envelope.Policy) != len(envelope.Files) {
		t.Fatalf("policy/file count mismatch: %s", h.out)
	}
	for i, entry := range envelope.Policy {
		if !reflect.DeepEqual(entry.Project, envelope.Files[i].Project) || len(entry.Keys) != 4 {
			t.Fatalf("policy shape/order: %+v", entry)
		}
	}
	return envelope.Policy
}

func TestWorkflowPolicyDefaults(t *testing.T) {
	h := newHarness(t)
	h.write(filepath.Join(h.gridHome, "projects", "alpha", "project.md"), "---\nid: id-alpha\n---\n")
	acts := map[string]any{}
	for _, act := range []string{"turn", "add", "note", "snooze", "dismiss", "wake", "reopen"} {
		acts[act] = []any{"owner", "member", "local", "agent"}
	}
	want := map[string]shownPolicyValue{
		"refusal_message":              {Value: "plugin", Source: "default"},
		"inbox_act_roles":              {Value: acts, Source: "default"},
		"project_document_write_roles": {Value: []any{"owner", "member"}, Source: "default"},
		"task_undo_roles":              {Value: []any{"owner", "member"}, Source: "default"},
	}
	for _, text := range []string{"", cliWorkflow} {
		if text != "" {
			h.write(filepath.Join(h.gridHome, "workflows.md"), text)
		}
		entries := showPolicy(t, h, "")
		if len(entries) != 2 {
			t.Fatalf("entries=%d", len(entries))
		}
		for _, entry := range entries {
			if !reflect.DeepEqual(entry.Keys, want) {
				t.Fatalf("defaults: %+v", entry.Keys)
			}
		}
	}
}

func TestWorkflowPolicyInheritance(t *testing.T) {
	h := newHarness(t)
	h.write(filepath.Join(h.gridHome, "projects", "alpha", "project.md"), "---\nid: id-alpha\n---\n")
	h.write(filepath.Join(h.gridHome, "workflows.md"), "---\nrefusal_message: none\ninbox_act_roles: {note: [member]}\nproject_document_write_roles: [owner]\ntask_undo_roles: [member]\nignored_key: secret\nperson_verbs: invalid-but-outside-allowlist\n---\n"+cliWorkflow)
	for _, selector := range []string{"alpha", "id-alpha"} {
		entries := showPolicy(t, h, selector)
		for _, entry := range entries {
			if entry.Keys["refusal_message"] != (shownPolicyValue{Value: "none", Source: "workflow", From: "workspace"}) {
				t.Fatalf("workspace inheritance: %+v", entry.Keys)
			}
			for _, key := range []string{"project_document_write_roles", "task_undo_roles", "inbox_act_roles"} {
				if entry.Keys[key].Source != "workflow" || entry.Keys[key].From != "workspace" {
					t.Fatalf("inherit %s: %+v", key, entry.Keys[key])
				}
			}
		}
	}
	h.write(filepath.Join(h.gridHome, "projects", "alpha", "workflows.md"), "---\nrefusal_message: plugin\ninbox_act_roles: {turn: [owner]}\nproject_document_write_roles: []\ntask_undo_roles: []\n---\n"+cliWorkflow)
	entries := showPolicy(t, h, "alpha")
	want := map[string]shownPolicyValue{
		"refusal_message":              {Value: "plugin", Source: "workflow", From: "project"},
		"inbox_act_roles":              {Value: map[string]any{"turn": []any{"owner"}}, Source: "workflow", From: "project"},
		"project_document_write_roles": {Value: []any{}, Source: "workflow", From: "project"},
		"task_undo_roles":              {Value: []any{}, Source: "workflow", From: "project"},
	}
	if !reflect.DeepEqual(entries[1].Keys, want) {
		t.Fatalf("project replacement: %+v", entries[1].Keys)
	}
	if entries[0].Keys["refusal_message"].Value != "none" {
		t.Fatal("project changed workspace")
	}
	h.write(filepath.Join(h.gridHome, "projects", "alpha", "workflows.md"), "---\ninbox_act_roles: {}\n---\n"+cliWorkflow)
	entries = showPolicy(t, h, "alpha")
	if !reflect.DeepEqual(entries[1].Keys["inbox_act_roles"].Value, map[string]any{}) {
		t.Fatal("empty map did not replace workspace")
	}
}

func TestWorkflowPolicyInvalid(t *testing.T) {
	cases := []struct{ name, key, front string }{
		{"refusal-value", "refusal_message", "refusal_message: maybe"},
		{"refusal-type", "refusal_message", "refusal_message: [none]"},
		{"refusal-null", "refusal_message", "refusal_message: null"},
		{"unknown-act", "inbox_act_roles", "inbox_act_roles: {fly: [owner]}"},
		{"guest", "inbox_act_roles", "inbox_act_roles: {turn: [guest]}"},
		{"role-scalar", "inbox_act_roles", "inbox_act_roles: {turn: owner}"},
		{"role-number", "inbox_act_roles", "inbox_act_roles: {turn: [123]}"},
		{"act-number", "inbox_act_roles", "inbox_act_roles: {123: [owner]}"},
		{"act-list", "inbox_act_roles", "inbox_act_roles: []"},
		{"act-null", "inbox_act_roles", "inbox_act_roles: null"},
		{"document-agent", "project_document_write_roles", "project_document_write_roles: [agent]"},
		{"document-scalar", "project_document_write_roles", "project_document_write_roles: owner"},
		{"undo-guest", "task_undo_roles", "task_undo_roles: [guest]"},
		{"undo-null", "task_undo_roles", "task_undo_roles: null"},
		{"unclosed", "front matter", "UNCLOSED"},
		{"yaml", "front matter", "refusal_message: ["},
		{"root-list", "front matter", "- secret"},
		{"duplicate", "front matter", "refusal_message: none\nrefusal_message: plugin"},
	}
	for _, level := range []string{"workspace", "project"} {
		for _, tc := range cases {
			t.Run(level+"/"+tc.name, func(t *testing.T) {
				h := newHarness(t)
				h.write(filepath.Join(h.gridHome, "projects", "alpha", "project.md"), "---\nid: id-alpha\n---\n")
				path := filepath.Join(h.gridHome, "workflows.md")
				if level == "project" {
					path = filepath.Join(h.gridHome, "projects", "alpha", "workflows.md")
				}
				text := "---\n" + tc.front + "\n---\n" + cliWorkflow
				if tc.front == "UNCLOSED" {
					text = "---\nrefusal_message: none\n"
				}
				h.write(path, text)
				code := h.run(h.root, "workflow", "show", "--json", "--policy", "--project", "alpha")
				want := "grid workflow show: invalid " + tc.key + " in " + level + " workflow\n"
				if code != 1 || h.out.Len() != 0 || h.errb.String() != want {
					t.Fatalf("exit=%d stdout-empty=%t stderr=%q want=%q", code, h.out.Len() == 0, h.errb.String(), want)
				}
			})
		}
	}
}

func TestWorkflowPolicyLegacyBytes(t *testing.T) {
	h := newHarness(t)
	h.write(filepath.Join(h.gridHome, "workflows.md"), "---\nrefusal_message: none\n---\n"+cliWorkflow)
	for _, mode := range []string{"plain", "json"} {
		args := []string{"workflow", "show"}
		if mode == "json" {
			args = append(args, "--json")
		}
		if code := h.run(h.root, args...); code != 0 {
			t.Fatal(code, h.errb)
		}
		before := h.out.String()
		if mode == "json" {
			var shape map[string]json.RawMessage
			if err := json.Unmarshal([]byte(before), &shape); err != nil {
				t.Fatal(err)
			}
			if len(shape) != 3 {
				t.Fatal("legacy envelope changed")
			}
			showPolicy(t, h, "")
			var legacy struct {
				Files     []workflow.File     `json:"files"`
				Templates []workflow.Template `json:"templates"`
				Errors    []workflow.Problem  `json:"errors"`
			}
			if err := json.Unmarshal(h.out.Bytes(), &legacy); err != nil {
				t.Fatal(err)
			}
			var buf bytes.Buffer
			if code := writeJSON(&buf, legacy); code != 0 || buf.String() != before {
				t.Fatal("policy changed legacy fields or bytes")
			}
		} else {
			if before != "file: workflows.md\ntemplates: Default\nerrors: 0\n" {
				t.Fatalf("plain changed: %q", before)
			}
		}
	}
	if code := h.run(h.root, "workflow", "show", "--policy"); code != 2 || h.out.Len() != 0 || h.errb.String() != "usage: grid workflow show [--project <name-or-id>] [--json] [--policy]\n" {
		t.Fatalf("usage exit=%d stderr=%q", code, h.errb.String())
	}
}

func TestWorkflowPolicyUnreadable(t *testing.T) {
	for _, kind := range []string{"oversized", "invalid-utf8", "directory", "symlink"} {
		t.Run(kind, func(t *testing.T) {
			h := newHarness(t)
			path := filepath.Join(h.gridHome, "workflows.md")
			switch kind {
			case "oversized":
				h.write(path, strings.Repeat("x", 32769))
			case "invalid-utf8":
				h.write(path, "\xff")
			case "directory":
				h.mkdir(path)
			case "symlink":
				target := filepath.Join(h.root, "policy-target")
				h.write(target, "---\nrefusal_message: none\n---\n"+cliWorkflow)
				h.mkdir(h.gridHome)
				if err := os.Symlink(target, path); err != nil {
					t.Skip("symlinks unavailable")
				}
			}
			if code := h.run(h.root, "workflow", "show", "--json", "--policy"); code != 1 || h.out.Len() != 0 || h.errb.String() != "grid workflow show: invalid front matter in workspace workflow\n" {
				t.Fatalf("unreadable policy exit=%d stdout-empty=%t stderr=%q", code, h.out.Len() == 0, h.errb.String())
			}
		})
	}
}
