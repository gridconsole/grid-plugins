package feed_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/feed"
	"github.com/gridconsole/grid-plugins/internal/job"
)

const plKtTask = "01ARZ3NDEKTSV4RRFFQ69G5FAV"

func plKtStore(t *testing.T) feed.Store {
	t.Helper()
	return feed.Store{TaskDir: t.TempDir(), TaskID: plKtTask}
}

func plKtWriter(role string) feed.Writer {
	return feed.Writer{Author: feed.Author{ID: role, Role: role}, Worker: role + "@01ARZ3NDEKTSV4RRFFQ69G5FAW"}
}

// plKtJSON gives v the shape it has after a JSON round-trip, so numbers and
// lists compare the way the Feed hands them back.
func plKtJSON(t *testing.T, v any) any {
	t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	var out any
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatal(err)
	}
	return out
}

// plKtRecord builds the Job record the planner writes: the valid job-record
// fixture with set applied, decoded into job.Job and written back by
// Job.Record, so the Job's own fields decide what the Feed gets. It fails when
// a field of set does not survive job.Job.
func plKtRecord(t *testing.T, set map[string]any) map[string]any {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "contract", "fixtures", "job-record", "valid", "basic.json"))
	if err != nil {
		t.Fatal(err)
	}
	var rec map[string]any
	if err := json.Unmarshal(data, &rec); err != nil {
		t.Fatal(err)
	}
	// A runner Job runs in a worktree its verb makes, as in the land round-trip
	// test in internal/cli; the fixture's worktree is a builder's.
	delete(rec, "worktree")
	for k, v := range set {
		rec[k] = v
	}
	var j job.Job
	if err := json.Unmarshal(plKtJSONBytes(t, rec), &j); err != nil {
		t.Fatal(err)
	}
	body, err := j.Record()
	if err != nil {
		t.Fatal(err)
	}
	if err := contract.Validate("job-record", body); err != nil {
		t.Fatalf("job-record: %v\n%s", err, body)
	}
	var out map[string]any
	if err := json.Unmarshal(body, &out); err != nil {
		t.Fatal(err)
	}
	for k, v := range set {
		if want := plKtJSON(t, v); !reflect.DeepEqual(out[k], want) {
			t.Fatalf("job.Job record %s = %#v, want %#v", k, out[k], want)
		}
	}
	return out
}

func plKtJSONBytes(t *testing.T, v any) []byte {
	t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// plKtReadOne posts in as w and reads the Feed back by type, returning the one
// record it holds, counted.
func plKtReadOne(t *testing.T, s feed.Store, in feed.Input, w feed.Writer) feed.Entry {
	t.Helper()
	posted, err := s.Post(in, w)
	if err != nil {
		t.Fatal(err)
	}
	read, err := s.Read(feed.Filter{Type: in.Type})
	if err != nil {
		t.Fatal(err)
	}
	if len(read.Bad) != 0 || len(read.Records) != 1 {
		t.Fatalf("read = %+v", read)
	}
	got := read.Records[0]
	if got.ID != posted.ID || got.Verdict != "counted" {
		t.Fatalf("read back %+v, want %s counted", got, posted.ID)
	}
	return got
}

// Rows 2, 4 and 5 (V12, V27, V26): a land Job whose branch is a full ref, a
// command Job with its argv and a wait Job with every and timeout are written by
// job.Job, posted to the Feed, read back whole and folded with their kind.
func TestPlKtJobKindsRoundTrip(t *testing.T) {
	for _, tc := range []struct {
		name string
		set  map[string]any
	}{
		{"land full ref", map[string]any{"kind": "land", "target": "main", "branch": "refs/grid/T-1/head", "sha": "0123456789abcdef0123456789abcdef01234567"}},
		{"command", map[string]any{"kind": "command", "command": []any{"land", "T-1"}}},
		{"runner command", map[string]any{"kind": "command", "command": []any{"land", "test", "T-1", "J-3"}}},
		{"wait", map[string]any{"kind": "wait", "every": "1m", "timeout": "30m"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			set := map[string]any{"id": "J-7", "role": "runner"}
			for k, v := range tc.set {
				set[k] = v
			}
			body := plKtRecord(t, set)
			s := plKtStore(t)
			got := plKtReadOne(t, s, feed.Input{Type: "job", Job: "J-7", Body: body}, plKtWriter("member"))
			if !reflect.DeepEqual(map[string]any(got.Body), body) {
				t.Fatalf("read back body %#v\nwant %#v", got.Body, body)
			}
			state, err := s.State()
			if err != nil {
				t.Fatal(err)
			}
			if j := state.Jobs["J-7"]; j == nil || j.Kind != tc.set["kind"] || j.Role != "runner" {
				t.Fatalf("folded Job = %+v, want kind %v and role runner", j, tc.set["kind"])
			}
		})
	}
}

// Row 6 (V12.c): the tested post of the contract example and a nothing-to-land
// tested post are posted by the CLI on the runner Job, read back whole and
// counted.
func TestPlKtTestedRoundTrip(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "contract", "fixtures", "tested", "valid", "basic.json"))
	if err != nil {
		t.Fatal(err)
	}
	var fx struct {
		Body map[string]any `json:"body"`
	}
	if err := json.Unmarshal(data, &fx); err != nil {
		t.Fatal(err)
	}
	nothing := map[string]any{"sha": "3f2c9e1", "base_sha": "3f2c9e1", "head_sha": "3f2c9e1", "result": "nothing", "flakes": []any{}, "preexisting": []any{}}
	for _, tc := range []struct {
		name string
		body map[string]any
	}{
		{"example", fx.Body},
		{"nothing to land", nothing},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, key := range []string{"sha", "base_sha", "head_sha", "result", "flakes", "preexisting"} {
				if _, ok := tc.body[key]; !ok {
					t.Fatalf("body has no %s: %#v", key, tc.body)
				}
			}
			got := plKtReadOne(t, plKtStore(t), feed.Input{Type: "tested", Job: "J-7", Attempt: 1, Body: tc.body}, plKtWriter("cli"))
			if got.Job == nil || *got.Job != "J-7" || !reflect.DeepEqual(map[string]any(got.Body), plKtJSON(t, tc.body)) {
				t.Fatalf("read back %+v\nwant body %#v on J-7", got, tc.body)
			}
		})
	}
}
