package lease

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/feed"
	"github.com/gridconsole/grid-plugins/internal/session"
)

// Row 2: a worker id with no '@' is an error, not a panic, and nothing is written. The cases are a free
// lease and a lease held by someone else, which is where AcquireTask reads the machine off the worker id.
func TestAcquireTaskRefusesWorkerWithoutAt(t *testing.T) {
	for _, tc := range []struct {
		name string
		held bool
	}{{"free lease", false}, {"held by another worker", true}} {
		t.Run(tc.name, func(t *testing.T) {
			s := acquireStore(t)
			var before []byte
			if tc.held {
				if _, err := s.AcquireTask(acquireWriter("other"), 777, acquireStart, probeOf(session.Alive)); err != nil {
					t.Fatal(err)
				}
				before = leaseBytes(t, s)
			}
			w := feed.Writer{Author: feed.Author{ID: "worker", Role: "cli"}, Worker: "worker", Lease: "task"}
			err := func() (err error) {
				defer func() {
					if r := recover(); r != nil {
						err = fmt.Errorf("AcquireTask panicked: %v", r)
					}
				}()
				_, err = s.AcquireTask(w, 4242, acquireStart, probeOf(session.Dead))
				return err
			}()
			if err == nil {
				t.Fatal("a worker id with no @ was accepted")
			}
			if len(err.Error()) >= 8 && err.Error()[:8] == "AcquireT" {
				t.Fatal(err)
			}
			if errors.Is(err, ErrTaskHeld) {
				t.Fatalf("the worker id is wrong, the lease is not held against it: %v", err)
			}
			path := filepath.Join(s.Feed.Dir(), "lease", "task.json")
			if tc.held && !bytes.Equal(before, leaseBytes(t, s)) {
				t.Fatal("the held lease was rewritten")
			}
			if !tc.held {
				if _, statErr := os.Stat(path); !os.IsNotExist(statErr) {
					t.Fatalf("a lease file was created: %v", statErr)
				}
			}
		})
	}
}
