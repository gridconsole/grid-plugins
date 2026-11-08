package golden_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/golden"
)

func TestSyntheticCases(t *testing.T) {
	for _, name := range []string{"pass", "diff"} {
		t.Run(name, func(t *testing.T) {
			box := t.TempDir()
			// Inherited process values must not affect the recorded environment.
			t.Setenv("GRID_HOME", "must-not-be-used")
			result, err := golden.Replay("testdata/fixtures", filepath.Join("testdata", name), box, cliDriver())
			if err != nil {
				t.Fatal(err)
			}
			if result.Skip != "" {
				t.Fatal(result.Skip)
			}
			if name == "pass" && len(result.Differences) != 0 {
				t.Fatal(result.Differences)
			}
			if name == "diff" && (len(result.Differences) != 1 || !strings.Contains(result.Differences[0], "stdout differs")) {
				t.Fatal(result.Differences)
			}
			b, err := os.ReadFile(filepath.Join(box, "demo", "setup.txt"))
			if err != nil || string(b) != "setup" {
				t.Fatalf("setup: %q, %v", b, err)
			}
			b, err = os.ReadFile(filepath.Join(box, "demo", "seed.txt"))
			if err != nil || strings.Contains(string(b), "$GRID_HOME") {
				t.Fatalf("fixture expansion: %q, %v", b, err)
			}
		})
	}
}

func copyCase(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	entries, err := os.ReadDir("testdata/pass")
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		b, err := os.ReadFile(filepath.Join("testdata/pass", entry.Name()))
		if err != nil {
			t.Fatal(err)
		}
		write(t, filepath.Join(dir, entry.Name()), string(b))
	}
	return dir
}
func write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestIntendedDifference(t *testing.T) {
	dir := copyCase(t)
	write(t, filepath.Join(dir, "go-diff.md"), "Deliberate fixture expectation; docs/example.md:12.\n")
	_, err := golden.Replay("testdata/fixtures", dir, t.TempDir(), cliDriver())
	if err == nil {
		t.Fatal("missing go-expected must fail")
	}
	for _, name := range []string{"stdout", "stderr", "exit", "files.diff"} {
		b, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		write(t, filepath.Join(dir, "go-expected", name), string(b))
	}
	write(t, filepath.Join(dir, "stdout"), "old output")
	r, err := golden.Replay("testdata/fixtures", dir, t.TempDir(), cliDriver())
	if err != nil || len(r.Differences) > 0 {
		t.Fatalf("%+v, %v", r, err)
	}
	write(t, filepath.Join(dir, "go-diff.md"), "No citation")
	if _, err := golden.Replay("testdata/fixtures", dir, t.TempDir(), cliDriver()); err == nil {
		t.Fatal("missing citation must fail")
	}
}

func TestPreludeAndFileEffects(t *testing.T) {
	dir := copyCase(t)
	write(t, filepath.Join(dir, "setup.json"), `{"argv":["main"],"pre":[{"argv":["pre"],"input":"literal stdin"}],"after_pre":{"$GRID_HOME/state":"tool edit\n"}}`)
	write(t, filepath.Join(dir, "stdout"), "$GRID_HOME <TIME> <ID-1>\n")
	write(t, filepath.Join(dir, "stderr"), "")
	write(t, filepath.Join(dir, "exit"), "0\n")
	write(t, filepath.Join(dir, "files.diff"), "diff --grid a/demo/state b/demo/state\n--- a/demo/state\n+++ b/demo/state\n@@ -1 +1 @@\n-tool edit\n+done <TIME> <ID-1>\n")
	calls := 0
	driver := golden.Driver{
		Resolve: func(c golden.Command) ([]string, bool) { return c.Argv, true },
		Run: func(in golden.Invocation) golden.Output {
			calls++
			if _, ok := in.Env["EMPTY"]; !ok || in.Env["EMPTY"] != "" {
				t.Fatal("lost empty environment value")
			}
			if _, ok := in.Env["UNSET"]; ok {
				t.Fatal("inherited environment")
			}
			path := filepath.Join(in.Root, "demo", "state")
			if in.Argv[0] == "pre" {
				if in.Stdin != "literal stdin" {
					t.Fatalf("pre stdin %q", in.Stdin)
				}
				write(t, path, "prelude\n")
				return golden.Output{}
			}
			before, err := os.ReadFile(path)
			if err != nil || string(before) != "tool edit\n" {
				t.Fatalf("after_pre: %q %v", before, err)
			}
			text := "2025-01-02T03:04:05.000Z 01ARZ3NDEKTSV4RRFFQ69G5FAV\n"
			write(t, path, "done "+text)
			return golden.Output{Stdout: in.Env["GRID_HOME"] + " " + text}
		},
	}
	r, err := golden.Replay("testdata/fixtures", dir, t.TempDir(), driver)
	if err != nil || len(r.Differences) > 0 || calls != 2 {
		t.Fatalf("%+v, %v, calls %d", r, err, calls)
	}
	driver.Resolve = func(c golden.Command) ([]string, bool) { return c.Argv, c.Argv[0] != "pre" }
	calls = 0
	r, err = golden.Replay("testdata/fixtures", dir, t.TempDir(), driver)
	if err != nil || !strings.Contains(r.Skip, "grid pre") || calls != 0 {
		t.Fatalf("%+v, %v, calls %d", r, err, calls)
	}
}
