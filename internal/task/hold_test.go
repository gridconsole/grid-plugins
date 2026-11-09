package task

import (
	"path/filepath"
	"testing"
)

const holdStandard = "# Workflows\n\n## Standard\napplies when: nothing else matches\nreview rounds: 2\n\n" +
	"| # | step | kind | who | sizes | on reject, back to |\n|---|------|------|-----|-------|--------------------|\n" +
	"| 1 | plan | approve | owner | all | |\n| 2 | build | run | grid-builder | all | |\n\n"

func holdText(project, step, workflow string) string {
	text := "---\nid: T-01\ntitle: Held\n"
	if project != "" {
		text += "project: " + project + "\n"
	}
	return text + "size: large\nworkflow: " + workflow + "\nstate: running\nstep: " + step + "\n---\n"
}

// The project a task is told through is the folder's, and task.md must agree:
// a folder moved under another project reads as unreadable, so it stays held.
// Personal tasks, which belong to no project, and inline lists read as before.
func TestStepsForNeedsTheFolderProject(t *testing.T) {
	s, _ := storeFor(t)
	write(t, filepath.Join(s.Resolver.GridHome, "workflows.md"), holdStandard)
	for _, project := range []string{"alpha", "beta"} {
		write(t, filepath.Join(s.Resolver.GridHome, "projects", project, "project.md"), "---\nid: id-"+project+"\n---\n")
	}
	personal := filepath.Join(s.Resolver.GridHome, "personal", "tasks", "T-01-held")
	inProjects := func(project string) string {
		return filepath.Join(s.Resolver.GridHome, "projects", project, "tasks", "T-01-held")
	}
	const inline = "[{step: plan, kind: approve}, {step: build, kind: run}]"
	for _, tc := range []struct {
		name, dir, text, stage string
	}{
		{"project task in its folder", inProjects("alpha"), holdText("alpha", "plan", "''"), "gate"},
		{"project task past plan", inProjects("alpha"), holdText("alpha", "build", "''"), "past"},
		{"moved to another project", inProjects("beta"), holdText("alpha", "plan", "''"), "unreadable"},
		{"moved past plan", inProjects("beta"), holdText("alpha", "build", "''"), "unreadable"},
		{"moved into Personal", personal, holdText("alpha", "plan", "''"), "unreadable"},
		{"Personal task moved into a project", inProjects("alpha"), holdText("personal", "plan", "''"), "unreadable"},
		{"no project moved into a project", inProjects("alpha"), holdText("", "plan", "''"), "unreadable"},
		{"a named workflow moved", inProjects("beta"), holdText("alpha", "plan", "Standard"), "unreadable"},
		{"Personal task in its folder", personal, holdText("personal", "plan", "''"), "gate"},
		{"Personal task past plan", personal, holdText("personal", "build", "''"), "past"},
		{"Personal task with no project key", personal, holdText("", "plan", "''"), "gate"},
		{"Personal task, named workflow", personal, holdText("personal", "plan", "Standard"), "gate"},
		{"inline list in another project's folder", inProjects("beta"), holdText("alpha", "plan", inline), "gate"},
		{"inline list with no project", inProjects("beta"), holdText("", "plan", inline), "gate"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := PlanStageWith(StepsFor(s.Resolver, tc.dir), tc.text, ""); got != tc.stage {
				t.Errorf("stage %q, want %q", got, tc.stage)
			}
			if _, err := StepsFor(s.Resolver, tc.dir)(tc.text); (err != nil) != (tc.stage == "unreadable") {
				t.Errorf("lookup err=%v, stage %q", err, tc.stage)
			}
		})
	}
}
