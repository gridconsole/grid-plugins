package hook

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/feed"
	"github.com/gridconsole/grid-plugins/internal/hookio"
)

// jobFixture is the hook environment of a Job's agent: the env the app's Worker
// sets (GRID_ROLE, GRID_TASK, GRID_MEMBER, GRID_WORKER_ID, GRID_JOB,
// GRID_ATTEMPT), a task with its files under the Grid home, one project brain
// note, and a cwd that is a repo folder outside the Grid home (a Job's worktree).
type jobFixture struct {
	*eventFixture
	task, repo, note string
}

func jobSetup(t *testing.T) *jobFixture {
	t.Helper()
	f := eventSetup(t)
	root := filepath.Dir(f.home)
	repo := filepath.Join(root, "repo")
	if err := os.MkdirAll(repo, 0o700); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"task.md", "plan.md", "history.md"} {
		p := filepath.Join(f.dir, name)
		if _, err := os.Stat(p); err != nil {
			if err := os.WriteFile(p, []byte("# "+name+"\n"), 0o600); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := os.MkdirAll(filepath.Join(f.dir, "feed"), 0o700); err != nil {
		t.Fatal(err)
	}
	project := filepath.Dir(filepath.Dir(f.dir))
	note := filepath.Join(project, "knowledge", "ops", "deploy.md")
	if err := os.MkdirAll(filepath.Dir(note), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(note, []byte("# Deploy\nHow we deploy.\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	// The Worker names the task by the id in its task.md (where.go: taskULID).
	id, err := feed.TaskID(f.dir)
	if err != nil {
		t.Fatal(err)
	}
	f.env["GRID_ROLE"] = "worker"
	f.env["GRID_TASK"] = id
	f.env["GRID_MEMBER"] = "member@example.com"
	f.env["GRID_WORKER_ID"] = "worker-demo"
	f.env["GRID_JOB"] = "J-1"
	f.env["GRID_ATTEMPT"] = "1"
	f.o.Cwd = repo
	return &jobFixture{eventFixture: f, task: id, repo: repo, note: note}
}

// decide runs one tool call through the hook entry point and returns the
// decision main gives ("allow", "deny", or "silence" for no decision) and the
// text the hook wrote.
func (j *jobFixture) decide(t *testing.T, event, tool string, input map[string]any) (string, string) {
	t.Helper()
	out, log, code := j.run(t, event, hookio.Input{ToolName: tool, Cwd: j.repo, ToolInput: input})
	if code != 0 || log != "" {
		t.Fatalf("%s %s: exit %d, log %q", event, tool, code, log)
	}
	if strings.TrimSpace(out) == "" {
		return "silence", ""
	}
	var d struct {
		Output struct {
			Behavior string `json:"permissionDecision"`
			Reason   string `json:"permissionDecisionReason"`
			Decision struct {
				Behavior string `json:"behavior"`
				Message  string `json:"message"`
			} `json:"decision"`
		} `json:"hookSpecificOutput"`
		SystemMessage string `json:"systemMessage"`
	}
	if err := json.Unmarshal([]byte(out), &d); err != nil {
		t.Fatalf("%s %s: %v: %s", event, tool, err, out)
	}
	got, reason := d.Output.Behavior, d.Output.Reason
	if event == "PermissionRequest" {
		got, reason = d.Output.Decision.Behavior, d.Output.Decision.Message
	}
	if got == "" {
		got = "silence"
	}
	if d.SystemMessage != "" {
		reason = strings.TrimSpace(reason + " [systemMessage: " + d.SystemMessage + "]")
	}
	return got, reason
}

func TestJobAgentToolCallsThroughTheHook(t *testing.T) {
	type call struct {
		name, tool string
		input      func(j *jobFixture) map[string]any
		// pre and perm are the decisions main gives on each event.
		pre, perm string
	}
	task := func(j *jobFixture, name string) string { return filepath.Join(j.dir, name) }
	read := func(name string) func(j *jobFixture) map[string]any {
		return func(j *jobFixture) map[string]any { return map[string]any{"file_path": task(j, name)} }
	}
	bash := func(cmd string) func(j *jobFixture) map[string]any {
		return func(*jobFixture) map[string]any { return map[string]any{"command": cmd} }
	}
	calls := []call{
		{"Read task.md", "Read", read("task.md"), "allow", "allow"},
		{"Read plan.md", "Read", read("plan.md"), "allow", "allow"},
		{"Read history.md", "Read", read("history.md"), "allow", "allow"},
		{"Read brain note", "Read", func(j *jobFixture) map[string]any { return map[string]any{"file_path": j.note} }, "allow", "allow"},
		{"Grep in task folder", "Grep", func(j *jobFixture) map[string]any { return map[string]any{"pattern": "x", "path": j.dir} }, "allow", "allow"},
		{"Glob in task folder", "Glob", func(j *jobFixture) map[string]any { return map[string]any{"pattern": "*.md", "path": j.dir} }, "allow", "allow"},
		{"Bash grid job show", "Bash", func(j *jobFixture) map[string]any {
			return map[string]any{"command": "grid job show " + j.task + " J-1"}
		}, "silence", "silence"},
		{"Bash grid job result", "Bash", func(j *jobFixture) map[string]any {
			return map[string]any{"command": "grid job result --task " + j.task + " --job J-1 --body '{}'"}
		}, "silence", "silence"},
		{"Bash grid brain search", "Bash", bash("grid brain search deploy"), "silence", "silence"},
		{"Bash git status", "Bash", bash("git status"), "silence", "silence"},
		{"Write in repo", "Write", func(j *jobFixture) map[string]any {
			return map[string]any{"file_path": filepath.Join(j.repo, "main.go"), "content": "package main\n"}
		}, "silence", "silence"},
		{"Edit in repo", "Edit", func(j *jobFixture) map[string]any {
			return map[string]any{"file_path": filepath.Join(j.repo, "main.go"), "old_string": "a", "new_string": "b"}
		}, "silence", "silence"},
		{"Write brain-proposals/x.md", "Write", func(j *jobFixture) map[string]any {
			return map[string]any{"file_path": task(j, "brain-proposals/x.md"), "content": "x"}
		}, "allow", "allow"},
		{"Write plan.md", "Write", func(j *jobFixture) map[string]any {
			return map[string]any{"file_path": task(j, "plan.md"), "content": "x"}
		}, "allow", "allow"},
	}
	for _, event := range []string{"PreToolUse", "PermissionRequest"} {
		for _, c := range calls {
			t.Run(event+"/"+c.name, func(t *testing.T) {
				j := jobSetup(t)
				got, reason := j.decide(t, event, c.tool, c.input(j))
				// The table in the lane report is made from these lines.
				t.Logf("TABLE | %s | %s | %s | %s", event, c.name, got, strings.ReplaceAll(reason, "\n", " "))
				want := c.pre
				if event == "PermissionRequest" {
					want = c.perm
				}
				if got != want {
					t.Fatalf("decision %q, want %q (reason: %s)", got, want, reason)
				}
			})
		}
	}
}

// A Worker that names its task by the folder name, not by the id in task.md,
// from a worktree outside any project with a record: main denies the Read, with
// a settings-resolution reason. This pins what main does; the app's GRID_TASK
// must therefore be the task.md id (or GRID_PROJECT must be set).
func TestJobAgentTaskNamedByFolderNeedsProject(t *testing.T) {
	for _, event := range []string{"PreToolUse", "PermissionRequest"} {
		t.Run(event, func(t *testing.T) {
			j := jobSetup(t)
			j.env["GRID_TASK"] = filepath.Base(j.dir)
			got, reason := j.decide(t, event, "Read", map[string]any{"file_path": filepath.Join(j.dir, "task.md")})
			t.Logf("TABLE | %s | Read task.md, GRID_TASK=folder name | %s | %s", event, got, reason)
			if got != "deny" {
				t.Fatalf("decision %q, want deny (reason: %s)", got, reason)
			}
		})
	}
}
