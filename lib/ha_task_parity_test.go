package lib

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// haTkEnviron is the one environment every call in this file takes, lib and binary
// alike: the fixture's member, the Grid home, private Claude and Codex folders under
// the home's root, and the test process's PATH so both sides find the same git.
func haTkEnviron(h home, extra ...string) []string {
	env := h.environ(
		"GRID_HOME="+h.grid,
		"CLAUDE_CONFIG_DIR="+filepath.Join(h.root, "claude"),
		"CODEX_HOME="+filepath.Join(h.root, "codex"),
		"PATH="+os.Getenv("PATH"),
	)
	return append(env, extra...)
}

// haTkResult is what one call showed.
type haTkResult struct {
	code        int
	out, errout string
}

// haTkSide makes calls on one home, through lib.Run or through the built binary.
type haTkSide struct {
	name string
	h    home
	bin  string // empty: through lib.Run
}

func (s haTkSide) call(dir string, environ []string, args ...string) haTkResult {
	var r haTkResult
	if s.bin == "" {
		r.code, r.out, r.errout = s.h.run(dir, environ, args...)
	} else {
		r.code, r.out, r.errout = binary(s.bin, dir, environ, args...)
	}
	return r
}

// run is a call in the project folder with the common environment.
func (s haTkSide) run(args ...string) haTkResult {
	return s.call(s.h.folder, haTkEnviron(s.h), args...)
}

var (
	haTkULID = regexp.MustCompile(`[0-9A-HJKMNP-TV-Z]{26}`)
	haTkTime = regexp.MustCompile(`\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(\.\d+)?(Z|[+-]\d{2}:\d{2})`)
)

// haTkNorm replaces what a second home legitimately makes different: its root path,
// ULIDs and RFC3339 times.
func haTkNorm(h home, s string) string {
	s = strings.ReplaceAll(s, h.root, "<ROOT>")
	s = haTkTime.ReplaceAllString(s, "<TIME>")
	return haTkULID.ReplaceAllString(s, "<ULID>")
}

func (r haTkResult) norm(h home) haTkResult {
	return haTkResult{r.code, haTkNorm(h, r.out), haTkNorm(h, r.errout)}
}

// haTkSame fails unless both results are byte-equal in exit code, stdout and stderr.
func haTkSame(t *testing.T, what string, lib, bin haTkResult) {
	t.Helper()
	if lib.code != bin.code || lib.out != bin.out || lib.errout != bin.errout {
		t.Errorf("%s: lib and binary differ\nlib:    exit %d stdout %q stderr %q\nbinary: exit %d stdout %q stderr %q",
			what, lib.code, lib.out, lib.errout, bin.code, bin.out, bin.errout)
		return
	}
	t.Logf("%s: equal, exit %d, stdout %d bytes, stderr %d bytes", what, lib.code, len(lib.out), len(lib.errout))
}

// haTkTwins makes a fresh home for lib and another for the binary.
func haTkTwins(t *testing.T) (lib, bin haTkSide) {
	t.Helper()
	exe := buildBinary(t)
	return haTkSide{name: "lib", h: newHome(t, "First")}, haTkSide{name: "binary", h: newHome(t, "First"), bin: exe}
}

// haTkTaskID reads the task's id from its own record.
func haTkTaskID(t *testing.T, s haTkSide) string {
	t.Helper()
	r := s.run("task", "show", s.h.task, "--json")
	var got struct{ ID string }
	if r.code != 0 || json.Unmarshal([]byte(r.out), &got) != nil || got.ID == "" {
		t.Fatalf("%s task show: exit %d stdout %q stderr %q", s.name, r.code, r.out, r.errout)
	}
	return got.ID
}

func haTkOK(t *testing.T, s haTkSide, r haTkResult, args ...string) {
	t.Helper()
	if r.code != 0 {
		t.Fatalf("%s %v: exit %d stdout %q stderr %q", s.name, args, r.code, r.out, r.errout)
	}
}

func TestHaTkReads(t *testing.T) {
	exe := buildBinary(t)
	h := newHome(t, "Read through both")
	viaLib, viaBin := haTkSide{name: "lib", h: h}, haTkSide{name: "binary", h: h, bin: exe}
	id := haTkTaskID(t, viaLib)
	for _, args := range [][]string{
		{"task", "show", h.task, "--json"},
		{"task", "list", "--json"},
		{"lease", "status", "task", "--task", id, "--json"},
	} {
		l, b := viaLib.run(args...), viaBin.run(args...)
		haTkOK(t, viaLib, l, args...)
		if l.out == "" {
			t.Errorf("%v: empty answer", args)
		}
		haTkSame(t, strings.Join(args, " "), l, b)
	}
	for _, args := range [][]string{{"task", "--help"}, {"approve", "--help"}} {
		l, b := viaLib.run(args...), viaBin.run(args...)
		haTkSame(t, strings.Join(args, " "), l, b)
		// taskfolders.go reads both streams as one text.
		if lj, bj := l.out+l.errout, b.out+b.errout; lj != bj || !strings.Contains(strings.ToLower(lj), "usage") {
			t.Errorf("%v: joined help differs or is not a usage text\nlib:    %q\nbinary: %q", args, lj, bj)
		}
	}
}

func TestHaTkNew(t *testing.T) {
	lib, bin := haTkTwins(t)
	args := []string{"task", "new", "Second", "--project", "example", "--json"}
	l, b := lib.run(args...), bin.run(args...)
	haTkOK(t, lib, l, args...)
	haTkOK(t, bin, b, args...)
	haTkSame(t, "task new normalised", l.norm(lib.h), b.norm(bin.h))
	paths := map[string]string{}
	for _, c := range []struct {
		s haTkSide
		r haTkResult
	}{{lib, l}, {bin, b}} {
		var made struct{ ID, Path string }
		if err := json.Unmarshal([]byte(c.r.out), &made); err != nil || made.Path == "" {
			t.Fatalf("%s task new output: %v %q", c.s.name, err, c.r.out)
		}
		paths[c.s.name] = made.Path
	}
	// Each record read back through the other path is the same bytes as the writer's own read.
	for _, c := range []struct{ writer, reader haTkSide }{{lib, bin}, {bin, lib}} {
		read := []string{"task", "show", paths[c.writer.name], "--json"}
		own := c.writer.run(read...)
		other := haTkSide{name: c.reader.name, h: c.writer.h, bin: c.reader.bin}.run(read...)
		haTkOK(t, c.writer, own, read...)
		haTkSame(t, "readback of the "+c.writer.name+" task through the "+c.reader.name, other, own)
	}
}

func TestHaTkMoveStopReopen(t *testing.T) {
	lib, bin := haTkTwins(t)
	id := haTkTaskID(t, lib)
	if bid := haTkTaskID(t, bin); bid != id {
		t.Fatalf("twin task ids differ: %s %s", id, bid)
	}
	// The app's own argv for a move (internal/slices/tasks/intent.go: task move --to <target>
	// --project --task --who --json); on this base the target is a machine, not a state.
	for _, args := range [][]string{
		{"task", "stop", id},
		{"task", "reopen", id},
		{"task", "move", "--to", "other-machine", "--project", "example", "--task", id, "--who", "app", "--json"},
	} {
		l, b := lib.run(args...), bin.run(args...)
		haTkOK(t, lib, l, args...)
		haTkSame(t, strings.Join(args, " "), l.norm(lib.h), b.norm(bin.h))
	}
	read := []string{"task", "show", id, "--json"}
	l, b := lib.run(read...), bin.run(read...)
	haTkOK(t, lib, l, read...)
	haTkSame(t, "task show after the sequence", l.norm(lib.h), b.norm(bin.h))
	// The record each side wrote reads the same through the other path.
	for _, c := range []struct{ writer, reader haTkSide }{{lib, bin}, {bin, lib}} {
		other := haTkSide{name: c.reader.name, h: c.writer.h, bin: c.reader.bin}.run(read...)
		haTkSame(t, "readback of the "+c.writer.name+" record through the "+c.reader.name, other, c.writer.run(read...))
	}
}

func TestHaTkTrash(t *testing.T) {
	lib, bin := haTkTwins(t)
	id := haTkTaskID(t, lib)
	// The app's argv: trash.go task delete <id> --project <p> --json, and task restore <id> --project <p> --json.
	for _, args := range [][]string{
		{"task", "delete", id, "--project", "example", "--json"},
		{"task", "restore", id, "--project", "example", "--json"},
	} {
		l, b := lib.run(args...), bin.run(args...)
		haTkOK(t, lib, l, args...)
		haTkSame(t, strings.Join(args, " "), l.norm(lib.h), b.norm(bin.h))
	}
	read := []string{"task", "list", "--json"}
	for _, c := range []struct{ writer, reader haTkSide }{{lib, bin}, {bin, lib}} {
		other := haTkSide{name: c.reader.name, h: c.writer.h, bin: c.reader.bin}.run(read...)
		own := c.writer.run(read...)
		haTkOK(t, c.writer, own, read...)
		if !strings.Contains(own.out, id) {
			t.Errorf("%s: the restored task is not listed: %s", c.writer.name, own.out)
		}
		haTkSame(t, "list after restore, written by "+c.writer.name+" read through "+c.reader.name, other, own)
	}
}

func TestHaTkRefusals(t *testing.T) {
	exe := buildBinary(t)
	h := newHome(t, "Refusals")
	viaLib, viaBin := haTkSide{name: "lib", h: h}, haTkSide{name: "binary", h: h, bin: exe}
	id := haTkTaskID(t, viaLib)
	guest := haTkEnviron(h, "GRID_ROLE=guest")
	for _, c := range []struct {
		name    string
		environ []string
		args    []string
		want    int
	}{
		{"missing task", haTkEnviron(h), []string{"task", "show", "T-99"}, 1},
		{"move without operand", haTkEnviron(h), []string{"task", "move"}, 2},
		{"guest delete", guest, []string{"task", "delete", id}, 3},
		{"guest stop", guest, []string{"task", "stop", id}, 1},
	} {
		l, b := viaLib.call(h.folder, c.environ, c.args...), viaBin.call(h.folder, c.environ, c.args...)
		if l.code != c.want {
			t.Errorf("%s: lib exit %d, want %d; stderr %q", c.name, l.code, c.want, l.errout)
		}
		if l.code != b.code || l.errout != b.errout {
			t.Errorf("%s: lib and binary differ\nlib:    exit %d stderr %q\nbinary: exit %d stderr %q", c.name, l.code, l.errout, b.code, b.errout)
		}
	}
}
