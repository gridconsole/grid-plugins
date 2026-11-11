package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/settings"
)

func TestAccountSignedInWord(t *testing.T) {
	yes, no := true, false
	for _, tc := range []struct {
		in   *bool
		want string
	}{{nil, "unknown"}, {&yes, "signed in"}, {&no, "not signed in"}} {
		if got := signedInWord(tc.in); got != tc.want {
			t.Errorf("signedInWord = %q, want %q", got, tc.want)
		}
	}
}

func TestAccountListLines(t *testing.T) {
	h := accountHarness(t, "")
	if code := h.run(h.home, "account", "list"); code != 0 || h.out.String() != "account: none registered\n" {
		t.Fatalf("empty list: exit %d, out %q, err %q", code, h.out.String(), h.errb.String())
	}
	h.loginAs("ann@example.com")
	h.accounts("status")
	h.loginAs("bob@example.com")
	h.accounts("status") // bob is the newest and takes the default tag
	rows := h.accounts("list")
	if len(rows) != 2 || rows[0].ID != "claude-ann" || rows[1].ID != "claude-bob" || rows[0].Default || !rows[1].Default {
		t.Fatalf("rows: %+v", rows)
	}
	if code := h.run(h.home, "account", "list"); code != 0 {
		t.Fatal(h.errb.String())
	}
	var want string
	for _, r := range rows {
		tag := ""
		if r.Default {
			tag = " (default)"
		}
		want += "account " + r.ID + tag + ": " + r.Email + ", " + signedInWord(r.SignedIn) + ", " + r.Dir + "\n"
	}
	if got := h.out.String(); got != want {
		t.Fatalf("list lines:\n got %q\nwant %q", got, want)
	}
	if !strings.Contains(want, "account claude-bob (default): bob@example.com, ") || strings.Count(want, "(default)") != 1 {
		t.Fatalf("the format drifted: %q", want)
	}
	// status answers the same lines and takes the sign-in check.
	if code := h.run(h.home, "account", "status"); code != 0 || strings.Count(h.out.String(), "\n") != 2 {
		t.Fatalf("status: exit %d, out %q", code, h.out.String())
	}
}

func TestAccountScanLinesAndUsage(t *testing.T) {
	h := accountHarness(t, "ann@example.com")
	if code := h.run(h.home, "account", "folder", "--kind", "claude", "--id", "claude-work"); code != 0 {
		t.Fatal(h.errb.String())
	}
	if code := h.run(h.home, "account", "scan"); code != 0 {
		t.Fatal(h.errb.String())
	}
	lines := strings.Split(strings.TrimSuffix(h.out.String(), "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("want two lines, got %q", h.out.String())
	}
	home, work := filepath.Join(h.home, ".claude"), filepath.Join(h.home, ".claude-work")
	if !strings.HasPrefix(lines[0], "claude "+home+": ann@example.com, ") {
		t.Errorf("first line: %q", lines[0])
	}
	if !strings.HasPrefix(lines[1], "claude "+work+": no email, ") {
		t.Errorf("second line: %q", lines[1])
	}
	if code := h.run(h.home, "account", "scan", "--json"); code != 0 {
		t.Fatal(h.errb.String())
	}
	var doc struct {
		Folders []struct {
			Kind  string `json:"kind"`
			Dir   string `json:"dir"`
			Email string `json:"email"`
		} `json:"folders"`
	}
	if err := json.Unmarshal(h.out.Bytes(), &doc); err != nil || len(doc.Folders) != 2 {
		t.Fatalf("%v: %s", err, h.out.String())
	}
	if doc.Folders[0].Dir != home || doc.Folders[0].Email != "ann@example.com" || doc.Folders[1].Dir != work || doc.Folders[1].Email != "" {
		t.Errorf("folders: %+v", doc.Folders)
	}
	if code := h.run(h.home, "account", "scan", "--bogus"); code != 2 || h.errb.Len() == 0 || h.out.Len() != 0 {
		t.Errorf("unknown flag: exit %d, stdout %q, stderr %q", code, h.out.String(), h.errb.String())
	}
	if !strings.Contains(h.errb.String(), "bogus") || !strings.Contains(h.errb.String(), "account scan") {
		t.Errorf("usage on stderr: %q", h.errb.String())
	}
}

// fakeClaudeFolder writes a `claude` stand-in made only of shell builtins into a
// new folder, puts that folder alone on PATH and returns the file that logs each
// run's argv (a status probe also runs it, so fakeRuns counts the logins only).
func fakeClaudeFolder(t *testing.T, exit int) string {
	t.Helper()
	bin := t.TempDir()
	ran := filepath.Join(bin, "ran")
	script := "#!/bin/sh\n" +
		"echo \"$*\" >> '" + ran + "'\n" +
		"echo \"argv=$*\"\n" +
		"echo \"CLAUDE_CONFIG_DIR=[$CLAUDE_CONFIG_DIR]\"\n" +
		"exit " + string(rune('0'+exit)) + "\n"
	if err := os.WriteFile(filepath.Join(bin, "claude"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)
	return ran
}

func fakeRuns(t *testing.T, ran string) int {
	t.Helper()
	b, err := os.ReadFile(ran)
	if err != nil {
		return 0
	}
	return strings.Count(string(b), "auth login\n")
}

func signInHarness(t *testing.T, exit int) (h *harness, work, ran string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("the fake claude is a sh script, which Windows does not run")
	}
	h = accountHarness(t, "")
	t.Setenv("CLAUDE_CONFIG_DIR", "")
	work = filepath.Join(h.home, ".claude-work")
	if code := h.run(h.home, "account", "folder", "--kind", "claude", "--id", "claude-work"); code != 0 {
		t.Fatal(h.errb.String())
	}
	ran = fakeClaudeFolder(t, exit)
	return h, work, ran
}

func TestAccountSignInRunsTheLoginOnTheFolder(t *testing.T) {
	h, work, ran := signInHarness(t, 0)
	if code := h.run(h.home, "account", "sign-in", "--dir", work, "--kind", "claude"); code != 0 {
		t.Fatalf("exit %d: %s", code, h.errb.String())
	}
	if want := "argv=auth login\nCLAUDE_CONFIG_DIR=[" + work + "]\n"; h.out.String() != want {
		t.Errorf("stdout %q, want %q", h.out.String(), want)
	}
	if n := fakeRuns(t, ran); n != 1 {
		t.Errorf("the fake ran %d times, want 1", n)
	}
	// With --json the provider's output goes to stderr and stdout is the answer.
	if code := h.run(h.home, "account", "sign-in", "--dir", work, "--kind", "claude", "--json"); code != 0 {
		t.Fatalf("exit %d: %s", code, h.errb.String())
	}
	if !strings.Contains(h.errb.String(), "CLAUDE_CONFIG_DIR=["+work+"]") {
		t.Errorf("provider output not on stderr: %q", h.errb.String())
	}
	var res struct {
		Dir  string `json:"dir"`
		Kind string `json:"kind"`
		Exit *int   `json:"exit"`
	}
	if err := json.Unmarshal(h.out.Bytes(), &res); err != nil || res.Dir != work || res.Kind != "claude" || res.Exit == nil || *res.Exit != 0 {
		t.Errorf("stdout %q: %v %+v", h.out.String(), err, res)
	}
	if strings.Contains(h.out.String(), "argv=") {
		t.Errorf("provider output leaked to stdout: %q", h.out.String())
	}
}

func TestAccountSignInHomeFolderGetsNoFolderVariable(t *testing.T) {
	h, _, ran := signInHarness(t, 0)
	// A variable from the builder's shell is cleared by the child's environment.
	t.Setenv("CLAUDE_CONFIG_DIR", "/somewhere/else")
	if code := h.run(h.home, "account", "sign-in", "--dir", filepath.Join(h.home, ".claude"), "--kind", "claude"); code != 0 {
		t.Fatalf("exit %d: %s", code, h.errb.String())
	}
	if want := "argv=auth login\nCLAUDE_CONFIG_DIR=[]\n"; h.out.String() != want {
		t.Errorf("stdout %q, want %q", h.out.String(), want)
	}
	if fakeRuns(t, ran) != 1 {
		t.Error("the fake did not run once")
	}
}

func TestAccountSignInRefusals(t *testing.T) {
	h, work, ran := signInHarness(t, 0)
	unknown := t.TempDir()
	if code := h.run(h.home, "account", "sign-in", "--dir", unknown, "--kind", "claude"); code != 1 || !strings.HasPrefix(h.errb.String(), "grid account sign-in: ") {
		t.Errorf("unknown folder: exit %d, stderr %q", code, h.errb.String())
	}
	if code := h.run(h.home, "account", "sign-in", "--dir", work, "--kind", "bogus"); code != 1 || !strings.Contains(h.errb.String(), "is not claude or codex") {
		t.Errorf("bogus kind: exit %d, stderr %q", code, h.errb.String())
	}
	if code := h.run(h.home, "account", "sign-in", "--id", "claude-nobody"); code != 1 || !strings.HasPrefix(h.errb.String(), "grid account sign-in: no account") {
		t.Errorf("unknown id: exit %d, stderr %q", code, h.errb.String())
	}
	if code := h.run(h.home, "account", "sign-in"); code != 2 || !strings.Contains(h.errb.String(), "give --id, or --dir and --kind") {
		t.Errorf("no target: exit %d, stderr %q", code, h.errb.String())
	}
	if code := h.run(h.home, "account", "sign-in", "--dir", work); code != 2 {
		t.Errorf("--dir without --kind: exit %d", code)
	}
	if n := fakeRuns(t, ran); n != 0 {
		t.Errorf("the fake ran %d times on a refused sign-in", n)
	}
}

func TestAccountSignInByID(t *testing.T) {
	h, work, ran := signInHarness(t, 0)
	h.loginAs("work@example.com")
	// A folder with its own login email is registered by status, with a folder of its own.
	h.write(filepath.Join(work, ".claude.json"), `{"oauthAccount":{"emailAddress":"work@example.com"}}`)
	h.accounts("status")
	var id string
	for _, a := range h.accounts("list") {
		if a.Dir == work {
			id = a.ID
		}
	}
	if id == "" {
		t.Skipf("status did not register the folder %s; no registered account to resolve", work)
	}
	if code := h.run(h.home, "account", "sign-in", "--id", id); code != 0 {
		t.Fatalf("exit %d: %s", code, h.errb.String())
	}
	if want := "argv=auth login\nCLAUDE_CONFIG_DIR=[" + work + "]\n"; h.out.String() != want {
		t.Errorf("stdout %q, want %q", h.out.String(), want)
	}
	if fakeRuns(t, ran) != 1 {
		t.Error("the fake did not run once")
	}
}

func TestAccountSignInFailureAndMissingProvider(t *testing.T) {
	h, work, _ := signInHarness(t, 3)
	if code := h.run(h.home, "account", "sign-in", "--dir", work, "--kind", "claude"); code != 1 {
		t.Errorf("fake exiting 3: exit %d", code)
	}
	if code := h.run(h.home, "account", "sign-in", "--dir", work, "--kind", "claude", "--json"); code != 1 {
		t.Errorf("fake exiting 3 with --json: exit %d", code)
	}
	var res struct {
		Exit *int `json:"exit"`
	}
	if err := json.Unmarshal(h.out.Bytes(), &res); err != nil || res.Exit == nil || *res.Exit != 3 {
		t.Errorf("stdout %q: %v", h.out.String(), err)
	}
	t.Setenv("PATH", t.TempDir())
	if code := h.run(h.home, "account", "sign-in", "--dir", work, "--kind", "claude"); code != 1 || !strings.Contains(h.errb.String(), "is not installed or cannot start") {
		t.Errorf("no claude on PATH: exit %d, stderr %q", code, h.errb.String())
	}
}

func TestAccountEmptyReader(t *testing.T) {
	n, err := emptyReader{}.Read(make([]byte, 4))
	if n != 0 || err == nil || err.Error() != "EOF" {
		t.Fatalf("Read = %d, %v; want 0, EOF", n, err)
	}
	// A run with no stdin at all reads the empty reader instead of the terminal.
	h := accountHarness(t, "")
	h.env.Stdin = nil
	if code := h.run(h.home, "account", "note"); code != 1 || !strings.HasPrefix(h.errb.String(), "grid account note: ") {
		t.Errorf("note with no stdin: exit %d: %s", code, h.errb.String())
	}
}

// agentTemplates writes two agent templates and returns their folder.
func agentTemplates(t *testing.T, dir string) string {
	t.Helper()
	for _, name := range []string{"grid-alpha", "grid-beta"} {
		text := "---\nname: " + name + "\ndescription: test agent " + name + "\nmodel: sonnet\n---\nBody of " + name + ".\n"
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, name+".md"), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

// agentSyncHarness is a harness whose environment names only the plugin root and
// the Claude config folder, so the builder's own variables never reach the run.
func agentSyncHarness(t *testing.T, root, config string) *harness {
	t.Helper()
	h := newHarness(t)
	t.Setenv("CLAUDE_CONFIG_DIR", config)
	t.Setenv("CLAUDE_PLUGIN_ROOT", root)
	h.env.Getenv = func(key string) string {
		return map[string]string{"CLAUDE_PLUGIN_ROOT": root, "CLAUDE_CONFIG_DIR": config}[key]
	}
	return h
}

func decodeSync(t *testing.T, h *harness) []settings.SyncResult {
	t.Helper()
	var doc struct {
		Agents []settings.SyncResult `json:"agents"`
	}
	if err := json.Unmarshal(h.out.Bytes(), &doc); err != nil {
		t.Fatalf("%v: %s", err, h.out.String())
	}
	return doc.Agents
}

func TestAgentSyncRefusals(t *testing.T) {
	config := t.TempDir()
	h := agentSyncHarness(t, "", config)
	h.env.GridHome = ""
	if code := h.run(h.home, "agent", "sync", "--templates", agentTemplates(t, t.TempDir()), "--dir", filepath.Join(config, "agents")); code == 0 || !strings.Contains(h.errb.String(), "set GRID_HOME") {
		t.Errorf("no Grid home: exit %d, stderr %q", code, h.errb.String())
	}
	g, cwd := guestLeakFixture(t, "guest")
	agents := filepath.Join(t.TempDir(), "agents")
	if code := g.run(cwd, "agent", "sync", "--templates", agentTemplates(t, t.TempDir()), "--dir", agents); code == 0 || !strings.Contains(g.errb.String(), guestSettingsRefusal) {
		t.Errorf("guest: exit %d, stderr %q", code, g.errb.String())
	}
	if _, err := os.Stat(agents); err == nil {
		t.Error("a guest sync wrote the agents folder")
	}
	if code := h.run(h.home, "agent", "sync", "--bogus"); code != 2 {
		t.Errorf("unknown flag: exit %d", code)
	}
}

func TestAgentSyncWritesThenReportsUnchanged(t *testing.T) {
	config := t.TempDir()
	h := agentSyncHarness(t, "", config)
	templates := agentTemplates(t, t.TempDir())
	agents := filepath.Join(t.TempDir(), "agents")
	args := []string{"agent", "sync", "--templates", templates, "--dir", agents}
	if code := h.run(h.home, args...); code != 0 {
		t.Fatalf("exit %d: %s", code, h.errb.String())
	}
	for _, name := range []string{"grid-alpha", "grid-beta"} {
		want := name + " written " + filepath.Join(agents, name+".md") + "\n"
		if !strings.Contains(h.out.String(), want) {
			t.Errorf("stdout lacks %q: %q", want, h.out.String())
		}
		b, err := os.ReadFile(filepath.Join(agents, name+".md"))
		if err != nil || !strings.Contains(string(b), "owner: grid") || !strings.Contains(string(b), "Body of "+name) {
			t.Errorf("%s: %v %q", name, err, b)
		}
	}
	if code := h.run(h.home, append(args, "--json")...); code != 0 {
		t.Fatalf("second run: exit %d: %s", code, h.errb.String())
	}
	got := decodeSync(t, h)
	if len(got) != 2 {
		t.Fatalf("results: %+v", got)
	}
	for _, r := range got {
		if r.State != "unchanged" || r.Path != filepath.Join(agents, r.Agent+".md") {
			t.Errorf("second run: %+v", r)
		}
	}
	if entries, _ := os.ReadDir(config); len(entries) != 0 {
		t.Errorf("an explicit --dir still wrote into the config folder: %v", entries)
	}
}

func TestAgentSyncLeavesThePersonsFileAlone(t *testing.T) {
	h := agentSyncHarness(t, "", t.TempDir())
	templates := agentTemplates(t, t.TempDir())
	agents := t.TempDir()
	mine := "---\nname: grid-beta\n---\nmy own agent, no Grid marker\n"
	path := filepath.Join(agents, "grid-beta.md")
	if err := os.WriteFile(path, []byte(mine), 0o644); err != nil {
		t.Fatal(err)
	}
	if code := h.run(h.home, "agent", "sync", "--templates", templates, "--dir", agents, "--json"); code != 0 {
		t.Fatalf("exit %d: %s", code, h.errb.String())
	}
	states := map[string]string{}
	for _, r := range decodeSync(t, h) {
		states[r.Agent] = r.State
	}
	if states["grid-alpha"] != "written" || states["grid-beta"] != "yours" || len(states) != 2 {
		t.Errorf("states: %v", states)
	}
	if b, err := os.ReadFile(path); err != nil || string(b) != mine {
		t.Errorf("the person's file changed: %v %q", err, b)
	}
}

func TestAgentSyncDefaultPaths(t *testing.T) {
	root, config := t.TempDir(), t.TempDir()
	agentTemplates(t, settings.AgentTemplatesPath(root))
	h := agentSyncHarness(t, root, config)
	if code := h.run(h.home, "agent", "sync", "--json"); code != 0 {
		t.Fatalf("exit %d: %s", code, h.errb.String())
	}
	got := decodeSync(t, h)
	if len(got) != 2 {
		t.Fatalf("results: %+v", got)
	}
	for _, r := range got {
		if r.State != "written" || r.Path != filepath.Join(config, "agents", r.Agent+".md") {
			t.Errorf("result: %+v", r)
		}
		if _, err := os.Stat(r.Path); err != nil {
			t.Error(err)
		}
	}
	// Nowhere else: the home folder holds no agents folder.
	if _, err := os.Stat(filepath.Join(h.home, ".claude", "agents")); err == nil {
		t.Error("a sync with CLAUDE_CONFIG_DIR set wrote under the home .claude")
	}
	// The Grid home and the plugin root hold only what they held.
	if entries, _ := os.ReadDir(config); len(entries) != 1 || entries[0].Name() != "agents" {
		t.Errorf("config folder: %v", entries)
	}
}

func TestAgentSyncMissingTemplatesFolder(t *testing.T) {
	config := t.TempDir()
	h := agentSyncHarness(t, "", config)
	missing := filepath.Join(t.TempDir(), "nope")
	if code := h.run(h.home, "agent", "sync", "--templates", missing, "--dir", filepath.Join(config, "agents")); code == 0 || h.errb.Len() == 0 {
		t.Errorf("exit %d, stderr %q", code, h.errb.String())
	}
	if code := h.run(h.home, "agent", "sync", "--dir", filepath.Join(config, "agents")); code == 0 {
		t.Errorf("no templates and no plugin root: exit %d", code)
	}
}
