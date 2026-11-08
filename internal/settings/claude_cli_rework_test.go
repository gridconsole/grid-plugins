package settings_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path"
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
			root := t.TempDir()
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
			data, err := os.ReadFile(filepath.Join(home, ".claude", "settings.json"))
			if err != nil {
				t.Fatal(err)
			}
			var doc struct{ Permissions struct{ Deny []string } }
			if err := json.Unmarshal(data, &doc); err != nil {
				t.Fatal(err)
			}
			for _, suffix := range []string{"/*/[tT][aA][sS][kK][sS]/[Tt]-[0-9]*/**", "/*/[kK][nN][oO][wW][lL][eE][dD][gG][eE]/**", "/*/[pP][rR][oO][jJ][eE][cC][tT].[mM][dD]"} {
				if !slices.Contains(doc.Permissions.Deny, "Edit("+anchor+suffix+")") {
					t.Errorf("missing project deny %s%s in %v", anchor, suffix, doc.Permissions.Deny)
				}
			}
			for _, tool := range []string{"Edit", "Write"} {
				if !slices.Contains(doc.Permissions.Deny, tool+"(~/.grid/setup/**)") {
					t.Errorf("missing %s setup protection", tool)
				}
			}
		})
	}
}

func TestClaudeReworkBlockCaseFolding(t *testing.T) {
	home := t.TempDir()
	rules, err := settings.ClaudeBlock(home, filepath.Join(home, ".grid"))
	if err != nil {
		t.Fatal(err)
	}
	for _, target := range []string{"Projects/p/tasks/T-1/file", "PROJECTS/p/KNOWLEDGE/file", "Projects/p/Project.MD", "Personal/Tasks/T-1/file", "PERSONAL/TASKS/t-1/file"} {
		matched := false
		for _, rule := range rules {
			if rule.Key != "deny" || !strings.HasPrefix(rule.Value, "Edit(~/.grid/") {
				continue
			}
			pattern := strings.TrimSuffix(strings.TrimPrefix(rule.Value, "Edit(~/.grid/"), ")")
			// Each test target has only one level below a recursive pattern.
			if strings.HasSuffix(pattern, "**") {
				pattern = strings.TrimSuffix(pattern, "**") + "*"
			}
			ok, err := path.Match(pattern, target)
			if err != nil {
				t.Fatal(err)
			}
			matched = matched || ok
		}
		if !matched {
			t.Errorf("case variant is unprotected: %s", target)
		}
	}
}

func TestClaudeReworkSetupDenyUsesRealGridHome(t *testing.T) {
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
	for _, tool := range []string{"Edit", "Write"} {
		if !slices.Contains(rules, settings.ClaudeRule{Section: "permissions", Key: "deny", Value: tool + "(~/data/grid/setup/**)"}) {
			t.Errorf("missing %s protection anchored to real Grid home", tool)
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
