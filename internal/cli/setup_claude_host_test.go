package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/guard"
	"github.com/gridconsole/grid-plugins/internal/hookio"
	"github.com/gridconsole/grid-plugins/internal/settings"
)

var foldedLetter = regexp.MustCompile(`\[([a-z])[A-Z]\]`)

// hostDeny reports a rule written for the files EvaluateHost refuses to the file
// tools (settings, workflows, the host folder), as opposed to the writer guard's.
func hostDeny(rule string) bool {
	plain := foldedLetter.ReplaceAllString(rule, "$1")
	for _, suffix := range []string{"/settings.json)", "/.settings.lock)", "/workflows.md)", "/.host/**)"} {
		if strings.HasSuffix(plain, suffix) {
			return true
		}
	}
	return false
}

func claudeSettingsDoc(t *testing.T) map[string]json.RawMessage {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(os.Getenv("CLAUDE_CONFIG_DIR"), "settings.json"))
	if err != nil {
		t.Fatal(err)
	}
	doc := map[string]json.RawMessage{}
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatal(err)
	}
	return doc
}

func claudeDenyRules(t *testing.T) []string {
	t.Helper()
	var permissions struct{ Deny []string }
	if err := json.Unmarshal(claudeSettingsDoc(t)["permissions"], &permissions); err != nil {
		t.Fatal(err)
	}
	return permissions.Deny
}

// claudeRuleMatches applies one Edit(...) rule to an absolute path the way the
// generated patterns are defined: ~/ is the user home, // is filesystem-absolute.
func claudeRuleMatches(t *testing.T, home, rule, target string) bool {
	t.Helper()
	pattern := strings.TrimSuffix(strings.TrimPrefix(rule, "Edit("), ")")
	switch {
	case strings.HasPrefix(pattern, "~/"):
		rel, err := filepath.Rel(home, target)
		if err != nil {
			t.Fatal(err)
		}
		return claudePatternMatches(t, strings.TrimPrefix(pattern, "~/"), filepath.ToSlash(rel))
	case strings.HasPrefix(pattern, "//"):
		return claudePatternMatches(t, strings.TrimPrefix(pattern, "//"), strings.TrimLeft(filepath.ToSlash(target), "/"))
	}
	t.Fatalf("unexpected path anchor %s", pattern)
	return false
}

// The deny rules cover every file EvaluateHost refuses to the file tools in the
// places setup covers, and the task-folder files the rules alone carry. The
// host-guard level is pinned to block, so the test holds whatever its default is.
// Every host rule must be the only rule that matches at least one path.
func TestSetupWritesDenyRulesForTheHostGuardFiles(t *testing.T) {
	for _, location := range []string{"default", "outside home"} {
		t.Run(location, func(t *testing.T) {
			h := claudeHarness(t)
			projects := filepath.Join(h.gridHome, "projects")
			if location == "outside home" {
				projects = filepath.Join(h.root, "work")
			}
			t.Setenv("GRID_PROJECTS", projects)
			store := settings.Store{Home: h.gridHome}
			if err := store.SetHook("host-guard", "block"); err != nil {
				t.Fatal(err)
			}
			if code := h.run(h.home, "setup", "--yes"); code != 0 {
				t.Fatalf("code=%d: %s", code, h.errb)
			}
			var edit []string
			for _, rule := range claudeDenyRules(t) {
				if strings.HasPrefix(rule, "Edit(") {
					edit = append(edit, rule)
				}
			}
			if !slices.ContainsFunc(edit, hostDeny) {
				t.Error("no host rules written")
			}
			env := map[string]string{"HOME": h.home, "GRID_HOME": h.gridHome, "GRID_PROJECTS": projects}

			type target struct {
				path, label string
				want        bool // refused by EvaluateHost, or a task-folder file the deny rules carry
			}
			var targets []target
			taskFolder := map[string]bool{} // paths under a task folder, which only the deny rules refuse
			seen := map[string]bool{}
			add := func(label, path string, want bool) {
				if !seen[path] {
					seen[path] = true
					targets = append(targets, target{path, label, want})
				}
			}
			// Three file names at the five places, in lower and upper case. In the
			// default layout <projects> is <home>/projects, so two places coincide
			// with others and add() drops the repeats.
			places := []struct{ place, dir string }{
				{"home", h.gridHome},
				{"one below home", filepath.Join(h.gridHome, "notes")},
				{"home/projects/p", filepath.Join(h.gridHome, "projects", "p")},
				{"projects", projects},
				{"one below projects", filepath.Join(projects, "p")},
			}
			for _, p := range places {
				for _, name := range []string{"settings.json", ".settings.lock", "workflows.md"} {
					add(p.place+" "+name, filepath.Join(p.dir, name), true)
					add(p.place+" "+strings.ToUpper(name), filepath.Join(p.dir, strings.ToUpper(name)), true)
				}
			}
			add("mixed case", filepath.Join(h.gridHome, "Settings.Json"), true)
			add("PROJECTS folder", filepath.Join(h.gridHome, "PROJECTS", "p", "Workflows.md"), true)
			// The host folder in the Grid home and beside the projects folder (the
			// same folder in the default layout).
			for _, host := range []string{h.gridHome, filepath.Dir(projects)} {
				add("host "+host, filepath.Join(host, ".host", "hold.json"), true)
				add("HOST "+host, filepath.Join(host, ".HOST", "Hold.json"), true)
				add("host deep "+host, filepath.Join(host, ".host", "approval", "x.json"), true)
			}
			// Task files the task-folder rules already cover.
			for _, task := range []string{filepath.Join(projects, "p", "tasks", "T-01-work"), filepath.Join(h.gridHome, "personal", "tasks", "T-2")} {
				for _, rel := range []string{"approval.json", "approval-ship.json", "approval-preview.json", "feed/0001.json", "questions/q1.md", "workflow.md", "workflows.md", "task.md", "refusal-R-1.json"} {
					add("task "+rel, filepath.Join(task, filepath.FromSlash(rel)), true)
					taskFolder[filepath.Join(task, filepath.FromSlash(rel))] = true
				}
			}
			// Paths neither guard refuses.
			for label, path := range map[string]string{
				"settings.json.bak":    filepath.Join(projects, "p", "settings.json.bak"),
				"notes/settings.json":  filepath.Join(projects, "p", "notes", "settings.json"),
				"nested workflows.md":  filepath.Join(projects, "p", "nested", "deep", "workflows.md"),
				"other/deep/settings":  filepath.Join(h.gridHome, "other", "deep", "settings.json"),
				"other/deep/workflows": filepath.Join(h.gridHome, "other", "deep", "workflows.md"),
				"home/notes/x.md":      filepath.Join(h.gridHome, "notes", "x.md"),
				"project notes":        filepath.Join(projects, "p", "notes.md"),
				"tasks/README.md":      filepath.Join(projects, "p", "tasks", "README.md"),
				"settings.jsonx":       filepath.Join(h.gridHome, "settings.jsonx"),
				"deep lock":            filepath.Join(projects, "p", "nested", ".settings.lock"),
				"host-like name":       filepath.Join(h.gridHome, "host", "x"),
				"dot host file":        filepath.Join(h.gridHome, ".hostile"),
			} {
				add("allowed "+label, path, false)
			}
			if len(targets) < 30 {
				t.Fatalf("only %d paths", len(targets))
			}

			hits := map[string]bool{} // host rules that are the only match for some path
			for _, tg := range targets {
				for _, tool := range []string{"Edit", "Write"} {
					in := hookio.Input{Cwd: h.home, ToolName: tool, ToolInput: map[string]any{"file_path": tg.path}}
					// A task-folder file is refused by the deny rules alone (no Grid guard
					// judges it any more); the rule check below proves it.
					refused := guard.EvaluateHost(in, h.home, env).Behavior == "deny"
					if !taskFolder[tg.path] && refused != tg.want {
						t.Fatalf("%s %s: table says refused=%v, the host guard says %v", tool, tg.label, tg.want, refused)
					}
				}
				var matching []string
				for _, rule := range edit {
					if claudeRuleMatches(t, h.home, rule, tg.path) {
						matching = append(matching, rule)
					}
				}
				if (len(matching) > 0) != tg.want {
					t.Errorf("%s: deny rules match=%v want %v (%v)", tg.label, len(matching) > 0, tg.want, matching)
				}
				if len(matching) == 1 && hostDeny(matching[0]) {
					hits[matching[0]] = true
				}
			}
			for _, rule := range edit {
				if hostDeny(rule) && !hits[rule] {
					t.Errorf("host rule that no path needs: %s", rule)
				}
			}
		})
	}
}

func TestSetupSandboxFlag(t *testing.T) {
	h := claudeHarness(t)
	file := filepath.Join(os.Getenv("CLAUDE_CONFIG_DIR"), "settings.json")
	run := func(want int, args ...string) {
		t.Helper()
		if code := h.run(h.home, args...); code != want {
			t.Fatalf("%v: code=%d want %d: %s%s", args, code, want, h.out, h.errb)
		}
	}
	sandbox := func() (string, bool) {
		raw, ok := claudeSettingsDoc(t)["sandbox"]
		return string(raw), ok
	}
	compact := func(raw string) string {
		var v any
		if err := json.Unmarshal([]byte(raw), &v); err != nil {
			t.Fatal(err)
		}
		b, _ := json.Marshal(v)
		return string(b)
	}

	// Without the flag the sandbox is not written, and the deny rules are.
	run(0, "setup", "--yes")
	if raw, ok := sandbox(); ok {
		t.Fatalf("plain setup wrote sandbox: %s", raw)
	}
	if len(claudeDenyRules(t)) == 0 {
		t.Fatal("plain setup wrote no deny rules")
	}
	// With it, exactly sandbox.enabled is written; a second run changes nothing.
	run(0, "setup", "--sandbox", "--yes")
	if raw, ok := sandbox(); !ok || compact(raw) != `{"enabled":true}` {
		t.Fatalf("sandbox=%s", raw)
	}
	before, _ := os.ReadFile(file)
	run(0, "setup", "--sandbox", "--yes", "--json")
	var result struct {
		Present []string
		Written bool
	}
	if err := json.Unmarshal(h.out.Bytes(), &result); err != nil || result.Written || !slices.Contains(result.Present, "sandbox.enabled: true") {
		t.Fatalf("second run: %s %v", h.out, err)
	}
	after, _ := os.ReadFile(file)
	if string(before) != string(after) {
		t.Fatal("second --sandbox run wrote settings")
	}
	// A plain setup afterwards leaves it on. --sandbox with --remove is a usage
	// error and changes nothing; plain --remove takes the sandbox out with the rest.
	run(0, "setup", "--yes")
	if _, ok := sandbox(); !ok {
		t.Fatal("plain setup removed the sandbox")
	}
	run(2, "setup", "--sandbox", "--remove", "--yes")
	if again, _ := os.ReadFile(file); string(again) != string(after) {
		t.Fatal("usage error changed the settings file")
	}
	run(0, "setup", "--remove", "--yes")
	if raw, ok := sandbox(); ok {
		t.Fatalf("remove left sandbox: %s", raw)
	}
	if rules := claudeDenyRules(t); len(rules) != 0 {
		t.Fatalf("remove left deny rules: %v", rules)
	}

	// Setup owns only the key it wrote. The person's other sandbox keys stay.
	h.write(file, `{"sandbox":{"enabled":false,"excludedCommands":["docker"]}}`)
	run(0, "setup", "--sandbox", "--yes")
	if raw, _ := sandbox(); compact(raw) != `{"enabled":true,"excludedCommands":["docker"]}` {
		t.Fatalf("sandbox=%s", raw)
	}
	run(0, "setup", "--remove", "--yes")
	if raw, _ := sandbox(); compact(raw) != `{"excludedCommands":["docker"]}` {
		t.Fatalf("after remove sandbox=%s", raw)
	}
	// A sandbox the person had already switched on is not ours to switch off,
	// whether setup ran before or after they switched it on.
	h.write(file, `{"sandbox":{"enabled":true}}`)
	run(0, "setup", "--yes")
	run(0, "setup", "--sandbox", "--yes")
	run(0, "setup", "--remove", "--yes")
	if raw, _ := sandbox(); compact(raw) != `{"enabled":true}` {
		t.Fatalf("remove touched a sandbox setup did not write: %s", raw)
	}

	// A malformed sandbox entry is refused only when the flag asks for it.
	h.write(file, `{"sandbox":{"enabled":"yes"}}`)
	run(2, "setup", "--sandbox", "--yes")
	if data, _ := os.ReadFile(file); string(data) != `{"sandbox":{"enabled":"yes"}}` {
		t.Fatalf("invalid file changed: %s", data)
	}
	run(0, "setup", "--yes")
	if raw, _ := sandbox(); compact(raw) != `{"enabled":"yes"}` {
		t.Fatalf("plain setup touched the malformed sandbox: %s", raw)
	}
}
