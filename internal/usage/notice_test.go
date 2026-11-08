package usage

import (
	"encoding/json"
	"errors"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/feed"
)

const taskID = "01ARZ3NDEKTSV4RRFFQ69G5FAV"
const machineID = "01ARZ3NDEKTSV4RRFFQ69G5FAW"

func noticeStore(t *testing.T) Store {
	t.Helper()
	s := testStore(t)
	if err := os.WriteFile(filepath.Join(s.TaskPath, "task.md"), []byte("---\nid: "+taskID+"\nstatus: running\n---\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	s.Writer = func() (feed.Writer, error) {
		return feed.Writer{Author: feed.Author{ID: "usage", Role: "cli"}, Worker: "usage@" + machineID}, nil
	}
	return s
}

func configure(t *testing.T, s Store, change Change) Configuration {
	t.Helper()
	c, err := s.Configure(change)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func post(t *testing.T, s Store, dollars, minutes float64) PostResult {
	t.Helper()
	out, err := s.Post(Input{Job: "J-1", Dollars: number(dollars), Minutes: number(minutes)})
	if err != nil || len(out.Warnings) != 0 {
		t.Fatalf("Post: %v, warnings %v", err, out.Warnings)
	}
	return out
}

func notices(t *testing.T, s Store) []feed.Entry {
	t.Helper()
	out, err := (feed.Store{TaskDir: s.TaskPath, TaskID: taskID}).Read(feed.Filter{Type: "usage-notice"})
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range out.Records {
		data, err := json.Marshal(e.Record)
		if err != nil {
			t.Fatal(err)
		}
		if err := contract.Validate("usage-notice", data); err != nil {
			t.Fatal(err)
		}
		if e.Verdict != "counted" || e.Author.Role != "cli" || e.Body["task"] != taskID {
			t.Fatalf("bad notice: %+v", e)
		}
	}
	return out.Records
}

func TestNotices(t *testing.T) {
	for _, tc := range []struct {
		name                     string
		budget, thresholds       Change
		before, dollars, minutes float64
		percentages              []float64
		units                    []string
	}{
		{name: "no-budget", dollars: 1000},
		{name: "no-thresholds", budget: Change{Budget: true, Dollars: []float64{200}}, thresholds: Change{Dollars: []float64{}}, dollars: 1000},
		{name: "below", budget: Change{Budget: true, Dollars: []float64{200}}, dollars: 99},
		{name: "equal", budget: Change{Budget: true, Dollars: []float64{200}}, dollars: 100, percentages: []float64{50}, units: []string{"dollars"}},
		{name: "cross-two", budget: Change{Budget: true, Dollars: []float64{200}}, dollars: 170, percentages: []float64{50, 80}, units: []string{"dollars", "dollars"}},
		{name: "already-fired", budget: Change{Budget: true, Dollars: []float64{200}}, before: 100, dollars: 1},
		{name: "both", budget: Change{Budget: true, Dollars: []float64{200}, Minutes: []float64{120}}, dollars: 100, minutes: 60, percentages: []float64{50, 50}, units: []string{"dollars", "minutes"}},
		{name: "zero-percent", budget: Change{Budget: true, Dollars: []float64{200}}, thresholds: Change{Dollars: []float64{0}}, dollars: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := noticeStore(t)
			if tc.budget.Budget {
				configure(t, s, tc.budget)
			}
			if tc.thresholds.Dollars != nil {
				configure(t, s, tc.thresholds)
			}
			beforeTask, _ := os.ReadFile(filepath.Join(s.TaskPath, "task.md"))
			beforeState, err := (feed.Store{TaskDir: s.TaskPath, TaskID: taskID}).State()
			if err != nil {
				t.Fatal(err)
			}
			if tc.before != 0 {
				post(t, s, tc.before, 0)
			}
			out := post(t, s, tc.dollars, tc.minutes)
			if len(out.Notices) != len(tc.percentages) {
				t.Fatalf("got %d notices: %+v", len(out.Notices), out)
			}
			for i, r := range out.Notices {
				want := out.Totals.Dollars
				if tc.units[i] == "minutes" {
					want = out.Totals.Minutes
				}
				if r.Body["threshold"] != tc.percentages[i] || r.Body[tc.units[i]] != want || len(r.Body) != 3 {
					t.Fatal(r)
				}
			}
			all := notices(t, s)
			count := len(tc.percentages)
			if tc.before > 0 {
				count++
			}
			if len(all) != count {
				t.Fatal(all)
			}
			c, err := s.Config()
			if err != nil || len(c.Fired) != count {
				t.Fatalf("%+v %v", c, err)
			}
			afterTask, _ := os.ReadFile(filepath.Join(s.TaskPath, "task.md"))
			if string(beforeTask) != string(afterTask) {
				t.Fatal("task changed")
			}
			afterState, err := (feed.Store{TaskDir: s.TaskPath, TaskID: taskID}).State()
			// Admission verdicts describe the new records, not Job state.
			afterState.Verdicts = beforeState.Verdicts
			if err != nil || !reflect.DeepEqual(beforeState, afterState) {
				t.Fatalf("notice changed task state: %v", err)
			}
		})
	}
}

func TestNoRetroactiveNotice(t *testing.T) {
	for _, kind := range []string{"budget", "threshold"} {
		t.Run(kind, func(t *testing.T) {
			s := noticeStore(t)
			if kind == "threshold" {
				configure(t, s, Change{Budget: true, Dollars: []float64{200}})
				configure(t, s, Change{Dollars: []float64{100}})
			}
			post(t, s, 110, 0)
			if kind == "threshold" {
				configure(t, s, Change{Dollars: []float64{50, 80, 100}})
			} else {
				configure(t, s, Change{Budget: true, Dollars: []float64{200}})
			}
			if out := post(t, s, 1, 0); len(out.Notices) != 0 {
				t.Fatal(out)
			}
			out := post(t, s, 49, 0)
			if len(out.Notices) != 1 || out.Notices[0].Body["threshold"] != float64(80) {
				t.Fatal(out)
			}
		})
	}
}

func TestNoticeRecovery(t *testing.T) {
	for _, fail := range []string{"after-feed", "feed-unavailable", "writer-unavailable", "agent-writer"} {
		t.Run(fail, func(t *testing.T) {
			s := noticeStore(t)
			configure(t, s, Change{Budget: true, Dollars: []float64{200}})
			broken := s
			switch fail {
			case "after-feed":
				broken.beforeSave = func() error { return errors.New("injected failure after Feed post") }
			case "feed-unavailable":
				if err := os.WriteFile(filepath.Join(s.TaskPath, "feed"), nil, 0o600); err != nil {
					t.Fatal(err)
				}
			case "writer-unavailable":
				broken.Writer = func() (feed.Writer, error) { return feed.Writer{}, errors.New("writer unavailable") }
			case "agent-writer":
				broken.Writer = func() (feed.Writer, error) { w, _ := s.Writer(); w.Author.Role = "worker"; return w, nil }
			}
			out, err := broken.Post(Input{Job: "J-1", Dollars: number(100)})
			if err != nil || len(out.Warnings) != 1 || out.Totals.Dollars != 100 {
				t.Fatalf("%+v %v", out, err)
			}
			if fail == "feed-unavailable" {
				if err := os.Remove(filepath.Join(s.TaskPath, "feed")); err != nil {
					t.Fatal(err)
				}
			}
			if fail == "after-feed" && len(notices(t, s)) != 1 {
				t.Fatal("no Feed record before failure")
			}
			out = post(t, s, 0, 0)
			if fail == "after-feed" && len(out.Notices) != 0 {
				t.Fatal("reposted existing notice")
			}
			if len(notices(t, s)) != 1 {
				t.Fatal("missing or repeated notice")
			}
			c, err := s.Config()
			if err != nil || len(c.Fired) != 1 || out.Totals.Posts != 2 {
				t.Fatalf("%+v %+v %v", c, out, err)
			}
		})
	}
}

func TestConcurrentPosts(t *testing.T) {
	s := noticeStore(t)
	configure(t, s, Change{Budget: true, Dollars: []float64{20}, Minutes: []float64{40}})
	const count = 32
	var wg sync.WaitGroup
	errs := make(chan error, count)
	for i := 0; i < count; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			out, err := s.Post(Input{Job: "lead", Dollars: number(1), Minutes: number(2)})
			if err == nil && len(out.Warnings) > 0 {
				err = errors.New(strings.Join(out.Warnings, ", "))
			}
			errs <- err
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	out, err := s.Show()
	if err != nil || out.Totals != (Totals{Dollars: 32, Minutes: 64, Posts: 32}) || len(out.Jobs) != 1 {
		t.Fatalf("%+v %v", out, err)
	}
	all := notices(t, s)
	if len(all) != 6 {
		t.Fatalf("got %d notices", len(all))
	}
	keys := map[string]bool{}
	for _, r := range all {
		for _, unit := range []string{"dollars", "minutes"} {
			if _, ok := r.Body[unit]; ok {
				key := noticeKey(unit, r.Body["threshold"].(float64))
				if keys[key] {
					t.Fatal("duplicate", key)
				}
				keys[key] = true
			}
		}
	}
}

func TestConfiguration(t *testing.T) {
	s := noticeStore(t)
	c, err := s.Config()
	if err != nil || !reflect.DeepEqual(c.Dollars, []float64{50, 80, 100}) || len(c.Budget) != 0 {
		t.Fatalf("%+v %v", c, err)
	}
	for _, change := range []Change{
		{}, {Dollars: []float64{101}}, {Minutes: []float64{-1}}, {Dollars: []float64{math.NaN()}},
		{Dollars: []float64{1, 2, 3, 4, 5, 6, 7, 8, 9}}, {Budget: true, Dollars: []float64{0}},
		{Budget: true, Minutes: []float64{1, 2}}, {Budget: true, Minutes: []float64{math.Inf(1)}},
	} {
		if _, err := s.Configure(change); err == nil {
			t.Fatalf("accepted %+v", change)
		}
	}
	c = configure(t, s, Change{Dollars: []float64{80, 50, 50, 100}})
	if !reflect.DeepEqual(c.Dollars, []float64{50, 80, 100}) {
		t.Fatal(c)
	}
	configure(t, s, Change{Budget: true, Dollars: []float64{200}})
	post(t, s, 100, 0)
	c = configure(t, s, Change{Dollars: []float64{}})
	if len(c.Dollars) != 0 || len(c.Minutes) != 3 || len(c.Fired) != 1 {
		t.Fatal(c)
	}
	configure(t, s, Change{Dollars: []float64{50}})
	configure(t, s, Change{Budget: true, Dollars: []float64{400}})
	if out := post(t, s, 100, 0); len(out.Notices) != 0 {
		t.Fatal("rearmed fired percentage", out)
	}
	c = configure(t, s, Change{Budget: true, Dollars: []float64{}})
	if len(c.Budget) != 0 {
		t.Fatal(c)
	}
}

func TestInvalidConfigurationDoesNotLoseUsage(t *testing.T) {
	s := noticeStore(t)
	configure(t, s, Change{Budget: true, Dollars: []float64{200}})
	path := filepath.Join(s.TaskPath, "usage", "thresholds.json")
	if err := os.WriteFile(path, []byte("broken"), 0o600); err != nil {
		t.Fatal(err)
	}
	out, err := s.Post(Input{Job: "lead", Minutes: number(2)})
	if err != nil || len(out.Warnings) != 1 || out.Totals.Minutes != 2 {
		t.Fatalf("%+v %v", out, err)
	}
	if _, err := s.Config(); err == nil {
		t.Fatal("accepted broken settings")
	}
	if out, err := s.Show(); err != nil || out.Totals.Posts != 1 {
		t.Fatalf("%+v %v", out, err)
	}
}
