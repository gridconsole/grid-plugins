package agent

import (
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
)

// pinBase is one base environment for the linux and darwin pins. The wants below
// were walked by hand from the childEnv on main 55c46cbd and are also checked
// against that childEnv (TestChildEnvOnHostMatchesThePins).
var pinBase = []string{
	"HOME=ignored", "PATH=ignored", "USER=u", "LOGNAME=l", "LANG=C.UTF-8", "TMPDIR=/tmp", "SHELL=/bin/sh", "LC_ALL=C",
	"CLAUDE_CONFIG_DIR=claude-home", "CODEX_HOME=codex-home", "ANTHROPIC_API_KEY=secret",
	"USERNAME=win", "APPDATA=a", "LOCALAPPDATA=la", "SystemRoot=x", "TEMP=t", "USERPROFILE=up", "PATHEXT=.EXE", "COMSPEC=c", "UNRELATED=no",
}

type unixPin struct {
	goos, name, home, binary string
	want                     []string
}

var unixPins = []unixPin{
	{"linux", "claude", "/home/me", "/home/me/.local/bin/claude", []string{
		"USER=u", "LOGNAME=l", "LANG=C.UTF-8", "TMPDIR=/tmp", "SHELL=/bin/sh", "LC_ALL=C", "CLAUDE_CONFIG_DIR=claude-home",
		"HOME=/home/me", "PATH=/home/me/.local/bin:/usr/bin:/bin:/usr/sbin:/sbin:/opt/homebrew/bin:/usr/local/bin"}},
	{"linux", "codex", "/home/me", "/usr/local/bin/codex", []string{
		"USER=u", "LOGNAME=l", "LANG=C.UTF-8", "TMPDIR=/tmp", "SHELL=/bin/sh", "LC_ALL=C", "CODEX_HOME=codex-home",
		"HOME=/home/me", "PATH=/usr/local/bin:/usr/bin:/bin:/usr/sbin:/sbin:/opt/homebrew/bin:/usr/local/bin"}},
	{"darwin", "claude", "/Users/me", "/opt/homebrew/bin/claude", []string{
		"USER=u", "LOGNAME=l", "LANG=C.UTF-8", "TMPDIR=/tmp", "SHELL=/bin/sh", "LC_ALL=C", "CLAUDE_CONFIG_DIR=claude-home",
		"HOME=/Users/me", "PATH=/opt/homebrew/bin:/usr/bin:/bin:/usr/sbin:/sbin:/opt/homebrew/bin:/usr/local/bin"}},
	{"darwin", "codex", "/Users/me", "/Users/me/.claude/local/codex", []string{
		"USER=u", "LOGNAME=l", "LANG=C.UTF-8", "TMPDIR=/tmp", "SHELL=/bin/sh", "LC_ALL=C", "CODEX_HOME=codex-home",
		"HOME=/Users/me", "PATH=/Users/me/.claude/local:/usr/bin:/bin:/usr/sbin:/sbin:/opt/homebrew/bin:/usr/local/bin"}},
}

// On a linux or darwin host childEnv must give exactly the pinned lines. This
// test uses only childEnv, so it also runs on main, where it passes.
func TestChildEnvOnHostMatchesThePins(t *testing.T) {
	if runtime.GOOS != "linux" && runtime.GOOS != "darwin" {
		t.Skip("childEnv uses the host GOOS; the pins are for linux and darwin")
	}
	for _, c := range unixPins {
		got := Env{Home: c.home, Base: pinBase}.childEnv(c.name, c.binary)
		if !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s/%s: childEnv %q, want %q", c.goos, c.name, got, c.want)
		}
	}
}

// childEnvFor with goos linux or darwin gives the same lines as main's childEnv,
// whatever the host is: the folder of the binary comes from path.Dir, not the host.
func TestChildEnvForUnixMatchesMain(t *testing.T) {
	for _, c := range unixPins {
		got := childEnvFor(c.goos, ":", pinBase, c.home, c.name, c.binary)
		if !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s/%s: childEnvFor %q, want %q", c.goos, c.name, got, c.want)
		}
	}
}

// wantProbePath is the PATH TestRunnerEnvironment expects the probe child to see:
// on Windows the binary's folder, then <SYSTEMROOT>\System32 and <SYSTEMROOT>;
// elsewhere the folder, then systemPath.
func wantProbePath(exe string) string {
	if runtime.GOOS != "windows" {
		return strings.Join(append([]string{filepath.Dir(exe)}, systemPath...), string(os.PathListSeparator))
	}
	dirs := []string{filepath.Dir(exe)}
	if root := os.Getenv("SYSTEMROOT"); root != "" {
		dirs = append(dirs, root+`\System32`, root)
	}
	return strings.Join(dirs, ";")
}

const winBinary = `C:\Users\me\.local\bin\claude.exe`

func TestChildEnvForWindows(t *testing.T) {
	base := []string{`SYSTEMROOT=C:\Windows`, `PATHEXT=.COM;.EXE;.BAT;.CMD`, `COMSPEC=C:\Windows\system32\cmd.exe`, "HOME=x"}
	got := childEnvFor("windows", ";", base, `C:\Users\me`, "claude", winBinary)
	want := []string{
		`SYSTEMROOT=C:\Windows`, `PATHEXT=.COM;.EXE;.BAT;.CMD`, `COMSPEC=C:\Windows\system32\cmd.exe`,
		`HOME=C:\Users\me`, `PATH=C:\Users\me\.local\bin;C:\Windows\System32;C:\Windows`, `USERPROFILE=C:\Users\me`,
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("childEnvFor windows %q, want %q", got, want)
	}
	joined := strings.Join(got, "\n")
	for _, unix := range []string{"/usr/bin", "/bin", "/opt/homebrew/bin"} {
		if strings.Contains(joined, unix) {
			t.Errorf("windows environment holds %s: %q", unix, got)
		}
	}
}

func TestChildEnvForWindowsSystemRoot(t *testing.T) {
	const home = `C:\Users\me`
	for _, c := range []struct {
		name string
		base []string
		want []string
	}{
		{"lower-case key", []string{`systemroot=D:\Win`, `pathext=.EXE`, `comspec=D:\Win\system32\cmd.exe`},
			[]string{`systemroot=D:\Win`, `pathext=.EXE`, `comspec=D:\Win\system32\cmd.exe`, `HOME=` + home,
				`PATH=C:\Users\me\.local\bin;D:\Win\System32;D:\Win`, `USERPROFILE=` + home}},
		{"mixed-case key", []string{`SystemRoot=E:\NT`},
			[]string{`SystemRoot=E:\NT`, `HOME=` + home, `PATH=C:\Users\me\.local\bin;E:\NT\System32;E:\NT`, `USERPROFILE=` + home}},
		{"no SYSTEMROOT: the binary folder alone", []string{`PATH=C:\Other`},
			[]string{`HOME=` + home, `PATH=C:\Users\me\.local\bin`, `USERPROFILE=` + home}},
		{"empty SYSTEMROOT counts as none", []string{`SYSTEMROOT=`},
			[]string{`SYSTEMROOT=`, `HOME=` + home, `PATH=C:\Users\me\.local\bin`, `USERPROFILE=` + home}},
	} {
		t.Run(c.name, func(t *testing.T) {
			if got := childEnvFor("windows", ";", c.base, home, "claude", winBinary); !reflect.DeepEqual(got, c.want) {
				t.Fatalf("got %q, want %q", got, c.want)
			}
		})
	}
}

// The Windows keys already passed stay passed, the person's own PATH, HOME and
// USERPROFILE do not, and secrets stay out.
func TestChildEnvForWindowsKeys(t *testing.T) {
	base := []string{`PATH=C:\Other`, `Path=C:\Other2`, `USERPROFILE=C:\Old`, `UserProfile=C:\Old2`, `USERNAME=me`, `APPDATA=C:\A`, `LOCALAPPDATA=C:\L`,
		`TEMP=C:\T`, `TMP=C:\T`, `LANG=C`, `ANTHROPIC_API_KEY=secret`, `UNRELATED=no`, `CLAUDE_CONFIG_DIR=cc`, `CODEX_HOME=ch`}
	got := childEnvFor("windows", ";", base, `C:\Users\me`, "codex", winBinary)
	want := []string{`USERNAME=me`, `APPDATA=C:\A`, `LOCALAPPDATA=C:\L`, `TEMP=C:\T`, `TMP=C:\T`, `LANG=C`, `CODEX_HOME=ch`,
		`HOME=C:\Users\me`, `PATH=C:\Users\me\.local\bin`, `USERPROFILE=C:\Users\me`}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %q, want %q", got, want)
	}
}

// filepath.Dir on a Linux host does not split on a backslash, so on windows the
// folder is cut at the last backslash or slash by hand.
func TestChildEnvForWindowsBinaryFolder(t *testing.T) {
	for binary, folder := range map[string]string{
		`C:\Users\me\.local\bin\claude.exe`: `C:\Users\me\.local\bin`,
		`C:/Users/me/bin/claude.exe`:        `C:/Users/me/bin`,
		`C:\Program Files\x/claude.exe`:     `C:\Program Files\x`,
		`C:\claude.exe`:                     `C:\`,
		`\claude.exe`:                       `\`,
		`\\srv\share\bin\claude.exe`:        `\\srv\share\bin`,
		`claude.exe`:                        `.`,
	} {
		got := childEnvFor("windows", ";", nil, `C:\h`, "claude", binary)
		if want := []string{`HOME=C:\h`, `PATH=` + folder, `USERPROFILE=C:\h`}; !reflect.DeepEqual(got, want) {
			t.Errorf("binary %q: got %q, want %q", binary, got, want)
		}
	}
}
