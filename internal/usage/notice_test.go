package usage

import (
	"encoding/json"
	"errors"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
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
		posts, earlier           int // posts repeats the dollars post (0 is once); earlier counts notices before the last post
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
		{name: "cents-80", budget: Change{Budget: true, Dollars: []float64{1}}, dollars: 0.1, posts: 8, earlier: 1, percentages: []float64{80}, units: []string{"dollars"}},
		{name: "cents-100", budget: Change{Budget: true, Dollars: []float64{1}}, dollars: 0.1, posts: 10, earlier: 2, percentages: []float64{100}, units: []string{"dollars"}},
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
			for i := 1; i < tc.posts; i++ {
				post(t, s, tc.dollars, tc.minutes)
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
			count := len(tc.percentages) + tc.earlier
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
	if err != nil || out.Totals.Dollars != 32 || out.Totals.Minutes != 64 || out.Totals.Posts != 32 || len(out.Jobs) != 1 {
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

func TestTokenConfigurationCompatibility(t *testing.T) {
	s := noticeStore(t)
	// Version 1 settings fixture from plugin main 5dcc524.
	const old = `{"v":1,"dollars":[50,80,100],"minutes":[50,80,100],"budget":{"dollars":200},"baseline":{"dollars":0,"minutes":0},"fired":{}}`
	dir := filepath.Join(s.TaskPath, "usage")
	if err := os.Mkdir(dir, 0700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "thresholds.json")
	if err := os.WriteFile(path, []byte(old), 0600); err != nil {
		t.Fatal(err)
	}
	c, err := s.Config()
	if err != nil || c.Budget["dollars"] != "200" || c.Baseline["tokens"] != "0" || !reflect.DeepEqual(c.Tokens, []float64{80, 100}) || c.TokenBudgetSource != "none" {
		t.Fatalf("old settings: %+v %v", c, err)
	}
	configure(t, s, Change{Budget: true, BudgetTokens: []uint64{math.MaxUint64}})
	c, err = s.Config()
	if err != nil || c.Budget["tokens"] != "18446744073709551615" || c.TokenBudgetSource != "task" {
		t.Fatalf("large budget: %+v %v", c, err)
	}
	for _, bad := range []string{`null`, `"100"`, `-1`, `1.5`, `0`, `18446744073709551616`} {
		if err := os.WriteFile(path, []byte(strings.Replace(old, `"dollars":200`, `"tokens":`+bad, 1)), 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := s.Config(); err == nil {
			t.Fatalf("accepted token budget %s", bad)
		}
	}
}

func TestTokenCrossingsExact(t *testing.T) {
	for _, tc := range []struct {
		base, total, budget uint64
		percent             float64
		want                bool
	}{
		{0, 79, 100, 80, false}, {0, 80, 100, 80, true}, {80, 81, 100, 80, false},
		{0, 2, 3, 80, false}, {0, 3, 3, 80, true},
		{0, 800, 1000, 80.1, false}, {0, 801, 1000, 80.1, true},
		{0, 1, 100, 0, false},
		{0, math.MaxUint64 - 1, math.MaxUint64, 100, false},
		{0, math.MaxUint64, math.MaxUint64, 100, true},
		{0, 7205759403792794, 9007199254740993, 80, false},
		{0, 7205759403792795, 9007199254740993, 80, true},
	} {
		if got := tokenCrossing(tc.base, tc.total, tc.budget, tc.percent); got != tc.want {
			t.Fatalf("%+v: %v", tc, got)
		}
	}
}

func TestTokenNoticeRecoveryAndLifetime(t *testing.T) {
	s := noticeStore(t)
	s.GridHome = t.TempDir()
	path := filepath.Join(s.GridHome, "workspace.md")
	if err := os.WriteFile(path, []byte("---\ntokens_per_task: 100\n---\n"), 0600); err != nil {
		t.Fatal(err)
	}
	configure(t, s, Change{Tokens: []float64{80, 100}})
	broken := s
	broken.beforeSave = func() error { return errors.New("injected save failure") }
	n := uint64(80)
	out, err := broken.Post(Input{Job: "lead", Tokens: &n})
	if err != nil || len(out.Notices) != 1 || len(out.Warnings) != 1 {
		t.Fatalf("%+v %v", out, err)
	}
	n = 0
	out, err = s.Post(Input{Job: "lead", Tokens: &n})
	if err != nil || len(out.Notices) != 0 || len(out.Warnings) != 0 || len(notices(t, s)) != 1 {
		t.Fatalf("retry: %+v %v", out, err)
	}
	c, err := readConfig(filepath.Join(s.TaskPath, "usage"))
	if err != nil || len(c.Budget) != 0 || len(c.Fired) != 1 {
		t.Fatalf("persisted workspace default: %+v %v", c, err)
	}
	configure(t, s, Change{Budget: true, BudgetTokens: []uint64{200}})
	configure(t, s, Change{Budget: true, BudgetTokens: []uint64{}})
	n = 20
	out, err = s.Post(Input{Job: "J-1", Tokens: &n})
	if err != nil || len(out.Notices) != 1 || out.Notices[0].Body["threshold"] != float64(100) {
		t.Fatalf("100%%: %+v %v", out, err)
	}
	n = 1
	out, err = s.Post(Input{Job: "J-2", Tokens: &n})
	if err != nil || out.Totals.Tokens != 101 || len(out.Notices) != 0 {
		t.Fatalf("notice only, no hold: %+v %v", out, err)
	}
}

func TestWorkspaceTokenReader(t *testing.T) {
	for _, value := range []string{"1", "00100", `"100"`, "100 # limit", "18446744073709551615"} {
		t.Run(value, func(t *testing.T) {
			s := testStore(t)
			s.GridHome = t.TempDir()
			path := filepath.Join(s.GridHome, "workspace.md")
			text := "---\ntokens_per_task: " + value + "\n---\n"
			text += strings.Repeat("x", maxWorkspaceBytes-len(text))
			if err := os.WriteFile(path, []byte(text), 0600); err != nil {
				t.Fatal(err)
			}
			if n := s.WorkspaceTokens(); n == nil {
				t.Fatal("valid size-boundary workspace ignored")
			}
			if err := os.WriteFile(path, []byte(text+"x"), 0600); err != nil {
				t.Fatal(err)
			}
			if n := s.WorkspaceTokens(); n != nil {
				t.Fatal("oversized workspace accepted")
			}
		})
	}
}

func TestTokenNoticeLargeCountAndSchema(t *testing.T) {
	s := noticeStore(t)
	configure(t, s, Change{Budget: true, BudgetTokens: []uint64{math.MaxUint64}})
	configure(t, s, Change{Tokens: []float64{100}})
	n := uint64(math.MaxUint64)
	out, err := s.Post(Input{Job: "lead", Tokens: &n})
	if err != nil || len(out.Warnings) != 0 || len(out.Notices) != 1 {
		t.Fatalf("large notice: %+v %v", out, err)
	}
	for _, record := range []feed.Record{out.Notices[0], notices(t, s)[0].Record, notices(t, s)[0].Record} {
		data, err := json.Marshal(record)
		if err != nil || !strings.Contains(string(data), `"tokens":18446744073709551615`) {
			t.Fatalf("rounded token notice: %s %v", data, err)
		}
		for _, bad := range []string{
			strings.Replace(string(data), `"tokens":18446744073709551615`, `"tokens":1.5`, 1),
			strings.Replace(string(data), `"tokens":18446744073709551615`, `"tokens":-1`, 1),
			strings.Replace(string(data), `"tokens":18446744073709551615`, `"tokens":1,"dollars":1`, 1),
			strings.Replace(string(data), `"tokens":18446744073709551615`, `"tokens":1,"minutes":1`, 1),
		} {
			if err := contract.Validate("usage-notice", []byte(bad)); err == nil {
				t.Fatal("accepted invalid token notice")
			}
		}
	}
}

// Older version-1 files omitted tokens; both read paths must use the same default.
func TestTokenNoticeDefaults(t *testing.T) {
	for _, source := range []string{"missing-folder", "missing-file", "legacy-file"} {
		t.Run(source, func(t *testing.T) {
			s := noticeStore(t)
			setNoticeWorkspaceBudget(t, &s)
			if source != "missing-folder" {
				dir := filepath.Join(s.TaskPath, "usage")
				if err := os.Mkdir(dir, 0700); err != nil {
					t.Fatal(err)
				}
				if source == "legacy-file" {
					const old = `{"v":1,"dollars":[50,80,100],"minutes":[50,80,100],"budget":{},"baseline":{"dollars":0,"minutes":0},"fired":{}}`
					if err := os.WriteFile(filepath.Join(dir, "thresholds.json"), []byte(old), 0600); err != nil {
						t.Fatal(err)
					}
				}
			}
			c, err := s.Config()
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(c.Tokens, []float64{80, 100}) {
				t.Errorf("default tokens = %v, want [80 100]", c.Tokens)
			}
			if !reflect.DeepEqual(c.Dollars, []float64{50, 80, 100}) || !reflect.DeepEqual(c.Minutes, []float64{50, 80, 100}) {
				t.Fatal("changed non-token defaults")
			}
			for _, step := range []struct {
				delta uint64
				want  []float64
			}{
				{60, nil}, {25, []float64{80}}, {0, nil}, {15, []float64{100}}, {1, nil},
			} {
				out := postNoticeTokens(t, s, step.delta)
				if got := noticePercentages(out); !reflect.DeepEqual(got, step.want) {
					t.Errorf("total %d: notices %v, want %v", out.Totals.Tokens, got, step.want)
				}
				if _, err := s.Show(); err != nil {
					t.Fatal(err)
				}
				if _, err := s.Config(); err != nil {
					t.Fatal(err)
				}
			}
			if got := len(notices(t, s)); got != 2 {
				t.Errorf("got %d lifetime notices, want 2", got)
			}
		})
	}
}

func setNoticeWorkspaceBudget(t *testing.T, s *Store) {
	t.Helper()
	s.GridHome = t.TempDir()
	if err := os.WriteFile(filepath.Join(s.GridHome, "workspace.md"), []byte("---\ntokens_per_task: 100\n---\n"), 0600); err != nil {
		t.Fatal(err)
	}
}

func postNoticeTokens(t *testing.T, s Store, delta uint64) PostResult {
	t.Helper()
	out, err := s.Post(Input{Job: "lead", Tokens: &delta})
	if err != nil || len(out.Warnings) != 0 {
		t.Fatalf("Post tokens: %v, warnings %v", err, out.Warnings)
	}
	return out
}

func noticePercentages(out PostResult) []float64 {
	var percentages []float64
	for _, record := range out.Notices {
		percentages = append(percentages, record.Body["threshold"].(float64))
	}
	return percentages
}

func TestTokenNoticeExplicitFifty(t *testing.T) {
	s := noticeStore(t)
	configure(t, s, Change{Budget: true, BudgetTokens: []uint64{100}, Dollars: []float64{100}, Minutes: []float64{100}})
	configure(t, s, Change{Tokens: []float64{50, 80, 100}})
	out := postNoticeTokens(t, s, 50)
	if !reflect.DeepEqual(noticePercentages(out), []float64{50}) {
		t.Fatalf("explicit token 50 lost: %+v", out)
	}
	out = post(t, s, 50, 50)
	if !reflect.DeepEqual(noticePercentages(out), []float64{50, 50}) || out.Notices[0].Body["dollars"] != float64(50) || out.Notices[1].Body["minutes"] != float64(50) {
		t.Fatalf("non-token 50 lost: %+v", out)
	}
}

func TestTokenNoticeExistingFilesUnchanged(t *testing.T) {
	s := noticeStore(t)
	setNoticeWorkspaceBudget(t, &s)
	writer, err := s.Writer()
	if err != nil {
		t.Fatal(err)
	}
	fs := feed.Store{TaskDir: s.TaskPath, TaskID: taskID}
	// Simulate old notices surviving a crash before thresholds.json was saved.
	for _, percent := range []float64{50, 80} {
		if _, err := fs.Post(feed.Input{Type: "usage-notice", Body: map[string]any{"task": taskID, "threshold": percent, "tokens": uint64(85)}}, writer); err != nil {
			t.Fatal(err)
		}
	}
	files, err := filepath.Glob(filepath.Join(fs.Dir(), "*.jsonl"))
	if err != nil || len(files) == 0 {
		t.Fatalf("notice files missing: %v", err)
	}
	before := make(map[string]string)
	for _, path := range files {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		before[path] = string(data)
	}
	if out := postNoticeTokens(t, s, 85); len(out.Notices) != 0 {
		t.Fatal("rewrote existing notices")
	}
	if out := postNoticeTokens(t, s, 0); len(out.Notices) != 0 {
		t.Fatal("repeated existing notices")
	}
	if len(notices(t, s)) != 2 {
		t.Fatal("lost or duplicated old notice")
	}
	for path, want := range before {
		data, err := os.ReadFile(path)
		if err != nil || string(data) != want {
			t.Fatal("existing notice file changed")
		}
	}
}

func TestWorkflowNoticeThresholdPrecedence(t *testing.T) {
	for _, source := range []string{"workflow", "task-file", "task-configure", "legacy-file", "task-clear"} {
		t.Run(source, func(t *testing.T) {
			s := noticeStore(t)
			setNoticeWorkspaceBudget(t, &s)
			if source == "task-file" || source == "task-clear" {
				values := []float64{50, 80, 100}
				if source == "task-clear" {
					values = []float64{}
				}
				configure(t, s, Change{Tokens: values})
			}
			if source == "legacy-file" {
				dir := filepath.Join(s.TaskPath, "usage")
				if err := os.Mkdir(dir, 0700); err != nil {
					t.Fatal(err)
				}
				const old = `{"v":1,"dollars":[50,80,100],"minutes":[50,80,100],"budget":{},"baseline":{"dollars":0,"minutes":0},"fired":{}}`
				if err := os.WriteFile(filepath.Join(dir, "thresholds.json"), []byte(old), 0600); err != nil {
					t.Fatal(err)
				}
			}
			s.UsageNoticeThresholds = &NoticeThresholds{Tokens: []float64{90, 100}}
			if source == "task-configure" {
				configure(t, s, Change{Tokens: []float64{50, 80, 100}})
			}
			want := []float64{90, 100}
			if source == "task-file" || source == "task-configure" {
				want = []float64{50, 80, 100}
			} else if source == "task-clear" {
				want = []float64{}
			}
			c, err := s.Config()
			if err != nil || !reflect.DeepEqual(c.Tokens, want) {
				t.Fatalf("effective tokens = %v, want %v: %v", c.Tokens, want, err)
			}
			var got []float64
			for _, delta := range []uint64{50, 35, 5, 10, 0} {
				out := postNoticeTokens(t, s, delta)
				got = append(got, noticePercentages(out)...)
				for _, notice := range out.Notices {
					if notice.Body["tokens"] != json.Number(strconv.FormatUint(out.Totals.Tokens, 10)) || notice.Body["threshold"].(float64) > float64(out.Totals.Tokens) {
						t.Fatalf("invalid crossing: %+v", notice)
					}
				}
			}
			if len(got) != len(want) || len(got) > 0 && !reflect.DeepEqual(got, want) {
				t.Fatalf("posted thresholds = %v, want %v", got, want)
			}
			if len(notices(t, s)) != len(want) {
				t.Fatal("lost or repeated workflow notice")
			}
		})
	}
}

func TestWorkflowNoticeThresholdUnits(t *testing.T) {
	s := noticeStore(t)
	s.UsageNoticeThresholds = &NoticeThresholds{
		Dollars: []float64{100, 90, 90}, Minutes: []float64{},
	}
	c, err := s.Config()
	if err != nil || !reflect.DeepEqual(c.Dollars, []float64{90, 100}) || c.Minutes == nil || len(c.Minutes) != 0 || !reflect.DeepEqual(c.Tokens, []float64{80, 100}) {
		t.Fatalf("per-unit defaults: %+v %v", c, err)
	}
	if _, err := os.Stat(filepath.Join(s.TaskPath, "usage")); !os.IsNotExist(err) {
		t.Fatal("Config created usage data")
	}
	// Neither normalization nor callers may mutate the supplied policy or defaults.
	c.Dollars[0], c.Tokens[0] = 1, 1
	if !reflect.DeepEqual(s.UsageNoticeThresholds.Dollars, []float64{100, 90, 90}) {
		t.Fatal("mutated workflow policy")
	}
	c = configure(t, s, Change{Budget: true, Dollars: []float64{100}, Minutes: []float64{100}, BudgetTokens: []uint64{100}})
	if !reflect.DeepEqual(c.Dollars, []float64{90, 100}) || !reflect.DeepEqual(c.Tokens, []float64{80, 100}) {
		t.Fatal("caller mutated later defaults")
	}
	if out := post(t, s, 50, 50); len(out.Notices) != 0 {
		t.Fatal("ignored per-unit workflow thresholds")
	}
	out := post(t, s, 40, 50)
	if !reflect.DeepEqual(noticePercentages(out), []float64{90}) || out.Notices[0].Body["dollars"] != float64(90) {
		t.Fatalf("workflow dollars/minutes: %+v", out)
	}
}

func TestWorkflowNoticeThresholdInvalid(t *testing.T) {
	for _, values := range [][]float64{{-1}, {101}, {math.NaN()}, {math.Inf(1)}, {1, 2, 3, 4, 5, 6, 7, 8, 9}} {
		for _, unit := range []string{"dollars", "minutes", "tokens"} {
			s := noticeStore(t)
			policy := &NoticeThresholds{}
			switch unit {
			case "dollars":
				policy.Dollars = values
			case "minutes":
				policy.Minutes = values
			case "tokens":
				policy.Tokens = values
			}
			s.UsageNoticeThresholds = policy
			if _, err := s.Config(); err == nil || !strings.Contains(err.Error(), "usage_notice_thresholds "+unit) {
				t.Fatalf("accepted invalid %s workflow policy: %v", unit, err)
			}
			if _, err := s.Configure(Change{Budget: true, BudgetTokens: []uint64{100}}); err == nil {
				t.Fatal("Configure accepted invalid workflow policy")
			}
			n := uint64(85)
			out, err := s.Post(Input{Job: "lead", Tokens: &n})
			if err != nil || len(out.Warnings) != 1 || len(out.Notices) != 0 || out.Totals.Tokens != 85 {
				t.Fatalf("invalid workflow policy lost usage: %+v %v", out, err)
			}
		}
	}
}
