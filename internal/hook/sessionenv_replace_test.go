package hook

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// runSessionStart runs SessionStart once with the given env file, plugin root and task.
func runSessionStart(t *testing.T, o Options, envFile, pluginRoot, task string) {
	t.Helper()
	o.Getenv = func(key string) string {
		switch key {
		case "CLAUDE_ENV_FILE":
			return envFile
		case "CLAUDE_PLUGIN_ROOT":
			return pluginRoot
		case "GRID_TASK":
			return task
		case "GRID_PROJECT":
			if task != "" {
				return "example"
			}
		}
		return ""
	}
	if code := Run(o); code != 0 {
		t.Fatalf("exit %d", code)
	}
}

func countLines(text, prefix string) int {
	n := 0
	for _, l := range strings.Split(text, "\n") {
		if strings.HasPrefix(l, prefix) {
			n++
		}
	}
	return n
}

func readEnvFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestSessionEnvFiveRunsKeepOneBlock(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("posix path line")
	}
	o, _, root := fixture(t)
	plug := pluginRootWithGrid(t, "plug")
	envFile := filepath.Join(root, "env")
	for i := 0; i < 5; i++ {
		runSessionStart(t, o, envFile, plug, "")
	}
	text := readEnvFile(t, envFile)
	if n := countLines(text, "export GRID_HOME="); n != 1 {
		t.Errorf("GRID_HOME lines: %d, want 1\n%s", n, text)
	}
	if n := strings.Count(text, filepath.Join(plug, "bin")); n != 1 {
		t.Errorf("PATH lines with the plugin bin dir: %d, want 1\n%s", n, text)
	}
	if n := countLines(text, "export PATH="); n != 1 {
		t.Errorf("export PATH lines: %d, want 1\n%s", n, text)
	}
	if n := countLines(text, "export GRID_PROJECTS="); n != 1 {
		t.Errorf("GRID_PROJECTS lines: %d, want 1\n%s", n, text)
	}
	if n := countLines(text, "unset GRID_TASK"); n != 1 {
		t.Errorf("unset GRID_TASK lines: %d, want 1\n%s", n, text)
	}
}

func TestSessionEnvTaskChangeKeepsNewValueOnly(t *testing.T) {
	o, _, root := fixture(t)
	for _, name := range []string{"T-07-example", "T-08-other"} {
		if err := os.MkdirAll(filepath.Join(o.Resolver.GridHome, "projects", "example", "tasks", name), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	envFile := filepath.Join(root, "env")
	runSessionStart(t, o, envFile, "", "T-07")
	if text := readEnvFile(t, envFile); !strings.Contains(text, "export GRID_TASK='T-07-example'\n") {
		t.Fatalf("first run wrote no T-07 task:\n%s", text)
	}
	runSessionStart(t, o, envFile, "", "T-08")
	text := readEnvFile(t, envFile)
	if !strings.Contains(text, "export GRID_TASK='T-08-other'\n") {
		t.Errorf("second run wrote no T-08 task:\n%s", text)
	}
	if strings.Contains(text, "T-07") {
		t.Errorf("old task value is still in the file:\n%s", text)
	}
	if n := countLines(text, "export GRID_TASK="); n != 1 {
		t.Errorf("GRID_TASK lines: %d, want 1\n%s", n, text)
	}
}

func TestSessionEnvForeignLinesBeforeAndAfterSurvive(t *testing.T) {
	o, _, root := fixture(t)
	envFile := filepath.Join(root, "env")
	runSessionStart(t, o, envFile, "", "")
	// Put foreign lines on both sides of the block, as another tool would.
	text := readEnvFile(t, envFile)
	if err := os.WriteFile(envFile, []byte("export BEFORE=1\n"+text+"export AFTER=2\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 3; i++ {
		runSessionStart(t, o, envFile, "", "")
	}
	got := readEnvFile(t, envFile)
	if !strings.HasPrefix(got, "export BEFORE=1\n") {
		t.Errorf("line before the block is gone or moved:\n%s", got)
	}
	if !strings.HasSuffix(got, "export AFTER=2\n") {
		t.Errorf("line after the block is gone or moved:\n%s", got)
	}
	if n := countLines(got, "export GRID_HOME="); n != 1 {
		t.Errorf("GRID_HOME lines: %d, want 1\n%s", n, got)
	}
	// A file that ends without a newline keeps its last line whole.
	if err := os.WriteFile(envFile, []byte("export TAIL=3"), 0o600); err != nil {
		t.Fatal(err)
	}
	runSessionStart(t, o, envFile, "", "")
	if got := readEnvFile(t, envFile); !strings.HasPrefix(got, "export TAIL=3\n") {
		t.Errorf("last line without a newline was joined to the block:\n%s", got)
	}
}

func TestSessionEnvUnclosedStartMarkerIsAForeignLine(t *testing.T) {
	o, _, root := fixture(t)
	envFile := filepath.Join(root, "env")
	if err := os.WriteFile(envFile, []byte(envBlockStart+"\nexport KEEP=1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	runSessionStart(t, o, envFile, "", "")
	runSessionStart(t, o, envFile, "", "")
	got := readEnvFile(t, envFile)
	if !strings.Contains(got, "export KEEP=1\n") {
		t.Errorf("a line after an unclosed start marker was dropped:\n%s", got)
	}
	if n := countLines(got, "export GRID_HOME="); n != 1 {
		t.Errorf("GRID_HOME lines: %d, want 1\n%s", n, got)
	}
}
