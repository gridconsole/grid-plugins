package routine

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gridconsole/grid-plugins/internal/gridhome"
)

type executorFunc func(context.Context, Store, Entry) error

func (f executorFunc) Execute(c context.Context, s Store, e Entry) error { return f(c, s, e) }

func TestStoreAndRun(t *testing.T) {
	for _, project := range []string{"", "example"} {
		t.Run("scope_"+project, func(t *testing.T) {
			now := time.Date(2026, 1, 2, 3, 4, 0, 0, time.UTC)
			s := Store{GridHome: gridhome.Real(t.TempDir()), Project: project, Now: func() time.Time { return now }}
			e, err := s.Add("daily", "24h", "grid version")
			if err != nil {
				t.Fatal(err)
			}
			if e.Machine != gridhome.Machine(s.GridHome) {
				t.Fatal("owner not persisted")
			}
			if _, err = s.Add("daily", "1h", "grid version"); err == nil {
				t.Fatal("overwrote entry")
			}
			var calls atomic.Int32
			runner := executorFunc(func(context.Context, Store, Entry) error { calls.Add(1); return nil })
			for i := 0; i < 2; i++ {
				r, err := s.Execute(context.Background(), "daily", true, runner, nil)
				if err != nil || r.Skipped != (i == 1) {
					t.Fatalf("run: %+v %v", r, err)
				}
			}
			if calls.Load() != 1 {
				t.Fatal("guard did not suppress duplicate")
			}
			r, err := s.Execute(context.Background(), "daily", false, runner, nil)
			if err != nil || r.Skipped || calls.Load() != 2 {
				t.Fatalf("manual run: %+v %v", r, err)
			}
			now = now.Add(24 * time.Hour)
			failure := errors.New("example failure")
			signals := 0
			_, err = s.Execute(context.Background(), "daily", true, executorFunc(func(context.Context, Store, Entry) error { return failure }), func(Entry, error) error { signals++; return nil })
			if !errors.Is(err, failure) || signals != 1 {
				t.Fatalf("failure: %v %d", err, signals)
			}
			e, err = s.Get("daily")
			if err != nil || e.Outcome != "failed" || !e.LastRun.Equal(now) {
				t.Fatalf("persisted: %+v %v", e, err)
			}
			if err = s.Remove("daily"); err != nil {
				t.Fatal(err)
			}
			entries, err := s.List()
			if err != nil || len(entries) != 0 {
				t.Fatalf("remove: %v %v", entries, err)
			}
		})
	}
}

func TestConcurrentDueRuns(t *testing.T) {
	s := Store{GridHome: gridhome.Real(t.TempDir())}
	if _, err := s.Add("daily", "24h", "grid version"); err != nil {
		t.Fatal(err)
	}
	var calls atomic.Int32
	runner := executorFunc(func(context.Context, Store, Entry) error { calls.Add(1); return nil })
	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			if _, err := s.Execute(context.Background(), "daily", true, runner, nil); err != nil {
				t.Error(err)
			}
		})
	}
	wg.Wait()
	if calls.Load() != 1 {
		t.Fatalf("executed %d times", calls.Load())
	}
}

func TestIntervals(t *testing.T) {
	now := time.Date(2026, 11, 8, 12, 30, 0, 0, time.UTC) // Sunday
	for _, tc := range []struct {
		text       string
		valid, due bool
	}{
		{"1h", true, true}, {"1m30s", true, true}, {"0s", false, false}, {"30s", false, false}, {"-1h", false, false},
		{"30 12 * * 0", true, true}, {"*/15 * * * *", true, true}, {"0 12 * * *", true, false},
		{"30 12 1 * 7", true, true}, {"0-59/30 12 * 1,11 *", true, true}, {"60 * * * *", false, false},
		{"* * 0 * *", false, false}, {"* * * 13 *", false, false}, {"*/0 * * * *", false, false},
		{"* * * * 8", false, false}, {"@daily", false, false}, {"* * * *", false, false},
	} {
		t.Run(tc.text, func(t *testing.T) {
			s, err := ParseInterval(tc.text)
			if (err == nil) != tc.valid {
				t.Fatalf("parse: %v", err)
			}
			if err == nil {
				if s.Due(now, time.Time{}) != tc.due {
					t.Fatal("wrong due")
				}
				if s.Due(now, now) {
					t.Fatal("same instant is due")
				}
			}
		})
	}
}

func TestRunSyntax(t *testing.T) {
	for _, tc := range []struct {
		text  string
		valid bool
		argc  int
	}{
		{`grid signal add --title "A message"`, true, 5}, {"brain index", true, 3}, {"signals", true, 2},
		{"grid:signals", true, 2}, {"", false, 0}, {"grid", false, 0}, {"grid routine run daily", false, 0},
		{"grid version; echo bad", false, 0}, {"grid version | other", false, 0}, {"grid version '", false, 0},
	} {
		t.Run(tc.text, func(t *testing.T) {
			args, err := ParseRun(tc.text)
			if (err == nil) != tc.valid || err == nil && len(args) != tc.argc {
				t.Fatalf("%v %v", args, err)
			}
		})
	}
}
