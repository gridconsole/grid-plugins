package eval

import (
	"context"
	"strings"
	"testing"
	"time"
)

const smallDir = "T-02-fix-title"
const normalDir = "T-03-tag-filter"

const smallPlan = `# Plan, rev 1

## Where it stands

Nothing is built yet.
As of: plan

## How we approach it

Trim the name and fall back to the bare word when it is empty.
Recall: nothing found

## Risks

A name of only spaces must give the bare word too. Step 1 covers it.

## Steps

- [ ] 1 Fix pageTitle in src/title.js
  Return "Notes" when the trimmed name is empty.
  needs: none

## Verify

An empty name shows Notes.
`

const normalPlan = `# Plan, rev 1

## Where it stands

Nothing is built yet.
As of: plan

## How we approach it

One filter in the store reaches the list view and the command line, as research.md shows (list.js:3, cli.js:3).
Used: brain:project/conventions/tag-matching

## Options

- A: add filterByTag to the store. Small, and the list view reuses it.
- B: teach searchNotes a tag: prefix. One entry point, but it needs a parser.
Chosen: A, because research.md found no query parser and A touches fewer files.

## Risks

Tags typed as Home would miss home. Step 1 matches without case.

## Steps

- [ ] 1 Add filterByTag to the store
  Match tags without case sensitivity.
  needs: none
- [ ] 2 Write the store tests
  Cover a hit, a miss and a different case.
  needs: 1
- [ ] 3 Pass the tag through renderList and the command line
  Take the tag as a second argument.
  needs: 1
- [ ] 4 Write the release note
  Explain the tag argument.
  needs: none

## Verify

Running the command line with the tag home lists Groceries only.
`

func writePlanAgent(taskDir, plan string) []string {
	return []string{"sh", "-c", "cat > \"$GRID_HOME\"/projects/notes-app/tasks/" + taskDir + "/plan.md <<'PLAN'\n" + plan + "PLAN\n"}
}

func planQualityTask(t *testing.T, id string) Task {
	t.Helper()
	tasks, err := LoadSuite("../../evals/tasks")
	if err != nil {
		t.Fatal(err)
	}
	for _, task := range tasks {
		if task.ID == id {
			return task
		}
	}
	t.Fatalf("case %s is not in evals/tasks", id)
	return Task{}
}

// TestPlanQualityCasesAreReadChecksOnly proves each case loads under the eval schema,
// carries the tier and size the brief names, and asks nothing of a model or the network.
func TestPlanQualityCasesAreReadChecksOnly(t *testing.T) {
	for id, size := range map[string]string{"plan-quality-small": "small", "plan-quality-normal": "normal"} {
		task := planQualityTask(t, id)
		if task.Tier != "b" || task.Size != size {
			t.Errorf("%s: tier %q size %q, want b %s", id, task.Tier, task.Size, size)
		}
		if !strings.Contains(task.Setup, "mkdir -p work/notes-app") || !strings.Contains(task.Setup, "$GRID_HOME") {
			t.Errorf("%s: setup does not build the fixture repo and task folder", id)
		}
		for _, c := range task.Checks {
			if c.Type == "llm_judge" {
				t.Errorf("%s: check %s calls a judge", id, c.ID)
			}
			if c.Type == "command_passes" {
				joined := strings.Join(c.Command, " ")
				for _, word := range []string{"claude", "curl", "wget", "http://", "https://", "nc "} {
					if strings.Contains(joined, word) {
						t.Errorf("%s: check %s uses %q", id, c.ID, word)
					}
				}
			}
		}
	}
	has := func(task Task, id string) bool {
		for _, c := range task.Checks {
			if c.ID == id {
				return true
			}
		}
		return false
	}
	small, normal := planQualityTask(t, "plan-quality-small"), planQualityTask(t, "plan-quality-normal")
	for _, id := range []string{"sections", "needs-lines", "recall-line", "no-research-step"} {
		if !has(small, id) {
			t.Errorf("plan-quality-small lacks check %s", id)
		}
	}
	for _, id := range []string{"sections", "needs-lines", "recall-line", "research-cited", "options-chosen"} {
		if !has(normal, id) {
			t.Errorf("plan-quality-normal lacks check %s", id)
		}
	}
}

// TestPlanQualityChecksSeparateGoodPlansFromBad runs setup and the checks with a scripted
// agent that writes a plan, so the checks are proven against real files, not assumed.
func TestPlanQualityChecksSeparateGoodPlansFromBad(t *testing.T) {
	cases := []struct {
		name, id, dir, plan string
		wantFailed          []string // empty: the run passes
	}{
		{"small good", "plan-quality-small", smallDir, smallPlan, nil},
		{"small cites research", "plan-quality-small", smallDir,
			strings.Replace(smallPlan, "Recall: nothing found", "Recall: nothing found\nSee research.md.", 1), []string{"no-research-step"}},
		{"small step without needs", "plan-quality-small", smallDir,
			strings.Replace(smallPlan, "  needs: none\n", "", 1), []string{"needs-lines"}},
		{"small no recall", "plan-quality-small", smallDir,
			strings.Replace(smallPlan, "Recall: nothing found\n", "", 1), []string{"recall-line"}},
		{"normal good", "plan-quality-normal", normalDir, normalPlan, nil},
		{"normal no research cited", "plan-quality-normal", normalDir,
			strings.ReplaceAll(normalPlan, "research.md", "the notes"), []string{"research-cited"}},
		{"normal no options", "plan-quality-normal", normalDir,
			strings.Replace(normalPlan, "## Options", "## Other", 1), []string{"sections"}},
		{"normal no choice", "plan-quality-normal", normalDir,
			strings.Replace(normalPlan, "Chosen:", "Picked:", 1), []string{"options-chosen"}},
		{"normal step without needs", "plan-quality-normal", normalDir,
			strings.Replace(normalPlan, "  needs: 1\n", "", 1), []string{"needs-lines"}},
		{"normal no risks", "plan-quality-normal", normalDir,
			strings.Replace(normalPlan, "## Risks", "## Worries", 1), []string{"sections"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			runner := Runner{Agent: writePlanAgent(tc.dir, tc.plan), ResultsDir: t.TempDir(), TempDir: t.TempDir(), Timeout: 20 * time.Second}
			result, err := runner.Run(context.Background(), planQualityTask(t, tc.id))
			if err != nil {
				t.Fatal(err)
			}
			if result.Setup.Status != "passed" || result.AgentResult.Status != "passed" {
				t.Fatalf("setup %s agent %s: %+v", result.Setup.Status, result.AgentResult.Status, result.Setup)
			}
			var failed []string
			for _, c := range result.Checks {
				if c.Status != "passed" {
					failed = append(failed, c.ID)
				}
			}
			if strings.Join(failed, ",") != strings.Join(tc.wantFailed, ",") {
				t.Fatalf("failed checks %v, want %v", failed, tc.wantFailed)
			}
		})
	}
}

func TestPlanQualityMissingPlanFailsTheGate(t *testing.T) {
	runner := Runner{Agent: []string{"sh", "-c", "true"}, ResultsDir: t.TempDir(), TempDir: t.TempDir(), Timeout: 20 * time.Second}
	result, err := runner.Run(context.Background(), planQualityTask(t, "plan-quality-small"))
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "failed" || result.Summary.GatesPassed != 0 {
		t.Fatalf("status %s gates %d/%d", result.Status, result.Summary.GatesPassed, result.Summary.GatesTotal)
	}
}
