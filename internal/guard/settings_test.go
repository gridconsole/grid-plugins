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

// No guard is fixed at block. plan-guard
// defaults to warn (a warning, no refusal); a setting raises it to block or
// lowers it to off, and a settings file that cannot be read keeps the refusal and reports why.
func TestGuardsFollowTheirLevel(t *testing.T) {
	for _, tc := range []struct {
		name  string
		eval  func(hookio.Input, string, map[string]string) hookio.Decision
		input func(*fixture) hookio.Input
	}{
		{"plan-guard", EvaluatePlan, func(f *fixture) hookio.Input {
			return f.input("Write", filepath.Join(f.projects, "example", "tasks.", "T-03", "notes.md"))
		}},
	} {
		for _, level := range []string{"none", "off", "warn", "block", "malformed"} {
			t.Run(tc.name+"/"+level, func(t *testing.T) {
				f := setup(t)
				if err := project.Write(f.grid, project.Record{Name: "example"}); err != nil {
					t.Fatal(err)
				}
				want := "deny"
				// plan-guard warns by default (verdicts-11nov v-4; needed.md:47).
				if level == "none" {
					want = ""
				}
				switch level {
				case "off", "warn", "block":
					if err := (settings.Store{Home: f.grid}).SetHook(tc.name, level); err != nil {
						t.Fatalf("the command refused %s %s: %v", tc.name, level, err)
					}
					if level != "block" {
						want = ""
					}
				case "malformed":
					f.write(filepath.Join(f.grid, "settings.json"), `{"hooks":`)
				}
				d := tc.eval(tc.input(f), f.root, f.env)
				expect(t, d, want)
				if (level == "warn" || level == "malformed") && len(d.Warnings) != 1 {
					t.Fatalf("%s %s: want one warning, got %+v", tc.name, level, d)
				}
				if level == "off" && len(d.Warnings) != 0 {
					t.Fatalf("off must say nothing: %+v", d)
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

func TestConfiguredLevelPreservesDenialOnSettingsError(t *testing.T) {
	for _, mode := range []string{"off", "warn", "block", "malformed", "unreadable", "resolution"} {
		t.Run(mode, func(t *testing.T) {
			f := setup(t)
			in := f.input("Read", filepath.Join(f.grid, "notes.md"))
			in.Cwd = f.repo
			path := filepath.Join(f.grid, "settings.json")
			switch mode {
			case "malformed":
				f.write(path, `{"hooks":`)
			case "unreadable":
				if err := os.Mkdir(path, 0700); err != nil {
					t.Fatal(err)
				}
			case "resolution":
				f.env["GRID_PROJECT"] = "missing-project"
			default:
				if err := (settings.Store{Home: f.grid}).SetHook("home-allow", mode); err != nil {
					t.Fatal(err)
				}
			}
			original := hookio.Deny("original denial")
			got := configuredLevel("home-allow", in, f.root, f.env, original)
			switch mode {
			case "off":
				if got.Behavior != "" || len(got.Warnings) != 0 {
					t.Fatal(got)
				}
			case "warn":
				if got.Behavior != "" || len(got.Warnings) != 1 || !strings.Contains(got.Warnings[0], original.Reason) {
					t.Fatal(got)
				}
			default:
				if got.Behavior != original.Behavior || got.Reason != original.Reason {
					t.Fatal(got)
				}
			}
		})
	}
}
