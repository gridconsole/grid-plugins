package guard

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/plan"
)

func TestPlanStages(t *testing.T) {
	cases := []struct{ name, text, history, want string }{
		{"missing", "", "", "before"},
		{"accepted despite missing", "", accepted, ""},
		{"no frontmatter", "broken", "", "unreadable"},
		{"before", strings.Replace(pendingTask, "step: plan\n", "step: research\n", 1), "", "before"},
		{"at gate", pendingTask, "", "gate"},
		{"past gate", strings.Replace(pendingTask, "step: plan\n", "step: build\n", 1), "", "past"},
		{"unknown step", strings.Replace(pendingTask, "step: plan\n", "step: unknown\n", 1), "", "past"},
		{"no step", strings.Replace(pendingTask, "step: plan\n", "", 1), "", "unreadable"},
		{"no approval", strings.ReplaceAll(pendingTask, "kind: approve", "kind: work"), "", ""},
		{"odd key", strings.Replace(pendingTask, "status:", "constructor:", 1), "", "unreadable"},
		{"odd nested key", strings.Replace(pendingTask, "kind: approve", "kind: approve, prototype: odd", 1), "", "unreadable"},
		{"scalar item", strings.Replace(pendingTask, "{step: plan, kind: approve}", "plan", 1), "", "unreadable"},
		{"duplicate step", strings.Replace(pendingTask, "step: build", "step: plan", 1), "", "unreadable"},
		{"uppercase step", strings.Replace(pendingTask, "step: build", "step: Build", 1), "", "unreadable"},
		{"missing kind", strings.Replace(pendingTask, ", kind: approve", "", 1), "", "unreadable"},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			if got := planStage(tt.text, tt.history); got != tt.want {
				t.Fatalf("got %q, want %q", got, tt.want)
			}
			f := setup(t)
			f.write(filepath.Join(f.task, "task.md"), tt.text)
			f.write(filepath.Join(f.task, "history.md"), tt.history)
			i := f.input("Write", filepath.Join(f.repo, "file.go"))
			want := ""
			if heldStage(tt.want) {
				want = "deny"
			}
			d := EvaluatePlan(i, f.root, f.env)
			expect(t, d, want)
			checkAdapter(t, PlanGuard, i, f.env, d)
		})
	}
}
func TestRecallBash(t *testing.T) {
	cases := []struct {
		command string
		deny    bool
	}{
		{`node plan-hash.mjs plan.md`, true},
		{`node plan-hash.mjs 'plan.md'`, true},
		{`node plan-hash.mjs "plan.md"`, true},
		{`node plan-hash.mjs plan.md --print`, false},
		{`node plan-hash.mjs other.md`, false},
		{`echo '· plan ready ·' >> history.md`, true},
		{`echo '· plan ready ·' | tee history.md`, true},
		{`echo '· plan ready ·' | tee hist*.md`, true},
		{`grep 'plan ready' history.md`, false},
		{`grep 'plan ready' history.md | tee notes.md`, false},
		{"grep 'plan ready' history.md | tee \"$OUTPUT\"", true},
		{`sed -i 's/old/plan ready/' history.md`, true},
		{`perl -pi -e 's/old/plan ready/' history.md`, true},
		{`python3 -c "open('history.md', 'a').write('plan ready')"`, true},
		{`echo ordinary >> history.md`, false},
	}
	for _, tt := range cases {
		for _, recall := range []bool{false, true} {
			t.Run(tt.command+map[bool]string{false: "/missing", true: "/recall"}[recall], func(t *testing.T) {
				f := setup(t)
				if recall {
					f.write(filepath.Join(f.task, "plan.md"), "- **Used:** knowledge/note.md\n")
				}
				i := f.input("Bash", tt.command)
				want := ""
				if tt.deny && !recall {
					want = "deny"
				}
				d := EvaluatePlan(i, f.root, f.env)
				expect(t, d, want)
				checkAdapter(t, PlanGuard, i, f.env, d)
			})
		}
	}
}
func TestEdits(t *testing.T) {
	cases := []struct {
		name, tool, text string
		input            map[string]any
		deny             bool
	}{
		{"literal safe", "Edit", "ordinary", map[string]any{"old_string": "ordinary", "new_string": "updated"}, false},
		{"new acceptance", "Edit", "ordinary", map[string]any{"old_string": "ordinary", "new_string": accepted}, true},
		{"CRLF reading", "Edit", "first\r\nsecond", map[string]any{"old_string": "first\nsecond", "new_string": accepted}, true},
		{"replacement reuses marker", "Edit", accepted + "tail", map[string]any{"old_string": "tail", "new_string": "$`"}, true},
		{"replacement match", "Edit", accepted, map[string]any{"old_string": accepted, "new_string": "$&"}, false},
		{"duplicate marker", "Edit", accepted, map[string]any{"old_string": accepted, "new_string": "$&$&"}, true},
		{"literal dollar", "Edit", "ordinary", map[string]any{"old_string": "ordinary", "new_string": "$$"}, false},
		{"bad replace_all", "Edit", "ordinary", map[string]any{"old_string": "ordinary", "new_string": "text", "replace_all": "yes"}, true},
		{"empty edit existing", "Edit", "ordinary", map[string]any{"old_string": "", "new_string": "text"}, true},
		{"empty edit new", "Edit", "", map[string]any{"old_string": "", "new_string": "text"}, false},
		{"sequential multi", "MultiEdit", "ordinary", map[string]any{"edits": []any{map[string]any{"old_string": "ordinary", "new_string": "changed"}, map[string]any{"old_string": "changed", "new_string": accepted}}}, true},
		{"safe multi", "MultiEdit", "ordinary", map[string]any{"edits": []any{map[string]any{"old_string": "ordinary", "new_string": "changed"}}}, false},
		{"empty multi", "MultiEdit", "ordinary", map[string]any{"edits": []any{}}, true},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			f := setup(t)
			// A marker in detail position does not release the held task.
			text := strings.ReplaceAll(tt.text, accepted, "detail "+accepted)
			f.write(filepath.Join(f.task, "history.md"), text)
			i := f.input(tt.tool, filepath.Join(f.task, "history.md"))
			for k, v := range tt.input {
				i.ToolInput[k] = v
			}
			if old, ok := i.ToolInput["old_string"].(string); ok {
				i.ToolInput["old_string"] = strings.ReplaceAll(old, accepted, "detail "+accepted)
			}
			want := ""
			if tt.deny {
				want = "deny"
			}
			d := EvaluatePlan(i, f.root, f.env)
			expect(t, d, want)
			checkAdapter(t, PlanGuard, i, f.env, d)
		})
	}
}
func TestRecallLineForms(t *testing.T) {
	for _, tt := range []struct {
		text string
		want bool
	}{
		{"Recall: nothing found", true}, {" Used: note", true}, {"- **Used:** note", true}, {"__Recall__: nothing found", true}, {"**Recall:** nothing found", true}, {"recall: Nothing Found", true},
		{"Recall: none", false}, {"    Used: note", false}, {"```\nUsed: note\n```", false}, {"<!-- Used: note -->", false},
		{"Recall:", false}, {"Recall: **", false}, {"prefix Recall: none", false}, {"Used: ___", false},
	} {
		if got := plan.HasRecall(tt.text); got != tt.want {
			t.Errorf("%q: %v", tt.text, got)
		}
	}
}

func TestExplicitFilesystemBase(t *testing.T) {
	f := setup(t)
	f.env["GRID_PROJECTS"] = "home/.grid/projects"
	f.env["CLAUDE_PLUGIN_DATA"] = "data"
	f.env["PWD"] = f.root
	i := f.input("Write", "file.go")
	i.Cwd = "repo"
	// The pointer is relative to the explicit filesystem base, not the process cwd.
	f.write(filepath.Join(f.root, "data", "sessions", "session.tasks"), f.task+"\n")
	d := EvaluatePlan(i, f.root, f.env)
	expect(t, d, "deny")
}
