package projectmd

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/localdata"
	"github.com/gridconsole/grid-plugins/internal/projects"
	"github.com/gridconsole/grid-plugins/internal/where"
)

// These tests hold the read-modify-write of project.md front matter that Key
// does under .projects.lock: writers of different keys, in goroutines and in
// separate processes, keep every edit. A child process is this test binary run
// again on TestGgProjectmdConcurrentChild, the way internal/cli/delivery_test.go
// runs TestHistoryCommandProcess. The tests wait on exit codes and channels,
// never on a sleep, and assert no time.

const (
	ggProject      = "ggproj"
	ggChildMarker  = "GG_PROJECTMD_CHILD"
	ggChildName    = "GG_PROJECTMD_CHILD_NAME"
	ggChildSets    = "GG_PROJECTMD_CHILD_SETS"
	ggResultPrefix = "ggresult "
	// ggFront is the front matter in the writer's own layout, so the keys a
	// test adds follow it and the lines before them stay byte equal.
	ggFront = "id: gg-project\nlands: reviewed\nowner: someone\n"
	ggBody  = "# Gg\n\nBody line one.\n\n    indented code\n\nLast line, tab\there.\n"
)

// ggEnvKeys are the four variables every test and every child points into the
// test's temp folder, so nothing reads or writes a real home.
var ggEnvKeys = []string{"HOME", "CLAUDE_CONFIG_DIR", "CODEX_HOME", "GRID_HOME"}

// ggSetup points the four variables under one temp root and writes the project
// into the Grid home (the GRID_HOME folder). It returns the root and the home.
func ggSetup(t *testing.T) (root, home string) {
	t.Helper()
	root = t.TempDir()
	for _, k := range ggEnvKeys {
		dir := filepath.Join(root, strings.ToLower(k))
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		t.Setenv(k, dir)
	}
	rel, err := filepath.Rel(root, os.Getenv("HOME"))
	if err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		t.Fatalf("HOME is not under the temp root %s", root)
	}
	home = os.Getenv("GRID_HOME")
	// A Grid home that holds a project already has its format marker.
	if err := localdata.Init(home); err != nil {
		t.Fatal(err)
	}
	writeProjectMD(t, home, ggProject, "---\n"+ggFront+"---\n"+ggBody)
	return root, home
}

func ggPath(home string) string { return filepath.Join(home, "projects", ggProject, "project.md") }

// ggKV is the i-th key and its value: k01 and value-01.
func ggKV(i int) (key, value string) {
	return fmt.Sprintf("k%02d", i), fmt.Sprintf("value-%02d", i)
}

func ggSet(r where.Resolver, key, value string) error {
	_, err := Key(r, ggProject, key, &value)
	return err
}

func ggGet(t *testing.T, r where.Resolver, key string) any {
	t.Helper()
	got, err := Key(r, ggProject, key, nil)
	if err != nil {
		t.Fatalf("read %s: %v", key, err)
	}
	return got.Value
}

// ggCheckFile fails unless project.md is the fixture's own front matter, then
// exactly the lines in want in any order, then the fixture's body byte for byte.
func ggCheckFile(t *testing.T, home string, want []string) {
	t.Helper()
	b, err := os.ReadFile(ggPath(home))
	if err != nil {
		t.Fatal(err)
	}
	head, tail := "---\n"+ggFront, "---\n"+ggBody
	text := string(b)
	if !strings.HasPrefix(text, head) || !strings.HasSuffix(text, tail) {
		t.Fatalf("the keys that were there before or the body changed:\n%q", text)
	}
	middle := strings.TrimSuffix(strings.TrimPrefix(text, head), tail)
	got := strings.Split(strings.TrimSuffix(middle, "\n"), "\n")
	want = append([]string(nil), want...)
	sort.Strings(got)
	sort.Strings(want)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("front matter lines after the old keys:\n got %q\nwant %q", got, want)
	}
	for key, value := range map[string]string{"id": "gg-project", "lands": "reviewed", "owner": "someone"} {
		if got := ggGet(t, where.Resolver{GridHome: home}, key); got != value {
			t.Fatalf("old key %s reads %#v; want %q", key, got, value)
		}
	}
}

// ggNothingBeside fails when project.md has a neighbour: a temp file of the
// atomic writer left beside it.
func ggNothingBeside(t *testing.T, home string) {
	t.Helper()
	entries, err := os.ReadDir(filepath.Dir(ggPath(home)))
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	if !reflect.DeepEqual(names, []string{"project.md"}) {
		t.Fatalf("beside project.md: %q; want only project.md", names)
	}
}

// ggChild is one child process: its label and the key=value pairs it sets in
// order, joined by ";".
type ggChild struct{ name, sets string }

// TestGgProjectmdConcurrentChild is the child process. It does nothing unless
// the parent set the marker. It blocks on stdin until the parent has started
// every child and closed the pipes, so the writes overlap, then sets its keys
// and prints its own result line.
func TestGgProjectmdConcurrentChild(t *testing.T) {
	if os.Getenv(ggChildMarker) != "1" {
		t.Skip("helper of the process tests; it runs only as a child of them")
	}
	home := os.Getenv("GRID_HOME")
	if home == "" {
		t.Fatal("GRID_HOME is not set")
	}
	if _, err := io.Copy(io.Discard, os.Stdin); err != nil {
		t.Fatal(err)
	}
	r := where.Resolver{GridHome: home}
	n := 0
	for _, pair := range strings.Split(os.Getenv(ggChildSets), ";") {
		key, value, ok := strings.Cut(pair, "=")
		if !ok {
			t.Fatalf("bad pair %q", pair)
		}
		if err := ggSet(r, key, value); err != nil {
			t.Fatalf("set %s: %v", key, err)
		}
		n++
	}
	fmt.Printf("%s%s set %d keys\n", ggResultPrefix, os.Getenv(ggChildName), n)
}

// ggRunChildren starts every child, then closes every stdin pipe so none runs
// before the last has started, then waits on all of them. A child that exits
// non-zero, or whose output lacks its own result line exactly once, fails the
// test with what it printed.
func ggRunChildren(t *testing.T, children []ggChild) {
	t.Helper()
	ggRunChildrenDuring(t, children, nil)
}

// ggRunChildrenDuring is ggRunChildren with a step for the parent: during runs
// after every child is released and before the parent waits on any of them.
func ggRunChildrenDuring(t *testing.T, children []ggChild, during func()) {
	t.Helper()
	cmds := make([]*exec.Cmd, 0, len(children))
	pipes := make([]io.WriteCloser, 0, len(children))
	outs := make([]*bytes.Buffer, len(children))
	var startErr error
	for i, c := range children {
		outs[i] = new(bytes.Buffer)
		cmd := exec.Command(os.Args[0], "-test.run=^TestGgProjectmdConcurrentChild$")
		cmd.Env = os.Environ()
		for _, k := range ggEnvKeys {
			cmd.Env = append(cmd.Env, k+"="+os.Getenv(k))
		}
		cmd.Env = append(cmd.Env, ggChildMarker+"=1", ggChildName+"="+c.name, ggChildSets+"="+c.sets)
		cmd.Stdout, cmd.Stderr = outs[i], outs[i]
		in, err := cmd.StdinPipe()
		if err != nil {
			startErr = err
			break
		}
		if err := cmd.Start(); err != nil {
			startErr = err
			break
		}
		cmds, pipes = append(cmds, cmd), append(pipes, in)
	}
	for _, in := range pipes {
		in.Close()
	}
	if startErr != nil {
		for _, cmd := range cmds {
			cmd.Wait()
		}
		t.Fatal(startErr)
	}
	if during != nil {
		during()
	}
	for i, cmd := range cmds {
		err := cmd.Wait()
		var exit *exec.ExitError
		if err != nil && !errors.As(err, &exit) {
			t.Errorf("child %s: %v", children[i].name, err)
			continue
		}
		if code := cmd.ProcessState.ExitCode(); code != 0 {
			t.Errorf("child %s exited %d:\n%s", children[i].name, code, outs[i])
			continue
		}
		var results []string
		for _, line := range strings.Split(outs[i].String(), "\n") {
			if strings.HasPrefix(line, ggResultPrefix) {
				results = append(results, line)
			}
		}
		n := strings.Count(children[i].sets, ";") + 1
		if want := fmt.Sprintf("%s%s set %d keys", ggResultPrefix, children[i].name, n); !reflect.DeepEqual(results, []string{want}) {
			t.Errorf("child %s result lines %q; want [%q]", children[i].name, results, want)
		}
	}
	if t.Failed() {
		t.FailNow()
	}
}

// ggFiveKeys builds the children that each set 5 distinct keys, and the front
// matter lines the file must hold afterwards.
func ggFiveKeys(count int) (children []ggChild, lines []string) {
	for c := 0; c < count; c++ {
		var sets []string
		for j := 1; j <= 5; j++ {
			key, value := ggKV(c*5 + j)
			sets = append(sets, key+"="+value)
			lines = append(lines, key+": "+value)
		}
		children = append(children, ggChild{name: fmt.Sprint(c + 1), sets: strings.Join(sets, ";")})
	}
	return children, lines
}

// Twelve goroutines each set their own key. Each waits on one channel that is
// closed after all are started, so the writes overlap.
func TestGgProjectmdConcurrentGoroutines(t *testing.T) {
	_, home := ggSetup(t)
	r := where.Resolver{GridHome: home}
	const n = 12
	errs := make([]error, n)
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i := range errs {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			key, value := ggKV(i + 1)
			errs[i] = ggSet(r, key, value)
		}(i)
	}
	close(start)
	wg.Wait()
	var lines []string
	for i, err := range errs {
		key, value := ggKV(i + 1)
		if err != nil {
			t.Errorf("set %s: %v", key, err)
		}
		lines = append(lines, key+": "+value)
	}
	if t.Failed() {
		t.FailNow()
	}
	for i := 1; i <= n; i++ {
		key, value := ggKV(i)
		if got := ggGet(t, r, key); got != value {
			t.Errorf("%s reads %#v; want %q", key, got, value)
		}
	}
	ggCheckFile(t, home, lines)
	ggNothingBeside(t, home)
}

// Four child processes each set 5 distinct keys.
func TestGgProjectmdConcurrentProcesses(t *testing.T) {
	_, home := ggSetup(t)
	children, lines := ggFiveKeys(4)
	ggRunChildren(t, children)
	r := where.Resolver{GridHome: home}
	for _, line := range lines {
		key, value, _ := strings.Cut(line, ": ")
		if got := ggGet(t, r, key); got != value {
			t.Errorf("%s reads %#v; want %q", key, got, value)
		}
	}
	if len(lines) != 20 {
		t.Fatalf("the fixture sets %d keys; want 20", len(lines))
	}
	ggCheckFile(t, home, lines)
	ggNothingBeside(t, home)
}

// Two child processes set 5 keys each while the parent relinks the project 5
// times under the same lock. A relink rewrites the front matter from the
// project record, so it must keep every key it does not own: all 10 keys read
// back, the keys that were there before are still there, and the body is byte
// equal. The children wait on stdin until all have started; nothing sleeps.
func TestGgSetBesideRelink(t *testing.T) {
	root, home := ggSetup(t)
	linked := filepath.Join(root, "work", "linked")
	if err := os.MkdirAll(linked, 0o755); err != nil {
		t.Fatal(err)
	}
	store := projects.Store{Resolver: where.Resolver{GridHome: home, Home: os.Getenv("HOME"), Ceiling: root, TempRoots: []string{filepath.Join(root, "tmp")}}}
	children, lines := ggFiveKeys(2)
	ggRunChildrenDuring(t, children, func() {
		for i := 1; i <= 5; i++ {
			if _, err := store.Relink(ggProject, linked); err != nil {
				t.Errorf("relink %d: %v", i, err)
				return
			}
		}
	})
	r := where.Resolver{GridHome: home}
	for _, line := range lines {
		key, value, _ := strings.Cut(line, ": ")
		if got := ggGet(t, r, key); got != value {
			t.Errorf("%s reads %#v; want %q", key, got, value)
		}
	}
	for key, value := range map[string]string{"id": "gg-project", "lands": "reviewed", "owner": "someone"} {
		if got := ggGet(t, r, key); got != value {
			t.Errorf("old key %s reads %#v; want %q", key, got, value)
		}
	}
	b, err := os.ReadFile(ggPath(home))
	if err != nil {
		t.Fatal(err)
	}
	if text := string(b); !strings.Contains(text, "\nfolders:\n") || !strings.HasSuffix(text, "---\n"+ggBody) {
		t.Errorf("the relink is not in the file, or the body changed:\n%q", text)
	}
	ggNothingBeside(t, home)
}

// Two children set one key to two values, five times each. Whichever write
// is last wins, and the file still holds one such line and parses.
func TestGgProjectmdConcurrentSameKey(t *testing.T) {
	_, home := ggSetup(t)
	sets := func(value string) string {
		return strings.TrimSuffix(strings.Repeat("shared="+value+";", 5), ";")
	}
	ggRunChildren(t, []ggChild{{"a", sets("from-a")}, {"b", sets("from-b")}})
	got := ggGet(t, where.Resolver{GridHome: home}, "shared")
	if got != "from-a" && got != "from-b" {
		t.Fatalf("shared reads %#v; want from-a or from-b", got)
	}
	ggCheckFile(t, home, []string{"shared: " + got.(string)})
	ggNothingBeside(t, home)
}
