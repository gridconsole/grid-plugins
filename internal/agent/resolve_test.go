package agent

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

func TestIsBatchLauncher(t *testing.T) {
	for _, tc := range []struct {
		path string
		want bool
	}{
		{"claude.CMD", true},
		{"codex.bat", true},
		{`C:\agents\claude.CmD`, true},
		{`C:\agents\codex.BaT`, true},
		{"claude", false},
		{"codex.exe", false},
		{"claude.cmd.exe", false},
		{filepath.Join("agents.cmd", "claude"), false},
	} {
		if got := isBatchLauncher(tc.path); got != tc.want {
			t.Errorf("isBatchLauncher(%q) = %v; want %v", tc.path, got, tc.want)
		}
	}
}

func TestResolveAgentLaunchPath(t *testing.T) {
	first, second := t.TempDir(), t.TempDir()
	name := "claude"
	if runtime.GOOS == "windows" {
		t.Setenv("PATHEXT", ".EXE;.CMD")
		name += ".exe"
	}
	for _, dir := range []string{first, second} {
		if err := os.WriteFile(filepath.Join(dir, name), nil, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", second)
	got, err := resolveAgent(first+string(os.PathListSeparator)+second, "claude")
	if err != nil || got != filepath.Join(first, name) {
		t.Fatalf("resolved %q, %v", got, err)
	}
	if got, err := resolveAgent("", "claude"); got != "" || !errors.Is(err, exec.ErrNotFound) {
		t.Fatalf("empty launch PATH resolved host executable: %q, %v", got, err)
	}
	if os.Getenv("PATH") != second {
		t.Fatal("resolver changed the process PATH")
	}
}

func TestResolveAgentExecutableRules(t *testing.T) {
	first, second := t.TempDir(), t.TempDir()
	name := "codex"
	if runtime.GOOS == "windows" {
		t.Setenv("PATHEXT", ".EXE;.CMD")
		name += ".cmd"
	} else {
		if err := os.WriteFile(filepath.Join(first, name), nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(second, name), nil, 0o755); err != nil {
		t.Fatal(err)
	}
	got, err := resolveAgent(first+string(os.PathListSeparator)+second, "codex")
	if err != nil || got != filepath.Join(second, name) {
		t.Fatalf("resolved %q, %v", got, err)
	}
}
