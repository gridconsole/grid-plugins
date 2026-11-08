package hook

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/hookio"
	"github.com/gridconsole/grid-plugins/internal/settings"
)

// The commands refuse to lower a guard that always blocks, so a hand-written file
// is the only way to try it; the dispatcher must keep the guard's denial.
func TestHookKeepsAlwaysBlockGuardsWhateverTheSettingsSay(t *testing.T) {
	for _, event := range []string{"pretooluse", "permissionrequest"} {
		for _, doc := range []string{`{"hooks":{"only-writer":"off","plan-guard":"warn"}}`, `{"hooks":{"only-writer":"warn"}}`, `{"hooks":`} {
			t.Run(event+"/"+doc, func(t *testing.T) {
				o, out, _ := fixture(t)
				o.Event = event
				for _, path := range []string{filepath.Join(o.Resolver.GridHome, "settings.json"), filepath.Join(o.Resolver.GridHome, "projects", "example", "settings.json")} {
					if err := os.WriteFile(path, []byte(doc), 0o600); err != nil {
						t.Fatal(err)
					}
				}
				in := hookio.Input{ToolName: "Write", Cwd: o.Cwd, ToolInput: map[string]any{"file_path": filepath.Join(o.Resolver.GridHome, "projects", "example", "tasks", "T-01", "notes.md"), "content": "text"}}
				data, _ := json.Marshal(in)
				o.Stdin = bytes.NewReader(data)
				if code := Run(o); code != 0 {
					t.Fatal(code)
				}
				if !strings.Contains(out.String(), "deny") || !strings.Contains(out.String(), "written only by grid") || strings.Contains(out.String(), "systemMessage") {
					t.Fatal(out)
				}
			})
		}
	}
}

func TestHookHomeAllowLevelFollowsTheProject(t *testing.T) {
	for _, event := range []string{"pretooluse", "permissionrequest"} {
		for _, level := range []string{"off", "warn", "block"} {
			t.Run(event+"/"+level, func(t *testing.T) {
				o, out, _ := fixture(t)
				o.Event = event
				s := settings.Store{Home: o.Resolver.GridHome}
				if err := s.SetHook("home-allow", "off"); err != nil {
					t.Fatal(err)
				}
				s.Project = "example"
				if err := s.SetHook("home-allow", level); err != nil {
					t.Fatal(err)
				}
				in := hookio.Input{ToolName: "Read", Cwd: o.Cwd, ToolInput: map[string]any{"file_path": filepath.Join(s.Home, "notes.md")}}
				data, _ := json.Marshal(in)
				o.Stdin = bytes.NewReader(data)
				if code := Run(o); code != 0 {
					t.Fatal(code)
				}
				if got := strings.Contains(out.String(), "allow"); got != (level != "off") {
					t.Fatalf("level %s: %s", level, out)
				}
			})
		}
	}
}

func TestSessionStartLevelKeepsEnvironment(t *testing.T) {
	o, out, root := fixture(t)
	s := settings.Store{Home: o.Resolver.GridHome}
	if err := s.SetHook("session-start", "off"); err != nil {
		t.Fatal(err)
	}
	envFile := filepath.Join(root, "env")
	o.Getenv = func(key string) string {
		if key == "CLAUDE_ENV_FILE" {
			return envFile
		}
		return ""
	}
	o.Stdin = strings.NewReader(`{}`)
	Run(o)
	if out.Len() != 0 {
		t.Fatal(out)
	}
	data, err := os.ReadFile(envFile)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "export GRID_HOME=") {
		t.Fatal(string(data))
	}
	s.Project = "example"
	if err := s.SetHook("session-start", "warn"); err != nil {
		t.Fatal(err)
	}
	o.Stdin = strings.NewReader(`{}`)
	Run(o)
	if !strings.Contains(contextFrom(t, out), "Grid project: example") {
		t.Fatal(out)
	}
}

func TestSessionStartSaysWhenSettingsCannotBeRead(t *testing.T) {
	o, out, root := fixture(t)
	if err := os.WriteFile(filepath.Join(o.Resolver.GridHome, "settings.json"), []byte(`{"hooks":`), 0o600); err != nil {
		t.Fatal(err)
	}
	envFile := filepath.Join(root, "env")
	o.Getenv = func(key string) string {
		if key == "CLAUDE_ENV_FILE" {
			return envFile
		}
		return ""
	}
	o.Stdin = strings.NewReader(`{}`)
	Run(o)
	context := contextFrom(t, out)
	if !strings.Contains(context, "Grid project: example") || !strings.Contains(context, "Hook settings cannot be read") {
		t.Fatal(context)
	}
	if _, err := os.Stat(envFile); err != nil {
		t.Fatal(err)
	}
}
