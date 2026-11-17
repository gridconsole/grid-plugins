package braincmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/brain"
)

// Row 5, as the command: a write based on the note's current rev to a note a person
// made (no Grid record) is written on top, exits 0 with the written path, writes no
// proposal and keeps the person's bytes under .grid/versions/.
func TestPbAdWriteCommandOnPersonNote(t *testing.T) {
	env, vault := pbVrEnv(t)
	note := filepath.Join(vault, "Ideas", "Hand.md")
	person := "A line a person wrote.\n"
	if err := os.WriteFile(note, []byte(person), 0o644); err != nil {
		t.Fatal(err)
	}
	agent := person + "\n## Added\n\nBy the agent.\n"
	f := filepath.Join(t.TempDir(), "agent.md")
	if err := os.WriteFile(f, []byte(agent), 0o600); err != nil {
		t.Fatal(err)
	}
	rev := brain.NoteRev([]byte(person))
	code, out, errs := qfRun(env, "write", "Ideas/Hand", "--expect", rev, "--file", f, "--level", "home", "--reason", "pbad")
	if code != 0 || !strings.Contains(out, note) || strings.Contains(out, "proposed:") {
		t.Fatalf("exit %d, stdout %q, stderr %q", code, out, errs)
	}
	if got, _ := os.ReadFile(note); string(got) != agent {
		t.Errorf("note %q, want %q", got, agent)
	}
	if _, err := os.Stat(filepath.Join(vault, "Proposals")); err == nil {
		t.Errorf("Proposals/ exists")
	}
	v := filepath.Join(vault, ".grid", "versions", "home", "Ideas", "Hand", rev[:16]+".version")
	if b, err := os.ReadFile(v); err != nil || string(b) != person {
		t.Errorf("the person's previous bytes are not under .grid/versions/: %v", err)
	}
}

// Row 9, as the commands: a draft in Proposals/ changed on disk with no Grid record
// is adopted by `grid brain check` as edited, origin person, and `grid brain list
// --json` shows the proposal with origin person.
func TestPbAdCheckAdoptsDraftAndListShowsPersonOrigin(t *testing.T) {
	env, vault := pbVrEnv(t)
	draft := filepath.Join(vault, "Proposals", "draft.md")
	if err := os.MkdirAll(filepath.Dir(draft), 0o755); err != nil {
		t.Fatal(err)
	}
	text := "---\nstatus: proposed\n---\n# Draft\n\nA person wrote this draft.\n"
	if err := os.WriteFile(draft, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
	code, out, errs := qfRun(env, "check", "--json")
	if code != 0 {
		t.Fatalf("check: exit %d, stdout %q, stderr %q", code, out, errs)
	}
	var res struct {
		Adopted  int
		Findings []any
	}
	if err := json.Unmarshal([]byte(out), &res); err != nil || len(res.Findings) != 0 {
		t.Fatalf("check output %q: %v", out, err)
	}
	if _, err := os.Stat(filepath.Join(vault, ".grid", "versions", "home", "Proposals", "draft", brain.NoteRev([]byte(text))[:16]+".version")); err != nil {
		t.Errorf("the draft's version is not under .grid/versions/<level>/Proposals/<name>/: %v", err)
	}
	code, out, errs = qfRun(env, "list", "--json")
	if code != 0 {
		t.Fatalf("list: exit %d, stderr %q", code, errs)
	}
	var list struct {
		Notes []struct {
			ID, Origin, Kind string
		}
	}
	if err := json.Unmarshal([]byte(out), &list); err != nil {
		t.Fatalf("list output %q: %v", out, err)
	}
	for _, n := range list.Notes {
		if n.ID == "brain:home/Proposals/draft" {
			if n.Origin != "person" || n.Kind != "proposal" {
				t.Errorf("row = %+v", n)
			}
			return
		}
	}
	t.Fatalf("no listing row for the draft: %s", out)
}

// Rows 10 and 11, as the command: one secret-shaped finding with the file and the
// line and no text, exit 3 as for any finding, no part of the value in the JSON
// output, and the note is left alone.
func TestPbAdCheckCommandSecretShapedFinding(t *testing.T) {
	env, vault := pbVrEnv(t)
	const secret = "ghp_" + "abcdefghijklmnopqrstuvwxyz0123456789"
	text := "# Keys\n\nSome words.\napi_key = " + secret + "\nMore words.\n"
	note := filepath.Join(vault, "Ideas", "Keys.md")
	if err := os.WriteFile(note, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
	code, out, errs := qfRun(env, "check", "--json")
	if code != 3 {
		t.Fatalf("exit %d, stdout %q, stderr %q; want 3", code, out, errs)
	}
	for _, all := range []string{out, errs} {
		for _, part := range []string{secret, secret[4:20], "abcdefghijklmnop"} {
			if strings.Contains(all, part) {
				t.Errorf("the output holds the matched value (%q): %s", part, all)
			}
		}
	}
	var res struct {
		Findings []struct {
			Code, File, Text string
			Line             int
		}
	}
	if err := json.Unmarshal([]byte(out), &res); err != nil {
		t.Fatalf("output %q: %v", out, err)
	}
	if len(res.Findings) != 1 || res.Findings[0].Code != "secret-shaped" || res.Findings[0].File != "Ideas/Keys.md" || res.Findings[0].Line != 4 || res.Findings[0].Text != "" {
		t.Fatalf("findings = %+v", res.Findings)
	}
	if got, _ := os.ReadFile(note); string(got) != text {
		t.Errorf("the note's bytes changed")
	}
	if _, err := os.Stat(filepath.Join(vault, "Proposals")); err == nil {
		t.Errorf("Proposals/ exists")
	}
}
