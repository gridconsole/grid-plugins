package guard

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/hookio"
	"github.com/gridconsole/grid-plugins/internal/project"
	"github.com/gridconsole/grid-plugins/internal/settings"
)

// The guards that hold the gates read no level. A settings file that switches one
// off or to warn, personal or project, neither silences nor softens it, and the
// commands refuse to write such a level.
func TestAlwaysBlockGuardsIgnoreHookLevels(t *testing.T) {
	for _, tc := range []struct {
		name  string
		eval  func(hookio.Input, string, map[string]string) hookio.Decision
		input func(*fixture) hookio.Input
	}{
		{"plan-guard", EvaluatePlan, func(f *fixture) hookio.Input { return f.input("Write", filepath.Join(f.repo, "code.go")) }},
		{"host-guard", EvaluateHost, func(f *fixture) hookio.Input { return f.input("Write", filepath.Join(f.grid, ".host", "approval")) }},
		{"only-writer", EvaluateOnlyWriter, func(f *fixture) hookio.Input { return f.input("Write", filepath.Join(f.task, "notes.md")) }},
		{"brief-lint", EvaluateBrief, func(f *fixture) hookio.Input {
			f.write(filepath.Join(f.task, "subagents", "01-work.md"), "incomplete")
			return f.input("Agent", "Read subagents/01-work.md")
		}},
	} {
		for _, level := range []string{"none", "off", "warn", "block", "malformed"} {
			t.Run(tc.name+"/"+level, func(t *testing.T) {
				f := setup(t)
				if err := project.Write(f.grid, project.Record{Name: "example"}); err != nil {
					t.Fatal(err)
				}
				if level == "off" || level == "warn" {
					if err := (settings.Store{Home: f.grid}).SetHook(tc.name, level); err == nil {
						t.Fatalf("the command accepted %s %s", tc.name, level)
					}
				}
				if level != "none" {
					doc := `{"hooks":{"` + tc.name + `":"` + level + `"}}`
					if level == "malformed" {
						doc = `{"hooks":`
					}
					f.write(filepath.Join(f.grid, "settings.json"), doc)
					f.write(filepath.Join(f.grid, "projects", "example", "settings.json"), doc)
				}
				d := tc.eval(tc.input(f), f.root, f.env)
				expect(t, d, "deny")
				if len(d.Warnings) > 0 || strings.Contains(d.Reason, "Cannot read hook settings") {
					t.Fatalf("settings changed the decision: %+v", d)
				}
			})
		}
	}
}

// home-allow is the guard whose level a person may change; the project's level
// wins over the personal one, and warn leaves the grant as block does.
func TestHomeAllowProjectLevel(t *testing.T) {
	for level, want := range map[string]string{"off": "", "warn": "allow", "block": "allow"} {
		t.Run(level, func(t *testing.T) {
			f := setup(t)
			if err := project.Write(f.grid, project.Record{Name: "example"}); err != nil {
				t.Fatal(err)
			}
			if err := (settings.Store{Home: f.grid}).SetHook("home-allow", "off"); err != nil {
				t.Fatal(err)
			}
			if err := (settings.Store{Home: f.grid, Project: "example"}).SetHook("home-allow", level); err != nil {
				t.Fatal(err)
			}
			in := f.input("Read", filepath.Join(f.grid, "notes.md"))
			in.Cwd = f.repo
			f.env["GRID_PROJECT"] = "example"
			expect(t, EvaluateHome(in, f.root, f.env), want)
		})
	}
}

func TestHomeAllowSettingAndInvalidSettings(t *testing.T) {
	f := setup(t)
	s := settings.Store{Home: f.grid}
	in := f.input("Read", filepath.Join(f.grid, "notes.md"))
	in.Cwd = f.repo
	if err := s.SetHook("home-allow", "off"); err != nil {
		t.Fatal(err)
	}
	if d := EvaluateHome(in, f.root, f.env); d.Behavior != "" {
		t.Fatal(d)
	}
	if err := s.SetHook("home-allow", "warn"); err != nil {
		t.Fatal(err)
	}
	if d := EvaluateHome(in, f.root, f.env); d.Behavior != "allow" {
		t.Fatal(d)
	}
	if err := os.WriteFile(filepath.Join(f.grid, "settings.json"), []byte(`{"hooks":{"home-allow":"bogus"}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	d := EvaluateHome(in, f.root, f.env)
	if d.Behavior != "deny" || !strings.Contains(d.Reason, "Cannot read hook settings") {
		t.Fatal(d)
	}
}
