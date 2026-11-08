package hook

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/gridconsole/grid-plugins/internal/project"
	"github.com/gridconsole/grid-plugins/internal/testenv"
	"github.com/gridconsole/grid-plugins/internal/where"
)

func fixture(t *testing.T) (Options, *bytes.Buffer, string) {
	t.Helper()
	root := t.TempDir()
	home := filepath.Join(root, "data")
	cwd := filepath.Join(root, "work")
	if err := os.MkdirAll(cwd, 0o700); err != nil {
		t.Fatal(err)
	}
	const machine = "01TESTMACHINE"
	rec := project.Record{Name: "example", ID: "01EXAMPLE", Folders: []project.Folder{{Machine: machine, Path: cwd}}}
	if err := project.Write(home, rec); err != nil {
		t.Fatal(err)
	}
	out := &bytes.Buffer{}
	return Options{Event: "sessionstart", Cwd: cwd, Stdout: out, Resolver: where.Resolver{GridHome: home, Home: root, Machine: machine}, Getenv: func(string) string { return "" }}, out, root
}

func contextFrom(t *testing.T, out *bytes.Buffer) string {
	t.Helper()
	var result struct {
		Out struct {
			Event   string `json:"hookEventName"`
			Context string `json:"additionalContext"`
		} `json:"hookSpecificOutput"`
	}
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatal(err, out.String())
	}
	if result.Out.Event != "SessionStart" {
		t.Fatalf("%s", out)
	}
	if utf8.RuneCountInString(result.Out.Context) >= 10000 {
		t.Fatal("context exceeds host ceiling")
	}
	return result.Out.Context
}

func TestSessionStartSourcesAndEnvironment(t *testing.T) {
	for _, source := range []string{"startup", "resume", "clear", "compact", "fork"} {
		t.Run(source, func(t *testing.T) {
			o, out, root := fixture(t)
			envFile := filepath.Join(root, "env")
			if err := os.WriteFile(envFile, []byte("export EXISTING=kept"), 0o600); err != nil {
				t.Fatal(err)
			}
			o.Getenv = func(key string) string {
				if key == "CLAUDE_ENV_FILE" {
					return envFile
				}
				return ""
			}
			o.Stdin = strings.NewReader(`{"source":"` + source + `"}`)
			if code := Run(o); code != 0 {
				t.Fatalf("exit %d", code)
			}
			context := contextFrom(t, out)
			for _, want := range []string{"GRID_HOME", "GRID_PROJECTS", "Grid project: example", "Task files are written by grid", "grid:grid-builder", "!grid approve"} {
				if !strings.Contains(context, want) {
					t.Errorf("missing %q: %s", want, context)
				}
			}
			b, err := os.ReadFile(envFile)
			if err != nil {
				t.Fatal(err)
			}
			for _, want := range []string{"export EXISTING=kept\n", "export GRID_HOME='" + o.Resolver.GridHome + "'", "export GRID_PROJECTS='" + filepath.Join(o.Resolver.GridHome, "projects") + "'", "unset GRID_TASK"} {
				if !strings.Contains(string(b), want) {
					t.Errorf("missing %q: %s", want, b)
				}
			}
		})
	}
}

func TestSessionStartTaskAndExplicitEnvironment(t *testing.T) {
	o, out, root := fixture(t)
	task := filepath.Join(o.Resolver.GridHome, "projects", "example", "tasks", "T-07-example")
	if err := os.MkdirAll(task, 0o700); err != nil {
		t.Fatal(err)
	}
	envFile := filepath.Join(root, "env")
	o.Getenv = func(key string) string {
		switch key {
		case "GRID_TASK":
			return "T-07"
		case "GRID_PROJECT":
			return "example"
		case "CLAUDE_ENV_FILE":
			return envFile
		}
		return ""
	}
	Run(o)
	if got := contextFrom(t, out); !strings.Contains(got, "Grid task: T-07") {
		t.Fatal(got)
	}
	b, err := os.ReadFile(envFile)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), "export GRID_TASK='T-07-example'") {
		t.Fatal(string(b))
	}
	// Without explicit variables, task-folder resolution still exports the task.
	out.Reset()
	o.Cwd = task
	o.Getenv = func(key string) string {
		if key == "CLAUDE_ENV_FILE" {
			return envFile
		}
		return ""
	}
	Run(o)
	if got := contextFrom(t, out); !strings.Contains(got, "Grid task: T-07") {
		t.Fatal(got)
	}
}

type brokenReader struct{}

func (brokenReader) Read([]byte) (int, error) { return 0, errors.New("read failed") }

type brokenWriter struct{}

func (brokenWriter) Write([]byte) (int, error) { return 0, errors.New("write failed") }

func TestSessionStartErrorsAreSilent(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(*Options, string)
	}{
		{"invalid JSON", func(o *Options, _ string) { o.Stdin = strings.NewReader(`{"cwd":`) }},
		{"null", func(o *Options, _ string) { o.Stdin = strings.NewReader(`null`) }},
		{"trailing JSON", func(o *Options, _ string) { o.Stdin = strings.NewReader(`{} {}`) }},
		{"read failure", func(o *Options, _ string) { o.Stdin = brokenReader{} }},
		{"oversized input", func(o *Options, _ string) { o.Stdin = strings.NewReader(strings.Repeat(" ", 1<<20) + "{}") }},
		{"env write failure", func(o *Options, root string) {
			o.Getenv = func(k string) string {
				if k == "CLAUDE_ENV_FILE" {
					return root
				}
				return ""
			}
		}},
		{"missing cwd", func(o *Options, _ string) { o.Cwd = "" }},
		{"missing home", func(o *Options, _ string) { o.Resolver.GridHome = "" }},
		{"bad projects", func(o *Options, _ string) {
			path := filepath.Join(o.Resolver.GridHome, "projects")
			if err := os.RemoveAll(path); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, nil, 0o600); err != nil {
				t.Fatal(err)
			}
		}},
		{"unknown event", func(o *Options, _ string) { o.Event = "unknown" }},
		{"output failure", func(o *Options, _ string) { o.Stdout = brokenWriter{} }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			o, out, root := fixture(t)
			tc.change(&o, root)
			if code := Run(o); code != 0 || out.Len() != 0 {
				t.Fatalf("exit %d stdout %s", code, out)
			}
		})
	}
}

func TestEnvironmentShellQuoting(t *testing.T) {
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("sh unavailable")
	}
	root := t.TempDir()
	home := filepath.Join(root, "data'\n$(printf unexpected)")
	file := filepath.Join(root, "env")
	if err := writeEnv(file, home, "T-07"); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("sh", "-c", `. "$1"; printf '%s\n%s\n%s' "$GRID_HOME" "$GRID_PROJECTS" "$GRID_TASK"`, "sh", file)
	cmd.Env = testenv.Temp(t, cmd.Environ())
	b, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	want := home + "\n" + filepath.Join(home, "projects") + "\nT-07"
	if string(b) != want {
		t.Fatalf("got %q want %q", b, want)
	}
}

func TestContextCeiling(t *testing.T) {
	text := sessionContext(where.Result{Project: strings.Repeat("界", 12000)}, where.Resolver{GridHome: "/example"}, "")
	if !utf8.ValidString(text) || utf8.RuneCountInString(text) >= 10000 {
		t.Fatal("invalid context bound")
	}
}

func TestCwdFromInputAndNoProjectCreated(t *testing.T) {
	o, out, root := fixture(t)
	o.Cwd = ""
	cwd := filepath.Join(root, "work")
	b, err := json.Marshal(map[string]string{"cwd": cwd})
	if err != nil {
		t.Fatal(err)
	}
	o.Stdin = bytes.NewReader(b)
	Run(o)
	if got := contextFrom(t, out); !strings.Contains(got, "Grid project: example") {
		t.Fatal(got)
	}
	emptyHome := filepath.Join(root, "empty")
	o.Resolver.GridHome = emptyHome
	o.Cwd = cwd
	o.Stdin = nil
	o.Stdout = io.Discard
	Run(o)
	if _, err := os.Stat(emptyHome); !os.IsNotExist(err) {
		t.Fatalf("session created a Grid home: %v", err)
	}
}

func TestStaleExplicitTaskIsIgnoredWithANote(t *testing.T) {
	o, out, _ := fixture(t)
	o.Getenv = func(key string) string {
		if key == "GRID_TASK" {
			return "T-99"
		}
		return ""
	}
	if code := Run(o); code != 0 {
		t.Fatalf("exit %d", code)
	}
	got := contextFrom(t, out)
	if !strings.Contains(got, "Grid project: example") || !strings.Contains(got, "is gone; ignored.") {
		t.Fatal(got)
	}
}
