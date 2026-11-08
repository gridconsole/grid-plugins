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

// PlanApprovalRequired reads the inline workflow used by existing task guards.
// Missing, unresolved named or malformed workflows require plan approval.
func PlanApprovalRequired(text string) bool {
	steps := WorkflowSteps(text)
	if len(steps) == 0 {
		return true
	}
	for _, step := range steps {
		switch step.Kind {
		case "run", "work":
		case "approve":
			if strings.Contains(step.Step, "plan") {
				return true
			}
		default:
			return true
		}
	}
	return false
}
