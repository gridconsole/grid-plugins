package agent

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

// lookupToolDir makes a temp dir holding an executable file "tool" and returns
// the dir's parent and the dir's name.
func lookupToolDir(t *testing.T) (parent, name string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("the fake tool is not an .exe, so Windows LookPath (PATHEXT) would not accept it")
	}
	parent = t.TempDir()
	name = "bin"
	dir := filepath.Join(parent, name)
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "tool"), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	return parent, name
}

func TestLookupRelativePathEntryIsRefused(t *testing.T) {
	parent, rel := lookupToolDir(t)
	t.Chdir(parent)
	if got, err := resolveAgent(rel, "tool"); !errors.Is(err, exec.ErrDot) || got != "" {
		t.Fatalf("resolveAgent(%q, tool) = %q, %v; want \"\" and exec.ErrDot", rel, got, err)
	}
}

func TestLookupEmptyPathEntryIsCurrentDirectory(t *testing.T) {
	parent, rel := lookupToolDir(t)
	t.Chdir(filepath.Join(parent, rel))
	path := string(os.PathListSeparator)
	if got, err := resolveAgent(path, "tool"); !errors.Is(err, exec.ErrDot) || got != "" {
		t.Fatalf("resolveAgent(empty entries, tool) = %q, %v; want \"\" and exec.ErrDot", got, err)
	}
}

func TestLookupResolveInSkipsRelativeStandardDir(t *testing.T) {
	parent, rel := lookupToolDir(t)
	abs := filepath.Join(parent, rel)
	got, err := resolveIn([]string{"rel"}, abs, "tool")
	if err != nil || got != filepath.Join(abs, "tool") {
		t.Fatalf("resolveIn = %q, %v; want %q and nil", got, err, filepath.Join(abs, "tool"))
	}
}

func TestLookupPathWithoutToolIsNotFound(t *testing.T) {
	if got, err := resolveAgent(t.TempDir(), "tool"); !errors.Is(err, exec.ErrNotFound) || got != "" {
		t.Fatalf("resolveAgent(no tool) = %q, %v; want \"\" and exec.ErrNotFound", got, err)
	}
}

func TestLookupCheckWithoutLookupUsesResolve(t *testing.T) {
	const name = "grid-no-such-agent"
	r := check(context.Background(), Env{Home: t.TempDir(), PATH: t.TempDir()}, name)
	if r.Found {
		t.Fatalf("Found = true for %q: %+v", name, r)
	}
	if want := fixLine(name, "install", false, runtime.GOOS); r.Fix != want {
		t.Fatalf("Fix = %q, want %q", r.Fix, want)
	}
}

func TestLookupFromBrewRegularFileOutsideCaskroom(t *testing.T) {
	file := filepath.Join(t.TempDir(), "claude")
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if fromBrew(file) {
		t.Fatalf("fromBrew(%q) = true, want false", file)
	}
}

func TestLookupFixLineClaudeInstallLinux(t *testing.T) {
	const want = "curl -fsSL https://claude.ai/install.sh | bash"
	if got := fixLine("claude", "install", false, "linux"); got != want {
		t.Fatalf("fixLine = %q, want %q", got, want)
	}
}

func TestLookupProbeOutputStartFailure(t *testing.T) {
	out, code := probeOutput(context.Background(), exec.Command(filepath.Join(t.TempDir(), "missing")))
	if out != "" || code != -1 {
		t.Fatalf("probeOutput = %q, %d; want \"\" and -1", out, code)
	}
}
