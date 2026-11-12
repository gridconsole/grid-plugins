package orchestrator

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/feed"
)

// abDnRounds is how many fresh folders TestAbDnTwoTicksOneJob races.
const abDnRounds = 20

// abDnResult posts a result on job with decisions as its decisions_needed, the
// way the fixture worker posts one (orchestrator_test.go TestWakeIdempotency):
// it acks the assignment the tick made when the Job is not running yet, then
// posts under the lease and fence of that assignment.
func abDnResult(t *testing.T, f *fixture, job string, decisions []string) feed.Record {
	t.Helper()
	state, err := f.s.Feed.State()
	if err != nil {
		t.Fatal(err)
	}
	j := state.Jobs[job]
	if j == nil || j.Assign == nil {
		t.Fatalf("%s is not assigned: %+v", job, j)
	}
	w := f.worker
	w.Lease, w.Fence, w.Gen = job, j.Assign.Fence, j.Generation
	if j.Status == contract.Assigned {
		if _, err := Write(f.s.Feed, w, feed.Input{Type: "ack", Job: job, Attempt: j.Attempt, Body: map[string]any{}}); err != nil {
			t.Fatal(err)
		}
	}
	body := resultBody()
	body["decisions_needed"] = decisions
	return f.post(t, feed.Input{Type: "result", Job: job, Attempt: j.Attempt, Body: body}, w)
}

// abDnRestart is a new Store on the folder of f: its own feed.Store value and
// nothing of the first one kept, as a restarted or second worker process holds it.
func abDnRestart(f *fixture) Store {
	s := f.s
	s.Feed = feed.Store{TaskDir: f.s.Feed.TaskDir, TaskID: taskID}
	return s
}

// abDnFeed is what the Feed of f holds: the id of every Job, the id of every
// job record read and the body of each Job.
func abDnFeed(t *testing.T, f *fixture) (all, records []string, bodies map[string]map[string]any) {
	t.Helper()
	snap, err := Load(f.s.Feed)
	if err != nil {
		t.Fatal(err)
	}
	for _, it := range snap.Items {
		all = append(all, it.ID)
	}
	return all, snap.JobIDs, snap.Bodies
}

// abDnWant fails unless the Feed holds exactly the Jobs in want.
func abDnWant(t *testing.T, f *fixture, when string, want ...string) map[string]map[string]any {
	t.Helper()
	all, _, bodies := abDnFeed(t, f)
	if !reflect.DeepEqual(all, want) {
		t.Fatalf("%s: Jobs on the Feed = %v, want %v", when, all, want)
	}
	return bodies
}

// abDnA1 is the text of A1 of a decoded job body that holds one acceptance row.
func abDnA1(t *testing.T, body map[string]any) string {
	t.Helper()
	rows, _ := body["acceptance"].([]any)
	if len(rows) != 1 {
		t.Fatalf("acceptance = %v, want one row", body["acceptance"])
	}
	a1, _ := rows[0].(map[string]any)
	if a1["id"] != "A1" {
		t.Fatalf("A1 = %v", a1)
	}
	text, _ := a1["text"].(string)
	return text
}

// abDnNaming is the id of each orchestrator Job whose A1 begins with the lead of
// the result record id.
func abDnNaming(bodies map[string]map[string]any, result string) []string {
	var naming []string
	for id, body := range bodies {
		if body["role"] != "orchestrator" {
			continue
		}
		rows, _ := body["acceptance"].([]any)
		for _, row := range rows {
			a, _ := row.(map[string]any)
			if text, _ := a["text"].(string); a["id"] == "A1" && strings.HasPrefix(text, abDnA1Lead+result+" (") {
				naming = append(naming, id)
			}
		}
	}
	return naming
}

func TestAbDnOneJobPerResult(t *testing.T) {
	f := setup(t)
	f.job(t, "J-1", nil)
	f.job(t, "J-2", nil)
	f.tick(t)
	decisions := []string{"Which export format: csv or json?", "Who approves the\nrelease date?"}
	result := abDnResult(t, f, "J-1", decisions)
	abDnResult(t, f, "J-2", []string{})
	abDnWant(t, f, "before the tick", "J-1", "J-2")
	f.tick(t)
	bodies := abDnWant(t, f, "after one tick", "J-1", "J-2", "J-3")
	body := bodies["J-3"]
	if body["role"] != "orchestrator" {
		t.Fatalf("role = %v, want orchestrator", body["role"])
	}
	text := abDnA1(t, body)
	if lead := abDnA1Lead + result.ID + " ("; !strings.HasPrefix(text, lead) {
		t.Fatalf("A1 %q does not begin with %q", text, lead)
	}
	for _, want := range []string{"Which export format: csv or json?", "Who approves the release date?", "J-1", "grid question ask", "grid job result"} {
		if !strings.Contains(text, want) {
			t.Errorf("A1 %q lacks %q", text, want)
		}
	}
	if want := "Decisions needed on J-1: Which export format: csv or json?"; body["title"] != want {
		t.Fatalf("title = %v, want %q", body["title"], want)
	}
	if body["discovered_from"] != nil {
		t.Errorf("discovered_from = %v, want null", body["discovered_from"])
	}
	gkStrings(body, func(s string) {
		if strings.HasPrefix(s, "job:") {
			t.Errorf("body holds a job: pointer %q", s)
		}
	})
	if naming := abDnNaming(bodies, result.ID); !reflect.DeepEqual(naming, []string{"J-3"}) {
		t.Fatalf("orchestrator Jobs naming result %s = %v, want [J-3]", result.ID, naming)
	}
	// What the pass posts is what AbDnJobBody builds.
	want, err := json.Marshal(AbDnJobBody("J-3", result.ID, "J-1", decisions))
	if err != nil {
		t.Fatal(err)
	}
	var wantBody map[string]any
	if err := json.Unmarshal(want, &wantBody); err != nil {
		t.Fatal(err)
	}
	for k, v := range wantBody {
		if !reflect.DeepEqual(body[k], v) {
			t.Errorf("field %s = %v, want %v", k, body[k], v)
		}
	}
}

func TestAbDnSecondTickPostsNothing(t *testing.T) {
	f := setup(t)
	f.job(t, "J-1", nil)
	f.tick(t)
	first := abDnResult(t, f, "J-1", []string{"Which export format?"})
	f.tick(t)
	abDnWant(t, f, "after the first tick", "J-1", "J-2")
	f.tick(t)
	abDnWant(t, f, "after a second tick", "J-1", "J-2")
	// A restart: a new Store on the same folder reads the Feed, not memory.
	if _, err := abDnRestart(f).Tick(); err != nil {
		t.Fatal(err)
	}
	bodies := abDnWant(t, f, "after a restart", "J-1", "J-2")
	if text := abDnA1(t, bodies["J-2"]); !strings.HasPrefix(text, abDnA1Lead+first.ID+" (") {
		t.Fatalf("J-2 A1 %q does not name the first result %s", text, first.ID)
	}
	// A second result with a new decision on the same Job is asked about on its own.
	second := abDnResult(t, f, "J-1", []string{"Which time zone do the reports use?"})
	f.tick(t)
	bodies = abDnWant(t, f, "after the second result", "J-1", "J-2", "J-3")
	if text := abDnA1(t, bodies["J-3"]); !strings.HasPrefix(text, abDnA1Lead+second.ID+" (") || strings.Contains(text, first.ID) || !strings.Contains(text, "time zone") {
		t.Fatalf("J-3 A1 %q must name %s and its decision, and not %s", text, second.ID, first.ID)
	}
	if text := abDnA1(t, bodies["J-2"]); strings.Contains(text, second.ID) {
		t.Fatalf("J-2 A1 %q names the second result", text)
	}
	f.tick(t)
	if _, err := abDnRestart(f).Tick(); err != nil {
		t.Fatal(err)
	}
	abDnWant(t, f, "after both results were asked about", "J-1", "J-2", "J-3")
}

// abDnRace is one round on a fresh task folder: a Job J-1 with a result that
// needs a decision, then two Stores on the folder ticking on one start barrier
// (kbFrTickTogether). It fails unless both Ticks answered and the Feed holds one
// orchestrator Job that names the result id, J-2, and no other Job but J-1.
func abDnRace(t *testing.T, round int) {
	t.Helper()
	f := setup(t)
	f.job(t, "J-1", nil)
	f.tick(t)
	result := abDnResult(t, f, "J-1", []string{"Which export format?"})
	blocked := kbFrBlocked(t)
	for i, err := range kbFrTickTogether(f.s, abDnRestart(f)) {
		if err != nil {
			t.Fatalf("round %d: Tick %d: %v", round, i+1, err)
		}
	}
	all, records, bodies := abDnFeed(t, f)
	if naming := abDnNaming(bodies, result.ID); !reflect.DeepEqual(naming, []string{"J-2"}) {
		t.Fatalf("round %d: orchestrator Jobs naming result %s = %v, want [J-2] (Feed holds %v)", round, result.ID, naming, all)
	}
	if want := []string{"J-1", "J-2"}; !reflect.DeepEqual(all, want) {
		t.Fatalf("round %d: Jobs on the Feed = %v, want %v", round, all, want)
	}
	if want := []string{"J-1", "J-2"}; !reflect.DeepEqual(records, want) {
		t.Fatalf("round %d: job records on the Feed = %v, want %v", round, records, want)
	}
	t.Logf("round %d: one Job J-2 for result %s; a Tick found the orchestrator lock held and waited %d time(s)", round, result.ID, blocked.Load())
}

func TestAbDnTwoTicksOneJob(t *testing.T) {
	for round := 1; round <= abDnRounds; round++ {
		abDnRace(t, round)
	}
}

func TestAbDnBodyIsOneLine(t *testing.T) {
	long := strings.Repeat("a very long decision ", 40) + "\n\tend"
	body := AbDnJobBody("J-5", "01ARZ3NDEKTSV4RRFFQ69G5FB0", "J-3", []string{long, "a second\tdecision"})
	title, _ := body["title"].(string)
	prefix := "Decisions needed on J-3: "
	if !strings.HasPrefix(title, prefix+"a very long decision") || strings.ContainsAny(title, "\n\r\t") || len([]rune(title)) > len([]rune(prefix))+kbSfTitleMax {
		t.Fatalf("title = %q", title)
	}
	rows, _ := body["acceptance"].([]map[string]any)
	if len(rows) != 1 {
		t.Fatalf("acceptance = %v", body["acceptance"])
	}
	if text, _ := rows[0]["text"].(string); strings.ContainsAny(text, "\n\r\t") || !strings.Contains(text, "; a second decision.") {
		t.Fatalf("A1 = %q", text)
	}
	data, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	if err := contract.Validate("job-record", data); err != nil {
		t.Fatalf("not a job-record: %v", err)
	}

	// A result of a decisions Job that itself holds decisions_needed asks no
	// further Job: the question that Job asks is the path to the person.
	f := setup(t)
	f.post(t, feed.Input{Type: "job", Job: "J-1", Body: AbDnJobBody("J-1", "01ARZ3NDEKTSV4RRFFQ69G5FB0", "J-9", []string{"Which export format?"})}, f.s.Writer)
	f.tick(t)
	abDnResult(t, f, "J-1", []string{"Which export format should the person pick?"})
	state, err := f.s.Feed.State()
	if err != nil {
		t.Fatal(err)
	}
	if j := state.Jobs["J-1"]; j == nil || len(j.Results) != 1 || len(j.DecisionsNeeded) != 1 {
		t.Fatalf("the result on the decisions Job J-1 was not folded: %+v", j)
	}
	f.tick(t)
	if _, err := abDnRestart(f).Tick(); err != nil {
		t.Fatal(err)
	}
	abDnWant(t, f, "after a result with decisions on the decisions Job", "J-1")
}
