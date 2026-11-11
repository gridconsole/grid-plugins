package agent

import (
	"os"
	"reflect"
	"runtime"
	"slices"
	"strings"
	"testing"
)

// jzAgFake answers from a map and records every name it was asked for.
type jzAgFake struct {
	values map[string]string
	asked  []string
}

func (f *jzAgFake) get(name string) string {
	f.asked = append(f.asked, name)
	return f.values[name]
}

// jzAgHas reports whether env holds the exact NAME=value line.
func jzAgHas(env []string, line string) bool { return slices.Contains(env, line) }

// jzAgHasKey reports whether env holds a line for the key, whatever the value.
func jzAgHasKey(env []string, key string) bool {
	for _, entry := range env {
		if k, _, _ := strings.Cut(entry, "="); k == key {
			return true
		}
	}
	return false
}

// Rows 1 and 2: the Call's variables reach the child and the host's do not; a
// name outside the passed list is not read into the child; HOME is the Env's.
func TestJzAgChildEnvIsTheCallsNotTheHosts(t *testing.T) {
	t.Setenv("CLAUDE_CONFIG_DIR", "/host")
	t.Setenv("LANG", "host")
	t.Setenv("SECRET_TOKEN", "host-secret")
	home := t.TempDir()
	fake := &jzAgFake{values: map[string]string{
		"CLAUDE_CONFIG_DIR": "/call", "LANG": "call", "SECRET_TOKEN": "x", "HOME": "/call-home",
	}}
	got := Env{Home: home, Getenv: fake.get}.childEnv("claude", "/opt/bin/claude")
	for _, want := range []string{"CLAUDE_CONFIG_DIR=/call", "LANG=call", "HOME=" + home} {
		if !jzAgHas(got, want) {
			t.Errorf("child env lacks %q: %q", want, got)
		}
	}
	for _, line := range got {
		if strings.Contains(line, "/host") || line == "LANG=host" || strings.Contains(line, "host-secret") {
			t.Errorf("child env holds a host value %q: %q", line, got)
		}
	}
	if jzAgHasKey(got, "SECRET_TOKEN") {
		t.Errorf("child env holds SECRET_TOKEN: %q", got)
	}
	if n := jzAgCountKey(got, "HOME"); n != 1 {
		t.Errorf("HOME appears %d times, want once, the Env's Home: %q", n, got)
	}
}

func jzAgCountKey(env []string, key string) int {
	n := 0
	for _, entry := range env {
		if k, _, _ := strings.Cut(entry, "="); k == key {
			n++
		}
	}
	return n
}

// The builder asks for the fixed list and nothing else, and writes only the
// non-empty answers. On a non-windows goos it never asks for a Windows name.
func TestJzAgGetenvAsksOnlyTheFixedList(t *testing.T) {
	fake := &jzAgFake{values: map[string]string{"USER": "u", "LANG": "", "SYSTEMROOT": `C:\Windows`}}
	got := baseFromGetenv("linux", fake.get)
	want := []string{"USER=u"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("linux base %q, want %q", got, want)
	}
	wantAsked := []string{"USER", "LOGNAME", "LANG", "TMPDIR", "SHELL", "LC_ALL", "LC_CTYPE", "LC_MESSAGES", "CLAUDE_CONFIG_DIR", "CODEX_HOME"}
	if !reflect.DeepEqual(fake.asked, wantAsked) {
		t.Errorf("linux asked %q, want %q", fake.asked, wantAsked)
	}
	fake = &jzAgFake{}
	baseFromGetenv("windows", fake.get)
	wantAsked = append(wantAsked, "USERNAME", "APPDATA", "LOCALAPPDATA", "SYSTEMROOT", "TEMP", "TMP", "PATHEXT", "COMSPEC")
	if !reflect.DeepEqual(fake.asked, wantAsked) {
		t.Errorf("windows asked %q, want %q", fake.asked, wantAsked)
	}
}

// Only the config variable of the probed agent goes through, and only the three
// LC_ names asked for are read.
func TestJzAgGetenvConfigDirFollowsTheAgent(t *testing.T) {
	fake := &jzAgFake{values: map[string]string{
		"CLAUDE_CONFIG_DIR": "/call-claude", "CODEX_HOME": "/call-codex",
		"LC_ALL": "C", "LC_CTYPE": "C.UTF-8", "LC_MESSAGES": "en", "LC_TIME": "nl",
	}}
	e := Env{Home: "/h", Getenv: fake.get}
	claude := e.childEnv("claude", "/b/claude")
	if !jzAgHas(claude, "CLAUDE_CONFIG_DIR=/call-claude") || jzAgHasKey(claude, "CODEX_HOME") {
		t.Errorf("claude env %q", claude)
	}
	codex := e.childEnv("codex", "/b/codex")
	if !jzAgHas(codex, "CODEX_HOME=/call-codex") || jzAgHasKey(codex, "CLAUDE_CONFIG_DIR") {
		t.Errorf("codex env %q", codex)
	}
	for _, line := range []string{"LC_ALL=C", "LC_CTYPE=C.UTF-8", "LC_MESSAGES=en"} {
		if !jzAgHas(claude, line) {
			t.Errorf("claude env lacks %q: %q", line, claude)
		}
	}
	if jzAgHasKey(claude, "LC_TIME") {
		t.Errorf("LC_TIME is not asked for, but the env holds it: %q", claude)
	}
}

// Row 3: the windows goos through the same builder. SYSTEMROOT from the fake
// gives the System32 entries of childEnvFor, and a host SYSTEMROOT does not.
func TestJzAgWindowsSystemRootFromTheCall(t *testing.T) {
	t.Setenv("SYSTEMROOT", `D:\HostWin`)
	t.Setenv("APPDATA", `D:\HostApp`)
	const home = `C:\Users\me`
	fake := &jzAgFake{values: map[string]string{
		"SYSTEMROOT": `C:\Windows`, "PATHEXT": ".COM;.EXE", "COMSPEC": `C:\Windows\system32\cmd.exe`,
		"USERNAME": "me", "APPDATA": `C:\A`, "LOCALAPPDATA": `C:\L`, "TEMP": `C:\T`, "TMP": `C:\T`,
		"USERPROFILE": `C:\Old`, "SECRET_TOKEN": "x",
	}}
	got := Env{Home: home, Getenv: fake.get}.childEnvOn("windows", ";", "claude", winBinary)
	want := []string{
		`USERNAME=me`, `APPDATA=C:\A`, `LOCALAPPDATA=C:\L`, `SYSTEMROOT=C:\Windows`, `TEMP=C:\T`, `TMP=C:\T`,
		`PATHEXT=.COM;.EXE`, `COMSPEC=C:\Windows\system32\cmd.exe`,
		`HOME=` + home, `PATH=C:\Users\me\.local\bin;C:\Windows\System32;C:\Windows`, `USERPROFILE=` + home,
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("windows env\n got %q\nwant %q", got, want)
	}
	if strings.Contains(strings.Join(got, "\n"), "HostWin") || strings.Contains(strings.Join(got, "\n"), "HostApp") {
		t.Errorf("windows env holds a host value: %q", got)
	}
}

// Row 3, absent case: no SYSTEMROOT from the call leaves the binary folder alone.
func TestJzAgWindowsNoSystemRootFromTheCall(t *testing.T) {
	t.Setenv("SYSTEMROOT", `D:\HostWin`)
	const home = `C:\Users\me`
	fake := &jzAgFake{values: map[string]string{}}
	got := Env{Home: home, Getenv: fake.get}.childEnvOn("windows", ";", "claude", winBinary)
	want := []string{`HOME=` + home, `PATH=C:\Users\me\.local\bin`, `USERPROFILE=` + home}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("windows env %q, want %q", got, want)
	}
}

// Row 4: Getenv nil and Base nil is the binary path, os.Environ as before.
func TestJzAgNilGetenvNilBaseKeepsOsEnviron(t *testing.T) {
	t.Setenv("CLAUDE_CONFIG_DIR", "/host")
	t.Setenv("LANG", "host")
	home := t.TempDir()
	got := Env{Home: home}.childEnv("claude", "/opt/bin/claude")
	want := childEnvFor(runtime.GOOS, string(os.PathListSeparator), os.Environ(), home, "claude", "/opt/bin/claude")
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("childEnv\n got %q\nwant %q", got, want)
	}
	if !jzAgHas(got, "CLAUDE_CONFIG_DIR=/host") || !jzAgHas(got, "LANG=host") {
		t.Errorf("the binary path lost the host values: %q", got)
	}
}

// Base still wins when both are set, so the existing test seam is unchanged.
func TestJzAgBaseWinsOverGetenv(t *testing.T) {
	fake := &jzAgFake{values: map[string]string{"LANG": "call"}}
	got := Env{Home: "/h", Base: []string{"LANG=base"}, Getenv: fake.get}.childEnv("claude", "/b/claude")
	if !jzAgHas(got, "LANG=base") || jzAgHas(got, "LANG=call") {
		t.Errorf("Base did not win: %q", got)
	}
	if len(fake.asked) != 0 {
		t.Errorf("Getenv was asked %q although Base is set", fake.asked)
	}
}
