package session

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gridconsole/grid-plugins/internal/contract"
)

type fixture struct {
	store  Store
	now    time.Time
	record Record
}

func newFixture(t *testing.T, job bool) *fixture {
	t.Helper()
	f := &fixture{now: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}
	f.store = Store{TaskDir: t.TempDir(), Now: func() time.Time { return f.now }}
	r := Record{Key: Key{"session-1", "machine-1"}, PID: 42, PIDStart: "start-1", Transcript: "transcript.jsonl"}
	if job {
		r.Lease, r.Holder, r.Fence, r.Attempt = "J-1", "member@machine-1", 1, 1
	}
	var err error
	f.record, err = f.store.Start(r)
	if err != nil {
		t.Fatal(err)
	}
	return f
}
func (f *fixture) livePath(t *testing.T, r Record) string {
	t.Helper()
	path, err := f.store.livePath(r, false)
	if err != nil {
		t.Fatal(err)
	}
	return path
}
func (f *fixture) observe(t *testing.T, state ProcessState, wake bool) Record {
	t.Helper()
	r, err := f.store.Reconcile(f.record.Key, Observation{Process: state, PID: f.record.PID, PIDStart: f.record.PIDStart, AfterWake: wake})
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func TestHeartbeatAndReadOnlyFold(t *testing.T) {
	f := newFixture(t, true)
	if f.record.State != LivenessUnknown {
		t.Fatal(f.record.State)
	}
	out := f.now.Add(-time.Second)
	r, err := f.store.Reconcile(f.record.Key, Observation{Process: Alive, PID: 42, PIDStart: "start-1", OutputAt: out})
	if err != nil {
		t.Fatal(err)
	}
	live, err := f.store.ReadHeartbeat(r)
	if err != nil || live.BeatAt != f.now || live.Until.Sub(live.BeatAt) != LeaseDuration || live.OutputAt != out {
		t.Fatalf("heartbeat: %+v %v", live, err)
	}
	f.now = f.now.Add(HeartbeatInterval)
	r = f.observe(t, Alive, false)
	live, err = f.store.ReadHeartbeat(r)
	if err != nil || live.BeatAt != f.now || live.OutputAt != out {
		t.Fatalf("renew heartbeat: %+v %v", live, err)
	}
	before, err := os.ReadFile(f.livePath(t, r))
	if err != nil {
		t.Fatal(err)
	}
	f.now = f.now.Add(time.Hour)
	if _, err := f.store.Touch(r.Key, "new-transcript.jsonl"); err != nil {
		t.Fatal(err)
	}
	got, err := f.store.Show(r.Key)
	if err != nil || Fold(got) != Running {
		t.Fatalf("reader inferred death: %+v %v", got, err)
	}
	records, err := f.store.List()
	if err != nil || len(records) != 1 || records[0].State != Running {
		t.Fatalf("list: %+v %v", records, err)
	}
	after, err := os.ReadFile(f.livePath(t, r))
	if err != nil || string(before) != string(after) {
		t.Fatal("hook activity renewed liveness")
	}
	root, err := filepath.EvalSymlinks(f.store.TaskDir)
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(root, "feed", ".live", "member@machine-1.1.json"); f.livePath(t, r) != want {
		t.Fatalf("heartbeat path %s, want %s", f.livePath(t, r), want)
	}
	if _, err := os.Stat(filepath.Join(f.store.TaskDir, ".live")); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("heartbeat wrote outside feed/")
	}
	if entries, err := os.ReadDir(filepath.Join(f.store.TaskDir, "feed")); err != nil || len(entries) != 1 || entries[0].Name() != ".live" {
		t.Fatalf("heartbeat wrote a Feed: %v %v", entries, err)
	}
}

func TestUnknownAndWake(t *testing.T) {
	for _, tc := range []struct {
		name      string
		job, wake bool
		want      State
	}{
		{"job offline after grace", true, false, Stale},
		{"wake needs process evidence", true, true, LivenessUnknown},
		{"terminal needs process evidence", false, false, LivenessUnknown},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newFixture(t, tc.job)
			f.observe(t, Alive, false)
			f.now = f.now.Add(UnknownLimit - time.Nanosecond)
			if r := f.observe(t, Unknown, tc.wake); r.State != LivenessUnknown {
				t.Fatal(r.State)
			}
			f.now = f.now.Add(time.Nanosecond)
			// Wake protection survives the first post-wake tick.
			r := f.observe(t, Unknown, false)
			if r.State != tc.want {
				t.Fatalf("got %s, want %s", r.State, tc.want)
			}
			if tc.want == Stale && !r.HostOffline {
				t.Fatal("missing host offline decision")
			}
			if tc.wake {
				r = f.observe(t, Alive, false)
				if r.State != Running || r.WakePending || r.Until != f.now.Add(LeaseDuration) {
					t.Fatalf("wake: %+v", r)
				}
				if r := f.observe(t, Dead, true); r.State != Stale {
					t.Fatal("proven death after wake was ignored")
				}
			}
		})
	}
}

func TestNeverObservedJobGetsFullGrace(t *testing.T) {
	f := newFixture(t, true)
	f.now = f.now.Add(UnknownLimit - time.Nanosecond)
	if r := f.observe(t, Unknown, false); r.State != LivenessUnknown {
		t.Fatal(r.State)
	}
	f.now = f.now.Add(time.Nanosecond)
	if r := f.observe(t, Unknown, false); r.State != Stale {
		t.Fatal(r.State)
	}
}

func TestSleepResumeAndCleanup(t *testing.T) {
	f := newFixture(t, true)
	f.observe(t, Alive, false)
	f.now = f.now.Add(IdleSleepAfter)
	r, err := f.store.Sleep(f.record.Key)
	if err != nil || r.State != Sleeping || r.Slept == nil {
		t.Fatalf("sleep: %+v %v", r, err)
	}
	f.now = f.now.Add(24 * time.Hour)
	cleaned, err := f.store.Cleanup(func(Record) (Observation, error) { t.Fatal("probed sleeping session"); return Observation{}, nil })
	if err != nil || len(cleaned) != 0 {
		t.Fatalf("cleanup: %v %v", cleaned, err)
	}
	r.PID, r.PIDStart, r.Fence = 43, "start-2", 2
	r.Transcript = ""
	resumed, err := f.store.Start(r)
	if err != nil || resumed.Started != f.record.Started || resumed.Slept != nil || resumed.State != LivenessUnknown || resumed.Transcript != f.record.Transcript {
		t.Fatalf("resume: %+v %v", resumed, err)
	}
	if _, err := f.store.Reconcile(r.Key, Observation{Process: Dead, PID: 42, PIDStart: "start-1"}); err == nil {
		t.Fatal("accepted old process evidence")
	}
	f.record = resumed
	f.observe(t, Alive, false)
	cleaned, err = f.store.Cleanup(func(r Record) (Observation, error) {
		return Observation{Process: Dead, PID: r.PID, PIDStart: r.PIDStart}, nil
	})
	if err != nil || len(cleaned) != 1 {
		t.Fatalf("cleanup: %v %v", cleaned, err)
	}
	r, err = f.store.Show(r.Key)
	if err != nil || r.State != Stale || r.Ended == nil || r.Transcript == "" {
		t.Fatalf("lost history: %+v %v", r, err)
	}
	if _, err := f.store.ReadHeartbeat(r); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("left dead heartbeat: %v", err)
	}
}

func TestCleanupProbeFailureIsUnknown(t *testing.T) {
	f := newFixture(t, true)
	f.observe(t, Alive, false)
	f.now = f.now.Add(LeaseDuration)
	cleaned, err := f.store.Cleanup(func(Record) (Observation, error) { return Observation{}, errors.New("process check unavailable") })
	if err != nil || len(cleaned) != 0 {
		t.Fatalf("cleanup: %v %v", cleaned, err)
	}
	r, err := f.store.Show(f.record.Key)
	if err != nil || r.State != LivenessUnknown || r.Ended != nil {
		t.Fatalf("unknown marked dead: %+v %v", r, err)
	}
	if _, err := f.store.ReadHeartbeat(r); err != nil {
		t.Fatal(err)
	}
}

func TestCleanupMismatchedProbeIsUnknown(t *testing.T) {
	f := newFixture(t, true)
	other := f.record
	other.SessionID, other.Holder, other.Fence, other.PID, other.PIDStart = "session-2", "member+tag@machine-1", 2, 43, "start-2"
	if _, err := f.store.Start(other); err != nil {
		t.Fatal(err)
	}
	// session-1 gets evidence about another pid; session-2 is confirmed dead.
	cleaned, err := f.store.Cleanup(func(r Record) (Observation, error) {
		if r.SessionID == "session-1" {
			return Observation{Process: Dead, PID: 99, PIDStart: "other"}, nil
		}
		return Observation{Process: Dead, PID: r.PID, PIDStart: r.PIDStart}, nil
	})
	if err != nil || len(cleaned) != 1 || cleaned[0].SessionID != "session-2" {
		t.Fatalf("a mismatched probe stopped the sweep: %v %v", cleaned, err)
	}
	r, err := f.store.Show(f.record.Key)
	if err != nil || r.State != LivenessUnknown || r.Ended != nil {
		t.Fatalf("mismatch marked dead: %+v %v", r, err)
	}
}

func TestHolderMayCarryAPlusTag(t *testing.T) {
	f := newFixture(t, true)
	r := f.record
	r.Holder = "name+tag@machine-1"
	if _, err := f.store.Start(r); err != nil {
		t.Fatal(err)
	}
	if _, err := f.store.Reconcile(r.Key, Observation{Process: Alive, PID: r.PID, PIDStart: r.PIDStart}); err != nil {
		t.Fatal(err)
	}
}

func TestMachineIdentityAndEnd(t *testing.T) {
	f := newFixture(t, false)
	other := f.record
	other.MachineID = "machine-2"
	if _, err := f.store.Start(other); err != nil {
		t.Fatal(err)
	}
	records, err := f.store.List()
	if err != nil || len(records) != 2 {
		t.Fatalf("records: %v %v", records, err)
	}
	for _, state := range []State{Finished, Errored, StoppedByYou} {
		if _, err := f.store.Start(f.record); err != nil {
			t.Fatal(err)
		}
		r, err := f.store.End(f.record.Key, state)
		if err != nil || r.Ended == nil || Fold(r) != state {
			t.Fatalf("end: %+v %v", r, err)
		}
	}
}

func TestStorageRefusesTraversalAndSymlinks(t *testing.T) {
	f := newFixture(t, true)
	for _, bad := range []Key{{"../outside", "machine-1"}, {"session-1", ".."}, {"", "machine-1"}} {
		r := f.record
		r.Key = bad
		if _, err := f.store.Start(r); err == nil {
			t.Fatalf("accepted %v", bad)
		}
	}
	for _, target := range []string{"sessions", "feed", "feed/.live"} {
		t.Run(target, func(t *testing.T) {
			f := newFixture(t, true)
			if err := os.RemoveAll(filepath.Join(f.store.TaskDir, target)); err != nil {
				t.Fatal(err)
			}
			outside := t.TempDir()
			if err := os.MkdirAll(filepath.Dir(filepath.Join(f.store.TaskDir, target)), 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(outside, filepath.Join(f.store.TaskDir, target)); err != nil {
				t.Fatal(err)
			}
			var err error
			if target == "sessions" {
				_, err = f.store.Start(f.record)
			} else {
				_, err = f.store.Reconcile(f.record.Key, Observation{Process: Alive, PID: 42, PIDStart: "start-1"})
			}
			if err == nil {
				t.Fatal("followed storage symlink")
			}
			files, err := os.ReadDir(outside)
			if err != nil || len(files) != 0 {
				t.Fatal("wrote outside storage")
			}
		})
	}
}

// macOS keeps /var and /tmp as symlinks, and a Grid home may sit under any
// symlinked parent. Only components below the task folder are refused.
func TestSymlinksAboveTheTaskFolderAreAllowed(t *testing.T) {
	real := t.TempDir()
	if err := os.Mkdir(filepath.Join(real, "task"), 0o700); err != nil {
		t.Fatal(err)
	}
	parent := filepath.Join(t.TempDir(), "home")
	if err := os.Symlink(real, parent); err != nil {
		t.Fatal(err)
	}
	alias := filepath.Join(t.TempDir(), "alias")
	if err := os.Symlink(filepath.Join(real, "task"), alias); err != nil {
		t.Fatal(err)
	}
	for _, dir := range []string{filepath.Join(parent, "task"), alias} {
		s := Store{TaskDir: dir}
		r := Record{Key: Key{"session-1", "machine-1"}, PID: 42, PIDStart: "start-1"}
		if _, err := s.Start(r); err != nil {
			t.Fatalf("%s: %v", dir, err)
		}
		if records, err := s.List(); err != nil || len(records) != 1 {
			t.Fatalf("%s: %v %v", dir, records, err)
		}
		if _, err := s.Show(r.Key); err != nil {
			t.Fatalf("%s: %v", dir, err)
		}
	}
}

func TestHeartbeatTimesMatchSchema(t *testing.T) {
	for _, ms := range []int{0, 120, 10, 123} {
		now := time.Date(2026, 1, 1, 0, 0, 5, ms*int(time.Millisecond), time.UTC)
		store := Store{TaskDir: t.TempDir(), Now: func() time.Time { return now }}
		r, err := store.Start(Record{Key: Key{"session-1", "machine-1"}, PID: 42, PIDStart: "2026-01-01T00:00:00.000Z", Transcript: "t.jsonl",
			Lease: "J-1", Holder: "member@01ARZ3NDEKTSV4RRFFQ69G5FAV", Fence: 1, Attempt: 1})
		if err != nil {
			t.Fatal(err)
		}
		r, err = store.Reconcile(r.Key, Observation{Process: Alive, PID: 42, PIDStart: r.PIDStart})
		if err != nil {
			t.Fatal(err)
		}
		path, err := store.livePath(r, false)
		if err != nil {
			t.Fatal(err)
		}
		b, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if err := contract.Validate("heartbeat", b); err != nil {
			t.Fatalf("ms=%d: %v\n%s", ms, err, b)
		}
		if live, err := store.ReadHeartbeat(r); err != nil || !live.BeatAt.Equal(now) {
			t.Fatalf("ms=%d: read back %v %v", ms, live.BeatAt, err)
		}
	}
}

func TestTerminalResumeCannotDropJobTenure(t *testing.T) {
	f := newFixture(t, true)
	running := f.observe(t, Alive, false)
	before, err := f.store.ReadHeartbeat(running)
	if err != nil {
		t.Fatal(err)
	}
	_, err = f.store.Start(Record{Key: running.Key, Where: "terminal", PID: 43, PIDStart: "start-43"})
	if err == nil {
		t.Fatal("terminal replaced Job session")
	}
	after, err := f.store.Show(running.Key)
	if err != nil || after != running {
		t.Fatalf("session changed: %+v %v", after, err)
	}
	heartbeat, err := f.store.ReadHeartbeat(running)
	if err != nil || heartbeat != before {
		t.Fatalf("heartbeat changed: %+v %v", heartbeat, err)
	}
}
