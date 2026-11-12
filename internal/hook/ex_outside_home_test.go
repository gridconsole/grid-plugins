package hook

import (
	"bytes"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/project"
	"github.com/gridconsole/grid-plugins/internal/settings"
	"github.com/gridconsole/grid-plugins/internal/where"
)

// These tests pin what SessionStart writes outside the Grid home (docs/privacy.md):
// the agent files in the Claude agents folder at a level other than off, and the
// lines appended to CLAUDE_ENV_FILE at every level. Every path is under t.TempDir().

// exHomeRig is one temp machine: a HOME, a Grid home inside it, a project folder, a
// plugin root with templates and an optional Claude config folder.
type exHomeRig struct {
	root, home, gridHome, cwd, plugin, configDir, agentsDir, envFile string
	task                                                             string // GRID_TASK to pass, empty for none
}

func exHomeNewRig(t *testing.T, withConfigDir bool) *exHomeRig {
	t.Helper()
	r := &exHomeRig{root: t.TempDir()}
	r.home = filepath.Join(r.root, "home")
	r.gridHome = filepath.Join(r.home, ".grid")
	r.cwd = filepath.Join(r.root, "work")
	r.plugin = filepath.Join(r.root, "plugin")
	r.envFile = filepath.Join(r.root, "claude-env")
	r.configDir = filepath.Join(r.home, ".claude")
	if withConfigDir {
		r.configDir = filepath.Join(r.root, "claude-config")
	}
	r.agentsDir = filepath.Join(r.configDir, "agents")
	exHomeMkdir(t, r.cwd)
	exHomeMkdir(t, r.home)
	rec := project.Record{Name: "example", ID: "01EXAMPLE", Folders: []project.Folder{{Machine: "01TESTMACHINE", Path: r.cwd}}}
	if err := project.Write(r.gridHome, rec); err != nil {
		t.Fatal(err)
	}
	templates := filepath.Join(r.plugin, "agent-templates")
	exHomeMkdir(t, templates)
	for _, name := range []string{"Explore", "grid-builder", "grid-scout"} {
		text := "---\nname: " + name + "\ndescription: " + name + "\nmodel: sonnet\neffort: medium\n---\n\nBody of " + name + ".\n"
		exHomeWrite(t, filepath.Join(templates, name+".md"), text)
	}
	r.envFile = filepath.Join(r.root, "claude-env")
	exHomeWrite(t, r.envFile, "export EARLIER=kept\n")
	return r
}

func exHomeMkdir(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
}

func exHomeWrite(t *testing.T, path, text string) {
	t.Helper()
	exHomeMkdir(t, filepath.Dir(path))
	if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}
}

// exHomeSnapshot maps every file and folder under the rig root, the Grid home left out, to its content.
func exHomeSnapshot(t *testing.T, r *exHomeRig) map[string]string {
	t.Helper()
	seen := map[string]string{}
	err := filepath.WalkDir(r.root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if path == r.gridHome {
			return filepath.SkipDir
		}
		if d.IsDir() {
			seen[path] = "<dir>"
			return nil
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		seen[path] = string(b)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return seen
}

// exHomeChanged lists the paths that are new or whose bytes differ between two snapshots.
func exHomeChanged(before, after map[string]string) (created, modified []string) {
	for p, text := range after {
		old, ok := before[p]
		switch {
		case !ok:
			created = append(created, p)
		case old != text:
			modified = append(modified, p)
		}
	}
	for p := range before {
		if _, ok := after[p]; !ok {
			modified = append(modified, p+" (removed)")
		}
	}
	sort.Strings(created)
	sort.Strings(modified)
	return created, modified
}

// exHomeRun runs one SessionStart with level on or off and returns what it printed.
func exHomeRun(t *testing.T, r *exHomeRig, level string, withEnvFile, withConfigVar bool) string {
	t.Helper()
	if level != "" {
		if err := (settings.Store{Home: r.gridHome}).SetHook("session-start", level); err != nil {
			t.Fatal(err)
		}
	}
	if r.task != "" {
		exHomeMkdir(t, filepath.Join(r.gridHome, "projects", "example", "tasks", r.task))
	}
	out := &bytes.Buffer{}
	o := Options{
		Event:    "sessionstart",
		Cwd:      r.cwd,
		Stdin:    strings.NewReader(`{"source":"startup"}`),
		Stdout:   out,
		Resolver: where.Resolver{GridHome: r.gridHome, Home: r.home, Machine: "01TESTMACHINE"},
		Getenv: func(key string) string {
			switch key {
			case "CLAUDE_PLUGIN_ROOT":
				return r.plugin
			case "CLAUDE_ENV_FILE":
				if withEnvFile {
					return r.envFile
				}
			case "CLAUDE_CONFIG_DIR":
				if withConfigVar {
					return r.configDir
				}
			case "GRID_TASK":
				if r.task != "" {
					return "T-07"
				}
			case "GRID_PROJECT":
				if r.task != "" {
					return "example"
				}
			}
			return ""
		},
	}
	if code := Run(o); code != 0 {
		t.Fatalf("exit %d", code)
	}
	return out.String()
}

// exHomeAgentFile reports whether a new path is one SessionStart may write: Explore.md or
// grid-*.md in the agents folder, or a folder on the way down to it.
func exHomeAgentFile(r *exHomeRig, path string) bool {
	if filepath.Dir(path) == r.agentsDir {
		name := filepath.Base(path)
		return name == "Explore.md" || strings.HasPrefix(name, "grid-") && strings.HasSuffix(name, ".md")
	}
	return path == r.agentsDir || path == r.configDir
}

func TestExHomeLevelOnWritesOnlyAgents(t *testing.T) {
	for _, withVar := range []bool{false, true} {
		name := "home-dot-claude"
		if withVar {
			name = "claude-config-dir"
		}
		t.Run(name, func(t *testing.T) {
			r := exHomeNewRig(t, withVar)
			before := exHomeSnapshot(t, r)
			out := exHomeRun(t, r, "", false, withVar)
			if !strings.Contains(out, "additionalContext") {
				t.Fatalf("level on printed no context: %q", out)
			}
			created, modified := exHomeChanged(before, exHomeSnapshot(t, r))
			for _, p := range created {
				if !exHomeAgentFile(r, p) {
					t.Errorf("SessionStart created %s outside the Grid home", p)
				}
			}
			if len(modified) > 0 {
				t.Errorf("SessionStart changed existing paths outside the Grid home: %v", modified)
			}
			for _, want := range []string{"Explore.md", "grid-builder.md", "grid-scout.md"} {
				b, err := os.ReadFile(filepath.Join(r.agentsDir, want))
				if err != nil {
					t.Fatalf("no agent file: %v", err)
				}
				if !strings.Contains(string(b), "owner: grid") {
					t.Errorf("%s carries no Grid marker:\n%s", want, b)
				}
			}
		})
	}
}

func TestExHomeEnvFileGetsItsLinesAtEveryLevel(t *testing.T) {
	for _, level := range []string{"", "off"} {
		for _, task := range []string{"", "T-07-example"} {
			name := "level-" + level + "/task-" + task
			t.Run(name, func(t *testing.T) {
				r := exHomeNewRig(t, false)
				r.task = task
				before := exHomeSnapshot(t, r)
				out := exHomeRun(t, r, level, true, false)
				want := "\n" + envBlockStart + "\nexport GRID_HOME='" + r.gridHome + "'\nexport GRID_PROJECTS='" + filepath.Join(r.gridHome, "projects") + "'\n"
				if task != "" {
					want += "export GRID_TASK='T-07-example'\n"
				} else {
					want += "unset GRID_TASK\n"
				}
				want += envBlockEnd + "\n"
				got, err := os.ReadFile(r.envFile)
				if err != nil {
					t.Fatal(err)
				}
				if string(got) != before[r.envFile]+want {
					t.Errorf("env file %q, want the earlier bytes %q then %q", got, before[r.envFile], want)
				}
				created, modified := exHomeChanged(before, exHomeSnapshot(t, r))
				if level == "off" {
					if out != "" {
						t.Errorf("level off printed context: %q", out)
					}
					if len(created) > 0 {
						t.Errorf("level off created %v outside the Grid home", created)
					}
					if _, err := os.Stat(r.agentsDir); err == nil {
						t.Errorf("level off wrote the agents folder %s", r.agentsDir)
					}
				} else {
					for _, p := range created {
						if !exHomeAgentFile(r, p) {
							t.Errorf("created %s outside the Grid home", p)
						}
					}
				}
				if len(modified) != 1 || modified[0] != r.envFile {
					t.Errorf("changed paths %v, want only the env file", modified)
				}
			})
		}
	}
}

func TestExHomeKeepsThePersonsOwnAgents(t *testing.T) {
	r := exHomeNewRig(t, false)
	own := map[string]string{
		"mine.md":         "---\nname: mine\n---\nMy own agent.\n",
		"grid-custom.md":  "---\nname: grid-custom\n---\nUnmarked, mine.\n",
		"grid-scout.md":   "---\nname: grid-scout\nmodel: opus\n---\nUnmarked file named like a template.\n",
		"not-an-agent.md": "no front matter\n",
	}
	for name, text := range own {
		exHomeWrite(t, filepath.Join(r.agentsDir, name), text)
	}
	exHomeRun(t, r, "", false, false)
	for name, text := range own {
		b, err := os.ReadFile(filepath.Join(r.agentsDir, name))
		if err != nil {
			t.Fatalf("%s is gone: %v", name, err)
		}
		if string(b) != text {
			t.Errorf("%s changed:\n%s", name, b)
		}
	}
	if _, err := os.Stat(filepath.Join(r.agentsDir, "grid-builder.md")); err != nil {
		t.Errorf("the marked template was not written: %v", err)
	}
}
