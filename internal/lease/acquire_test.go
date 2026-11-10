package lease

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/feed"
	"github.com/gridconsole/grid-plugins/internal/session"
)

const (
	acquireMachine = "01ARZ3NDEKTSV4RRFFQ69G5FAW"
	acquireStart   = "2026-01-02T03:04:05.000Z"
)

func acquireStore(t *testing.T) TaskStore {
	t.Helper()
	return TaskStore{Feed: feed.Store{TaskDir: t.TempDir(), TaskID: "01ARZ3NDEKTSV4RRFFQ69G5FAV"}}
}

func acquireWriter(member string) feed.Writer {
	return feed.Writer{Author: feed.Author{ID: member, Role: "cli"}, Worker: member + "@" + acquireMachine, Lease: "task"}
}

func probeOf(state session.ProcessState) func(session.Live) session.ProcessState {
	return func(session.Live) session.ProcessState { return state }
}

func leaseBytes(t *testing.T, s TaskStore) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(s.Feed.Dir(), "lease", "task.json"))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// P3: the same holder, pid and start reuse the tenure, with the same fence and no write.
func TestAcquireTaskTakesThenReuses(t *testing.T) {
	s := acquireStore(t)
	w := acquireWriter("worker")
	first, err := s.AcquireTask(w, 4242, acquireStart, probeOf(session.Alive))
	if err != nil || first.How != AcquiredTaken || first.Tenure.Fence != 1 {
		t.Fatalf("take: %+v %v", first, err)
	}
	before := leaseBytes(t, s)
	again, err := s.AcquireTask(w, 4242, acquireStart, probeOf(session.Alive))
	if err != nil || again.How != AcquiredReused || again.Tenure.Fence != first.Tenure.Fence {
		t.Fatalf("reuse: %+v %v", again, err)
	}
	if !bytes.Equal(before, leaseBytes(t, s)) {
		t.Fatal("a reused tenure was rewritten")
	}
}

// P3: a holder whose process is provably gone is adopted with a new, higher fence. The
// fence comes from the per-task counter, which Job grants advance, so it is not the old fence plus one.
func TestAcquireTaskAdoptsDeadHolderWithHigherFence(t *testing.T) {
	s := acquireStore(t)
	old := acquireWriter("worker")
	first, err := s.AcquireTask(old, 4242, acquireStart, probeOf(session.Alive))
	if err != nil {
		t.Fatal(err)
	}
	old.Fence = first.Tenure.Fence
	if _, err := s.GrantJob(old, "J-1", "worker@"+acquireMachine, 1); err != nil {
		t.Fatal(err)
	}
	got, err := s.AcquireTask(acquireWriter("worker"), 4343, acquireStart, probeOf(session.Dead))
	if err != nil || got.How != AcquiredAdopted {
		t.Fatalf("adopt: %+v %v", got, err)
	}
	if got.Tenure.Fence <= first.Tenure.Fence+1 {
		t.Fatalf("adopted fence %d: want higher than old fence %d plus the Job grant", got.Tenure.Fence, first.Tenure.Fence)
	}
	if got.From.Fence != first.Tenure.Fence || got.From.PID == nil || *got.From.PID != 4242 {
		t.Fatalf("the replaced tenure should be reported: %+v", got.From)
	}
	if *got.Tenure.PID != 4343 {
		t.Fatalf("new tenure: %+v", got.Tenure)
	}
}

// P3: a live foreign holder, or one on another machine, keeps the lease and the file is not touched.
func TestAcquireTaskLeavesLiveOrRemoteHolderAlone(t *testing.T) {
	for name, tc := range map[string]struct {
		holder string
		probe  session.ProcessState
	}{
		"live on this host":    {"person", session.Alive},
		"unknown on this host": {"person", session.Unknown},
	} {
		t.Run(name, func(t *testing.T) {
			s := acquireStore(t)
			if _, err := s.AcquireTask(acquireWriter(tc.holder), 777, acquireStart, probeOf(session.Alive)); err != nil {
				t.Fatal(err)
			}
			before := leaseBytes(t, s)
			_, err := s.AcquireTask(acquireWriter("worker"), 4242, acquireStart, probeOf(tc.probe))
			if !errors.Is(err, ErrTaskHeld) {
				t.Fatalf("want ErrTaskHeld, got %v", err)
			}
			if !bytes.Equal(before, leaseBytes(t, s)) {
				t.Fatal("a held lease was rewritten")
			}
		})
	}
	t.Run("holder on another machine", func(t *testing.T) {
		s := acquireStore(t)
		other := feed.Writer{Author: feed.Author{ID: "person", Role: "cli"}, Worker: "person@01ARZ3NDEKTSV4RRFFQ69G5FAX", Lease: "task"}
		if _, err := s.TakeTask(other, 777, acquireStart); err != nil {
			t.Fatal(err)
		}
		before := leaseBytes(t, s)
		_, err := s.AcquireTask(acquireWriter("worker"), 4242, acquireStart, probeOf(session.Dead))
		if !errors.Is(err, ErrTaskHeld) {
			t.Fatalf("a holder on another machine cannot be proven gone: %v", err)
		}
		if !bytes.Equal(before, leaseBytes(t, s)) {
			t.Fatal("a held lease was rewritten")
		}
	})
}
