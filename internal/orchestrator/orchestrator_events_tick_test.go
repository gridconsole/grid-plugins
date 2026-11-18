package orchestrator

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/feed"
	"github.com/gridconsole/grid-plugins/internal/testgit"
)

// eventsTickProgress hashes the progress of Job J-1 whose body holds worktree.
func eventsTickProgress(s Store, worktree any) string {
	body := map[string]any{}
	if worktree != nil {
		body["worktree"] = worktree
	}
	snap := Snapshot{Bodies: map[string]map[string]any{"J-1": body}, Records: map[string]feed.Record{}}
	return s.progress(snap, &contract.Job{ID: "J-1"})
}

// eventsTickRepo makes dir/w1 a git repo with one commit.
func eventsTickRepo(t *testing.T) (dir string) {
	t.Helper()
	testgit.Isolate(t)
	dir = t.TempDir()
	w1 := filepath.Join(dir, "w1")
	if err := os.Mkdir(w1, 0o755); err != nil {
		t.Fatal(err)
	}
	testgit.Init(t, w1, "", true)
	return dir
}

func eventsTickPtr(s string) *string { return &s }

// Row 2: the shapes of a Job body's worktree.
func TestEventsTickProgressShapes(t *testing.T) {
	dir := eventsTickRepo(t)
	s := Store{WorktreesDir: dir}
	none := eventsTickProgress(s, nil)
	str := eventsTickProgress(s, "w1")
	list := eventsTickProgress(s, []any{"w1"})
	obj := eventsTickProgress(s, map[string]any{"a": "w1"})
	escape := eventsTickProgress(s, "../w1")
	suffix := eventsTickProgress(s, "w1@x")
	if str == none {
		t.Fatalf("a worktree w1 gave the no-worktree value %s", none)
	}
	if list != str || obj != str {
		t.Errorf("string %s, list %s, map %s: want one value", str, list, obj)
	}
	if escape != none {
		t.Errorf("../w1 gave %s, want the no-worktree value %s", escape, none)
	}
	if suffix == none || suffix == str {
		t.Errorf("w1@x with no folder gave %s, want a value apart from none %s and w1 %s", suffix, none, str)
	}
	testgit.Run(t, filepath.Join(dir, "w1"), "commit", "-q", "--allow-empty", "-m", "two")
	if got := eventsTickProgress(s, "w1"); got == str {
		t.Errorf("a new commit in w1 left the w1 value at %s", got)
	}
	if got := eventsTickProgress(s, "w1@x"); got == suffix {
		t.Errorf("a new commit in w1 left the w1@x value at %s", got)
	}
	if got := eventsTickProgress(s, "../w1"); got != none {
		t.Errorf("a new commit in w1 changed the escaped value to %s, want %s", got, none)
	}
}

// Row 3: a memo returns the first value.
func TestEventsTickProgressMemo(t *testing.T) {
	dir := eventsTickRepo(t)
	s := Store{WorktreesDir: dir, progressMemo: map[string]string{}}
	first := eventsTickProgress(s, "w1")
	if got := s.progressMemo["J-1"]; got != first {
		t.Fatalf("memo holds %q after the first call, want %q", got, first)
	}
	testgit.Run(t, filepath.Join(dir, "w1"), "commit", "-q", "--allow-empty", "-m", "two")
	if second := eventsTickProgress(s, "w1"); second != first {
		t.Errorf("second call gave %s, want the memoised %s", second, first)
	}
}

// Row 4: park refuses without an owner, and reuses a question already posted.
func TestEventsTickPark(t *testing.T) {
	t.Run("no owner", func(t *testing.T) {
		f := setup(t)
		f.job(t, "J-1", nil)
		f.s.Owner = ""
		snap, err := Load(f.s.Feed)
		if err != nil {
			t.Fatal(err)
		}
		before := recordCount(t, f)
		err = f.s.park(snap, snap.State.Jobs["J-1"], &jobMemory{Failure: "F-1", Failures: []string{"F-1"}})
		want := "crash loop for J-1: task owner is required (--owner or GRID_OWNER)"
		if err == nil || err.Error() != want {
			t.Fatalf("err = %v, want %q", err, want)
		}
		if after := recordCount(t, f); after != before {
			t.Errorf("records %d -> %d, want none posted", before, after)
		}
	})
	t.Run("question reused", func(t *testing.T) {
		f := setup(t)
		failure := f.post(t, feed.Input{Type: "job", Job: "J-1", Body: jobBody(t, "J-1")}, f.s.Writer)
		f.post(t, feed.Input{Type: "question", Job: "J-1", Wake: failure.ID + "/1", Body: map[string]any{
			"question": "Three attempts made no progress. How should J-1 continue?", "who": "owner", "blocking": true,
			"options": []string{"Keep parked", "Retry"}, "recommended": "Keep parked", "why": failure.ID,
			"gate": map[string]any{"id": "Q-1", "kind": "person", "default": "Keep parked", "at": contract.FormatTime(f.now.Add(24 * time.Hour))},
		}}, f.s.Writer)
		snap, err := Load(f.s.Feed)
		if err != nil {
			t.Fatal(err)
		}
		questions := func() int {
			r, err := f.s.Feed.Read(feed.Filter{Type: "question"})
			if err != nil {
				t.Fatal(err)
			}
			return len(r.Records)
		}
		before := questions()
		if before != 1 {
			t.Fatalf("%d questions before park, want 1", before)
		}
		if err := f.s.park(snap, snap.State.Jobs["J-1"], &jobMemory{Failure: failure.ID, Failures: []string{failure.ID}}); err != nil {
			t.Fatal(err)
		}
		if after := questions(); after != before {
			t.Errorf("questions %d -> %d, want the posted one reused", before, after)
		}
		if got := jobStatus(t, f, "J-1"); got != contract.Parked {
			t.Errorf("status = %s, want %s", got, contract.Parked)
		}
		if reason := f.state(t).Reason; reason != "needs_person" {
			t.Errorf("reason = %q, want needs_person", reason)
		}
	})
}

// Row 5: a Rejected Job's history record with status rejected is a wake candidate.
func TestEventsTickWakeRejectedHistory(t *testing.T) {
	for _, tc := range []struct {
		status string
		want   *Wake
	}{
		{"rejected", &Wake{Job: "J-1", Event: "h1", N: 1}},
		{"done", nil},
	} {
		t.Run(tc.status, func(t *testing.T) {
			snap := Snapshot{
				State: contract.State{Jobs: map[string]*contract.Job{
					"J-1": {ID: "J-1", Status: contract.Rejected, History: []string{"h1"}},
				}},
				Records: map[string]feed.Record{"h1": {ID: "h1", Body: feed.Body{"status": tc.status}}},
			}
			m := &memory{Jobs: map[string]*jobMemory{}, Delivered: map[string]bool{}}
			got := (Store{Now: func() time.Time { return time.Unix(1700000000, 0) }}).wake(snap, m)
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("wake = %+v, want %+v", got, tc.want)
			}
		})
	}
}

// Row 6: what recoverMemory does with the failures it has counted.
func TestEventsTickRecoverMemory(t *testing.T) {
	one := 1
	crash := feed.Record{ID: "e0", Type: "worker-exit", Body: feed.Body{"reason": "crash"}}
	done := feed.Record{ID: "e1", Type: "worker-exit", Attempt: &one, Body: feed.Body{"reason": "done"}}
	result := feed.Record{ID: "e2", Type: "result", Attempt: &one, Body: feed.Body{}}
	for _, tc := range []struct {
		name         string
		next         feed.Record
		results      []string
		failures     []string
		lastFailure  string
		failuresNone bool
	}{
		{"checkpoint", feed.Record{ID: "e1", Type: "checkpoint", Body: feed.Body{}}, nil, nil, "e0", true},
		{"usage_limit", feed.Record{ID: "e1", Type: "worker-exit", Body: feed.Body{"reason": "usage_limit"}}, nil, nil, "e0", true},
		{"cancelled", feed.Record{ID: "e1", Type: "worker-exit", Body: feed.Body{"reason": "cancelled"}}, nil, nil, "e0", true},
		// A done exit with a result of its attempt is not a failure; one with none is.
		{"done with a result", done, []string{"e2"}, []string{"e0"}, "e0", false},
		{"done with no result", done, nil, []string{"e0", "e1"}, "e1", false},
		{"stale control", feed.Record{ID: "e1", Type: "control", Re: eventsTickPtr("r1"), Body: feed.Body{"status": "stale"}}, nil, []string{"e0", "r1"}, "r1", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			j := &contract.Job{ID: "J-1", History: []string{"e0", "e1"}, Results: tc.results}
			snap := Snapshot{Records: map[string]feed.Record{"e0": crash, "e1": tc.next, "e2": result}}
			m := (Store{}).recoverMemory(snap, j)
			if tc.failuresNone {
				if len(m.Failures) != 0 {
					t.Errorf("Failures = %v, want empty", m.Failures)
				}
			} else if !reflect.DeepEqual(m.Failures, tc.failures) {
				t.Errorf("Failures = %v, want %v", m.Failures, tc.failures)
			}
			if m.Failure != tc.lastFailure {
				t.Errorf("Failure = %q, want %q", m.Failure, tc.lastFailure)
			}
		})
	}
}

// Row 7: a Tick whose Writer holds a Job lease refuses; exclusive refuses a task
// that is a file.
func TestEventsTickRefusals(t *testing.T) {
	t.Run("tick with a job lease", func(t *testing.T) {
		f := setup(t)
		f.s.Writer.Lease = "job"
		out, err := f.s.Tick()
		if !errors.Is(err, feed.ErrStaleLease) {
			t.Fatalf("err = %v, want feed.ErrStaleLease", err)
		}
		if len(out.Rows) != 0 {
			t.Errorf("Rows = %v, want none", out.Rows)
		}
	})
	t.Run("task is a file", func(t *testing.T) {
		file := filepath.Join(t.TempDir(), "task")
		if err := os.WriteFile(file, []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
		called := false
		err := (Store{Feed: feed.Store{TaskDir: file}}).exclusive(func() error { called = true; return nil })
		if err == nil || err.Error() != "task is not a folder" {
			t.Errorf("err = %v, want \"task is not a folder\"", err)
		}
		if called {
			t.Error("fn was called")
		}
	})
}

// Row 8: a body that does not decode into job.Job is left to the schema.
func TestEventsTickCheckAfterMalformed(t *testing.T) {
	in := feed.Input{Type: "job", Job: "J-1", Body: map[string]any{"id": "J-1", "after": 7}}
	if err := checkAfterTargets(Snapshot{}, in); err != nil {
		t.Errorf("err = %v, want nil", err)
	}
}
