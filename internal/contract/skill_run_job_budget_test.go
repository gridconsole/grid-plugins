package contract

import (
	"strings"
	"testing"
)

// The run-job skill must not turn the planner's turns estimate into a stop rule.
func TestSkillRunJobBudgetIsNotAStopRule(t *testing.T) {
	text := readPluginFile(t, "skills/run-job/SKILL.md")
	if strings.Contains(text, "stop at the point it stops fitting") {
		t.Error("run-job SKILL.md still tells the agent to stop when the Job stops fitting its turns estimate")
	}
	if !strings.Contains(text, "not a limit") {
		t.Error("run-job SKILL.md must say budget.turns_estimate is not a limit")
	}
}
