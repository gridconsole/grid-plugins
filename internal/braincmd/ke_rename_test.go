package braincmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/brain"
)

// keRnCmd is a Grid home with home-level notes in the decisions topic: a, and c
// linking a with an anchor.
type keRnCmd struct {
	t    *testing.T
	home string
	gh   string
	st   brain.Store
}

func keRnCmdNew(t *testing.T) *keRnCmd {
	t.Helper()
	home := t.TempDir()
	c := &keRnCmd{t: t, home: home, gh: filepath.Join(home, ".grid")}
	c.st = brain.Store{GridHome: c.gh, ReadLevel: "home"}
	c.accept("a", nil)
	c.accept("c", []brain.Link{{To: "brain:home/a#why", Rel: "related", Summary: "The note this one points at.", Why: "Shows why the choice was made."}})
	return c
}

func (c *keRnCmd) accept(name string, links []brain.Link) {
	c.t.Helper()
	if links == nil {
		links = []brain.Link{}
	}
	n := brain.Note{Name: name, Topic: "decisions", Type: "reference", Description: "Read before changing " + name + ".",
		Summary: "Summary of " + name + ".", Source: "job:T-01/J-01", Level: "home", Status: "proposed",
		Origin: brain.Origin{Kind: "job", ID: "T-01/J-01"}, Links: links, Body: "# " + name + "\n\nBody.\n"}
	if _, err := c.st.Propose(n, n.Origin); err != nil {
		c.t.Fatal(err)
	}
	if _, err := c.st.Accept("home", name); err != nil {
		c.t.Fatal(err)
	}
}

func (c *keRnCmd) path(rel string) string { return filepath.Join(c.gh, filepath.FromSlash(rel)) }

func (c *keRnCmd) read(rel string) string {
	c.t.Helper()
	b, err := os.ReadFile(c.path(rel))
	if err != nil {
		c.t.Fatal(err)
	}
	return string(b)
}

func (c *keRnCmd) rev(name string) string {
	return brain.NoteRev([]byte(c.read("knowledge/decisions/" + name + ".md")))
}

// run calls runRename with the Resolver set as Run sets it.
func (c *keRnCmd) run(role, member string, args ...string) (int, string, string) {
	c.t.Helper()
	var out, errb bytes.Buffer
	env := Env{Stdout: &out, Stderr: &errb, Cwd: c.home, Home: c.home, GridHome: c.gh, Role: role, Member: member}
	env.Resolver.GridHome = env.GridHome
	env.Resolver.Home = env.Home
	env.Resolver.TempRoots = env.TempRoots
	return runRename(args, env), out.String(), errb.String()
}

func TestKeRnCommand(t *testing.T) {
	t.Run("usage", func(t *testing.T) {
		c := keRnCmdNew(t)
		rev := c.rev("a")
		for name, args := range map[string][]string{
			"one operand":       {"--level", "home", "--expect", rev, "decisions/a"},
			"no topic":          {"--level", "home", "--expect", rev, "a", "b"},
			"three-part path":   {"--level", "home", "--expect", rev, "decisions/a/x", "b"},
			"bad new name":      {"--level", "home", "--expect", rev, "decisions/a", "b/c"},
			"by flag":           {"--level", "home", "--expect", rev, "--by", "ana", "decisions/a", "b"},
			"missing expect":    {"--level", "home", "decisions/a", "b"},
			"three operands":    {"--level", "home", "--expect", rev, "decisions/a", "b", "c"},
			"home with project": {"--level", "home", "--project", "x", "--expect", rev, "decisions/a", "b"},
		} {
			if code, out, errb := c.run("owner", "", args...); code != 2 {
				t.Errorf("%s: exit %d, want 2 (stdout %q, stderr %q)", name, code, out, errb)
			}
		}
		if _, err := os.Stat(c.path("knowledge/decisions/a.md")); err != nil {
			t.Fatalf("a.md is gone after usage errors: %v", err)
		}
	})

	t.Run("roles refused", func(t *testing.T) {
		for _, role := range []string{"guest", "worker"} {
			c := keRnCmdNew(t)
			code, _, errb := c.run(role, "", "--level", "home", "--expect", c.rev("a"), "decisions/a", "b")
			want := "grid brain rename: caller role is not allowed to rename brain entries\n"
			if code != 3 || errb != want {
				t.Errorf("role %s: exit %d, stderr %q, want 3 and %q", role, code, errb, want)
			}
			if _, err := os.Stat(c.path("knowledge/decisions/a.md")); err != nil {
				t.Errorf("role %s: a.md is gone: %v", role, err)
			}
		}
	})

	t.Run("failures", func(t *testing.T) {
		c := keRnCmdNew(t)
		rev := c.rev("a")
		code, _, errb := c.run("owner", "", "--json", "--level", "home", "--expect", "0000000000000000", "decisions/a", "b")
		if code != 6 || errb != `{"error":"conflict","rev":"`+rev+`"}`+"\n" {
			t.Errorf("stale rev: exit %d, stderr %q", code, errb)
		}
		if code, _, errb = c.run("owner", "", "--json", "--level", "home", "--expect", rev, "machines/x", "y"); code != 7 || !strings.HasPrefix(errb, `{"error":"not_versioned"`) {
			t.Errorf("machines/x: exit %d, stderr %q", code, errb)
		}
		if code, _, errb = c.run("owner", "", "--json", "--level", "home", "--expect", rev, "decisions/nope", "b"); code != 8 || errb != `{"error":"no_such_note"}`+"\n" {
			t.Errorf("missing note: exit %d, stderr %q", code, errb)
		}
		c.accept("x", nil)
		if _, err := c.st.Drop("home", "decisions", "x", brain.OpDrop, c.rev("x"), brain.Origin{Kind: "person", ID: "ana"}, ""); err != nil {
			t.Fatal(err)
		}
		if code, _, errb = c.run("owner", "", "--json", "--level", "home", "--expect", c.rev("x"), "decisions/x", "y"); code != 9 || !strings.HasPrefix(errb, `{"error":"refused","reason":"note is dropped"`) {
			t.Errorf("dropped note: exit %d, stderr %q", code, errb)
		}
		if _, err := os.Stat(c.path("knowledge/decisions/a.md")); err != nil {
			t.Fatalf("a.md is gone after refusals: %v", err)
		}
	})

	t.Run("roles allowed", func(t *testing.T) {
		for _, role := range []string{"owner", "member", ""} {
			c := keRnCmdNew(t)
			code, out, errb := c.run(role, "", "--level", "home", "--expect", c.rev("a"), "decisions/a", "b")
			if code != 0 || !strings.HasPrefix(out, "renamed decisions/a -> decisions/b v1 rev ") || !strings.Contains(out, "linked decisions/c\n") {
				t.Errorf("role %q: exit %d, stdout %q, stderr %q", role, code, out, errb)
			}
			if !strings.Contains(c.read("knowledge/decisions/c.md"), "brain:home/b#why") {
				t.Errorf("role %q: c is not relinked", role)
			}
		}
	})

	t.Run("json result", func(t *testing.T) {
		c := keRnCmdNew(t)
		code, out, errb := c.run("owner", "", "--json", "--level", "home", "--expect", c.rev("a"), "--reason", "clearer", "decisions/a", "b")
		if code != 0 || !strings.HasPrefix(out, `{"status":"renamed","from":"decisions/a","to":"decisions/b","version":1,"rev":"`) ||
			!strings.HasSuffix(out, `"linked":["decisions/c"],"kept":[],"skipped":[]}`+"\n") {
			t.Errorf("exit %d, stdout %q, stderr %q", code, out, errb)
		}
	})

	t.Run("origin", func(t *testing.T) {
		c := keRnCmdNew(t)
		if code, _, errb := c.run("owner", "bo", "--level", "home", "--expect", c.rev("a"), "decisions/a", "b"); code != 0 {
			t.Fatalf("exit %d, stderr %q", code, errb)
		}
		log := c.read("knowledge/log.md")
		for _, want := range []string{"| renamed | b | person:bo |", "| linked | c | person:bo |"} {
			if !strings.Contains(log, want) {
				t.Errorf("log has no %q:\n%s", want, log)
			}
		}
		s := keRnCmdNew(t)
		if code, _, errb := s.run("owner", "", "--level", "home", "--expect", s.rev("a"), "decisions/a", "b"); code != 0 {
			t.Fatalf("exit %d, stderr %q", code, errb)
		}
		log = s.read("knowledge/log.md")
		for _, want := range []string{"| renamed | b | session:", "| linked | c | session:"} {
			if !strings.Contains(log, want) {
				t.Errorf("log has no %q:\n%s", want, log)
			}
		}
	})
}
