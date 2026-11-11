package brain

import (
	"bytes"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"
)

// Tests for Store.DecideTask, the curator's accept, reject and merge-into path.
// The task folder holds brain-proposals/<name>.md; the receipt for a decision
// is brain-proposals/.decisions/<name>.json in the same folder.

// tdHistory is the history callback: it records every line and can be made to
// fail its next calls.
type tdHistory struct {
	mu    sync.Mutex
	lines []string
	fails int
}

func (h *tdHistory) fn(line string) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.lines = append(h.lines, line)
	if h.fails > 0 {
		h.fails--
		return errors.New("history is down")
	}
	return nil
}

func (h *tdHistory) count() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.lines)
}

func tdFixture(t *testing.T, n Note) (Store, TaskRef) {
	t.Helper()
	return testStore(t), taskWithProposal(t, t.TempDir(), n)
}

// tdMergeFixture holds an accepted "checks" note and a task proposal "second"
// with its own source and link, to be merged into it.
func tdMergeFixture(t *testing.T) (Store, TaskRef, []byte) {
	t.Helper()
	s := testStore(t)
	accepted(t, s, sample())
	p := extra("second")
	p.Source = "job:T-02/J-09"
	p.Links = []Link{{To: "job:T-02/J-09", Rel: "decided-by", Summary: "The job that added the second paragraph.", Why: "Explains where the paragraph came from."}}
	return s, taskWithProposal(t, t.TempDir(), p), published(t, s, "conventions", "checks")
}

func tdReceiptPath(task TaskRef, proposal string) string {
	return filepath.Join(task.Path, "brain-proposals", ".decisions", proposal+".json")
}

func tdReceipt(t *testing.T, task TaskRef, proposal string) decisionWrite {
	t.Helper()
	b, err := os.ReadFile(tdReceiptPath(task, proposal))
	if err != nil {
		t.Fatal(err)
	}
	var w decisionWrite
	if err := json.Unmarshal(b, &w); err != nil {
		t.Fatal(err)
	}
	return w
}

func tdSetReceipt(t *testing.T, task TaskRef, proposal string, w decisionWrite) {
	t.Helper()
	b, err := json.Marshal(w)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(tdReceiptPath(task, proposal), b, 0o600); err != nil {
		t.Fatal(err)
	}
}

func tdNoReceipt(t *testing.T, task TaskRef, proposal string) {
	t.Helper()
	if _, err := os.Stat(tdReceiptPath(task, proposal)); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("a receipt was written: %v", err)
	}
}

func tdProposal(t *testing.T, task TaskRef, name string) Note {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(task.Path, "brain-proposals", name+".md"))
	if err != nil {
		t.Fatal(err)
	}
	n, err := Parse(b)
	if err != nil {
		t.Fatal(err)
	}
	return n
}

// tdTree lists every directory and file under the roots with its bytes; a root
// that does not exist is empty.
func tdTree(t *testing.T, roots ...string) map[string]string {
	t.Helper()
	got := map[string]string{}
	for _, root := range roots {
		err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				if p == root && errors.Is(err, fs.ErrNotExist) {
					return nil
				}
				return err
			}
			if d.IsDir() {
				got[p+"/"] = ""
				return nil
			}
			b, err := os.ReadFile(p)
			if err != nil {
				return err
			}
			got[p] = string(b)
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	return got
}

func tdWantErr(t *testing.T, err error, want string) {
	t.Helper()
	if err == nil || !strings.Contains(err.Error(), want) {
		t.Fatalf("error %v, want one containing %q", err, want)
	}
}

// Argument refusals come first: nothing is read or written, no history line.
func TestDecideTaskRefusesBadArguments(t *testing.T) {
	const oneLine = "note must be at most 300 characters on one line"
	cases := []struct{ name, proposal, action, target, note, want string }{
		{"proposal is a path", "a/b", "accept", "", "", "proposal id must be a plain name"},
		{"proposal is empty", "", "accept", "", "", "proposal id must be a plain name"},
		{"action is not a decision", "draft", "approve", "", "", "decision must be accept, reject or merge-into"},
		{"action is empty", "draft", "", "", "", "decision must be accept, reject or merge-into"},
		{"note is 301 characters", "draft", "accept", "", strings.Repeat("x", 301), oneLine},
		{"note is 301 multibyte characters", "draft", "reject", "", strings.Repeat("é", 301), oneLine},
		{"note has a newline", "draft", "accept", "", "first\nsecond", oneLine},
		{"note has a carriage return", "draft", "accept", "", "first\rsecond", oneLine},
		{"target level is unknown", "draft", "accept", "brain:nowhere/draft", "", "id must be brain:<home|project>/<name>"},
		{"target is not a plain name", "draft", "accept", "../draft", "", "note id must be a plain name"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s, task := tdFixture(t, extra("draft"))
			before := tdTree(t, s.GridHome, task.Path)
			h := &tdHistory{}
			_, err := s.DecideTask(task, c.proposal, c.action, c.target, c.note, h.fn)
			tdWantErr(t, err, c.want)
			if after := tdTree(t, s.GridHome, task.Path); !reflect.DeepEqual(before, after) {
				t.Fatalf("a refused call wrote files:\nbefore %v\nafter  %v", before, after)
			}
			tdNoReceipt(t, task, "draft")
			if h.count() != 0 {
				t.Fatalf("history called for a refused call: %q", h.lines)
			}
		})
	}
}

// The note limit is 300 characters, not bytes.
func TestDecideTaskTakesA300CharacterNote(t *testing.T) {
	s, task := tdFixture(t, extra("draft"))
	note := strings.Repeat("é", 300)
	got, err := s.DecideTask(task, "draft", "reject", "draft", note, (&tdHistory{}).fn)
	if err != nil {
		t.Fatal(err)
	}
	if got.Note != note {
		t.Fatalf("note %q", got.Note)
	}
	if n := tdProposal(t, task, "draft"); n.Status != "rejected" || n.Reason != note {
		t.Fatalf("proposal after reject: %+v", n)
	}
}

// What is not a proposed note of that name in the task folder is refused
// before any receipt.
func TestDecideTaskRefusesWhatIsNotAProposal(t *testing.T) {
	t.Run("missing", func(t *testing.T) {
		s, task := tdFixture(t, extra("draft"))
		_, err := s.DecideTask(task, "absent", "accept", "absent", "", (&tdHistory{}).fn)
		if err == nil || !errors.Is(err, fs.ErrNotExist) {
			t.Fatalf("error %v, want not exist", err)
		}
		tdNoReceipt(t, task, "absent")
	})
	t.Run("already accepted", func(t *testing.T) {
		n := extra("draft")
		n.Status = "accepted"
		s, task := tdFixture(t, n)
		_, err := s.DecideTask(task, "draft", "accept", "draft", "", (&tdHistory{}).fn)
		tdWantErr(t, err, "expected matching proposed note")
		tdNoReceipt(t, task, "draft")
	})
	t.Run("file name differs from the note name", func(t *testing.T) {
		s, task := tdFixture(t, extra("draft"))
		b, err := Encode(extra("other"))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(task.Path, "brain-proposals", "draft.md"), b, 0o600); err != nil {
			t.Fatal(err)
		}
		_, err = s.DecideTask(task, "draft", "accept", "draft", "", (&tdHistory{}).fn)
		tdWantErr(t, err, "expected matching proposed note")
		tdNoReceipt(t, task, "draft")
	})
}

// Row 2: accept files the note under knowledge/<topic>/ at the target level,
// keeps a receipt, runs history once, and a repeat decides nothing again.
func TestDecideTaskAcceptFilesTheNoteAndRepeatsAsTheSameDecision(t *testing.T) {
	s, task := tdFixture(t, extra("draft"))
	h := &tdHistory{}
	got, err := s.DecideTask(task, "draft", "accept", "brain:project/final", "Worth keeping.", h.fn)
	if err != nil {
		t.Fatal(err)
	}
	want := TaskDecision{Task: "T-01", Proposal: "draft", Action: "accept", Target: "brain:project/final", Note: "Worth keeping.", At: "2026-01-02T03:04:05.000Z"}
	if got != want {
		t.Fatalf("decision %+v, want %+v", got, want)
	}

	path := filepath.Join(s.GridHome, "projects", "example", "knowledge", "conventions", "final.md")
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("note not filed under knowledge/<topic>/: %v", err)
	}
	n, err := Parse(b)
	if err != nil || n.Name != "final" || n.Status != "accepted" || n.Level != "project" || n.Topic != "conventions" {
		t.Fatalf("filed note %+v, %v", n, err)
	}
	if !strings.Contains(n.Body, "More about draft.") || !strings.Contains(n.Body, "Source: [T-01](") || !strings.Contains(n.Body, "task.md), proposal `draft`.") {
		t.Fatalf("filed body %q", n.Body)
	}
	if _, err := os.Stat(filepath.Join(s.GridHome, "knowledge", "conventions", "final.md")); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("note also landed at the home level: %v", err)
	}

	w := tdReceipt(t, task, "draft")
	if w.TaskDecision != want || w.Action != "accept" || !w.Noted || !w.Done {
		t.Fatalf("receipt %+v", w)
	}
	if w.Entry.Name != "final" || w.Entry.Status != "accepted" || len(w.Before) != 0 {
		t.Fatalf("receipt entry %+v before %q", w.Entry, w.Before)
	}
	if p := tdProposal(t, task, "draft"); p.Status != "accepted" || p.Reason != "accept brain:project/final; Worth keeping." {
		t.Fatalf("proposal kept as the audit record: %+v", p)
	}
	if wantLines := []string{"brain decided: draft accept brain:project/final; Worth keeping."}; !reflect.DeepEqual(h.lines, wantLines) {
		t.Fatalf("history lines %q, want %q", h.lines, wantLines)
	}
	wantChain(t, s, "conventions", "final", 1, b)

	receipt, err := os.ReadFile(tdReceiptPath(task, "draft"))
	if err != nil {
		t.Fatal(err)
	}
	again, err := s.DecideTask(task, "draft", "accept", "brain:project/final", "Worth keeping.", h.fn)
	if err != nil {
		t.Fatalf("repeat: %v", err)
	}
	if again != got {
		t.Fatalf("repeat returned %+v, want %+v", again, got)
	}
	if h.count() != 1 {
		t.Fatalf("repeat called history again: %q", h.lines)
	}
	if now := published(t, s, "conventions", "final"); !bytes.Equal(now, b) {
		t.Fatalf("repeat rewrote the note:\n%s", now)
	}
	if now, _ := os.ReadFile(tdReceiptPath(task, "draft")); !bytes.Equal(now, receipt) {
		t.Fatalf("repeat rewrote the receipt:\n%s", now)
	}
	wantChain(t, s, "conventions", "final", 1, b)
}

// The receipt is read back by later runs, so its keys are part of the format.
func TestDecideTaskReceiptKeepsItsFormat(t *testing.T) {
	s, task := tdFixture(t, extra("draft"))
	if _, err := s.DecideTask(task, "draft", "accept", "draft", "", (&tdHistory{}).fn); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(tdReceiptPath(task, "draft"))
	if err != nil {
		t.Fatal(err)
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(b, &raw); err != nil {
		t.Fatal(err)
	}
	keys := make([]string, 0, len(raw))
	for k := range raw {
		keys = append(keys, k)
	}
	wantKeys := []string{"action", "at", "before", "done", "entry", "note", "noted", "proposal", "proposal_note", "target", "task"}
	have := map[string]bool{}
	for _, k := range keys {
		have[k] = true
	}
	if len(keys) != len(wantKeys) {
		t.Fatalf("receipt keys %v, want %v", keys, wantKeys)
	}
	for _, k := range wantKeys {
		if !have[k] {
			t.Fatalf("receipt lacks %q: %s", k, b)
		}
	}
	var task2, target string
	if json.Unmarshal(raw["task"], &task2) != nil || task2 != "T-01" || json.Unmarshal(raw["target"], &target) != nil || target != "brain:project/draft" {
		t.Fatalf("receipt task %s target %s", raw["task"], raw["target"])
	}
}

// A rejection publishes nothing and says so in the proposal.
func TestDecideTaskRejectPublishesNothing(t *testing.T) {
	s, task := tdFixture(t, extra("draft"))
	h := &tdHistory{}
	got, err := s.DecideTask(task, "draft", "reject", "draft", "", h.fn)
	if err != nil {
		t.Fatal(err)
	}
	if got.Action != "reject" || got.Target != "brain:project/draft" {
		t.Fatalf("decision %+v", got)
	}
	if _, err := os.Stat(filepath.Join(s.GridHome, "projects", "example", "knowledge", "conventions", "draft.md")); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("a rejected proposal was published: %v", err)
	}
	if p := tdProposal(t, task, "draft"); p.Status != "rejected" || p.Reason != "Rejected by curator." {
		t.Fatalf("proposal %+v", p)
	}
	if w := tdReceipt(t, task, "draft"); w.Action != "reject" || !w.Noted || !w.Done {
		t.Fatalf("receipt %+v", w)
	}
	if want := []string{"brain decided: draft reject brain:project/draft"}; !reflect.DeepEqual(h.lines, want) {
		t.Fatalf("history lines %q, want %q", h.lines, want)
	}
	if got := versionsOf(t, s, "conventions", "draft"); len(got) != 0 {
		t.Fatalf("a rejection stored a version: %+v", got)
	}
}

// A target without a level means the project, or home when the Store has none.
func TestDecideTaskDefaultsTheTargetLevel(t *testing.T) {
	t.Run("project", func(t *testing.T) {
		s, task := tdFixture(t, extra("draft"))
		got, err := s.DecideTask(task, "draft", "accept", "final", "", (&tdHistory{}).fn)
		if err != nil {
			t.Fatal(err)
		}
		if got.Target != "brain:project/final" {
			t.Fatalf("target %q", got.Target)
		}
		published(t, s, "conventions", "final")
	})
	t.Run("home without a project", func(t *testing.T) {
		n := extra("draft")
		n.Level, n.Topic = "home", "decisions" // conventions is a project topic
		s, task := tdFixture(t, n)
		s.Project = ""
		got, err := s.DecideTask(task, "draft", "accept", "final", "", (&tdHistory{}).fn)
		if err != nil {
			t.Fatal(err)
		}
		if got.Target != "brain:home/final" {
			t.Fatalf("target %q", got.Target)
		}
		b, err := os.ReadFile(filepath.Join(s.GridHome, "knowledge", "decisions", "final.md"))
		if err != nil {
			t.Fatalf("note not filed at home: %v", err)
		}
		if n, err := Parse(b); err != nil || n.Status != "accepted" || n.Level != "home" {
			t.Fatalf("home note %+v, %v", n, err)
		}
		if w := tdReceipt(t, task, "draft"); w.Target != "brain:home/final" || !w.Done {
			t.Fatalf("receipt %+v", w)
		}
	})
	t.Run("project proposal on a store without a project", func(t *testing.T) {
		s, task := tdFixture(t, extra("draft"))
		s.Project = ""
		_, err := s.DecideTask(task, "draft", "accept", "final", "", (&tdHistory{}).fn)
		tdWantErr(t, err, "proposal level disagrees with destination")
		tdNoReceipt(t, task, "draft")
	})
}

// Row 3: conflicts. None of the refused calls leaves a receipt, except the
// repeat with a different decision, which leaves the first receipt as it was.
func TestDecideTaskRefusesADifferentSecondDecision(t *testing.T) {
	s, task := tdFixture(t, extra("draft"))
	h := &tdHistory{}
	if _, err := s.DecideTask(task, "draft", "accept", "brain:project/final", "Keep.", h.fn); err != nil {
		t.Fatal(err)
	}
	receipt, err := os.ReadFile(tdReceiptPath(task, "draft"))
	if err != nil {
		t.Fatal(err)
	}
	note := published(t, s, "conventions", "final")
	other := TaskRef{ID: "T-02", Path: task.Path}
	for _, c := range []struct {
		name                   string
		task                   TaskRef
		action, target, remark string
	}{
		{"another action", task, "reject", "brain:project/final", "Keep."},
		{"another target", task, "accept", "brain:project/elsewhere", "Keep."},
		{"another note", task, "accept", "brain:project/final", "Changed my mind."},
		{"no note", task, "accept", "brain:project/final", ""},
		{"another task", other, "accept", "brain:project/final", "Keep."},
	} {
		_, err := s.DecideTask(c.task, "draft", c.action, c.target, c.remark, h.fn)
		if err == nil || !strings.Contains(err.Error(), "proposal already has a different decision") {
			t.Fatalf("%s: error %v", c.name, err)
		}
	}
	if now, _ := os.ReadFile(tdReceiptPath(task, "draft")); !bytes.Equal(now, receipt) {
		t.Fatalf("a refused decision rewrote the receipt:\n%s", now)
	}
	if !bytes.Equal(published(t, s, "conventions", "final"), note) {
		t.Fatal("a refused decision changed the note")
	}
	if h.count() != 1 {
		t.Fatalf("history lines %q", h.lines)
	}
	if p := tdProposal(t, task, "draft"); p.Status != "accepted" {
		t.Fatalf("proposal %+v", p)
	}
}

func TestDecideTaskRefusesAcceptOntoAnExistingNote(t *testing.T) {
	s := testStore(t)
	accepted(t, s, sample())
	original := published(t, s, "conventions", "checks")
	task := taskWithProposal(t, t.TempDir(), extra("second"))
	h := &tdHistory{}
	_, err := s.DecideTask(task, "second", "accept", "brain:project/checks", "", h.fn)
	tdWantErr(t, err, "note brain:project/checks already exists; use merge-into")
	tdNoReceipt(t, task, "second")
	if !bytes.Equal(published(t, s, "conventions", "checks"), original) {
		t.Fatal("the existing note changed")
	}
	if p := tdProposal(t, task, "second"); p.Status != "proposed" {
		t.Fatalf("proposal %+v", p)
	}
	if h.count() != 0 {
		t.Fatalf("history lines %q", h.lines)
	}
}

func TestDecideTaskRefusesMergeIntoAMissingTarget(t *testing.T) {
	s, task := tdFixture(t, extra("second"))
	h := &tdHistory{}
	_, err := s.DecideTask(task, "second", "merge-into", "brain:project/checks", "", h.fn)
	tdWantErr(t, err, "merge target brain:project/checks does not exist")
	tdNoReceipt(t, task, "second")
	if p := tdProposal(t, task, "second"); p.Status != "proposed" {
		t.Fatalf("proposal %+v", p)
	}
	if h.count() != 0 {
		t.Fatalf("history lines %q", h.lines)
	}
}

func TestDecideTaskRefusesATargetAtTheOtherLevel(t *testing.T) {
	for _, action := range []string{"accept", "merge-into", "reject"} {
		t.Run(action, func(t *testing.T) {
			s, task := tdFixture(t, extra("draft")) // a project proposal
			before := tdTree(t, task.Path)
			_, err := s.DecideTask(task, "draft", action, "brain:home/draft", "", (&tdHistory{}).fn)
			tdWantErr(t, err, "proposal level disagrees with destination")
			tdNoReceipt(t, task, "draft")
			// Only the lock file may appear in the task folder.
			after := tdTree(t, task.Path)
			delete(after, filepath.Join(task.Path, ".brain-decide.lock"))
			if !reflect.DeepEqual(before, after) {
				t.Fatalf("task folder changed:\nbefore %v\nafter  %v", before, after)
			}
			if _, err := os.Stat(filepath.Join(s.GridHome, "knowledge", "conventions", "draft.md")); !errors.Is(err, fs.ErrNotExist) {
				t.Fatalf("the note was published at the other level: %v", err)
			}
		})
	}
}

// Row 4: merge-into updates the target with the proposal.
func TestDecideTaskMergeIntoUpdatesTheTarget(t *testing.T) {
	s, task, original := tdMergeFixture(t)
	h := &tdHistory{}
	if _, err := s.DecideTask(task, "second", "merge-into", "brain:project/checks", "Same topic.", h.fn); err != nil {
		t.Fatal(err)
	}
	b := published(t, s, "conventions", "checks")
	n, err := Parse(b)
	if err != nil {
		t.Fatal(err)
	}
	first, second := strings.Index(n.Body, "Run the project checks."), strings.Index(n.Body, "More about second.")
	if first < 0 || second < first {
		t.Fatalf("merged body keeps the target first, then the proposal:\n%s", n.Body)
	}
	if n.Name != "checks" || n.Status != "accepted" || n.Source != "job:T-01/J-01; job:T-02/J-09" {
		t.Fatalf("merged note %+v", n)
	}
	if len(n.Links) != 2 || n.Links[1].To != "job:T-02/J-09" {
		t.Fatalf("merged links %+v", n.Links)
	}
	if !strings.Contains(n.Body, "proposal `second`.") {
		t.Fatalf("merged body names no source:\n%s", n.Body)
	}
	if w := tdReceipt(t, task, "second"); !bytes.Equal(w.Before, original) || !w.Done || !w.Noted {
		t.Fatalf("receipt before %q, done %v noted %v", w.Before, w.Done, w.Noted)
	}
	if p := tdProposal(t, task, "second"); p.Status != "accepted" || p.Reason != "merge-into brain:project/checks; Same topic." {
		t.Fatalf("proposal %+v", p)
	}
	if want := []string{"brain decided: second merge-into brain:project/checks; Same topic."}; !reflect.DeepEqual(h.lines, want) {
		t.Fatalf("history lines %q, want %q", h.lines, want)
	}
	wantChain(t, s, "conventions", "checks", 2, b)
}

// Row 4: a receipt that is not done and a target edited since refuse to publish.
func TestDecideTaskRefusesToOverwriteADestinationEditedAfterTheDecision(t *testing.T) {
	s, task, _ := tdMergeFixture(t)
	h := &tdHistory{}
	if _, err := s.DecideTask(task, "second", "merge-into", "brain:project/checks", "", h.fn); err != nil {
		t.Fatal(err)
	}
	// As if the process stopped after the receipt: not noted, not done, and a
	// person edited the target meanwhile.
	w := tdReceipt(t, task, "second")
	w.Noted, w.Done = false, false
	tdSetReceipt(t, task, "second", w)
	edited := append(published(t, s, "conventions", "checks"), "\nEdited by a person.\n"...)
	path := filepath.Join(s.GridHome, "projects", "example", "knowledge", "conventions", "checks.md")
	if err := os.WriteFile(path, edited, 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := s.DecideTask(task, "second", "merge-into", "brain:project/checks", "", h.fn)
	tdWantErr(t, err, "destination changed since decision; refusing to overwrite it")
	if b, _ := os.ReadFile(path); !bytes.Equal(b, edited) {
		t.Fatalf("the edit was overwritten:\n%s", b)
	}
	if got := tdReceipt(t, task, "second"); got.Done || got.Noted {
		t.Fatalf("receipt finished despite the refusal: %+v", got)
	}
	if h.count() != 1 {
		t.Fatalf("history lines %q", h.lines)
	}
}

// The other interrupted state: stopped before publishing, the target is still
// the bytes the receipt saw, so the retry publishes and finishes.
func TestDecideTaskFinishesADecisionStoppedBeforePublication(t *testing.T) {
	s, task, original := tdMergeFixture(t)
	h := &tdHistory{}
	if _, err := s.DecideTask(task, "second", "merge-into", "brain:project/checks", "", h.fn); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(s.GridHome, "projects", "example", "knowledge", "conventions", "checks.md")
	merged := mustRead(t, path)
	w := tdReceipt(t, task, "second")
	w.Noted, w.Done = false, false
	tdSetReceipt(t, task, "second", w)
	if err := os.WriteFile(path, original, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := s.DecideTask(task, "second", "merge-into", "brain:project/checks", "", h.fn); err != nil {
		t.Fatal(err)
	}
	if b := mustRead(t, path); !bytes.Equal(b, merged) {
		t.Fatalf("retry published other bytes:\n%s", b)
	}
	if got := tdReceipt(t, task, "second"); !got.Noted || !got.Done {
		t.Fatalf("receipt %+v", got)
	}
	if h.count() != 2 {
		t.Fatalf("history lines %q", h.lines)
	}
	wantChain(t, s, "conventions", "checks", 2, merged)
}

func mustRead(t *testing.T, path string) []byte {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// Row 5: a history failure leaves the note published and the receipt not
// noted; the next call runs history again and finishes without merging twice.
func TestDecideTaskRetriesAFailedHistoryWithoutMergingAgain(t *testing.T) {
	s, task, _ := tdMergeFixture(t)
	h := &tdHistory{fails: 1}
	path := filepath.Join(s.GridHome, "projects", "example", "knowledge", "conventions", "checks.md")

	_, err := s.DecideTask(task, "second", "merge-into", "brain:project/checks", "", h.fn)
	tdWantErr(t, err, "history is down")
	w := tdReceipt(t, task, "second")
	if w.Noted || w.Done {
		t.Fatalf("receipt after a failed history: noted %v done %v", w.Noted, w.Done)
	}
	merged := mustRead(t, path)
	if h.count() != 1 {
		t.Fatalf("history lines %q", h.lines)
	}

	got, err := s.DecideTask(task, "second", "merge-into", "brain:project/checks", "", h.fn)
	if err != nil {
		t.Fatalf("retry: %v", err)
	}
	if got != w.TaskDecision {
		t.Fatalf("retry decided %+v, receipt had %+v", got, w.TaskDecision)
	}
	if h.count() != 2 || h.lines[0] != h.lines[1] {
		t.Fatalf("retry history lines %q", h.lines)
	}
	if w := tdReceipt(t, task, "second"); !w.Noted || !w.Done {
		t.Fatalf("receipt after the retry: noted %v done %v", w.Noted, w.Done)
	}
	if b := mustRead(t, path); !bytes.Equal(b, merged) {
		t.Fatalf("retry rewrote the note:\n%s", b)
	}
	if n := strings.Count(string(merged), "More about second."); n != 1 {
		t.Fatalf("proposal merged %d times:\n%s", n, merged)
	}
	wantChain(t, s, "conventions", "checks", 2, merged)
	if n := strings.Count(readLog(t, s), "merge-into brain:project/checks"); n != 1 {
		t.Fatalf("log holds the decision %d times:\n%s", n, readLog(t, s))
	}

	// Done now: a third call does not call history.
	if _, err := s.DecideTask(task, "second", "merge-into", "brain:project/checks", "", h.fn); err != nil || h.count() != 2 {
		t.Fatalf("third call: %v, history lines %q", err, h.lines)
	}
}

// History runs after the level lock is released and after publication: the
// note is on disk, the receipt is written and not noted, and another writer
// can take the level lock meanwhile.
func TestDecideTaskRunsHistoryAfterTheLevelLock(t *testing.T) {
	s, task := tdFixture(t, extra("draft"))
	var inside struct {
		receipt  decisionWrite
		filed    bool
		lockFree bool
	}
	history := func(line string) error {
		inside.receipt = tdReceipt(t, task, "draft")
		_, err := os.Stat(filepath.Join(s.GridHome, "projects", "example", "knowledge", "conventions", "draft.md"))
		inside.filed = err == nil
		done := make(chan error, 1)
		go func() {
			_, err := s.Propose(extra("other"), Origin{Kind: "job", ID: "T-01/J-01"})
			done <- err
		}()
		select {
		case err := <-done:
			inside.lockFree = err == nil
		case <-time.After(10 * time.Second):
		}
		return nil
	}
	if _, err := s.DecideTask(task, "draft", "accept", "draft", "", history); err != nil {
		t.Fatal(err)
	}
	if inside.receipt.Action != "accept" || inside.receipt.Noted || inside.receipt.Done {
		t.Fatalf("receipt seen by history: %+v", inside.receipt)
	}
	if !inside.filed {
		t.Fatal("history ran before the note was published")
	}
	if !inside.lockFree {
		t.Fatal("history ran while the level lock was held")
	}
	if w := tdReceipt(t, task, "draft"); !w.Noted || !w.Done {
		t.Fatalf("receipt after history: %+v", w)
	}
}

// Noted is written when history succeeds and Done after the index is rebuilt.
// With the index blocked the receipt stays Noted and not Done, and the retry
// finishes without a second history line.
func TestDecideTaskWritesNotedBeforeDone(t *testing.T) {
	s, task := tdFixture(t, extra("draft"))
	h := &tdHistory{}
	blocker := filepath.Join(s.GridHome, "projects", "example", "knowledge", "backlinks.json")
	if err := os.MkdirAll(filepath.Join(blocker, "x"), 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := s.DecideTask(task, "draft", "accept", "draft", "", h.fn); err == nil {
		t.Fatal("a blocked index rebuild was hidden")
	}
	if w := tdReceipt(t, task, "draft"); !w.Noted || w.Done {
		t.Fatalf("receipt with the index blocked: noted %v done %v", w.Noted, w.Done)
	}
	if err := os.RemoveAll(blocker); err != nil {
		t.Fatal(err)
	}
	if _, err := s.DecideTask(task, "draft", "accept", "draft", "", h.fn); err != nil {
		t.Fatalf("retry: %v", err)
	}
	if w := tdReceipt(t, task, "draft"); !w.Noted || !w.Done {
		t.Fatalf("receipt after the retry: noted %v done %v", w.Noted, w.Done)
	}
	if h.count() != 1 {
		t.Fatalf("history lines %q", h.lines)
	}
	wantChain(t, s, "conventions", "draft", 1, published(t, s, "conventions", "draft"))
}

// Row 6: goroutines deciding one proposal. The task lock serialises them: one
// decision lands, the same decision repeats for the others, a different one is
// refused with "different decision" and nothing else.
func TestDecideTaskSameDecisionAtOnce(t *testing.T) {
	s, task := tdFixture(t, extra("draft"))
	h := &tdHistory{}
	const n = 8
	got := make([]TaskDecision, n)
	errs := make([]error, n)
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			got[i], errs[i] = s.DecideTask(task, "draft", "accept", "brain:project/final", "", h.fn)
		}()
	}
	close(start)
	wg.Wait()
	for i := range errs {
		if errs[i] != nil {
			t.Fatalf("call %d: %v", i, errs[i])
		}
		if got[i] != got[0] {
			t.Fatalf("call %d decided %+v, call 0 %+v", i, got[i], got[0])
		}
	}
	if h.count() != 1 {
		t.Fatalf("history lines %q", h.lines)
	}
	b := published(t, s, "conventions", "final")
	if c := strings.Count(string(b), "Source: [T-01]("); c != 1 {
		t.Fatalf("source line %d times:\n%s", c, b)
	}
	wantChain(t, s, "conventions", "final", 1, b)
	if w := tdReceipt(t, task, "draft"); !w.Noted || !w.Done {
		t.Fatalf("receipt %+v", w)
	}
}

func TestDecideTaskDifferentActionsAtOnceLandOneDecision(t *testing.T) {
	for round := 0; round < 12; round++ {
		s, task := tdFixture(t, extra("draft"))
		h := &tdHistory{}
		actions := []string{"accept", "reject", "accept", "reject", "reject", "accept"}
		errs := make([]error, len(actions))
		start := make(chan struct{})
		var wg sync.WaitGroup
		for i, action := range actions {
			wg.Add(1)
			go func() {
				defer wg.Done()
				<-start
				_, errs[i] = s.DecideTask(task, "draft", action, "brain:project/draft", "", h.fn)
			}()
		}
		close(start)
		wg.Wait()

		w := tdReceipt(t, task, "draft")
		if !w.Noted || !w.Done {
			t.Fatalf("round %d: receipt %+v", round, w)
		}
		landed := 0
		for i, err := range errs {
			switch {
			case err == nil && actions[i] == w.Action:
				landed++
			case err != nil && actions[i] != w.Action && strings.Contains(err.Error(), "different decision"):
			default:
				t.Fatalf("round %d: %s call returned %v, the decision is %s", round, actions[i], err, w.Action)
			}
		}
		if landed == 0 {
			t.Fatalf("round %d: no call landed the %s decision", round, w.Action)
		}
		if h.count() != 1 {
			t.Fatalf("round %d: history lines %q", round, h.lines)
		}
		path := filepath.Join(s.GridHome, "projects", "example", "knowledge", "conventions", "draft.md")
		_, statErr := os.Stat(path)
		status := tdProposal(t, task, "draft").Status
		if w.Action == "accept" {
			if statErr != nil || status != "accepted" {
				t.Fatalf("round %d: accepted but note %v, proposal %s", round, statErr, status)
			}
			wantChain(t, s, "conventions", "draft", 1, mustRead(t, path))
		} else if !errors.Is(statErr, fs.ErrNotExist) || status != "rejected" {
			t.Fatalf("round %d: rejected but note %v, proposal %s", round, statErr, status)
		}
	}
}
