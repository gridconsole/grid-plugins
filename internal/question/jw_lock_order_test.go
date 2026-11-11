package question

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/gridconsole/grid-plugins/internal/feed"
	"github.com/gridconsole/grid-plugins/internal/fslock"
)

// jwLoPort is a JobPort that checks on every call that questions/.lock is free:
// it tries the lock on a new descriptor without waiting and lets it go at once.
// A call that finds the lock held is recorded and then carries on, so every verb
// of a run is checked.
type jwLoPort struct {
	mu     sync.Mutex
	dir    string
	calls  int
	locked []string
	jobs   int
}

func (p *jwLoPort) probe(call string) error {
	f, err := fslock.OpenNoFollow(filepath.Join(p.dir, "questions", ".lock"), os.O_RDWR, 0)
	if err != nil {
		return err
	}
	defer f.Close()
	err = fslock.TryLockExclusive(f)
	p.mu.Lock()
	defer p.mu.Unlock()
	p.calls++
	if errors.Is(err, fslock.ErrLocked) {
		p.locked = append(p.locked, call)
		return nil
	}
	if err != nil {
		return err
	}
	fslock.Unlock(f)
	return nil
}

func (p *jwLoPort) port() JobPort {
	return JobPort{
		Post: func(feed.Store, feed.Writer, feed.Writer, map[string]any) (string, error) {
			if err := p.probe("Post"); err != nil {
				return "", err
			}
			p.mu.Lock()
			defer p.mu.Unlock()
			p.jobs++
			return fmt.Sprintf("J-%d", p.jobs), nil
		},
		Close: func(_ feed.Store, _, _ feed.Writer, id, _ string) error { return p.probe("Close " + id) },
	}
}

func (p *jwLoPort) seen() (int, int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.calls, len(p.locked)
}

// No Job port call runs while questions/.lock is held: the port's Post and Close
// reach .orchestrator.lock, and a holder pass that holds that lock waits for the
// questions lock in ReserveGate.
func TestJwLoJobPortRunsWithQuestionsLockFree(t *testing.T) {
	s, now := setup(t)
	p := &jwLoPort{dir: s.TaskDir}
	s.Jobs = p.port()
	ask := func() error { _, err := s.Ask(input()); return err }
	for _, step := range []struct {
		verb string
		prep func() error
		run  func() error
	}{
		{"ask", nil, ask},
		{"answer", nil, func() error { _, err := s.Answer("Q-1", "JSON"); return err }},
		{"repeated answer", nil, func() error { _, err := s.Answer("Q-1", "JSON"); return err }},
		{"withdraw", ask, func() error { _, err := s.Withdraw("Q-2", "Not needed"); return err }},
		{"tick default", func() error {
			if err := ask(); err != nil {
				return err
			}
			*now = start.Add(24 * time.Hour)
			return nil
		}, func() error {
			changed, err := s.Tick()
			if err == nil && (len(changed) != 1 || changed[0].ID != "Q-3" || changed[0].State != "defaulted") {
				err = fmt.Errorf("tick changed %+v, want Q-3 defaulted", changed)
			}
			return err
		}},
	} {
		if step.prep != nil {
			if err := step.prep(); err != nil {
				t.Fatalf("%s: %v", step.verb, err)
			}
		}
		calls, locked := p.seen()
		if err := step.run(); err != nil {
			t.Fatalf("%s: %v", step.verb, err)
		}
		c, l := p.seen()
		if c-calls != 1 {
			t.Errorf("%s: the Job port was called %d times, want once", step.verb, c-calls)
		}
		if l-locked != 0 {
			t.Errorf("%s: the Job port ran while questions/.lock was held", step.verb)
		}
	}
}
