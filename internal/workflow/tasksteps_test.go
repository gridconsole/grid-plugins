package workflow

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/where"
)

const stepsTable = "| # | step | kind | who | sizes | on reject, back to |\n|---|------|------|-----|-------|--------------------|\n"

const stepsStandard = "## Standard\napplies when: nothing else matches\nreview rounds: 2\n\n" + stepsTable +
	"| 1 | plan | approve | owner | all | |\n| 2 | preview | approve | owner | L | |\n| 3 | build | run | grid-builder | all | |\n\n"

const stepsReview = "## Review\napplies when: label review\nreview rounds: 2\n\n" + stepsTable +
	"| 1 | check-it | check | grid-runner | all | |\n\n"

const stepsSpaced = "## Spaced\napplies when: label spaced\nreview rounds: 2\n\n" + stepsTable +
	"| 1 | Write it | run | grid-builder | all | |\n\n"

const stepsReviewCopy = "## Review\napplies when: as in the workspace Review\nreview rounds: 2\n\n" + stepsTable +
	"| 1 | ship | run | grid-builder | all | |\n\n"

const stepsBuildDefault = "## Standard\napplies when: nothing else matches\nreview rounds: 2\n\n" + stepsTable +
	"| 1 | build | run | grid-builder | all | |\n\n"

const stepsDefaultCopy = "## Standard\napplies when: as in the workspace Standard\nreview rounds: 2\n\n" + stepsTable +
	"| 1 | plan | approve | owner | all | |\n| 2 | build | run | grid-builder | all | |\n\n"

const stepsRates = "## Rates\napplies when: the title has rates\nreview rounds: 2\n\n" + stepsTable +
	"| 1 | rate | run | grid-builder | all | |\n\n"

const stepsBroken = "## Broken\napplies when: label broken\nreview rounds: 2\n\n" + stepsTable +
	"| 1 | wait | sleep | grid-builder | all | |\n\n"

func stepsWorkspace(sections ...string) string {
	return "# Workflows\n\n" + strings.Join(sections, "")
}

func stepsTask(size, workflow string) string {
	text := "---\nid: T-01\ntitle: Steps\nproject: alpha\n"
	if size != "-" {
		text += "size: " + size + "\n"
	}
	if workflow != "-" {
		text += "workflow: " + workflow + "\n"
	}
	return text + "state: queued\ncreated: 2026-11-01T09:00:00Z\n---\n"
}

const stepsInline = "---\nid: T-01\ntitle: Steps\nproject: alpha\nsize: normal\nworkflow:\n  - {step: plan, kind: approve}\n  - {step: build, kind: run}\nstate: queued\n---\n"

func stepsString(t *testing.T, r where.Resolver, project, text string) (string, error) {
	t.Helper()
	steps, err := TaskSteps(r, project, text)
	if err == nil && len(steps) == 0 {
		t.Fatalf("TaskSteps returned no steps and no error")
	}
	var parts []string
	for _, s := range steps {
		parts = append(parts, s.Step+":"+s.Kind)
	}
	return strings.Join(parts, ","), err
}

func TestTaskSteps(t *testing.T) {
	std := stepsWorkspace(stepsStandard, stepsReview, stepsSpaced)
	for _, tc := range []struct {
		name, workspace, alpha, project, role, task string
		symlink                                     bool
		want, wantErr                               string
	}{
		{name: "inline list", task: stepsInline, want: "plan:approve,build:run"},
		{name: "inline list wins over a template", workspace: std, task: stepsInline, want: "plan:approve,build:run"},
		{name: "malformed inline list", workspace: std, task: strings.Replace(stepsInline, "step: build", "step: Build", 1), wantErr: "task.md workflow list is malformed"},
		{name: "empty inline list", workspace: std, task: stepsTask("normal", "[]"), wantErr: "task.md workflow list is malformed"},
		{name: "project template beats workspace", workspace: std, alpha: stepsWorkspace(stepsReviewCopy), task: stepsTask("normal", "review"), want: "ship:run"},
		{name: "name only in workspace", workspace: std, task: stepsTask("normal", "Review"), want: "check-it:check"},
		{name: "personal reads the workspace only", workspace: std, alpha: stepsWorkspace(stepsReviewCopy), project: "personal", task: stepsTask("normal", "Review"), want: "check-it:check"},
		{name: "personal refused to a guest", workspace: std, project: "personal", role: "guest", task: stepsTask("normal", "Review"), wantErr: "missing"},
		{name: "name not found", workspace: std, task: stepsTask("normal", "Missing"), wantErr: `no workflow is named "Missing"`},
		{name: "project file a symlink", workspace: std, symlink: true, task: stepsTask("normal", "Standard"), wantErr: "alpha/workflows.md: symlink refused"},
		{name: "parse error in the named section", workspace: std + stepsReview, task: stepsTask("normal", "review"), wantErr: `section "Review"`},
		{name: "parse error in another section", workspace: std + stepsBroken, task: stepsTask("normal", "Review"), want: "check-it:check"},
		{name: "workspace file-level problem", workspace: "# Not workflows\n", task: stepsTask("normal", "Standard"), wantErr: "workflows.md"},
		{name: "empty name takes the default", workspace: stepsWorkspace(stepsStandard), task: stepsTask("normal", "''"), want: "plan:approve,build:run"},
		{name: "absent name takes the default", workspace: stepsWorkspace(stepsStandard), task: stepsTask("normal", "-"), want: "plan:approve,build:run"},
		{name: "empty name with a rule template", workspace: stepsWorkspace(stepsStandard), alpha: stepsWorkspace(stepsRates), task: stepsTask("normal", "''"), wantErr: "task.md names no workflow and the rules need a choice"},
		{name: "empty name with a broken section", workspace: stepsWorkspace(stepsStandard, stepsBroken), task: stepsTask("normal", "''"), wantErr: "task.md names no workflow and the rules need a choice"},
		{name: "empty name with no workspace file", task: stepsTask("normal", "''"), wantErr: "the workspace has no default workflow"},
		{name: "empty name takes the project copy of the default", workspace: stepsWorkspace(stepsBuildDefault), alpha: stepsWorkspace(stepsDefaultCopy), task: stepsTask("normal", "''"), want: "plan:approve,build:run"},
		{name: "absent name takes the project copy of the default", workspace: stepsWorkspace(stepsBuildDefault), alpha: stepsWorkspace(stepsDefaultCopy), task: stepsTask("normal", "-"), want: "plan:approve,build:run"},
		{name: "size large keeps the L step", workspace: std, task: stepsTask("large", "Standard"), want: "plan:approve,preview:approve,build:run"},
		{name: "size normal drops the L step", workspace: std, task: stepsTask("normal", "Standard"), want: "plan:approve,build:run"},
		{name: "size auto with an L step", workspace: std, task: stepsTask("auto", "Standard"), wantErr: "task size is not set; the workflow's steps depend on it"},
		{name: "size absent with an L step", workspace: std, task: stepsTask("-", "Standard"), wantErr: "task size is not set; the workflow's steps depend on it"},
		{name: "size auto with only all steps", workspace: std, task: stepsTask("auto", "Review"), want: "check-it:check"},
		{name: "size not a size", workspace: std, task: stepsTask("huge", "Standard"), wantErr: `task size "huge"`},
		{name: "step name with a space", workspace: std, task: stepsTask("normal", "Spaced"), wantErr: `step "Write it"`},
		{name: "workflow a map", workspace: std, task: stepsTask("normal", "{a: b}"), wantErr: "not a name or a list"},
		{name: "no project", workspace: std, project: "-", task: stepsTask("normal", "Standard"), wantErr: "no project"},
		{name: "no front matter", workspace: std, task: "workflow: Standard\n", wantErr: "front matter"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			home := t.TempDir()
			putWorkflowFixture(t, filepath.Join(home, "projects", "alpha", "project.md"), "---\nid: id-alpha\n---\n")
			if tc.workspace != "" {
				putWorkflowFixture(t, filepath.Join(home, "workflows.md"), tc.workspace)
			}
			if tc.alpha != "" {
				putWorkflowFixture(t, filepath.Join(home, "projects", "alpha", "workflows.md"), tc.alpha)
			}
			if tc.symlink {
				if err := os.Symlink(filepath.Join(home, "workflows.md"), filepath.Join(home, "projects", "alpha", "workflows.md")); err != nil {
					t.Skipf("symlinks unavailable: %v", err)
				}
			}
			project := tc.project
			switch project {
			case "":
				project = "alpha"
			case "-":
				project = ""
			}
			r := where.Resolver{GridHome: home, Caller: where.Caller{Role: tc.role}}
			got, err := stepsString(t, r, project, tc.task)
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("got %q, %v; want error with %q", got, err, tc.wantErr)
				}
				if strings.Contains(err.Error(), home) {
					t.Fatalf("error names a path: %v", err)
				}
				return
			}
			if err != nil || got != tc.want {
				t.Fatalf("got %q, %v; want %q", got, err, tc.want)
			}
		})
	}
}
