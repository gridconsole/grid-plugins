package settings_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/cli"
	"github.com/gridconsole/grid-plugins/internal/settings"
)

func TestClaudeReworkCLIProjectAnchors(t *testing.T) {
	for _, location := range []string{"default", "outside grid", "outside home", "relative"} {
		t.Run(location, func(t *testing.T) {
			root, err := filepath.EvalSymlinks(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			home := filepath.Join(root, "home")
			projects := ""
			anchor := "~/.grid/[pP][rR][oO][jJ][eE][cC][tT][sS]"
			switch location {
			case "outside grid":
				projects, anchor = filepath.Join(home, "work"), "~/work"
			case "outside home":
				projects = filepath.Join(root, "work")
				anchor = "/" + filepath.ToSlash(projects)
				if runtime.GOOS == "windows" {
					anchor = "//" + strings.ToLower(projects[:1]) + filepath.ToSlash(projects[2:])
				}
			case "relative":
				projects, anchor = "work", "~/work"
			}
			// An earlier setup denied a project's setup answers and held commands here;
			// setup takes those rules out and writes no deny of its own.
			var old []string
			for _, suffix := range []string{"/*/[sS][eE][tT][uU][pP]-[aA][nN][sS][wW][eE][rR][sS].[jJ][sS][oO][nN]", "/*/[sS][eE][tT][uU][pP]-[pP][eE][nN][dD][iI][nN][gG].[jJ][sS][oO][nN]"} {
				for _, tool := range []string{"Edit", "Write"} {
					old = append(old, tool+"("+anchor+suffix+")")
				}
			}
			seeded, err := json.Marshal(map[string]any{"permissions": map[string]any{"deny": append([]string{"Read(~/.ssh/**)"}, old...)}})
			if err != nil {
				t.Fatal(err)
			}
			settingsPath := filepath.Join(home, ".claude", "settings.json")
			if err := os.MkdirAll(filepath.Dir(settingsPath), 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(settingsPath, seeded, 0o600); err != nil {
				t.Fatal(err)
			}
			var out, stderr bytes.Buffer
			env := cli.Env{Home: home, GridHome: filepath.Join(home, ".grid"), Cwd: home, Stdin: strings.NewReader(""), Stdout: &out, Stderr: &stderr,
				Getenv: func(key string) string {
					if key == "GRID_PROJECTS" {
						return projects
					}
					return ""
				},
			}
			if code := cli.Run([]string{"setup", "--yes"}, env); code != 0 {
				t.Fatalf("code=%d: %s", code, &stderr)
			}
			data, err := os.ReadFile(settingsPath)
			if err != nil {
				t.Fatal(err)
			}
			var doc struct{ Permissions struct{ Deny []string } }
			if err := json.Unmarshal(data, &doc); err != nil {
				t.Fatal(err)
			}
			// Agents write task folders, the brain, project.md, the setup folder and a
			// project's setup answers; only the person's own rule stays.
			if want := []string{"Read(~/.ssh/**)"}; !slices.Equal(doc.Permissions.Deny, want) {
				t.Errorf("denies %v, want %v", doc.Permissions.Deny, want)
			}
		})
	}
}

func TestClaudeCLIHomePatternCharacters(t *testing.T) {
	for _, name := range []string{"John (Work)", "John [Work]", "John *", "John ?"} {
		t.Run(name, func(t *testing.T) {
			if runtime.GOOS == "windows" && strings.ContainsAny(name, "*?") {
				t.Skip("character is not allowed in Windows directory names")
			}
			home := filepath.Join(t.TempDir(), name)
			var out, stderr bytes.Buffer
			env := cli.Env{Home: home, GridHome: filepath.Join(home, ".grid"), Cwd: home,
				Stdin: strings.NewReader(""), Stdout: &out, Stderr: &stderr,
				Getenv: func(string) string { return "" },
			}
			for _, args := range [][]string{{"setup", "--yes"}, {"setup", "--remove", "--yes"}} {
				out.Reset()
				stderr.Reset()
				if code := cli.Run(args, env); code != 0 {
					t.Fatalf("%v: code=%d: %s", args, code, &stderr)
				}
				data, err := os.ReadFile(filepath.Join(home, ".claude", "settings.json"))
				if err != nil {
					t.Fatal(err)
				}
				var doc struct {
					Permissions struct{ Allow, Deny []string }
				}
				if err := json.Unmarshal(data, &doc); err != nil {
					t.Fatal(err)
				}
				if len(args) == 2 {
					if !slices.Contains(doc.Permissions.Allow, "Edit(~/.grid/**)") || len(doc.Permissions.Deny) != 0 {
						t.Fatal("missing home-relative rules")
					}
				} else if len(doc.Permissions.Allow)+len(doc.Permissions.Deny) != 0 {
					t.Fatal("owned rules remain after removal")
				}
			}
		})
	}
}

func TestClaudeReworkSetupRulesUseRealGridHome(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink privilege required")
	}
	home := t.TempDir()
	real := filepath.Join(home, "data", "grid")
	if err := os.MkdirAll(real, 0700); err != nil {
		t.Fatal(err)
	}
	alias := filepath.Join(home, ".grid")
	if err := os.Symlink(real, alias); err != nil {
		t.Fatal(err)
	}
	rules, err := settings.ClaudeBlock(home, alias)
	if err != nil {
		t.Fatal(err)
	}
	for _, tool := range []string{"Read", "Edit"} {
		if !slices.Contains(rules, settings.ClaudeRule{Section: "permissions", Key: "allow", Value: tool + "(~/data/grid/**)"}) {
			t.Errorf("missing %s allow rule anchored to real Grid home", tool)
		}
	}
}

func TestClaudeReworkCLIRemoveForgedRecord(t *testing.T) {
	home := t.TempDir()
	grid := filepath.Join(home, ".grid")
	config := filepath.Join(home, ".claude")
	if err := os.MkdirAll(config, 0700); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(config, "settings.json")
	if err := os.WriteFile(file, []byte(`{"permissions":{"deny":["Read(~/.ssh/**)","Bash(curl *)"]}}`), 0600); err != nil {
		t.Fatal(err)
	}
	var out, stderr bytes.Buffer
	env := cli.Env{Home: home, GridHome: grid, Cwd: home, Stdin: strings.NewReader(""), Stdout: &out, Stderr: &stderr, Getenv: func(string) string { return "" }}
	if code := cli.Run([]string{"setup", "--yes"}, env); code != 0 {
		t.Fatalf("install=%d: %s", code, &stderr)
	}
	recordPath := filepath.Join(grid, "setup", "claude-block.json")
	data, err := os.ReadFile(recordPath)
	if err != nil {
		t.Fatal(err)
	}
	var record struct {
		Path  string                `json:"path"`
		Rules []settings.ClaudeRule `json:"rules"`
	}
	if err := json.Unmarshal(data, &record); err != nil {
		t.Fatal(err)
	}
	personal := []string{"Read(~/.ssh/**)", "Bash(curl *)"}
	for _, value := range personal {
		record.Rules = append(record.Rules, settings.ClaudeRule{Section: "permissions", Key: "deny", Value: value})
	}
	data, err = json.Marshal(record)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(recordPath, data, 0600); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	stderr.Reset()
	if code := cli.Run([]string{"setup", "--remove", "--yes"}, env); code != 0 {
		t.Fatalf("remove=%d: %s", code, &stderr)
	}
	data, err = os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	var doc struct{ Permissions struct{ Deny []string } }
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(doc.Permissions.Deny, personal) {
		t.Fatalf("personal denies changed: %v", doc.Permissions.Deny)
	}
	for _, value := range personal {
		if !strings.Contains(stderr.String(), "not a Grid rule, left in place: "+value+"\n") {
			t.Errorf("missing warning for %s", value)
		}
	}
}
