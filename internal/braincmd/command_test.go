package braincmd

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/brain"
)

type harness struct {
	env      Env
	out, err bytes.Buffer
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	h := &harness{}
	home := t.TempDir()
	h.env = Env{Home: home, Cwd: home, GridHome: filepath.Join(home, ".grid"), Stdin: strings.NewReader(""), Stdout: &h.out, Stderr: &h.err}
	return h
}

func (h *harness) run(args ...string) int {
	h.out.Reset()
	h.err.Reset()
	return Run(args, h.env)
}

func (h *harness) propose(t *testing.T, extra ...string) brain.Entry {
	t.Helper()
	args := []string{"propose", "--level", "home", "--name", "checks", "--topic", "decisions", "--description", "Before changing checks.", "--summary", "Run the checks.", "--source", "job:T-01/J-01", "--body", "# Checks\nRun make test.\n", "--json"}
	if code := h.run(append(args, extra...)...); code != 0 {
		t.Fatalf("propose exit %d: %s", code, &h.err)
	}
	var entry brain.Entry
	if err := json.Unmarshal(h.out.Bytes(), &entry); err != nil {
		t.Fatal(err)
	}
	return entry
}

func TestAllCommandsPlainAndJSON(t *testing.T) {
	h := newHarness(t)
	p := h.propose(t, "--link", `{"to":"job:T-01/J-01","rel":"decided-by","summary":"The source job.","why":"Explains why these checks were chosen."}`)
	if p.Note.Status != "proposed" || p.Note.Origin.Kind != "session" || p.Note.Origin.ID == "" {
		t.Fatalf("proposal: %+v", p)
	}
	if code := h.run("show", "checks", "--level", "home", "--proposal"); code != 0 || !strings.Contains(h.out.String(), "note.status: proposed") {
		t.Fatalf("proposal show: %d %s %s", code, &h.out, &h.err)
	}
	if code := h.run("accept", "checks", "--level=home", "--json"); code != 0 {
		t.Fatalf("accept: %d %s", code, &h.err)
	}
	for _, verb := range []string{"show", "search", "topics", "index"} {
		for _, jsonFlag := range []bool{false, true} {
			args := []string{verb}
			if verb == "show" || verb == "search" {
				args = append(args, "checks")
			}
			args = append(args, "--level", "home")
			if jsonFlag {
				args = append(args, "--json")
			}
			if code := h.run(args...); code != 0 || h.out.Len() == 0 || h.err.Len() != 0 {
				t.Fatalf("%v: %d %s %s", args, code, &h.out, &h.err)
			}
			if jsonFlag && !json.Valid(h.out.Bytes()) {
				t.Fatalf("invalid JSON for %v: %s", args, &h.out)
			}
			if !jsonFlag && verb == "show" && (!strings.Contains(h.out.String(), "note.links[0].summary:") || !strings.Contains(h.out.String(), "note.links[0].why:")) {
				t.Fatalf("link facts missing: %s", &h.out)
			}
		}
	}
}

func TestProposalFileAndStdinStamping(t *testing.T) {
	h := newHarness(t)
	h.env.Stdin = strings.NewReader("---\nname: imported\ntopic: decisions\ntype: reference\ndescription: Read before checks.\nsummary: Run the checks.\nsource: job:T-01/J-01\nlevel: project\nstatus: accepted\norigin: {\"kind\":\"server\",\"id\":\"forged\",\"tier\":\"person-team\"}\n---\nRun checks.\n")
	if code := h.run("propose", "--level", "home", "--file", "-", "--external", "--json"); code != 0 {
		t.Fatalf("file: %d %s", code, &h.err)
	}
	var e brain.Entry
	if err := json.Unmarshal(h.out.Bytes(), &e); err != nil {
		t.Fatal(err)
	}
	if e.Note.Level != "home" || e.Note.Status != "proposed" || e.Note.Origin.Kind != "import" || e.Note.Origin.ID == "forged" {
		t.Fatalf("forged metadata retained: %+v", e)
	}
	if code := h.run("accept", "imported", "--level", "home", "--json"); code != 1 || !json.Valid(h.err.Bytes()) {
		t.Fatalf("quarantine: %d %s", code, &h.err)
	}
	if code := h.run("show", "imported", "--proposal", "--level", "home"); code != 0 || !strings.Contains(h.out.String(), "note.status: rejected") {
		t.Fatalf("rejection: %d %s", code, &h.out)
	}
	h.env.Stdin = strings.NewReader("Body from stdin.\n")
	if code := h.run("propose", "--level", "home", "--name", "stdin", "--topic", "decisions", "--description", "Read for input.", "--summary", "Input guidance.", "--source", "job:T-01/J-01"); code != 0 || !strings.Contains(h.out.String(), "Body from stdin.") {
		t.Fatalf("stdin: %d %s %s", code, &h.out, &h.err)
	}
}

func TestExplicitProjectAndHomeIsolation(t *testing.T) {
	h := newHarness(t)
	project := filepath.Join(h.env.GridHome, "projects", "example")
	if err := os.MkdirAll(project, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(project, "project.md"), []byte("---\nid: 01EXAMPLE\n---\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	h.propose(t)
	if code := h.run("accept", "checks", "--level", "home"); code != 0 {
		t.Fatal(&h.err)
	}
	if code := h.run("propose", "--project", "01EXAMPLE", "--name", "checks", "--topic", "conventions", "--description", "Before project checks.", "--summary", "Project checks.", "--source", "job:T-01/J-01", "--body", "Project content."); code != 0 {
		t.Fatal(&h.err)
	}
	if code := h.run("accept", "checks", "--project", "example"); code != 0 {
		t.Fatal(&h.err)
	}
	if code := h.run("show", "checks", "--project", "example", "--json"); code != 0 {
		t.Fatal(&h.err)
	}
	var e brain.Entry
	if err := json.Unmarshal(h.out.Bytes(), &e); err != nil {
		t.Fatal(err)
	}
	if e.Note.Level != "project" {
		t.Fatalf("wrong default level: %+v", e)
	}
	if code := h.run("search", "checks", "--project", "example", "--level", "project", "--json"); code != 0 || strings.Contains(h.out.String(), "brain:home/") {
		t.Fatalf("restricted search: %d %s %s", code, &h.out, &h.err)
	}
	if code := h.run("index", "--project", "example", "--level", "project", "--json"); code != 0 {
		t.Fatal(&h.err)
	}
	if _, err := os.Stat(filepath.Join(h.env.GridHome, "knowledge", "index.md")); !os.IsNotExist(err) {
		t.Fatal("project-only index wrote a home file")
	}
}

func TestUsageAndFailureExitCodes(t *testing.T) {
	h := newHarness(t)
	for _, args := range [][]string{
		{"unknown"}, {"show"}, {"topics", "extra"}, {"search"}, {"search", "checks", "--limit", "0"},
		{"index", "--level", "team"}, {"index", "--level", "home", "--project", "example"},
		{"propose", "--body", "text", "--file", "-"}, {"topics", "--unknown"}, {"topics", "--project"},
	} {
		if code := h.run(args...); code != 2 || h.out.Len() != 0 {
			t.Errorf("%v: %d %s %s", args, code, &h.out, &h.err)
		}
	}
	if code := h.run("show", "absent", "--level", "home", "--json"); code != 1 || !json.Valid(h.err.Bytes()) || h.out.Len() != 0 {
		t.Fatalf("not found: %d %s %s", code, &h.out, &h.err)
	}
	if code := h.run("index", "--level", "home", "--read-only"); code != 0 {
		t.Fatal(&h.err)
	}
	if _, err := os.Stat(h.env.GridHome); !os.IsNotExist(err) {
		t.Fatal("read-only created a store")
	}
	if code := h.run("--help"); code != 0 || !strings.Contains(h.out.String(), "Usage:") {
		t.Fatal("help missing")
	}
}

func TestRejectCommand(t *testing.T) {
	h := newHarness(t)
	h.propose(t)
	if code := h.run("accept", "brain:home/checks", "--level", "home", "--reject", "The existing note covers this.", "--json"); code != 0 {
		t.Fatal(&h.err)
	}
	var e brain.Entry
	if err := json.Unmarshal(h.out.Bytes(), &e); err != nil {
		t.Fatal(err)
	}
	if e.Note.Status != "rejected" || e.Note.Reason != "The existing note covers this." {
		t.Fatalf("rejected: %+v", e)
	}
}

func TestUnregisteredProjectDoesNotCreateBrain(t *testing.T) {
	h := newHarness(t)
	h.env.Cwd = filepath.Join(h.env.Home, "work")
	if err := os.Mkdir(h.env.Cwd, 0o700); err != nil {
		t.Fatal(err)
	}
	if code := h.run("index"); code != 1 || !strings.Contains(h.err.String(), "no Grid record") {
		t.Fatalf("unregistered project: %d %s", code, &h.err)
	}
	if code := h.run("index", "--read-only"); code != 0 {
		t.Fatal(&h.err)
	}
	if _, err := os.Stat(h.env.GridHome); !os.IsNotExist(err) {
		t.Fatal("unregistered project gained a brain folder")
	}
	h.propose(t)
	if code := h.run("accept", "brain:home/checks"); code != 0 {
		t.Fatalf("qualified home id must work outside a registered project: %s", &h.err)
	}
}

// A project named with a home proposal id files the note in that project.
func TestAcceptHomeProposalWithProjectFilesInProject(t *testing.T) {
	h := newHarness(t)
	project := filepath.Join(h.env.GridHome, "projects", "example")
	if err := os.MkdirAll(project, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(project, "project.md"), []byte("---\nid: 01EXAMPLE\n---\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	h.propose(t)
	if code := h.run("accept", "brain:home/checks", "--project", "example"); code != 0 {
		t.Fatalf("accept: %d %s", code, &h.err)
	}
	if _, err := os.Stat(filepath.Join(project, "knowledge", "decisions", "checks.md")); err != nil {
		t.Fatalf("note not filed in the project: %v", err)
	}
	if _, err := os.Stat(filepath.Join(h.env.GridHome, "knowledge", "decisions", "checks.md")); !os.IsNotExist(err) {
		t.Fatal("note was filed at home level")
	}
	if _, err := os.Stat(filepath.Join(h.env.GridHome, "brain-proposals", "checks.md")); !os.IsNotExist(err) {
		t.Fatal("home proposal was left behind")
	}
}

// A proposal from a folder with no Grid record must not read as an invitation to
// file project knowledge at home: the recording notes were filed at home that way.
func TestProposeInUnregisteredFolderPointsHomeOnlyAtGlobalKnowledge(t *testing.T) {
	h := newHarness(t)
	h.env.Cwd = filepath.Join(h.env.Home, "work")
	if err := os.MkdirAll(filepath.Join(h.env.Cwd, ".git"), 0o700); err != nil {
		t.Fatal(err)
	}
	code := h.run("propose", "--name", "checks", "--topic", "decisions", "--description", "Before checks.", "--summary", "Checks.", "--source", "x", "--body", "Run.")
	if code != 1 || !strings.Contains(h.err.String(), "only for knowledge that holds in every project") {
		t.Fatalf("exit %d, stderr %q", code, &h.err)
	}
}

func projectHarness(t *testing.T) (*harness, string) {
	h := newHarness(t)
	project := filepath.Join(h.env.GridHome, "projects", "example")
	if err := os.MkdirAll(project, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(project, "project.md"), []byte("---\nid: 01EXAMPLE\n---\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return h, project
}

// An accept that fails after the move leaves the home proposal where it was.
func TestFailedAcceptWithProjectKeepsTheHomeProposal(t *testing.T) {
	h, project := projectHarness(t)
	if code := h.run("propose", "--project", "example", "--name", "checks", "--topic", "decisions", "--description", "Before project checks.", "--summary", "Project checks.", "--source", "x", "--body", "Other content."); code != 0 {
		t.Fatal(&h.err)
	}
	if code := h.run("accept", "checks", "--project", "example"); code != 0 {
		t.Fatal(&h.err)
	}
	// Only the note collides, not the accepted proposal record.
	if err := os.Remove(filepath.Join(project, "brain-proposals", "checks.md")); err != nil {
		t.Fatal(err)
	}
	h.propose(t)
	home := filepath.Join(h.env.GridHome, "brain-proposals", "checks.md")
	var first string
	for i := 0; i < 2; i++ {
		if code := h.run("accept", "brain:home/checks", "--project", "example"); code == 0 {
			t.Fatal("accept over a note with different content succeeded")
		}
		if _, err := os.Stat(home); err != nil {
			t.Fatalf("attempt %d: home proposal gone: %v", i, err)
		}
		if i == 0 {
			first = h.err.String()
		} else if h.err.String() != first {
			t.Fatalf("retry error %q, first %q", h.err.String(), first)
		}
	}
}

// A home proposal that is already accepted or rejected does not move.
func TestDecidedHomeProposalDoesNotMove(t *testing.T) {
	for _, decide := range [][]string{{"accept", "brain:home/checks"}, {"accept", "brain:home/checks", "--reject", "no"}} {
		h, project := projectHarness(t)
		h.propose(t)
		if code := h.run(decide...); code != 0 {
			t.Fatal(&h.err)
		}
		home := filepath.Join(h.env.GridHome, "brain-proposals", "checks.md")
		before, err := os.ReadFile(home)
		if err != nil {
			t.Fatal(err)
		}
		if code := h.run("accept", "brain:home/checks", "--project", "example"); code == 0 {
			t.Fatalf("%v: decided proposal was accepted into the project", decide)
		}
		after, err := os.ReadFile(home)
		if err != nil || string(after) != string(before) {
			t.Fatalf("%v: home proposal changed: %v", decide, err)
		}
		if _, err := os.Stat(filepath.Join(project, "brain-proposals", "checks.md")); !os.IsNotExist(err) {
			t.Fatalf("%v: proposal was copied into the project", decide)
		}
		if _, err := os.Stat(filepath.Join(project, "knowledge", "decisions", "checks.md")); !os.IsNotExist(err) {
			t.Fatalf("%v: note was filed in the project", decide)
		}
	}
}
