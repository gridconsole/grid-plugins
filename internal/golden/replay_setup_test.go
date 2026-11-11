package golden_test

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/golden"
)

// grDriver never runs the CLI: Resolve echoes the case argv and Run returns out,
// after calling hook (when set) with the invocation.
func grDriver(out golden.Output, hook func(golden.Invocation) golden.Output) golden.Driver {
	return golden.Driver{
		Resolve: func(c golden.Command) ([]string, bool) { return c.Argv, true },
		Run: func(in golden.Invocation) golden.Output {
			if hook != nil {
				return hook(in)
			}
			return out
		},
	}
}

// grSetup writes setup.json for the case with argv "main" plus the given keys.
func grSetup(t *testing.T, dir string, extra map[string]any) {
	t.Helper()
	m := map[string]any{"argv": []string{"main"}}
	for k, v := range extra {
		m[k] = v
	}
	b, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(dir, "setup.json"), string(b))
}

// grRun replays dir into a fresh box with the stub driver.
func grRun(t *testing.T, dir string) (golden.Result, error) {
	t.Helper()
	return golden.Replay("testdata/fixtures", dir, t.TempDir(), grDriver(golden.Output{}, nil))
}

// grOutside is a path in a second TempDir, outside any box.
func grOutside(t *testing.T) string {
	t.Helper()
	return filepath.Join(t.TempDir(), "x")
}

func grWantErr(t *testing.T, err error, want string) {
	t.Helper()
	if err == nil || err.Error() != want {
		t.Fatalf("got %v, want %q", err, want)
	}
}

func grWantPrefix(t *testing.T, err error, prefix string) {
	t.Helper()
	if err == nil || !strings.HasPrefix(err.Error(), prefix) {
		t.Fatalf("got %v, want prefix %q", err, prefix)
	}
}

func grWantNotExist(t *testing.T, err error) {
	t.Helper()
	if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("got %v, want os.ErrNotExist", err)
	}
}

// Row 2: replay.go:84.
func TestGRCasesWalkError(t *testing.T) {
	_, err := golden.Cases(filepath.Join(t.TempDir(), "missing"))
	grWantNotExist(t, err)
}

// Row 3: replay.go:103.
func TestGRSetupMissing(t *testing.T) {
	dir := copyCase(t)
	if err := os.Remove(filepath.Join(dir, "setup.json")); err != nil {
		t.Fatal(err)
	}
	_, err := grRun(t, dir)
	grWantNotExist(t, err)
}

// Row 3: replay.go:107.
func TestGRSetupSyntax(t *testing.T) {
	dir := copyCase(t)
	write(t, filepath.Join(dir, "setup.json"), "{")
	_, err := grRun(t, dir)
	var syn *json.SyntaxError
	if !errors.As(err, &syn) {
		t.Fatalf("got %v, want *json.SyntaxError", err)
	}
}

// Row 3: replay.go:112.
func TestGRBoxMissing(t *testing.T) {
	dir := copyCase(t)
	_, err := golden.Replay("testdata/fixtures", dir, filepath.Join(t.TempDir(), "missing"), grDriver(golden.Output{}, nil))
	grWantNotExist(t, err)
}

// Row 4: replay.go:134.
func TestGRBoxFolderBlocked(t *testing.T) {
	dir := copyCase(t)
	box := t.TempDir()
	write(t, filepath.Join(box, "demo"), "file")
	_, err := golden.Replay("testdata/fixtures", dir, box, grDriver(golden.Output{}, nil))
	grWantPrefix(t, err, "mkdir ")
}

// Row 5: replay.go:139.
func TestGRFixturesMissing(t *testing.T) {
	dir := copyCase(t)
	_, err := golden.Replay(filepath.Join(t.TempDir(), "missing"), dir, t.TempDir(), grDriver(golden.Output{}, nil))
	grWantNotExist(t, err)
}

// Row 5: replay.go:150 and :166.
func TestGRFixtureSymlinkUndeclared(t *testing.T) {
	dir := copyCase(t)
	fixtures := t.TempDir()
	write(t, filepath.Join(fixtures, "seed.txt"), "seed")
	if err := os.Symlink("seed.txt", filepath.Join(fixtures, "l")); err != nil {
		t.Skip("symlinks unavailable:", err)
	}
	_, err := golden.Replay(fixtures, dir, t.TempDir(), grDriver(golden.Output{}, nil))
	grWantErr(t, err, "fixture symlinks must be declared in setup.json: l")
}

// Row 6: replay.go:203 and safePath :352.
func TestGRDirsRelative(t *testing.T) {
	dir := copyCase(t)
	grSetup(t, dir, map[string]any{"dirs": []string{"rel/x"}})
	_, err := grRun(t, dir)
	grWantErr(t, err, "fixture path must be absolute: rel/x")
}

func TestGRDirsEscape(t *testing.T) {
	dir := copyCase(t)
	out := grOutside(t)
	grSetup(t, dir, map[string]any{"dirs": []string{out}})
	_, err := grRun(t, dir)
	grWantErr(t, err, "fixture path escapes case: "+out)
}

// Row 6: replay.go:206.
func TestGRDirsOverFile(t *testing.T) {
	dir := copyCase(t)
	grSetup(t, dir, map[string]any{"dirs": []string{"$GRID_HOME/seed.txt"}})
	_, err := grRun(t, dir)
	grWantPrefix(t, err, "mkdir ")
}

// Row 7: replay.go:173.
func TestGRFilesEscape(t *testing.T) {
	dir := copyCase(t)
	out := grOutside(t)
	grSetup(t, dir, map[string]any{"files": map[string]any{out: "x"}})
	_, err := grRun(t, dir)
	grWantErr(t, err, "fixture path escapes case: "+out)
}

// Row 7: replay.go:178 (removing a missing file is not an error) and :210.
func TestGRFilesRemoveMissing(t *testing.T) {
	dir := copyCase(t)
	grSetup(t, dir, map[string]any{"files": map[string]any{"$GRID_HOME/none": nil}})
	if _, err := grRun(t, dir); err != nil {
		t.Fatal(err)
	}
}

// Row 7: replay.go:181.
func TestGRFilesRemoveNonEmptyDir(t *testing.T) {
	dir := copyCase(t)
	grSetup(t, dir, map[string]any{
		"dirs":  []string{"$GRID_HOME/full/sub"},
		"files": map[string]any{"$GRID_HOME/full": nil},
	})
	_, err := grRun(t, dir)
	grWantPrefix(t, err, "remove ")
}

// Row 7: replay.go:190.
func TestGRFilesWriteOverDir(t *testing.T) {
	dir := copyCase(t)
	grSetup(t, dir, map[string]any{
		"dirs":  []string{"$GRID_HOME/empty"},
		"files": map[string]any{"$GRID_HOME/empty": "x"},
	})
	_, err := grRun(t, dir)
	grWantPrefix(t, err, "open ")
}

// Row 8: replay.go:215.
func TestGRLinksRelative(t *testing.T) {
	dir := copyCase(t)
	grSetup(t, dir, map[string]any{"links": map[string]string{"l": "$GRID_HOME/seed.txt"}})
	_, err := grRun(t, dir)
	grWantErr(t, err, "fixture path must be absolute: l")
}

// Row 8: replay.go:219.
func TestGRLinksTargetEscapes(t *testing.T) {
	dir := copyCase(t)
	out := grOutside(t)
	grSetup(t, dir, map[string]any{"links": map[string]string{"$GRID_HOME/l": out}})
	_, err := grRun(t, dir)
	grWantErr(t, err, "fixture path escapes case: "+out)
}

// Row 8: replay.go:225. Replay itself makes the link; if it cannot, the error
// still starts with "symlink ", so the row holds on every platform.
func TestGRLinksOverFile(t *testing.T) {
	dir := copyCase(t)
	grSetup(t, dir, map[string]any{
		"files": map[string]any{"$GRID_HOME/taken": "x"},
		"links": map[string]string{"$GRID_HOME/taken": "$GRID_HOME/seed.txt"},
	})
	_, err := grRun(t, dir)
	grWantPrefix(t, err, "symlink ")
}

// Row 9: replay.go:230.
func TestGREnvMissing(t *testing.T) {
	dir := copyCase(t)
	if err := os.Remove(filepath.Join(dir, "env.txt")); err != nil {
		t.Fatal(err)
	}
	_, err := grRun(t, dir)
	grWantNotExist(t, err)
}

// Row 9: replay.go:235.
func TestGREnvBlankLine(t *testing.T) {
	dir := copyCase(t)
	b, err := os.ReadFile(filepath.Join(dir, "env.txt"))
	if err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(dir, "env.txt"), "\n"+string(b))
	if _, err := grRun(t, dir); err != nil {
		t.Fatal(err)
	}
}

// Row 9: replay.go:239.
func TestGREnvInvalid(t *testing.T) {
	for _, line := range []string{"NOEQUALS", "=v"} {
		t.Run(line, func(t *testing.T) {
			dir := copyCase(t)
			write(t, filepath.Join(dir, "env.txt"), line+"\n")
			_, err := grRun(t, dir)
			grWantErr(t, err, `invalid env line "`+line+`"`)
		})
	}
}

// Row 10: replay.go:245.
func TestGRInputMissing(t *testing.T) {
	dir := copyCase(t)
	if err := os.Remove(filepath.Join(dir, "input.json")); err != nil {
		t.Fatal(err)
	}
	_, err := grRun(t, dir)
	grWantNotExist(t, err)
}

// Row 10: replay.go:255.
func TestGRDiffNoteIsFolder(t *testing.T) {
	dir := copyCase(t)
	write(t, filepath.Join(dir, "go-diff.md", "x"), "x")
	_, err := grRun(t, dir)
	if err == nil || errors.Is(err, os.ErrNotExist) {
		t.Fatalf("got %v, want a non-ErrNotExist error", err)
	}
}

// Row 11: replay.go:307.
func TestGRPreludeFails(t *testing.T) {
	dir := copyCase(t)
	write(t, filepath.Join(dir, "setup.json"), `{"argv":["main"],"pre":[{"argv":["pre"],"input":""}]}`)
	driver := grDriver(golden.Output{}, func(in golden.Invocation) golden.Output {
		if in.Argv[0] == "pre" {
			return golden.Output{Exit: 3, Stderr: "boom"}
		}
		return golden.Output{}
	})
	_, err := golden.Replay("testdata/fixtures", dir, t.TempDir(), driver)
	grWantErr(t, err, `prelude grid pre failed: exit 3 stdout "" stderr "boom"`)
}

// Row 11: replay.go:311.
func TestGRAfterPreRelative(t *testing.T) {
	dir := copyCase(t)
	write(t, filepath.Join(dir, "setup.json"), `{"argv":["main"],"after_pre":{"rel":"x"}}`)
	_, err := grRun(t, dir)
	grWantErr(t, err, "fixture path must be absolute: rel")
}
