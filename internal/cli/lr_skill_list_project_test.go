package cli

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/project"
)

// lrSlpFixture records projects alpha and beta with folders away from h.home.
func lrSlpFixture(t *testing.T) *harness {
	t.Helper()
	h := newHarness(t)
	for _, name := range []string{"alpha", "beta"} {
		dir := filepath.Join(h.root, name)
		h.mkdir(dir)
		h.record(project.Record{Name: name, ID: "01" + strings.ToUpper(name), Folders: []project.Folder{{Machine: h.env.Machine, Path: dir}}})
	}
	return h
}

// lrSlpRun runs grid from h.home and returns the exit code and stdout.
func lrSlpRun(t *testing.T, h *harness, args ...string) (int, string) {
	t.Helper()
	code := h.run(h.home, args...)
	return code, h.out.String()
}

func TestLrSlpListsForTheNamedProject(t *testing.T) {
	h := lrSlpFixture(t)
	for _, args := range [][]string{
		{"skill", "disable", "research", "--project", "beta"},
		{"agent", "set", "grid-builder", "--model", "opus", "--project", "beta"},
	} {
		if code := h.run(h.home, args...); code != 0 {
			t.Fatalf("%v: exit %d: %s", args, code, h.errb)
		}
	}
	for _, tc := range []struct {
		args []string
		want string
	}{
		{[]string{"skill", "list", "--project", "beta", "--json"}, `"research":false`},
		{[]string{"skill", "list", "--project", "alpha", "--json"}, `"research":true`},
		{[]string{"agent", "list", "--project", "beta", "--json"}, `"grid-builder":{"model":"opus"`},
		{[]string{"agent", "list", "--project", "alpha", "--json"}, `"grid-builder":{"model":"sonnet"`},
	} {
		code, out := lrSlpRun(t, h, tc.args...)
		if code != 0 || !strings.Contains(out, tc.want) {
			t.Errorf("%v: exit %d, stdout %q, stderr %q; want stdout holding %s", tc.args, code, out, h.errb, tc.want)
		}
	}
}

func TestLrSlpRefusals(t *testing.T) {
	h := lrSlpFixture(t)
	for _, kind := range []string{"skill", "agent"} {
		code, out := lrSlpRun(t, h, kind, "list", "--project", "nosuch")
		want := kind + ": project \"nosuch\" is not registered\n"
		if code != 1 || out != "" || h.errb.String() != want {
			t.Errorf("%s list nosuch: exit %d, stdout %q, stderr %q; want stderr %q", kind, code, out, h.errb, want)
		}
	}

	// gus is a Client of alpha only: beta is recorded but has no people entry for gus.
	g, _ := settingsAccessFixture(t, "guest")
	beta := filepath.Join(g.root, "beta")
	g.mkdir(beta)
	g.record(project.Record{Name: "beta", ID: "01BETA", Folders: []project.Folder{{Machine: g.env.Machine, Path: beta}}})
	if code := g.run(g.home, "skill", "list", "--project", "alpha"); code != 0 {
		t.Errorf("guest alpha: exit %d, stderr %q", code, g.errb)
	}
	if code := g.run(g.home, "skill", "list", "--project", "beta"); code != 1 || g.out.Len() != 0 {
		t.Errorf("guest beta: exit %d, stdout %q, stderr %q", code, g.out, g.errb)
	}
}

func TestLrSlpHookLevelsTakesNoProject(t *testing.T) {
	h := lrSlpFixture(t)
	if code := h.run(h.home, "hook", "levels", "--project", "beta"); code != 2 {
		t.Errorf("hook levels --project: exit %d, want 2", code)
	}
}
