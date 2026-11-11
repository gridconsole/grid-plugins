package guard

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
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
	f.write(filepath.Join(f.projects, "example", "project.md"), "---\nname: example\n---\n")
	f.write(filepath.Join(f.projects, "example", "knowledge", "old.md"), "old\n")
	f.write(filepath.Join(f.grid, "knowledge", "old.md"), "old\n")
	f.write(filepath.Join(f.task, "feed", "000001.json"), "{}\n")
	for _, tt := range []struct {
		name  string
		path  string
		guard func(string) string
	}{
		{"project record", alias(filepath.Join(f.projects, "example", "project.md"), "projects"), f.onlyWriter},
		{"project record, project folder spelled differently", alias(filepath.Join(f.projects, "example", "project.md"), "example"), f.onlyWriter},
		{"project knowledge", alias(filepath.Join(f.projects, "example", "knowledge", "n.md"), "projects"), f.onlyWriter},
		{"project knowledge, existing file", alias(filepath.Join(f.projects, "example", "knowledge", "old.md"), "projects", "knowledge"), f.onlyWriter},
		{"home knowledge", alias(filepath.Join(f.grid, "knowledge", "n.md"), ".grid"), f.onlyWriter},
		{"home knowledge, existing file", alias(filepath.Join(f.grid, "knowledge", "old.md"), ".grid", "knowledge"), f.onlyWriter},
		{"task file", alias(filepath.Join(f.task, "notes.md"), "projects", "tasks"), f.onlyWriter},
		{"feed", alias(filepath.Join(f.task, "feed", "000002.json"), "projects"), f.host},
		{"brain from a builder", alias(filepath.Join(f.grid, "knowledge", "n.md"), ".grid"), f.planAs("grid:grid-builder")},
		{"project knowledge from a builder", alias(filepath.Join(f.projects, "example", "knowledge", "n.md"), "projects"), f.planAs("grid:grid-builder")},
		{"repo write from a held task, cwd spelled differently", filepath.Join(f.repo, "file.go"), f.heldCwd(alias(f.task, "projects"))},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.guard(tt.path); got != "deny" {
				t.Fatalf("%s: %q, want deny", tt.path, got)
			}
		})
	}
}

func (f *fixture) onlyWriter(path string) string {
	return EvaluateOnlyWriter(f.input("Write", path), f.root, f.env).Behavior
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

func TestOnlyWriterBlocksTaskInputsAndAttachments(t *testing.T) {
	f := setup(t)
	for _, folder := range []string{"inputs", "attachments"} {
		t.Run(folder, func(t *testing.T) {
			if got := f.onlyWriter(filepath.Join(f.task, folder, "example.txt")); got != "deny" {
				t.Fatalf("got %q, want deny", got)
			}
		})
	}
}
