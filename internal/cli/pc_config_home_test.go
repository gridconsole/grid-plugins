package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/project"
)

// pcConfigHome makes a home folder whose ~/.grid/config.json is the contract
// fixture case, with "~" read as that folder, and clears every variable that would
// pick the Grid home without the config.
func pcConfigHome(t *testing.T, fixture string) string {
	t.Helper()
	home := t.TempDir()
	for _, key := range []string{"GRID_HOME", "GRID_PROJECTS", "USERPROFILE"} {
		t.Setenv(key, "")
	}
	t.Setenv("HOME", home)
	if os.Getenv("USERPROFILE") == "" && filepath.Separator == '\\' {
		t.Setenv("USERPROFILE", home)
	}
	b, err := os.ReadFile(filepath.Join("..", "..", "contract", "gridconfig", fixture, "config.json"))
	if err != nil {
		t.Fatal(err)
	}
	q, _ := json.Marshal(home)
	text := strings.ReplaceAll(string(b), "~", strings.Trim(string(q), `"`))
	if err := os.MkdirAll(filepath.Join(home, ".grid"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, ".grid", "config.json"), []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}
	return home
}

// pcRun runs one grid command with the environment the binary builds for itself.
func pcRun(t *testing.T, args ...string) (string, int) {
	t.Helper()
	env := OSEnv()
	var out, errOut strings.Builder
	env.Stdout, env.Stderr, env.Stdin = &out, &errOut, strings.NewReader("")
	code := Run(args, env)
	return out.String() + errOut.String(), code
}

// Row (1): the owner-shaped config (v2, data in ~/.grid/<name>) makes the brain and
// project reads list what sits in that data folder, and nothing is copied to ~/.grid.
func TestPcConfigDataFolderIsRead(t *testing.T) {
	home := pcConfigHome(t, "v2-owner")
	data := filepath.Join(home, ".grid", "acme")
	work := filepath.Join(home, "workspace", "example")
	if err := os.MkdirAll(work, 0o700); err != nil {
		t.Fatal(err)
	}
	rec := project.Record{Name: "pcexample", ID: "01PCEXAMPLE", Folders: []project.Folder{{Machine: "01PCMACHINE", Path: work}}}
	if err := project.Write(data, rec); err != nil {
		t.Fatal(err)
	}
	// The note is written into the data folder the way a person would, by naming it.
	if out, code := pcRun(t, "--grid-home", data, "brain", "propose", "--level", "home", "--name", "pc-checks", "--topic", "decisions",
		"--description", "Before a merge.", "--summary", "Run the checks.", "--source", "job:T-01/J-01", "--body", "Run the checks.\n"); code != 0 {
		t.Fatalf("propose: %d %s", code, out)
	}
	if out, code := pcRun(t, "--grid-home", data, "brain", "accept", "pc-checks", "--level", "home"); code != 0 {
		t.Fatalf("accept: %d %s", code, out)
	}

	if env := OSEnv(); env.GridHome != data {
		t.Fatalf("OSEnv GridHome = %q, want the config's data folder %q", env.GridHome, data)
	}
	out, code := pcRun(t, "project", "list")
	if code != 0 || !strings.Contains(out, "pcexample") {
		t.Fatalf("project list exit %d, want pcexample listed:\n%s", code, out)
	}
	out, code = pcRun(t, "brain", "topics")
	if code != 0 || !strings.Contains(out, "decisions") {
		t.Fatalf("brain topics exit %d, want the decisions topic:\n%s", code, out)
	}
	out, code = pcRun(t, "brain", "show", "pc-checks", "--level", "home")
	if code != 0 || !strings.Contains(out, "Run the checks.") {
		t.Fatalf("brain show exit %d, want the note's body:\n%s", code, out)
	}
	for _, name := range []string{"projects", "knowledge", "VERSION"} {
		if _, err := os.Stat(filepath.Join(home, ".grid", name)); err == nil {
			t.Fatalf("~/.grid/%s exists: the data was copied or written to ~/.grid", name)
		}
	}
}

// Row (2): GRID_HOME and --grid-home still win over the config.
func TestPcEnvAndFlagWinOverConfig(t *testing.T) {
	home := pcConfigHome(t, "v2-owner")
	envHome := filepath.Join(home, "from-env")
	t.Setenv("GRID_HOME", envHome)
	if got := OSEnv().GridHome; got != envHome {
		t.Fatalf("GRID_HOME: GridHome = %q, want %q", got, envHome)
	}
	flagHome := filepath.Join(home, "from-flag")
	_, env, code := applyGlobalFlags([]string{"--grid-home", flagHome, "version"}, OSEnv())
	if code != 0 || env.GridHome != flagHome {
		t.Fatalf("--grid-home: code %d GridHome = %q, want %q", code, env.GridHome, flagHome)
	}
}

// Row (3): no config and no data gives ~/.grid, as before.
func TestPcNoConfigGivesDotGrid(t *testing.T) {
	home := t.TempDir()
	t.Setenv("GRID_HOME", "")
	t.Setenv("HOME", home)
	if filepath.Separator == '\\' {
		t.Setenv("USERPROFILE", home)
	}
	if got, want := OSEnv().GridHome, filepath.Join(home, ".grid"); got != want {
		t.Fatalf("GridHome = %q, want %q", got, want)
	}
}
