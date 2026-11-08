// Package workflow reads task-local workflow policy.
package workflow

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/gridconsole/grid-plugins/internal/contract"
	"go.yaml.in/yaml/v4"
)

// GatePostRoles reads gate_post_roles from the task's workflow.md front matter.
// Omitted kinds retain their defaults; a supplied list replaces a default, and
// an empty list forbids that kind. Policy never expands the question schema's
// set of author roles. Missing workflow files preserve older tasks' defaults.
func GatePostRoles(taskDir string) (map[string][]string, error) {
	roles := map[string][]string{
		"person": {"cli", "owner", "member", "lead"},
		"timer":  {"cli", "owner", "member", "lead"},
		"ci":     {"cli", "owner", "member", "lead", "agent", "worker"},
		"job":    {"cli", "owner", "member", "lead", "agent", "worker"},
	}
	raw, err := os.ReadFile(filepath.Join(taskDir, "workflow.md"))
	if errors.Is(err, os.ErrNotExist) {
		return roles, nil
	}
	if err != nil {
		return nil, err
	}
	lines := strings.Split(strings.ReplaceAll(string(raw), "\r\n", "\n"), "\n")
	if len(lines) < 3 || strings.TrimSpace(lines[0]) != "---" {
		return nil, fmt.Errorf("workflow.md needs YAML front matter")
	}
	end := 1
	for end < len(lines) && strings.TrimSpace(lines[end]) != "---" {
		end++
	}
	if end == len(lines) {
		return nil, fmt.Errorf("workflow.md front matter is not closed")
	}
	var front map[string]any
	if err := yaml.Load([]byte(strings.Join(lines[1:end], "\n")), &front); err != nil {
		return nil, fmt.Errorf("workflow.md front matter: %w", err)
	}
	value, exists := front["gate_post_roles"]
	if !exists {
		return roles, nil
	}
	overrides, ok := value.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("workflow.md gate_post_roles must be a map of gate kinds to role lists")
	}
	question, _ := contract.LookupType("question")
	for kind, value := range overrides {
		if _, ok := roles[kind]; !ok {
			return nil, fmt.Errorf("workflow.md gate_post_roles: unknown gate kind %q", kind)
		}
		list, ok := value.([]any)
		if !ok {
			return nil, fmt.Errorf("workflow.md gate_post_roles.%s must be a role list", kind)
		}
		allowed := []string{}
		for _, value := range list {
			role, ok := value.(string)
			if !ok || !slices.Contains(question.Roles, role) {
				return nil, fmt.Errorf("workflow.md gate_post_roles.%s: invalid question role %v", kind, value)
			}
			allowed = append(allowed, role)
		}
		roles[kind] = allowed
	}
	return roles, nil
}
