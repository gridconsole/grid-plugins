package packaging_test

import (
	"bytes"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/cli"
	"github.com/gridconsole/grid-plugins/internal/doctor"
	"github.com/gridconsole/grid-plugins/internal/hook"
	"github.com/gridconsole/grid-plugins/internal/packaging"
	"github.com/gridconsole/grid-plugins/internal/version"
)

func TestGeneratedLayout(t *testing.T) {
	files, err := packaging.Files(version.Version)
	if err != nil {
		t.Fatal(err)
	}
	for name, want := range files {
		got, err := os.ReadFile(filepath.Join("../..", name))
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(got, want) {
			t.Errorf("%s is stale; run make generate", name)
		}
	}
	b := files["hooks/hooks.json"]
	if err := doctor.ValidateHooks(b); err != nil {
		t.Fatal(err)
	}
	var config packaging.HookConfig
	if err := json.Unmarshal(b, &config); err != nil {
		t.Fatal(err)
	}
	for event, groups := range config.Hooks {
		if !hook.Handles(event) {
			t.Errorf("unhandled event %s", event)
		}
		for _, group := range groups {
			for _, command := range group.Hooks {
				want := `"${CLAUDE_PLUGIN_ROOT}/bin/grid" hook ` + strings.ToLower(event)
				if command.Command != want {
					t.Errorf("%s: command %q", event, command.Command)
				}
			}
		}
	}
	for _, event := range hook.Events() {
		if len(config.Hooks[event.Name]) != 1 {
			t.Errorf("missing or duplicated event %s", event.Name)
		}
	}
}

// The plugin root's .mcp.json starts `grid mcp` as the grid-feed server, in the
// generated set and in what Write puts in a plugin directory.
func TestPackagedMCPManifest(t *testing.T) {
	files, err := packaging.Files(version.Version)
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	if err := packaging.Write(root, version.Version); err != nil {
		t.Fatal(err)
	}
	written, err := os.ReadFile(filepath.Join(root, ".mcp.json"))
	if err != nil || !bytes.Equal(written, files[".mcp.json"]) {
		t.Fatalf("Write did not package .mcp.json: %v", err)
	}
	var manifest struct {
		Servers map[string]struct {
			Command string   `json:"command"`
			Args    []string `json:"args"`
		} `json:"mcpServers"`
	}
	if err := json.Unmarshal(written, &manifest); err != nil {
		t.Fatal(err)
	}
	server, ok := manifest.Servers["grid-feed"]
	if !ok || len(manifest.Servers) != 1 || server.Command != `${CLAUDE_PLUGIN_ROOT}/bin/grid` || len(server.Args) != 1 || server.Args[0] != "mcp" {
		t.Fatalf("manifest: %+v", manifest.Servers)
	}
}

// Check all Markdown under skills, including reference files and fenced examples.
// Grid CLI names are lower case; Grid prose and grid:<skill> invocations are not CLI calls.
var gridCommand = regexp.MustCompile("(?:^|[^A-Za-z0-9_-])(?:[^\\s`\"']*/)?grid(?:[\"']?)[ \\t\\r\\n]+([a-z][a-z0-9-]*)")

func skillCommands(text string) []string {
	var names []string
	for _, match := range gridCommand.FindAllStringSubmatch(text, -1) {
		names = append(names, match[1])
	}
	return names
}

func TestSkillCommandsImplemented(t *testing.T) {
	err := filepath.WalkDir("../../skills", func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || !strings.EqualFold(filepath.Ext(path), ".md") {
			return nil
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for _, name := range skillCommands(string(b)) {
			if !cli.Implemented(name) {
				t.Errorf("%s names unimplemented grid command %q", path, name)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestSkillCommandExtraction(t *testing.T) {
	text := "Run `grid task new`, !grid where and ${CLAUDE_PLUGIN_ROOT}/bin/grid brain search.\n" +
		"```sh\ngrid job show && grid version\n```\n\"${PLUGIN_ROOT}/bin/grid\" doctor\ngrid:task is a skill."
	got := strings.Join(skillCommands(text), ",")
	if want := "task,where,brain,job,version,doctor"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
	if cli.Implemented("not-a-command") {
		t.Fatal("unimplemented commands accepted")
	}
}
