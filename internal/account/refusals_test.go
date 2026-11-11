package account

import (
	"encoding/base64"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// refusalFile reads a file whose bytes a test must see unchanged.
func refusalFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestCodexIdentityRefusesBadAuthJSON(t *testing.T) {
	enc := func(s string) string { return base64.RawURLEncoding.EncodeToString([]byte(s)) }
	for name, auth := range map[string]string{
		"auth.json is not JSON":     `not json`,
		"id_token has two parts":    `{"tokens":{"id_token":"a.b"}}`,
		"payload is not base64url":  `{"tokens":{"id_token":"a.!!!.c"}}`,
		"payload is not JSON":       `{"tokens":{"id_token":"a.` + enc("not json") + `.c"}}`,
		"no id_token at all":        `{"tokens":{}}`,
		"id_token with four parts":  `{"tokens":{"id_token":"a.b.c.d"}}`,
		"payload an empty document": `{"tokens":{"id_token":"a..c"}}`,
	} {
		t.Run(name, func(t *testing.T) {
			if email, plan := codexIdentity([]byte(auth)); email != "" || plan != "" {
				t.Fatalf("want empty, got %q %q", email, plan)
			}
		})
	}
	// An email with no "@" is dropped, the plan claim is kept.
	email, plan := codexIdentity([]byte(`{"tokens":{"id_token":"` + idToken("no-at-sign") + `"}}`))
	if email != "" || plan != "plus" {
		t.Fatalf("no-at-sign: got %q %q, want \"\" and plus", email, plan)
	}
}

func TestEnvGetenvConfigDirAndNow(t *testing.T) {
	dir := t.TempDir()
	if got := (Env{Getenv: func(string) string { return "set" }}).getenv("ANY"); got != "set" {
		t.Fatalf("getenv through Env.Getenv: %q", got)
	}
	env := Env{Getenv: func(string) string { return dir + "/x/../y" }}
	if got, want := ConfigDir(env), filepath.Join(dir, "y"); got != want {
		t.Fatalf("ConfigDir with the variable set: got %q want %q", got, want)
	}
	home := t.TempDir()
	env = Env{Home: home, Getenv: func(string) string { return "" }}
	if got, want := ConfigDir(env), filepath.Join(home, ".claude"); got != want {
		t.Fatalf("ConfigDir with the variable unset: got %q want %q", got, want)
	}
	before := time.Now()
	got := (Env{}).now()
	if got.Before(before.Add(-time.Minute)) || got.After(before.Add(time.Minute)) {
		t.Fatalf("Env{}.now() = %v, not within a minute of %v", got, before)
	}
}

func TestMakeFolderRefusals(t *testing.T) {
	if _, err := MakeFolder(Env{}, "claude", "claude-x"); err == nil || err.Error() != "cannot find the home folder" {
		t.Fatalf("no home: %v", err)
	}
	env, _ := machine(t)
	file := filepath.Join(env.Home, ".claude-x")
	write(t, file, "not a folder")
	_, err := MakeFolder(env, "claude", "claude-x")
	if !errors.Is(err, ErrBadFolder) {
		t.Fatalf("a regular file at ~/.claude-x: want ErrBadFolder, got %v", err)
	}
	if got := refusalFile(t, file); got != "not a folder" {
		t.Fatalf("the file was changed: %q", got)
	}
}

func TestRegistryPathAndNameHelpers(t *testing.T) {
	home := t.TempDir()
	env := Env{Home: home}
	outside := filepath.Join(filepath.Dir(home), "elsewhere")
	if got := (Env{}).tilde(outside); got != outside {
		t.Fatalf("tilde with no Home: %q", got)
	}
	if got := env.tilde(outside); got != outside {
		t.Fatalf("tilde outside Home: %q", got)
	}
	if got := env.tilde(filepath.Dir(home)); got != filepath.Dir(home) {
		t.Fatalf("tilde of Home's parent: %q", got)
	}
	if got := env.tilde(home); got != "~" {
		t.Fatalf("tilde of Home itself: %q", got)
	}
	if got := env.expand("~"); got != home {
		t.Fatalf("expand ~: %q", got)
	}
	if got, want := env.expand("/a/b/../c"), filepath.Clean("/a/b/../c"); got != want {
		t.Fatalf("expand of an absolute path: got %q want %q", got, want)
	}
	if got := slugOf("+++@example.com"); got != "account" {
		t.Fatalf("slugOf: %q", got)
	}
	long := strings.Repeat("a", 30) + "@" + strings.Repeat("b", 20) + ".example.com"
	if got := nameFor(long); got != strings.Repeat("a", 30) {
		t.Fatalf("nameFor of a long email: %q", got)
	}
	if got := nameFor("me@example.com"); got != "me@example.com" {
		t.Fatalf("nameFor of a short email: %q", got)
	}
}

func TestStatusAndWindowsRefuseABrokenRegistry(t *testing.T) {
	if _, err := Status(Env{}, false); err == nil || err.Error() != "cannot find the Grid home; set GRID_HOME" {
		t.Fatalf("no Grid home: %v", err)
	}

	env, _ := machine(t)
	reg := filepath.Join(env.GridHome, "accounts", "registry.json")
	write(t, reg, "not json")
	if _, err := Status(env, false); err == nil || !strings.Contains(err.Error(), "is not readable JSON") || !strings.Contains(err.Error(), "it was left as it is") {
		t.Fatalf("Status on bad JSON: %v", err)
	}
	if _, err := Windows(env, ""); err == nil || !strings.Contains(err.Error(), "is not readable JSON") || !strings.Contains(err.Error(), "it was left as it is") {
		t.Fatalf("Windows on bad JSON: %v", err)
	}
	if got := refusalFile(t, reg); got != "not json" {
		t.Fatalf("registry.json was changed: %q", got)
	}

	// {} is an empty registry, not an error.
	write(t, reg, "{}")
	got, err := Status(env, false)
	if err != nil || len(got) != 0 {
		t.Fatalf("Status on {}: %v %v", got, err)
	}

	// registry.json that is a folder cannot be read.
	env, _ = machine(t)
	if err := os.MkdirAll(filepath.Join(env.GridHome, "accounts", "registry.json"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := Status(env, false); err == nil {
		t.Fatal("registry.json as a folder: want an error")
	}

	// <GridHome>/accounts that is a regular file stops a registering Status.
	env, _ = machine(t)
	accounts := filepath.Join(env.GridHome, "accounts")
	write(t, accounts, "a file, not a folder")
	if _, err := Status(env, true); err == nil {
		t.Fatal("accounts as a file: want an error")
	}
	if got := refusalFile(t, accounts); got != "a file, not a folder" {
		t.Fatalf("accounts file was changed: %q", got)
	}
}

func TestNoteAndSnapshotRefusals(t *testing.T) {
	if err := Note(Env{}, strings.NewReader("{}"), t.TempDir()); err == nil || err.Error() != "cannot find the Grid home; set GRID_HOME" {
		t.Fatalf("Note with no Grid home: %v", err)
	}
	if _, ok := readSnapshot(Env{}, t.TempDir()); ok {
		t.Fatal("readSnapshot with no Grid home: want false")
	}

	env, _ := machine(t)
	home := filepath.Join(env.Home, ".claude")
	write(t, filepath.Join(home, "keep"), "")
	snap := snapshotPath(env, home)

	if err := Note(env, strings.NewReader("not json"), home); err == nil {
		t.Fatal("Note on not json: want an error")
	}
	if _, err := os.Stat(snap); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("Note on not json wrote a snapshot: %v", err)
	}

	for name, body := range map[string]string{"not json": "not json", "no window": "{}"} {
		t.Run(name, func(t *testing.T) {
			write(t, snap, body)
			got, err := Windows(env, "")
			if err != nil || len(got) != 1 {
				t.Fatalf("%+v %v", got, err)
			}
			if w := got[0]; w.Source != "none" || w.Windows == nil || len(w.Windows) != 0 || w.Note == "" {
				t.Fatalf("want no data, got %+v", w)
			}
		})
	}
}
