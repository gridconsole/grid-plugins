package packaging

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func writeTestFile(t *testing.T, path, contents string, mode os.FileMode) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(contents), mode); err != nil {
		t.Fatal(err)
	}
}

func TestLauncher(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("release launcher supports macOS and Linux")
	}
	for _, tc := range []struct{ os, arch, target string }{
		{"Darwin", "arm64", "darwin-arm64"},
		{"Darwin", "x86_64", "darwin-amd64"},
		{"Linux", "aarch64", "linux-arm64"},
		{"Linux", "x86_64", "linux-amd64"},
		{"Linux", "arm64", "linux-arm64"},
		{"Linux", "amd64", "linux-amd64"},
	} {
		t.Run(tc.os+"-"+tc.arch, func(t *testing.T) {
			root := filepath.Join(t.TempDir(), "plugin with spaces")
			if err := Write(root, "test"); err != nil {
				t.Fatal(err)
			}
			tools := filepath.Join(root, "tools")
			writeTestFile(t, filepath.Join(tools, "uname"), "#!/bin/sh\ncase $1 in -s) echo "+tc.os+";; -m) echo "+tc.arch+";; esac\n", 0o755)
			writeTestFile(t, filepath.Join(root, "bin", "grid-"+tc.target), "#!/bin/sh\nprintf '%s\\n' '"+tc.target+"' \"$@\"\ncat\necho diagnostic >&2\nexit 23\n", 0o755)
			// A relative link followed by an absolute link, with spaces throughout.
			link := filepath.Join(tools, "relative grid")
			if err := os.Symlink("../bin/grid", link); err != nil {
				t.Fatal(err)
			}
			absolute := filepath.Join(tools, "grid")
			if err := os.Symlink(link, absolute); err != nil {
				t.Fatal(err)
			}
			t.Setenv("PATH", tools+string(os.PathListSeparator)+os.Getenv("PATH"))
			cmd := exec.Command(absolute, "hook", "pretooluse", "two words", "")
			cmd.Dir = t.TempDir()
			cmd.Stdin = strings.NewReader("{\"input\":true}\n")
			var stdout, stderr strings.Builder
			cmd.Stdout, cmd.Stderr = &stdout, &stderr
			err := cmd.Run()
			if exit, ok := err.(*exec.ExitError); !ok || exit.ExitCode() != 23 {
				t.Fatalf("exit: %v", err)
			}
			want := tc.target + "\nhook\npretooluse\ntwo words\n\n{\"input\":true}\n"
			if stdout.String() != want || stderr.String() != "diagnostic\n" {
				t.Fatalf("stdout=%q stderr=%q", stdout.String(), stderr.String())
			}
		})
	}
}

func TestLauncherFailures(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("release launcher supports macOS and Linux")
	}
	for _, tc := range []struct{ os, arch, want string }{
		{"FreeBSD", "x86_64", "unsupported operating system: FreeBSD"},
		{"MINGW64_NT", "x86_64", "unsupported operating system: MINGW64_NT"},
		{"Linux", "riscv64", "unsupported architecture: riscv64 on Linux"},
		{"Linux", "x86_64", "missing executable for linux-amd64"},
	} {
		t.Run(tc.os+"-"+tc.arch, func(t *testing.T) {
			root := t.TempDir()
			if err := Write(root, "test"); err != nil {
				t.Fatal(err)
			}
			tools := filepath.Join(root, "tools")
			writeTestFile(t, filepath.Join(tools, "uname"), "#!/bin/sh\ncase $1 in -s) echo "+tc.os+";; -m) echo "+tc.arch+";; esac\n", 0o755)
			t.Setenv("PATH", tools+string(os.PathListSeparator)+os.Getenv("PATH"))
			cmd := exec.Command(filepath.Join(root, "bin", "grid"), "version")
			var stdout, stderr strings.Builder
			cmd.Stdout, cmd.Stderr = &stdout, &stderr
			err := cmd.Run()
			if err == nil || stdout.Len() != 0 || !strings.Contains(stderr.String(), tc.want) {
				t.Fatalf("err=%v stdout=%q stderr=%q", err, stdout.String(), stderr.String())
			}
		})
	}
}

func TestCheckoutLauncherUsesDistribution(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("release launcher supports macOS and Linux")
	}
	root := t.TempDir()
	if err := Write(root, "test"); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(root, "go.mod"), "module example.com/plugin\n", 0o644)
	tools := filepath.Join(root, "tools")
	writeTestFile(t, filepath.Join(tools, "uname"), "#!/bin/sh\ncase $1 in -s) echo Linux;; -m) echo x86_64;; esac\n", 0o755)
	writeTestFile(t, filepath.Join(root, "dist", "plugin", "bin", "grid-linux-amd64"), "#!/bin/sh\necho built-release\n", 0o755)
	t.Setenv("PATH", tools+string(os.PathListSeparator)+os.Getenv("PATH"))
	b, err := exec.Command(filepath.Join(root, "bin", "grid"), "version").CombinedOutput()
	if err != nil || string(b) != "built-release\n" {
		t.Fatalf("%q %v", b, err)
	}
}
