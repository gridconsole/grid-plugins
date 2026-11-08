package lease

import (
	"bytes"
	"errors"
	"github.com/gridconsole/grid-plugins/internal/session"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/feed"
)

func TestTaskAndJobTenures(t *testing.T) {
	s := TaskStore{Feed: feed.Store{TaskDir: t.TempDir(), TaskID: "01ARZ3NDEKTSV4RRFFQ69G5FAV"}}
	w := feed.Writer{Author: feed.Author{ID: "lead", Role: "lead"}, Worker: "lead@01ARZ3NDEKTSV4RRFFQ69G5FAW", Lease: "task"}
	tenure, err := s.TakeTask(w, os.Getpid(), contract.FormatTime(time.Now()))
	if err != nil {
		t.Fatal(err)
	}
	w.Fence = tenure.Fence
	if _, err := s.TakeTask(w, os.Getpid(), tenure.Since); err == nil {
		t.Fatal("replaced held task")
	}
	worker := feed.Writer{Author: feed.Author{ID: "worker", Role: "worker"}, Worker: "worker@01ARZ3NDEKTSV4RRFFQ69G5FAW", Lease: "J-1"}
	first, err := s.GrantJob(w, "J-1", worker.Worker, 1)
	if err != nil {
		t.Fatal(err)
	}
	worker.Fence = first.Fence
	n := 1
	if err := s.Feed.WithLease(worker, &n, func() error { return nil }); err != nil {
		t.Fatal(err)
	}
	second, err := s.GrantJob(w, "J-1", worker.Worker, 2)
	if err != nil {
		t.Fatal(err)
	}
	if second.Fence <= first.Fence {
		t.Fatal("fence did not advance")
	}
	if err := s.Feed.WithLease(worker, &n, func() error { t.Fatal("old tenure admitted"); return nil }); !errors.Is(err, feed.ErrStaleLease) {
		t.Fatal(err)
	}
	wrong := w
	wrong.Fence++
	if _, err := s.GrantJob(wrong, "J-2", worker.Worker, 1); !errors.Is(err, feed.ErrStaleLease) {
		t.Fatal(err)
	}
	if _, err := s.Read("../task"); err == nil {
		t.Fatal("accepted traversal")
	}
}

func TestTaskAdoptionAndRelease(t *testing.T) {
	for _, tc := range []struct {
		name   string
		remote bool
		state  session.ProcessState
		adopt  bool
	}{
		{"dead local", false, session.Dead, true}, {"live local", false, session.Alive, false},
		{"unknown local", false, session.Unknown, false}, {"dead remote", true, session.Dead, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := TaskStore{Feed: feed.Store{TaskDir: t.TempDir(), TaskID: "01ARZ3NDEKTSV4RRFFQ69G5FAV"}}
			w := feed.Writer{Author: feed.Author{ID: "member", Role: "member"}, Worker: "member@01ARZ3NDEKTSV4RRFFQ69G5FAW", Lease: "task", Gen: 2}
			first, err := s.TakeTask(w, 42, contract.FormatTime(time.Now()))
			if err != nil {
				t.Fatal(err)
			}
			next := w
			next.Worker = "other@01ARZ3NDEKTSV4RRFFQ69G5FAW"
			if tc.remote {
				next.Worker = "other@01ARZ3NDEKTSV4RRFFQ69G5FAX"
			}
			called := false
			got, old, err := s.AdoptTask(next, 43, first.Since, func(l session.Live) session.ProcessState {
				called = true
				if l.PID != 42 || l.PIDStart != first.Since || l.Holder != w.Worker {
					t.Fatalf("bad probe: %+v", l)
				}
				return tc.state
			})
			if (err == nil) != tc.adopt {
				t.Fatalf("adopt: %+v %v", got, err)
			}
			if tc.remote && called {
				t.Fatal("probed remote holder")
			}
			current, err := s.Read("task")
			if err != nil {
				t.Fatal(err)
			}
			if !tc.adopt {
				if current.Fence != first.Fence || *current.Holder != w.Worker {
					t.Fatal("refusal changed tenure")
				}
				return
			}
			if old.Fence != first.Fence || got.Fence != first.Fence+1 {
				t.Fatal("wrong adoption fences")
			}
			next.Fence = got.Fence
			for _, bad := range []feed.Writer{w, {Worker: next.Worker, Lease: "task", Gen: 1, Fence: got.Fence}, {Worker: next.Worker, Lease: "task", Gen: 2, Fence: first.Fence}} {
				if err := s.ReleaseTask(bad); err == nil || !strings.Contains(err.Error(), next.Worker) {
					t.Fatalf("bad release: %v", err)
				}
			}
			if err := s.ReleaseTask(next); err != nil {
				t.Fatal(err)
			}
			current, err = s.Read("task")
			if err != nil || current.Holder != nil {
				t.Fatalf("release: %+v %v", current, err)
			}
			log, err := os.ReadFile(filepath.Join(s.Feed.Dir(), "lease", "takeovers.jsonl"))
			if err != nil {
				t.Fatal(err)
			}
			lines := bytes.Split(bytes.TrimSpace(log), []byte{'\n'})
			if len(lines) != 2 {
				t.Fatalf("log: %s", log)
			}
			for _, line := range lines {
				if err := contract.Validate("takeover", line); err != nil {
					t.Fatal(err)
				}
			}
		})
	}
}

func TestTaskConcurrentTake(t *testing.T) {
	s := TaskStore{Feed: feed.Store{TaskDir: t.TempDir(), TaskID: "01ARZ3NDEKTSV4RRFFQ69G5FAV"}}
	w := feed.Writer{Worker: "member@01ARZ3NDEKTSV4RRFFQ69G5FAW", Lease: "task"}
	start := contract.FormatTime(time.Now())
	ready := make(chan struct{})
	results := make(chan error, 2)
	for range 2 {
		go func() { <-ready; _, err := s.TakeTask(w, 42, start); results <- err }()
	}
	close(ready)
	wins := 0
	for range 2 {
		if err := <-results; err == nil {
			wins++
		} else if !strings.Contains(err.Error(), "held by") {
			t.Fatal(err)
		}
	}
	seq, err := os.ReadFile(filepath.Join(s.Feed.Dir(), "lease", "seq"))
	if err != nil || string(seq) != "1\n" || wins != 1 {
		t.Fatalf("wins=%d seq=%q err=%v", wins, seq, err)
	}
}

func TestAdoptionProbesUnderLeaseLock(t *testing.T) {
	s := TaskStore{Feed: feed.Store{TaskDir: t.TempDir(), TaskID: "01ARZ3NDEKTSV4RRFFQ69G5FAV"}}
	w := feed.Writer{Worker: "member@01ARZ3NDEKTSV4RRFFQ69G5FAW", Lease: "task"}
	start := contract.FormatTime(time.Now())
	first, err := s.TakeTask(w, 42, start)
	if err != nil {
		t.Fatal(err)
	}
	w.Fence = first.Fence
	probing, unblock, done := make(chan struct{}), make(chan struct{}), make(chan error, 1)
	go func() {
		_, _, err := s.AdoptTask(w, 43, start, func(session.Live) session.ProcessState { close(probing); <-unblock; return session.Dead })
		done <- err
	}()
	<-probing
	released := make(chan error, 1)
	go func() { released <- s.ReleaseTask(w) }()
	select {
	case err := <-released:
		close(unblock)
		t.Fatalf("release passed probe lock: %v", err)
	case <-time.After(30 * time.Millisecond):
	}
	close(unblock)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if err := <-released; err == nil {
		t.Fatal("stale release succeeded")
	}
}

func TestLeaseReadersRejectTheSameInvalidGrant(t *testing.T) {
	s := TaskStore{Feed: feed.Store{TaskDir: t.TempDir()}}
	path := filepath.Join(s.Feed.Dir(), "lease", "J-1.json")
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile("../../contract/fixtures/lease/valid/alternate.json")
	if err != nil {
		t.Fatal(err)
	}
	for _, data := range [][]byte{nil, bytes.Replace(raw, []byte(`"J-1"`), []byte(`"J-2"`), 1), bytes.Replace(raw, []byte(`"lease": "J-1",`), nil, 1)} {
		if err := os.WriteFile(path, data, 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := s.Read("J-1"); err == nil {
			t.Errorf("TaskStore accepted invalid grant %s", data)
		}
		if _, err := ReadJob(s.Feed.TaskDir, "J-1"); err == nil {
			t.Errorf("ReadJob accepted invalid grant %s", data)
		}
	}
}
