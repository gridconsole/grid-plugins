package gridhome

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/gridconfig"
)

// pcExpect is a contract/gridconfig case's expect.json.
type pcExpect struct {
	Data    string            `json:"data"`
	Source  Source            `json:"source"`
	Member  string            `json:"member"`
	Brain   string            `json:"brain"`
	Machine string            `json:"machine"`
	Copied  bool              `json:"copied"`
	Env     map[string]string `json:"env"`
}

// pcPlace writes a fixture file at path with "~" read as home; it reports whether
// the fixture has the file.
func pcPlace(t *testing.T, src, path, home string) ([]byte, bool) {
	t.Helper()
	b, err := os.ReadFile(src)
	if err != nil {
		return nil, false
	}
	q, _ := json.Marshal(home)
	text := []byte(strings.ReplaceAll(string(b), "~", strings.Trim(string(q), `"`)))
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, text, 0o600); err != nil {
		t.Fatal(err)
	}
	return text, true
}

// TestPcResolveContractCases resolves every contract/gridconfig case with no
// GRID_HOME: config.json at the config path, old-config.json at ~/.grid/config.json
// (copied once to the config path when that has no file), the v3 and v2 shapes by
// the current entry, the v1 shape by the older desktop's rules (rows 3 and 4), and
// no or a broken config to ~/.grid.
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
			getenv := func(k string) string { return want.Env[k] }
			path := gridconfig.Path(getenv, home)
			old := filepath.Join(home, ".grid", "config.json")
			newText, hasNew := pcPlace(t, filepath.Join(root, d.Name(), "config.json"), path, home)
			oldText, hasOld := pcPlace(t, filepath.Join(root, d.Name(), "old-config.json"), old, home)

			got := Resolve(getenv, home)
			if got.Dir != filepath.FromSlash(want.Data) || got.Source != want.Source || got.Member != want.Member || got.Brain != filepath.FromSlash(want.Brain) {
				t.Fatalf("Resolve = dir %q source %q member %q brain %q, want %q %q %q %q (problem %q)", got.Dir, got.Source, got.Member, got.Brain, want.Data, want.Source, want.Member, want.Brain, got.ConfigProblem)
			}
			wantMachine, wantMachineSource := filepath.FromSlash(want.Machine), SourceConfig
			if want.Machine == "" {
				wantMachine, wantMachineSource = gridconfig.MachineDirFor(runtime.GOOS, getenv, home), SourceDefault
			}
			if got.Machine != wantMachine || got.MachineSource != wantMachineSource {
				t.Fatalf("Machine = %q (%s), want %q (%s)", got.Machine, got.MachineSource, wantMachine, wantMachineSource)
			}
			if got.DataRoot != filepath.Join(home, ".grid") {
				t.Fatalf("DataRoot = %q", got.DataRoot)
			}
			if (got.ConfigCopiedFrom != "") != want.Copied {
				t.Fatalf("ConfigCopiedFrom = %q, want copied %v", got.ConfigCopiedFrom, want.Copied)
			}
			if hasOld {
				if b, err := os.ReadFile(old); err != nil || string(b) != string(oldText) {
					t.Fatalf("the older ~/.grid/config.json changed or went: %q %v", b, err)
				}
			}
			switch b, err := os.ReadFile(path); {
			case hasNew && string(b) != string(newText):
				t.Fatalf("the config file changed: %s", b)
			case !hasNew && want.Copied && string(b) != string(oldText):
				t.Fatalf("the copy differs from the old file: %q %v", b, err)
			case !hasNew && !want.Copied && err == nil:
				t.Fatalf("a config file was made: %s", b)
			}
			if (hasNew || hasOld) && got.ConfigProblem == "" && got.Config != path {
				t.Fatalf("Config = %q, want %q", got.Config, path)
			}
		})
	}
	if seen < 9 {
		t.Fatalf("found %d cases, want at least 9", seen)
	}
}

// Row (2) at the resolver: GRID_HOME wins over a config that names another folder;
// it picks the workspace whose data folder it names, and its brain, or none.
func TestPcResolveEnvWinsOverConfig(t *testing.T) {
	home := t.TempDir()
	path := gridconfig.Path(func(string) string { return "" }, home)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	beta := filepath.Join(home, "beta-data")
	brain := filepath.Join(home, "Brain")
	cfg, _ := json.Marshal(map[string]any{"v": 3, "current": "acme", "workspaces": []map[string]any{
		{"name": "acme", "member": "sam"},
		{"name": "beta", "data": beta, "brain": brain},
	}})
	if err := os.WriteFile(path, cfg, 0o600); err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct{ env, workspace, brain string }{
		{filepath.Join(home, "other"), "", ""},
		{beta, "beta", brain},
	} {
		got := Resolve(func(k string) string {
			if k == "GRID_HOME" {
				return c.env
			}
			return ""
		}, home)
		if got.Dir != c.env || got.Source != SourceEnv || got.Workspace != c.workspace || got.Brain != c.brain || got.Member != "sam" {
			t.Fatalf("GRID_HOME %s: got %+v", c.env, got)
		}
	}
}

// The machine folder on macOS and Linux sits at the OS standard place without a
// config, and where the config's machineFolder says with one; the config is read
// from ~/.config/grid/config.json on both.
func TestPcMachineFolderMacAndLinux(t *testing.T) {
	none := func(string) string { return "" }
	for goos, want := range map[string]string{
		"darwin": filepath.Join("Library", "Application Support", "Grid"),
		"linux":  filepath.Join(".local", "state", "grid"),
	} {
		home := t.TempDir()
		got := resolve(goos, none, home)
		if got.Machine != filepath.Join(home, want) || got.MachineSource != SourceDefault || got.Dir != filepath.Join(home, ".grid") {
			t.Fatalf("%s without a config: %+v", goos, got)
		}
		path := gridconfig.PathFor(goos, none, home)
		if path != filepath.Join(home, ".config", "grid", "config.json") {
			t.Fatalf("%s: config path %q", goos, path)
		}
		shared := filepath.Join(home, "shared", "grid")
		cfg, _ := json.Marshal(map[string]any{"v": 3, "machineFolder": shared})
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, cfg, 0o600); err != nil {
			t.Fatal(err)
		}
		got = resolve(goos, none, home)
		if got.Machine != shared || got.MachineSource != SourceConfig || got.Config != path {
			t.Fatalf("%s with machineFolder: %+v", goos, got)
		}
	}
	appdata := func(k string) string {
		return map[string]string{"APPDATA": `C:\U\s\AppData\Roaming`, "LOCALAPPDATA": `C:\U\s\AppData\Local`}[k]
	}
	if got := gridconfig.PathFor("windows", appdata, `C:\U\s`); got != filepath.Join(`C:\U\s\AppData\Roaming`, "Grid", "config.json") {
		t.Fatalf("windows config path %q", got)
	}
	if got := gridconfig.MachineDirFor("windows", appdata, `C:\U\s`); got != filepath.Join(`C:\U\s\AppData\Local`, "Grid") {
		t.Fatalf("windows machine folder %q", got)
	}
}

// The writer and the lookup agree: a home of this person's (a config workspace, or
// ~/.grid) points the machine folder's launcher at the running grid too, and
// ~/.grid's when it is there; a GRID_HOME no config names keeps only its own. The
// lookup finds the machine folder first, then ~/.grid, then the data folder.
func TestPcLauncherDirs(t *testing.T) {
	home := t.TempDir()
	dot := filepath.Join(home, ".grid")
	machine := gridconfig.MachineDirFor(runtime.GOOS, os.Getenv, home)
	data := filepath.Join(dot, "acme")
	cases := []struct {
		h    Home
		want []string
	}{
		{Home{Dir: data, Source: SourceConfig, Workspace: "acme", Machine: machine, DataRoot: dot}, []string{data, machine}},
		{Home{Dir: data, Source: SourceEnv, Workspace: "acme", Machine: machine, DataRoot: dot}, []string{data, machine}},
		{Home{Dir: filepath.Join(home, "tmp-home"), Source: SourceEnv, Machine: machine, DataRoot: dot}, []string{filepath.Join(home, "tmp-home")}},
		{Home{Dir: dot, Source: SourceDefault, Machine: machine, DataRoot: dot}, []string{dot, machine}},
	}
	for _, c := range cases {
		if got := c.h.LauncherDirs(); strings.Join(got, "|") != strings.Join(c.want, "|") {
			t.Errorf("%+v: LauncherDirs = %q, want %q", c.h, got, c.want)
		}
	}
	none := func(string) string { return "" }
	if got := FindLauncherDir(none, home, data); got != data {
		t.Fatalf("no launcher elsewhere: got %q, want %q", got, data)
	}
	name := "grid"
	if runtime.GOOS == "windows" {
		name = "grid.exe"
	}
	put := func(dir string) {
		if err := os.MkdirAll(filepath.Join(dir, "bin"), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "bin", name), []byte("x"), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	put(dot)
	if got := FindLauncherDir(none, home, data); got != dot {
		t.Fatalf("older ~/.grid launcher: got %q, want %q", got, dot)
	}
	if got := cases[0].h.LauncherDirs(); strings.Join(got, "|") != strings.Join([]string{data, machine, dot}, "|") {
		t.Fatalf("~/.grid launcher there: LauncherDirs = %q", got)
	}
	put(machine)
	if got := FindLauncherDir(none, home, data); got != machine {
		t.Fatalf("machine launcher: got %q, want %q", got, machine)
	}
}

// The name offered when the config names none: the Claude account's displayName,
// from CLAUDE_CONFIG_DIR's .claude.json when set, else ~/.claude.json; else the
// OS login name. It never writes.
func TestPcDefaultMember(t *testing.T) {
	home := t.TempDir()
	env := map[string]string{"USER": "sam"}
	getenv := func(k string) string { return env[k] }
	if name, src := DefaultMember(getenv, home); name != "sam" || src != SourceLogin {
		t.Fatalf("no account: %q %q", name, src)
	}
	write := func(path, display string) {
		b, _ := json.Marshal(map[string]any{"oauthAccount": map[string]string{"displayName": display, "emailAddress": "a@example.com"}})
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, b, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write(filepath.Join(home, ".claude.json"), "Sam Example")
	if name, src := DefaultMember(getenv, home); name != "Sam Example" || src != SourceAccount {
		t.Fatalf("home account: %q %q", name, src)
	}
	other := filepath.Join(home, ".claude-work")
	write(filepath.Join(other, ".claude.json"), " Sam at Work ")
	env["CLAUDE_CONFIG_DIR"] = other
	if name, src := DefaultMember(getenv, home); name != "Sam at Work" || src != SourceAccount {
		t.Fatalf("CLAUDE_CONFIG_DIR account: %q %q", name, src)
	}
	if _, err := os.Stat(gridconfig.Path(getenv, home)); err == nil {
		t.Fatal("DefaultMember wrote a config")
	}
}
