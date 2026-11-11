package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/feed"
	"github.com/gridconsole/grid-plugins/internal/orchestrator"
	"github.com/gridconsole/grid-plugins/internal/question"
)

const jwRpAskTitle = "Q-1: Which export format?"

// jwRpHolder is the task lease holder orchestratorHarness made, read from the
// epoch it exported, so a test can post straight through the Store as that holder.
func jwRpHolder(t *testing.T) feed.Writer {
	t.Helper()
	gen, fence, ok := strings.Cut(os.Getenv("GRID_LEASE_EPOCH"), ".")
	if !ok {
		t.Fatalf("GRID_LEASE_EPOCH %q is not gen.fence", os.Getenv("GRID_LEASE_EPOCH"))
	}
	g, err := strconv.ParseInt(gen, 10, 64)
	if err != nil {
		t.Fatal(err)
	}
	f, err := strconv.ParseInt(fence, 10, 64)
	if err != nil {
		t.Fatal(err)
	}
	return feed.Writer{Author: feed.Author{ID: "member", Role: "member"}, Worker: "member@" + feedMachine, Lease: "task", Gen: g, Fence: f}
}

// jwRpPlant posts a job record for id straight through feed.Store.Post, as another
// writer that picked the same id would.
func jwRpPlant(t *testing.T, f feed.Store, w feed.Writer, id string) {
	t.Helper()
	var body map[string]any
	if err := json.Unmarshal([]byte(personBody(t, id, nil)), &body); err != nil {
		t.Error(err)
		return
	}
	body["title"] = "Planted " + id
	if _, err := f.Post(feed.Input{Type: "job", Job: id, Body: body}, w); err != nil {
		t.Errorf("plant %s: %v", id, err)
	}
}

// jwRpHook sets the after-pick hook for one test: it records every picked id and
// plants a job record under the first plants picks. It must run before the test
// switches to the person or the holder environment, which clears the epoch.
func jwRpHook(t *testing.T, plants int) *[]string {
	t.Helper()
	w := jwRpHolder(t)
	picks := new([]string)
	jwRpAfterPickForTest = func(f feed.Store, id string) {
		*picks = append(*picks, id)
		if len(*picks) <= plants {
			jwRpPlant(t, f, w, id)
		}
	}
	t.Cleanup(func() { jwRpAfterPickForTest = nil })
	return picks
}

// jwRpAsk runs `grid question ask --json` as a person of the team, or as the task
// lease holder when lease is set, and returns the exit code and the question record.
func jwRpAsk(t *testing.T, h *harness, task string, lease bool) (int, question.Record) {
	t.Helper()
	if lease {
		asHolder(t, os.Getenv("GRID_LEASE_EPOCH"))
		t.Setenv("GRID_LEASE", "task")
	} else {
		asPerson(t, "member")
	}
	h.out.Reset()
	h.errb.Reset()
	code := h.run(task, "question", "ask", "Which export format?", "--who", "dana", "--option", "csv", "--option", "json",
		"--recommended", "csv", "--default", "csv", "--applies", "2099-01-01T00:00:00Z", "--json")
	var r question.Record
	if code == 0 {
		if err := json.Unmarshal(h.out.Bytes(), &r); err != nil {
			t.Fatal(err, h.out.String())
		}
	}
	return code, r
}

// jwRpJobTitles is every job record on the Feed by Job id, with its title.
func jwRpJobTitles(t *testing.T, store feed.Store) map[string]string {
	t.Helper()
	read, err := store.Read(feed.Filter{Type: "job"})
	if err != nil {
		t.Fatal(err)
	}
	titles := map[string]string{}
	for _, e := range read.Records {
		id, _ := e.Body["id"].(string)
		title, _ := e.Body["title"].(string)
		if _, dup := titles[id]; dup {
			t.Errorf("two job records hold %s", id)
		}
		titles[id] = title
	}
	return titles
}

// jwRpQuestionFiles lists questions/ as the ask left it, without .lock: the
// question store creates that lock file on any ask, and it holds no question.
func jwRpQuestionFiles(t *testing.T, task string) []string {
	t.Helper()
	entries, err := os.ReadDir(filepath.Join(task, "questions"))
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, e := range entries {
		if e.Name() != ".lock" {
			names = append(names, e.Name())
		}
	}
	return names
}

// jwRpCheckRepick holds what an ask that lost its first pick to a planted record
// must leave: the planted record, the question's own Job under the next id, the
// question naming that Job, and the file that shows it.
func jwRpCheckRepick(t *testing.T, h *harness, task string, store feed.Store, code int, q question.Record, picks []string) {
	t.Helper()
	if code != 0 {
		t.Fatalf("ask exited %d: %s", code, h.errb)
	}
	if q.Job != "J-3" || q.ID != "Q-1" {
		t.Fatalf("ask: %+v, want Q-1 on J-3", q)
	}
	if want := []string{"J-2", "J-3"}; !slices.Equal(picks, want) {
		t.Fatalf("picks %v, want %v: the second pick must read the planted record", picks, want)
	}
	titles := jwRpJobTitles(t, store)
	if len(titles) != 3 || titles["J-2"] != "Planted J-2" || titles["J-3"] != jwRpAskTitle {
		t.Fatalf("job records %v, want J-1, the planted J-2 and the question's J-3", titles)
	}
	posts, err := store.Read(feed.Filter{Type: "question"})
	if err != nil || len(posts.Records) != 1 || posts.Records[0].Body["job"] != "J-3" {
		t.Fatalf("question post: %v %+v", err, posts.Records)
	}
	snap, err := orchestrator.Load(store)
	if err != nil {
		t.Fatal(err)
	}
	if j := snap.State.Jobs["J-3"]; j == nil || j.Title != jwRpAskTitle || j.Role != "Person" {
		t.Fatalf("J-3 in the fold: %+v", j)
	}
	if j := snap.State.Jobs["J-2"]; j == nil || j.Title != "Planted J-2" {
		t.Fatalf("J-2 in the fold: %+v", j)
	}
	file, err := os.ReadFile(filepath.Join(task, "questions", "Q-1.md"))
	if err != nil || !strings.Contains(string(file), `"job": "J-3"`) {
		t.Fatalf("question file: %v %s", err, file)
	}
}

// Row 1: a person's ask picks J-2, another writer posts J-2 first, and the ask
// takes J-3.
func TestJwRpPersonAskRepicksWhenItsPickWasTaken(t *testing.T) {
	h, path, store := orchestratorHarness(t)
	picks := jwRpHook(t, 1)
	code, q := jwRpAsk(t, h, path, false)
	jwRpCheckRepick(t, h, path, store, code, q, *picks)
}

// Row 2: the same under the task lease, where the Job is posted through f.Post.
func TestJwRpLeaseAskRepicksWhenItsPickWasTaken(t *testing.T) {
	h, path, store := orchestratorHarness(t)
	picks := jwRpHook(t, 1)
	code, q := jwRpAsk(t, h, path, true)
	jwRpCheckRepick(t, h, path, store, code, q, *picks)
}

// Row 3: when every pick is taken the ask stops after three picks, names the
// Job that exists, and leaves no question file and no question post.
func TestJwRpAskFailsWhenEveryPickIsTaken(t *testing.T) {
	for _, lease := range []bool{false, true} {
		name := "person"
		if lease {
			name = "lease"
		}
		t.Run(name, func(t *testing.T) {
			h, path, store := orchestratorHarness(t)
			before := jwRpQuestionFiles(t, path)
			picks := jwRpHook(t, 99)
			code, _ := jwRpAsk(t, h, path, lease)
			if code == 0 {
				t.Fatalf("ask exited 0 with every pick taken: %s", h.out)
			}
			stderr := h.errb.String()
			if !strings.Contains(stderr, "J-4 already exists") {
				t.Fatalf("stderr %q does not name the existing Job (ErrJobExists text)", stderr)
			}
			if want := []string{"J-2", "J-3", "J-4"}; !slices.Equal(*picks, want) {
				t.Fatalf("picks %v, want %v: three tries and no more", *picks, want)
			}
			after := jwRpQuestionFiles(t, path)
			t.Logf("exit %d; stderr %q; questions/ before %v, after %v", code, stderr, before, after)
			if !slices.Equal(before, after) {
				t.Fatalf("questions/ changed from %v to %v", before, after)
			}
			posts, err := store.Read(feed.Filter{Type: "question"})
			if err != nil || len(posts.Records) != 0 {
				t.Fatalf("question posts after a failed ask: %v %+v", err, posts.Records)
			}
			titles := jwRpJobTitles(t, store)
			for id, title := range titles {
				if title == jwRpAskTitle {
					t.Fatalf("a question Job %s was posted: %v", id, titles)
				}
			}
		})
	}
}
