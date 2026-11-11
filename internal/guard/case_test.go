package guard

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/settings"
)

// foldsCase reports whether dir's filesystem treats two spellings of a name as
// one file, as the macOS and Windows defaults do. It is probed, never assumed
// from the OS, because a volume can be formatted either way.
func foldsCase(t *testing.T, dir string) bool {
	t.Helper()
	probe := filepath.Join(dir, "CaseProbe")
	if err := os.WriteFile(probe, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	defer os.Remove(probe)
	_, err := os.Stat(filepath.Join(dir, "caseprobe"))
	return err == nil
}

func skipUnlessFolding(t *testing.T, f *fixture) {
	t.Helper()
	if !foldsCase(t, f.root) {
		t.Skip("case-sensitive filesystem: another spelling names a different path")
	}
}

// alias swaps the on-disk case of the named components of path for another
// spelling of the same names.
func alias(path string, names ...string) string {
	for _, n := range names {
		path = strings.Replace(path, string(filepath.Separator)+n, string(filepath.Separator)+strings.ToUpper(n), 1)
	}
	return path
}

func TestRealNativeReturnsOnDiskCase(t *testing.T) {
	f := setup(t)
	skipUnlessFolding(t, f)
	dir := filepath.Join(f.root, "Alpha", "Beta")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	f.link(filepath.Join(f.root, "Short"), filepath.Join(f.root, "Alpha"))
	for _, tt := range []struct{ name, given, want string }{
		{"existing", filepath.Join(f.root, "ALPHA", "beta"), dir},
		{"missing tail keeps its spelling", filepath.Join(f.root, "alpha", "BETA", "New", "x.md"), filepath.Join(dir, "New", "x.md")},
		{"through a link", filepath.Join(f.root, "SHORT", "beta"), dir},
		{"already exact", dir, dir},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := realNative(tt.given, 0); got != tt.want {
				t.Fatalf("got %s, want %s", got, tt.want)
			}
		})
	}
}

// Every rule below compares resolved paths, so a spelling that only differs in
// case must reach the same decision as the exact one on a case-folding volume.
func TestCaseAliasesAreJudgedLikeTheirTargets(t *testing.T) {
	f := setup(t)
	skipUnlessFolding(t, f)
	f.write(filepath.Join(f.task, "feed", "000001.json"), "{}\n")
	// plan-guard and host-guard warn by default (verdicts-11nov v-4); these rows pin the deny each gives at block.
	for _, name := range []string{"plan-guard", "host-guard"} {
		if err := (settings.Store{Home: f.grid}).SetHook(name, "block"); err != nil {
			t.Fatal(err)
		}
	}
	for _, tt := range []struct {
		name  string
		path  string
		guard func(string) string
	}{
		{"feed", alias(filepath.Join(f.task, "feed", "000002.json"), "projects"), f.host},
		{"brain from a builder", alias(filepath.Join(f.grid, "knowledge", "n.md"), ".grid"), f.planAs("grid:grid-builder")},
		{"project knowledge from a builder", alias(filepath.Join(f.projects, "example", "knowledge", "n.md"), "projects"), f.planAs("grid:grid-builder")},
		{"repo write from a held task, cwd spelled differently", filepath.Join(f.repo, "file.go"), f.sameAsExactCwd(f.task, alias(f.task, "projects"))},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.guard(tt.path); got != "deny" {
				t.Fatalf("%s: %q, want deny", tt.path, got)
			}
		})
	}
}

func (f *fixture) host(path string) string {
	return EvaluateHost(f.input("Write", path), f.root, f.env).Behavior
}
func (f *fixture) planAs(agent string) func(string) string {
	return func(path string) string {
		i := f.input("Write", path)
		i.AgentType = agent
		i.Cwd = f.repo
		return EvaluatePlan(i, f.root, f.env).Behavior
	}
}
func (f *fixture) heldCwd(cwd string) func(string) string {
	return func(path string) string {
		i := f.input("Write", path)
		i.Cwd = cwd
		return EvaluatePlan(i, f.root, f.env).Behavior
	}
}

// sameAsExactCwd answers "deny" when the plan guard gives the alias cwd the same
// verdict as the exact cwd, and otherwise names both verdicts. The plan hold this
// row once pinned was removed (b10002db, its tests dropped in 62e8f7fc), so the
// principle above is all that is left: another spelling changes nothing.
func (f *fixture) sameAsExactCwd(exact, aliased string) func(string) string {
	return func(path string) string {
		want, got := f.heldCwd(exact)(path), f.heldCwd(aliased)(path)
		if got == want {
			return "deny"
		}
		return "alias cwd gave " + got + ", exact cwd gave " + want
	}
}
