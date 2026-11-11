package contractdata_test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gridconsole/grid-plugins/internal/feed"
	"github.com/gridconsole/grid-plugins/internal/question"
)

const (
	t12PersonTaskID  = "01ARZ3NDEKTSV4RRFFQ69G5FAV"
	t12PersonMachine = "01ARZ3NDEKTSV4RRFFQ69G5FAW"
	t12PersonHeader  = "| Field | Value on a person Job | Why |"
	t12PersonPage    = "docs/person-job.md"
)

// t12PersonStub is the stub JobPort of ask_job_test.go: Post keeps the body.
type t12PersonStub struct {
	mu    sync.Mutex
	posts []map[string]any
}

func (c *t12PersonStub) port() question.JobPort {
	return question.JobPort{
		Post: func(_ feed.Store, _, _ feed.Writer, body map[string]any) (string, error) {
			c.mu.Lock()
			defer c.mu.Unlock()
			c.posts = append(c.posts, body)
			return "J-7", nil
		},
		Close: func(feed.Store, feed.Writer, feed.Writer, string, string) error { return nil },
	}
}

// t12PersonStore makes a Store the way fixtureStore in internal/question does:
// a task folder with task.md, a task lease file and one Handover by the control
// writer. The writer's role is the one given.
func t12PersonStore(t *testing.T, role string) (question.Store, *t12PersonStub) {
	t.Helper()
	dir := t.TempDir()
	now := func() time.Time { return time.Date(2026, 11, 8, 10, 0, 0, 0, time.UTC) }
	control := feed.Writer{Author: feed.Author{ID: "owner", Role: "owner"}, Worker: "owner@" + t12PersonMachine}
	c := control
	c.Lease = "task"
	c.Fence = 1
	if err := os.MkdirAll(filepath.Join(dir, "feed", "lease"), 0700); err != nil {
		t.Fatal(err)
	}
	data, _ := json.Marshal(feed.Lease{Lease: "task", Holder: &control.Worker, Fence: 1})
	if err := os.WriteFile(filepath.Join(dir, "feed", "lease", "task.json"), data, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "task.md"), []byte("---\nid: "+t12PersonTaskID+"\n---\n"), 0600); err != nil {
		t.Fatal(err)
	}
	f := feed.Store{TaskDir: dir, TaskID: t12PersonTaskID, Now: now}
	if _, err := f.Handover(feed.Input{Body: map[string]any{}}, c); err != nil {
		t.Fatal(err)
	}
	w := control
	w.Author.Role = role
	stub := &t12PersonStub{}
	return question.Store{TaskDir: dir, TaskID: t12PersonTaskID, Writer: w, ControlWriter: c, Now: now, Jobs: stub.port()}, stub
}

func t12PersonInput() question.Input {
	return question.Input{Question: "Which format?", Who: "alice", Options: []string{"CSV", "JSON"}, Recommended: "CSV", Default: "CSV",
		Applies: time.Date(2026, 11, 9, 10, 0, 0, 0, time.UTC), Blocking: true, AgeAfter: time.Hour}
}

// t12PersonCaptured asks as the owner and returns the one body the stub kept.
func t12PersonCaptured(t *testing.T) map[string]any {
	t.Helper()
	s, stub := t12PersonStore(t, "owner")
	if _, err := s.Ask(t12PersonInput()); err != nil {
		t.Fatal(err)
	}
	if len(stub.posts) != 1 {
		t.Fatalf("Post called %d times, want 1", len(stub.posts))
	}
	return stub.posts[0]
}

func t12PersonRead(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile(t12PersonPage)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// t12PersonRows returns the rows of the person Job table: the key (backticks
// removed) mapped to the value cell. A key listed twice is an error.
func t12PersonRows(page string) (map[string]string, error) {
	rows := map[string]string{}
	in := false
	for _, line := range strings.Split(page, "\n") {
		line = strings.TrimSpace(line)
		if line == t12PersonHeader {
			in = true
			continue
		}
		if !in {
			continue
		}
		if !strings.HasPrefix(line, "|") {
			break
		}
		cells := strings.Split(strings.Trim(line, "|"), "|")
		if len(cells) != 3 {
			return nil, fmt.Errorf("row %q has %d cells, want 3", line, len(cells))
		}
		key := strings.TrimSpace(cells[0])
		if strings.Trim(key, "-: ") == "" {
			continue // the separator row
		}
		if len(key) < 3 || !strings.HasPrefix(key, "`") || !strings.HasSuffix(key, "`") {
			return nil, fmt.Errorf("key cell %q is not in backticks", key)
		}
		key = strings.Trim(key, "`")
		if _, dup := rows[key]; dup {
			return nil, fmt.Errorf("key %q has two rows", key)
		}
		rows[key] = strings.TrimSpace(cells[1])
	}
	if !in {
		return nil, fmt.Errorf("no table with header %q", t12PersonHeader)
	}
	return rows, nil
}

// t12PersonDiff names the keys of the body with no row and the rows with no key.
func t12PersonDiff(page string, body map[string]any) (missing, extra []string, err error) {
	rows, err := t12PersonRows(page)
	if err != nil {
		return nil, nil, err
	}
	for k := range body {
		if _, ok := rows[k]; !ok {
			missing = append(missing, k)
		}
	}
	for k := range rows {
		if _, ok := body[k]; !ok {
			extra = append(extra, k)
		}
	}
	sort.Strings(missing)
	sort.Strings(extra)
	return missing, extra, nil
}

// t12PersonSection returns the text under the `## heading` up to the next one.
func t12PersonSection(t *testing.T, page, heading string) string {
	t.Helper()
	_, rest, ok := strings.Cut(page, "\n## "+heading+"\n")
	if !ok {
		t.Fatalf("%s has no section %q", t12PersonPage, heading)
	}
	before, _, _ := strings.Cut(rest, "\n## ")
	return before
}

func TestT12PersonFieldRows(t *testing.T) {
	body := t12PersonCaptured(t)
	missing, extra, err := t12PersonDiff(t12PersonRead(t), body)
	if err != nil {
		t.Fatal(err)
	}
	if len(missing) > 0 {
		t.Errorf("body keys with no row: %v", missing)
	}
	if len(extra) > 0 {
		t.Errorf("rows for keys the body lacks: %v", extra)
	}
}

func TestT12PersonFixedValues(t *testing.T) {
	body := t12PersonCaptured(t)
	rows, err := t12PersonRows(t12PersonRead(t))
	if err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"role", "kind", "on_blocked", "worktree"} {
		cell, ok := rows[k]
		if !ok {
			t.Errorf("no row for %q", k)
			continue
		}
		got := strings.ReplaceAll(cell, "`", "")
		if want := fmt.Sprint(body[k]); got != want {
			t.Errorf("%s: page says %q, body has %q", k, got, want)
		}
	}
}

func TestT12PersonAgentAsk(t *testing.T) {
	s, stub := t12PersonStore(t, "agent")
	_, err := s.Ask(t12PersonInput())
	if len(stub.posts) != 0 {
		t.Fatalf("agent ask called Post %d times, want 0", len(stub.posts))
	}
	// The Feed, not this package, refuses the agent's question post itself.
	if err == nil || err.Error() != "role agent may not post person gates" {
		t.Fatalf("agent ask err %v, want the Feed refusal the page names", err)
	}
	sec := t12PersonSection(t, t12PersonRead(t), "Who may ask")
	for _, want := range []string{"`agent`", "`cli`", "no Job", "role agent may not post person gates"} {
		if !strings.Contains(sec, want) {
			t.Errorf("who-may-ask section lacks %q", want)
		}
	}
	// The same section lists the roles that do post a Job, and they do.
	for _, role := range []string{"owner", "member", "lead"} {
		if !strings.Contains(sec, "`"+role+"`") {
			t.Errorf("who-may-ask section lacks %q", role)
		}
		s, stub := t12PersonStore(t, role)
		if _, err := s.Ask(t12PersonInput()); err != nil {
			t.Fatalf("%s ask: %v", role, err)
		}
		if len(stub.posts) != 1 {
			t.Errorf("%s ask called Post %d times, want 1", role, len(stub.posts))
		}
	}
	// A cli writer's question is posted, with no Job behind it.
	cs, cstub := t12PersonStore(t, "cli")
	rec, err := cs.Ask(t12PersonInput())
	if err != nil || len(cstub.posts) != 0 || rec.Job != "" {
		t.Fatalf("cli ask: err %v posts %d job %q", err, len(cstub.posts), rec.Job)
	}
}

func TestT12PersonNegative(t *testing.T) {
	body := t12PersonCaptured(t)
	var b strings.Builder
	b.WriteString(t12PersonHeader + "\n|---|---|---|\n")
	keys := make([]string, 0, len(body))
	for k := range body {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if k != "verify" {
			fmt.Fprintf(&b, "| `%s` | `x` | why |\n", k)
		}
	}
	missing, extra, err := t12PersonDiff(b.String(), body)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(missing, []string{"verify"}) || len(extra) != 0 {
		t.Fatalf("missing %v extra %v, want [verify] and none", missing, extra)
	}
	// A row for a key the body lacks is reported too.
	_, extra, _ = t12PersonDiff(b.String()+"| `bogus` | `x` | why |\n", body)
	if !slices.Equal(extra, []string{"bogus"}) {
		t.Fatalf("extra %v, want [bogus]", extra)
	}
}
