// Package workflow reads task-local workflow policy.
package workflow

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"

	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/gridhome"
)

// GatePostRoles reads gate_post_roles in this order: the task's workflow.md
// front matter, the project's brain workflow note, the Grid home's brain
// workflow note, then the shipped default. The first that exists answers.
// Omitted kinds retain their defaults; a supplied list replaces a default, and
// an empty list forbids that kind. Policy never expands the question schema's
// set of author roles. The withdraw action uses the control schema instead.
//
// A workflow.md that is present but broken returns its error. A brain note that
// is broken, unreadable or doubled is skipped as ReadFor skips it, so the next
// level or the default answers with no error. The project and the home come
// from the task folder's place under the home (projects/<p>/tasks/<id> or
// personal/tasks/<id>); any other folder has no brain level and gets the default.
func GatePostRoles(taskDir string) (map[string][]string, error) {
	raw, err := readTaskPolicy(taskDir)
	if errors.Is(err, os.ErrNotExist) {
		return brainGatePostRoles(taskDir), nil
	}
	if err != nil {
		return nil, err
	}
	return gatePostRoles(raw)
}

// taskNamePattern is the task folder name where.taskParts accepts.
var taskNamePattern = regexp.MustCompile(`^T-\d+(-.*)?$`)

// brainGatePostRoles answers for a task without a workflow.md: the project's
// single workflow note, else the home's, else the default. It reuses fromBrain,
// so a note that fails there falls through and its problems are dropped.
func brainGatePostRoles(taskDir string) map[string][]string {
	home, project, ok := taskHome(taskDir)
	if !ok {
		return defaultGatePostRoles()
	}
	home = gridhome.Real(home)
	var out Answer
	if project != "" && out.fromBrain(home, "project", project, SourceProjectBrain, project) {
		return out.GatePostRoles
	}
	if out.fromBrain(home, "home", "", SourceHomeBrain, project) {
		return out.GatePostRoles
	}
	return defaultGatePostRoles()
}

// taskHome splits <home>/projects/<p>/tasks/<id> into home and p, and
// <home>/personal/tasks/<id> into home and no project.
func taskHome(taskDir string) (home, project string, ok bool) {
	dir := filepath.Clean(taskDir)
	if !taskNamePattern.MatchString(filepath.Base(dir)) {
		return "", "", false
	}
	tasks := filepath.Dir(dir)
	if filepath.Base(tasks) != "tasks" {
		return "", "", false
	}
	parent := filepath.Dir(tasks)
	if filepath.Base(parent) == "personal" {
		return filepath.Dir(parent), "", true
	}
	projects := filepath.Dir(parent)
	if filepath.Base(projects) == "projects" {
		return filepath.Dir(projects), filepath.Base(parent), true
	}
	return "", "", false
}

// defaultGatePostRoles is the shipped default when no workflow sets a kind.
func defaultGatePostRoles() map[string][]string {
	return map[string][]string{
		"person":   {"cli", "owner", "member", "lead"},
		"timer":    {"cli", "owner", "member", "lead"},
		"ci":       {"cli", "owner", "member", "lead", "agent", "worker"},
		"job":      {"cli", "owner", "member", "lead", "agent", "worker"},
		"withdraw": DefaultQuestionWithdrawRoles(),
	}
}

// gatePostRoles is the one front matter reader for gate_post_roles. A task's
// workflow.md and a brain workflow note both go through it.
func gatePostRoles(raw []byte) (map[string][]string, error) {
	roles := defaultGatePostRoles()
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
