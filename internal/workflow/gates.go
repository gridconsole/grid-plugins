// Package workflow reads task-local workflow policy.
package workflow

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"

	"github.com/gridconsole/grid-plugins/internal/contract"
)

// GatePostRoles reads gate_post_roles from the task's workflow.md front matter.
// Omitted kinds retain their defaults; a supplied list replaces a default, and
// an empty list forbids that kind. Policy never expands the question schema's
// set of author roles. The withdraw action uses the control schema instead.
// Missing workflow files preserve older tasks' defaults.
func GatePostRoles(taskDir string) (map[string][]string, error) {
	roles := map[string][]string{
		"person":   {"cli", "owner", "member", "lead"},
		"timer":    {"cli", "owner", "member", "lead"},
		"ci":       {"cli", "owner", "member", "lead", "agent", "worker"},
		"job":      {"cli", "owner", "member", "lead", "agent", "worker"},
		"withdraw": DefaultQuestionWithdrawRoles(),
	}
	raw, err := os.ReadFile(filepath.Join(taskDir, "workflow.md"))
	if errors.Is(err, os.ErrNotExist) {
		return roles, nil
	}
	if err != nil {
		return nil, err
	}
	front, err := parseFrontmatter(raw)
	if err != nil {
		return nil, err
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
	control, _ := contract.LookupType("control")
	for kind, value := range overrides {
		if _, ok := roles[kind]; !ok {
			return nil, fmt.Errorf("workflow.md gate_post_roles: unknown gate kind %q", kind)
		}
		list, ok := value.([]any)
		if !ok {
			return nil, fmt.Errorf("workflow.md gate_post_roles.%s must be a role list", kind)
		}
		validRoles := question.Roles
		if kind == "withdraw" {
			validRoles = control.Roles
		}
		allowed := []string{}
		for _, value := range list {
			role, ok := value.(string)
			role = contract.NormalizeRole(role)
			if !ok || !slices.Contains(validRoles, role) {
				return nil, fmt.Errorf("workflow.md gate_post_roles.%s: invalid question role %v", kind, value)
			}
			allowed = append(allowed, role)
		}
		roles[kind] = allowed
	}
	return roles, nil
}

// DefaultQuestionWithdrawRoles is the fallback for leaseless person withdrawals.
// gate_post_roles.withdraw replaces it, including an empty list to forbid them.
func DefaultQuestionWithdrawRoles() []string { return []string{"owner", "member"} }
