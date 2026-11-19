package braincmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Failure paths of brain changes, revert and supersede: the store lookup per
// level, flag parsing and help, the log-file check, a closed stdout, and a
// supersede that wrote but could not rebuild the index.

// vlFail runs one command and asserts its exit code and exact stderr.
func vlFail(t *testing.T, h *harness, wantCode int, wantErr string, args ...string) {
	t.Helper()
	if code := h.run(args...); code != wantCode || h.err.String() != wantErr {
		t.Fatalf("%v: exit %d stderr %q, want exit %d stderr %q (stdout %q)", args, code, &h.err, wantCode, wantErr, &h.out)
	}
}

// vlBreakIndex replaces knowledge/index.md with a folder holding a file, so the
// index rebuild cannot replace it.
func vlBreakIndex(t *testing.T, h *harness) {
	t.Helper()
	idx := filepath.Join(h.env.GridHome, "knowledge", "index.md")
	if err := os.RemoveAll(idx); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(idx, "keep"), 0o700); err != nil {
		t.Fatal(err)
	}
}

func TestVLNoGridHome(t *testing.T) {
	const want = "brain: cannot find the Grid home; set GRID_HOME\n"
	for _, args := range [][]string{
		{"changes"},
		{"revert", "decisions/checks", "--to", "1"},
		{"supersede", "a", "--by", "b", "--reason", svReason},
	} {
		t.Run(args[0], func(t *testing.T) {
			h := newHarness(t)
			h.env.GridHome = ""
			vlFail(t, h, 1, want, args...)
		})
	}
}

func TestVLUnknownProject(t *testing.T) {
	h := newHarness(t)
	vlFail(t, h, 1, "brain: project \"nosuch\" is not registered\n", "changes", "--project", "nosuch")
}

func TestVLRevertFromAFolderWithNoGridRecord(t *testing.T) {
	h := newHarness(t)
	h.env.Cwd = filepath.Join(h.env.Home, "work")
	if err := os.MkdirAll(h.env.Cwd, 0o700); err != nil {
		t.Fatal(err)
	}
	vlFail(t, h, 1, "brain: project \"work\" has no Grid record; create its first task, or use --level home only for knowledge that holds in every project\n",
		"revert", "decisions/checks", "--to", "1")
}

func TestVLProjectLevelWithoutAProject(t *testing.T) {
	h := newHarness(t)
	vlFail(t, h, 1, "brain: project level needs a project; select --project or use --level home\n", "changes", "--level", "project")
}

func TestVLHelpPrintsUsageOnStdoutAndExitsZero(t *testing.T) {
	for _, c := range []struct{ verb, usage string }{
		{"changes", changesUsage},
		{"supersede", supersedeUsage},
	} {
		t.Run(c.verb, func(t *testing.T) {
			h := newHarness(t)
			if code := h.run(c.verb, "-h"); code != 0 || !strings.HasPrefix(h.out.String(), c.usage+"\n") {
				t.Fatalf("%s -h: exit %d stdout %q, want exit 0 and the usage", c.verb, code, &h.out)
			}
		})
	}
}

func TestVLSupersedeRefusesAnUnknownLevel(t *testing.T) {
	h := newHarness(t)
	vlFail(t, h, 2, "brain supersede: level must be home or project. Run: grid brain help\n",
		"supersede", "a", "--by", "b", "--reason", svReason, "--level", "bogus")
}

func TestVLChangesRefusesALogThatIsNotAFile(t *testing.T) {
	h := newHarness(t)
	h.threeVersions(t)
	log := filepath.Join(h.env.GridHome, "knowledge", "log.md")
	if err := os.RemoveAll(log); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(log, 0o700); err != nil {
		t.Fatal(err)
	}
	vlFail(t, h, 1, "brain: brain log must be a regular file\n", "changes", "--level", "home")
}

func TestVLClosedStdoutOnChangesAndRevert(t *testing.T) {
	for _, c := range []struct {
		name, want string
		args       []string
	}{
		{"changes json", `{"error":"stdout closed"}` + "\n", []string{"changes", "--level", "home", "--json"}},
		{"changes", "brain: stdout closed\n", []string{"changes", "--level", "home"}},
		{"revert", "brain: stdout closed\n", []string{"revert", "decisions/checks", "--to", "1", "--level", "home"}},
	} {
		t.Run(c.name, func(t *testing.T) {
			h := newHarness(t)
			h.threeVersions(t)
			h.env.Stdout = failingWriter{}
			vlFail(t, h, 1, c.want, c.args...)
		})
	}
}

func TestVLClosedStdoutOnSupersede(t *testing.T) {
	for _, c := range []struct{ name, want, flag string }{
		{"json", `{"error":"stdout closed"}` + "\n", "--json"},
		{"text", "brain: stdout closed\n", ""},
	} {
		t.Run(c.name, func(t *testing.T) {
			h := newHarness(t)
			h.svNotes(t, "c1", "c2")
			h.env.Stdout = failingWriter{}
			args := []string{"supersede", "brain:home/c1", "--by", "brain:home/c2", "--reason", svReason, "--level", "home"}
			if c.flag != "" {
				args = append(args, c.flag)
			}
			vlFail(t, h, 1, c.want, args...)
		})
	}
}

func TestVLSupersedeReportsTheWriteWhenTheIndexRebuildFails(t *testing.T) {
	args := []string{"supersede", "brain:home/c1", "--by", "brain:home/c2", "--reason", svReason, "--level", "home"}

	t.Run("json", func(t *testing.T) {
		h := newHarness(t)
		h.svNotes(t, "c1", "c2")
		vlBreakIndex(t, h)
		if code := h.run(append(args, "--json")...); code != 1 {
			t.Fatalf("exit %d, want 1: %q %q", code, &h.out, &h.err)
		}
		var res struct {
			Wrote bool `json:"wrote"`
		}
		if err := json.Unmarshal(h.out.Bytes(), &res); err != nil || !res.Wrote {
			t.Fatalf("stdout %q (%v), want JSON with wrote true", &h.out, err)
		}
		var e struct {
			Error string `json:"error"`
		}
		if err := json.Unmarshal(h.err.Bytes(), &e); err != nil ||
			!strings.Contains(e.Error, "is superseded by") || !strings.Contains(e.Error, "but rebuilding the index failed") {
			t.Fatalf("stderr %q (%v)", &h.err, err)
		}
	})

	t.Run("text", func(t *testing.T) {
		h := newHarness(t)
		h.svNotes(t, "c1", "c2")
		vlBreakIndex(t, h)
		if code := h.run(args...); code != 1 {
			t.Fatalf("exit %d, want 1: %q %q", code, &h.out, &h.err)
		}
		if h.out.String() != "brain:home/c1 superseded by brain:home/c2 (v2, v2)\n" {
			t.Fatalf("stdout %q", &h.out)
		}
		if !strings.HasPrefix(h.err.String(), "brain: ") || !strings.Contains(h.err.String(), "but rebuilding the index failed") {
			t.Fatalf("stderr %q", &h.err)
		}
	})
}
