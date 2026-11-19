package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/project"
)

func claudeHarness(t *testing.T) *harness {
	t.Helper()
	h := newHarness(t)
	t.Setenv("HOME", h.home)
	t.Setenv("GRID_HOME", h.gridHome)
	t.Setenv("GRID_PROJECTS", filepath.Join(h.gridHome, "projects"))
	t.Setenv("CLAUDE_CONFIG_DIR", filepath.Join(h.home, "custom-claude"))
	return h
}

func TestSetupClaudeCLI(t *testing.T) {
	h := claudeHarness(t)
	file := filepath.Join(os.Getenv("CLAUDE_CONFIG_DIR"), "settings.json")
	for _, args := range [][]string{{"setup"}, {"setup", "--json"}} {
		h.env.Stdin = strings.NewReader("yes\n")
		if code := h.run(h.home, args...); code != 0 {
			t.Fatalf("%d %s", code, h.errb)
		}
		if _, err := os.Stat(file); !os.IsNotExist(err) {
			t.Fatal("non-terminal wrote settings", err)
		}
	}
	if code := h.run(h.home, "setup", "--yes"); code != 0 {
		t.Fatalf("%d %s", code, h.errb)
	}
	golden, err := os.ReadFile(filepath.Join("..", "settings", "testdata", "claude", "fresh.json"))
	if err != nil {
		t.Fatal(err)
	}
	var output string
	if err := json.Unmarshal(golden, &output); err != nil {
		t.Fatal(err)
	}
	want := strings.TrimSuffix(output, "Write these? [y/N] ")
	if h.out.String() != want {
		t.Fatalf("got %s want %s", h.out, want)
	}
	before, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(h.home, ".claude", "settings.json")); !os.IsNotExist(err) {
		t.Fatal("ignored custom directory")
	}
	if code := h.run(h.home, "setup", "--yes", "--json"); code != 0 {
		t.Fatalf("%d %s", code, h.errb)
	}
	var result struct {
		Path           string
		Added, Present []string
		Written        bool
	}
	if err := json.Unmarshal(h.out.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.Path != file || result.Written || len(result.Added) != 0 || len(result.Present) == 0 {
		t.Fatal(h.out)
	}
	after, err := os.ReadFile(file)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("second setup wrote settings", err)
	}
	if code := h.run(h.home, "setup", "--remove", "--yes", "--json"); code != 0 {
		t.Fatalf("%d %s", code, h.errb)
	}
	if err := json.Unmarshal(h.out.Bytes(), &result); err != nil || !result.Written {
		t.Fatal(h.out, err)
	}
	if code := h.run(h.home, "setup", "--remove"); code != 0 || h.out.String() != "Nothing to remove.\n" {
		t.Fatalf("%d %s %s", code, h.out, h.errb)
	}
}

func TestSetupClaudeInvalidCLI(t *testing.T) {
	h := claudeHarness(t)
	file := filepath.Join(os.Getenv("CLAUDE_CONFIG_DIR"), "settings.json")
	h.write(file, "{broken")
	for _, args := range [][]string{{"setup", "--yes"}, {"setup", "--yes", "--json"}} {
		if code := h.run(h.home, args...); code != 2 {
			t.Fatalf("%d %s", code, h.errb)
		}
		want := "Settings file " + file + " is not valid JSON; fix it or remove it. Nothing written.\n"
		errorCode := ""
		if jsonRequested(args) {
			errorCode = CodeInvalidArgument
		}
		assertErrorOutput(t, h.out.String(), h.errb.String(), errorCode)
		if h.errb.String() != want {
			t.Fatalf("stdout=%s stderr=%s", h.out, h.errb)
		}
		b, err := os.ReadFile(file)
		if err != nil || string(b) != "{broken" {
			t.Fatal("invalid file changed", err)
		}
	}
}

func TestSetupClaudeHelpAndProjectAnswersGolden(t *testing.T) {
	h := claudeHarness(t)
	if code := h.run(h.home, "setup", "--help"); code != 0 {
		t.Fatalf("%d %s", code, h.errb)
	}
	want := "Usage: grid setup [--sandbox] [--remove] [--yes] [--json]\n       grid setup <project> [--answers <file|->] [--json]\n"
	if h.out.String() != want {
		t.Fatal(h.out)
	}
	h.record(project.Record{Name: "example"})
	answers := `{"test":"make test","model":"Sonnet","effort":"high"}`
	for _, args := range [][]string{
		{"setup", "example", "--answers", "-", "--json"},
		{"setup", "--json", "--answers", "-", "example"},
	} {
		h.env.Stdin = strings.NewReader(answers)
		if code := h.run(h.home, args...); code != 0 {
			t.Fatalf("%d %s", code, h.errb)
		}
		golden, err := os.ReadFile(filepath.Join("testdata", "settings", "setup.json"))
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(h.out.Bytes(), golden) {
			t.Fatalf("project golden changed: %s", h.out)
		}
	}
	h.write(filepath.Join(h.home, "answers.json"), answers)
	if code := h.run(h.home, "setup", "example", "--answers", "answers.json"); code != 0 || h.out.String() != "Setup answers recorded for example\n" {
		t.Fatalf("%d %s %s", code, h.out, h.errb)
	}
	if _, err := os.Stat(filepath.Join(os.Getenv("CLAUDE_CONFIG_DIR"), "settings.json")); !os.IsNotExist(err) {
		t.Fatal("project path wrote Claude settings")
	}
	for _, args := range [][]string{{"setup", "--purge"}, {"setup", "--answers", "-"}, {"setup", "example", "--remove"}, {"setup", "example", "--yes"}} {
		if code := h.run(h.home, args...); code != 2 {
			t.Fatalf("accepted %v: %d", args, code)
		}
	}
}

// `grid setup` writes no deny rule: agents write their own task state (task folders,
// project.md and the brain, in any spelling Claude Code folds), a project's setup
// answers and the person's approval identity. The table is the one the deny rules it
// used to write were checked against.
func TestClaudeDenyRulesLeaveAgentStateOpen(t *testing.T) {
	for _, location := range []string{"default", "outside home"} {
		t.Run(location, func(t *testing.T) {
			h := claudeHarness(t)
			projects := filepath.Join(h.gridHome, "projects")
			if location == "outside home" {
				projects = filepath.Join(h.root, "work")
			}
			t.Setenv("GRID_PROJECTS", projects)
			if code := h.run(h.home, "setup", "--yes"); code != 0 {
				t.Fatalf("code=%d: %s", code, h.errb)
			}
			data, err := os.ReadFile(filepath.Join(os.Getenv("CLAUDE_CONFIG_DIR"), "settings.json"))
			if err != nil {
				t.Fatal(err)
			}
			var doc struct{ Permissions struct{ Deny []string } }
			if err := json.Unmarshal(data, &doc); err != nil {
				t.Fatal(err)
			}
			// settings.TestClaudeBlockLeavesTheSetupAnswerFilesToAgents checks the block
			// itself and settings.TestClaudeSetupTakesRetiredDeniesOut the retirement.
			if len(doc.Permissions.Deny) != 0 {
				t.Errorf("setup wrote deny rules: %q", doc.Permissions.Deny)
			}
			paths := []string{
				"projects/p/tasks/T-01-work", "projects/p/tasks/T-01-work/task.md", "projects/p/tasks/T-1/sub/deep/file", "projects/p/TASKS/t-22-work/brief.md",
				"personal/tasks/T-01-work", "personal/tasks/t-02/task.md", "personal/tasks/T-9/sub/file",
				"projects/p/knowledge", "projects/p/knowledge/topics/x.md", "projects/p/KNOWLEDGE/file",
				"knowledge", "knowledge/topics/x.md", "KNOWLEDGE/x",
				"projects/p/project.md", "projects/p/PROJECT.MD",
				"projects/p/tasks/README.md", "projects/p/tasks/T-x/task.md", "projects/p/tasks/T-/task.md", "personal/tasks/T-x/task.md",
				"projects/p/notes/x.md", "projects/p/nested/project.md", "projects/p/project.md/nested", "projects/p/project.md.bak", "projects/p/tasks", "settings.json", "personal/project.md", "personal/knowledge/x.md",
				"setup/answers.json", "setup/claude-block.json",
			}
			for _, rel := range paths {
				target := filepath.Join(h.gridHome, filepath.FromSlash(rel))
				if suffix, ok := strings.CutPrefix(rel, "projects/"); ok {
					target = filepath.Join(projects, filepath.FromSlash(suffix))
				}
				for _, rule := range doc.Permissions.Deny {
					if !strings.HasPrefix(rule, "Edit(") && !strings.HasPrefix(rule, "Write(") {
						continue
					}
					edit := "Edit(" + strings.SplitN(rule, "(", 2)[1]
					if claudeRuleMatches(t, h.home, edit, target) {
						t.Errorf("%s is denied by %s", h.norm(target), rule)
					}
				}
			}
		})
	}
}

// These generated patterns use only segment globs plus a final recursive /**.
// The docs specify that a directory/** also matches the directory itself.
func claudePatternMatches(t *testing.T, pattern, target string) bool {
	t.Helper()
	pp, tp := strings.Split(pattern, "/"), strings.Split(target, "/")
	recursive := pp[len(pp)-1] == "**"
	if recursive {
		pp = pp[:len(pp)-1]
	}
	if len(tp) < len(pp) || !recursive && len(tp) != len(pp) {
		return false
	}
	for i, p := range pp {
		ok, err := path.Match(p, tp[i])
		if err != nil {
			t.Fatal(err)
		}
		if !ok {
			return false
		}
	}
	return true
}
