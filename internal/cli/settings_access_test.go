package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/project"
	"github.com/gridconsole/grid-plugins/internal/settings"
)

func settingsAccessFixture(t *testing.T, role string) (*harness, string) {
	t.Helper()
	h := newHarness(t)
	cwd := filepath.Join(h.root, "alpha")
	h.mkdir(cwd)
	h.record(project.Record{Name: "alpha", ID: "01ALPHA", Folders: []project.Folder{{Machine: h.env.Machine, Path: cwd}}})
	h.write(filepath.Join(h.gridHome, "projects", "alpha", "project-roles.json"), `{"people":[{"id":"gus","perm":"Client"}]}`)
	h.env.Getenv = func(k string) string {
		return map[string]string{"GRID_ROLE": role, "GRID_MEMBER": "gus"}[k]
	}
	return h, cwd
}

func TestSettingsPersonalWritesRefused(t *testing.T) {
	for _, role := range []string{"member"} {
		for _, args := range [][]string{
			{"hook", "level", "home-allow", "off"},
			{"hook", "level", "needs-you", "off"},
			{"skill", "enable", "research"},
			{"skill", "disable", "research"},
			{"agent", "set", "grid-builder", "--model", "opus"},
		} {
			t.Run(role+"/"+strings.Join(args, "-"), func(t *testing.T) {
				h, cwd := settingsAccessFixture(t, role)
				path := filepath.Join(h.gridHome, "settings.json")
				before := []byte("{\n  \"agents\": {}, \"skills\": {\"research\": false}, \"hooks\": {}\n}\n")
				h.write(path, string(before))
				for _, scope := range [][]string{{"--project", "personal"}, nil} {
					cmd := append(append([]string{}, args...), scope...)
					code := h.run(cwd, cmd...)
					want := args[0] + ": project \"personal\" is not registered\n"
					if code != 1 || h.out.Len() != 0 || h.errb.String() != want {
						t.Errorf("%v: exit %d, stdout %q, stderr %q; want %q", cmd, code, h.out, h.errb, want)
					}
					after, err := os.ReadFile(path)
					if err != nil || !bytes.Equal(before, after) {
						t.Errorf("settings changed: %s (error %v)", after, err)
					}
					for _, file := range []string{".settings.lock", "VERSION"} {
						if _, err := os.Stat(filepath.Join(h.gridHome, file)); !os.IsNotExist(err) {
							t.Errorf("refused write created %s: %v", file, err)
						}
					}
				}
			})
		}
	}
}

func TestSettingsPersonalReadsRefused(t *testing.T) {
	for _, role := range []string{"guest", "unknown"} {
		for _, args := range [][]string{{"hook", "levels"}, {"skill", "list"}, {"agent", "list"}} {
			t.Run(role+"/"+args[0], func(t *testing.T) {
				h, cwd := settingsAccessFixture(t, role)
				unregistered := filepath.Join(h.root, "unregistered")
				h.mkdir(unregistered)
				for _, dir := range []string{h.home, unregistered} {
					if code := h.run(dir, args...); code != 1 || h.out.Len() != 0 {
						t.Errorf("Personal read: exit %d, stdout %q, stderr %q", code, h.out, h.errb)
					}
				}
				// Registered projects still inherit effective settings.
				if code := h.run(cwd, args...); code != 0 {
					t.Errorf("project read: exit %d: %s", code, h.errb)
				}
			})
		}
	}
}

func TestSettingsPersonalAllowedCallers(t *testing.T) {
	for _, role := range append([]string{"", "owner", "OWNER", "cli", "worker", "agent", "builder", "reviewer", "curator", "host", "checker", "designer", "library", "operator", "planner", "researcher", "runner", "scout"}, settings.AgentNames()...) {
		t.Run(role, func(t *testing.T) {
			h, cwd := settingsAccessFixture(t, role)
			for _, args := range [][]string{{"hook", "level", "home-allow", "off"}, {"skill", "enable", "research"}, {"agent", "set", "grid-builder", "--model", "opus"}} {
				code := h.run(cwd, args...)
				if code != 0 || !strings.Contains(h.out.String(), "updated in personal") {
					t.Fatalf("%v: exit %d: %s %s", args, code, h.out, h.errb)
				}
			}
			if code := h.run(h.home, "hook", "levels"); code != 0 || !strings.Contains(h.out.String(), "home-allow off") {
				t.Fatalf("read: exit %d: %s %s", code, h.out, h.errb)
			}
		})
	}
}

func TestPersonalInvalidTaskPathIsPrivate(t *testing.T) {
	testPersonalTaskPaths(t, []string{"Personal", "PERSONAL", "pErSoNaL", "not-a-task"})
}

func TestPersonalCaseVariantTaskPathDarwin(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("macOS case-variant Personal path regression")
	}
	testPersonalTaskPaths(t, []string{"Personal", "PERSONAL", "pErSoNaL"})
}

func testPersonalTaskPaths(t *testing.T, roots []string) {
	t.Helper()
	h := newHarness(t)
	h.env.Getenv = func(string) string { return "" }
	personal := taskFixture(t, h)
	h.env.Getenv = func(k string) string { return map[string]string{"GRID_ROLE": "guest", "GRID_MEMBER": "gus"}[k] }
	for _, root := range roots {
		for _, task := range []string{filepath.Base(personal.Path), "T-99-missing"} {
			path := filepath.Join(h.gridHome, root, "tasks", task)
			for _, cmd := range [][]string{{"where"}, {"feed", "read"}} {
				args := append(append([]string{}, cmd...), "--task", path)
				if code := h.run(h.home, args...); code != 1 || h.out.Len() != 0 || h.errb.String() != map[string]string{"where": "grid", "feed": "grid feed"}[cmd[0]]+": task not found\n" {
					t.Errorf("%v: exit %d, stdout %q, stderr %q", args, code, h.out, h.errb)
				}
			}
		}
	}
}

func TestSettingsRefusalCreatesNothing(t *testing.T) {
	for _, role := range []string{"guest", "unknown"} {
		for _, args := range [][]string{
			{"hook", "level", "home-allow", "off"},
			{"skill", "enable", "research"},
			{"setup", "personal", "--answers", "-"},
		} {
			t.Run(role+"/"+args[0], func(t *testing.T) {
				h, cwd := settingsAccessFixture(t, role)
				h.env.Stdin = strings.NewReader(`{"test":"make test"}`)
				before := settingsAccessTree(t, h.gridHome)
				if code := h.run(cwd, args...); code != 1 || h.out.Len() != 0 {
					t.Errorf("exit %d, stdout %q, stderr %q", code, h.out, h.errb)
				}
				if !reflect.DeepEqual(before, settingsAccessTree(t, h.gridHome)) {
					t.Error("refused command changed Grid home")
				}
			})
		}
	}
}

func settingsAccessTree(t *testing.T, root string) map[string]string {
	t.Helper()
	files := map[string]string{}
	if err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			files[path] = "directory"
			return nil
		}
		data, err := os.ReadFile(path)
		files[path] = string(data)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	return files
}
