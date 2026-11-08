package agent

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
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
		{"claude.cmd.", true},
		{"claude.cmd..", true},
		{"claude.CMD ", true},
		{"claude.bat. .", true},
		{`C:\agents\codex.cmd.`, true},
		{".", true},
		{" ", true},
		{"", true},
		{"... . ", true},
		{`C:\agents\. `, true},
		{"agents/. ", true},
		{`C:\agents.cmd\claude`, false},
		{"claude.exe. ", false},
		{"claude.cmd.exe. ", false},
	} {
		if got := isBatchLauncher(tc.path); got != tc.want {
			t.Errorf("isBatchLauncher(%q) = %v; want %v", tc.path, got, tc.want)
		}
	}
}

func TestAgentCheckBatchTrailingDots(t *testing.T) {
	for _, name := range []string{"claude", "codex"} {
		t.Run(name, func(t *testing.T) {
			checkBatchResult(t, name, t.TempDir(), name+".cmd.", true)
		})
	}
}

func TestAgentCheckBatchSymlinks(t *testing.T) {
	for _, tc := range []struct {
		name, link, target string
		dangling, refused  bool
	}{
		{"batch", "claude", "real.cmd", false, true},
		{"native", "claude", "real", false, false},
		{"dangling", "claude", "missing", true, true},
		{"exe_batch", "claude.exe", "real.cmd", false, true},
		{"exe_native", "claude.exe", "real", false, false},
		{"exe_dangling", "claude.exe", "missing", true, true},
		{"batch_link_native", "claude.cmd", "real", false, true},
		{"loop", "claude", "claude", true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			if !tc.dangling {
				if err := os.WriteFile(filepath.Join(dir, tc.target), nil, 0o755); err != nil {
					t.Fatal(err)
				}
			}
			if err := os.Symlink(tc.target, filepath.Join(dir, tc.link)); err != nil {
				t.Skipf("symlink creation unavailable: %v", err)
			}
			checkBatchResult(t, "claude", dir, tc.link, tc.refused)
		})
	}
}

func checkBatchResult(t *testing.T, name, dir, binary string, refused bool) {
	t.Helper()
	calls := 0
	env := Env{
		Home:   dir,
		Lookup: func(string) (string, error) { return filepath.Join(dir, binary), nil },
		Runner: func(_ context.Context, _ []string) (string, int) {
			calls++
			if calls == 1 {
				return "3.0.0", 0
			}
			return `{"authMethod":"oauth"}`, 0
		},
	}
	min := ClaudeMin
	if name == "codex" {
		min = CodexMin
	}
	want := Report{Agent: name, Found: true, Path: "~" + string(filepath.Separator) + binary, Min: min}
	wantCalls := 0
	if refused {
		want.Fix = fixLine(name, "install", false, runtime.GOOS)
	} else {
		want.Version, want.SignedIn = "3.0.0", boolPtr(true)
		wantCalls = 2
	}
	if got := check(context.Background(), env, name); !reflect.DeepEqual(got, want) {
		t.Errorf("report = %+v; want %+v", got, want)
	}
	if calls != wantCalls {
		t.Errorf("runner called %d times; want %d", calls, wantCalls)
	}
}

func TestBatchExtensionsReplaceable(t *testing.T) {
	original := BatchExtensions
	t.Cleanup(func() { BatchExtensions = original })
	BatchExtensions = []string{".WRAPPER"}
	for _, tc := range []struct {
		path string
		want bool
	}{
		{"claude.wrapper. ", true},
		{"claude.cmd", false},
		{"codex.bat", false},
		{"claude.exe", false},
		{". ", true},
	} {
		if got := isBatchLauncher(tc.path); got != tc.want {
			t.Errorf("custom isBatchLauncher(%q) = %v; want %v", tc.path, got, tc.want)
		}
	}
	checkBatchResult(t, "claude", t.TempDir(), "claude.wrapper. ", true)
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
