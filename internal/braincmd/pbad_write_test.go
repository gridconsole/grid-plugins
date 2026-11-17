package braincmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/brain"
)

const pbAdNote = "---\ntitle: Plan\nowner: sam\n---\n# Plan\n\nFirst section.\n\n## Later\n\nSecond section.\n"

// pbAdWrite runs `grid brain write Ideas/Plan` with the agent's bytes in a file.
func pbAdWrite(t *testing.T, env Env, rev, agent string) (int, string, string) {
	t.Helper()
	f := filepath.Join(t.TempDir(), "agent.md")
	if err := os.WriteFile(f, []byte(agent), 0o600); err != nil {
		t.Fatal(err)
	}
	return qfRun(env, "write", "Ideas/Plan", "--expect", rev, "--file", f, "--level", "home", "--reason", "pbad")
}

func pbAdSetup(t *testing.T) (Env, string, string) {
	t.Helper()
	env, vault := pbVrEnv(t)
	note := filepath.Join(vault, "Ideas", "Plan.md")
	if err := os.WriteFile(note, []byte(pbAdNote), 0o644); err != nil {
		t.Fatal(err)
	}
	// The agent reads the note through Grid: the bytes are recorded and kept.
	if code, _, errs := qfRun(env, "show", "brain:home/Ideas/Plan"); code != 0 {
		t.Fatalf("show: %d %s", code, errs)
	}
	return env, vault, note
}

// F9 other lines, as the command: exit 0 and the written path, no proposal.
func TestPbAdWriteCommandRedoesOnTop(t *testing.T) {
	env, vault, note := pbAdSetup(t)
	rev := brain.NoteRev([]byte(pbAdNote))
	person := strings.Replace(pbAdNote, "First section.", "First section, and a person's line.", 1)
	if err := os.WriteFile(note, []byte(person), 0o644); err != nil {
		t.Fatal(err)
	}
	agent := strings.Replace(pbAdNote, "Second section.", "Second section, by the agent.", 1)
	code, out, errs := pbAdWrite(t, env, rev, agent)
	if code != 0 || !strings.Contains(out, note) || strings.Contains(out, "proposed:") {
		t.Fatalf("exit %d, stdout %q, stderr %q", code, out, errs)
	}
	got, _ := os.ReadFile(note)
	if want := strings.Replace(person, "Second section.", "Second section, by the agent.", 1); string(got) != want {
		t.Errorf("note %q, want %q", got, want)
	}
	if _, err := os.Stat(filepath.Join(vault, "Proposals")); err == nil {
		t.Errorf("Proposals/ exists")
	}
}

// F9 same lines, as the command: exit 0 with `proposed:`, the note keeps the
// person's bytes.
func TestPbAdWriteCommandProposesOnSameKey(t *testing.T) {
	env, vault, note := pbAdSetup(t)
	rev := brain.NoteRev([]byte(pbAdNote))
	person := strings.Replace(pbAdNote, "owner: sam", "owner: robin", 1)
	if err := os.WriteFile(note, []byte(person), 0o644); err != nil {
		t.Fatal(err)
	}
	agent := strings.Replace(pbAdNote, "owner: sam", "owner: kim", 1)
	code, out, errs := pbAdWrite(t, env, rev, agent)
	if code != 0 || !strings.HasPrefix(out, "proposed: "+filepath.Join(vault, "Proposals")) {
		t.Fatalf("exit %d, stdout %q, stderr %q", code, out, errs)
	}
	if got, _ := os.ReadFile(note); string(got) != person {
		t.Errorf("note %q, want the person's bytes %q", got, person)
	}
	entries, _ := os.ReadDir(filepath.Join(vault, "Proposals"))
	if len(entries) != 1 {
		t.Errorf("%d files in Proposals/, want 1", len(entries))
	}
}
