package guard

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/hookio"
)

func TestHostFileToolsGatePaths(t *testing.T) {
	f := hostSetup(t)
	f.env["GRID_PLAN_GUARD"] = "off"
	personal := filepath.Join(f.grid, "personal", "tasks", "T-02-mine")
	for _, task := range []string{f.task, personal} {
		for _, rel := range []string{
			"questions", "questions/Q-1.md", "questions/Q-1.answer.md", "questions/.lock", "QUESTIONS/Q-2.md", "questions/nested/x.md",
			"approval.json", "APPROVAL.JSON", "refusal-R-1.json", "refusal-R-27.json", "Refusal-r-3.JSON",
			".task-write.json", ".task.lock", ".TASK.LOCK",
			"plan-revisions", "plan-revisions/1.md", "PLAN-REVISIONS/2.md",
		} {
			t.Run("Write/"+filepath.Base(task)+"/"+rel, func(t *testing.T) {
				d := EvaluateHost(f.input("Write", filepath.Join(task, rel)), f.root, f.env)
				expect(t, d, "deny")
				if !strings.Contains(d.Reason, "is a task record that grid writes; use the grid task or grid question command") {
					t.Fatalf("deny text: %s", d.Reason)
				}
			})
		}
		// Every file tool is fenced, and the adapter prints the same decision.
		for _, tool := range []string{"Write", "Edit", "MultiEdit", "NotebookEdit"} {
			i := f.input(tool, filepath.Join(task, "questions", "Q-1.md"))
			d := EvaluateHost(i, f.root, f.env)
			expect(t, d, "deny")
			checkAdapter(t, HostGuard, i, f.env, d)
		}
		for _, rel := range []string{
			"plan.md", "intent.md", "questions.md", "question/Q-1.md", "approval.json.bak", "my-approval.json", "refusal-R-0.json", "refusal-R-1.json.bak",
			"plan-revisions.md", ".task.lock.bak", "my.task.lock", "artifacts/.task-write.json", "artifacts/approval.json", "artifacts/questions/Q-1.md", "notes/plan-revisions/1.md",
		} {
			t.Run("allow/"+filepath.Base(task)+"/"+rel, func(t *testing.T) {
				expect(t, EvaluateHost(f.input("Write", filepath.Join(task, rel)), f.root, f.env), "")
			})
		}
	}
	for _, p := range []string{filepath.Join(f.repo, "questions", "Q-1.md"), filepath.Join(f.repo, "approval.json")} {
		t.Run("outside/"+p, func(t *testing.T) { expect(t, EvaluateHost(f.input("Write", p), f.root, f.env), "") })
	}
}

func TestHostFileToolsSettingsPaths(t *testing.T) {
	f := hostSetup(t)
	f.env["GRID_PLAN_GUARD"] = "off"
	for _, rel := range []string{
		"settings.json", "SETTINGS.JSON", ".settings.lock", ".Settings.Lock",
		"projects/example/settings.json", "projects/example/.settings.lock", "projects/Example/SETTINGS.json", "projects/other/settings.json",
	} {
		t.Run("Write/"+rel, func(t *testing.T) {
			d := EvaluateHost(f.input("Write", filepath.Join(f.grid, rel)), f.root, f.env)
			expect(t, d, "deny")
			if !strings.Contains(d.Reason, "holds the person's Grid settings; grid hook level and grid settings change it") {
				t.Fatalf("deny text: %s", d.Reason)
			}
		})
	}
	for _, tool := range []string{"Write", "Edit", "MultiEdit", "NotebookEdit"} {
		i := f.input(tool, filepath.Join(f.grid, "settings.json"))
		d := EvaluateHost(i, f.root, f.env)
		expect(t, d, "deny")
		checkAdapter(t, HostGuard, i, f.env, d)
	}
	for _, p := range []string{
		filepath.Join(f.grid, "settings.json.bak"), filepath.Join(f.grid, "my-settings.json"), filepath.Join(f.grid, "notes.md"),
		filepath.Join(f.projects, "example", "knowledge", "settings.json"), filepath.Join(f.task, "settings.json"),
		filepath.Join(f.projects, "example", "tasks", "settings.json"),
		filepath.Join(f.repo, "settings.json"), filepath.Join(f.repo, ".settings.lock"), filepath.Join(f.root, "settings.json"),
	} {
		t.Run("allow/"+p, func(t *testing.T) { expect(t, EvaluateHost(f.input("Write", p), f.root, f.env), "") })
	}
}

// The home fallback allows writes inside the Grid home. A settings file is in the
// Grid home, so the arbitration must keep the host guard's deny.
func TestAgentCannotWriteGuardOffIntoSettings(t *testing.T) {
	f := hostSetup(t)
	off := `{"hooks":{"host-guard":"off"}}`
	for _, file := range []string{"settings.json", ".settings.lock", "projects/example/settings.json"} {
		for _, tool := range []string{"Write", "Edit", "MultiEdit"} {
			t.Run(tool+"/"+file, func(t *testing.T) {
				i := f.input(tool, filepath.Join(f.grid, file))
				i.Cwd = f.repo
				i.ToolInput["content"], i.ToolInput["new_string"] = off, off
				if got := EvaluateHome(i, f.root, f.env); got.Behavior != "allow" {
					t.Fatalf("home fallback no longer allows %s (%q); the regression below proves nothing", file, got.Behavior)
				}
				expect(t, hookio.Strongest(EvaluateHost(i, f.root, f.env), EvaluateHome(i, f.root, f.env)), "deny")
			})
		}
	}
}

// A Bash command is never refused by Grid's reading of shell text: Claude Code's
// permission rules decide what a shell may write.
func TestHostNeverRefusesBash(t *testing.T) {
	f := hostSetup(t)
	for _, command := range []string{
		"grid approve T-01", "echo x > questions/Q-1.md", "grid hook level off", "echo x > settings.json",
		"echo '{}' > approval.json", "echo .host", "cat > $GRID_HOME/settings.json <<'EOF'\n{}\nEOF",
	} {
		t.Run(command, func(t *testing.T) { expect(t, EvaluateHost(f.input("Bash", command), f.root, f.env), "") })
	}
}
