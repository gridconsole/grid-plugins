package gridhome

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// pcExpect is a contract/gridconfig case's expect.json.
type pcExpect struct {
	Data   string            `json:"data"`
	Source Source            `json:"source"`
	Member string            `json:"member"`
	Env    map[string]string `json:"env"`
}

// TestPcResolveContractCases resolves every contract/gridconfig case with no
// GRID_HOME: the v3 and v2 shapes by the current entry, the v1 shape by the older
// desktop's rules (rows 3 and 4), and no or a broken config to ~/.grid.
func TestPcResolveContractCases(t *testing.T) {
	root := filepath.Join("..", "..", "contract", "gridconfig")
	dirs, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	seen := 0
	for _, d := range dirs {
		if !d.IsDir() {
			continue
		}
		seen++
		t.Run(d.Name(), func(t *testing.T) {
			home := t.TempDir()
			q, _ := json.Marshal(home)
			tilde := func(s string) string { return strings.ReplaceAll(s, "~", strings.Trim(string(q), `"`)) }
			b, err := os.ReadFile(filepath.Join(root, d.Name(), "expect.json"))
			if err != nil {
				t.Fatal(err)
			}
			var want pcExpect
			if err := json.Unmarshal([]byte(tilde(string(b))), &want); err != nil {
				t.Fatal(err)
			}
			if cfg, err := os.ReadFile(filepath.Join(root, d.Name(), "config.json")); err == nil {
				if err := os.MkdirAll(filepath.Join(home, ".grid"), 0o700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(home, ".grid", "config.json"), []byte(tilde(string(cfg))), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			getenv := func(k string) string { return want.Env[k] }
			got := Resolve(getenv, home)
			if got.Dir != filepath.FromSlash(want.Data) || got.Source != want.Source || got.Member != want.Member {
				t.Fatalf("Resolve = dir %q source %q member %q, want %q %q %q (problem %q)", got.Dir, got.Source, got.Member, want.Data, want.Source, want.Member, got.ConfigProblem)
			}
			if got.Machine != filepath.Join(home, ".grid") {
				t.Fatalf("Machine = %q", got.Machine)
			}
		})
	}
	if seen < 6 {
		t.Fatalf("found %d cases, want at least 6", seen)
	}
}

// Row (2) at the resolver: GRID_HOME wins over a config that names another folder.
func TestPcResolveEnvWinsOverConfig(t *testing.T) {
	home := t.TempDir()
	if err := os.MkdirAll(filepath.Join(home, ".grid"), 0o700); err != nil {
		t.Fatal(err)
	}
	cfg := `{"v":3,"current":"acme","workspaces":[{"name":"acme","member":"sam"}]}`
	if err := os.WriteFile(filepath.Join(home, ".grid", "config.json"), []byte(cfg), 0o600); err != nil {
		t.Fatal(err)
	}
	env := filepath.Join(home, "other")
	got := Resolve(func(k string) string {
		if k == "GRID_HOME" {
			return env
		}
		return ""
	}, home)
	if got.Dir != env || got.Source != SourceEnv || got.Workspace != "acme" {
		t.Fatalf("got %+v", got)
	}
}

// The writer and the lookup agree: data in ~/.grid/<name> puts the launcher in ~/.grid,
// a GRID_HOME elsewhere keeps it in that folder, and the lookup finds the machine
// folder's launcher first.
func TestPcLauncherDir(t *testing.T) {
	home := t.TempDir()
	machine := filepath.Join(home, ".grid")
	cases := []struct {
		h    Home
		want string
	}{
		{Home{Dir: filepath.Join(machine, "acme"), Source: SourceConfig, Machine: machine}, machine},
		{Home{Dir: filepath.Join(home, "elsewhere"), Source: SourceConfig, Machine: machine}, machine},
		{Home{Dir: filepath.Join(machine, "acme"), Source: SourceEnv, Machine: machine}, machine},
		{Home{Dir: filepath.Join(home, "tmp-home"), Source: SourceEnv, Machine: machine}, filepath.Join(home, "tmp-home")},
		{Home{Dir: machine, Source: SourceDefault, Machine: machine}, machine},
	}
	for _, c := range cases {
		if got := c.h.LauncherDir(); got != c.want {
			t.Errorf("%+v: LauncherDir = %q, want %q", c.h, got, c.want)
		}
	}
	data := filepath.Join(machine, "acme")
	if got := FindLauncherDir(home, data); got != data {
		t.Fatalf("no machine launcher: got %q, want %q", got, data)
	}
	if err := os.MkdirAll(filepath.Join(machine, "bin"), 0o700); err != nil {
		t.Fatal(err)
	}
	name := "grid"
	if filepath.Separator == '\\' {
		name = "grid.exe"
	}
	if err := os.WriteFile(filepath.Join(machine, "bin", name), []byte("x"), 0o700); err != nil {
		t.Fatal(err)
	}
	if got := FindLauncherDir(home, data); got != machine {
		t.Fatalf("machine launcher: got %q, want %q", got, machine)
	}
}
