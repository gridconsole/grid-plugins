package cli

import (
	"bytes"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/gridconfig"
	"github.com/gridconsole/grid-plugins/internal/gridhome"
)

// grid init: --detect on every contract/gridconfig case, --answers writing the one
// config, and --workspace, GRID_HOME and --grid-home picking the folder. PATH holds
// no claude or codex, so the account scan reads the folders' own files only.

// cpInitFixtures are the contract/gridconfig cases.
func cpInitFixtures(t *testing.T) []string {
	t.Helper()
	entries, err := os.ReadDir(filepath.Join("..", "..", "contract", "gridconfig"))
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, e := range entries {
		if e.IsDir() {
			out = append(out, e.Name())
		}
	}
	if len(out) < 11 {
		t.Fatalf("found %d gridconfig cases, want the 11 of contract/gridconfig", len(out))
	}
	return out
}

// cpInitExpect is a case's expect.json.
type cpInitExpect struct {
	Data   string            `json:"data"`
	Source string            `json:"source"`
	Member string            `json:"member"`
	Env    map[string]string `json:"env"`
}

// cpInitHome makes a home folder holding the case (none for no case): its
// config.json at gridconfig.Path and its old-config.json at ~/.grid/config.json,
// with "~" read as that folder. It clears every variable that would pick the Grid
// home, the member or an account folder from the machine running the test, and sets
// the case's own.
func cpInitHome(t *testing.T, fixture string) (string, cpInitExpect) {
	t.Helper()
	home := gridhome.Real(t.TempDir())
	for _, key := range []string{"GRID_HOME", "GRID_PROJECTS", "USERPROFILE", "APPDATA", "LOCALAPPDATA",
		"CLAUDE_CONFIG_DIR", "CODEX_HOME", "USER", "USERNAME", "LOGNAME", "GRID_MACHINE_FOLDER"} {
		t.Setenv(key, "")
	}
	t.Setenv("HOME", home)
	t.Setenv("PATH", t.TempDir())
	if filepath.Separator == '\\' {
		t.Setenv("USERPROFILE", home)
	}
	var exp cpInitExpect
	if fixture == "" {
		return home, exp
	}
	dir := filepath.Join("..", "..", "contract", "gridconfig", fixture)
	if b, err := os.ReadFile(filepath.Join(dir, "expect.json")); err == nil {
		if err := json.Unmarshal(b, &exp); err != nil {
			t.Fatal(err)
		}
		exp.Data = cpInitTilde(home, exp.Data)
	}
	for k, v := range exp.Env {
		t.Setenv(k, v)
	}
	for name, path := range map[string]string{
		"config.json":     gridconfig.Path(os.Getenv, home),
		"old-config.json": gridconfig.OldPath(home),
	} {
		b, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			continue
		}
		q, _ := json.Marshal(home)
		text := strings.ReplaceAll(string(b), "~", strings.Trim(string(q), `"`))
		cpInitWrite(t, path, text)
	}
	return home, exp
}

func cpInitTilde(home, p string) string {
	if p == "~" {
		return home
	}
	if strings.HasPrefix(p, "~/") {
		return filepath.Join(home, filepath.FromSlash(p[2:]))
	}
	return p
}

func cpInitWrite(t *testing.T, path, text string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}
}

// cpInitRun runs grid in home with stdin and answers stdout, stderr and the exit code.
func cpInitRun(t *testing.T, home, stdin string, args ...string) ([]byte, string, int) {
	t.Helper()
	var out, errOut bytes.Buffer
	env := Env{Stdin: strings.NewReader(stdin), Stdout: &out, Stderr: &errOut, Cwd: home, Home: home, Ceiling: home}
	code := Run(args, env)
	return out.Bytes(), errOut.String(), code
}

// cpInitQuestionRow is one question of the --detect answer as the app reads it.
type cpInitQuestionRow struct {
	ID      string          `json:"id"`
	Default json.RawMessage `json:"default"`
	Source  string          `json:"source"`
	Asked   bool            `json:"asked"`
}

type cpInitDetected struct {
	Config    string              `json:"config"`
	Found     bool                `json:"found"`
	Problem   string              `json:"problem"`
	Workspace string              `json:"workspace"`
	Questions []cpInitQuestionRow `json:"questions"`
}

// cpInitDetect runs grid init --detect --json, checks it against cmd-init and
// answers the questions by id.
func cpInitDetect(t *testing.T, home string, args ...string) (cpInitDetected, map[string]cpInitQuestionRow) {
	t.Helper()
	out, errOut, code := cpInitRun(t, home, "", append(args, "init", "--detect", "--json")...)
	if code != 0 {
		t.Fatalf("grid init --detect: exit %d: %s%s", code, out, errOut)
	}
	if err := contract.Validate("cmd-init", out); err != nil {
		t.Fatalf("cmd-init rejects the real output: %v\n%s", err, out)
	}
	var d cpInitDetected
	if err := json.Unmarshal(out, &d); err != nil {
		t.Fatal(err)
	}
	byID := map[string]cpInitQuestionRow{}
	var ids []string
	for _, q := range d.Questions {
		byID[q.ID] = q
		ids = append(ids, q.ID)
	}
	if got, want := strings.Join(ids, " "), "data brain member accounts claudeSettings sandbox testDefault packs"; got != want {
		t.Fatalf("questions %q, want every section 5 question %q", got, want)
	}
	return d, byID
}

func cpInitString(t *testing.T, raw json.RawMessage) string {
	t.Helper()
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		t.Fatalf("default %s is not a string: %v", raw, err)
	}
	return s
}

// Row 1: every case prints every question with its default and source, and the
// name falls back from the config's member to the account's displayName to the login.
func TestCpInitDetect(t *testing.T) {
	for _, fixture := range cpInitFixtures(t) {
		t.Run(fixture, func(t *testing.T) {
			home, exp := cpInitHome(t, fixture)
			d, q := cpInitDetect(t, home)
			if d.Config != gridconfig.Path(os.Getenv, home) {
				t.Errorf("config %q, want gridconfig.Path %q", d.Config, gridconfig.Path(os.Getenv, home))
			}
			if got := cpInitString(t, q["data"].Default); got != exp.Data || q["data"].Source != exp.Source {
				t.Errorf("data %q from %q, want %q from %q (expect.json)", got, q["data"].Source, exp.Data, exp.Source)
			}
			member := cpInitString(t, q["member"].Default)
			switch {
			case exp.Member != "":
				if member != exp.Member || (q["member"].Source != "config" && q["member"].Source != "legacy") {
					t.Errorf("member %q from %q, want the config's %q", member, q["member"].Source, exp.Member)
				}
			case exp.Env["USER"] != "":
				if want := gridconfig.HandleFromLogin(exp.Env["USER"]); member != want || q["member"].Source != "login" {
					t.Errorf("member %q from %q, want the login handle %q", member, q["member"].Source, want)
				}
			default:
				if member != "" || q["member"].Source != "default" {
					t.Errorf("member %q from %q, want none offered", member, q["member"].Source)
				}
			}
			if !q["member"].Asked || q["sandbox"].Asked || q["testDefault"].Asked || q["packs"].Asked || q["brain"].Asked {
				t.Errorf("asked: member %v (want true), sandbox %v, testDefault %v, packs %v, brain %v (want false)",
					q["member"].Asked, q["sandbox"].Asked, q["testDefault"].Asked, q["packs"].Asked, q["brain"].Asked)
			}
			if fixture == "broken" && (d.Problem == "" || !d.Found) {
				t.Errorf("broken: found %v problem %q, want the problem reported", d.Found, d.Problem)
			}
			if fixture == "none" && d.Found {
				t.Errorf("none: found %v, want no config", d.Found)
			}
			if fixture == "brain-machine" {
				if got := cpInitString(t, q["brain"].Default); got != filepath.Join(home, "Brain", "acme") || q["brain"].Source != "config" {
					t.Errorf("brain %q from %q, want the config's", got, q["brain"].Source)
				}
			}
			if fixture == "v3" && string(q["claudeSettings"].Default) != `"yes"` || fixture == "v3" && q["claudeSettings"].Source != "config" {
				t.Errorf("v3 claudeSettings %s from %q, want the config's yes", q["claudeSettings"].Default, q["claudeSettings"].Source)
			}
		})
	}

	t.Run("member falls back to displayName then login", func(t *testing.T) {
		home, _ := cpInitHome(t, "none")
		t.Setenv("USER", "Ann.Login")
		cpInitWrite(t, filepath.Join(home, ".claude.json"), `{"oauthAccount":{"displayName":"Sam Example","emailAddress":"sam@example.com"}}`)
		if err := os.MkdirAll(filepath.Join(home, ".claude"), 0o700); err != nil {
			t.Fatal(err)
		}
		_, q := cpInitDetect(t, home)
		if got := cpInitString(t, q["member"].Default); got != "sam-example" || q["member"].Source != "account" {
			t.Errorf("member %q from %q, want sam-example from the account's displayName", got, q["member"].Source)
		}
		var accts []struct{ Email, Source string }
		if err := json.Unmarshal(q["accounts"].Default, &accts); err != nil || len(accts) != 1 || accts[0].Email != "sam@example.com" || accts[0].Source != "found" || !q["accounts"].Asked {
			t.Errorf("accounts %s asked %v, want sam's folder found and asked", q["accounts"].Default, q["accounts"].Asked)
		}
		if err := os.Remove(filepath.Join(home, ".claude.json")); err != nil {
			t.Fatal(err)
		}
		_, q = cpInitDetect(t, home)
		if got := cpInitString(t, q["member"].Default); got != "ann-login" || q["member"].Source != "login" {
			t.Errorf("member %q from %q, want ann-login from the login", got, q["member"].Source)
		}
		home2, _ := cpInitHome(t, "v2-owner")
		cpInitWrite(t, filepath.Join(home2, ".claude.json"), `{"oauthAccount":{"displayName":"Sam Example"}}`)
		_, q = cpInitDetect(t, home2)
		if got := cpInitString(t, q["member"].Default); got != "sam" || q["member"].Source != "config" {
			t.Errorf("member %q from %q, want the config's sam over the displayName", got, q["member"].Source)
		}
	})
}

// cpInitSnapshot is every file under root with its bytes.
func cpInitSnapshot(t *testing.T, root string) map[string]string {
	t.Helper()
	out := map[string]string{}
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.Type()&fs.ModeSymlink != 0 {
			target, _ := os.Readlink(path)
			out[path] = "link:" + target
			return nil
		}
		if d.IsDir() {
			out[path] = "dir"
			return nil
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		out[path] = string(b)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

type cpInitResult struct {
	Config    string   `json:"config"`
	Written   bool     `json:"written"`
	Workspace string   `json:"workspace"`
	Data      string   `json:"data"`
	Member    string   `json:"member"`
	Accounts  []string `json:"accounts"`
}

func cpInitAnswer(t *testing.T, home, answers string, args ...string) cpInitResult {
	t.Helper()
	out, errOut, code := cpInitRun(t, home, answers, append(args, "init", "--answers", "-", "--json")...)
	if code != 0 {
		t.Fatalf("grid init --answers -: exit %d: %s%s", code, out, errOut)
	}
	if err := contract.Validate("cmd-init", out); err != nil {
		t.Fatalf("cmd-init rejects the real output: %v\n%s", err, out)
	}
	var r cpInitResult
	if err := json.Unmarshal(out, &r); err != nil {
		t.Fatal(err)
	}
	return r
}

// Row 2: the answers are written at gridconfig.Path in version 3 with the unknown keys
// kept, only the kept accounts are registered, and a second run changes no byte.
func TestCpInitAnswers(t *testing.T) {
	t.Run("v3 kept keys and kept accounts", func(t *testing.T) {
		home, _ := cpInitHome(t, "v3")
		cpInitWrite(t, filepath.Join(home, ".claude-bob", ".claude.json"), `{"oauthAccount":{"emailAddress":"bob@example.com"}}`)
		cpInitWrite(t, filepath.Join(home, ".claude-cara", ".claude.json"), `{"oauthAccount":{"emailAddress":"cara@example.com"}}`)
		q, _ := json.Marshal(filepath.Join(home, ".claude-bob"))
		answers := `{"workspace":"beta","member":"sam-b","accounts":[{"kind":"claude","dir":` + string(q) + `}],"claudeSettings":"yes","sandbox":false}`
		r := cpInitAnswer(t, home, answers)
		path := gridconfig.Path(os.Getenv, home)
		data := filepath.Join(home, "elsewhere", "beta-data")
		if !r.Written || r.Config != path || r.Workspace != "beta" || r.Data != data || r.Member != "sam-b" {
			t.Fatalf("result %+v, want beta written at %s with data %s", r, path, data)
		}
		b, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		var top map[string]any
		if err := json.Unmarshal(b, &top); err != nil {
			t.Fatal(err)
		}
		if top["v"] != float64(3) || top["current"] != "beta" || top["laterTop"] == nil {
			t.Errorf("config v %v current %v laterTop %v, want 3, beta and the unknown key kept:\n%s", top["v"], top["current"], top["laterTop"], b)
		}
		if !strings.Contains(string(b), `"laterKey"`) {
			t.Errorf("the entry's unknown key laterKey was dropped:\n%s", b)
		}
		reg, err := os.ReadFile(filepath.Join(data, "accounts", "registry.json"))
		if err != nil {
			t.Fatalf("no registry in the data folder: %v", err)
		}
		if !strings.Contains(string(reg), "bob@example.com") || strings.Contains(string(reg), "cara@example.com") {
			t.Errorf("registry holds %s, want bob (kept) and not cara (dropped)", reg)
		}
		if len(r.Accounts) != 1 || r.Accounts[0] != "claude-bob" {
			t.Errorf("accounts %v, want [claude-bob]", r.Accounts)
		}

		before := cpInitSnapshot(t, home)
		again := cpInitAnswer(t, home, answers)
		after := cpInitSnapshot(t, home)
		if again.Written {
			t.Errorf("second run wrote the config again: %+v", again)
		}
		for p, v := range before {
			if after[p] != v {
				t.Errorf("second run changed %s", p)
			}
		}
		for p := range after {
			if _, ok := before[p]; !ok {
				t.Errorf("second run made %s", p)
			}
		}
	})

	t.Run("no config is an offer", func(t *testing.T) {
		home, _ := cpInitHome(t, "none")
		t.Setenv("USER", "ann")
		r := cpInitAnswer(t, home, `{}`)
		path := gridconfig.Path(os.Getenv, home)
		if !r.Written || r.Member != "ann" || r.Workspace != "ann" || r.Data != filepath.Join(home, ".grid") {
			t.Fatalf("result %+v, want a new config for ann with data ~/.grid", r)
		}
		cfg, found, err := gridconfig.Read(path)
		if !found || err != nil || cfg.V != gridconfig.Version {
			t.Fatalf("config at %s: found %v err %v v %d", path, found, err, cfg.V)
		}
		if h := gridhome.Resolve(os.Getenv, home); h.Dir != filepath.Join(home, ".grid") || h.Source != gridhome.SourceConfig || h.Member != "ann" {
			t.Errorf("Resolve after init = %+v, want ~/.grid from the config with member ann", h)
		}
		before := cpInitSnapshot(t, home)
		if again := cpInitAnswer(t, home, `{}`); again.Written {
			t.Errorf("second run wrote again: %+v", again)
		}
		if after := cpInitSnapshot(t, home); len(after) != len(before) {
			t.Errorf("second run changed the file set: %d files, was %d", len(after), len(before))
		}
	})

	t.Run("bad answers are refused", func(t *testing.T) {
		home, _ := cpInitHome(t, "none")
		for _, answers := range []string{`{"member":"not a name"}`, `{"nope":1}`, `{"data":"relative"}`} {
			if _, _, code := cpInitRun(t, home, answers, "init", "--answers", "-"); code == 0 {
				t.Errorf("answers %s exit 0, want refused", answers)
			}
		}
		if _, err := os.Stat(gridconfig.Path(os.Getenv, home)); err == nil {
			t.Errorf("a refused answer wrote the config")
		}
	})
}

// Row 3: --workspace picks the entry; GRID_HOME and --grid-home win over the config.
func TestCpInitHomeWins(t *testing.T) {
	home, _ := cpInitHome(t, "v3")
	d, q := cpInitDetect(t, home)
	if d.Workspace != "beta" || cpInitString(t, q["data"].Default) != filepath.Join(home, "elsewhere", "beta-data") {
		t.Fatalf("current: workspace %q data %s, want beta's", d.Workspace, q["data"].Default)
	}
	out, errOut, code := cpInitRun(t, home, "", "init", "--workspace", "acme", "--detect", "--json")
	if code != 0 {
		t.Fatalf("--workspace acme: exit %d %s", code, errOut)
	}
	if err := json.Unmarshal(out, &d); err != nil {
		t.Fatal(err)
	}
	q = map[string]cpInitQuestionRow{}
	for _, row := range d.Questions {
		q[row.ID] = row
	}
	if d.Workspace != "acme" || cpInitString(t, q["data"].Default) != filepath.Join(home, ".grid", "acme") || q["data"].Source != "config" || cpInitString(t, q["member"].Default) != "sam" {
		t.Errorf("--workspace acme: %s", out)
	}

	envHome := filepath.Join(home, "from-env")
	t.Setenv("GRID_HOME", envHome)
	_, q = cpInitDetect(t, home)
	if got := cpInitString(t, q["data"].Default); got != envHome || q["data"].Source != "env" || q["data"].Asked {
		t.Errorf("GRID_HOME: data %q from %q asked %v, want %q from env, settled", got, q["data"].Source, q["data"].Asked, envHome)
	}
	t.Setenv("GRID_HOME", "")

	flagHome := filepath.Join(home, "from-flag")
	_, q = cpInitDetect(t, home, "--grid-home", flagHome)
	if got := cpInitString(t, q["data"].Default); got != flagHome || q["data"].Source != "flag" || q["data"].Asked {
		t.Errorf("--grid-home: data %q from %q asked %v, want %q from flag, settled", got, q["data"].Source, q["data"].Asked, flagHome)
	}

	r := cpInitAnswer(t, home, `{"member":"sam"}`, "--grid-home", flagHome)
	if r.Data != flagHome {
		t.Errorf("--grid-home answers: data %q, want %q", r.Data, flagHome)
	}
	if _, _, code := cpInitRun(t, home, `{}`, "init", "--workspace", "acme", "--answers", "-"); code != 0 {
		t.Fatalf("--workspace acme --answers: exit %d", code)
	}
	cfg, _, err := gridconfig.Read(gridconfig.Path(os.Getenv, home))
	if err != nil || cfg.Current != "acme" {
		t.Errorf("after --workspace acme --answers: current %q err %v, want acme", cfg.Current, err)
	}
	if h := gridhome.Resolve(os.Getenv, home); h.Dir != filepath.Join(home, ".grid", "acme") || h.Workspace != "acme" {
		t.Errorf("Resolve after the switch = %+v, want acme's data folder", h)
	}
}
