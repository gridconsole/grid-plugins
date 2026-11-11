package account

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gridconsole/grid-plugins/internal/gridhome"
)

// These tests cover Env.Environ: the provider child takes its environment from
// the Env, not from the host process. The host environment (t.Setenv) always
// points at a folder where the fake provider is signed out, so a child that
// still copied os.Environ would answer signed out and fail the test.

// fakeCodex stands in for the provider's `codex`: `login status` exits 0 when a
// fake-login file is in the folder CODEX_HOME names (FAKE_HOME/.codex when unset).
const kkProbeFakeCodex = `#!/bin/sh
dir="${CODEX_HOME:-$FAKE_HOME/.codex}"
case "$1 $2" in
"login status")
  [ -f "$dir/fake-login" ] && exit 0
  exit 1;;
esac
exit 2
`

// kkProbeHome makes a home folder with a ~/.claude folder; email "" leaves the
// fake provider signed out there.
func kkProbeHome(t *testing.T, email string) string {
	t.Helper()
	home := filepath.Join(gridhome.Real(t.TempDir()), "home")
	if err := os.MkdirAll(filepath.Join(home, ".claude"), 0o755); err != nil {
		t.Fatal(err)
	}
	if email != "" {
		login(t, filepath.Join(home, ".claude"), email)
	}
	return home
}

// kkProbeGetenv reads a name from a KEY=VALUE list, as a caller's own Getenv does.
func kkProbeGetenv(list []string) func(string) string {
	return func(key string) string {
		for _, kv := range list {
			if k, v, ok := strings.Cut(kv, "="); ok && k == key {
				return v
			}
		}
		return ""
	}
}

func kkProbeHomeFolder(t *testing.T, folders []Folder) Folder {
	t.Helper()
	if len(folders) != 1 || folders[0].Kind != Claude || !folders[0].Home {
		t.Fatalf("want the one home Claude folder, got %+v", folders)
	}
	if folders[0].SignedIn == nil {
		t.Fatalf("the provider did not run: %+v", folders[0])
	}
	return folders[0]
}

func TestKkProbeEnvironHome(t *testing.T) {
	env, bin := machine(t) // host FAKE_HOME is env.Home, host PATH is bin
	installFakeClaude(t, bin)
	if err := os.MkdirAll(filepath.Join(env.Home, ".claude"), 0o755); err != nil { // signed out on the host's FAKE_HOME
		t.Fatal(err)
	}
	other := kkProbeHome(t, "a@example.com")
	wrong := filepath.Join(t.TempDir(), "wrong")
	login(t, wrong, "wrong@example.com")
	list := []string{"FAKE_HOME=" + other, "PATH=" + bin, "CLAUDE_CONFIG_DIR=" + wrong}
	env.Environ = func() []string { return list }

	got := kkProbeHomeFolder(t, Scan(env))
	if !*got.SignedIn || got.Email != "a@example.com" {
		t.Fatalf("with Environ set the child reads that list (signed in as a@example.com, CLAUDE_CONFIG_DIR dropped for the home folder): %+v", got)
	}

	env.Environ = nil
	got = kkProbeHomeFolder(t, Scan(env))
	if *got.SignedIn || got.Email != "" {
		t.Fatalf("with Environ nil the child reads the host environment (signed out): %+v", got)
	}
}

func TestKkProbeProviderEnv(t *testing.T) {
	const claudeDir = "CLAUDE_CONFIG_DIR"
	list := []string{"Path=x", "Claude_Config_Dir=/wrong", "A=1"}
	kept := append([]string(nil), list...)
	cases := []struct {
		name         string
		in           []string
		dirVar, dir  string
		goos         string
		want         []string
		wantNonEmpty bool
	}{
		{"windows folds case, home folder", list, claudeDir, "", "windows", []string{"Path=x", "A=1"}, true},
		{"linux is exact, home folder", list, claudeDir, "", "linux", list, true},
		{"darwin is exact, home folder", list, claudeDir, "", "darwin", list, true},
		{"windows, own folder", list, claudeDir, "/d", "windows", []string{"Path=x", "A=1", "CLAUDE_CONFIG_DIR=/d"}, true},
		{"linux, own folder", list, claudeDir, "/d", "linux", []string{"Path=x", "Claude_Config_Dir=/wrong", "A=1", "CLAUDE_CONFIG_DIR=/d"}, true},
		{"linux drops the exact name", []string{"A=1", "CLAUDE_CONFIG_DIR=/old", "B=2"}, claudeDir, "/d", "linux", []string{"A=1", "B=2", "CLAUDE_CONFIG_DIR=/d"}, true},
		{"codex variable", []string{"codex_home=/w", "A=1"}, "CODEX_HOME", "", "windows", []string{"A=1"}, true},
		{"a value with an equals sign and a drive entry stay", []string{`=C:=C:\x`, "A=b=c", "claude_config_dir=/w"}, claudeDir, "", "windows", []string{`=C:=C:\x`, "A=b=c"}, true},
		{"empty list is an empty list, not nil", nil, claudeDir, "", "linux", []string{}, false},
		{"only the folder variable left", []string{"CLAUDE_CONFIG_DIR=/w"}, claudeDir, "", "linux", []string{}, false},
	}
	for _, c := range cases {
		got := providerEnv(c.in, c.dirVar, c.dir, c.goos)
		if got == nil {
			t.Errorf("%s: nil list; exec.Cmd would hand the host environment to the child", c.name)
		}
		if !reflect.DeepEqual(got, c.want) && !(len(got) == 0 && len(c.want) == 0) {
			t.Errorf("%s: got %q, want %q", c.name, got, c.want)
		}
	}
	if !reflect.DeepEqual(list, kept) {
		t.Errorf("providerEnv changed its input: %q", list)
	}
}

func TestKkProbeTwoEnvsDoNotSeeEachOther(t *testing.T) {
	_, bin := machine(t) // host PATH is bin; host FAKE_HOME holds no login
	installFakeClaude(t, bin)
	emails := []string{"a@example.com", "b@example.com"}
	envs := make([]Env, len(emails))
	for i, email := range emails {
		home := kkProbeHome(t, email)
		list := []string{"HOME=" + home, "FAKE_HOME=" + home, "PATH=" + bin}
		envs[i] = Env{
			Home:     home,
			GridHome: filepath.Join(gridhome.Real(t.TempDir()), "grid"),
			Getenv:   kkProbeGetenv(list),
			Environ:  func() []string { return list },
			Now:      func() time.Time { return fixedNow },
		}
	}

	start := make(chan struct{})
	errs := make([]error, len(envs))
	var wg sync.WaitGroup
	for i := range envs {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			_, errs[i] = Status(envs[i], true)
		}(i)
	}
	close(start)
	wg.Wait()

	for i, env := range envs {
		if errs[i] != nil {
			t.Fatalf("Status for %s: %v", emails[i], errs[i])
		}
		recs, err := loadRecords(env)
		if err != nil {
			t.Fatal(err)
		}
		have := map[string]bool{}
		for _, r := range recs {
			have[r.Email] = true
		}
		if !have[emails[i]] || have[emails[1-i]] || len(recs) != 1 {
			t.Errorf("registry of %s holds %+v, want only that email", emails[i], recs)
		}
	}
}

func TestKkProbeEmptyEnvironIsNotTheHost(t *testing.T) {
	env, bin := machine(t)
	installFakeClaude(t, bin)
	login(t, filepath.Join(env.Home, ".claude"), "host@example.com") // the host environment would sign in
	if got := kkProbeHomeFolder(t, Scan(env)); !*got.SignedIn {
		t.Fatalf("with Environ nil the host environment signs in: %+v", got)
	}
	env.Environ = func() []string { return []string{} }
	if got := kkProbeHomeFolder(t, Scan(env)); *got.SignedIn || got.Email != "" {
		t.Fatalf("an empty Environ is an empty environment, not the host's: %+v", got)
	}
}

func TestKkProbeEnvironReadAtCallTime(t *testing.T) {
	env, bin := machine(t)
	installFakeClaude(t, bin)
	a, b := kkProbeHome(t, "a@example.com"), kkProbeHome(t, "b@example.com")
	if err := os.MkdirAll(filepath.Join(env.Home, ".claude"), 0o755); err != nil {
		t.Fatal(err)
	}
	cur := a
	env.Environ = func() []string { return []string{"FAKE_HOME=" + cur, "PATH=" + bin} }
	if got := kkProbeHomeFolder(t, Scan(env)); got.Email != "a@example.com" {
		t.Fatalf("first read: %+v", got)
	}
	cur = b
	if got := kkProbeHomeFolder(t, Scan(env)); got.Email != "b@example.com" {
		t.Fatalf("the list is read again at each call: %+v", got)
	}
}

func TestKkProbeEnvironSignInAndCodex(t *testing.T) {
	env, bin := machine(t)
	installFakeClaude(t, bin)
	if err := os.WriteFile(filepath.Join(bin, "codex"), []byte(kkProbeFakeCodex), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, d := range []string{".claude", ".codex"} {
		if err := os.MkdirAll(filepath.Join(env.Home, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	other := kkProbeHome(t, "")
	write(t, filepath.Join(other, ".codex", "fake-login"), "")
	env.Environ = func() []string { return []string{"FAKE_HOME=" + other, "PATH=" + bin} }

	var out bytes.Buffer
	homeDir := filepath.Join(env.Home, ".claude")
	if code, err := SignIn(env, Claude, homeDir, strings.NewReader(""), &out, io.Discard); err != nil || code != 0 || !strings.Contains(out.String(), "login on "+filepath.Join(other, ".claude")) {
		t.Fatalf("SignIn runs the child on Environ's FAKE_HOME: %d %v %q", code, err, out.String())
	}

	signed := func() *bool {
		for _, f := range Scan(env) {
			if f.Kind == Codex {
				return f.SignedIn
			}
		}
		t.Fatal("no codex folder")
		return nil
	}
	if s := signed(); s == nil || !*s {
		t.Fatalf("codex login status runs on Environ's FAKE_HOME (signed in): %v", s)
	}
	env.Environ = nil
	if s := signed(); s == nil || *s {
		t.Fatalf("with Environ nil codex reads the host's FAKE_HOME (signed out): %v", s)
	}
}
