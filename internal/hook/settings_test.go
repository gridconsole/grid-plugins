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

func TestHookSettingsProjectOverride(t *testing.T) {
	for _, event := range []string{"pretooluse", "permissionrequest"} {
		for _, level := range []string{"off", "warn", "block"} {
			t.Run(event+"/"+level, func(t *testing.T) {
				o, out, _ := fixture(t)
				o.Event = event
				s := settings.Store{Home: o.Resolver.GridHome}
				for _, name := range []string{"only-writer", "plan-guard", "home-allow"} {
					if err := s.SetHook(name, "off"); err != nil {
						t.Fatal(err)
					}
				}
				s.Project = "example"
				if err := s.SetHook("only-writer", level); err != nil {
					t.Fatal(err)
				}
				in := hookio.Input{ToolName: "Write", Cwd: o.Cwd, ToolInput: map[string]any{"file_path": filepath.Join(s.Home, "projects", "example", "tasks", "T-01", "notes.md"), "content": "text"}}
				data, _ := json.Marshal(in)
				o.Stdin = bytes.NewReader(data)
				if code := Run(o); code != 0 {
					t.Fatal(code)
				}
				if level == "off" {
					if out.Len() != 0 {
						t.Fatal(out)
					}
					return
				}
				if !json.Valid(out.Bytes()) {
					t.Fatal(out)
				}
				if level == "warn" {
					if !strings.Contains(out.String(), "systemMessage") || strings.Contains(out.String(), "deny") {
						t.Fatal(out)
					}
				} else if !strings.Contains(out.String(), "deny") {
					t.Fatal(out)
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
