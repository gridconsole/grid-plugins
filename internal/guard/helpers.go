package guard

import "github.com/gridconsole/grid-plugins/internal/taskfile"

// AgentName, GateLike and PlanStage share the guard's migration semantics with
// the remaining session hooks and the recorded helper corpus.
func AgentName(s string) string             { return agentName(s) }
func GateLike(s string) bool                { return gateLike(s) }
func PlanStage(text, history string) string { return planStage(text, history) }

type WorkflowStep = taskfile.WorkflowStep

func WorkflowSteps(text string) []WorkflowStep { return taskfile.WorkflowSteps(text) }
