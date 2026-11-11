package brain

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Accept and DecideTask publish what passes the keeper gate. The proposal check
// is a read the keeper may run (`grid brain check`); a finding does not stop an
// accept. These tests put a stand-in in ProposalCheck that always finds one
// thing, so any call that still consults the check would refuse.

const dxNoGateMark = "dx-no-gate-finding"

// dxNoGateFinding replaces ProposalCheck with a stand-in that reports one
// finding for every proposal and restores the real check when the test ends.
func dxNoGateFinding(t *testing.T) {
	t.Helper()
	real := ProposalCheck
	ProposalCheck = func(level, name string, n Note, b []byte, res *CheckResult) error {
		res.add("test_finding", scoped(level, "brain-proposals/"+name+".md"), 3, "always finds one thing", dxNoGateMark)
		return nil
	}
	t.Cleanup(func() { ProposalCheck = real })
}

func dxNoGateNote(level, topic string) Note {
	n := sample()
	n.Name, n.Topic, n.Level, n.Links, n.Body = "nogate", topic, level, nil, "Plain text.\n"
	return n
}

// dxNoGateRoot is the folder that holds the level's knowledge and log.
func dxNoGateRoot(s Store, level string) string {
	if level == "home" {
		return s.GridHome
	}
	return filepath.Join(s.GridHome, "projects", s.Project)
}

func dxNoGateWantNoEntry(t *testing.T, s Store, level, topic, name string) {
	t.Helper()
	if _, err := os.Stat(filepath.Join(dxNoGateRoot(s, level), "knowledge", topic, name+".md")); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("an entry was written: %v", err)
	}
}

// Rows 1 and 2: Accept at each level publishes although the check would find.
func TestDxNoGateAcceptPublishesDespiteAFinding(t *testing.T) {
	dxNoGateFinding(t)
	for _, level := range []string{"project", "home"} {
		t.Run(level, func(t *testing.T) {
			s := testStore(t)
			n := dxNoGateNote(level, "decisions")
			if _, err := s.Propose(n, n.Origin); err != nil {
				t.Fatal(err)
			}
			// The stand-in is in force: the read still reports its finding.
			if res, err := s.CheckProposal(level, n.Name); err != nil || len(res.Findings) != 1 {
				t.Fatalf("the check read: %+v %v", res, err)
			}
			if _, err := s.Accept(level, n.Name); err != nil {
				t.Fatalf("Accept: %v (%T)", err, err)
			}
			root := dxNoGateRoot(s, level)
			b, err := os.ReadFile(filepath.Join(root, "knowledge", n.Topic, n.Name+".md"))
			if err != nil {
				t.Fatalf("no entry under knowledge/%s/: %v", n.Topic, err)
			}
			if got, err := Parse(b); err != nil || got.Status != "accepted" {
				t.Fatalf("entry: %+v %v", got.Status, err)
			}
			got, err := s.Proposal(level, n.Name)
			if err != nil || got.Note.Status != "accepted" {
				t.Fatalf("proposal status: %v %v", got.Note.Status, err)
			}
			log, err := os.ReadFile(filepath.Join(root, "knowledge", "log.md"))
			if err != nil || strings.Count(string(log), "| accepted | "+n.Name+" |") != 1 || strings.Count(string(log), "\n- ") != 1 {
				t.Fatalf("audit log should hold one accepted line: %v\n%s", err, log)
			}
		})
	}
}

// Row 3: DecideTask accept on a task-folder proposal publishes and writes the receipt.
func TestDxNoGateDecideTaskAcceptPublishesDespiteAFinding(t *testing.T) {
	dxNoGateFinding(t)
	s, task := tdFixture(t, dxNoGateNote("project", "conventions"))
	h := &tdHistory{}
	if _, err := s.DecideTask(task, "nogate", "accept", "brain:project/nogate", "Worth keeping.", h.fn); err != nil {
		t.Fatalf("DecideTask: %v (%T)", err, err)
	}
	wantChain(t, s, "conventions", "nogate", 1, published(t, s, "conventions", "nogate"))
	if w := tdReceipt(t, task, "nogate"); w.Entry.Name != "nogate" || !w.Done {
		t.Fatalf("receipt %+v", w)
	}
	if p := tdProposal(t, task, "nogate"); p.Status != "accepted" {
		t.Fatalf("proposal status %q", p.Status)
	}
}

// merge-into took the same check before the gate went.
func TestDxNoGateDecideTaskMergeIntoPublishesDespiteAFinding(t *testing.T) {
	s, task, _ := tdMergeFixture(t)
	dxNoGateFinding(t)
	h := &tdHistory{}
	if _, err := s.DecideTask(task, "second", "merge-into", "brain:project/checks", "Adds a paragraph.", h.fn); err != nil {
		t.Fatalf("DecideTask: %v (%T)", err, err)
	}
	if got := string(published(t, s, "conventions", "checks")); !strings.Contains(got, "More about second.") {
		t.Fatalf("the merge did not publish:\n%s", got)
	}
	if w := tdReceipt(t, task, "second"); !w.Done {
		t.Fatalf("receipt %+v", w)
	}
}

// Row 4: a config topic is still refused, by the keeper gate and not by the check.
func TestDxNoGateConfigTopicStillRefusedByTheKeeperGate(t *testing.T) {
	dxNoGateFinding(t)
	const reason = "Config topics must be changed through their own commands."
	t.Run("Accept", func(t *testing.T) {
		s := testStore(t)
		n := dxNoGateNote("project", "agents")
		if _, err := s.Propose(n, n.Origin); err != nil {
			t.Fatal(err)
		}
		_, err := s.Accept("project", n.Name)
		if err == nil || !strings.Contains(err.Error(), "proposal rejected: "+reason) || strings.Contains(err.Error(), "proposal check") {
			t.Fatalf("Accept: %v", err)
		}
		got, perr := s.Proposal("project", n.Name)
		if perr != nil || got.Note.Status != "rejected" {
			t.Fatalf("proposal status: %v %v", got.Note.Status, perr)
		}
		dxNoGateWantNoEntry(t, s, "project", "agents", n.Name)
	})
	t.Run("DecideTask", func(t *testing.T) {
		s, task := tdFixture(t, dxNoGateNote("project", "agents"))
		_, err := s.DecideTask(task, "nogate", "accept", "brain:project/nogate", "", (&tdHistory{}).fn)
		tdWantErr(t, err, "proposal refused: "+reason)
		tdNoReceipt(t, task, "nogate")
		dxNoGateWantNoEntry(t, s, "project", "agents", "nogate")
	})
}
