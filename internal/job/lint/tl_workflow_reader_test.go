package lint_test

import (
	"encoding/json"
	"strings"
	"testing"

	contractdata "github.com/gridconsole/grid-plugins/contract"
)

const tlWorkflowCommand = "grid workflow show --task <task> --json"

// tlWorkflowSkillProblem reports what is wrong with the plan-jobs skill text, or "".
// The skill must hold one line with the command, task.steps and task.rounds, and
// no line may name `grid workflow show` without --task.
func tlWorkflowSkillProblem(text string) string {
	named := false
	for _, line := range strings.Split(text, "\n") {
		if strings.Contains(line, "grid workflow show") && !strings.Contains(line, "--task") {
			return "a line names `grid workflow show` without --task: " + strings.TrimSpace(line)
		}
		if strings.Contains(line, tlWorkflowCommand) && strings.Contains(line, "task.steps") && strings.Contains(line, "task.rounds") {
			named = true
		}
	}
	if !named {
		return "no line holds `" + tlWorkflowCommand + "`, `task.steps` and `task.rounds`"
	}
	return ""
}

// tlWorkflowOrchestratorProblem reports what is wrong with the orchestrator text, or "".
// The command must be on one line inside step 1, before the line "2. Decide the Jobs".
func tlWorkflowOrchestratorProblem(text string) string {
	for _, line := range strings.Split(text, "\n") {
		if strings.HasPrefix(line, "2. Decide the Jobs") {
			return "no line before \"2. Decide the Jobs\" holds `" + tlWorkflowCommand + "`"
		}
		if strings.Contains(line, tlWorkflowCommand) {
			// The command is one backticked span, so it holds no `;` and no pipe.
			if !strings.Contains(line, "`"+tlWorkflowCommand+"`") {
				return "the command is not one backticked span on its own: " + strings.TrimSpace(line)
			}
			return ""
		}
	}
	return "no line \"2. Decide the Jobs\" and no line with `" + tlWorkflowCommand + "`"
}

// tlWorkflowShowHasFlag reads the contract: command workflow, subcommand show, flag name.
func tlWorkflowShowHasFlag(t *testing.T, flag string) bool {
	t.Helper()
	var doc struct {
		Commands []struct {
			Name        string `json:"name"`
			Subcommands []struct {
				Name  string `json:"name"`
				Flags []struct {
					Name string `json:"name"`
				} `json:"flags"`
			} `json:"subcommands"`
		} `json:"commands"`
	}
	if err := json.Unmarshal(contractdata.Commands, &doc); err != nil {
		t.Fatal(err)
	}
	for _, c := range doc.Commands {
		if c.Name != "workflow" {
			continue
		}
		for _, s := range c.Subcommands {
			if s.Name != "show" {
				continue
			}
			for _, f := range s.Flags {
				if f.Name == flag {
					return true
				}
			}
		}
	}
	return false
}

func TestTlWorkflowReader(t *testing.T) {
	skill := readText(t, skillPath)
	orch := readText(t, orchestratorPath)

	// The check itself must bite: the skill text with --task cut out fails (a).
	cut := strings.ReplaceAll(skill, "--task <task> ", "")
	cut = strings.ReplaceAll(cut, "--task", "")
	if tlWorkflowSkillProblem(cut) == "" {
		t.Fatal("check (a) passes a skill text with --task cut out, so it proves nothing")
	}
	if tlWorkflowOrchestratorProblem(strings.ReplaceAll(orch, "--task <task> --json", "--json")) == "" {
		t.Fatal("check (b) passes an orchestrator text with --task cut out, so it proves nothing")
	}

	if p := tlWorkflowSkillProblem(skill); p != "" {
		t.Errorf("skills/plan-jobs/SKILL.md does not name `grid workflow show --task`: %s", p)
	}
	if p := tlWorkflowOrchestratorProblem(orch); p != "" {
		t.Errorf("agents/orchestrator.md does not name `grid workflow show --task`: %s", p)
	}
	if !tlWorkflowShowHasFlag(t, "--task") {
		t.Error("contract commands.json lists no flag --task under workflow show")
	}
}
