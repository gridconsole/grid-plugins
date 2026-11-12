package projectenv

import (
	"errors"
	"io/fs"
	"reflect"
	"sync"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/brain"
)

func bwEnvOrigin() brain.Origin { return brain.Origin{Kind: "person", ID: "bo"} }

// Eight writers each set one field of one stored entry and keep the rest of old.
// The read of old and the write happen under one brain lock, so no field is lost.
func TestBwEnvEditByMergesUnderTheLock(t *testing.T) {
	s := lnEnvStore(t)
	if err := Save(s, Entry{ID: "dev", Name: "Seed"}); err != nil {
		t.Fatal(err)
	}
	sets := []func(*Entry){
		func(e *Entry) { e.Name = "Builds" },
		func(e *Entry) { e.Kind = "cloud" },
		func(e *Entry) { e.Where = "the office" },
		func(e *Entry) { e.Runs = "app" },
		func(e *Entry) { e.Test = "make test" },
		func(e *Entry) { e.How = "Run in CI" },
		func(e *Entry) { e.From = "template" },
		func(e *Entry) { e.Who = []string{"ann", "bo"} },
	}
	var wg sync.WaitGroup
	errs := make([]error, len(sets))
	for i, set := range sets {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, errs[i] = EditBy(s, "dev", bwEnvOrigin(), func(old Entry, exists bool) (Entry, error) {
				if !exists {
					return old, errors.New("the seeded entry is missing")
				}
				next := old
				set(&next)
				return next, nil
			})
		}()
	}
	wg.Wait()
	for i, err := range errs {
		if err != nil {
			t.Fatalf("writer %d: %v", i, err)
		}
	}
	got, err := Parse("dev", lnEnvRead(t, s, "dev"))
	if err != nil {
		t.Fatal(err)
	}
	want := Entry{ID: "dev", Name: "Builds", Kind: "cloud", Where: "the office", Runs: "app", Test: "make test",
		How: "Run in CI", From: "template", Who: []string{"ann", "bo"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("stored entry = %+v, want all eight values %+v", got, want)
	}
}

// change sees exists false for a missing id and the answer is the stored entry;
// an error from change writes nothing.
func TestBwEnvEditByMissingAndRefused(t *testing.T) {
	s := lnEnvStore(t)
	boom := errors.New("refused by change")
	if _, err := EditBy(s, "dev", bwEnvOrigin(), func(old Entry, exists bool) (Entry, error) {
		if exists || old.ID != "dev" {
			t.Errorf("old = %+v, exists = %v for a missing entry", old, exists)
		}
		return old, boom
	}); !errors.Is(err, boom) {
		t.Fatalf("EditBy error = %v, want the error of change", err)
	}
	if _, err := s.ReadConfig("project", "environments", "dev"); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("a refused change left an entry: %v", err)
	}
	got, err := EditBy(s, "dev", bwEnvOrigin(), func(old Entry, exists bool) (Entry, error) {
		old.Name = "Development"
		return old, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if want := (Entry{ID: "dev", Name: "Development"}); !reflect.DeepEqual(got, want) {
		t.Fatalf("answer = %+v, want %+v", got, want)
	}
	if stored, err := Parse("dev", lnEnvRead(t, s, "dev")); err != nil || !reflect.DeepEqual(stored, got) {
		t.Fatalf("stored = %+v, %v, want the answer %+v", stored, err, got)
	}
	if _, err := EditBy(s, "dev", bwEnvOrigin(), func(old Entry, exists bool) (Entry, error) {
		old.ID = "other"
		return old, nil
	}); err == nil {
		t.Fatal("a change that moves the entry to another id was stored")
	}
}
