package hook

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/project"
	"github.com/gridconsole/grid-plugins/internal/testenv"
	"github.com/gridconsole/grid-plugins/internal/where"
)

const voPathMachine = "01TESTMACHINE"

// voPathSession is one SessionStart run: the Grid home it resolves, the plugin
// root it was told about and the env file it appends to.
type voPathSession struct {
	home    string
	root    string
	envFile string
}

// voPathNew makes a Grid home named homeName with a project record, a plugin root
// holding bin/grid and an empty env file path. The home holds no bin/grid yet.
func voPathNew(t *testing.T, homeName string) voPathSession {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("posix path line")
	}
	base := t.TempDir()
	home := filepath.Join(base, homeName)
	cwd := filepath.Join(base, "work")
	if err := os.MkdirAll(cwd, 0o700); err != nil {
		t.Fatal(err)
	}
	rec := project.Record{Name: "example", ID: "01EXAMPLE", Folders: []project.Folder{{Machine: voPathMachine, Path: cwd}}}
	if err := project.Write(home, rec); err != nil {
		t.Fatal(err)
	}
	return voPathSession{home: home, root: pluginRootWithGrid(t, "plug"), envFile: filepath.Join(t.TempDir(), "env")}
}

// voPathLauncher puts a bin/grid file in the Grid home.
func voPathLauncher(t *testing.T, s voPathSession) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(s.home, "bin"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(s.home, "bin", "grid"), []byte("#!/bin/sh\n"), 0o700); err != nil {
		t.Fatal(err)
	}
}

// voPathRun runs the sessionstart hook the way hook_test.go does and returns the env file.
func voPathRun(t *testing.T, s voPathSession, pathEnv string) string {
	t.Helper()
	cwd := filepath.Join(filepath.Dir(s.home), "work")
	o := Options{
		Event:    "sessionstart",
		Cwd:      cwd,
		Stdout:   &strings.Builder{},
		Resolver: where.Resolver{GridHome: s.home, Home: filepath.Dir(s.home), Machine: voPathMachine},
		Getenv: func(key string) string {
			switch key {
			case "CLAUDE_ENV_FILE":
				return s.envFile
			case "CLAUDE_PLUGIN_ROOT":
				return s.root
			case "PATH":
				return pathEnv
			}
			return ""
		},
		Stdin: strings.NewReader(`{"source":"startup"}`),
	}
	if code := Run(o); code != 0 {
		t.Fatalf("exit %d", code)
	}
	b, err := os.ReadFile(s.envFile)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// voPathQuoted spells the line for a bin folder, written out here and not through shellQuote.
func voPathQuoted(bin string) string {
	return "export PATH='" + strings.ReplaceAll(bin, "'", `'\''`) + "':\"$PATH\"\n"
}

func TestVoPathHomeLauncherWinsOverPluginRoot(t *testing.T) {
	s := voPathNew(t, "data")
	voPathLauncher(t, s)
	got := voPathRun(t, s, "/usr/bin")
	home := voPathQuoted(filepath.Join(s.home, "bin"))
	if !strings.Contains(got, home) {
		t.Fatalf("A1 want %q in env file:\n%s", home, got)
	}
	if strings.Contains(got, filepath.Join(s.root, "bin")) {
		t.Fatalf("A1 the plugin root's bin folder is on the PATH line too:\n%s", got)
	}
	if n := strings.Count(got, "export PATH="); n != 1 {
		t.Fatalf("A1 want one PATH line, got %d:\n%s", n, got)
	}
}

func TestVoPathNoLauncherKeepsPluginRootLine(t *testing.T) {
	s := voPathNew(t, "data")
	got := voPathRun(t, s, "/usr/bin")
	want := filepath.Join(t.TempDir(), "want")
	if err := writeEnv(want, s.home, "", pluginPathLine(s.root, "/usr/bin")); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(want)
	if err != nil {
		t.Fatal(err)
	}
	if got != string(b) {
		t.Fatalf("A2 env file changed without a launcher:\ngot  %q\nwant %q", got, b)
	}
	if !strings.Contains(got, voPathQuoted(filepath.Join(s.root, "bin"))) {
		t.Fatalf("A2 the plugin root's line is missing:\n%s", got)
	}
}

func TestVoPathHomeBinAlreadyOnPathWritesNoHomeLine(t *testing.T) {
	s := voPathNew(t, "data")
	voPathLauncher(t, s)
	bin := filepath.Join(s.home, "bin")
	// A trailing slash is the same folder after filepath.Clean.
	got := voPathRun(t, s, "/usr/bin:"+bin+"/")
	if strings.Contains(got, voPathQuoted(bin)) || strings.Contains(got, "PATH='"+bin) {
		t.Fatalf("A2 home line written although the folder is on PATH:\n%s", got)
	}
	// No plugin root: no PATH line at all.
	s.root = ""
	s.envFile = filepath.Join(t.TempDir(), "env")
	if got := voPathRun(t, s, "/usr/bin:"+bin); strings.Contains(got, "export PATH=") {
		t.Fatalf("A2 PATH line with the folder already on PATH and no plugin root:\n%s", got)
	}
}

func TestVoPathLauncherFallsBackToPluginRootLine(t *testing.T) {
	s := voPathNew(t, "data")
	voPathLauncher(t, s)
	bin := filepath.Join(s.home, "bin")
	// The home folder is already on PATH, so the home line is "" and the plugin root's line is
	// written as before (the brief: voLauncherPathLine when not "", else pluginPathLine).
	got := voPathRun(t, s, "/usr/bin:"+bin)
	if want := voPathQuoted(filepath.Join(s.root, "bin")); !strings.Contains(got, want) {
		t.Fatalf("A2 want the plugin root's line %q:\n%s", want, got)
	}
}

func TestVoPathLauncherDirectoryWritesPluginRootLine(t *testing.T) {
	s := voPathNew(t, "data")
	if err := os.MkdirAll(filepath.Join(s.home, "bin", "grid"), 0o700); err != nil {
		t.Fatal(err)
	}
	got := voPathRun(t, s, "/usr/bin")
	if want := voPathQuoted(filepath.Join(s.root, "bin")); !strings.Contains(got, want) {
		t.Fatalf("A2 want the plugin root's line %q:\n%s", want, got)
	}
	if strings.Contains(got, voPathQuoted(filepath.Join(s.home, "bin"))) {
		t.Fatalf("A2 a directory named grid counted as the launcher:\n%s", got)
	}
}

func TestVoPathLauncherLinkToFileCounts(t *testing.T) {
	s := voPathNew(t, "data")
	target := filepath.Join(t.TempDir(), "real-grid")
	if err := os.WriteFile(target, []byte("#!/bin/sh\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(s.home, "bin"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(s.home, "bin", "grid")); err != nil {
		t.Skip("symlinks unavailable: " + err.Error())
	}
	got := voPathRun(t, s, "/usr/bin")
	if want := voPathQuoted(filepath.Join(s.home, "bin")); !strings.Contains(got, want) {
		t.Fatalf("A2 a link to a file is the launcher, want %q:\n%s", want, got)
	}
}

func TestVoPathLauncherBrokenLinkWritesPluginRootLine(t *testing.T) {
	s := voPathNew(t, "data")
	if err := os.MkdirAll(filepath.Join(s.home, "bin"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(t.TempDir(), "gone"), filepath.Join(s.home, "bin", "grid")); err != nil {
		t.Skip("symlinks unavailable: " + err.Error())
	}
	got := voPathRun(t, s, "/usr/bin")
	if want := voPathQuoted(filepath.Join(s.root, "bin")); !strings.Contains(got, want) {
		t.Fatalf("A2 want the plugin root's line %q:\n%s", want, got)
	}
	if strings.Contains(got, voPathQuoted(filepath.Join(s.home, "bin"))) {
		t.Fatalf("A2 a broken link counted as the launcher:\n%s", got)
	}
}

func TestVoPathHomeWithSpaceAndQuoteResolvesGrid(t *testing.T) {
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("sh unavailable")
	}
	s := voPathNew(t, "a b'c")
	voPathLauncher(t, s)
	got := voPathRun(t, s, "/usr/bin")
	bin := filepath.Join(s.home, "bin")
	if want := voPathQuoted(bin); !strings.Contains(got, want) {
		t.Fatalf("A3 want %q in env file:\n%s", want, got)
	}
	if !strings.Contains(bin, " ") || !strings.Contains(bin, "'") {
		t.Fatalf("A3 the test home lost its space or quote: %q", bin)
	}
	cmd := exec.Command("sh", "-c", `. "$1"; command -v grid`, "sh", s.envFile)
	cmd.Env = testenv.Temp(t, append(cmd.Environ(), "PATH=/usr/bin:/bin"))
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("A3 grid not resolved: %v", err)
	}
	if want := filepath.Join(bin, "grid"); strings.TrimSpace(string(out)) != want {
		t.Fatalf("A3 got %q want %q", out, want)
	}
}
