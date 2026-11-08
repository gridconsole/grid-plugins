package orchestrator

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/feed"
)

// Simulate a pre-L55 Feed by changing a previously unique gate on disk.
func legacyGateDuplicate(t *testing.T, f *fixture) {
	t.Helper()
	files, err := filepath.Glob(filepath.Join(f.s.Feed.Dir(), "*.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(file, []byte(strings.ReplaceAll(string(data), `"Q-2"`, `"Q-1"`)), 0o600); err != nil {
			t.Fatal(err)
		}
	}
}

func TestLegacyDuplicateGates(t *testing.T) {
	for _, kinds := range [][2]string{{"person", "timer"}, {"timer", "ci"}, {"person", "person"}, {"timer", "timer"}, {"ci", "ci"}} {
		t.Run(kinds[0]+"-"+kinds[1], func(t *testing.T) {
			f := setup(t)
			f.job(t, "J-1", []any{"gate:Q-1"})
			var first feed.Record
			for i, kind := range kinds {
				gate := map[string]any{"id": []string{"Q-1", "Q-2"}[i], "kind": kind}
				if kind == "timer" {
					at := f.now.Add(-time.Hour)
					if i == 0 {
						at = f.now.Add(time.Hour)
					}
					gate["at"] = contract.FormatTime(at)
				}
				if kind == "ci" {
					gate["check"] = "tests"
				}
				q := f.post(t, feed.Input{Type: "question", Body: map[string]any{"question": "Proceed?", "who": "owner", "gate": gate}}, f.s.Writer)
				if i == 0 {
					first = q
				} else if kind != "timer" {
					f.post(t, feed.Input{Type: "answer", Re: q.ID, Body: map[string]any{"answer": "Proceed"}}, feed.Writer{Author: feed.Author{ID: "owner", Role: "owner"}, Worker: "owner@" + machine})
				}
			}
			legacyGateDuplicate(t, f)
			snap, err := Load(f.s.Feed)
			if err != nil {
				t.Fatal(err)
			}
			for i := range 100 {
				ready, err := Ready(snap, f.now)
				if err != nil || len(ready.Ready) != 0 {
					t.Fatalf("iteration %d: closed strict gate bypassed: %+v %v", i, ready, err)
				}
			}
			for range 3 {
				if out := f.tick(t); out.Assigned != 0 {
					t.Fatalf("assigned through duplicate gate: %+v", out)
				}
				if err := os.Remove(filepath.Join(f.s.Feed.Dir(), ".orchestrator.json")); err != nil {
					t.Fatal(err)
				}
			}
			r, err := f.s.Feed.Read(feed.Filter{Type: "post"})
			if err != nil {
				t.Fatal(err)
			}
			warnings := 0
			for _, e := range r.Records {
				if strings.Contains(stringValue(e.Body["text"]), "Warning: duplicate gate id Q-1") {
					warnings++
				}
			}
			if warnings != 1 {
				t.Fatalf("want one persistent warning, got %d", warnings)
			}
			if kinds[0] == "timer" {
				f.now = f.now.Add(2 * time.Hour)
			} else {
				f.post(t, feed.Input{Type: "answer", Re: first.ID, Body: map[string]any{"answer": "Proceed"}}, feed.Writer{Author: feed.Author{ID: "owner", Role: "owner"}, Worker: "owner@" + machine})
			}
			if out := f.tick(t); out.Assigned != 1 {
				t.Fatalf("strict gate did not open: %+v", out)
			}
		})
	}
}
