package workflow

import (
	"errors"
	"fmt"
	"slices"
	"strings"
)

// PolicyEntry reports effective application policy for one visible source file.
type PolicyEntry struct {
	Project *string                `json:"project"`
	Keys    map[string]PolicyValue `json:"keys"`
}

// PolicyValue distinguishes built-in defaults from inherited workflow values.
type PolicyValue struct {
	Value  any    `json:"value"`
	Source string `json:"source"`
	From   string `json:"from,omitempty"`
}

var appPolicyKeys = []string{"refusal_message", "inbox_act_roles", "project_document_write_roles", "task_undo_roles"}
var inboxActs = []string{"turn", "add", "note", "snooze", "dismiss", "wake", "reopen"}

func policyDefaults() map[string]PolicyValue {
	acts := map[string][]string{}
	for _, act := range inboxActs {
		acts[act] = []string{"owner", "member", "local", "agent"}
	}
	return map[string]PolicyValue{
		"refusal_message":              {Value: "plugin", Source: "default"},
		"inbox_act_roles":              {Value: acts, Source: "default"},
		"project_document_write_roles": {Value: []string{"owner", "member"}, Source: "default"},
		"task_undo_roles":              {Value: []string{"owner", "member"}, Source: "default"},
	}
}

// ErrPolicy identifies a sanitized application policy diagnostic.
var ErrPolicy = errors.New("invalid workflow policy")

type policyError struct{ key, level string }

func (e policyError) Error() string { return fmt.Sprintf("invalid %s in %s workflow", e.key, e.level) }
func (e policyError) Unwrap() error { return ErrPolicy }

func invalidPolicy(key string, file File) error {
	level := "workspace"
	if file.Project != nil {
		level = "project"
	}
	return policyError{key: key, level: level}
}

// effectivePolicy only consumes the already bounded and guest-filtered File.Text.
// Read failures cannot be interpreted as missing policy and permissive defaults.
func effectivePolicy(files []File, problems []fileProblem) ([]PolicyEntry, error) {
	if len(problems) > 0 {
		return nil, invalidPolicy("front matter", files[problems[0].index])
	}
	out := make([]PolicyEntry, 0, len(files))
	workspace := policyDefaults()
	for _, file := range files {
		keys := make(map[string]PolicyValue, len(workspace))
		for key, value := range workspace {
			keys[key] = value
		}
		// Use L122's front-matter splitter and YAML dialect, including CRLF.
		first, _, _ := strings.Cut(file.Text, "\n")
		if strings.TrimSpace(first) == "---" {
			front, err := parseFrontmatter([]byte(file.Text))
			if err != nil {
				return nil, invalidPolicy("front matter", file)
			}
			for _, key := range appPolicyKeys {
				raw, set := front[key]
				if !set {
					continue
				}
				value, valid := appPolicyValue(key, raw)
				if !valid {
					return nil, invalidPolicy(key, file)
				}
				from := "workspace"
				if file.Project != nil {
					from = "project"
				}
				keys[key] = PolicyValue{Value: value, Source: "workflow", From: from}
			}
		}
		if file.Project == nil {
			workspace = keys
		}
		out = append(out, PolicyEntry{Project: file.Project, Keys: keys})
	}
	return out, nil
}

func appPolicyValue(key string, raw any) (any, bool) {
	switch key {
	case "refusal_message":
		value, ok := raw.(string)
		return value, ok && (value == "plugin" || value == "none")
	case "inbox_act_roles":
		value, ok := raw.(map[string]any)
		if !ok {
			return nil, false
		}
		acts := make(map[string][]string, len(value))
		for act, rawRoles := range value {
			if !slices.Contains(inboxActs, act) {
				return nil, false
			}
			roles, ok := policyRoles(rawRoles, []string{"owner", "member", "local", "agent"})
			if !ok {
				return nil, false
			}
			acts[act] = roles
		}
		return acts, true
	case "project_document_write_roles", "task_undo_roles":
		return policyRoles(raw, []string{"owner", "member"})
	default:
		return nil, false
	}
}

func policyRoles(raw any, allowed []string) ([]string, bool) {
	list, ok := raw.([]any)
	if !ok {
		return nil, false
	}
	roles := make([]string, 0, len(list))
	for _, item := range list {
		role, ok := item.(string)
		if !ok || !slices.Contains(allowed, role) {
			return nil, false
		}
		roles = append(roles, role)
	}
	return roles, true
}
