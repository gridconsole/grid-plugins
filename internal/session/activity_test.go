package session

import (
	"os"
	"path/filepath"
	"testing"
)

func TestUsageReplacesRetriesAndDoesNotPost(t *testing.T) {
	s := Store{TaskDir: t.TempDir()}
	for _, n := range []float64{10, 20} {
		if err := s.RecordUsage("session-demo", "tool-1", Usage{Input: n}); err != nil {
			t.Fatal(err)
		}
	}
	a, err := s.Activity("session-demo")
	if err != nil {
		t.Fatal(err)
	}
	if got := FoldUsage([]Activity{a}); got.Input != 20 {
		t.Fatalf("usage: %+v", got)
	}
	if _, err := os.Stat(filepath.Join(s.TaskDir, "feed")); !os.IsNotExist(err) {
		t.Fatal("usage created a Feed")
	}
	if r, err := s.List(); err != nil || len(r) != 0 {
		t.Fatalf("activity leaked into process records: %v %v", r, err)
	}
	if err := s.RecordUsage("../escape", "tool", Usage{}); err == nil {
		t.Fatal("unsafe session accepted")
	}
	if err := s.RecordUsage("session-demo", "tool", Usage{Input: -1}); err == nil {
		t.Fatal("negative usage accepted")
	}
}
