package guard

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/hookio"
)

// Every verb that writes a gate or turns a guard off, with the arguments a real call has.
var gateVerbs = []string{
	"approve T-01", "answer T-01 Q-1 yes",
	"feed post --type answer --re X --body -", "feed post --type=answer --re X --body -", "feed post -type answer --re X --body -",
	"job post J-1 --attempt 1 --type answer --re X --body -", "job post J-1 --type=answer --re X --body -", "job post -type answer --re X --body -",
	"question answer T-01 Q-1 yes", "question withdraw T-01 Q-1",
	"refusal decide T-01 R-1 allow",
	"task status T-01 closed", "task close T-01", "task reopen T-01", "task stop T-01",
}
var switchVerbs = []string{
	"hook level off", "agent set builder sonnet", "skill enable brain-keeper", "skill disable brain-keeper",
	"setup", "setup demo", "setup codex", "setup --remove",
}

// Each form runs the verb behind a different shell construct.
var bypassForms = []string{
	"grid %s", "GRID_HOME=/x grid %s", "FOO=1 BAR=2 grid %s", "cd /tmp && grid %s", "true; grid %s", "true || grid %s",
	"(grid %s)", "( grid %s )", "$(grid %s)", "echo `grid %s`", "{ grid %s; }", "echo x | grid %s", "true &\ngrid %s",
	"/usr/local/bin/grid %s", "'/usr/local/bin/grid' %s", `"grid" %s`, "gr''id %s", `g\rid %s`, `"/opt/gr"id %s`,
	"env -i grid %s", "command grid %s", "sh -c 'grid %s'", `bash -c "grid %s"`, "eval 'grid %s'",
	"grid   %s", "grid\t%s", "grid \\\n%s", "Grid %s", "GRID %s", "./dist/grid %s", "go run ./cmd/grid %s", `C:\bin\grid.exe %s`,
	"~/.local/bin/grid %s", "nohup grid %s &",
}

func TestHostDeniesGateAndSwitchVerbsInEveryForm(t *testing.T) {
	f := setup(t)
	for _, verb := range append(append([]string{}, gateVerbs...), switchVerbs...) {
		for _, form := range bypassForms {
			command := fmt.Sprintf(form, verb)
			t.Run(command, func(t *testing.T) {
				d := EvaluateHost(f.input("Bash", command), f.root, f.env)
				expect(t, d, "deny")
				if !strings.Contains(d.Reason, "Reading is allowed") {
					t.Fatalf("deny text does not say reading is allowed: %s", d.Reason)
				}
			})
		}
	}
}

func TestHostDeniesQuotedVerbWords(t *testing.T) {
	f := setup(t)
	for _, command := range []string{
		`grid "approve" T-01`, `grid 'ap''prove' T-01`, `grid $'approve' T-01`, `grid "question" 'answer' T-01 Q-1 x`,
		`grid task "cl"ose T-01`, `grid  refusal   decide T-01 R-1 deny`, `grid "hook" "level" off`, `grid ski"ll" enable x`,
	} {
		t.Run(command, func(t *testing.T) { expect(t, EvaluateHost(f.input("Bash", command), f.root, f.env), "deny") })
	}
}

func TestHostAllowsOtherGridVerbsAndLookalikes(t *testing.T) {
	f := setup(t)
	for _, command := range []string{
		"grid job result J-01", "grid feed read", "grid task new 'a task'", "grid task show T-01", "grid task list --json",
		"grid question add T-01 'why?'", "grid question list T-01", "grid ask T-01 'why?'", "grid plan write T-01", "grid brain list",
		"grid hook plan-guard", "grid agent list", "grid skill list", "grid refusal list T-01", "grid doctor",
		"mygrid approve", "grid-approve T-01", "grid_answer T-01", "echo gridapprove", "approve T-01", "ls approve",
		"grid feed read --type answer", "grid feed post --type question --body '{}'", "grid feed post --type answers --body '{}'",
		"grid job post J-1 --attempt 1 --type checkpoint --body '{}'", "grid job post J-1 --type answers --body '{}'", "grid job result J-1 --type answer",
		"git commit -m 'fix the question text'", "cat README.md", "go test ./...",
	} {
		t.Run(command, func(t *testing.T) { expect(t, EvaluateHost(f.input("Bash", command), f.root, f.env), "") })
	}
}

func TestHostBashGatePaths(t *testing.T) {
	f := setup(t)
	taskPath := filepath.ToSlash(f.task)
	for _, tt := range []struct {
		command string
		deny    bool
	}{
		// Writes to the questions folder, the approval, a refusal decision and revision archives.
		{"echo yes > questions/Q-1.md", true}, {"echo yes >> questions/Q-1.answer.md", true}, {`printf x > "questions/Q-1.md"`, true},
		{"echo yes > 'questions'/Q-1.md", true}, {"echo yes > quest''ions/Q-1.md", true},
		{"echo yes > " + taskPath + "/questions/Q-1.md", true}, {"cd " + taskPath + " && echo y > questions/Q-2.md", true},
		{"echo yes | tee questions/Q-1.md", true}, {"cp /tmp/a questions/Q-1.md", true}, {"mv /tmp/a questions/", true},
		{"rm questions/Q-1.md", true}, {"rm -rf questions", true}, {"touch questions/Q-9.md", true}, {"sed -i s/a/b/ questions/Q-1.md", true},
		{"cat questions/Q-1.md > questions/Q-2.md", true}, {"find questions -delete", true}, {"find questions -exec rm {} +", true},
		{"python3 -c \"open('questions/Q-1.md','w').write('x')\"", true}, {"ls questions; rm questions/Q-1.md", true},
		{"echo '{}' > approval.json", true}, {"echo '{}' > " + taskPath + "/approval.json", true}, {"cp /tmp/a APPROVAL.JSON", true},
		{"echo '{}' > .task-write.json", true}, {"rm .task.lock", true}, {"touch " + taskPath + "/.task.lock", true},
		{"echo '{}' > refusal-R-1.json", true}, {"rm " + taskPath + "/refusal-R-12.json", true},
		{"echo x > plan-revisions/3.md", true}, {"mv /tmp/a plan-revisions/1.md", true}, {"rm -r plan-revisions", true},
		// Reads stay allowed.
		{"cat questions/Q-1.md", false}, {"cat " + taskPath + "/questions/Q-1.answer.md", false}, {"grep -rn answer questions/", false},
		{"grep -n x questions/*.md | head -5", false}, {"ls questions", false}, {"ls -la " + taskPath + "/questions/", false},
		{"head -20 questions/Q-1.md", false}, {"tail -f questions/Q-1.md", false}, {"wc -l questions/Q-1.md", false},
		{"cat .task-write.json", false}, {"ls -la .task.lock", false}, {"cat approval.json", false}, {"jq . approval.json", false}, {"cat " + taskPath + "/refusal-R-1.json", false},
		{"ls plan-revisions", false}, {"cat plan-revisions/2.md", false}, {"diff plan.md plan-revisions/2.md", false},
		{"find questions -name '*.md'", false}, {"cat questions/Q-1.md | grep -i yes", false}, {"cd questions && ls", false},
		{"FOO=1 cat questions/Q-1.md", false}, {"echo questions", false}, {"stat questions", false},
		// Other names are not gates.
		{"echo x > questions.md", false}, {"echo x > question/Q-1.md", false}, {"echo x > approval.json.bak", false},
		{"echo x > my-approval.json", false}, {"echo x > .task.lock.bak", false}, {"echo x > refusal-R-0.json", false}, {"echo x > plan-revisions-notes.md", false},
		{"echo x > docs/questioning/a.md", false}, {"echo x > plan.md", false},
	} {
		t.Run(tt.command, func(t *testing.T) {
			want := ""
			if tt.deny {
				want = "deny"
			}
			expect(t, EvaluateHost(f.input("Bash", tt.command), f.root, f.env), want)
		})
	}
}

func TestHostBashSettingsPaths(t *testing.T) {
	f := setup(t)
	for _, tt := range []struct {
		command string
		deny    bool
	}{
		{`echo '{"hooks":{"host-guard":"off"}}' > ~/.grid/settings.json`, true},
		{`echo '{"hooks":{"host-guard":"off"}}' > ` + filepath.ToSlash(filepath.Join(f.grid, "settings.json")), true},
		{`echo '{"hooks":{"plan-guard":"off"}}' > $GRID_HOME/settings.json`, true},
		{`echo '{}' | tee projects/example/settings.json`, true}, {"cp /tmp/a " + filepath.ToSlash(filepath.Join(f.projects, "example", "settings.json")), true},
		{"sed -i s/on/off/ settings.json", true}, {"rm .settings.lock", true}, {"touch .settings.lock", true}, {"echo 1 > .settings.lock", true},
		{"mv /tmp/s SETTINGS.JSON", true}, {"echo x > 'settings'.json", true}, {"cd ~/.grid && echo '{}' > settings.json", true},
		{"cat ~/.grid/settings.json", false}, {"grep host-guard settings.json", false}, {"ls -la .settings.lock", false},
		{"jq .hooks projects/example/settings.json", false},
		{"echo x > my-settings.json", false}, {"echo x > settings.json.bak", false}, {"echo x > settings.jsonl", false}, {"echo x > app-settings.lock", false},
	} {
		t.Run(tt.command, func(t *testing.T) {
			d := EvaluateHost(f.input("Bash", tt.command), f.root, f.env)
			want := ""
			if tt.deny {
				want = "deny"
				if !strings.Contains(d.Reason, "cannot turn a guard off") {
					t.Fatalf("settings deny text: %s", d.Reason)
				}
			}
			expect(t, d, want)
		})
	}
}

func TestHostFileToolsGatePaths(t *testing.T) {
	f := setup(t)
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
				if !strings.Contains(d.Reason, "Reading is allowed") {
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
	f := setup(t)
	f.env["GRID_PLAN_GUARD"] = "off"
	for _, rel := range []string{
		"settings.json", "SETTINGS.JSON", ".settings.lock", ".Settings.Lock",
		"projects/example/settings.json", "projects/example/.settings.lock", "projects/Example/SETTINGS.json", "projects/other/settings.json",
	} {
		t.Run("Write/"+rel, func(t *testing.T) {
			d := EvaluateHost(f.input("Write", filepath.Join(f.grid, rel)), f.root, f.env)
			expect(t, d, "deny")
			if !strings.Contains(d.Reason, "cannot turn a guard off") {
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
	f := setup(t)
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
	t.Run("Bash", func(t *testing.T) {
		for _, command := range []string{
			"echo '" + off + "' > " + filepath.Join(f.grid, "settings.json"),
			"printf '%s' '" + off + "' > ~/.grid/settings.json",
			"cat > $GRID_HOME/settings.json <<'EOF'\n" + off + "\nEOF",
		} {
			i := f.input("Bash", command)
			i.Cwd = f.repo
			expect(t, hookio.Strongest(EvaluateHost(i, f.root, f.env), EvaluateHome(i, f.root, f.env)), "deny")
		}
	})
}

// Without a Grid home or projects folder the shell checks stay off, as for the other host rules.
func TestHostGateShellChecksNeedAGridFolder(t *testing.T) {
	f := setup(t)
	delete(f.env, "GRID_PROJECTS")
	delete(f.env, "GRID_HOME")
	for _, command := range []string{"grid approve T-01", "echo x > questions/Q-1.md", "grid hook level off", "echo x > settings.json"} {
		expect(t, EvaluateHost(f.input("Bash", command), f.root, f.env), "")
	}
}

// An answer post closes a person gate, so every argument order and quoting of
// grid feed post and grid job post with --type answer is refused like grid answer.
func TestHostDeniesFeedAnswerPosts(t *testing.T) {
	f := setup(t)
	for _, command := range []string{
		`grid feed post --type answer --re 01ARZ3NDEKTSV4RRFFQ69G5FAV --body '{"answer":"Yes"}'`,
		`grid feed post --body '{"answer":"Yes"}' --re 01ARZ3NDEKTSV4RRFFQ69G5FAV --type answer`,
		`grid feed post T-01 --type answer --body '{"answer":"Yes"}'`,
		`grid feed post --body '{"answer":"a;b"}' --type=answer`,
		`grid feed post --type "answer" --body -`, `grid feed post --type 'an''swer' --body -`, `grid feed post --TYPE ANSWER --body -`,
		`GRID_ROLE=owner grid feed post --type answer --body -`, `env GRID_ROLE=owner grid feed post --type answer --body -`,
		`grid job post J-1 --attempt 1 --type answer --re 01ARZ3NDEKTSV4RRFFQ69G5FAV --body '{"answer":"Yes"}'`,
		`grid job post --body '{"answer":"Yes"}' --job J-1 --type=answer`, `grid job post J-1 --type "answer" --body -`, `grid job post J-1 --TYPE ANSWER --body -`,
		`GRID_ROLE=owner grid job post J-1 --type answer --body -`, `env GRID_ROLE=owner grid job post J-1 --type answer --body -`,
	} {
		t.Run(command, func(t *testing.T) {
			d := EvaluateHost(f.input("Bash", command), f.root, f.env)
			expect(t, d, "deny")
			if d.Reason != gateReason {
				t.Fatalf("feed answer refusal differs from grid answer: %s", d.Reason)
			}
		})
	}
}

// The CLI writes as member when GRID_ROLE is unset, which a person gate admits,
// so the guard must refuse the post itself in a session without GRID_ROLE.
func TestHostDeniesFeedAnswerWithoutRole(t *testing.T) {
	f := setup(t)
	env := map[string]string{}
	for k, v := range f.env {
		if k != "GRID_ROLE" && k != "GRID_MEMBER" {
			env[k] = v
		}
	}
	for _, command := range []string{
		`grid feed post --type answer --re 01ARZ3NDEKTSV4RRFFQ69G5FAV --body '{"answer":"Yes"}'`,
		`grid job post J-1 --attempt 1 --type answer --re 01ARZ3NDEKTSV4RRFFQ69G5FAV --body '{"answer":"Yes"}'`,
	} {
		expect(t, EvaluateHost(f.input("Bash", command), f.root, env), "deny")
	}
	expect(t, EvaluateHost(f.input("Bash", "grid answer T-01 Q-1 Yes"), f.root, env), "deny")
}
