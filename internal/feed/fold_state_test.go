package feed

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gridconsole/grid-plugins/internal/contract"
)

// foldJob posts a Job record as the task lease holder.
func foldJob(t *testing.T, s Store, task Writer, id string) Record {
	t.Helper()
	return mustPost(t, s, Input{Type: "job", Job: id, Body: contractBody(t, "job", map[string]any{"id": id, "title": "Job " + id})}, task)
}

func readFoldFile(t *testing.T, s Store, name string, into any) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(s.Dir(), name))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, into); err != nil {
		t.Fatalf("%s does not decode: %v\n%s", name, err, data)
	}
	return data
}

func foldJobIDs(state contract.State) []string {
	ids := []string{}
	for id := range state.Jobs {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

func mustJSON(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// Row 1: the fold admits only counted, stale and unknown entries, and marks
// stale ones. One Job per verdict makes the admitted set visible in the fold.
func TestFoldReadAdmitsOnlyCountedAndStale(t *testing.T) {
	s := testStore(t)
	task := taskWriter(t, s)
	counted := foldJob(t, s, task, "J-1")
	held := foldJob(t, s, task, "J-2")
	ignored := foldJob(t, s, task, "J-3")
	stale := foldJob(t, s, task, "J-4")
	read, err := s.Read(Filter{})
	if err != nil {
		t.Fatal(err)
	}
	verdicts := map[string]string{counted.ID: "counted", held.ID: "held", ignored.ID: "ignored", stale.ID: "stale"}
	seen := 0
	for i := range read.Records {
		if v, ok := verdicts[read.Records[i].ID]; ok {
			read.Records[i].Verdict = v
			seen++
		}
	}
	if seen != 4 {
		t.Fatalf("read held %d of the 4 posted Job records", seen)
	}

	got := posts(read)
	byID := map[string]contract.Post{}
	for _, p := range got {
		byID[p.ID] = p
	}
	if _, ok := byID[held.ID]; ok {
		t.Fatal("held entry reached the fold")
	}
	if _, ok := byID[ignored.ID]; ok {
		t.Fatal("ignored entry reached the fold")
	}
	if p, ok := byID[counted.ID]; !ok || p.Stale {
		t.Fatalf("counted entry: present=%v post=%+v", ok, p)
	}
	if p, ok := byID[stale.ID]; !ok || !p.Stale {
		t.Fatalf("stale entry must reach the fold marked Stale: present=%v post=%+v", ok, p)
	}
	// The read also holds the task lease's own records (counted); only the held
	// and the ignored entry are left out.
	if len(got) != len(read.Records)-2 {
		t.Fatalf("admitted %d of %d entries, want all but the held and ignored", len(got), len(read.Records))
	}

	state := FoldRead(read)
	if _, ok := state.Jobs["J-1"]; !ok {
		t.Fatalf("counted Job missing from fold: %v", foldJobIDs(state))
	}
	for _, id := range []string{"J-2", "J-3"} {
		if _, ok := state.Jobs[id]; ok {
			t.Fatalf("Job %s from a held or ignored entry was folded", id)
		}
	}
	found := false
	for _, v := range state.Verdicts {
		if v.ID == stale.ID && v.Kind == "stale" {
			found = true
		}
	}
	if !found {
		t.Fatalf("fold did not see the stale post as stale: %+v", state.Verdicts)
	}
	if _, ok := state.Jobs["J-4"]; ok {
		t.Fatal("stale Job post was folded into a Job")
	}
}

// An unknown-typed entry is admitted too (the fold decides what it means).
func TestPostsAdmitsUnknownEntry(t *testing.T) {
	read := ReadResult{Records: []Entry{
		{Record: Record{ID: "A", Body: map[string]any{}}, Verdict: "unknown"},
		{Record: Record{ID: "B", Body: map[string]any{}}, Verdict: "bad"},
	}}
	got := posts(read)
	if len(got) != 1 || got[0].ID != "A" || got[0].Stale {
		t.Fatalf("posts: %+v", got)
	}
}

// Row 2: SaveState publishes both projections under the Feed dir.
func TestSaveStateWritesProjections(t *testing.T) {
	s := testStore(t)
	task := taskWriter(t, s)
	foldJob(t, s, task, "J-1")
	q := mustPost(t, s, Input{Type: "question", Body: map[string]any{"question": "Proceed?", "who": "member", "gate": map[string]any{"id": "Q-1", "kind": "person", "default": "Wait", "at": "2099-01-01T00:00:00.000Z"}}}, testWriter())

	state, err := s.SaveState()
	if err != nil {
		t.Fatal(err)
	}
	if len(state.NeedsYou) != 1 || state.NeedsYou[0] != q.ID {
		t.Fatalf("test needs an open question: %+v", state.NeedsYou)
	}
	if _, ok := state.Jobs["J-1"]; !ok {
		t.Fatalf("Job missing: %v", foldJobIDs(state))
	}

	var disk contract.State
	readFoldFile(t, s, "state.json", &disk)
	if mustJSON(t, disk) != mustJSON(t, state) {
		t.Fatalf("state.json differs from the returned State:\n%s\n%s", mustJSON(t, disk), mustJSON(t, state))
	}
	if !reflect.DeepEqual(foldJobIDs(disk), []string{"J-1"}) {
		t.Fatalf("state.json Jobs: %v", foldJobIDs(disk))
	}
	var needs []string
	readFoldFile(t, s, "needs-you.json", &needs)
	if !reflect.DeepEqual(needs, state.NeedsYou) {
		t.Fatalf("needs-you.json %v, State.NeedsYou %v", needs, state.NeedsYou)
	}
	if _, err := os.Stat(filepath.Join(s.Dir(), ".cache", "fold.lock")); err != nil {
		t.Fatalf("fold lock file: %v", err)
	}
}

// Row 3: liveness is overlaid on a Job in state.json only.
func TestSaveStateLivenessOverlaysProjectionOnly(t *testing.T) {
	s := testStore(t)
	task := taskWriter(t, s)
	foldJob(t, s, task, "J-1")
	foldJob(t, s, task, "J-2")

	plain, err := s.SaveState()
	if err != nil {
		t.Fatal(err)
	}
	var plainNeeds []byte
	plainNeeds, err = os.ReadFile(filepath.Join(s.Dir(), "needs-you.json"))
	if err != nil {
		t.Fatal(err)
	}

	state, err := s.SaveStateLiveness(map[string]string{"J-1": "offline", "J-9": "alive"})
	if err != nil {
		t.Fatal(err)
	}
	var view struct {
		Jobs map[string]map[string]any `json:"jobs"`
	}
	raw := readFoldFile(t, s, "state.json", &view)
	if view.Jobs["J-1"]["liveness"] != "offline" {
		t.Fatalf("J-1 liveness: %v", view.Jobs["J-1"]["liveness"])
	}
	if _, ok := view.Jobs["J-2"]["liveness"]; ok {
		t.Fatal("J-2 got a liveness value it was not given")
	}
	if _, ok := view.Jobs["J-9"]; ok {
		t.Fatalf("J-9 is not a Job and must not be added: %v", view.Jobs["J-9"])
	}
	if len(view.Jobs) != 2 {
		t.Fatalf("Jobs in state.json: %d", len(view.Jobs))
	}
	if strings.Count(string(raw), `"liveness"`) != 1 {
		t.Fatalf("liveness must appear once in state.json: %s", raw)
	}

	if strings.Contains(mustJSON(t, state), "liveness") {
		t.Fatal("returned State carries liveness")
	}
	after, err := s.State()
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(mustJSON(t, after), "liveness") {
		t.Fatal("State() carries liveness")
	}
	if mustJSON(t, state) != mustJSON(t, plain) || mustJSON(t, after) != mustJSON(t, plain) {
		t.Fatal("liveness changed the fold")
	}
	needsAfter, err := os.ReadFile(filepath.Join(s.Dir(), "needs-you.json"))
	if err != nil || string(needsAfter) != string(plainNeeds) {
		t.Fatalf("needs-you.json changed with liveness: %q %v", needsAfter, err)
	}

	// An empty overlay writes the plain fold again, dropping the display value.
	if _, err := s.SaveStateLiveness(nil); err != nil {
		t.Fatal(err)
	}
	raw = readFoldFile(t, s, "state.json", &view)
	if strings.Contains(string(raw), "liveness") {
		t.Fatalf("liveness stayed after a save without it: %s", raw)
	}
}

// Row 4: an unchanged projection is not rewritten; a changed one is.
func TestSaveStateLeavesUnchangedProjectionsAlone(t *testing.T) {
	s := testStore(t)
	task := taskWriter(t, s)
	foldJob(t, s, task, "J-1")
	if _, err := s.SaveState(); err != nil {
		t.Fatal(err)
	}
	old := time.Date(2001, 2, 3, 4, 5, 6, 0, time.UTC)
	names := []string{"state.json", "needs-you.json"}
	for _, name := range names {
		if err := os.Chtimes(filepath.Join(s.Dir(), name), old, old); err != nil {
			t.Fatal(err)
		}
	}
	mtime := func(name string) time.Time {
		info, err := os.Stat(filepath.Join(s.Dir(), name))
		if err != nil {
			t.Fatal(err)
		}
		return info.ModTime()
	}
	before, err := os.ReadFile(filepath.Join(s.Dir(), "state.json"))
	if err != nil {
		t.Fatal(err)
	}

	if _, err := s.SaveState(); err != nil {
		t.Fatal(err)
	}
	for _, name := range names {
		if !mtime(name).Equal(old) {
			t.Fatalf("%s rewritten with no new post: mtime %s", name, mtime(name))
		}
	}

	foldJob(t, s, task, "J-2")
	state, err := s.SaveState()
	if err != nil {
		t.Fatal(err)
	}
	if mtime("state.json").Equal(old) {
		t.Fatal("state.json not rewritten after a new Job post")
	}
	after, err := os.ReadFile(filepath.Join(s.Dir(), "state.json"))
	if err != nil || string(after) == string(before) {
		t.Fatalf("state.json content unchanged after a new Job post: %v", err)
	}
	var disk contract.State
	readFoldFile(t, s, "state.json", &disk)
	if _, ok := disk.Jobs["J-2"]; !ok || len(state.Jobs) != 2 {
		t.Fatalf("state.json lacks the new Job: %v", foldJobIDs(disk))
	}
	// needs_you did not change, so its file stays untouched.
	if !mtime("needs-you.json").Equal(old) {
		t.Fatal("needs-you.json rewritten though needs_you did not change")
	}
}

// Row 5: concurrent SaveState calls agree and leave a decodable state.json.
func TestSaveStateConcurrent(t *testing.T) {
	s := testStore(t)
	task := taskWriter(t, s)
	foldJob(t, s, task, "J-1")
	foldJob(t, s, task, "J-2")
	foldJob(t, s, task, "J-3")

	const n = 8
	var wg sync.WaitGroup
	start := make(chan struct{})
	ids := make([][]string, n)
	errs := make([]error, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			state, err := s.SaveState()
			errs[i] = err
			ids[i] = foldJobIDs(state)
		}(i)
	}
	close(start)
	wg.Wait()
	want := []string{"J-1", "J-2", "J-3"}
	for i := 0; i < n; i++ {
		if errs[i] != nil {
			t.Fatalf("caller %d: %v", i, errs[i])
		}
		if !reflect.DeepEqual(ids[i], want) {
			t.Fatalf("caller %d saw Jobs %v, want %v", i, ids[i], want)
		}
	}
	var disk contract.State
	readFoldFile(t, s, "state.json", &disk)
	if !reflect.DeepEqual(foldJobIDs(disk), want) {
		t.Fatalf("state.json Jobs: %v", foldJobIDs(disk))
	}
	var needs []string
	readFoldFile(t, s, "needs-you.json", &needs)
}
