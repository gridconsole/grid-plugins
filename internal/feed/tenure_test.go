package feed

import (
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/fslock"
)

// takeOver is the grant of feed-contract.md section 4 as the lease component
// runs it: LOCK_EX on the lease file, the end line with the old file's size,
// then the lease rewritten in place through the locked descriptor, never renamed.
func takeOver(t *testing.T, s Store, old, next Writer) int64 {
	t.Helper()
	lf, err := os.OpenFile(filepath.Join(s.Dir(), "lease", "task.json"), os.O_RDWR, 0)
	if err != nil {
		t.Error(err)
		return -1
	}
	defer lf.Close()
	if err := fslock.LockExclusive(lf); err != nil {
		t.Error(err)
		return -1
	}
	defer fslock.Unlock(lf)
	info, err := os.Stat(filepath.Join(s.Dir(), writerFile(old.Worker, old.Fence)))
	if err != nil {
		t.Error(err)
		return -1
	}
	end, _ := json.Marshal(tenureEnd{File: writerFile(old.Worker, old.Fence), EndOffset: info.Size()})
	if err := os.WriteFile(filepath.Join(s.Dir(), "lease", "takeovers.jsonl"), append(end, '\n'), 0o600); err != nil {
		t.Error(err)
		return -1
	}
	data, _ := json.Marshal(Lease{Lease: "task", Holder: &next.Worker, Fence: next.Fence})
	if err := lf.Truncate(0); err != nil {
		t.Error(err)
		return -1
	}
	if _, err := lf.WriteAt(data, 0); err != nil {
		t.Error(err)
		return -1
	}
	if err := lf.Sync(); err != nil {
		t.Error(err)
	}
	return info.Size()
}

// Writers hold the lease shared across check and write, so a takeover (which
// takes it exclusive) lands between two writes, never inside one: the old file
// ends at the recorded offset, every accepted post is inside it, every refusal
// came after the takeover began, and the new holder's posts all count.
func TestTakeoverRacesWriters(t *testing.T) {
	s := testStore(t)
	a := taskWriter(t, s)
	b := Writer{Author: Author{ID: "other", Role: "member"}, Worker: "other@" + testMachine, Lease: "task", Fence: 2}
	body := contractBody(t, "control", nil)
	const writers, trigger = 8, 60
	var (
		started, accepted, stale, falseReject atomic.Int64
		wg                                    sync.WaitGroup
		paused                                atomic.Bool
		reached, resume                       = make(chan struct{}), make(chan struct{})
		once                                  sync.Once
		endOffset                             atomic.Int64
	)
	for i := 0; i < writers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for n := 0; n < 20000; n++ {
				// flock has no writer preference and the shared holders overlap while
				// they queue on the tenure lock, so a takeover would starve behind
				// writers that never pause. Real writers do: this one starts no new
				// post once the takeover is asking, and the posts in flight finish first.
				if paused.Load() {
					<-resume
				}
				_, err := s.Post(Input{Type: "control", Body: body}, a)
				switch {
				case err == nil:
					if accepted.Add(1) >= trigger {
						once.Do(func() { close(reached) })
					}
				case errors.Is(err, ErrStaleLease):
					stale.Add(1)
					if started.Load() == 0 {
						falseReject.Add(1)
					}
					return
				default:
					t.Error(err)
					return
				}
			}
			t.Error("writer was never refused")
		}()
	}
	<-reached
	paused.Store(true)
	started.Store(1)
	endOffset.Store(takeOver(t, s, a, b))
	close(resume)
	wg.Wait()
	if falseReject.Load() != 0 || stale.Load() != writers {
		t.Fatalf("false rejects=%d stale=%d of %d writers", falseReject.Load(), stale.Load(), writers)
	}
	file := filepath.Join(s.Dir(), writerFile(a.Worker, a.Fence))
	info, err := os.Stat(file)
	if err != nil || info.Size() != endOffset.Load() {
		t.Fatalf("old file is %v bytes, takeover recorded %d: %v", info, endOffset.Load(), err)
	}
	if _, err := s.Post(Input{Type: "control", Body: body}, a); !errors.Is(err, ErrStaleLease) {
		t.Fatalf("post after takeover: %v", err)
	}
	if after, _ := os.Stat(file); after.Size() != endOffset.Load() {
		t.Fatal("a refused post wrote")
	}
	if _, err := s.Handover(Input{}, b); err != nil {
		t.Fatal(err)
	}
	var next sync.WaitGroup
	for i := 0; i < 4; i++ {
		next.Add(1)
		go func() {
			defer next.Done()
			for n := 0; n < 10; n++ {
				if _, err := s.Post(Input{Type: "control", Body: body}, b); err != nil {
					t.Error(err)
					return
				}
			}
		}()
	}
	next.Wait()
	read, err := s.Read(Filter{})
	if err != nil {
		t.Fatal(err)
	}
	counted := map[string]int{}
	for _, e := range read.Records {
		if e.Verdict != "counted" {
			t.Fatalf("%s %s: %s", e.File, e.ID, e.Verdict)
		}
		counted[e.File]++
	}
	// The handover plus every accepted post, and nothing else, is in the old file.
	if counted[filepath.Base(file)] != int(accepted.Load())+1 || counted[writerFile(b.Worker, b.Fence)] != 41 || len(read.Bad) != 0 {
		t.Fatalf("counted %v, accepted %d, bad %v", counted, accepted.Load(), read.Bad)
	}
}

// A writer that dies mid-line leaves a fragment and drops its flock; the next
// writer to open the file ends the fragment, so it stays one bad line and the
// new record is whole and numbered after the last complete one.
func TestCrashMidAppendIsRepaired(t *testing.T) {
	w := testWriter()
	if dir := os.Getenv("GRID_FEED_CRASH_CHILD"); dir != "" {
		s := Store{TaskDir: dir, TaskID: testTask}
		r, err := s.record(message("dies"), w)
		if err != nil {
			t.Fatal(err)
		}
		r.Seq = 2
		line, _ := json.Marshal(r)
		f, err := os.OpenFile(filepath.Join(s.Dir(), writerFile(w.Worker, 0)), os.O_RDWR|os.O_APPEND, 0)
		if err != nil {
			t.Fatal(err)
		}
		if err := fslock.LockExclusive(f); err != nil {
			t.Fatal(err)
		}
		if _, err := writeOnce(f, line[:len(line)/2]); err != nil {
			t.Fatal(err)
		}
		os.Exit(3) // no unlock, no newline: the kernel drops the lock
	}
	s := testStore(t)
	first := mustPost(t, s, message("first"), w)
	cmd := exec.Command(os.Args[0], "-test.run=^TestCrashMidAppendIsRepaired$")
	cmd.Env = append(os.Environ(), "GRID_FEED_CRASH_CHILD="+s.TaskDir)
	var exit *exec.ExitError
	if err := cmd.Run(); !errors.As(err, &exit) || exit.ExitCode() != 3 {
		t.Fatalf("child did not die mid-append: %v", err)
	}
	file := filepath.Join(s.Dir(), writerFile(w.Worker, 0))
	if raw, _ := os.ReadFile(file); strings.HasSuffix(string(raw), "\n") {
		t.Fatal("the crash left no fragment")
	}
	after := mustPost(t, s, message("after"), w)
	if after.Seq != first.Seq+1 {
		t.Fatalf("seq %d after %d", after.Seq, first.Seq)
	}
	raw, _ := os.ReadFile(file)
	lines := strings.Split(strings.TrimSuffix(string(raw), "\n"), "\n")
	if !strings.HasSuffix(string(raw), "\n") || len(lines) != 3 || json.Valid([]byte(lines[1])) {
		t.Fatalf("tail was not repaired: %q", lines)
	}
	read, err := s.Read(Filter{})
	if err != nil || len(read.Records) != 2 || read.Bad[filepath.Base(file)] != 1 || read.Records[1].ID != after.ID {
		t.Fatalf("read: %+v bad=%v %v", read.Records, read.Bad, err)
	}
}
