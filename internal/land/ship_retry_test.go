package land_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/land"
	"github.com/gridconsole/grid-plugins/internal/task"
)

// A failed land that keeps the person's approval (Retry) arms the next pass: the same
// ship Job lands again with no new answer, once, and nothing re-asks the person.
func TestShipRetryRunsLandAgainWithoutANewAnswer(t *testing.T) {
	s, dir, _ := r7Fixture(t, nil)
	merges := 0
	s.Merge = func(string) (string, error) {
		merges++
		if merges == 1 {
			return "", errors.New("merging lane into main conflicts in a.txt")
		}
		return "fedcba9876543210fedcba9876543210fedcba98", nil
	}
	if _, ran, err := s.AfterShip(context.Background(), "", "T-01", "J-6"); !ran || err == nil {
		t.Fatalf("AfterShip = ran %t, %v; want a failed land", ran, err)
	}
	if armed, err := s.Armed("", "T-01", "J-6"); err != nil || armed {
		t.Fatalf("Armed after a failed land with no retry = %t, %v; want false", armed, err)
	}
	if err := s.Retry("", "T-01", "J-6"); err != nil {
		t.Fatal(err)
	}
	if armed, err := s.Armed("", "T-01", "J-6"); err != nil || !armed {
		t.Fatalf("Armed after Retry = %t, %v; want true", armed, err)
	}
	if _, ran, err := s.AfterShip(context.Background(), "", "T-01", "J-6"); err != nil || !ran || merges != 2 {
		t.Fatalf("AfterShip after Retry = ran %t, %v, %d merges; want a second merge", ran, err, merges)
	}
	if rec, err := task.Read(dir); err != nil || rec.State != "closed" {
		t.Fatalf("task state %q %v after the retry, want closed", rec.State, err)
	}
	if armed, _ := s.Armed("", "T-01", "J-6"); armed {
		t.Fatal("Armed after the land ran, want false")
	}
	if _, ran, err := s.AfterShip(context.Background(), "", "T-01", "J-6"); err != nil || ran || merges != 2 {
		t.Fatalf("a third AfterShip = ran %t, %v, %d merges; want no more", ran, err, merges)
	}
}

// RebaseQueued keeps the target tip and RebaseTip reads the newest one back for the
// ship Job it names; a line without a tip, or for another Job, gives none.
func TestShipRebaseTipIsKeptWithTheQueuedRebase(t *testing.T) {
	s, dir, _ := r7Fixture(t, nil)
	tip := "0123456789abcdef0123456789abcdef01234567"
	if err := s.RebaseQueued("", "T-01", "J-6", "J-9", ""); err != nil {
		t.Fatal(err)
	}
	if rebase, got, err := s.RebaseTip("", "T-01", "J-6"); err != nil || rebase != "J-9" || got != "" {
		t.Fatalf("RebaseTip of a line with no tip = %q %q %v; want J-9 and no tip", rebase, got, err)
	}
	if err := s.RebaseQueued("", "T-01", "J-6", "J-10.1", tip); err != nil {
		t.Fatal(err)
	}
	rebase, got, err := s.RebaseTip("", "T-01", "J-6")
	if err != nil || rebase != "J-10.1" || got == "" || !strings.HasPrefix(tip, got) {
		t.Fatalf("RebaseTip = %q %q %v; want J-10.1 and a prefix of %s", rebase, got, err, tip)
	}
	if rebase, got, _ := s.RebaseTip("", "T-01", "J-7"); rebase != "" || got != "" {
		t.Fatalf("RebaseTip of another Job = %q %q; want none", rebase, got)
	}
	if h := r7History(t, dir); !strings.Contains(h, "waits for rebase J-10.1 (target tip "+tip[:12]+"), which keeps both changes") {
		t.Fatalf("history does not hold the rebase line with its tip:\n%s", h)
	}
	if pending, err := (land.Store{Resolver: s.Resolver}).RebasePending("", "T-01", "J-6"); err != nil || !pending {
		t.Fatalf("RebasePending = %t, %v; want true after a queued rebase", pending, err)
	}
}
