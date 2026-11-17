package hook

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/gridconfig"
	"github.com/gridconsole/grid-plugins/internal/where"
)

// Row (6): with the data folder in ~/.grid/<name>, SessionStart puts the launcher of
// the machine folder (the OS standard place) on PATH; with none there, the older
// ~/.grid/bin; never the plugin root's bin while one of them is there.
func TestPcMachineLauncherWithDataElsewhere(t *testing.T) {
	user := t.TempDir()
	machineBin := filepath.Join(gridconfig.MachineDirFor(runtime.GOOS, func(string) string { return "" }, user), "bin")
	if runtime.GOOS == "windows" {
		t.Skip("the env file's PATH line is written for a POSIX shell")
	}
	// Only ~/.grid/bin first, then the machine folder's too.
	pcSessionPath(t, user, filepath.Join(user, ".grid", "bin"))
	pcSessionPath(t, user, machineBin)
}

// pcSessionPath writes a launcher in bin and runs SessionStart with the data folder
// in ~/.grid/acme; the env file must put bin on PATH.
func pcSessionPath(t *testing.T, user, bin string) {
	t.Helper()
	s := voPathNew(t, "data")
	data := filepath.Join(user, ".grid", "acme")
	_ = os.RemoveAll(data)
	if err := os.MkdirAll(filepath.Dir(data), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(s.home, data); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(bin, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bin, "grid"), []byte("#!/bin/sh\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	o := Options{
		Event:    "sessionstart",
		Cwd:      filepath.Join(user, "work"),
		Stdout:   &strings.Builder{},
		Resolver: where.Resolver{GridHome: data, Home: user, Machine: voPathMachine},
		Getenv: func(key string) string {
			switch key {
			case "CLAUDE_ENV_FILE":
				return s.envFile
			case "CLAUDE_PLUGIN_ROOT":
				return s.root
			case "PATH":
				return "/usr/bin"
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
	if want := voPathQuoted(bin); !strings.Contains(string(b), want) {
		t.Fatalf("want %q in env file:\n%s", want, b)
	}
}
