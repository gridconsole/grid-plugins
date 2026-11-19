package contract

import (
	"strings"
	"testing"
)

// T-20: review and checks judge whether the work is done, not the
// environment. These tests read the review skill and the two agents and pin
// the environment rule.

const (
	t20Skill   = "skills/review-job/SKILL.md"
	t20Review  = "agents/reviewer.md"
	t20Checker = "agents/checker.md"
)

func t20Need(t *testing.T, file string, phrases ...string) {
	t.Helper()
	text := readPluginFile(t, file)
	for _, p := range phrases {
		if !strings.Contains(text, p) {
			t.Errorf("%s: the environment rule lost %q", file, p)
		}
	}
}

func TestT20SkillWorksAroundEnvironment(t *testing.T) {
	t20Need(t, t20Skill,
		"environment cause",
		"Name the error lines as the proof",
		"fix it or work around it without touching the Job's files",
		"Rerun the check and judge the row on that output",
	)
}

func TestT20SkillNeverRejectsForEnvironment(t *testing.T) {
	t20Need(t, t20Skill,
		"judged from the code and the builder's evidence",
		"`proposed_jobs`",
		"never a must-fix, never a rework and never a question to a person",
		"NOT DONE (environment: <cause>)",
	)
	text := readPluginFile(t, t20Skill)
	if strings.Contains(text, "rejects the Job at once") {
		t.Errorf("%s: step 2 still rejects a NOT DONE row at once", t20Skill)
	}
}

func TestT20ReviewerMaySetUpEnvironment(t *testing.T) {
	t20Need(t, t20Review,
		"set up the environment for a check",
		"never edit the Job's files",
		"never set `dangerouslyDisableSandbox`",
		"never retry a refused permission",
		"NOT DONE (environment: <cause>)",
	)
	if strings.Contains(readPluginFile(t, t20Review), "is `REJECTED` at once") {
		t.Errorf("%s: still rejects a NOT DONE row at once", t20Review)
	}
}

func TestT20CheckerEnvironmentIsNotFail(t *testing.T) {
	t20Need(t, t20Checker,
		"started or worked around first",
		"NOT VERIFIED with the cause",
		"not a FAIL",
		"does not by itself make the verdict `REJECTED`",
	)
}
