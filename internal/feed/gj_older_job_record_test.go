package feed

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/contract"
)

// Job records written before e11cf80a still sit in every older task's Feed. They
// carry "review" and "max_rounds", which the schema no longer requires and the
// fold no longer decodes. The reader validates each counted record against the
// current schema, so these tests read the old bytes through Store.Read and
// Store.State and show the old Job still counts and still folds into the Job.

// gjOldSecondID is the id of the record that wraps the planned-fields body. The
// whole-record fixture keeps its own id.
const gjOldSecondID = "01K9J000000000000000000003"

// gjOldHome points every home the code under test could read at folders under
// one temp root, and stops the test when HOME is not under that root.
func gjOldHome(t *testing.T) {
	t.Helper()
	root := t.TempDir()
	for _, key := range []string{"HOME", "CLAUDE_CONFIG_DIR", "CODEX_HOME", "GRID_HOME"} {
		dir := filepath.Join(root, strings.ToLower(key))
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
		t.Setenv(key, dir)
	}
	rel, err := filepath.Rel(root, os.Getenv("HOME"))
	if err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		t.Fatal("HOME is not under the test's temp root")
	}
}

// gjOldStore is a test store whose home is isolated first.
func gjOldStore(t *testing.T) Store {
	t.Helper()
	gjOldHome(t)
	s := testStore(t)
	if err := s.prepare(); err != nil {
		t.Fatal(err)
	}
	return s
}

func gjOldFixture(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "older-records", name))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// gjOldRecord is the whole Feed record git held, with the body untouched. Only
// the task, the worker, the seq and the author id (the writer check wants it to
// be the member of the worker) are the test store's.
func gjOldRecord(t *testing.T) Record {
	t.Helper()
	var r Record
	if err := json.Unmarshal(gjOldFixture(t, "job-basic.json"), &r); err != nil {
		t.Fatal(err)
	}
	w := testWriter()
	r.Task = testTask
	r.Worker = w.Worker
	r.Author.ID = w.Author.ID
	r.Seq = 1
	return r
}

// gjOldPlannedRecord wraps the older Job body git held (planned-fields, with kind
// and the review fields) as the body of a second record of type job.
func gjOldPlannedRecord(t *testing.T) Record {
	t.Helper()
	r := gjOldRecord(t)
	r.ID = gjOldSecondID
	r.Body = nil
	if err := json.Unmarshal(gjOldFixture(t, "job-record-planned-fields.json"), &r.Body); err != nil {
		t.Fatal(err)
	}
	return r
}

type gjOldCase struct {
	name   string
	record func(*testing.T) Record
	kind   string
}

var gjOldCases = []gjOldCase{
	{"whole record", gjOldRecord, ""},
	{"planned fields body", gjOldPlannedRecord, "non-code"},
}

// gjOldBodyOf decodes the fixture body on its own, for an equality check against
// what the reader returns.
func gjOldBodyOf(t *testing.T, r Record) map[string]any {
	t.Helper()
	data, err := json.Marshal(r.Body)
	if err != nil {
		t.Fatal(err)
	}
	var out map[string]any
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatal(err)
	}
	return out
}

func gjOldEntry(t *testing.T, read ReadResult, id string) Entry {
	t.Helper()
	for _, e := range read.Records {
		if e.ID == id {
			return e
		}
	}
	t.Fatalf("record %s is not in the read", id)
	return Entry{}
}

func gjOldKeys(v any, into map[string]bool) {
	switch x := v.(type) {
	case map[string]any:
		for k, child := range x {
			into[k] = true
			gjOldKeys(child, into)
		}
	case []any:
		for _, child := range x {
			gjOldKeys(child, into)
		}
	}
}

// gjOldFoldVerdict is the fold's own verdict for one record.
func gjOldFoldVerdict(state contract.State, id string) string {
	for _, v := range state.Verdicts {
		if v.ID == id {
			return v.Kind
		}
	}
	return ""
}

// Row 1: the reader counts an old Job record and drops nothing from its body.
func TestGjOlderRecordCounts(t *testing.T) {
	for _, tc := range gjOldCases {
		t.Run(tc.name, func(t *testing.T) {
			s := gjOldStore(t)
			r := tc.record(t)
			want := gjOldBodyOf(t, r)
			rawAppend(t, s, r)
			read, err := s.Read(Filter{})
			if err != nil {
				t.Fatal(err)
			}
			if len(read.Records) != 1 || len(read.Bad) != 0 {
				t.Fatalf("records=%d bad=%v", len(read.Records), read.Bad)
			}
			e := read.Records[0]
			if e.ID != r.ID || e.Verdict != "counted" || e.Reason != "" {
				t.Fatalf("verdict=%q reason=%q", e.Verdict, e.Reason)
			}
			if e.Body["review"] != "L1" || e.Body["max_rounds"] != float64(2) {
				t.Fatalf("body lost the old keys: review=%v max_rounds=%v", e.Body["review"], e.Body["max_rounds"])
			}
			if got := gjOldBodyOf(t, Record{Body: e.Body}); !reflect.DeepEqual(got, want) {
				t.Fatalf("the reader changed the body\n got %v\nwant %v", got, want)
			}
		})
	}
	// Control: the same path still turns a broken body away, so "counted" above
	// comes from the validator accepting the old keys and not from it not looking.
	t.Run("control: a body without a title is ignored", func(t *testing.T) {
		s := gjOldStore(t)
		r := gjOldRecord(t)
		delete(r.Body, "title")
		rawAppend(t, s, r)
		read, err := s.Read(Filter{})
		if err != nil {
			t.Fatal(err)
		}
		if len(read.Records) != 1 || read.Records[0].Verdict != "ignored" {
			t.Fatalf("records=%+v", read.Records)
		}
	})
}

// Row 2: the old Job folds into state.Jobs as a posted Job without the dropped keys.
func TestGjOlderRecordFolds(t *testing.T) {
	for _, tc := range gjOldCases {
		t.Run(tc.name, func(t *testing.T) {
			s := gjOldStore(t)
			r := tc.record(t)
			rawAppend(t, s, r)
			state, err := s.State()
			if err != nil {
				t.Fatal(err)
			}
			job := state.Jobs["J-1"]
			if job == nil {
				t.Fatalf("no Job J-1 in state.Jobs: %v", foldJobIDs(state))
			}
			if job.ID != "J-1" || job.Title != "Build example feature" || job.Status != contract.Posted || job.Role != "builder" || job.Kind != tc.kind {
				t.Fatalf("job id=%q title=%q status=%q role=%q kind=%q", job.ID, job.Title, job.Status, job.Role, job.Kind)
			}
			if len(job.History) != 1 || job.History[0] != r.ID || gjOldFoldVerdict(state, r.ID) != "counted" {
				t.Fatalf("history=%v verdict=%q", job.History, gjOldFoldVerdict(state, r.ID))
			}
			var view any
			if err := json.Unmarshal([]byte(mustJSON(t, state)), &view); err != nil {
				t.Fatal(err)
			}
			keys := map[string]bool{}
			gjOldKeys(view, keys)
			for _, key := range []string{"max_rounds", "level_raised"} {
				if keys[key] {
					t.Fatalf("the folded state carries %q", key)
				}
			}
			var jobView map[string]any
			if err := json.Unmarshal([]byte(mustJSON(t, job)), &jobView); err != nil {
				t.Fatal(err)
			}
			if _, ok := jobView["review"]; ok {
				t.Fatal("the folded Job carries review")
			}
		})
	}
}

// Row 3: an old Job and a Job posted today on one store both count and both fold.
func TestGjOlderRecordMixedWithNew(t *testing.T) {
	s := gjOldStore(t)
	old := gjOldRecord(t)
	rawAppend(t, s, old)
	task := taskWriter(t, s)
	fresh := foldJob(t, s, task, "J-2")
	read, err := s.Read(Filter{})
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{old.ID, fresh.ID} {
		if e := gjOldEntry(t, read, id); e.Verdict != "counted" {
			t.Fatalf("record %s verdict=%q reason=%q", id, e.Verdict, e.Reason)
		}
	}
	if e := gjOldEntry(t, read, old.ID); e.Body["max_rounds"] != float64(2) {
		t.Fatalf("old body max_rounds=%v", e.Body["max_rounds"])
	}
	state, err := s.State()
	if err != nil {
		t.Fatal(err)
	}
	if ids := foldJobIDs(state); !reflect.DeepEqual(ids, []string{"J-1", "J-2"}) {
		t.Fatalf("jobs=%v", ids)
	}
	for _, id := range []string{"J-1", "J-2"} {
		if state.Jobs[id].Status != contract.Posted {
			t.Fatalf("job %s status=%q", id, state.Jobs[id].Status)
		}
	}
}

// Row 4: the fixtures keep the old shape and the README names the commit, so a
// recapture cannot silently turn them into the new shape.
func TestGjOlderRecordFixturesKeepTheOldShape(t *testing.T) {
	gjOldHome(t)
	var whole struct {
		Type string         `json:"type"`
		Body map[string]any `json:"body"`
	}
	if err := json.Unmarshal(gjOldFixture(t, "job-basic.json"), &whole); err != nil {
		t.Fatal(err)
	}
	if whole.Type != "job" {
		t.Fatalf("job-basic.json type=%q", whole.Type)
	}
	var planned map[string]any
	if err := json.Unmarshal(gjOldFixture(t, "job-record-planned-fields.json"), &planned); err != nil {
		t.Fatal(err)
	}
	for name, body := range map[string]map[string]any{"job-basic.json": whole.Body, "job-record-planned-fields.json": planned} {
		if body["review"] != "L1" || body["max_rounds"] != float64(2) {
			t.Errorf("%s: review=%v max_rounds=%v, want L1 and 2", name, body["review"], body["max_rounds"])
		}
		if _, ok := body["level_raised"]; !ok {
			t.Errorf("%s: no level_raised key", name)
		}
	}
	if readme := gjOldFixture(t, "README.md"); !strings.Contains(string(readme), "e11cf80a") {
		t.Error("README.md does not name commit e11cf80a")
	}
}
