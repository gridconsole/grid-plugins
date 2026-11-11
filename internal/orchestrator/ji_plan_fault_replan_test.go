package orchestrator

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/feed"
)

const jiPfNoticePrefix = "plan.md cannot be published: "
const jiPfNoticeSuffix = ". No Job from plan.md is posted until it is fixed."

// jiPfReplanJobs returns the job records on the Feed with role orchestrator and
// the plan-fault title.
func jiPfReplanJobs(t *testing.T, f *fixture) []feed.Entry {
	t.Helper()
	read, err := f.s.Feed.Read(feed.Filter{Type: "job"})
	if err != nil {
		t.Fatal(err)
	}
	var out []feed.Entry
	for _, r := range read.Records {
		if r.Body["role"] == "orchestrator" && r.Body["title"] == jiPlanFaultTitle {
			out = append(out, r)
		}
	}
	return out
}

// jiPfCause is the cause the one notice on the Feed names: the text between its
// fixed prefix and suffix.
func jiPfCause(t *testing.T, f *fixture) string {
	t.Helper()
	notices := hvPlanNotices(t, f)
	if len(notices) != 1 || !strings.HasSuffix(notices[0], jiPfNoticeSuffix) {
		t.Fatalf("notices: %q", notices)
	}
	return strings.TrimSuffix(strings.TrimPrefix(notices[0], jiPfNoticePrefix), jiPfNoticeSuffix)
}

// jiPfPlanAssigned counts the Jobs a tick assigned without the re-plan Job the
// tick posts for a plan fault (role orchestrator).
func jiPfPlanAssigned(out Output) int {
	n := 0
	for _, sp := range out.Spawns {
		if sp.Role != gkReplanRole {
			n++
		}
	}
	return n
}

// jiPfTick runs one tick and fails on an error, as hv_plan_unreadable_test.go does.
func jiPfTick(t *testing.T, f *fixture) {
	t.Helper()
	if _, err := f.s.Tick(); err != nil {
		t.Fatalf("tick stopped on an unpublishable plan: %v", err)
	}
}

func TestJiPfReplanJobPosted(t *testing.T) {
	f := setupPlan(t)
	writePlan(t, f, hvPlanScalarJobs)
	jiPfTick(t, f)
	jobs := jiPfReplanJobs(t, f)
	if len(jobs) != 1 {
		t.Fatalf("want one re-plan Job, got %d: %+v", len(jobs), jobs)
	}
	got := jobs[0]
	if got.Job == nil || *got.Job != "J-4" {
		t.Fatalf("id = %v, want J-4 (above J-3 of the plan)", got.Job)
	}
	if got.Lease == nil || *got.Lease != "task" {
		t.Fatalf("lease = %v, want task", got.Lease)
	}
	rows, _ := got.Body["acceptance"].([]any)
	if len(rows) != 1 {
		t.Fatalf("acceptance = %v", got.Body["acceptance"])
	}
	a1, _ := rows[0].(map[string]any)
	text, _ := a1["text"].(string)
	if a1["id"] != "A1" || !strings.Contains(text, "each jobs entry must be an object") || !strings.Contains(text, "J-4") {
		t.Fatalf("A1 = %v", a1)
	}
	// What the tick posts is what ReplanPlanFaultJobBody builds, field by field.
	want, err := json.Marshal(ReplanPlanFaultJobBody("J-4", jiPfCause(t, f)))
	if err != nil {
		t.Fatal(err)
	}
	var wantBody map[string]any
	if err := json.Unmarshal(want, &wantBody); err != nil {
		t.Fatal(err)
	}
	for k, v := range wantBody {
		if !reflect.DeepEqual(got.Body[k], v) {
			t.Errorf("field %s = %v, want %v", k, got.Body[k], v)
		}
	}
	for k := range got.Body {
		if _, ok := wantBody[k]; !ok {
			t.Errorf("field %s is on the Feed but not in the body builder: %v", k, got.Body[k])
		}
	}
	if got.Body["discovered_from"] != nil {
		t.Errorf("discovered_from = %v, want null", got.Body["discovered_from"])
	}
	// The cause is the plan fault only: it holds no path of the task folder.
	if strings.Contains(text, f.s.Feed.TaskDir) {
		t.Errorf("A1 holds the task folder path: %q", text)
	}
}

func TestJiPfOnceWhileOpen(t *testing.T) {
	f := setupPlan(t)
	writePlan(t, f, hvPlanScalarJobs)
	jiPfTick(t, f)
	jiPfTick(t, f)
	jiPfTick(t, f)
	if jobs := jiPfReplanJobs(t, f); len(jobs) != 1 {
		t.Fatalf("want one re-plan Job after three ticks, got %d: %+v", len(jobs), jobs)
	}
	if notices := hvPlanNotices(t, f); len(notices) != 1 {
		t.Fatalf("notice repeated: %q", notices)
	}
}

func TestJiPfAgainAfterAccepted(t *testing.T) {
	f := setupPlan(t)
	writePlan(t, f, hvPlanScalarJobs)
	jiPfTick(t, f)
	state, err := f.s.Feed.State()
	if err != nil || state.Jobs["J-4"] == nil {
		t.Fatalf("J-4 is not on the Feed: %v", err)
	}
	if err := f.s.control(state.Jobs["J-4"], contract.Accepted, "accepted", ""); err != nil {
		t.Fatal(err)
	}
	if state, err = f.s.Feed.State(); err != nil || state.Jobs["J-4"].Status != contract.Accepted {
		t.Fatalf("J-4 is not accepted: %+v %v", state.Jobs["J-4"], err)
	}
	jiPfTick(t, f)
	jobs := jiPfReplanJobs(t, f)
	if len(jobs) != 2 || jobs[1].Job == nil || *jobs[1].Job != "J-5" {
		t.Fatalf("want a second re-plan Job J-5, got %+v", jobs)
	}
	// The notice is still deduplicated by its text (hv note 4 stays for the notice).
	if notices := hvPlanNotices(t, f); len(notices) != 1 {
		t.Fatalf("notices: %q", notices)
	}
}

// A Job of a person that carries the title has no task lease: it does not stand
// for the tick's own Job.
func TestJiPfPersonJobDoesNotSilence(t *testing.T) {
	f := setupPlan(t)
	writePlan(t, f, hvPlanScalarJobs)
	person := feed.Writer{Author: feed.Author{ID: "dana", Role: "member"}, Worker: "dana@" + machine}
	f.post(t, feed.Input{Type: "job", Job: "J-9", Body: ReplanPlanFaultJobBody("J-9", "by hand")}, person)
	jiPfTick(t, f)
	jobs := jiPfReplanJobs(t, f)
	if len(jobs) != 2 || jobs[1].Job == nil || *jobs[1].Job != "J-10" || jobs[1].Lease == nil || *jobs[1].Lease != "task" {
		t.Fatalf("want the tick's own Job J-10 beside the person's J-9, got %+v", jobs)
	}
}

func TestJiPfIdAbovePlanIds(t *testing.T) {
	f := setupPlan(t)
	writePlan(t, f, strings.Replace(hvPlanScalarJobs, "J-3", "J-7", 1))
	jiPfTick(t, f)
	jobs := jiPfReplanJobs(t, f)
	if len(jobs) != 1 || jobs[0].Job == nil || *jobs[0].Job != "J-8" {
		t.Fatalf("want one re-plan Job J-8, got %+v", jobs)
	}
}

// The fold counts too: a Job on the Feed above every id of the plan text sets the id.
func TestJiPfIdAboveFold(t *testing.T) {
	f := setupPlan(t)
	f.job(t, "J-12", nil)
	writePlan(t, f, hvPlanScalarJobs)
	jiPfTick(t, f)
	jobs := jiPfReplanJobs(t, f)
	if len(jobs) != 1 || jobs[0].Job == nil || *jobs[0].Job != "J-13" {
		t.Fatalf("want one re-plan Job J-13, got %+v", jobs)
	}
}

func TestJiPfPlanIDNumbers(t *testing.T) {
	for _, tc := range []struct{ name, text, want string }{
		{"no id", "not a plan", "J-1"},
		{"one id", "jobs: [J-3]", "J-4"},
		{"highest of several", "J-2 J-11 J-5", "J-12"},
		{"sub id counts its top level", "J-3.1", "J-4"},
		{"ten digits are not counted", "J-1234567890 J-2", "J-3"},
		{"nine digits are counted", "J-999999999", "J-1000000000"},
		{"no digits", "J-x", "J-1"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := jiPlanFaultJobID(Snapshot{}, tc.text); got != tc.want {
				t.Fatalf("jiPlanFaultJobID(%q) = %s, want %s", tc.text, got, tc.want)
			}
		})
	}
}
