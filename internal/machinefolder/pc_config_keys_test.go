package machinefolder_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/machinefolder"
)

// The two keys are machineFolder and sharedGroup, like every other key of the one
// config. The snake_case spellings machine_folder and shared_group are not read: Grid
// is not released, so no config holds them (the owner's decision for P-shared-home-5).
// These replace TestResolveBadConfigFallsBack and TestWriteConfigKeepsOtherKeys, which
// held the old spellings.

func TestResolveBadConfigFallsBackCamelCase(t *testing.T) {
	home := t.TempDir()
	e := envOf(home, "linux", nil)
	path := machinefolder.ConfigPath(e)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	for name, body := range map[string]string{
		"array":              `["machineFolder"]`,
		"not json":           `machineFolder: /srv/grid`,
		"folder is a number": `{"machineFolder": 7}`,
		"group is a number":  `{"machineFolder": "/srv/grid", "sharedGroup": 7}`,
	} {
		t.Run(name, func(t *testing.T) {
			if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
				t.Fatal(err)
			}
			f := machinefolder.Resolve(e)
			if f.Path != machinefolder.DefaultPath(e) || f.Problem == "" {
				t.Fatalf("a config that cannot be read falls back to the default and says why: %+v", f)
			}
			if strings.Contains(f.Problem, "/srv/grid") {
				t.Fatalf("the problem must not echo the file's content: %s", f.Problem)
			}
		})
	}
}

func TestOldSnakeCaseKeysAreNotRead(t *testing.T) {
	home := t.TempDir()
	e := envOf(home, "linux", nil)
	path := machinefolder.ConfigPath(e)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(`{"machine_folder": "/srv/grid", "shared_group": "grid"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	f := machinefolder.Resolve(e)
	if f.Path != machinefolder.DefaultPath(e) || f.Source != machinefolder.SourceDefault || f.Shared() || f.Problem != "" {
		t.Fatalf("the old spellings name nothing: %+v", f)
	}
}

func TestWriteConfigKeepsOtherKeysCamelCase(t *testing.T) {
	home := t.TempDir()
	e := envOf(home, "linux", nil)
	path := machinefolder.ConfigPath(e)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	before := `{"other": {"a": [1, 2]}, "machineFolder": "/old", "name": "keep", "workspaces": [{"name": "acme", "folder": "/w"}], "current": "acme"}`
	if err := os.WriteFile(path, []byte(before), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := machinefolder.WriteConfig(e, machinefolder.Config{MachineFolder: "/srv/grid", SharedGroup: "grid"})
	if err != nil || got != path {
		t.Fatalf("WriteConfig = %q, %v", got, err)
	}
	b, _ := os.ReadFile(path)
	text := string(b)
	for _, want := range []string{`"other"`, `"name": "keep"`, `"machineFolder": "/srv/grid"`, `"sharedGroup": "grid"`, `"acme"`, `"current": "acme"`} {
		if !strings.Contains(text, want) {
			t.Fatalf("config lost %s:\n%s", want, text)
		}
	}
	cfg, err := machinefolder.ReadConfig(e)
	if err != nil || cfg.MachineFolder != "/srv/grid" || cfg.SharedGroup != "grid" {
		t.Fatalf("ReadConfig = %+v, %v", cfg, err)
	}
	if st, err := os.Stat(path); err != nil || st.Mode().Perm()&0o077 != 0 && !isWindows() {
		t.Fatalf("the config is private to the login: %v %v", st, err)
	}

	// An empty value removes the key.
	if _, err := machinefolder.WriteConfig(e, machinefolder.Config{}); err != nil {
		t.Fatal(err)
	}
	b, _ = os.ReadFile(path)
	if strings.Contains(string(b), "sharedGroup") || strings.Contains(string(b), "machineFolder") || !strings.Contains(string(b), `"other"`) {
		t.Fatalf("clearing must drop only the two keys:\n%s", b)
	}
}

// A config that is still the older ~/.grid/config.json is adopted, so a write of the two
// keys never leaves the person's workspaces behind in a file the readers no longer reach.
func TestWriteConfigAdoptsTheOlderConfig(t *testing.T) {
	home := t.TempDir()
	e := envOf(home, "linux", nil)
	old := filepath.Join(home, ".grid", "config.json")
	if err := os.MkdirAll(filepath.Dir(old), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(old, []byte(`{"v": 3, "current": "acme", "workspaces": [{"name": "acme"}]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	path, err := machinefolder.WriteConfig(e, machinefolder.Config{MachineFolder: "/srv/grid"})
	if err != nil || path != machinefolder.ConfigPath(e) {
		t.Fatalf("WriteConfig = %q, %v", path, err)
	}
	b, _ := os.ReadFile(path)
	if !strings.Contains(string(b), `"acme"`) || !strings.Contains(string(b), `"machineFolder": "/srv/grid"`) {
		t.Fatalf("the new config must hold the older one's workspaces and the key:\n%s", b)
	}
	if kept, _ := os.ReadFile(old); !strings.Contains(string(kept), `"acme"`) {
		t.Fatalf("the older file stays as it was: %s", kept)
	}
}
