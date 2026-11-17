package hook

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/where"
)

// Row (6): with the data folder in ~/.grid/<name>, SessionStart puts the launcher in
// the machine folder ~/.grid/bin on PATH, not the plugin root's bin.
func TestPcMachineLauncherWithDataElsewhere(t *testing.T) {
	s := voPathNew(t, "data")
	user := filepath.Dir(s.home)
	data := filepath.Join(user, ".grid", "acme")
	if err := os.Rename(s.home, data); err != nil {
		if err := os.MkdirAll(filepath.Dir(data), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.Rename(s.home, data); err != nil {
			t.Fatal(err)
		}
	}
	bin := filepath.Join(user, ".grid", "bin")
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
