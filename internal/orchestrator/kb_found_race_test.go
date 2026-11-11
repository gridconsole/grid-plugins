package orchestrator

import (
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/feed"
	"github.com/gridconsole/grid-plugins/internal/fslock"
)

// kbFrRounds is how many fresh folders TestKbFrTwentyRounds races.
const kbFrRounds = 20

// kbFrSecondStore is a second Store on the task folder of f, as a second worker
// process holds it: its own feed.Store value, nothing of the first kept, and the
// one task tenure the fixture holds (the same Writer, lease and fence). The
// fixture gives the folder one tenure, so both Stores carry it.
func kbFrSecondStore(f *fixture) Store {
	s := f.s
	s.Feed = feed.Store{TaskDir: f.s.Feed.TaskDir, TaskID: taskID}
	return s
}

// kbFrTickTogether runs Tick on every Store at once and returns what each
// returned. Each goroutine is started and then waits on start, a channel closed
// once, after every goroutine is parked at it: no sleep and no timeout releases
// them. A Tick error is returned, not failed here, so the test goroutine fails.
func kbFrTickTogether(stores ...Store) []error {
	start := make(chan struct{})
	errs := make([]error, len(stores))
	var parked, done sync.WaitGroup
	parked.Add(len(stores))
	done.Add(len(stores))
	for i, s := range stores {
		go func(i int, s Store) {
			defer done.Done()
			parked.Done()
			<-start
			_, errs[i] = s.Tick()
		}(i, s)
	}
	parked.Wait()
	close(start)
	done.Wait()
	return errs
}

// kbFrBlocked counts the lock calls on the orchestrator lock that found it held
// and waited, so a log line says whether a round really made one Tick wait on
// the other. The count is reported and never asserted: whether two goroutines
// overlap is the scheduler's to say, and a proof that needed it would flake.
func kbFrBlocked(t *testing.T) *atomic.Int64 {
	t.Helper()
	var n atomic.Int64
	t.Cleanup(fslock.OnBlockForTest(func(path string) {
		if filepath.Base(path) == ".orchestrator.lock" {
			n.Add(1)
		}
	}))
	return &n
}

// kbFrJobs is what the Feed of f holds: the id of every Job (the state view and
// the id of every job record read, which also counts a record the fold refuses)
// and the id of each orchestrator Job whose acceptance rows name the found id.
func kbFrJobs(t *testing.T, f *fixture, found string) (all, records, naming []string) {
	t.Helper()
	snap, err := Load(f.s.Feed)
	if err != nil {
		t.Fatal(err)
	}
	for _, it := range snap.Items {
		all = append(all, it.ID)
		if !strings.EqualFold(strings.TrimSpace(it.Role), "orchestrator") {
			continue
		}
		for _, a := range it.Acceptance {
			if strings.Contains(a.Text, found) {
				naming = append(naming, it.ID)
				break
			}
		}
	}
	return all, snap.JobIDs, naming
}

// kbFrRace is one round on a fresh task folder: a Job J-1, a security found
// record on it, then two Stores on the folder ticking on one start barrier. It
// fails unless both Ticks answered and the Feed holds one orchestrator Job that
// names the found id, J-2, and no other Job but J-1.
func kbFrRace(t *testing.T, round int) {
	t.Helper()
	f := setup(t)
	f.job(t, "J-1", nil)
	found := kbSfFound(t, f, "J-1", "security", "The token is written to the log in clear.")
	blocked := kbFrBlocked(t)
	for i, err := range kbFrTickTogether(f.s, kbFrSecondStore(f)) {
		if err != nil {
			t.Fatalf("round %d: Tick %d: %v", round, i+1, err)
		}
	}
	all, records, naming := kbFrJobs(t, f, found.ID)
	if want := []string{"J-2"}; !reflect.DeepEqual(naming, want) {
		t.Fatalf("round %d: orchestrator Jobs naming found %s = %v, want %v (Feed holds %v)", round, found.ID, naming, want, all)
	}
	if want := []string{"J-1", "J-2"}; !reflect.DeepEqual(all, want) {
		t.Fatalf("round %d: Jobs on the Feed = %v, want %v", round, all, want)
	}
	if want := []string{"J-1", "J-2"}; !reflect.DeepEqual(records, want) {
		t.Fatalf("round %d: job records on the Feed = %v, want %v", round, records, want)
	}
	t.Logf("round %d: one Job %s for found %s; a Tick found the orchestrator lock held and waited %d time(s)", round, naming[0], found.ID, blocked.Load())
}

func TestKbFrTwoTicksOneJob(t *testing.T) {
	kbFrRace(t, 1)
}

func TestKbFrTwentyRounds(t *testing.T) {
	for round := 1; round <= kbFrRounds; round++ {
		kbFrRace(t, round)
	}
}
