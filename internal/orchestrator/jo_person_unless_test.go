package orchestrator

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/feed"
	"github.com/gridconsole/grid-plugins/internal/fslock"
)

func joPjPersonJob(t *testing.T, id string) feed.Input {
	t.Helper()
	body := jobBody(t, id)
	body["role"] = "Person"
	body["intent"] = map[string]any{"owner": "dana", "item": "i-1", "goal": "Choose the name."}
	return feed.Input{Type: "job", Body: body, Job: id}
}

// PostPersonUnless asks done while it holds .orchestrator.lock, and a true answer
// writes nothing; a false answer writes the record as PostPerson does.
func TestJoPjPersonUnlessUnderLock(t *testing.T) {
	f := setup(t)
	lockPath := filepath.Join(f.s.Feed.Dir(), ".orchestrator.lock")
	probe := func(held *error) func() bool {
		return func() bool {
			p, err := fslock.OpenNoFollow(lockPath, os.O_CREATE|os.O_RDWR, 0o600)
			if err != nil {
				t.Errorf("open the lock probe: %v", err)
				return false
			}
			defer p.Close()
			*held = fslock.TryLockExclusive(p)
			if *held == nil {
				fslock.Unlock(p)
			}
			return true
		}
	}

	before := recordCount(t, f)
	var held error
	rec, skipped, err := PostPersonUnless(f.s.Feed, personWriterOf("member"), joPjPersonJob(t, "J-1"), probe(&held))
	if err != nil {
		t.Fatal(err)
	}
	if !errors.Is(held, fslock.ErrLocked) {
		t.Fatalf("the lock was not held while done ran: %v", held)
	}
	if !skipped || rec.ID != "" {
		t.Fatalf("done said taken: want skipped and an empty record, got skipped=%v record=%+v", skipped, rec)
	}
	if got := recordCount(t, f); got != before {
		t.Fatalf("a post that done stopped wrote %d record(s)", got-before)
	}

	rec, skipped, err = PostPersonUnless(f.s.Feed, personWriterOf("member"), joPjPersonJob(t, "J-1"), func() bool { return false })
	if err != nil {
		t.Fatal(err)
	}
	if skipped || rec.ID == "" || rec.Lease != nil || rec.Fence != 0 {
		t.Fatalf("done said free: want an unleased record, got skipped=%v record=%+v", skipped, rec)
	}
	if got := recordCount(t, f); got != before+1 {
		t.Fatalf("a post that done allowed wrote %d record(s), want 1", got-before)
	}

	// A writer that is not a person of the team is refused before done is asked.
	asked := false
	if _, _, err := PostPersonUnless(f.s.Feed, personWriterOf("worker"), joPjPersonJob(t, "J-2"), func() bool { asked = true; return false }); err == nil || asked {
		t.Fatalf("a worker's post: err=%v done asked=%v", err, asked)
	}
}
