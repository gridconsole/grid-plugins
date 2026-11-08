package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/golden"
	"github.com/gridconsole/grid-plugins/internal/gridhome"
	"github.com/gridconsole/grid-plugins/internal/project"
	"github.com/gridconsole/grid-plugins/internal/testgit"
	"github.com/gridconsole/grid-plugins/internal/version"
)

// harness is a fake machine under one temp folder: a home with a Grid home in it,
// and a temp folder of its own so the folders beside them are plain folders. The
// root is the git ceiling, so a repo around the real temp folder is never seen.
type harness struct {
	t                         *testing.T
	root, home, gridHome, tmp string
	env                       Env
	out, errb                 *bytes.Buffer
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	testgit.Isolate(t)
	root := gridhome.Real(t.TempDir())
	h := &harness{t: t, root: root, home: filepath.Join(root, "home"), tmp: filepath.Join(root, "tmp"),
		out: &bytes.Buffer{}, errb: &bytes.Buffer{}}
	h.gridHome = filepath.Join(h.home, ".grid")
	h.mkdir(h.home, h.gridHome, h.tmp)
	h.env = Env{
		Stdout: h.out, Stderr: h.errb, Stdin: strings.NewReader(""),
		Home: h.home, GridHome: h.gridHome, TempRoots: []string{h.tmp},
		Machine: "01LAPTOP", Ceiling: root,
		GitVersion: func() (string, error) { return "git version 2.0.0-test", nil },
	}
	personTestEnv(t, &h.env)
	return h
}

func (h *harness) mkdir(dirs ...string) {
	h.t.Helper()
	for _, d := range dirs {
		if err := os.MkdirAll(d, 0o755); err != nil {
			h.t.Fatal(err)
		}
	}
}

func (h *harness) write(path, text string) {
	h.t.Helper()
	h.mkdir(filepath.Dir(path))
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		h.t.Fatal(err)
	}
}

func (h *harness) record(rec project.Record) {
	h.t.Helper()
	if err := project.Write(h.gridHome, rec); err != nil {
		h.t.Fatal(err)
	}
}

// run resets the output buffers, runs grid in cwd and returns the exit code.
func (h *harness) run(cwd string, args ...string) int {
	h.out.Reset()
	h.errb.Reset()
	h.env.Cwd = cwd
	return Run(args, h.env)
}

var sha = regexp.MustCompile(`\b[0-9a-f]{40}\b`)

// norm makes output independent of the temp folder and of commit hashes.
func (h *harness) norm(s string) string {
	return sha.ReplaceAllString(strings.ReplaceAll(s, h.root, "<ROOT>"), "<SHA>")
}

func TestVersionGolden(t *testing.T) {
	old := version.Version
	version.Version = "1.2.3"
	t.Cleanup(func() { version.Version = old })
	for _, tc := range []struct {
		name string
		args []string
	}{
		{"version-plain", []string{"version"}},
		{"version-json", []string{"version", "--json"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t)
			if code := h.run(h.tmp, tc.args...); code != 0 {
				t.Fatalf("exit %d: %s", code, h.errb)
			}
			golden.Assert(t, tc.name, h.out.String())
		})
	}
}

func TestVersionJSONFields(t *testing.T) {
	h := newHarness(t)
	if code := h.run(h.tmp, "version", "--json"); code != 0 {
		t.Fatalf("exit %d", code)
	}
	var got map[string]string
	if err := json.Unmarshal(h.out.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got["version"] != version.Version || got["contract"] != version.Contract || got["app_min"] != version.AppMin || len(got) != 3 {
		t.Errorf("got %v", got)
	}
}

// inRepoModes runs a test twice: as is, and with TMPDIR, so the whole harness, inside
// a git checkout. Both must give the same output.
var inRepoModes = []struct {
	suffix string
	inRepo bool
}{{"", false}, {"/tmp-in-git-repo", true}}

func harnessFor(t *testing.T, inRepo bool) *harness {
	t.Helper()
	if inRepo {
		testgit.TempInRepo(t)
	}
	return newHarness(t)
}

func TestWhereGolden(t *testing.T) {
	tests := []struct {
		name  string
		setup func(h *harness) (cwd string)
		args  []string
		code  int
	}{
		{"where-record-plain", func(h *harness) string {
			folder := filepath.Join(h.root, "work", "example")
			h.mkdir(folder)
			h.record(project.Record{Name: "example", ID: "01EXAMPLE",
				Folders: []project.Folder{{Machine: h.env.Machine, Path: folder}},
				Keys:    []project.Key{{Key: "remote:example.com/org/example"}}})
			return folder
		}, []string{"where"}, 0},
		{"where-task-json", func(h *harness) string {
			h.record(project.Record{Name: "example", ID: "01EXAMPLE"})
			task := filepath.Join(h.gridHome, "projects", "example", "tasks", "T-07-first")
			h.mkdir(task)
			return task
		}, []string{"where", "--json"}, 0},
		{"where-git-plain", func(h *harness) string {
			repo := filepath.Join(h.root, "work", "app")
			h.mkdir(repo)
			testgit.Init(h.t, repo, "git@example.com:org/app.git", true)
			return repo
		}, []string{"where"}, 0},
		{"where-git-json", func(h *harness) string {
			repo := filepath.Join(h.root, "work", "app")
			h.mkdir(repo)
			testgit.Init(h.t, repo, "git@example.com:org/app.git", true)
			return repo
		}, []string{"where", "--json"}, 0},
		{"where-multirepo-plain", func(h *harness) string {
			suite := filepath.Join(h.root, "work", "suite")
			h.mkdir(filepath.Join(suite, "a"), filepath.Join(suite, "libs", "b"))
			testgit.Init(h.t, filepath.Join(suite, "a"), "", false)
			testgit.Init(h.t, filepath.Join(suite, "libs", "b"), "", false)
			return suite
		}, []string{"where"}, 0},
		{"where-personal-plain", func(h *harness) string { return h.home }, []string{"where"}, 0},
		{"where-folder-json", func(h *harness) string {
			dir := filepath.Join(h.root, "notes", "ideas")
			h.mkdir(dir)
			return dir
		}, []string{"where", "--json"}, 0},
		{"where-explicit-project-json", func(h *harness) string {
			h.record(project.Record{Name: "example", ID: "01EXAMPLE"})
			return h.tmp
		}, []string{"where", "--json", "--project", "example"}, 0},
	}
	for _, mode := range inRepoModes {
		for _, tc := range tests {
			t.Run(tc.name+mode.suffix, func(t *testing.T) {
				h := harnessFor(t, mode.inRepo)
				cwd := tc.setup(h)
				if code := h.run(cwd, tc.args...); code != tc.code {
					t.Fatalf("exit %d, stderr: %s", code, h.errb)
				}
				golden.Assert(t, tc.name, h.norm(h.out.String()))
			})
		}
	}
}

func TestFormatVersionCheck(t *testing.T) {
	tests := []struct {
		name       string
		version    string // "" writes no VERSION
		args       []string
		code       int
		wantStdout bool
		wantStderr string
	}{
		{"no VERSION yet", "", []string{"where"}, 0, true, ""},
		{"same format", "1\n", []string{"where"}, 0, true, ""},
		{"newer format refused", "99\n", []string{"where"}, 1, false, "This data is format 99; this grid reads up to 1. Update grid.\n"},
		{"older format asks for migrate", "0\n", []string{"where", "--json"}, 1, false, "Run: grid migrate\n"},
		{"unreadable VERSION", "one\n", []string{"where"}, 1, false, "Cannot read the format version in ~/.grid/VERSION. Run: grid doctor\n"},
		{"checked before a stub", "99\n", []string{"task", "new"}, 1, false, "Update grid.\n"},
		{"hook stays silent", "99\n", []string{"hook", "sessionstart"}, 0, false, ""},
		{"version runs on any format", "99\n", []string{"version"}, 0, true, ""},
		{"doctor reports it", "99\n", []string{"doctor"}, 1, true, ""},
		{"migrate is not refused", "0\n", []string{"migrate"}, 1, false, "grid migrate: no old projects folder found\n"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t)
			if tc.version != "" {
				h.write(filepath.Join(h.gridHome, "VERSION"), tc.version)
			}
			h.env.Stdin = strings.NewReader(`{"cwd":"` + h.tmp + `"}`)
			code := h.run(h.tmp, tc.args...)
			if code != tc.code || (h.out.Len() > 0) != tc.wantStdout || !strings.HasSuffix(h.errb.String(), tc.wantStderr) ||
				(tc.wantStderr == "" && h.errb.Len() > 0) {
				t.Errorf("grid %v: exit %d, stdout %q, stderr %q", tc.args, code, h.out, h.errb)
			}
		})
	}
}

func TestWhereRefusesTaskOutsideGridHome(t *testing.T) {
	h := newHarness(t)
	outside := filepath.Join(h.root, "elsewhere")
	h.mkdir(outside)
	if code := h.run(h.tmp, "where", "--task", outside); code != 1 {
		t.Fatalf("exit %d, want 1", code)
	}
	if !strings.Contains(h.errb.String(), "outside the Grid home") || h.out.Len() != 0 {
		t.Errorf("stdout %q, stderr %q", h.out, h.errb)
	}
}

func TestWhereNeedsAGridHome(t *testing.T) {
	h := newHarness(t)
	h.env.GridHome = ""
	if code := h.run(h.tmp, "where"); code != 1 || !strings.Contains(h.errb.String(), "GRID_HOME") {
		t.Errorf("exit %d, stderr %q", code, h.errb)
	}
}

func TestDoctorGolden(t *testing.T) {
	tests := []struct {
		name  string
		setup func(h *harness)
		args  []string
		code  int
	}{
		{"doctor-ok-plain", func(h *harness) { h.write(filepath.Join(h.gridHome, "VERSION"), "1\n") }, []string{"doctor"}, 0},
		{"doctor-ok-json", func(h *harness) { h.write(filepath.Join(h.gridHome, "VERSION"), "1\n") }, []string{"doctor", "--json"}, 0},
		{"doctor-missing-home-json", func(h *harness) { h.env.GridHome = filepath.Join(h.home, ".absent") }, []string{"doctor", "--json"}, 1},
		{"doctor-newer-format-plain", func(h *harness) { h.write(filepath.Join(h.gridHome, "VERSION"), "99\n") }, []string{"doctor"}, 1},
		{"doctor-no-git-plain", func(h *harness) {
			h.write(filepath.Join(h.gridHome, "VERSION"), "1\n")
			h.env.GitVersion = func() (string, error) { return "", errors.New("not found") }
		}, []string{"doctor"}, 1},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t)
			healthyDoctorEnvironment(t, h)
			tc.setup(h)
			if code := h.run(h.tmp, tc.args...); code != tc.code {
				t.Fatalf("exit %d, want %d; stdout %s stderr %s", code, tc.code, h.out, h.errb)
			}
			golden.Assert(t, tc.name, h.norm(h.out.String()))
		})
	}
}

func TestStubsAndUsageErrors(t *testing.T) {
	tests := []struct {
		args       []string
		code       int
		wantStderr string
	}{
		{[]string{"task"}, 2, "Usage:"},
		{[]string{"migrate", "--dry-run", "--apply"}, 2, "mutually exclusive"},
		{[]string{"nope"}, 2, `unknown command "nope"`},
		{[]string{"version", "extra"}, 2, "unexpected argument"},
		{[]string{"where", "--nope"}, 2, "flag provided but not defined"},
	}
	for _, tc := range tests {
		h := newHarness(t)
		if code := h.run(h.tmp, tc.args...); code != tc.code || !strings.Contains(h.errb.String(), tc.wantStderr) || h.out.Len() != 0 {
			t.Errorf("grid %v: exit %d, stdout %q, stderr %q", tc.args, code, h.out, h.errb)
		}
	}
}

func TestHelpListsEveryCommand(t *testing.T) {
	for _, args := range [][]string{{}, {"help"}, {"--help"}} {
		h := newHarness(t)
		if code := h.run(h.tmp, args...); code != 0 {
			t.Fatalf("grid %v: exit %d", args, code)
		}
		for _, c := range commands() {
			if !strings.Contains(h.out.String(), "  "+c.name+" ") {
				t.Errorf("grid %v: help does not list %q", args, c.name)
			}
		}
	}
	h := newHarness(t)
	h.run(h.tmp, "help")
	if !strings.Contains(h.out.String(), "task") || !strings.Contains(h.out.String(), "(not built yet)") {
		t.Error("help should mark commands that are not built")
	}
}

func TestHookUnknownEventsAreSilent(t *testing.T) {
	for _, event := range []string{"madeup", ""} {
		h := newHarness(t)
		h.env.Stdin = strings.NewReader(`{"cwd":"` + h.tmp + `"}`)
		args := []string{"hook"}
		if event != "" {
			args = append(args, event)
		}
		if code := h.run(h.tmp, args...); code != 0 || h.out.Len() != 0 || h.errb.Len() != 0 {
			t.Errorf("hook %q: exit %d, stdout %q, stderr %q", event, code, h.out, h.errb)
		}
	}
}

func TestHookSessionStartGolden(t *testing.T) {
	tests := []struct {
		name  string
		setup func(h *harness) (stdin, cwd string)
	}{
		{"hook-sessionstart-git", func(h *harness) (string, string) {
			repo := filepath.Join(h.root, "work", "app")
			h.mkdir(repo)
			testgit.Init(h.t, repo, "", true)
			return `{"session_id":"s1","cwd":"` + repo + `","hook_event_name":"SessionStart","source":"startup"}`, h.tmp
		}},
		{"hook-sessionstart-personal", func(h *harness) (string, string) { return `{"cwd":"` + h.home + `"}`, h.tmp }},
		{"hook-sessionstart-empty-input-uses-cwd", func(h *harness) (string, string) {
			dir := filepath.Join(h.root, "notes", "ideas")
			h.mkdir(dir)
			return ``, dir
		}},
	}
	for _, mode := range inRepoModes {
		for _, tc := range tests {
			t.Run(tc.name+mode.suffix, func(t *testing.T) {
				h := harnessFor(t, mode.inRepo)
				t.Setenv("CLAUDE_ENV_FILE", "")
				t.Setenv("GRID_TASK", "")
				t.Setenv("GRID_PROJECT", "")
				stdin, cwd := tc.setup(h)
				h.env.Stdin = strings.NewReader(stdin)
				if code := h.run(cwd, "hook", "sessionstart"); code != 0 {
					t.Fatalf("exit %d: %s", code, h.errb)
				}
				var parsed struct {
					Out struct {
						Event   string `json:"hookEventName"`
						Context string `json:"additionalContext"`
					} `json:"hookSpecificOutput"`
				}
				if err := json.Unmarshal(h.out.Bytes(), &parsed); err != nil || parsed.Out.Event != "SessionStart" || parsed.Out.Context == "" {
					t.Fatalf("not a SessionStart answer: %v: %s", err, h.out)
				}
				golden.Assert(t, tc.name, h.norm(h.out.String()))
			})
		}
	}
}

func TestHookPreToolUse(t *testing.T) {
	h := newHarness(t)
	h.env.Getenv = func(string) string { return "" }
	h.env.Cwd = filepath.Join(h.root, "repo")
	h.mkdir(h.env.Cwd)
	path := filepath.Join(h.gridHome, "projects", "example", "tasks", "T-01", "task.md")
	b, _ := json.Marshal(map[string]any{"tool_name": "Write", "tool_input": map[string]any{"file_path": path, "content": "text"}})
	h.env.Stdin = bytes.NewReader(b)
	if code := Run([]string{"hook", "pretooluse"}, h.env); code != 0 {
		t.Fatal(code)
	}
	if !strings.Contains(h.out.String(), `"permissionDecision":"deny"`) {
		t.Fatalf("got %s", h.out.String())
	}
}
