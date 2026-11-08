package orchestrator

import (
	"os"
	"path/filepath"
	"reflect"
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

func TestLegacyDuplicateGateReadyOrder(t *testing.T) {
	for _, kind := range []string{"person", "timer"} {
		t.Run(kind, func(t *testing.T) {
			f := setup(t)
			for _, id := range []string{"J-1", "J-2"} {
				body := jobBody(t, id)
				body["created_at"] = contract.FormatTime(f.now.Add(-4 * time.Hour))
				if id == "J-1" {
					body["after"] = []any{"gate:Q-1"}
				} else {
					body["created_at"] = contract.FormatTime(f.now.Add(-2 * time.Hour))
				}
				f.post(t, feed.Input{Type: "job", Job: id, Body: body}, f.s.Writer)
			}
			for i, gid := range []string{"Q-1", "Q-2"} {
				gate := map[string]any{"id": gid, "kind": kind, "at": contract.FormatTime(f.now.Add(time.Duration(-3+2*i) * time.Hour))}
				if kind == "person" {
					gate["default"] = "Proceed"
				}
				f.post(t, feed.Input{Type: "question", Body: map[string]any{"question": "Proceed?", "who": "owner", "gate": gate}}, f.s.Writer)
			}
			legacyGateDuplicate(t, f)
			snap, err := Load(f.s.Feed)
			if err != nil {
				t.Fatal(err)
			}
			for i := range 100 {
				ready, err := Ready(snap, f.now)
				if err != nil || !reflect.DeepEqual(ready.Ready, []string{"J-2", "J-1"}) {
					t.Fatalf("iteration %d: readiness must use latest strict gate time: %+v %v", i, ready, err)
				}
			}
		})
	}
}

func TestLegacyDuplicateStrictKindWins(t *testing.T) {
	for _, kinds := range [][2]string{{"person", "timer"}, {"timer", "ci"}, {"person", "job"}} {
		t.Run(kinds[0]+"-"+kinds[1], func(t *testing.T) {
			f := setup(t)
			f.job(t, "J-1", []any{"gate:Q-1"})
			for i, kind := range kinds {
				gate := map[string]any{"id": []string{"Q-1", "Q-2"}[i], "kind": kind}
				switch kind {
				case "timer", "person":
					at := f.now.Add(-time.Hour)
					if i == 1 {
						at = f.now.Add(time.Hour)
					}
					gate["at"] = contract.FormatTime(at)
					if kind == "person" {
						gate["default"] = "Proceed"
					}
				case "ci":
					gate["check"] = "tests"
				case "job":
					gate["job"], gate["state"] = "J-1", "accepted"
				}
				f.post(t, feed.Input{Type: "question", Body: map[string]any{"question": "Proceed?", "who": "owner", "gate": gate}}, f.s.Writer)
			}
			legacyGateDuplicate(t, f)
			snap, err := Load(f.s.Feed)
			if err != nil {
				t.Fatal(err)
			}
			for i := range 100 {
				ready, err := Ready(snap, f.now)
				if err != nil || !reflect.DeepEqual(ready.Ready, []string{"J-1"}) {
					t.Fatalf("iteration %d: weaker closed gate blocked strict open gate: %+v %v", i, ready, err)
				}
			}
		})
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
