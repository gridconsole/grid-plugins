package packaging

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// Every release platform ships one prebuilt binary, under the name the launchers look for.
func TestTargetsShipEveryPlatform(t *testing.T) {
	var got []string
	for _, target := range targets {
		got = append(got, binaryName(target[0], target[1]))
	}
	want := "grid-darwin-arm64 grid-darwin-amd64 grid-linux-amd64 grid-linux-arm64 grid-windows-amd64.exe"
	if strings.Join(got, " ") != want {
		t.Fatalf("targets %q, want %q", got, want)
	}
}

// The version goes in once, as Build's argument, and comes out of every manifest.
func TestVersionStampedInEveryManifest(t *testing.T) {
	const stamp = "0.1.4242"
	root := t.TempDir()
	if err := Write(root, stamp); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{".claude-plugin/plugin.json", ".codex-plugin/plugin.json"} {
		var manifest struct {
			Version string `json:"version"`
		}
		b, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(name)))
		if err != nil || json.Unmarshal(b, &manifest) != nil || manifest.Version != stamp {
			t.Errorf("%s: version %q, want %q (%v)", name, manifest.Version, stamp, err)
		}
	}
	var market struct {
		Plugins []struct{ Version string } `json:"plugins"`
	}
	b, err := os.ReadFile(filepath.Join(root, ".claude-plugin", "marketplace.json"))
	if err != nil || json.Unmarshal(b, &market) != nil || len(market.Plugins) != 1 || market.Plugins[0].Version != stamp {
		t.Errorf("marketplace.json: %+v (%v)", market, err)
	}
}

func TestCommitVersion(t *testing.T) {
	if got := CommitVersion(1221); got != "0.2.1221" {
		t.Fatalf("got %q", got)
	}
}

// ReleaseVersion is 0.2.<commits reachable from HEAD>: it grows with each commit.
func TestReleaseVersionCountsCommits(t *testing.T) {
	repo := t.TempDir()
	git := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-c", "user.name=t", "-c", "user.email=t@example.com", "-c", "commit.gpgsign=false"}, args...)...)
		cmd.Dir = repo
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	git("init", "-q")
	for want := 1; want <= 3; want++ {
		git("commit", "-q", "--allow-empty", "-m", "c")
		got, err := ReleaseVersion(repo)
		if err != nil || got != CommitVersion(want) {
			t.Fatalf("after %d commits: %q %v", want, got, err)
		}
	}
	if _, err := ReleaseVersion(t.TempDir()); err == nil {
		t.Fatal("a directory outside git got a release version")
	}
}

func fakeUname(t *testing.T, tools, osName, arch string) {
	t.Helper()
	writeTestFile(t, filepath.Join(tools, "uname"), "#!/bin/sh\ncase $1 in -s) echo "+osName+";; -m) echo "+arch+";; esac\n", 0o755)
}

// The sh entry picks the binary for each OS from uname, on all five release platforms.
func TestLauncherChoosesBinaryPerOS(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the sh entry runs under sh")
	}
	for _, tc := range []struct{ os, arch, binary string }{
		{"Darwin", "arm64", "grid-darwin-arm64"},
		{"Darwin", "x86_64", "grid-darwin-amd64"},
		{"Linux", "x86_64", "grid-linux-amd64"},
		{"Linux", "aarch64", "grid-linux-arm64"},
		{"MINGW64_NT-10.0-22631", "x86_64", "grid-windows-amd64.exe"},
		{"MSYS_NT-10.0-22631", "x86_64", "grid-windows-amd64.exe"},
		{"CYGWIN_NT-10.0-22631", "x86_64", "grid-windows-amd64.exe"},
	} {
		t.Run(tc.os+"-"+tc.arch, func(t *testing.T) {
			root := filepath.Join(t.TempDir(), "plugin dir")
			if err := Write(root, "test"); err != nil {
				t.Fatal(err)
			}
			tools := filepath.Join(root, "tools")
			fakeUname(t, tools, tc.os, tc.arch)
			// Every shipped binary exists; only the right one may run.
			for _, target := range targets {
				name := binaryName(target[0], target[1])
				writeTestFile(t, filepath.Join(root, "bin", name), "#!/bin/sh\nprintf '%s\\n' '"+name+"' \"$@\"\nexit 7\n", 0o755)
			}
			t.Setenv("PATH", tools+string(os.PathListSeparator)+os.Getenv("PATH"))
			out, err := exec.Command(filepath.Join(root, "bin", "grid"), "version", "two words").Output()
			if exit, ok := err.(*exec.ExitError); !ok || exit.ExitCode() != 7 {
				t.Fatalf("exit: %v", err)
			}
			if want := tc.binary + "\nversion\ntwo words\n"; string(out) != want {
				t.Fatalf("chose %q, want %q", out, want)
			}
		})
	}
}

// The Windows entry is a batch file that runs the Windows binary beside it and builds nothing.
func TestWindowsEntryIsCmd(t *testing.T) {
	files, err := Files("test")
	if err != nil {
		t.Fatal(err)
	}
	cmd, ok := files["bin/grid.cmd"]
	if !ok {
		t.Fatal("no bin/grid.cmd in the package")
	}
	if bytes.Contains(bytes.ReplaceAll(cmd, []byte("\r\n"), nil), []byte("\n")) || !bytes.HasSuffix(cmd, []byte("\r\n")) {
		t.Errorf("grid.cmd must use CRLF line endings throughout: %q", cmd)
	}
	text := string(cmd)
	for _, want := range []string{`"grid_exe=%~dp0grid-windows-amd64.exe"`, `"%grid_exe%" %*`, `exit /b %ERRORLEVEL%`, `if not exist "%grid_exe%"`} {
		if !strings.Contains(text, want) {
			t.Errorf("grid.cmd lacks %s", want)
		}
	}
	if strings.Contains(text, "go build") || strings.Contains(strings.ToLower(text), "curl") {
		t.Error("grid.cmd must not build or download anything")
	}
	if want := binaryName("windows", "amd64"); !strings.Contains(text, want) {
		t.Errorf("grid.cmd does not name %s", want)
	}
}

// An installed release runs with no Go on PATH: copy dist/plugin as `claude plugin install` would,
// then run the entry the hook commands name. Run make dist first; without dist/plugin this skips.
func TestDistPluginRunsWithoutGo(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the sh entry runs under sh")
	}
	source := filepath.Join("..", "..", "dist", "plugin")
	if _, err := os.Stat(filepath.Join(source, "bin", binaryName(runtime.GOOS, runtime.GOARCH))); err != nil {
		t.Skipf("no dist/plugin for this host (run make dist): %v", err)
	}
	installed := filepath.Join(t.TempDir(), "installed plugin")
	if err := copyDist(source, installed); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", "/usr/bin:/bin")
	if _, err := exec.LookPath("go"); err == nil {
		t.Skip("go is on the system PATH here")
	}
	t.Setenv("HOME", t.TempDir())
	t.Setenv("CLAUDE_PLUGIN_ROOT", installed)

	var manifest struct {
		Version string `json:"version"`
	}
	b, err := os.ReadFile(filepath.Join(installed, ".claude-plugin", "plugin.json"))
	if err != nil || json.Unmarshal(b, &manifest) != nil || manifest.Version == "" {
		t.Fatalf("manifest: %q %v", b, err)
	}
	var hooks HookConfig
	b, err = os.ReadFile(filepath.Join(installed, "hooks", "hooks.json"))
	if err != nil || json.Unmarshal(b, &hooks) != nil || len(hooks.Hooks) == 0 {
		t.Fatalf("hooks.json: %q %v", b, err)
	}
	// The command every hook runs, with its trailing `hook <event>` swapped for `version`.
	const entry = `"${CLAUDE_PLUGIN_ROOT}/bin/grid"`
	for event, groups := range hooks.Hooks {
		command := groups[0].Hooks[0].Command
		if !strings.HasPrefix(command, entry+" hook ") {
			t.Fatalf("%s: command %q does not run the bin/grid entry", event, command)
		}
		out, err := exec.Command("sh", "-c", entry+" version").CombinedOutput()
		if err != nil || !strings.HasPrefix(string(out), "grid "+manifest.Version+"\n") {
			t.Fatalf("%s entry: %q %v", event, out, err)
		}
		t.Logf("grid version through the hook entry (PATH=/usr/bin:/bin, no go):\n%s", out)
		break
	}
}

func copyDist(source, dest string) error {
	return filepath.WalkDir(source, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		to := filepath.Join(dest, rel)
		if entry.IsDir() {
			return os.MkdirAll(to, 0o755)
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(to, b, info.Mode().Perm())
	})
}
