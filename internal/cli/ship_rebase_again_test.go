package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/task"
)

// These tests pin the land of two tasks approved together while a third lands in between
// (pickup-test T-02 and T-04, 18 Nov): the first rebase Job is accepted after main moved
// again, so its branch conflicts too and the second try rebases it.

// landAnother lands one more task's branch on main that adds line to a.txt, after what
// landFirst wrote, so a branch rebased on the main before it conflicts again.
func landAnother(t *testing.T, repo, name, line string) string {
	t.Helper()
	shipGit(t, repo, "checkout", "-q", "-b", name, "main")
	old, err := os.ReadFile(filepath.Join(repo, "a.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "a.txt"), append(old, []byte(line+"\n")...), 0600); err != nil {
		t.Fatal(err)
	}
	shipGit(t, repo, "commit", "-q", "-am", "the "+name+" task's work")
	shipGit(t, repo, "checkout", "-q", "main")
	shipGit(t, repo, "merge", "-q", "--no-ff", "-m", "land the "+name+" task", name)
	return shipGit(t, repo, "rev-parse", "main")
}

// rebaseBuilder is the builder of rebase Job rebase: its branch starts from main and
// merges from, the branch the task delivers now, keeping main's a.txt and adding lane.
func rebaseBuilder(t *testing.T, dir, repo, rebase, from string) string {
	t.Helper()
	fid, _, err := task.FeedIdentity(dir)
	if err != nil {
		t.Fatal(err)
	}
	branch := "grid-" + fid + "-" + rebase + "-a1"
	main, err := os.ReadFile(filepath.Join(repo, "a.txt"))
	if err != nil {
		t.Fatal(err)
	}
	shipGit(t, repo, "checkout", "-q", "-b", branch, "main")
	shipGit(t, repo, "merge", "-q", "-s", "ours", "--no-edit", from)
	if err := os.WriteFile(filepath.Join(repo, "a.txt"), append(main, []byte("lane\n")...), 0600); err != nil {
		t.Fatal(err)
	}
	shipGit(t, repo, "commit", "-q", "-am", "resolve a.txt keeping every change")
	shipGit(t, repo, "checkout", "-q", "main")
	return branch
}

// The first rebase Job is accepted after another task landed: the land of its branch
// conflicts, the second rebase Job is posted with the approval kept, and once that one is
// accepted the task lands with every change on main.
func TestShipConflictAfterMainMovedAgainPostsTheSecondRebaseThenLands(t *testing.T) {
	env, dir, repo, laneSHA, store, stderr := shipMergeFixture(t, false, true)
	landFirst(t, repo)
	ship, _ := openApproveJob(t, store)
	if code := Run([]string{"approve", "T-01"}, env); code != 0 {
		t.Fatalf("approve: %d %s", code, stderr)
	}
	rebases, _ := rebaseJobs(t, store)
	if len(rebases) != 1 {
		t.Fatalf("%d rebase Jobs after the conflict, want 1; stderr %s", len(rebases), stderr)
	}
	first := rebaseBuilder(t, dir, repo, rebases[0].ID, "lane")
	moved := landAnother(t, repo, "second", "second")

	setJobStatus(t, env, store, rebases[0].ID, contract.Accepted, "the conflicts are resolved and the tests pass")
	landAfterShip(env, dir, contract.Author{ID: "local", Role: "owner"})

	if got := shipGit(t, repo, "rev-parse", "main"); got != moved {
		t.Fatalf("main moved to %s on a conflicting land; stderr %s", got, stderr)
	}
	rebases, snap := rebaseJobs(t, store)
	if len(rebases) != 2 {
		t.Fatalf("%d rebase Jobs after the second conflict, want 2; stderr %s; history:\n%s", len(rebases), stderr, shipHistory(t, dir))
	}
	rows := ""
	for _, a := range rebases[1].Acceptance {
		rows += a.Text + "\n"
	}
	if !strings.Contains(rows, first) {
		t.Fatalf("the second rebase Job does not rebase %s, the branch the first made:\n%s", first, rows)
	}
	if st := snap.State.Jobs[ship.ID]; st == nil || st.Status != contract.Accepted {
		t.Fatalf("ship Job %s is %+v after the second conflict, want accepted (the approval stands); stderr %s", ship.ID, st, stderr)
	}
	if rec, err := task.Read(dir); err != nil || rec.State != "waiting" {
		t.Fatalf("task state %q %v while the second rebase waits, want waiting", rec.State, err)
	}

	second := rebaseBuilder(t, dir, repo, rebases[1].ID, first)
	setJobStatus(t, env, store, rebases[1].ID, contract.Accepted, "the conflicts are resolved and the tests pass")
	landAfterShip(env, dir, contract.Author{ID: "local", Role: "owner"})

	if rec, err := task.Read(dir); err != nil || rec.State != "closed" {
		t.Fatalf("task state %q %v after the second rebase was accepted, want closed; stderr %s; history:\n%s", rec.State, err, stderr, shipHistory(t, dir))
	}
	if got := shipGit(t, repo, "show", "main:a.txt"); got != "first\nsecond\nlane" {
		t.Fatalf("main:a.txt is %q, want every task's change", got)
	}
	shipGit(t, repo, "merge-base", "--is-ancestor", laneSHA, "main")
	if b, _, err := taskMergeKeys(dir); err != nil || b != second {
		t.Fatalf("task.md branch %q %v, want the second rebase's %s", b, err, second)
	}
}

// A conflict after both tries puts the ship Job back with its person. Its reason says
// in plain words that main moved and what conflicts, and that approving again retries
// the land: no git steps by hand.
func TestShipConflictAfterTheTriesReopensWithAPlainReason(t *testing.T) {
	env, dir, repo, _, store, stderr := shipMergeFixture(t, false, true)
	landFirst(t, repo)
	ship, _ := openApproveJob(t, store)
	if code := Run([]string{"approve", "T-01"}, env); code != 0 {
		t.Fatalf("approve: %d %s", code, stderr)
	}
	from := "lane"
	for try := 0; try < rebaseTries; try++ {
		rebases, _ := rebaseJobs(t, store)
		if len(rebases) != try+1 {
			t.Fatalf("%d rebase Jobs on try %d, want %d; stderr %s", len(rebases), try+1, try+1, stderr)
		}
		from = rebaseBuilder(t, dir, repo, rebases[try].ID, from)
		landAnother(t, repo, "moved"+string(rune('a'+try)), "moved")
		setJobStatus(t, env, store, rebases[try].ID, contract.Accepted, "resolved")
		landAfterShip(env, dir, contract.Author{ID: "local", Role: "owner"})
	}
	if rebases, _ := rebaseJobs(t, store); len(rebases) != rebaseTries {
		t.Fatalf("%d rebase Jobs, want the %d tries and no more", len(rebases), rebaseTries)
	}
	st, err := store.State()
	if err != nil {
		t.Fatal(err)
	}
	j := st.Jobs[ship.ID]
	if j == nil || j.Status != contract.Posted {
		t.Fatalf("ship Job %s is %+v after the tries, want posted (back with its person); stderr %s", ship.ID, j, stderr)
	}
	for _, want := range []string{"land failed: ", "main moved", "conflicts in a.txt", "approve again"} {
		if !strings.Contains(j.Reason, want) {
			t.Errorf("ship Job reason %q does not say %q", j.Reason, want)
		}
	}
	for _, not := range []string{"by hand", "grid task merge", "aborted"} {
		if strings.Contains(j.Reason, not) {
			t.Errorf("ship Job reason %q still says %q", j.Reason, not)
		}
	}
	if rec, err := task.Read(dir); err != nil || rec.State != "waiting" {
		t.Fatalf("task state %q %v, want waiting", rec.State, err)
	}
}
