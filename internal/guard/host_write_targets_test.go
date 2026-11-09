package guard

import (
	"fmt"
	"path/filepath"
	"testing"
)

func TestHostWriteTargets(t *testing.T) {
	f := setup(t)
	project := filepath.Join(f.projects, "example")
	policy := filepath.Join(project, "workflows.md")
	f.write(policy, "policy")
	f.link(filepath.Join(project, "alias"), policy)
	f.link(filepath.Join(f.task, "task-alias"), filepath.Join(f.task, "task.md"))
	f.write(filepath.Join(f.task, "workflow.md"), "policy")
	f.link(filepath.Join(f.task, "workflow-alias"), filepath.Join(f.task, "workflow.md"))
	f.link(filepath.Join(f.repo, "policy alias"), policy)
	outside := filepath.Join(f.repo, "a*.txt")
	for _, tt := range []struct {
		name, cwd, command string
		deny               bool
	}{
		{"glob_question", f.repo, "echo x > " + filepath.Join(project, "workflow?.md"), true},
		{"glob_bracket", project, "echo x > workflow[s].md", true},
		{"glob_star", project, "echo x > workflows.m*", true},
		{"glob_missing_policy", f.grid, "echo x > workflow?.md", true},
		{"glob_task", f.task, "echo x > tas?.md", true},
		{"glob_task_workflow", f.task, "echo x > workflo?.md", true},
		{"glob_alias", project, "echo x > ali*", true},
		{"glob_directory", f.repo, "echo x > " + filepath.Join(f.projects, "exam*", "workflow?.md"), true},
		{"parameter", project, "echo x > workflow${E}s.md", true},
		{"substitution", project, "echo x > $(printf workflows).md", true},
		{"backticks", project, "echo x > `printf workflows`.md", true},
		{"absolute_expansion", f.repo, "echo x > " + filepath.Join(project, "workflow${E}s.md"), true},
		{"dd_prefix", f.repo, "dd of=" + filepath.Join(project, "alias"), true},
		{"output_prefix", f.repo, "writer --output=" + filepath.Join(project, "alias"), true},
		{"file_prefix", f.repo, "writer --file=" + filepath.Join(project, "alias"), true},
		{"quoted_prefix", f.repo, fmt.Sprintf("dd of=%q", filepath.Join(f.repo, "policy alias")), true},
		{"tee_append", project, "echo x | tee -a alias", true},
		{"cd_and", f.repo, "cd " + project + " && echo x > alias", true},
		{"cd_semicolon", f.repo, "cd " + project + "; echo x > alias", true},
		{"cd_glob", f.repo, "cd " + project + " && echo x > ali*", true},
		{"cd_expansion", project, "cd ${DEST} && echo x > alias", true},
		{"cd_absolute_expansion", f.repo, "cd " + project + "/${DEST}; echo x > alias", true},
		{"task_alias_prefix", f.task, "dd of=task-alias", true},
		{"workflow_alias_prefix", f.task, "dd of=workflow-alias", true},
		{"outside_glob", project, "echo x > " + outside, false},
		{"outside_echo_expansion", f.repo, "echo $HOME > notes.md", false},
		{"outside_dd", project, "dd of=" + filepath.Join(f.repo, "x"), false},
		{"read", project, "cat workflows.md", false},
		{"read_glob", project, "cat workflow?.md", false},
		{"read_expansion", project, "cat workflow${E}s.md", false},
		{"read_alias", project, "cat alias", false},
		{"read_then_write", project, "cat alias; echo x > notes.md", false},
		{"echo_expansion", project, "echo $HOME > notes.md", false},
		{"cd_outside", project, "cd " + f.repo + " && echo x > a*.txt", false},
		{"outside_target_expansion", project, "echo x > " + filepath.Join(f.repo, "${NAME}.txt"), false},
		{"ordinary_grid_glob", project, "echo x > notes*.txt", false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			in := f.input("Bash", tt.command)
			in.Cwd = tt.cwd
			got := EvaluateHost(in, f.root, f.env)
			want := ""
			if tt.deny {
				want = "deny"
			}
			if got.Behavior != want {
				t.Fatalf("behavior = %q, want %q", got.Behavior, want)
			}
			if tt.deny && tt.name != "workflow_alias_prefix" && got.Reason != gateReason {
				t.Fatal("write did not use the direct protected-write reason")
			}
		})
	}
}
