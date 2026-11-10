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

// No guard is fixed at block: the dispatcher follows the only-writer level the
// project sets (the project's level wins over the personal one). Block denies,
// warn lets the write through with the denial as a warning, off says nothing,
// and a settings file that cannot be read keeps the denial.
func TestHookFollowsTheOnlyWriterLevel(t *testing.T) {
	for _, event := range []string{"pretooluse", "permissionrequest"} {
		for _, level := range []string{"none", "off", "warn", "block", "malformed"} {
			t.Run(event+"/"+level, func(t *testing.T) {
				o, out, _ := fixture(t)
				o.Event = event
				s := settings.Store{Home: o.Resolver.GridHome}
				if err := s.SetHook("only-writer", "block"); err != nil {
					t.Fatal(err)
				}
				s.Project = "example"
				switch level {
				case "off", "warn", "block":
					if err := s.SetHook("only-writer", level); err != nil {
						t.Fatalf("the setting refused %s: %v", level, err)
					}
				case "malformed":
					if err := os.WriteFile(filepath.Join(o.Resolver.GridHome, "projects", "example", "settings.json"), []byte(`{"hooks":`), 0o600); err != nil {
						t.Fatal(err)
					}
				}
				in := hookio.Input{ToolName: "Write", Cwd: o.Cwd, ToolInput: map[string]any{"file_path": filepath.Join(o.Resolver.GridHome, "projects", "example", "tasks", "T-01", "notes.md"), "content": "text"}}
				data, _ := json.Marshal(in)
				o.Stdin = bytes.NewReader(data)
				if code := Run(o); code != 0 {
					t.Fatal(code)
				}
				denied := strings.Contains(out.String(), "deny") && strings.Contains(out.String(), "written only by grid")
				if want := level != "off" && level != "warn"; denied != want {
					t.Fatalf("level %s: denied %v, want %v: %s", level, denied, want, out)
				}
				message := ""
				if i := strings.Index(out.String(), `"systemMessage"`); i >= 0 {
					message = out.String()[i:]
				}
				switch level {
				case "warn":
					if !strings.Contains(message, "written only by grid") {
						t.Fatalf("warn must carry the denial as a warning: %s", out)
					}
				case "malformed":
					if !strings.Contains(message, "Cannot read hook settings") {
						t.Fatalf("an unreadable file must be reported: %s", out)
					}
				default:
					if message != "" {
						t.Fatalf("level %s must not warn: %s", level, out)
					}
				}
				if level == "off" && strings.Contains(out.String(), "written only by grid") {
					t.Fatalf("off must not mention the guard: %s", out)
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
