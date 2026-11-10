package account

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/gridconsole/grid-plugins/internal/gridhome"
)

// fakeScript stands in for the provider's `claude`: `auth status` answers from
// a fake-email file in the folder CLAUDE_CONFIG_DIR names (the home folder when
// it is unset), and `auth login` says which folder it ran on.
const fakeScript = `#!/bin/sh
dir="${CLAUDE_CONFIG_DIR:-$FAKE_HOME/.claude}"
case "$1 $2" in
"auth status")
  if [ -f "$dir/fake-email" ]; then
    read -r email < "$dir/fake-email"
    printf '{"loggedIn":true,"email":"%s"}\n' "$email"
    exit 0
  fi
  echo '{"loggedIn":false}'
  exit 1;;
"auth login")
  echo "login on $dir"
  exit 0;;
esac
exit 2
`

var fixedNow = time.Date(2026, 11, 11, 12, 0, 0, 0, time.UTC)

// machine makes a temporary HOME with no provider on PATH.
func machine(t *testing.T) (Env, string) {
	t.Helper()
	root := gridhome.Real(t.TempDir())
	home := filepath.Join(root, "home")
	bin := filepath.Join(root, "bin")
	for _, d := range []string{home, filepath.Join(home, ".grid"), bin} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("CLAUDE_CONFIG_DIR", "")
	t.Setenv("CODEX_HOME", "")
	t.Setenv("FAKE_HOME", home)
	t.Setenv("PATH", bin)
	return Env{Home: home, GridHome: filepath.Join(home, ".grid"), Now: func() time.Time { return fixedNow }}, bin
}

func installFakeClaude(t *testing.T, bin string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("the fake provider is a shell script")
	}
	if err := os.WriteFile(filepath.Join(bin, "claude"), []byte(fakeScript), 0o755); err != nil {
		t.Fatal(err)
	}
}

func write(t *testing.T, path, text string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}

// login signs the fake provider in on dir with email.
func login(t *testing.T, dir, email string) {
	t.Helper()
	write(t, filepath.Join(dir, "fake-email"), email)
}

func status(t *testing.T, env Env, register bool) []Account {
	t.Helper()
	a, err := Status(env, register)
	if err != nil {
		t.Fatal(err)
	}
	return a
}

func byID(t *testing.T, list []Account, id string) Account {
	t.Helper()
	for _, a := range list {
		if a.ID == id {
			return a
		}
	}
	t.Fatalf("no account %q in %+v", id, list)
	return Account{}
}

func idToken(email string) string {
	enc := func(s string) string { return base64.RawURLEncoding.EncodeToString([]byte(s)) }
	return enc(`{"alg":"none"}`) + "." + enc(`{"email":"`+email+`","https://api.openai.com/auth":{"chatgpt_plan_type":"plus"}}`) + ".SECRET-SIGNATURE-9"
}

func TestScanReadsEmailByAuthStatusThenByDetailsFile(t *testing.T) {
	env, bin := machine(t)
	home := filepath.Join(env.Home, ".claude")
	write(t, filepath.Join(env.Home, ".claude-work", "keep"), "")
	write(t, filepath.Join(home, "keep"), "")
	write(t, filepath.Join(env.Home, ".claude.json"), `{"oauthAccount":{"emailAddress":"Details@Example.com"}}`)
	write(t, filepath.Join(env.Home, ".claude-notes"), "a file, not a folder")
	write(t, filepath.Join(env.Home, ".codex", "auth.json"), `{"tokens":{"id_token":"`+idToken("Cody@Example.com")+`"}}`)

	// No provider on PATH: the details file answers, and sign-in is unknown.
	got := Scan(env)
	if len(got) != 3 {
		t.Fatalf("want ~/.claude, ~/.claude-work and ~/.codex, got %+v", got)
	}
	if f := got[0]; f.Kind != Claude || !f.Home || f.Email != "details@example.com" || f.SignedIn != nil {
		t.Fatalf("home folder without a provider: %+v", f)
	}
	if f := got[1]; f.Home || f.Email != "" {
		t.Fatalf("~/.claude-work has no details file of its own: %+v", f)
	}
	if f := got[2]; f.Kind != Codex || !f.Home || f.Email != "cody@example.com" || f.Plan != "plus" || f.SignedIn == nil || !*f.SignedIn {
		t.Fatalf("codex folder: %+v", f)
	}

	// With the provider: `auth status` answers first, and signed out means no email even with a details file.
	installFakeClaude(t, bin)
	login(t, home, "Status@Example.com")
	got = Scan(env)
	if f := got[0]; f.Email != "status@example.com" || f.SignedIn == nil || !*f.SignedIn {
		t.Fatalf("auth status email: %+v", f)
	}
	if f := got[1]; f.Email != "" || f.SignedIn == nil || *f.SignedIn {
		t.Fatalf("signed out folder: %+v", f)
	}
	if err := os.Remove(filepath.Join(home, "fake-email")); err != nil {
		t.Fatal(err)
	}
	if f := Scan(env)[0]; f.Email != "" || f.SignedIn == nil || *f.SignedIn {
		t.Fatalf("signed out must not fall back to a stale details file: %+v", f)
	}
}

func TestScanAddsTheFolderTheVariableNames(t *testing.T) {
	env, _ := machine(t)
	custom := filepath.Join(filepath.Dir(env.Home), "elsewhere")
	write(t, filepath.Join(custom, "keep"), "")
	t.Setenv("CLAUDE_CONFIG_DIR", custom)
	got := Scan(env)
	if len(got) != 1 || got[0].Dir != custom || got[0].Home {
		t.Fatalf("want only the variable's folder: %+v", got)
	}
}

func TestStatusRegistersAnEmailOnceWithItsOwnFolder(t *testing.T) {
	env, bin := machine(t)
	installFakeClaude(t, bin)
	home := filepath.Join(env.Home, ".claude")
	login(t, home, "ann@example.com")

	first := status(t, env, true)
	if len(first) != 1 {
		t.Fatalf("want one record: %+v", first)
	}
	a := first[0]
	own := filepath.Join(env.Home, ".claude-ann")
	if a.ID != "claude-ann" || a.Email != "ann@example.com" || a.Name != "ann@example.com" || a.Dir != own || a.Added != "auto" || !a.Default {
		t.Fatalf("record: %+v", a)
	}
	if a.SignedIn == nil || *a.SignedIn {
		t.Fatalf("its own folder is not signed in yet: %+v", a)
	}
	if st, err := os.Stat(own); err != nil || !st.IsDir() {
		t.Fatalf("own folder missing: %v", err)
	}
	before, err := os.ReadFile(registryPath(env))
	if err != nil {
		t.Fatal(err)
	}

	// The same email again, in another letter case, and once the person signed in on the new folder.
	login(t, home, "ANN@example.com")
	login(t, own, "ann@example.com")
	again := status(t, env, true)
	after, _ := os.ReadFile(registryPath(env))
	if len(again) != 1 || !bytes.Equal(before, after) {
		t.Fatalf("an email already seen must not register again:\n%s\n%s", before, after)
	}
	if again[0].SignedIn == nil || !*again[0].SignedIn {
		t.Fatalf("signed in on its own folder now: %+v", again[0])
	}
}

func TestTwoEmailsWithOneLocalPartGetTwoIDs(t *testing.T) {
	env, bin := machine(t)
	installFakeClaude(t, bin)
	home := filepath.Join(env.Home, ".claude")
	login(t, home, "ann@example.com")
	status(t, env, true)
	login(t, home, "ann@example.org")
	got := status(t, env, true)
	if len(got) != 2 || got[0].ID != "claude-ann" || got[1].ID != "claude-ann-2" {
		t.Fatalf("ids: %+v", got)
	}
}

func TestDefaultTagFollowsTheHomeFolderAndIsNotStored(t *testing.T) {
	env, bin := machine(t)
	installFakeClaude(t, bin)
	home := filepath.Join(env.Home, ".claude")
	login(t, home, "ann@example.com")
	status(t, env, true)

	login(t, home, "bob@example.com") // /login with another email
	got := status(t, env, true)
	if len(got) != 2 || byID(t, got, "claude-ann").Default || !byID(t, got, "claude-bob").Default {
		t.Fatalf("the tag moves to the new email and the old record stays: %+v", got)
	}
	login(t, home, "ann@example.com") // back again: nothing is registered twice
	got = status(t, env, true)
	if len(got) != 2 || !byID(t, got, "claude-ann").Default || byID(t, got, "claude-bob").Default {
		t.Fatalf("the tag returns to the first record: %+v", got)
	}
	if err := os.Remove(filepath.Join(home, "fake-email")); err != nil { // signed out
		t.Fatal(err)
	}
	for _, a := range status(t, env, true) {
		if a.Default {
			t.Fatalf("signed out: no record carries the tag: %+v", a)
		}
	}
	if data, _ := os.ReadFile(registryPath(env)); strings.Contains(string(data), "default") {
		t.Fatalf("the default tag must not be stored: %s", data)
	}
}

func TestListWritesNothing(t *testing.T) {
	env, bin := machine(t)
	installFakeClaude(t, bin)
	login(t, filepath.Join(env.Home, ".claude"), "ann@example.com")
	if got := status(t, env, false); len(got) != 0 {
		t.Fatalf("list must not register: %+v", got)
	}
	if _, err := os.Stat(filepath.Join(env.Home, ".claude-ann")); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("list made a folder: %v", err)
	}
	if _, err := os.Stat(registryPath(env)); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("list wrote the registry: %v", err)
	}
}

func TestMakeFolderIsIdempotentAndLinksConfigOnly(t *testing.T) {
	env, _ := machine(t)
	home := filepath.Join(env.Home, ".claude")
	for _, name := range []string{"settings.json", "CLAUDE.md", ".credentials.json", ".claude.json", "auth.json", "history.jsonl"} {
		write(t, filepath.Join(home, name), "x")
	}
	write(t, filepath.Join(home, "skills", "s.md"), "x")
	first, err := MakeFolder(env, Claude, "claude-ann")
	if err != nil || !first.Created {
		t.Fatalf("first: %+v %v", first, err)
	}
	second, err := MakeFolder(env, Claude, "claude-ann")
	if err != nil || second.Created || second.Dir != first.Dir {
		t.Fatalf("second run must say created:false: %+v %v", second, err)
	}
	for _, name := range []string{".credentials.json", ".claude.json", "auth.json", "history.jsonl"} {
		if _, err := os.Lstat(filepath.Join(first.Dir, name)); !errors.Is(err, fs.ErrNotExist) {
			t.Fatalf("%s must not appear in the new folder: %v", name, err)
		}
	}
	if runtime.GOOS == "windows" {
		return
	}
	if st, _ := os.Stat(first.Dir); st.Mode().Perm() != 0o700 {
		t.Fatalf("mode %v", st.Mode().Perm())
	}
	for _, name := range []string{"settings.json", "CLAUDE.md", "skills"} {
		if target, err := os.Readlink(filepath.Join(first.Dir, name)); err != nil || target != filepath.Join(home, name) {
			t.Fatalf("%s should link to the person's own: %q %v", name, target, err)
		}
	}
	if _, err := os.Lstat(filepath.Join(first.Dir, "agents")); !errors.Is(err, fs.ErrNotExist) {
		t.Fatal("a source that is missing is skipped, not linked")
	}
}

func TestMakeFolderOnlyMakesProviderFolders(t *testing.T) {
	env, _ := machine(t)
	for _, bad := range []struct{ kind, id string }{{Claude, "ssh"}, {Claude, "claude"}, {Claude, "claude-"}, {Claude, "claude-../x"}, {Claude, "codex-ann"}, {"other", "other-x"}} {
		if _, err := MakeFolder(env, bad.kind, bad.id); err == nil {
			t.Fatalf("%v should be refused", bad)
		}
	}
	if entries, _ := os.ReadDir(env.Home); len(entries) != 1 || entries[0].Name() != ".grid" {
		t.Fatalf("a refused id made something: %v", entries)
	}
}

func TestNoSecretReachesAGridFileOrTheNewFolder(t *testing.T) {
	env, bin := machine(t)
	installFakeClaude(t, bin)
	home := filepath.Join(env.Home, ".claude")
	secrets := []string{"SECRET-ACCESS-TOKEN-1", "SECRET-DETAILS-2", "SECRET-SIGNATURE-9"}
	write(t, filepath.Join(home, ".credentials.json"), `{"claudeAiOauth":{"accessToken":"SECRET-ACCESS-TOKEN-1"}}`)
	write(t, filepath.Join(env.Home, ".claude.json"), `{"userID":"SECRET-DETAILS-2","oauthAccount":{"emailAddress":"ann@example.com"}}`)
	write(t, filepath.Join(env.Home, ".codex", "auth.json"), `{"tokens":{"id_token":"`+idToken("cody@example.com")+`","access_token":"SECRET-ACCESS-TOKEN-1"}}`)
	login(t, home, "ann@example.com")

	status(t, env, true)
	if _, err := MakeFolder(env, Codex, "codex-cody"); err != nil {
		t.Fatal(err)
	}
	if err := Note(env, strings.NewReader(`{"session_id":"SECRET-DETAILS-2","rate_limits":{"five_hour":{"used_percentage":5,"resets_at":1}}}`), home); err != nil {
		t.Fatal(err)
	}
	if _, err := Windows(env, ""); err != nil {
		t.Fatal(err)
	}
	var found []string
	for _, root := range []string{env.GridHome, filepath.Join(env.Home, ".claude-ann"), filepath.Join(env.Home, ".codex-cody")} {
		_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return nil
			}
			data, err := os.ReadFile(path)
			for _, s := range secrets {
				if err == nil && strings.Contains(string(data), s) {
					found = append(found, path+" holds "+s)
				}
			}
			return nil
		})
	}
	if len(found) != 0 {
		t.Fatalf("secrets copied: %v", found)
	}
}

func TestNoteThenWindowsRoundTripsTheStatusLine(t *testing.T) {
	env, _ := machine(t)
	home := filepath.Join(env.Home, ".claude")
	write(t, filepath.Join(home, "keep"), "")
	future, past := fixedNow.Add(time.Hour).Unix(), fixedNow.Add(-time.Hour).Unix()
	line := func(five, week int64) string {
		b, _ := json.Marshal(map[string]any{"session_id": "s", "rate_limits": map[string]any{
			"five_hour": map[string]any{"used_percentage": 23.5, "resets_at": five},
			"seven_day": map[string]any{"used_percentage": 61, "resets_at": week}}})
		return string(b)
	}
	if err := Note(env, strings.NewReader(line(future, future+1000)), home); err != nil {
		t.Fatal(err)
	}
	got, err := Windows(env, "claude")
	if err != nil || len(got) != 1 {
		t.Fatalf("%+v %v", got, err)
	}
	w := got[0]
	if w.Source != "statusline" || w.AsOf == "" || len(w.Windows) != 2 ||
		w.Windows[0] != (Window{"5h", 23.5, future}) || w.Windows[1] != (Window{"7d", 61, future + 1000}) {
		t.Fatalf("round trip: %+v", w)
	}
	// A window whose reset has passed started over: used 0, with its reset time.
	if err := Note(env, strings.NewReader(line(past, future)), home); err != nil {
		t.Fatal(err)
	}
	if got, _ = Windows(env, "claude"); got[0].Windows[0] != (Window{"5h", 0, past}) || got[0].Windows[1].Used != 61 {
		t.Fatalf("after reset: %+v", got[0].Windows)
	}
	// A payload with no window (API-key sign-in, or before the first reply) keeps the old numbers.
	if err := Note(env, strings.NewReader(`{"session_id":"s"}`), home); err != nil {
		t.Fatal(err)
	}
	if got, _ = Windows(env, "claude"); got[0].Source != "statusline" || len(got[0].Windows) != 2 {
		t.Fatalf("silence overwrote the snapshot: %+v", got[0])
	}
}

func TestWindowsWithNoSourceSaysNoDataNotZero(t *testing.T) {
	env, _ := machine(t)
	write(t, filepath.Join(env.Home, ".claude", "keep"), "")
	write(t, filepath.Join(env.Home, ".codex", "keep"), "")
	got, err := Windows(env, "")
	if err != nil || len(got) != 2 {
		t.Fatalf("%+v %v", got, err)
	}
	data, _ := json.Marshal(got)
	for _, w := range got {
		if w.Source != "none" || w.Windows == nil || len(w.Windows) != 0 || w.Note == "" {
			t.Fatalf("no data: %+v", w)
		}
	}
	if strings.Contains(string(data), `"used"`) || !strings.Contains(string(data), `"windows":[]`) {
		t.Fatalf("a missing source must carry no used number: %s", data)
	}
	if _, err := Windows(env, "claude-nobody"); err == nil {
		t.Fatal("an unknown id is an error")
	}
}

func TestDefaultAccountReadsTheHomeFolderNumbers(t *testing.T) {
	env, bin := machine(t)
	installFakeClaude(t, bin)
	home := filepath.Join(env.Home, ".claude")
	login(t, home, "ann@example.com")
	status(t, env, true)
	if err := Note(env, strings.NewReader(`{"rate_limits":{"seven_day":{"used_percentage":40,"resets_at":`+itoa(fixedNow.Add(time.Hour).Unix())+`}}}`), home); err != nil {
		t.Fatal(err)
	}
	got, err := Windows(env, "claude-ann")
	if err != nil || len(got) != 1 || got[0].Source != "statusline" || len(got[0].Windows) != 1 || got[0].Windows[0].Label != "7d" {
		t.Fatalf("%+v %v", got, err)
	}
}

func itoa(n int64) string { b, _ := json.Marshal(n); return string(b) }

func TestSignInRunsTheProviderOnTheFoldersEnvironment(t *testing.T) {
	env, bin := machine(t)
	installFakeClaude(t, bin)
	own := filepath.Join(env.Home, ".claude-ann")
	write(t, filepath.Join(own, "keep"), "")
	write(t, filepath.Join(env.Home, ".claude", "keep"), "")
	var out bytes.Buffer
	if code, err := SignIn(env, Claude, own, strings.NewReader(""), &out, io.Discard); err != nil || code != 0 || !strings.Contains(out.String(), "login on "+own) {
		t.Fatalf("own folder: %d %v %q", code, err, out.String())
	}
	out.Reset()
	homeDir := filepath.Join(env.Home, ".claude")
	if _, err := SignIn(env, Claude, homeDir, strings.NewReader(""), &out, io.Discard); err != nil || !strings.Contains(out.String(), "login on "+homeDir) {
		t.Fatalf("home folder runs with the variable unset: %v %q", err, out.String())
	}
	if _, err := SignIn(env, Claude, filepath.Join(env.Home, ".ssh"), strings.NewReader(""), &out, io.Discard); !errors.Is(err, ErrBadFolder) {
		t.Fatalf("a folder that is not a provider folder: %v", err)
	}
	t.Setenv("PATH", t.TempDir())
	if _, err := SignIn(env, Claude, own, strings.NewReader(""), &out, io.Discard); !errors.Is(err, ErrNoAgent) {
		t.Fatalf("no provider installed: %v", err)
	}
}
