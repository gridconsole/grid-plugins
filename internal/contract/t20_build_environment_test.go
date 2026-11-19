package contract

import (
	"strings"
	"testing"
)

func requireAll(t *testing.T, file string, want ...string) {
	t.Helper()
	text := readPluginFile(t, file)
	for _, w := range want {
		if !strings.Contains(text, w) {
			t.Errorf("%s lost the rule text %q", file, w)
		}
	}
}

// A1 and A2: the run-job skill makes the builder work around an environment problem before a row is NOT DONE.
func TestRunJobSkillWorksAroundEnvironment(t *testing.T) {
	requireAll(t, "skills/run-job/SKILL.md",
		"fails for an environment cause",
		"fix or work around that environment problem yourself and rerun, before any row is NOT DONE",
		"`NOT DONE (environment: <cause>)`",
		"the error lines and the workaround you tried",
		"in `proposed_jobs`; do not ask a person a question for it",
	)
}

// A3: the builder agent matches the skill and names the cause and workaround in the proof table.
func TestBuilderAgentWorksAroundEnvironment(t *testing.T) {
	requireAll(t, "agents/builder.md",
		"fails for an environment cause",
		"before any row is NOT DONE",
		"`NOT DONE (environment: <cause>)`",
		"`proposed_jobs`, not as a question to a person",
		"the proof table names the cause and the workaround",
	)
}

// A4: the runner works around an environment cause once and keeps its permission rule.
func TestRunnerAgentEnvironmentAndPermissionRules(t *testing.T) {
	requireAll(t, "agents/runner.md",
		"fails for an environment cause",
		"run the command once more",
		"name the environment cause in the result apart from a real failure",
		"When a command is refused by the permission check, do not retry it and do not try another way around it.",
		"Grid shows the refusal to the person and resumes the task with the decision.",
	)
}
