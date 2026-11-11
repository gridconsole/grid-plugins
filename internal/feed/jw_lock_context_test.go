package feed

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gridconsole/grid-plugins/internal/fslock"
)

// jwLcDeadline is the failure timer of a wait that must never be reached on the
// fixed code; it is never the proof of anything.
const jwLcDeadline = 30 * time.Second

// jwLcFeedBytes is every Feed file's name and bytes, for a before/after compare.
func jwLcFeedBytes(t *testing.T, s Store) map[string]string {
	t.Helper()
	files, err := filepath.Glob(filepath.Join(s.Dir(), "*.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]string{}
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		out[filepath.Base(f)] = string(b)
	}
	return out
}

// jwLcWake is a wake key the record schema accepts: an event id and a number.
func jwLcWake(t *testing.T) string {
	t.Helper()
	event, err := NewID()
	if err != nil {
		t.Fatal(err)
	}
	return event + "/1"
}

// A Post whose Store Context is cancelled while it waits on wake.lock returns
// the context's error and writes nothing; the waiter it leaves behind drops the
// lock once the holder lets go, so the next wake Post is not blocked by it.
func TestJwLcPostCancelledOnWakeLock(t *testing.T) {
	s := testStore(t)
	w := testWriter()
	mustPost(t, s, message("first"), w)
	before := jwLcFeedBytes(t, s)
	if len(before) == 0 {
		t.Fatal("no Feed file written by the first post")
	}
	holder, err := s.wakeLock()
	if err != nil {
		t.Fatal(err)
	}
	released := false
	release := func() {
		if !released {
			released = true
			fslock.Unlock(holder)
			holder.Close()
		}
	}
	defer release()
	wakePath := filepath.Join(s.Dir(), ".cache", "wake.lock")

	blocked := make(chan string, 4)
	abandoned := make(chan string, 4)
	t.Cleanup(fslock.OnBlockForTest(func(p string) {
		select {
		case blocked <- p:
		default:
		}
	}))
	t.Cleanup(fslock.OnAbandonedWaitForTest(func(p string) {
		select {
		case abandoned <- p:
		default:
		}
	}))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	cs := s
	cs.Context = ctx
	in := message("while the wake lock is held")
	in.Wake = jwLcWake(t)
	result := make(chan error, 1)
	go func() {
		_, err := cs.Post(in, w)
		result <- err
	}()
	select {
	case p := <-blocked:
		if p != wakePath {
			t.Fatalf("Post queued on %q, want %q", p, wakePath)
		}
	case err := <-result:
		t.Fatalf("Post returned (%v) while the wake lock was held", err)
	case <-time.After(jwLcDeadline):
		t.Fatal("timed out waiting for Post to queue on wake.lock")
	}
	cancel()
	select {
	case err := <-result:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("cancelled Post returned %v, want context.Canceled", err)
		}
	case <-time.After(jwLcDeadline):
		t.Fatal("timed out waiting for the cancelled Post to return")
	}
	after := jwLcFeedBytes(t, s)
	if len(after) != len(before) {
		t.Fatalf("Feed files changed: before %d, after %d", len(before), len(after))
	}
	for name, b := range before {
		if after[name] != b {
			t.Fatalf("Feed file %s changed by a cancelled Post", name)
		}
	}
	release()
	select {
	case <-abandoned:
	case <-time.After(jwLcDeadline):
		t.Fatal("timed out waiting for the abandoned waiter to drop wake.lock")
	}
	next := message("after the cancel")
	next.Wake = jwLcWake(t)
	r := mustPost(t, s, next, w)
	if r.Seq != 2 {
		t.Fatalf("post after the cancel got seq %d, want 2", r.Seq)
	}
}
