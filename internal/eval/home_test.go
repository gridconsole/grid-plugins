package eval

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gridconsole/grid-plugins/internal/workflow"
)

// fakeAgentSamples are tier b too, but plain workspace tasks with no Grid home.
var fakeAgentSamples = map[string]bool{"clean-csv": true, "fix-total": true, "release-notes": true}

func tierB(t *testing.T) []Task {
	t.Helper()
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("no sh")
	}
	tasks, err := LoadSuite("../../evals/tasks")
	if err != nil {
		t.Fatal(err)
	}
	var out []Task
	for _, task := range tasks {
		if task.Tier == "b" && !fakeAgentSamples[task.ID] {
			out = append(out, task)
		}
	}
	if len(out) != 19 {
		t.Fatalf("only %d tier-b cases", len(out))
	}
	return out
}

// attemptDirs builds the folders the runner makes before setup and the runner's own environment.
func attemptDirs(t *testing.T, id string) (root, workspace string, env []string) {
	t.Helper()
	root = t.TempDir()
	workspace = filepath.Join(root, "workspace")
	home := filepath.Join(root, "home")
	tmp := filepath.Join(root, "tmp")
	for _, d := range []string{workspace, home, tmp, filepath.Join(home, ".grid", "projects"), filepath.Join(home, ".config")} {
		if err := os.MkdirAll(d, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	return root, workspace, taskEnvironment(home, workspace, tmp, id, nil)
}

func envValue(env []string, key string) string {
	for _, e := range env {
		if v, ok := strings.CutPrefix(e, key+"="); ok {
			return v
		}
	}
	return ""
}

func setup(t *testing.T, task Task, workspace string, env []string) ProcessResult {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	return RunProcess(ctx, []string{"sh", "-eu", "-c", task.Setup}, workspace, env, "")
}

func projectOf(id string) string {
	switch id {
	case "execute-check-after-make-together", "execute-check-fails-out-of-scope":
		return "site"
	}
	return "notes-app"
}

func TestSetupBuildsGridHomeAtGridHome(t *testing.T) {
	for _, task := range tierB(t) {
		t.Run(task.ID, func(t *testing.T) {
			_, workspace, env := attemptDirs(t, task.ID)
			if r := setup(t, task, workspace, env); r.Status != "passed" {
				t.Fatalf("setup %s: %+v", r.Status, r)
			}
			gridHome := envValue(env, "GRID_HOME")
			tasks := filepath.Join(gridHome, "projects", projectOf(task.ID), "tasks")
			if task.ID == "routing-skill" { // routing only: the project has a routing.md and no task
				tasks = filepath.Join(gridHome, "projects", projectOf(task.ID))
			}
			if entries, err := os.ReadDir(tasks); err != nil || len(entries) == 0 {
				t.Fatalf("no tasks under %s: %v", tasks, err)
			}
			if _, err := os.Stat(filepath.Join(workspace, "home", "projects")); err == nil {
				t.Fatal("workspace holds a home/projects tree")
			}
			if _, err := os.Stat(filepath.Join(gridHome, "VERSION")); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestSetupRefusesWithoutGridHome(t *testing.T) {
	for _, task := range tierB(t) {
		t.Run(task.ID, func(t *testing.T) {
			root, workspace, env := attemptDirs(t, task.ID)
			var bare []string
			for _, e := range env {
				if !strings.HasPrefix(e, "GRID_HOME=") && !strings.HasPrefix(e, "GRID_PROJECTS=") {
					bare = append(bare, e)
				}
			}
			r := setup(t, task, workspace, bare)
			if r.Status == "passed" {
				t.Fatal("setup passed without GRID_HOME")
			}
			if !strings.Contains(r.Stderr, "GRID_HOME") {
				t.Fatalf("no message naming GRID_HOME: %q", r.Stderr)
			}
			home := filepath.Join(root, "home")
			for _, dir := range []string{workspace, filepath.Join(home, ".config"), filepath.Join(root, "tmp")} {
				if entries, _ := os.ReadDir(dir); len(entries) != 0 {
					t.Fatalf("setup wrote into %s: %v", dir, entries)
				}
			}
			entries, _ := os.ReadDir(home)
			if len(entries) != 2 { // .grid and .config, made before setup
				t.Fatalf("setup wrote into the home folder: %v", entries)
			}
		})
	}
}

func TestPromptsDoNotPointAtDotHome(t *testing.T) {
	for _, task := range tierB(t) {
		if strings.Contains(task.Prompt, "./home") {
			t.Errorf("%s: prompt names ./home", task.ID)
		}
		if strings.Contains(task.Setup, "./home") {
			t.Errorf("%s: setup names ./home", task.ID)
		}
	}
}

// Every command check must exit 0 or 1 on the fresh fixture; 2 or more is a usage or path error.
func TestChecksRunOnFreshFixture(t *testing.T) {
	for _, task := range tierB(t) {
		t.Run(task.ID, func(t *testing.T) {
			_, workspace, env := attemptDirs(t, task.ID)
			if r := setup(t, task, workspace, env); r.Status != "passed" {
				t.Fatalf("setup: %+v", r)
			}
			for _, c := range task.Checks {
				if c.Type != "command_passes" {
					continue
				}
				r := evaluate(context.Background(), task, c, workspace, env, nil, traceInput{})
				if r.Process == nil || r.Process.ExitCode < 0 || r.Process.ExitCode > 1 {
					t.Errorf("%s: exit %+v", c.ID, r.Process)
				}
			}
		})
	}
}

// goodOutcome is a hand-made result for a case; every check of it must pass.
var goodOutcome = map[string]string{
	"execute-check-fails-out-of-scope": `
t="$GRID_HOME/projects/site/tasks/T-01-pricing-page"
mkdir -p "$t/questions"
sed -e 's/^status: running$/status: waiting/' -e 's/^next: .*/next: Sam answers Q-1./' -e 's/^updated: .*/&\nneeds: [Q-1]/' "$t/task.md" > "$t/task.new" && mv "$t/task.new" "$t/task.md"
printf -- '- 4 Mar · links · check failed (exit 1), broken links in signup.html and contact.html\n' >> "$t/task.md"
printf '## Q-1 · to: sam · blocking · open\nThe link check fails on signup.html and contact.html, which this task must leave alone. Fix them here?\n\nRecommendation: leave them to the other task.\n' > "$t/questions/sam.md"
`,
	"execute-review-changes-rework": `
t="$GRID_HOME/projects/notes-app/tasks/T-01-tag-filter"
sed -e 's/^step: review$/step: build/' -e 's/^round: 1$/round: 2/' -e 's/^status: running$/status: reworking/' "$t/task.md" > "$t/task.new" && mv "$t/task.new" "$t/task.md"
printf -- '- 2026-03-04T11:00 · review · sent back to build · round 2 of 3\n' >> "$t/history.md"
`,
	"agent-reviewer-read-only": `
f="$GRID_HOME/projects/notes-app/tasks/T-01-sample/subagents/01-total.md"
sed -e 's/^## Verdict$/## Verdict\nREJECTED\n\nMust fix:\n- A2 total([]) returns 1./' "$f" > "$f.new" && mv "$f.new" "$f"
`,
	"deliver-refuses-closed": ``,
	"skill-brain-keeper": `
f="$GRID_HOME/projects/notes-app/knowledge/conventions/tag-filter-ignores-case.md"
cat > "$f" <<'X'
---
name: tag-filter-ignores-case
topic: conventions
status: accepted
links:
  - to: brain:project/tag-storage
    rel: depends-on
    summary: How notes store their tags
    why: Read it first to know the tags array that this filter lowercases
---
The tag filter ignores case: work/notes-app/src/store.js:3 lowercases both sides.
X
`,
	"skill-browser": `
cat > verdict.md <<'X'
| Row | Result | Evidence |
| R1 heading reads Notes | NOT VERIFIED | No browser; widths 390, 1440, 3840 not captured |
| R2 Delete all visible | NOT VERIFIED | No browser; widths 390, 1440, 3840 not captured |
| R3 no sideways scroll | NOT VERIFIED | No browser; widths 390, 1440, 3840 not captured |
X
`,
	"skill-visual-design-review": `
cat > review.md <<'X'
Screenshots at 390, 1440 and 3840: NOT VERIFIED, no browser.
- Severity: high. Where: lede paragraph. What is wrong: low contrast. Fix: darken the colour.
- Severity: medium. Where: sentence. What is wrong: a sentence set in monospace. Fix: use the body font.
- Severity: medium. Where: meta line. What is wrong: 10px text. Fix: raise it to 12px.
- Severity: medium. Where: the plus button. What is wrong: the tap target is under 44px. Fix: make it 44px.
X
`,
}

func TestGoodOutcomesPass(t *testing.T) {
	byID := map[string]Task{}
	for _, task := range tierB(t) {
		byID[task.ID] = task
	}
	for id, script := range goodOutcome {
		t.Run(id, func(t *testing.T) {
			task := byID[id]
			_, workspace, env := attemptDirs(t, id)
			if r := setup(t, task, workspace, env); r.Status != "passed" {
				t.Fatalf("setup: %+v", r)
			}
			if script != "" {
				if r := RunProcess(context.Background(), []string{"sh", "-eu", "-c", script}, workspace, env, ""); r.Status != "passed" {
					t.Fatalf("outcome script: %+v", r)
				}
			}
			for _, c := range task.Checks {
				if r := evaluate(context.Background(), task, c, workspace, env, nil, traceInput{}); r.Status != "passed" {
					t.Errorf("%s: %s %s", c.ID, r.Status, fmt.Sprint(r.Process))
				}
			}
		})
	}
}

// The review round limit is the "review rounds" line of the project's workflows.md, which has no default.
func TestReviewRoundFixturesAreValidWorkflows(t *testing.T) {
	byID := map[string]Task{}
	for _, task := range tierB(t) {
		byID[task.ID] = task
	}
	for id, rounds := range map[string]uint8{"execute-review-changes-rework": 3, "execute-review-changes-past-limit": 2} {
		_, workspace, env := attemptDirs(t, id)
		if r := setup(t, byID[id], workspace, env); r.Status != "passed" {
			t.Fatalf("setup: %+v", r)
		}
		text, err := os.ReadFile(filepath.Join(envValue(env, "GRID_HOME"), "projects", "notes-app", "workflows.md"))
		if err != nil {
			t.Fatal(err)
		}
		parsed := workflow.Parse(string(text), "notes-app")
		if len(parsed.Errors) != 0 || len(parsed.Templates) != 1 || parsed.Templates[0].Rounds != rounds {
			t.Fatalf("%s: %+v", id, parsed)
		}
	}
}
