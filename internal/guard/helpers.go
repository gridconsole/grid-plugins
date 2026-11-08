package guard

import "github.com/gridconsole/grid-plugins/internal/taskfile"

// AgentName, GateLike and PlanStage share the guard's migration semantics with
// the remaining session hooks and the recorded helper corpus.
func AgentName(s string) string             { return agentName(s) }
func GateLike(s string) bool                { return gateLike(s) }
func PlanStage(text, history string) string { return planStage(text, history) }

type WorkflowStep struct {
	Step string `json:"step"`
	Kind string `json:"kind"`
}

func WorkflowSteps(text string) []WorkflowStep {
	fm := taskfile.ParseFrontmatter(text)
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
