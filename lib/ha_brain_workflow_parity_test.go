package lib

import (
	"bytes"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// haBwEnviron is the one environment every call of this file takes, lib and
// binary alike: the fixture's member, the Grid home, the agent config folders
// under the fixture root, and the test process's PATH so both sides find one git.
func haBwEnviron(h home, extra ...string) []string {
	env := h.environ(
		"GRID_HOME="+h.grid,
		"CLAUDE_CONFIG_DIR="+filepath.Join(h.root, "claude-config"),
		"CODEX_HOME="+filepath.Join(h.root, "codex-home"),
		"PATH="+os.Getenv("PATH"),
	)
	return append(env, extra...)
}

// haBwResult is what a person sees from one call.
type haBwResult struct {
	code        int
	out, errout string
}

// haBwLib runs the call through lib.Run with the shared environment.
func haBwLib(h home, dir, exe string, args ...string) haBwResult {
	var out, errb bytes.Buffer
	code := Run(Call{Args: args, GridHome: h.grid, Dir: dir, Environ: haBwEnviron(h), Executable: exe, Stdout: &out, Stderr: &errb})
	return haBwResult{code, out.String(), errb.String()}
}

// haBwBin runs the same argv through the built binary with the same environment.
func haBwBin(bin string, h home, dir string, args ...string) haBwResult {
	code, out, errs := binary(bin, dir, haBwEnviron(h), args...)
	return haBwResult{code, out, errs}
}

// haBwSame runs a read through lib and the binary on one home and wants byte-equal answers.
func haBwSame(t *testing.T, bin string, h home, dir string, args ...string) haBwResult {
	t.Helper()
	l, b := haBwLib(h, dir, "", args...), haBwBin(bin, h, dir, args...)
	if l != b {
		t.Errorf("%v in %s: lib and binary differ\nlib:    exit %d out %q err %q\nbinary: exit %d out %q err %q",
			args, dir, l.code, l.out, l.errout, b.code, b.out, b.errout)
	}
	return l
}

var (
	haBwULID = regexp.MustCompile(`[0-9A-HJKMNP-TV-Z]{26}`)
	haBwTime = regexp.MustCompile(`\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(\.\d+)?(Z|[+-]\d{2}:\d{2})`)
)

// haBwNorm replaces what a second home makes differently: ULIDs, times and its root path.
func haBwNorm(h home, s string) string {
	s = strings.ReplaceAll(s, h.root, "<root>")
	s = haBwTime.ReplaceAllString(s, "<time>")
	return haBwULID.ReplaceAllString(s, "<ulid>")
}

func haBwNormed(h home, r haBwResult) haBwResult {
	return haBwResult{r.code, haBwNorm(h, r.out), haBwNorm(h, r.errout)}
}

func TestHaBwWorkflowShow(t *testing.T) {
	bin := buildBinary(t)
	h := newHome(t, "Workflow show")
	for _, dir := range []string{h.grid, h.task} {
		for _, args := range [][]string{
			{"workflow", "show", "--json", "--project", "example"},
			{"workflow", "show", "--project", "example", "--json", "--policy"},
		} {
			r := haBwSame(t, bin, h, dir, args...)
			if r.code != 0 || r.out == "" {
				t.Errorf("%v in %s: exit %d out %q err %q", args, dir, r.code, r.out, r.errout)
			}
		}
	}
}

func TestHaBwWorkflowMissing(t *testing.T) {
	bin := buildBinary(t)
	h := newHome(t, "Workflow missing")
	for _, dir := range []string{h.grid, h.task} {
		for _, args := range [][]string{
			{"workflow", "show", "--json", "--project", "nope"},
			{"workflow", "show", "--project", "nope", "--json", "--policy"},
		} {
			r := haBwSame(t, bin, h, dir, args...)
			if r.code == 0 || r.errout == "" {
				t.Errorf("%v in %s: a missing project answered exit %d err %q", args, dir, r.code, r.errout)
			}
		}
	}
}

func TestHaBwBrain(t *testing.T) {
	bin := buildBinary(t)
	h := newHome(t, "Brain")
	for _, dir := range []string{h.grid, h.task} {
		for _, args := range [][]string{
			{"brain", "changes", "--level", "home"},
			{"brain", "changes", "--level", "home", "--json"},
			{"brain", "search", "--project", "example", "--limit", "5", "--json", "--", "hello"},
		} {
			r := haBwSame(t, bin, h, dir, args...)
			if r.code != 0 {
				t.Errorf("%v in %s: exit %d err %q", args, dir, r.code, r.errout)
			}
		}
		r := haBwSame(t, bin, h, dir, "brain", "search", "--project", "nope", "--limit", "5", "--json", "--", "hello")
		if r.code == 0 || r.errout == "" {
			t.Errorf("brain search in a missing project in %s: exit %d err %q", dir, r.code, r.errout)
		}
	}
}

func TestHaBwWhere(t *testing.T) {
	bin := buildBinary(t)
	h := newHome(t, "Where")
	r := haBwSame(t, bin, h, h.task, "where", "--json", "--task", h.task)
	if r.code != 0 || !strings.Contains(r.out, "example") {
		t.Errorf("where: exit %d out %q err %q", r.code, r.out, r.errout)
	}
	// version does not name the executable, so the Call's Executable changes nothing.
	for _, args := range [][]string{{"version"}, {"version", "--json"}} {
		bare := haBwSame(t, bin, h, h.task, args...)
		withExe := haBwLib(h, h.task, bin, args...)
		if bare.code != 0 || bare != withExe {
			t.Errorf("%v: Executable unset and set differ\nunset: %+v\nset:   %+v", args, bare, withExe)
		}
		if b := haBwBin(bin, h, h.task, args...); b != withExe {
			t.Errorf("%v: Executable = bin and the binary differ\nlib:    %+v\nbinary: %+v", args, withExe, b)
		}
	}
}

const (
	haBwBase    = "# Workflows\n\n## Rate change\napplies when: label rate\nreview rounds: 2\n\n| # | step | kind | who | on reject, back to |\n|---|---|---|---|---|\n| 1 | Build | run | grid-worker | |\n"
	haBwChanged = "# Workflows\n\n## Rate change\napplies when: label rate\nreview rounds: 3\n\n| # | step | kind | who | on reject, back to |\n|---|---|---|---|---|\n| 1 | Build | run | grid-worker | |\n"
)

var haBwRevPattern = regexp.MustCompile(`"rev":"([0-9a-f]{12})"`)

// haBwProjectRev is the rev workflow show prints for the example project's file.
func haBwProjectRev(t *testing.T, r haBwResult) string {
	t.Helper()
	if r.code != 0 {
		t.Fatalf("workflow show: exit %d err %q", r.code, r.errout)
	}
	m := haBwRevPattern.FindAllStringSubmatch(r.out, -1)
	if len(m) == 0 {
		return "none"
	}
	return m[len(m)-1][1]
}

func haBwWriteFile(t *testing.T, h home, name, text string) string {
	t.Helper()
	path := filepath.Join(h.root, name)
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestHaBwWorkflowWrite(t *testing.T) {
	bin := buildBinary(t)
	show := []string{"workflow", "show", "--json", "--project", "example"}
	viaLib, viaBin := newHome(t, "Written by lib"), newHome(t, "Written by binary")
	write := func(rev, file string, h home) []string {
		return []string{"workflow", "write", "--project", "example", "--rev", rev, "--file", file, "--json"}
	}

	// Step one: the project has no workflows.md yet, so show prints no rev and
	// the write expects "none". Step two: the rev show prints is the one written back.
	for step, text := range []string{haBwBase, haBwChanged} {
		revL := haBwProjectRev(t, haBwLib(viaLib, viaLib.grid, "", show...))
		revB := haBwProjectRev(t, haBwBin(bin, viaBin, viaBin.grid, show...))
		if step == 0 && (revL != "none" || revB != "none") {
			t.Fatalf("a fresh project already has a workflow rev: lib %q binary %q", revL, revB)
		}
		fileL := haBwWriteFile(t, viaLib, "in.md", text)
		fileB := haBwWriteFile(t, viaBin, "in.md", text)
		l := haBwNormed(viaLib, haBwLib(viaLib, viaLib.grid, "", write(revL, fileL, viaLib)...))
		b := haBwNormed(viaBin, haBwBin(bin, viaBin, viaBin.grid, write(revB, fileB, viaBin)...))
		if l != b {
			t.Errorf("write %d: lib and binary differ\nlib:    %+v\nbinary: %+v", step, l, b)
		}
		if l.code != 0 || !strings.Contains(l.out, `"file":"example/workflows.md"`) {
			t.Fatalf("write %d: exit %d out %q err %q", step, l.code, l.out, l.errout)
		}
	}

	// The record each path made reads back byte-equal through the other path.
	for _, c := range []struct {
		name string
		h    home
	}{{"written by lib", viaLib}, {"written by binary", viaBin}} {
		own := haBwLib(c.h, c.h.grid, "", show...)
		other := haBwBin(bin, c.h, c.h.grid, show...)
		if c.h.root == viaBin.root {
			own, other = other, own // own is always the path that wrote it
		}
		if own != other || own.code != 0 || !strings.Contains(own.out, "review rounds: 3") {
			t.Errorf("%s: read back differs or lacks the change\nwriter path: %+v\nother path:  %+v", c.name, own, other)
		}
	}
	// The two homes hold the same file, so the normalised reads agree too.
	l := haBwNormed(viaLib, haBwBin(bin, viaLib, viaLib.grid, show...))
	b := haBwNormed(viaBin, haBwLib(viaBin, viaBin.grid, "", show...))
	if l != b {
		t.Errorf("twin homes read differently\nlib home:    %+v\nbinary home: %+v", l, b)
	}

	// A stale rev is the same refusal through both paths.
	stale := func(h home, run func(args ...string) haBwResult) haBwResult {
		return haBwNormed(h, run(write("000000000000", haBwWriteFile(t, h, "stale.md", haBwBase), h)...))
	}
	sl := stale(viaLib, func(a ...string) haBwResult { return haBwLib(viaLib, viaLib.grid, "", a...) })
	sb := stale(viaBin, func(a ...string) haBwResult { return haBwBin(bin, viaBin, viaBin.grid, a...) })
	if sl != sb || sl.code != 3 {
		t.Errorf("stale rev: lib %+v binary %+v", sl, sb)
	}
}
