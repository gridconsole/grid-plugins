package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	contractdata "github.com/gridconsole/grid-plugins/contract"
	"github.com/gridconsole/grid-plugins/internal/project"
)

// 18 Nov: Grid-launched agents could not record a project's setup answers. grid setup runs
// inside their sandbox and dies on .settings.lock, and an exclusion for grid setup would also
// let an agent run the Claude Code form that writes its own deny rules. grid project settings
// record is the agent's verb: it records the answers, holds the keys that name a command for a
// person, and has no Claude Code form.

const psAnswers = `{"test":"make test","lands":"Open a pull request","deploys":"Deploy from this machine","deployCommand":"make deploy","starts":"make up","stops":"make down"}`

type psRecorded struct {
	Project  string            `json:"project"`
	Recorded map[string]string `json:"recorded"`
	Held     map[string]string `json:"held"`
	Hash     string            `json:"hash"`
}

// psEnv is Env.Getenv for a caller of role with member id member (empty for an agent).
func psEnv(h *harness, role, member string) func(string) string {
	return func(key string) string {
		switch key {
		case "GRID_ROLE":
			return role
		case "GRID_MEMBER":
			return member
		case "CLAUDE_CONFIG_DIR":
			return filepath.Join(h.home, ".claude")
		}
		return ""
	}
}

func psNew(t *testing.T) *harness {
	t.Helper()
	h := newHarness(t)
	h.record(project.Record{Name: "example"})
	h.env.Getenv = psEnv(h, "builder", "")
	return h
}

func (h *harness) psFile(name string) map[string]string {
	h.t.Helper()
	b, err := os.ReadFile(filepath.Join(h.gridHome, "projects", "example", name))
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		h.t.Fatal(err)
	}
	var m map[string]string
	if err := json.Unmarshal(b, &m); err != nil {
		h.t.Fatalf("%s: %v", name, err)
	}
	return m
}

func (h *harness) psRecord(answers string) psRecorded {
	h.t.Helper()
	path := filepath.Join(h.tmp, "answers.json")
	h.write(path, answers)
	if code := h.run(h.home, "project", "settings", "record", "example", "--answers", path, "--json"); code != 0 {
		h.t.Fatalf("record exit %d: %s", code, h.errb)
	}
	var got psRecorded
	if err := json.Unmarshal(h.out.Bytes(), &got); err != nil {
		h.t.Fatalf("record output %q: %v", h.out, err)
	}
	return got
}

func TestProjectSettingsRecordHoldsTheCommandKeys(t *testing.T) {
	h := psNew(t)
	got := h.psRecord(psAnswers)
	if got.Project != "example" || got.Hash == "" {
		t.Fatalf("record answered %+v", got)
	}
	for _, key := range []string{"test", "lands", "deploys"} {
		if got.Recorded[key] == "" {
			t.Errorf("recorded lacks %s: %+v", key, got.Recorded)
		}
	}
	answers := h.psFile("setup-answers.json")
	if answers["test"] != "make test" || answers["lands"] != "Open a pull request" || answers["deploys"] != "Deploy from this machine" {
		t.Errorf("setup-answers.json = %v", answers)
	}
	pending := h.psFile("setup-pending.json")
	for key, want := range map[string]string{"deployCommand": "make deploy", "starts": "make up", "stops": "make down"} {
		if _, ok := answers[key]; ok {
			t.Errorf("setup-answers.json holds the command key %s before a person approved it", key)
		}
		if got.Held[key] != want || pending[key] != want {
			t.Errorf("%s: held %q, pending %q, want %q", key, got.Held[key], pending[key], want)
		}
	}

	if code := h.run(h.home, "project", "settings", "show", "example", "--json"); code != 0 {
		t.Fatalf("show exit %d: %s", code, h.errb)
	}
	var shown struct {
		Answers map[string]string `json:"answers"`
		Pending map[string]string `json:"pending"`
		Hash    string            `json:"hash"`
	}
	if err := json.Unmarshal(h.out.Bytes(), &shown); err != nil {
		t.Fatal(err)
	}
	if shown.Hash != got.Hash || shown.Pending["starts"] != "make up" || shown.Answers["test"] != "make test" {
		t.Errorf("show = %+v, record hash %s", shown, got.Hash)
	}

	// The text form names what waits and the command a person runs.
	path := filepath.Join(h.tmp, "answers.json")
	if code := h.run(h.home, "project", "settings", "record", "example", "--answers", path); code != 0 {
		t.Fatalf("record exit %d: %s", code, h.errb)
	}
	if out := h.out.String(); !strings.Contains(out, "deployCommand") || !strings.Contains(out, "grid project settings approve example --hash "+got.Hash) {
		t.Errorf("text output does not name the held keys and the approve command:\n%s", out)
	}

	// The latest record is the whole proposal: one without a command key withdraws the held ones.
	h.psRecord(`{"test":"make test"}`)
	if h.psFile("setup-pending.json") != nil {
		t.Error("a record without command keys left the older ones waiting")
	}
}

func TestProjectSettingsRecordReadsStdin(t *testing.T) {
	h := psNew(t)
	h.env.Stdin = strings.NewReader(`{"test":"go test ./...","description":"An example"}`)
	if code := h.run(h.home, "project", "settings", "record", "example", "--answers", "-", "--json"); code != 0 {
		t.Fatalf("exit %d: %s", code, h.errb)
	}
	var got psRecorded
	if err := json.Unmarshal(h.out.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Hash != "" || len(got.Held) != 0 || h.psFile("setup-pending.json") != nil {
		t.Errorf("answers without a command key left something to approve: %+v", got)
	}
	if answers := h.psFile("setup-answers.json"); answers["test"] != "go test ./..." || answers["description"] != "An example" {
		t.Errorf("setup-answers.json = %v", answers)
	}
}

func TestProjectSettingsApproveWritesTheHeldCommands(t *testing.T) {
	h := psNew(t)
	got := h.psRecord(psAnswers)
	before := h.psFile("setup-answers.json")

	// An agent cannot approve; nor can a person with a hash that is not the pending one.
	if code := h.run(h.home, "project", "settings", "approve", "example", "--hash", got.Hash); code == 0 {
		t.Fatal("an agent approved the held commands")
	}
	// The app launches an agent with GRID_ROLE=worker and its holder's member id.
	h.env.Getenv = psEnv(h, "worker", "sam")
	h.errb.Reset()
	if code := h.run(h.home, "project", "settings", "approve", "example", "--hash", got.Hash); code == 0 || !strings.Contains(h.errb.String(), "for a person of the team") {
		t.Fatalf("a Grid-launched agent approve exit %d: %s", code, h.errb)
	}
	h.env.Getenv = psEnv(h, "", "sam")
	if code := h.run(h.home, "project", "settings", "approve", "example", "--hash", strings.Repeat("0", 64)); code == 0 {
		t.Fatal("a stale hash approved the held commands")
	}
	if code := h.run(h.home, "project", "settings", "approve", "example"); code != 2 {
		t.Fatalf("approve without --hash exit %d, want 2", code)
	}
	if answers := h.psFile("setup-answers.json"); len(answers) != len(before) || answers["starts"] != "" {
		t.Fatalf("a refused approve changed setup-answers.json: %v", answers)
	}

	if code := h.run(h.home, "project", "settings", "approve", "example", "--hash", got.Hash, "--json"); code != 0 {
		t.Fatalf("approve exit %d: %s", code, h.errb)
	}
	answers := h.psFile("setup-answers.json")
	if answers["deployCommand"] != "make deploy" || answers["starts"] != "make up" || answers["stops"] != "make down" || answers["test"] != "make test" {
		t.Errorf("after approve setup-answers.json = %v", answers)
	}
	if h.psFile("setup-pending.json") != nil {
		t.Error("approve left setup-pending.json")
	}

	// An agent that records again keeps the approved commands and cannot change them.
	h.env.Getenv = psEnv(h, "builder", "")
	again := h.psRecord(`{"test":"go test ./...","deploys":"Deploy from this machine","starts":"make up --fresh"}`)
	answers = h.psFile("setup-answers.json")
	if answers["test"] != "go test ./..." || answers["deployCommand"] != "make deploy" || answers["starts"] != "make up" || answers["lands"] != "" {
		t.Errorf("a second record = %v", answers)
	}
	if again.Held["starts"] != "make up --fresh" || h.psFile("setup-pending.json")["starts"] != "make up --fresh" {
		t.Errorf("a changed command was not held: %+v", again)
	}
}

// 18 Nov review: GRID_ROLE and GRID_MEMBER are an agent's own to set, so with no setup record
// `GRID_ROLE=owner GRID_MEMBER=x grid project settings approve` passed inside the sandbox. The
// sandbox denies writes to both setup files whatever the record; read-only files stand in for it.
func TestProjectSettingsApproveRefusesInsideTheSandbox(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("chmod does not deny writes as root")
	}
	h := psNew(t)
	got := h.psRecord(psAnswers)
	dir := filepath.Join(h.gridHome, "projects", "example")
	before := map[string][]byte{}
	for _, name := range []string{"setup-answers.json", "setup-pending.json"} {
		path := filepath.Join(dir, name)
		b, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		before[name] = b
		if err := os.Chmod(path, 0o444); err != nil {
			t.Fatal(err)
		}
	}
	h.env.Getenv = psEnv(h, "owner", "x")
	h.errb.Reset()
	if code := h.run(h.home, "project", "settings", "approve", "example", "--hash", got.Hash); code == 0 || !strings.Contains(h.errb.String(), "outside Claude Code or Codex") {
		t.Fatalf("a spoofed owner inside the sandbox: approve exit %d: %s", code, h.errb)
	}
	for name, want := range before {
		if b, err := os.ReadFile(filepath.Join(dir, name)); err != nil || string(b) != string(want) {
			t.Errorf("a refused approve changed %s: %q, %v", name, b, err)
		}
	}
}

func TestProjectSettingsRecordRefusesTheClaudeForm(t *testing.T) {
	h := psNew(t)
	path := filepath.Join(h.tmp, "answers.json")
	h.write(path, `{"test":"make test"}`)
	for _, args := range [][]string{
		{"project", "settings", "record", "--yes", "--answers", path},
		{"project", "settings", "record", "example", "--yes", "--answers", path},
		{"project", "settings", "record", "example", "--remove", "--answers", path},
		{"project", "settings", "record", "example", "--sandbox", "--answers", path},
		{"project", "settings", "record", "example", "--shared-group", "staff", "--answers", path},
		{"project", "settings", "record", "--answers", path},
		{"project", "settings", "record", "example"},
		{"project", "settings", "--yes", "--answers", path},
		{"project", "settings", "setup", "example", "--answers", path},
	} {
		if code := h.run(h.home, args...); code != 2 {
			t.Errorf("grid %s: exit %d, want 2", strings.Join(args, " "), code)
		}
	}
	if _, err := os.Stat(filepath.Join(h.home, ".claude")); !os.IsNotExist(err) {
		t.Errorf("the record verb touched the Claude Code config folder: %v", err)
	}
	if h.psFile("setup-answers.json") != nil {
		t.Error("a refused record wrote setup-answers.json")
	}
}

func TestProjectSettingsRecordWritesNothingOnInvalidAnswers(t *testing.T) {
	h := psNew(t)
	path := filepath.Join(h.tmp, "answers.json")
	for _, bad := range []string{`{"deploys":"CI/CD deploys","deployCommand":"make deploy"}`, `{"bogus":"x"}`, `{"test":1}`} {
		h.write(path, bad)
		if code := h.run(h.home, "project", "settings", "record", "example", "--answers", path); code == 0 {
			t.Errorf("%s was recorded", bad)
		}
	}
	if h.psFile("setup-answers.json") != nil || h.psFile("setup-pending.json") != nil {
		t.Error("invalid answers wrote a file")
	}
	h.env.Getenv = psEnv(h, "guest", "")
	h.write(path, `{"test":"make test"}`)
	if code := h.run(h.home, "project", "settings", "record", "example", "--answers", path); code == 0 {
		t.Error("a guest recorded setup answers")
	}
}

// The schema holds the three real outputs; UPDATE_YN_FIXTURES=1 writes them as fixtures.
func TestYnProjectSettings(t *testing.T) {
	h := ynSpNew(t)
	cwd := ynSpRegister(t, h, "example", "https://example.com/org/example.git")
	h.write(filepath.Join(cwd, "answers.json"), psAnswers)
	recorded := ynSpRun(t, h, cwd, "project", "settings", "record", "example", "--answers", "answers.json", "--json")
	ynSpRequire(t, "cmd-project-settings", recorded, "project", "recorded", "held", "hash")
	shown := ynSpRun(t, h, cwd, "project", "settings", "show", "example", "--json")
	ynSpRequire(t, "cmd-project-settings", shown, "project", "answers", "pending", "hash")
	var got psRecorded
	if err := json.Unmarshal(recorded, &got); err != nil {
		t.Fatal(err)
	}
	approved := ynSpRun(t, h, cwd, "project", "settings", "approve", "example", "--hash", got.Hash, "--json")
	ynSpRequire(t, "cmd-project-settings", approved, "project", "approved", "answers")

	ynSpWrite(t, h, "cmd-project-settings", "valid", "basic", recorded)
	ynSpWrite(t, h, "cmd-project-settings", "valid", "populated", shown)
	ynSpWrite(t, h, "cmd-project-settings", "valid", "approved", approved)
	ynSpInvalid(t, h, "cmd-project-settings", recorded, "project")
}

// commands.json lists the agent's verb with the flags the setup skill names, and none of the
// Claude Code form's flags, so the sandbox exclusion for it can never reach that form.
func TestContractProjectSettingsListedInCommands(t *testing.T) {
	var doc contractCommandDocument
	if err := json.Unmarshal(contractdata.Commands, &doc); err != nil {
		t.Fatalf("contract/commands.json does not parse: %v", err)
	}
	got := map[string]string{}
	found := false
	for _, c := range doc.Commands {
		if c.Name != "project" {
			continue
		}
		for _, sub := range c.Subcommands {
			if sub.Name != "settings" {
				continue
			}
			found = true
			if strings.Join(sub.Args, " ") != "<record|show|approve> <project>" {
				t.Errorf("project settings args = %v", sub.Args)
			}
			for _, f := range sub.Flags {
				got[f.Name] = f.Takes
			}
		}
	}
	if !found {
		t.Fatal("commands.json has no project settings")
	}
	for _, name := range []string{"--answers", "--hash"} {
		if got[name] != "value" {
			t.Errorf("project settings %s takes %q, want value; flags: %v", name, got[name], got)
		}
	}
	for _, name := range []string{"--yes", "--remove", "--sandbox", "--shared-group"} {
		if _, ok := got[name]; ok {
			t.Errorf("project settings lists the Claude Code form's flag %s", name)
		}
	}
}
