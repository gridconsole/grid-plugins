package guard

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/hookio"
	"github.com/gridconsole/grid-plugins/internal/project"
	"github.com/gridconsole/grid-plugins/internal/settings"
)

func TestGuardSettingsLevels(t *testing.T) {
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
		for _, level := range []string{"off", "warn", "block"} {
			t.Run(tc.name+"/"+level, func(t *testing.T) {
				f := setup(t)
				if err := project.Write(f.grid, project.Record{Name: "example"}); err != nil {
					t.Fatal(err)
				}
				personal := settings.Store{Home: f.grid}
				proj := settings.Store{Home: f.grid, Project: "example"}
				if err := personal.SetHook(tc.name, "off"); err != nil {
					t.Fatal(err)
				}
				if err := proj.SetHook(tc.name, level); err != nil {
					t.Fatal(err)
				}
				in := tc.input(f)
				d := tc.eval(in, f.root, f.env)
				want := ""
				if level == "block" {
					want = "deny"
				}
				if d.Behavior != want {
					t.Fatalf("got %+v, want %s", d, want)
				}
				if (len(d.Warnings) > 0) != (level == "warn") {
					t.Fatalf("unexpected warning: %+v", d)
				}
				if level == "warn" {
					var raw map[string]any
					if err := json.Unmarshal(d.Bytes(), &raw); err != nil {
						t.Fatal(err)
					}
					if _, ok := raw["systemMessage"]; !ok {
						t.Fatal(string(d.Bytes()))
					}
					if _, ok := raw["hookSpecificOutput"]; ok {
						t.Fatal("warning changed permission")
					}
				}
			})
		}
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
	if err := os.WriteFile(filepath.Join(f.grid, "settings.json"), []byte(`{"hooks":{"only-writer":"bogus"}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	d := EvaluateOnlyWriter(f.input("Write", filepath.Join(f.task, "notes.md")), f.root, f.env)
	if d.Behavior != "deny" || !strings.Contains(d.Reason, "Cannot read hook settings") {
		t.Fatal(d)
	}
}
