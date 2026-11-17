package gridhome

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/gridconfig"
	"github.com/gridconsole/grid-plugins/internal/machinefolder"
)

func oneFolderEnv(home string, vars map[string]string) func(string) string {
	return func(k string) string { return vars[k] }
}

func writeMachineConfig(t *testing.T, home, body string) {
	t.Helper()
	path := gridconfig.PathFor("linux", func(string) string { return "" }, home)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

// A config that holds only machineFolder gives one folder from every reader of it:
// machinefolder.Resolve, Home.Machine and Config.MachineDir.
func TestConfigWithOnlyMachineFolderGivesOneFolderFromEveryReader(t *testing.T) {
	for name, key := range map[string]func(home string) string{
		"absolute": func(string) string { return filepath.Join(t.TempDir(), "shared") },
		"home":     func(string) string { return "~/shared" },
	} {
		t.Run(name, func(t *testing.T) {
			home := t.TempDir()
			value := key(home)
			writeMachineConfig(t, home, `{"machineFolder": "`+filepath.ToSlash(value)+`"}`)
			none := oneFolderEnv(home, nil)

			folder := machinefolder.Resolve(machinefolder.Env{Getenv: none, Home: home, GOOS: "linux"})
			got := resolve("linux", none, home)
			l := gridconfig.Load(gridconfig.PathFor("linux", none, home), "")
			dir, fromConfig := l.Config.MachineDir("linux", none, home)

			want := filepath.Clean(value)
			if value == "~/shared" {
				want = filepath.Join(home, "shared")
			}
			if folder.Path != want || folder.Source != machinefolder.SourceConfig {
				t.Fatalf("machinefolder.Resolve = %+v, want %s from the config", folder, want)
			}
			if got.Machine != want || got.MachineSource != SourceConfig {
				t.Fatalf("Home.Machine = %q (%s), want %s (config)", got.Machine, got.MachineSource, want)
			}
			if dir != want || !fromConfig {
				t.Fatalf("Config.MachineDir = %q (%v), want %s from the config", dir, fromConfig, want)
			}
		})
	}
}

// GRID_MACHINE_DIR wins in Home.Machine as it does in machinefolder.Resolve, and the
// source says env.
func TestHomeMachineFollowsTheEnvOverride(t *testing.T) {
	home, shared, other := t.TempDir(), filepath.Join(t.TempDir(), "shared"), filepath.Join(t.TempDir(), "other")
	writeMachineConfig(t, home, `{"machineFolder": "`+filepath.ToSlash(shared)+`"}`)
	vars := map[string]string{machinefolder.EnvDir: other}
	got := resolve("linux", oneFolderEnv(home, vars), home)
	if got.Machine != other || got.MachineSource != SourceEnv {
		t.Fatalf("Home.Machine = %q (%s), want %s (env)", got.Machine, got.MachineSource, other)
	}
}

// A path that is not configured does not move: with no config and no env the folder
// is the OS standard place, Home.Machine says default, and the folder is not Active, so
// every file stays where it was.
func TestUnconfiguredMachineFolderStaysPut(t *testing.T) {
	home := t.TempDir()
	none := oneFolderEnv(home, nil)
	got := resolve("linux", none, home)
	want := filepath.Join(home, ".local", "state", "grid")
	if got.Machine != want || got.MachineSource != SourceDefault {
		t.Fatalf("Home.Machine = %q (%s), want %s (default)", got.Machine, got.MachineSource, want)
	}
	if folder := machinefolder.Resolve(machinefolder.Env{Getenv: none, Home: home, GOOS: "linux"}); folder.Active() {
		t.Fatalf("the OS standard place in private mode must not be Active: %+v", folder)
	}

	// A relative machineFolder is not used: the default stays, the source says default.
	writeMachineConfig(t, home, `{"machineFolder": "relative/dir"}`)
	got = resolve("linux", none, home)
	if got.Machine != want || got.MachineSource != SourceDefault {
		t.Fatalf("a relative machineFolder must leave %s, got %q (%s)", want, got.Machine, got.MachineSource)
	}
}
