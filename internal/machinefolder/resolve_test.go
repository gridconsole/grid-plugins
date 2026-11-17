package machinefolder_test

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/machinefolder"
)

func envOf(home, goos string, vars map[string]string) machinefolder.Env {
	return machinefolder.Env{
		Home: home,
		GOOS: goos,
		Getenv: func(k string) string {
			return vars[k]
		},
	}
}

// Row 1: the folder's path from the default, the config and the env override, per OS.
func TestResolvePerOS(t *testing.T) {
	home := t.TempDir()
	cases := []struct {
		name string
		goos string
		vars map[string]string
		want string
	}{
		{"linux default", "linux", nil, filepath.Join(home, ".local", "state", "grid")},
		{"darwin default", "darwin", nil, filepath.Join(home, "Library", "Application Support", "Grid")},
		{"windows default with LOCALAPPDATA", "windows", map[string]string{"LOCALAPPDATA": filepath.Join(home, "Local")}, filepath.Join(home, "Local", "Grid")},
		{"windows default without it", "windows", nil, filepath.Join(home, "AppData", "Local", "Grid")},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := machinefolder.Resolve(envOf(home, tc.goos, tc.vars))
			if f.Path != tc.want || f.Source != machinefolder.SourceDefault || f.Problem != "" || f.Shared() {
				t.Fatalf("Resolve = %+v, want default %s", f, tc.want)
			}
			if f.Configured() {
				t.Fatalf("the OS standard place in private mode is not a configured folder: %+v", f)
			}
		})
	}
}

func TestConfigPathPerOS(t *testing.T) {
	home := t.TempDir()
	if got, want := machinefolder.ConfigPath(envOf(home, "linux", nil)), filepath.Join(home, ".config", "grid", "config.json"); got != want {
		t.Fatalf("linux config = %s, want %s", got, want)
	}
	if got, want := machinefolder.ConfigPath(envOf(home, "darwin", nil)), filepath.Join(home, ".config", "grid", "config.json"); got != want {
		t.Fatalf("darwin config = %s, want %s", got, want)
	}
	roaming := filepath.Join(home, "Roaming")
	if got, want := machinefolder.ConfigPath(envOf(home, "windows", map[string]string{"APPDATA": roaming})), filepath.Join(roaming, "Grid", "config.json"); got != want {
		t.Fatalf("windows config = %s, want %s", got, want)
	}
}

func TestResolveConfigThenEnv(t *testing.T) {
	home := t.TempDir()
	shared := filepath.Join(t.TempDir(), "grid")
	other := filepath.Join(t.TempDir(), "other")
	e := envOf(home, "linux", nil)
	if _, err := machinefolder.WriteConfig(e, machinefolder.Config{MachineFolder: shared, SharedGroup: "grid"}); err != nil {
		t.Fatal(err)
	}

	f := machinefolder.Resolve(e)
	if f.Path != shared || f.Source != machinefolder.SourceConfig || f.SharedGroup != "grid" || !f.Shared() || !f.Configured() {
		t.Fatalf("config: %+v", f)
	}

	e = envOf(home, "linux", map[string]string{machinefolder.EnvDir: other})
	f = machinefolder.Resolve(e)
	if f.Path != other || f.Source != machinefolder.SourceEnv {
		t.Fatalf("env must win over the config: %+v", f)
	}
	if f.SharedGroup != "grid" {
		t.Fatalf("shared_group comes from the config whichever source named the path: %+v", f)
	}
}

func TestResolveHomeAndRelative(t *testing.T) {
	home := t.TempDir()
	e := envOf(home, "linux", map[string]string{machinefolder.EnvDir: "~/grid-here"})
	if f := machinefolder.Resolve(e); f.Path != filepath.Join(home, "grid-here") || f.Source != machinefolder.SourceEnv {
		t.Fatalf("~/ is the login's home: %+v", f)
	}

	e = envOf(home, "linux", map[string]string{machinefolder.EnvDir: "relative/dir"})
	f := machinefolder.Resolve(e)
	if f.Path != machinefolder.DefaultPath(e) || f.Source != machinefolder.SourceDefault {
		t.Fatalf("a relative path is not used: %+v", f)
	}
	if !strings.Contains(f.Problem, machinefolder.EnvDir) || !f.Configured() {
		t.Fatalf("a relative path is a Problem that names the setting: %+v", f)
	}
}

func TestResolveNoHome(t *testing.T) {
	f := machinefolder.Resolve(envOf("", "linux", nil))
	if f.Path != "" {
		t.Fatalf("with no home and no setting the place is unknown, got %q", f.Path)
	}
	f = machinefolder.Resolve(envOf("", "linux", map[string]string{machinefolder.EnvDir: "/srv/grid"}))
	if f.Path != "/srv/grid" {
		t.Fatalf("an absolute env path needs no home: %+v", f)
	}
}

func TestWriteConfigRefusesAFileThatIsNotAnObject(t *testing.T) {
	home := t.TempDir()
	e := envOf(home, "linux", nil)
	path := machinefolder.ConfigPath(e)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	const mine = "# my own notes, not json\n"
	if err := os.WriteFile(path, []byte(mine), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := machinefolder.WriteConfig(e, machinefolder.Config{SharedGroup: "grid"}); err == nil {
		t.Fatal("WriteConfig replaced a file that is not a JSON object")
	}
	if b, _ := os.ReadFile(path); string(b) != mine {
		t.Fatalf("the person's file changed: %q", b)
	}
}

func TestWriteConfigNewFileIsPrivate(t *testing.T) {
	if isWindows() {
		t.Skip("no unix permission bits")
	}
	e := envOf(t.TempDir(), "linux", nil)
	path, err := machinefolder.WriteConfig(e, machinefolder.Config{SharedGroup: "grid"})
	if err != nil {
		t.Fatal(err)
	}
	st, err := os.Stat(path)
	if err != nil || st.Mode().Perm() != 0o600 {
		t.Fatalf("a new config is 0600: %v %v", st, err)
	}
	if dst, err := os.Stat(filepath.Dir(path)); err != nil || dst.Mode().Perm()&0o077 != 0 {
		t.Fatalf("a new config folder is private: %v %v", dst, err)
	}
}

// Row 1 (the grep): nothing but this package computes the machine folder's path or
// reads its two config keys.
func TestOneResolver(t *testing.T) {
	root := filepath.Join("..", "..")
	needles := []string{machinefolder.EnvDir, `"` + machinefolder.KeyMachineFolder + `"`, `".local", "state", "grid"`}
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			switch d.Name() {
			case ".git", "node_modules", "tmp":
				return fs.SkipDir
			case "machinefolder":
				return fs.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for _, n := range needles {
			if strings.Contains(string(b), n) {
				t.Errorf("%s computes the machine folder itself (%s); call machinefolder.Resolve", path, n)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func isWindows() bool { return os.PathSeparator == '\\' }
