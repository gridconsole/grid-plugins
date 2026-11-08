package taskfile

import (
	"regexp"
	"strings"
)

var stepName = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)

type WorkflowStep struct {
	Step string `json:"step"`
	Kind string `json:"kind"`
}

func WorkflowSteps(text string) []WorkflowStep {
	fm := ParseFrontmatter(text)
	if fm == nil || fm.Odd {
		return nil
	}
	list, ok := fm.Data["workflow"].([]any)
	if !ok || len(list) == 0 {
		return nil
	}
	var steps []WorkflowStep
	seen := map[string]bool{}
	for _, v := range list {
		item, ok := v.(map[string]any)
		if !ok {
			return nil
		}
		step, sok := item["step"].(string)
		kind, kok := item["kind"].(string)
		if !sok || !kok || !stepName.MatchString(step) || seen[step] {
			return nil
		}
		seen[step] = true
		steps = append(steps, WorkflowStep{step, kind})
	}
	return steps
}

// PlanApprovalIndex returns the first plan approval step, or -1. Rust names
// plan gates; also treat an approve before any run/work as a plan gate, so a
// custom name cannot remove the initial approval. Later unnamed approvals ship.
func PlanApprovalIndex(steps []WorkflowStep) int {
	for i, step := range steps {
		if step.Kind == "approve" && (strings.Contains(step.Step, "plan") || beforeFirstRun(steps[:i])) {
			return i
		}
	}
	return -1
}

func beforeFirstRun(steps []WorkflowStep) bool {
	for _, step := range steps {
		if step.Kind == "run" || step.Kind == "work" {
			return false
		}
	}
	return true
}

// PlanApprovalRequired reads the inline workflow used by existing task guards.
// Missing, unresolved named or malformed workflows require plan approval.
func PlanApprovalRequired(text string) bool {
	steps := WorkflowSteps(text)
	if len(steps) == 0 || PlanApprovalIndex(steps) >= 0 {
		return true
	}
	for _, step := range steps {
		switch step.Kind {
		case "run", "work", "approve":
		default:
			return true
		}
	}
	return false
}
