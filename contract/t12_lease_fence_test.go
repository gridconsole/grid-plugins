package contractdata_test

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/feed"
)

const (
	t12LeasePage     = "docs/lease-and-fence.md"
	t12LeaseTaskID   = "01ARZ3NDEKTSV4RRFFQ69G5FAV"
	t12LeaseMachine  = "01ARZ3NDEKTSV4RRFFQ69G5FAW"
	t12LeaseWorker   = "owner@" + t12LeaseMachine
	t12LeaseStaleTxt = "stale lease, exit 3"
)

func t12LeaseReadPage(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile(t12LeasePage)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// t12LeaseSection returns the lines under one "## " heading, up to the next one.
func t12LeaseSection(page, heading string) []string {
	var out []string
	in := false
	for _, line := range strings.Split(page, "\n") {
		if strings.HasPrefix(line, "## ") {
			in = strings.TrimSpace(strings.TrimPrefix(line, "## ")) == heading
			continue
		}
		if in {
			out = append(out, line)
		}
	}
	return out
}

// t12LeaseRows returns the cells of each table row whose first cell is one word
// in backticks, keyed by that word.
func t12LeaseRows(lines []string) map[string][]string {
	rows := map[string][]string{}
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "|") {
			continue
		}
		cells := strings.Split(strings.Trim(line, "|"), "|")
		for i := range cells {
			cells[i] = strings.TrimSpace(cells[i])
		}
		first := cells[0]
		if len(first) > 2 && strings.HasPrefix(first, "`") && strings.HasSuffix(first, "`") {
			rows[strings.Trim(first, "`")] = cells
		}
	}
	return rows
}

func t12LeaseKeys(m map[string][]string) []string {
	var keys []string
	for k := range m {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return keys
}

// t12LeaseDiff names what is in a but not b, and in b but not a.
func t12LeaseDiff(a, b []string) (onlyA, onlyB []string) {
	for _, x := range a {
		if !slices.Contains(b, x) {
			onlyA = append(onlyA, x)
		}
	}
	for _, x := range b {
		if !slices.Contains(a, x) {
			onlyB = append(onlyB, x)
		}
	}
	slices.Sort(onlyA)
	slices.Sort(onlyB)
	return onlyA, onlyB
}

// t12LeaseFieldDiff compares the page's lease file rows with the schema's properties.
func t12LeaseFieldDiff(page string, props []string) (missing, extra []string) {
	rows := t12LeaseRows(t12LeaseSection(page, "The lease file"))
	return t12LeaseDiff(props, t12LeaseKeys(rows))
}

// t12LeaseRuleDiff compares the page's rule ids with the case ids.
func t12LeaseRuleDiff(page string, caseIDs []string) (noCase, noPage []string) {
	rows := t12LeaseRows(t12LeaseSection(page, "When a write is stale"))
	return t12LeaseDiff(t12LeaseKeys(rows), caseIDs)
}

func t12LeaseSchemaProps(t *testing.T) []string {
	t.Helper()
	raw, err := contract.Schema("lease")
	if err != nil {
		t.Fatal(err)
	}
	var schema struct {
		OneOf []struct {
			Properties map[string]json.RawMessage `json:"properties"`
		} `json:"oneOf"`
	}
	if err := json.Unmarshal(raw, &schema); err != nil {
		t.Fatal(err)
	}
	if len(schema.OneOf) != 2 {
		t.Fatalf("lease schema has %d oneOf variants, want 2 (task and Job)", len(schema.OneOf))
	}
	seen := map[string]bool{}
	var props []string
	for _, v := range schema.OneOf {
		for name := range v.Properties {
			if !seen[name] {
				seen[name] = true
				props = append(props, name)
			}
		}
	}
	slices.Sort(props)
	return props
}

func TestT12LeaseFileRows(t *testing.T) {
	missing, extra := t12LeaseFieldDiff(t12LeaseReadPage(t), t12LeaseSchemaProps(t))
	if len(missing) > 0 {
		t.Errorf("lease.json properties with no row on the page: %v", missing)
	}
	if len(extra) > 0 {
		t.Errorf("page rows for properties neither variant has: %v", extra)
	}
}

// t12LeaseFixture builds a task folder like the question tests' fixtureStore:
// feed/lease/task.json held by owner at fence 1, one Handover. It also writes a
// Job lease J-1 with attempt 1 for the same holder.
func t12LeaseFixture(t *testing.T) (feed.Store, feed.Writer, feed.Writer) {
	t.Helper()
	dir := t.TempDir()
	worker := t12LeaseWorker
	task := feed.Writer{Author: feed.Author{ID: "owner", Role: "owner"}, Worker: worker, Lease: "task", Fence: 1}
	job := task
	job.Lease = "J-1"
	if err := os.MkdirAll(filepath.Join(dir, "feed", "lease"), 0o700); err != nil {
		t.Fatal(err)
	}
	one := 1
	t12LeaseWrite(t, dir, "task", feed.Lease{Lease: "task", Holder: &worker, Fence: 1})
	t12LeaseWrite(t, dir, "J-1", feed.Lease{Lease: "J-1", Holder: &worker, Fence: 1, Attempt: &one})
	if err := os.WriteFile(filepath.Join(dir, "task.md"), []byte("---\nid: "+t12LeaseTaskID+"\n---\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	s := feed.Store{TaskDir: dir, TaskID: t12LeaseTaskID, Now: func() time.Time { return time.Date(2026, 11, 8, 10, 0, 0, 0, time.UTC) }}
	if _, err := s.Handover(feed.Input{Body: map[string]any{}}, task); err != nil {
		t.Fatal(err)
	}
	return s, task, job
}

func t12LeaseWrite(t *testing.T, dir, name string, l feed.Lease) {
	t.Helper()
	data, err := json.Marshal(l)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "feed", "lease", name+".json"), data, 0o600); err != nil {
		t.Fatal(err)
	}
}

type t12LeaseCase struct {
	id string
	// wrong makes the writer or the files wrong in one way and returns the writer
	// and attempt to call WithLease with.
	wrong func(t *testing.T, s feed.Store, task, job feed.Writer) (feed.Writer, *int)
}

func t12LeaseCases() []t12LeaseCase {
	two := 2
	return []t12LeaseCase{
		{"no-lease", func(t *testing.T, s feed.Store, task, job feed.Writer) (feed.Writer, *int) {
			task.Lease = ""
			return task, nil
		}},
		{"fence-zero", func(t *testing.T, s feed.Store, task, job feed.Writer) (feed.Writer, *int) {
			task.Fence = 0
			return task, nil
		}},
		{"unreadable", func(t *testing.T, s feed.Store, task, job feed.Writer) (feed.Writer, *int) {
			if err := os.Remove(filepath.Join(s.Dir(), "lease", "task.json")); err != nil {
				t.Fatal(err)
			}
			return task, nil
		}},
		{"undecodable", func(t *testing.T, s feed.Store, task, job feed.Writer) (feed.Writer, *int) {
			if err := os.WriteFile(filepath.Join(s.Dir(), "lease", "task.json"), []byte("{not json"), 0o600); err != nil {
				t.Fatal(err)
			}
			return task, nil
		}},
		{"holder", func(t *testing.T, s feed.Store, task, job feed.Writer) (feed.Writer, *int) {
			task.Worker = "other@" + t12LeaseMachine
			return task, nil
		}},
		{"lease-name", func(t *testing.T, s feed.Store, task, job feed.Writer) (feed.Writer, *int) {
			w := t12LeaseWorker
			t12LeaseWrite(t, s.TaskDir, "task", feed.Lease{Lease: "J-9", Holder: &w, Fence: 1})
			return task, nil
		}},
		{"fence", func(t *testing.T, s feed.Store, task, job feed.Writer) (feed.Writer, *int) {
			task.Fence = 2
			return task, nil
		}},
		{"gen", func(t *testing.T, s feed.Store, task, job feed.Writer) (feed.Writer, *int) {
			task.Gen = 1
			return task, nil
		}},
		{"attempt", func(t *testing.T, s feed.Store, task, job feed.Writer) (feed.Writer, *int) {
			return job, &two
		}},
		{"ended-tenure", func(t *testing.T, s feed.Store, task, job feed.Writer) (feed.Writer, *int) {
			line := `{"file": ` + strconv.Quote(t12LeaseWorker+".1.jsonl") + `, "end_offset": 0}` + "\n"
			if err := os.WriteFile(filepath.Join(s.Dir(), "lease", "takeovers.jsonl"), []byte(line), 0o600); err != nil {
				t.Fatal(err)
			}
			return task, nil
		}},
	}
}

func TestT12LeaseStaleRules(t *testing.T) {
	cases := t12LeaseCases()
	var ids []string
	for _, c := range cases {
		ids = append(ids, c.id)
	}
	page := t12LeaseReadPage(t)
	noCase, noPage := t12LeaseRuleDiff(page, ids)
	if len(noCase) > 0 {
		t.Errorf("page rule ids with no case: %v", noCase)
	}
	if len(noPage) > 0 {
		t.Errorf("cases with no page rule id: %v", noPage)
	}

	// The unchanged writers pass, for the task lease and for the Job lease.
	one := 1
	s, task, job := t12LeaseFixture(t)
	if err := s.WithLease(task, nil, func() error { return nil }); err != nil {
		t.Errorf("unchanged task writer: %v", err)
	}
	if err := s.WithLease(job, &one, func() error { return nil }); err != nil {
		t.Errorf("unchanged Job writer: %v", err)
	}

	rows := t12LeaseRows(t12LeaseSection(page, "When a write is stale"))
	cite := regexp.MustCompile(`(internal/[a-z/]+/[a-z_]+\.go):([0-9]+)(?:-([0-9]+))?`)
	for _, c := range cases {
		t.Run(c.id, func(t *testing.T) {
			s, task, job := t12LeaseFixture(t)
			w, attempt := c.wrong(t, s, task, job)
			called := false
			err := s.WithLease(w, attempt, func() error { called = true; return nil })
			if !errors.Is(err, feed.ErrStaleLease) {
				t.Errorf("WithLease error = %v, want feed.ErrStaleLease", err)
			}
			if called {
				t.Errorf("the pass ran under a stale tenure")
			}
			cells := rows[c.id]
			if len(cells) != 3 {
				t.Fatalf("page row for %s has %d cells, want Rule | When | Code", c.id, len(cells))
			}
			if cells[2] != t12LeaseStaleTxt {
				t.Errorf("page Code cell = %q, want %q", cells[2], t12LeaseStaleTxt)
			}
			m := cite.FindStringSubmatch(cells[1])
			if m == nil {
				t.Fatalf("page row for %s cites no file:line", c.id)
			}
			last := m[2]
			if m[3] != "" {
				last = m[3]
			}
			end, _ := strconv.Atoi(last)
			src, err := os.ReadFile(filepath.Join("..", filepath.FromSlash(m[1])))
			if err != nil {
				t.Fatalf("cited file: %v", err)
			}
			if n := strings.Count(string(src), "\n"); end > n {
				t.Errorf("%s cites %s:%d, file has %d lines", c.id, m[1], end, n)
			}
		})
	}
}

func TestT12LeaseNegative(t *testing.T) {
	var ids []string
	for _, c := range t12LeaseCases() {
		ids = append(ids, c.id)
	}
	var page strings.Builder
	page.WriteString("## When a write is stale\n\n| Rule | When | Code |\n| --- | --- | --- |\n")
	for _, id := range ids {
		if id == "ended-tenure" {
			continue
		}
		page.WriteString("| `" + id + "` | x | " + t12LeaseStaleTxt + " |\n")
	}
	noCase, noPage := t12LeaseRuleDiff(page.String(), ids)
	if len(noCase) != 0 {
		t.Errorf("noCase = %v, want none", noCase)
	}
	if !slices.Equal(noPage, []string{"ended-tenure"}) {
		t.Errorf("a page missing the ended-tenure row reports %v, want [ended-tenure]", noPage)
	}

	// The lease file rows catch a missing property and a made-up one the same way.
	props := t12LeaseSchemaProps(t)
	synthetic := "## The lease file\n\n| Field | Task | Job |\n| --- | --- | --- |\n| `lease` | a | b |\n| `bogus` | a | b |\n"
	missing, extra := t12LeaseFieldDiff(synthetic, props)
	if !slices.Contains(missing, "pid_start") {
		t.Errorf("missing = %v, want pid_start in it", missing)
	}
	if !slices.Equal(extra, []string{"bogus"}) {
		t.Errorf("extra = %v, want [bogus]", extra)
	}
}
