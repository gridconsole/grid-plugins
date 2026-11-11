package lib

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// Parity of lib.Run and the built binary for `question` and `person` calls: the
// exit code and both streams. The binary side is the one buildBinary builds from
// this module.

// haQsResult is what one call gave.
type haQsResult struct {
	code        int
	out, errout string
}

// haQsEnviron is the one environment of every call in this file, lib and binary
// alike: the fixture's own, the Grid home, the two agent config folders under the
// fixture root and the test process's PATH so both sides find the same git. An
// extra entry "!NAME" removes NAME; any other extra entry is appended and wins.
func haQsEnviron(h home, extra ...string) []string {
	env := h.environ("GRID_HOME="+h.grid,
		"CLAUDE_CONFIG_DIR="+filepath.Join(h.root, "claude-config"),
		"CODEX_HOME="+filepath.Join(h.root, "codex-home"),
		"PATH="+os.Getenv("PATH"))
	for _, e := range extra {
		if name, ok := strings.CutPrefix(e, "!"); ok {
			kept := env[:0:0]
			for _, have := range env {
				if !strings.HasPrefix(have, name+"=") {
					kept = append(kept, have)
				}
			}
			env = kept
			continue
		}
		env = append(env, e)
	}
	return env
}

func haQsLib(h home, dir string, env []string, args ...string) haQsResult {
	code, out, errs := h.run(dir, env, args...)
	return haQsResult{code, out, errs}
}

func haQsBin(bin, dir string, env []string, args ...string) haQsResult {
	code, out, errs := binary(bin, dir, env, args...)
	return haQsResult{code, out, errs}
}

var (
	haQsULID = regexp.MustCompile(`[0-9A-HJKMNP-TV-Z]{26}`)
	haQsTime = regexp.MustCompile(`\d{4}-\d\d-\d\dT\d\d:\d\d:\d\d(?:\.\d+)?(?:Z|[+-]\d\d:\d\d)`)
	haQsQID  = regexp.MustCompile(`Q-\d+`)
)

// haQsNorm makes the output of two fresh homes comparable: the home's root path,
// then ULIDs and RFC3339 times, each become a fixed token.
func haQsNorm(s string, h home) string {
	s = strings.ReplaceAll(s, h.root, "<ROOT>")
	s = haQsTime.ReplaceAllString(s, "<TIME>")
	return haQsULID.ReplaceAllString(s, "<ULID>")
}

// haQsSame fails when the two results differ byte for byte.
func haQsSame(t *testing.T, what string, lib, bin haQsResult) {
	t.Helper()
	if lib != bin {
		t.Errorf("%s: lib and binary differ\nlib:    exit %d stdout %q stderr %q\nbinary: exit %d stdout %q stderr %q",
			what, lib.code, lib.out, lib.errout, bin.code, bin.out, bin.errout)
	}
}

// haQsSameNorm fails when exit codes differ or the normalised streams differ.
func haQsSameNorm(t *testing.T, what string, lib haQsResult, lh home, bin haQsResult, bh home) {
	t.Helper()
	l := haQsResult{lib.code, haQsNorm(lib.out, lh), haQsNorm(lib.errout, lh)}
	b := haQsResult{bin.code, haQsNorm(bin.out, bh), haQsNorm(bin.errout, bh)}
	if l != b {
		t.Errorf("%s: lib and binary differ after normalising\nlib:    exit %d stdout %q stderr %q\nbinary: exit %d stdout %q stderr %q",
			what, l.code, l.out, l.errout, b.code, b.out, b.errout)
	}
}

// haQsReadBack reads the home's questions through both paths: the two answers must
// be byte-equal, and the lib answer is returned.
func haQsReadBack(t *testing.T, bin string, h home, what string) haQsResult {
	t.Helper()
	env := haQsEnviron(h)
	lib := haQsLib(h, h.task, env, "question", "list", "--json")
	got := haQsBin(bin, h.task, env, "question", "list", "--json")
	if lib.code != 0 {
		t.Fatalf("%s: question list: exit %d %s", what, lib.code, lib.errout)
	}
	haQsSame(t, what+": question list --json", lib, got)
	return lib
}

var haQsAskArgs = []string{"question", "ask", "Which one?", "--who", "member",
	"--option", "alpha", "--option", "beta", "--recommended", "alpha",
	"--default", "alpha", "--applies", "2099-01-01T00:00:00Z", "--json"}

// haQsAsked makes twin fresh homes; the first is asked through lib, the second
// through the binary. It returns the homes and the id of the question on each.
func haQsAsked(t *testing.T, bin string) (lh, bh home, lqid, bqid string) {
	t.Helper()
	lh, bh = newHome(t, "Question parity"), newHome(t, "Question parity")
	l := haQsLib(lh, lh.task, haQsEnviron(lh), haQsAskArgs...)
	b := haQsBin(bin, bh.task, haQsEnviron(bh), haQsAskArgs...)
	if l.code != 0 || b.code != 0 {
		t.Fatalf("ask: lib %d %q %q, binary %d %q %q", l.code, l.out, l.errout, b.code, b.out, b.errout)
	}
	haQsSameNorm(t, "question ask", l, lh, b, bh)
	lqid, bqid = haQsQID.FindString(l.out), haQsQID.FindString(b.out)
	if lqid == "" || lqid != bqid {
		t.Fatalf("question ids: lib %q binary %q; lib output %q", lqid, bqid, l.out)
	}
	return lh, bh, lqid, bqid
}

// Usage on this base (internal/question/command.go:59): grid question
// add|ask|withdraw|list|tick|sync [--task <path>] [--json]; ask takes
// --recommended when there are options (internal/question/question.go:119-121).
func TestHaQsAsk(t *testing.T) {
	bin := buildBinary(t)
	lh, bh, _, _ := haQsAsked(t, bin)

	// Each question is listed through the other path: byte-equal to the same
	// path's own read, and the same question in both homes once normalised.
	lReadByBin := haQsBin(bin, lh.task, haQsEnviron(lh), "question", "list", "--json")
	lReadByLib := haQsReadBack(t, bin, lh, "asked by lib")
	haQsSame(t, "asked by lib, read by binary vs lib", lReadByLib, lReadByBin)
	bReadByLib := haQsLib(bh, bh.task, haQsEnviron(bh), "question", "list", "--json")
	bReadByBin := haQsReadBack(t, bin, bh, "asked by binary")
	haQsSame(t, "asked by binary, read by lib vs binary", bReadByLib, bReadByBin)
	if !strings.Contains(lReadByLib.out, `"alpha"`) || !strings.Contains(lReadByLib.out, `"beta"`) {
		t.Errorf("the options are not in the listed question: %s", lReadByLib.out)
	}
	haQsSameNorm(t, "listed questions of the two homes", lReadByLib, lh, bReadByBin, bh)
}

// The app's argv (internal/slices/gates/withdraw.go:114 there): question withdraw
// <qid> --task <dir> --reason <text> --json.
func TestHaQsWithdraw(t *testing.T) {
	bin := buildBinary(t)
	lh, bh, lqid, bqid := haQsAsked(t, bin)
	lArgs := []string{"question", "withdraw", lqid, "--task", lh.task, "--reason", "not needed", "--json"}
	bArgs := []string{"question", "withdraw", bqid, "--task", bh.task, "--reason", "not needed", "--json"}
	l := haQsLib(lh, lh.folder, haQsEnviron(lh), lArgs...)
	b := haQsBin(bin, bh.folder, haQsEnviron(bh), bArgs...)
	if l.code != 0 || b.code != 0 {
		t.Fatalf("withdraw: lib %d %q %q, binary %d %q %q", l.code, l.out, l.errout, b.code, b.out, b.errout)
	}
	haQsSameNorm(t, "question withdraw", l, lh, b, bh)

	lRead := haQsReadBack(t, bin, lh, "withdrawn by lib")
	bRead := haQsReadBack(t, bin, bh, "withdrawn by binary")
	haQsSameNorm(t, "listed questions after withdraw", lRead, lh, bRead, bh)
	if !strings.Contains(lRead.out, "not needed") {
		t.Errorf("the reason is not in the listed question: %s", lRead.out)
	}

	// A second withdraw of the same question: same exit code, same stderr (on this
	// base it answers the withdrawn record again, exit 0, nothing on stderr).
	l2 := haQsLib(lh, lh.folder, haQsEnviron(lh), lArgs...)
	b2 := haQsBin(bin, bh.folder, haQsEnviron(bh), bArgs...)
	if l2.code != b2.code {
		t.Errorf("second withdraw: lib exit %d (%q), binary exit %d (%q)", l2.code, l2.errout, b2.code, b2.errout)
	}
	if l2.errout != b2.errout {
		t.Errorf("second withdraw stderr differs\nlib:    %q\nbinary: %q", l2.errout, b2.errout)
	}
	haQsSameNorm(t, "second withdraw stdout", l2, lh, b2, bh)
}

// The operand form on this base (internal/cli/question.go:16 to question.RunAnswer,
// internal/question/command.go:45-54): question answer <task> Q-N <text>.
func TestHaQsAnswer(t *testing.T) {
	bin := buildBinary(t)
	lh, bh, lqid, bqid := haQsAsked(t, bin)
	l := haQsLib(lh, lh.folder, haQsEnviron(lh), "question", "answer", lh.task, lqid, "alpha", "--json")
	b := haQsBin(bin, bh.folder, haQsEnviron(bh), "question", "answer", bh.task, bqid, "alpha", "--json")
	if l.code != 0 || b.code != 0 {
		t.Fatalf("answer: lib %d %q %q, binary %d %q %q", l.code, l.out, l.errout, b.code, b.out, b.errout)
	}
	haQsSameNorm(t, "question answer", l, lh, b, bh)

	lRead := haQsReadBack(t, bin, lh, "answered by lib")
	bRead := haQsReadBack(t, bin, bh, "answered by binary")
	haQsSameNorm(t, "listed questions after answer", lRead, lh, bRead, bh)
	if lRead.out == "" || !strings.Contains(lRead.out, "alpha") {
		t.Errorf("the answer is not in the listed question: %s", lRead.out)
	}
}

// A guest's withdraw is refused before any project is read (internal/cli/cli.go:197-203).
func TestHaQsGuest(t *testing.T) {
	bin := buildBinary(t)
	h := newHome(t, "Guest withdraw")
	env := haQsEnviron(h, "GRID_ROLE=guest")
	args := []string{"question", "withdraw", "Q-1", "--task", h.task, "--reason", "not needed", "--json"}
	l := haQsLib(h, h.folder, env, args...)
	b := haQsBin(bin, h.folder, env, args...)
	if l.code != 3 || b.code != 3 {
		t.Fatalf("guest withdraw: lib exit %d %q, binary exit %d %q; want 3 on both", l.code, l.errout, b.code, b.errout)
	}
	haQsSame(t, "guest withdraw", l, b)
	if !strings.HasPrefix(l.errout, "question: ") || !strings.HasPrefix(b.errout, "question: ") {
		t.Errorf("stderr should begin with \"question: \": lib %q binary %q", l.errout, b.errout)
	}
}

func TestHaQsPerson(t *testing.T) {
	bin := buildBinary(t)
	h := newHome(t, "Person parity")
	cases := []struct {
		name  string
		env   []string
		args  []string
		exit  int
		wantS string // the failed stdout a person verb must carry
	}{
		{"check owner", haQsEnviron(h), []string{"person", "check", "--json"}, 0, `"ok":true`},
		{"list owner", haQsEnviron(h), []string{"person", "list", "--json"}, 0, `"people"`},
		{"approvers owner", haQsEnviron(h), []string{"person", "approvers", "--area", "code"}, 2, ""},
		{"approvers owner with task", haQsEnviron(h), []string{"person", "approvers", "--area", "code", "--task", h.task, "--json"}, -1, ""},
		// A guest and a caller with no member are still people: exit 0.
		{"check guest", haQsEnviron(h, "GRID_ROLE=guest"), []string{"person", "check", "--json"}, 0, `"ok":true`},
		{"check no member", haQsEnviron(h, "!GRID_MEMBER"), []string{"person", "check", "--json"}, 0, `"ok":true`},
		// An agent role and a worker marker are refused: exit 3 with the object on stdout.
		{"check agent role", haQsEnviron(h, "GRID_ROLE=builder"), []string{"person", "check", "--json"}, 3, "agent_role"},
		{"check worker", haQsEnviron(h, "GRID_WORKER=member@01M752MV8YGPV38630XR6YEVTV"), []string{"person", "check", "--json"}, 3, "agent_worker"},
		{"check agent role, text", haQsEnviron(h, "GRID_ROLE=builder"), []string{"person", "check"}, 3, ""},
	}
	for _, c := range cases {
		l := haQsLib(h, h.folder, c.env, c.args...)
		b := haQsBin(bin, h.folder, c.env, c.args...)
		haQsSame(t, c.name, l, b)
		if c.exit >= 0 && l.code != c.exit {
			t.Errorf("%s: exit %d, want %d; stdout %q stderr %q", c.name, l.code, c.exit, l.out, l.errout)
		}
		if !strings.Contains(l.out, c.wantS) {
			t.Errorf("%s: stdout %q lacks %q", c.name, l.out, c.wantS)
		}
	}
}
