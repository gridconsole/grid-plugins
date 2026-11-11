package braincmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// riWant fails unless the run exited with code and stderr equals want.
func riWant(t *testing.T, h *harness, code int, want string, args ...string) {
	t.Helper()
	if got := h.run(args...); got != code {
		t.Fatalf("%v: exit %d, want %d; stdout %q stderr %q", args, got, code, h.out.String(), h.err.String())
	}
	if h.err.String() != want {
		t.Fatalf("%v: stderr %q, want %q", args, h.err.String(), want)
	}
}

// riProposeArgs is the propose call of the harness helper, without --body.
func riProposeArgs(extra ...string) []string {
	args := []string{"propose", "--level", "home", "--name", "checks", "--topic", "decisions", "--description", "Before changing checks.", "--summary", "Run the checks.", "--source", "job:T-01/J-01"}
	return append(args, extra...)
}

func TestRILinkFlagRejectsNonJSON(t *testing.T) {
	h := newHarness(t)
	args := []string{"propose", "--level", "home", "--link", "nope"}
	if code := h.run(args...); code != 2 {
		t.Fatalf("exit %d, want 2: %q", code, &h.err)
	}
	if want := `invalid value "nope" for flag -link: link must be a JSON object`; !strings.Contains(h.err.String(), want) {
		t.Fatalf("stderr %q lacks %q", &h.err, want)
	}
	if entries, err := os.ReadDir(h.env.GridHome); err == nil && len(entries) != 0 {
		t.Fatalf("Grid home holds %d entries after a refused link", len(entries))
	}
}

func TestRILinkFlagRejectsUnknownField(t *testing.T) {
	h := newHarness(t)
	if code := h.run("propose", "--level", "home", "--link", `{"to":"brain:home/x","bogus":1}`); code != 2 {
		t.Fatalf("exit %d, want 2: %q", code, &h.err)
	}
	if want := `json: unknown field "bogus"`; !strings.Contains(h.err.String(), want) {
		t.Fatalf("stderr %q lacks %q", &h.err, want)
	}
}

func TestRIProposeHelpPrintsUsageOnStdout(t *testing.T) {
	h := newHarness(t)
	if code := h.run("propose", "-h"); code != 0 {
		t.Fatalf("exit %d, want 0: %q", code, &h.err)
	}
	if h.err.Len() != 0 {
		t.Fatalf("stderr %q, want empty", &h.err)
	}
	if h.out.String() != usage {
		t.Fatalf("stdout %q, want the general usage", &h.out)
	}
}

func TestRILevelHomeWithTaskIsAUsageError(t *testing.T) {
	h := newHarness(t)
	riWant(t, h, 2, "brain show: --level home cannot be combined with --task. Run: grid brain help\n", "show", "x", "--level", "home", "--task", "T-01")
}

func TestRINoGridHomeFails(t *testing.T) {
	h := newHarness(t)
	h.env.GridHome = ""
	riWant(t, h, 1, "brain: cannot find the Grid home; set GRID_HOME\n", "search", "x")
}

func TestRIProjectLevelWithoutProjectFails(t *testing.T) {
	h := newHarness(t)
	riWant(t, h, 1, "brain: project level needs a project; select --project or use --level home\n", "search", "x", "--level", "project")
}

func TestRIInvalidBrainID(t *testing.T) {
	h := newHarness(t)
	riWant(t, h, 1, "brain: invalid brain id\n", "show", "brain:bad/x")
}

func TestRIBrainIDLevelDisagreesWithLevel(t *testing.T) {
	h := newHarness(t)
	riWant(t, h, 1, "brain: id level disagrees with --level\n", "show", "brain:project/x", "--level", "home")
}

func TestRIProposeFileWithoutNameAndTopic(t *testing.T) {
	h := newHarness(t)
	path := filepath.Join(t.TempDir(), "note.md")
	if err := os.WriteFile(path, []byte("---\nsummary: s\n---\nbody\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	riWant(t, h, 1, "brain: name and topic must be plain names\n", "propose", "--level", "home", "--file", path)
	riWant(t, h, 1, `{"error":"name and topic must be plain names"}`+"\n", "propose", "--level", "home", "--file", path, "--json")
}

func TestRIProposeFileMissing(t *testing.T) {
	h := newHarness(t)
	path := filepath.Join(t.TempDir(), "missing.md")
	if code := h.run("propose", "--level", "home", "--file", path); code != 1 {
		t.Fatalf("exit %d, want 1: %q", code, &h.err)
	}
	if want := "brain: open " + path; !strings.HasPrefix(h.err.String(), want) {
		t.Fatalf("stderr %q does not start with %q", &h.err, want)
	}
}

func TestRIStdinOverOneMiBFails(t *testing.T) {
	h := newHarness(t)
	h.env.Stdin = strings.NewReader(strings.Repeat("a", 1<<20+2))
	riWant(t, h, 1, `{"error":"input exceeds 1 MiB"}`+"\n", riProposeArgs("--json")...)
	h.env.Stdin = strings.NewReader(strings.Repeat("a", 1<<20+2))
	riWant(t, h, 1, "brain: input exceeds 1 MiB\n", riProposeArgs()...)
}

func TestRINilStdinProposesAnEmptyBody(t *testing.T) {
	h := newHarness(t)
	h.env.Stdin = nil
	if code := h.run(riProposeArgs("--json")...); code != 0 {
		t.Fatalf("exit %d, want 0: %q", code, &h.err)
	}
	if !strings.Contains(h.out.String(), `"body":""`) {
		t.Fatalf("stdout %q lacks an empty body", &h.out)
	}
}

func TestRIDoubleDashKeepsLeadingDashWords(t *testing.T) {
	h := newHarness(t)
	if code := h.run("search", "--level", "home", "--", "-words"); code != 0 {
		t.Fatalf("exit %d, want 0: %q", code, &h.err)
	}
	if want := "hits: []\nquery: -words\n"; h.out.String() != want {
		t.Fatalf("stdout %q, want %q", &h.out, want)
	}
}

func TestRIClosedStdoutFailsSearchAndIndex(t *testing.T) {
	h := newHarness(t)
	h.env.Stdout = failingWriter{}
	riWant(t, h, 1, `{"error":"stdout closed"}`+"\n", "search", "x", "--level", "home", "--json")
	riWant(t, h, 1, "brain: stdout closed\n", "search", "x", "--level", "home")
	riWant(t, h, 1, "brain: stdout closed\n", "index", "--level", "home")
}
