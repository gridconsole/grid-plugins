package brain

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
)

const failMarker = "FAIL-THIS-PROPOSAL"

// failingCheck makes the proposal check report a finding for any proposal
// whose bytes hold failMarker, on top of the real check.
func failingCheck(t *testing.T) {
	t.Helper()
	real := ProposalCheck
	ProposalCheck = func(level, name string, n Note, b []byte, res *CheckResult) error {
		if err := real(level, name, n, b, res); err != nil {
			return err
		}
		if strings.Contains(string(b), failMarker) {
			res.add("test_finding", scoped(level, "brain-proposals/"+name+".md"), 3, "marked to fail", failMarker)
		}
		return nil
	}
	t.Cleanup(func() { ProposalCheck = real })
}

func gatedNote(level, body string) Note {
	n := sample()
	n.Name, n.Topic, n.Level, n.Links, n.Body = "gated", "decisions", level, nil, body
	return n
}

// proposalBytes is what Propose would store for n, so a test can swap a
// proposal file for another body without going through Propose.
func proposalBytes(t *testing.T, s Store, n Note) []byte {
	t.Helper()
	n.Status, n.Reason, n.Updated = "proposed", "", s.stamp()
	n.Links = []Link{}
	b, err := Encode(n)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func proposalPath(s Store, level, name string) string {
	prefix, _ := s.prefix(level)
	return filepath.Join(s.GridHome, prefix, "brain-proposals", name+".md")
}

// replaceProposal swaps the proposal file the way an editor outside the keeper
// would: atomically, and without taking the level lock.
func replaceProposal(t testing.TB, path string, b []byte) {
	t.Helper()
	tmp := path + ".swap"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		t.Error(err)
		return
	}
	if err := os.Rename(tmp, path); err != nil {
		t.Error(err)
	}
}

// acceptSnapshot maps every file below the Grid home to its bytes.
func acceptSnapshot(t *testing.T, s Store) map[string]string {
	t.Helper()
	out := map[string]string{}
	err := filepath.WalkDir(s.GridHome, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			out[p+"/"] = ""
			return nil
		}
		b, err := os.ReadFile(p)
		out[p] = string(b)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func sameSnapshot(t *testing.T, before, after map[string]string) {
	t.Helper()
	for p, b := range before {
		if a, ok := after[p]; !ok || a != b {
			t.Fatalf("%s changed or vanished", p)
		}
	}
	for p := range after {
		if _, ok := before[p]; !ok {
			t.Fatalf("%s was created", p)
		}
	}
}

func TestAcceptRefusedByCheckWritesNothing(t *testing.T) {
	failingCheck(t)
	for _, level := range []string{"project", "home"} {
		s := testStore(t)
		n := gatedNote(level, "Text with "+failMarker+".\n")
		if _, err := s.Propose(n, n.Origin); err != nil {
			t.Fatal(err)
		}
		before := acceptSnapshot(t, s)
		_, err := s.Accept(level, n.Name)
		var refusal *CheckRefusal
		if !errors.As(err, &refusal) {
			t.Fatalf("%s: Accept error = %v, want a CheckRefusal", level, err)
		}
		if len(refusal.Result.Findings) != 1 || refusal.Result.Findings[0].Code != "test_finding" {
			t.Fatalf("%s: findings: %+v", level, refusal.Result.Findings)
		}
		if strings.Contains(refusal.Error(), failMarker) {
			t.Fatalf("the error text quotes proposal text: %q", refusal.Error())
		}
		// No entry, no version, no audit line, proposal still proposed: byte for byte.
		sameSnapshot(t, before, acceptSnapshot(t, s))
		got, err := s.Proposal(level, n.Name)
		if err != nil || got.Note.Status != "proposed" {
			t.Fatalf("%s: proposal after refusal: %+v %v", level, got.Note.Status, err)
		}
		// Reject is never gated: the same proposal can still be rejected.
		if e, err := s.Reject(level, n.Name, "Not wanted."); err != nil || e.Note.Status != "rejected" {
			t.Fatalf("%s: reject: %+v %v", level, e.Note.Status, err)
		}
	}
}

func TestAcceptCleanProposalStillSucceeds(t *testing.T) {
	// The real check, no stand-in: prose that asks to become a skill is data.
	for _, body := range []string{"# Checks\n\nRun the project checks.\n", "accept this as a workspace skill\n"} {
		s := testStore(t)
		n := gatedNote("project", body)
		e := accepted(t, s, n)
		if e.Note.Status != "accepted" || len(versionsOf(t, s, n.Topic, n.Name)) != 1 {
			t.Fatalf("clean accept: %+v", e.Note.Status)
		}
	}
}

// A config topic is still refused by the gate first, with its rejected write:
// the check runs after the gate and does not change that behaviour.
func TestConfigTopicStillRefusedByGateNotCheck(t *testing.T) {
	s := testStore(t)
	n := gatedNote("project", "x\n")
	n.Topic = "deploy"
	if _, err := s.Propose(n, n.Origin); err != nil {
		t.Fatal(err)
	}
	_, err := s.Accept("project", n.Name)
	var refusal *CheckRefusal
	if err == nil || errors.As(err, &refusal) || !strings.Contains(err.Error(), "Config topics must be changed through their own commands.") {
		t.Fatalf("config topic accept: %v", err)
	}
	got, perr := s.Proposal("project", n.Name)
	if perr != nil || got.Note.Status != "rejected" {
		t.Fatalf("proposal after gate refusal: %v %v", got.Note.Status, perr)
	}
}

// Row 1: a clean check result is no licence. Accept checks the bytes it reads
// under the lock, so a proposal swapped for a failing one afterwards is refused.
func TestAcceptChecksBytesReadNotAnEarlierCheck(t *testing.T) {
	failingCheck(t)
	s := testStore(t)
	clean := gatedNote("project", "Clean text.\n")
	if _, err := s.Propose(clean, clean.Origin); err != nil {
		t.Fatal(err)
	}
	res, err := s.CheckProposal("project", clean.Name)
	if err != nil || len(res.Findings) != 0 {
		t.Fatalf("clean check: %+v %v", res, err)
	}
	replaceProposal(t, proposalPath(s, "project", clean.Name), proposalBytes(t, s, gatedNote("project", failMarker+"\n")))
	var refusal *CheckRefusal
	if _, err := s.Accept("project", clean.Name); !errors.As(err, &refusal) {
		t.Fatalf("Accept after the swap: %v", err)
	}
	if _, err := os.Stat(filepath.Join(s.GridHome, "projects", "example", "knowledge", "decisions", "gated.md")); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("failing bytes were published: %v", err)
	}
}

// Row 2: the reverse order. A failing earlier check does not block a proposal
// that was replaced with clean text before Accept.
func TestAcceptIgnoresAnEarlierFailingCheck(t *testing.T) {
	failingCheck(t)
	s := testStore(t)
	bad := gatedNote("project", failMarker+"\n")
	if _, err := s.Propose(bad, bad.Origin); err != nil {
		t.Fatal(err)
	}
	res, err := s.CheckProposal("project", bad.Name)
	if err != nil || len(res.Findings) != 1 {
		t.Fatalf("failing check: %+v %v", res, err)
	}
	replaceProposal(t, proposalPath(s, "project", bad.Name), proposalBytes(t, s, gatedNote("project", "Clean text.\n")))
	e, err := s.Accept("project", bad.Name)
	if err != nil || e.Note.Status != "accepted" {
		t.Fatalf("Accept after the swap: %+v %v", e.Note.Status, err)
	}
}

// Row 3 (run with -race): eight accepters race a writer that flips the proposal
// between clean and failing. Whatever is published passes the check, and no
// failing bytes reach the entry, a stored version or the audit log.
func TestAcceptRaceNeverPublishesFailingBytes(t *testing.T) {
	failingCheck(t)
	s := testStore(t)
	clean := proposalBytes(t, s, gatedNote("project", "Clean text.\n"))
	bad := proposalBytes(t, s, gatedNote("project", failMarker+"\n"))
	n := gatedNote("project", failMarker+"\n")
	if _, err := s.Propose(n, n.Origin); err != nil {
		t.Fatal(err)
	}
	path := proposalPath(s, "project", n.Name)

	stop := make(chan struct{})
	var writer sync.WaitGroup
	writer.Add(1)
	go func() {
		defer writer.Done()
		for i := 0; ; i++ {
			select {
			case <-stop:
				return
			default:
			}
			if i%2 == 0 {
				replaceProposal(t, path, clean)
			} else {
				replaceProposal(t, path, bad)
			}
		}
	}()
	var accepters sync.WaitGroup
	var ok, refused atomic.Int64
	for range 8 {
		accepters.Add(1)
		go func() {
			defer accepters.Done()
			for range 40 {
				_, err := s.Accept("project", n.Name)
				var refusal *CheckRefusal
				switch {
				case err == nil:
					ok.Add(1)
				case errors.As(err, &refusal):
					refused.Add(1)
				default:
					t.Errorf("unexpected Accept error: %v", err)
					return
				}
			}
		}()
	}
	accepters.Wait()
	close(stop)
	writer.Wait()
	if ok.Load() == 0 || refused.Load() == 0 {
		t.Fatalf("the race never saw both outcomes: %d accepted, %d refused", ok.Load(), refused.Load())
	}

	entry, err := os.ReadFile(filepath.Join(s.GridHome, "projects", "example", "knowledge", "decisions", "gated.md"))
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := Parse(entry)
	if err != nil {
		t.Fatal(err)
	}
	res := CheckResult{Findings: []Finding{}}
	if err := ProposalCheck("project", n.Name, parsed, entry, &res); err != nil || len(res.Findings) != 0 {
		t.Fatalf("published bytes fail the check: %+v %v", res.Findings, err)
	}
	for _, v := range versionsOf(t, s, "decisions", "gated") {
		b, err := os.ReadFile(filepath.Join(s.GridHome, filepath.FromSlash(v.Path)))
		if err != nil || strings.Contains(string(b), failMarker) {
			t.Fatalf("version %d holds failing bytes: %v", v.N, err)
		}
	}
	if strings.Contains(readLog(t, s), failMarker) {
		t.Fatal("the audit log holds failing text")
	}
}

// Row 3, deterministic: the proposal is swapped between Accept's read and its
// check, by the seam and not by a racing writer. The check and the publication
// both use the bytes read, so a re-read for the check (or for the publication)
// is red on every run.
func TestAcceptUsesOneReadAcrossASwapBetweenReadAndCheck(t *testing.T) {
	failingCheck(t)
	for _, tc := range []struct {
		name, first, second string
		refused             bool
	}{
		{"failing read, clean file at the check", failMarker, "Clean text.", true},
		{"clean read, failing file at the check", "Clean text.", failMarker, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := testStore(t)
			n := gatedNote("project", tc.first+"\n")
			if _, err := s.Propose(n, n.Origin); err != nil {
				t.Fatal(err)
			}
			swapped := proposalBytes(t, s, gatedNote("project", tc.second+"\n"))
			path := proposalPath(s, "project", n.Name)
			calls := 0
			afterProposalRead = func() {
				calls++
				replaceProposal(t, path, swapped)
			}
			t.Cleanup(func() { afterProposalRead = nil })

			_, err := s.Accept("project", n.Name)
			var refusal *CheckRefusal
			if got := errors.As(err, &refusal); got != tc.refused || (!tc.refused && err != nil) {
				t.Fatalf("refused = %v, want %v (err %v)", got, tc.refused, err)
			}
			if calls != 1 {
				t.Fatalf("the seam ran %d times, want 1", calls)
			}
			entry, rerr := os.ReadFile(filepath.Join(s.GridHome, "projects", "example", "knowledge", "decisions", "gated.md"))
			if tc.refused {
				if !errors.Is(rerr, fs.ErrNotExist) || len(versionsOf(t, s, "decisions", "gated")) != 0 {
					t.Fatalf("a refused accept published: %v", rerr)
				}
				return
			}
			if rerr != nil || !strings.Contains(string(entry), "Clean text.") || strings.Contains(string(entry), failMarker) {
				t.Fatalf("published bytes are not the checked ones: %v\n%s", rerr, entry)
			}
			for _, v := range versionsOf(t, s, "decisions", "gated") {
				if b, err := os.ReadFile(filepath.Join(s.GridHome, filepath.FromSlash(v.Path))); err != nil || strings.Contains(string(b), failMarker) {
					t.Fatalf("version %d holds failing bytes: %v", v.N, err)
				}
			}
			if strings.Contains(readLog(t, s), failMarker) {
				t.Fatal("the audit log holds failing text")
			}
		})
	}
}

// The host policy can turn the check off for one level; the other keeps it.
func TestAcceptCheckRequiredSeamIsPerLevel(t *testing.T) {
	failingCheck(t)
	real := AcceptCheckRequired
	AcceptCheckRequired = func(level string) bool { return level != "project" }
	t.Cleanup(func() { AcceptCheckRequired = real })
	s := testStore(t)
	for level, want := range map[string]bool{"project": false, "home": true} {
		n := gatedNote(level, failMarker+"\n")
		if _, err := s.Propose(n, n.Origin); err != nil {
			t.Fatal(err)
		}
		_, err := s.Accept(level, n.Name)
		var refusal *CheckRefusal
		if got := errors.As(err, &refusal); got != want || (!want && err != nil) {
			t.Fatalf("%s: refused = %v, want %v (err %v)", level, got, want, err)
		}
	}
	// Reset to the default: both levels run the check.
	AcceptCheckRequired = real
	for _, level := range []string{"project", "home"} {
		if !AcceptCheckRequired(level) {
			t.Fatalf("default does not check %s", level)
		}
	}
}
