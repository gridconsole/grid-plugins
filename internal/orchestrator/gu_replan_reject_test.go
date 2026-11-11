package orchestrator

import (
	"encoding/json"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/feed"
)

// guRjReject closes a Job Rejected with a control, the way
// `grid job post --type control` does by hand.
func guRjReject(t *testing.T, f *fixture, id string) {
	t.Helper()
	f.post(t, feed.Input{Type: "control", Body: map[string]any{"target": id, "status": contract.Rejected, "reason": "owner rejected", "close_reason": "owner rejected"}}, f.s.Writer)
}

func TestGuRjReplanAfterReject(t *testing.T) {
	f := setup(t)
	f.job(t, "J-1", nil)
	f.job(t, "J-2", []any{"J-1"})
	guRjReject(t, f, "J-1")
	f.tick(t)
	if got := gkStatus(t, f, "J-1"); got != contract.Rejected {
		t.Fatalf("J-1 is %s, want rejected", got)
	}
	replans, all := gkReplans(t, f)
	if want := []string{"J-1", "J-2", "J-3"}; !reflect.DeepEqual(all, want) {
		t.Fatalf("Jobs on the Feed = %v, want %v", all, want)
	}
	if len(replans) != 1 || replans["J-3"] == nil {
		t.Fatalf("want one re-plan Job J-3, got %v", replans)
	}
	body := replans["J-3"]
	if body["role"] != "orchestrator" || body["discovered_from"] != "J-1" {
		t.Fatalf("role %v, discovered_from %v", body["role"], body["discovered_from"])
	}
	if title := body["title"]; title != "Re-plan after J-1 was rejected: J-2 wait on it" {
		t.Fatalf("title = %v", title)
	}
	rows, _ := body["acceptance"].([]any)
	if len(rows) != 1 {
		t.Fatalf("acceptance = %v", body["acceptance"])
	}
	a1, _ := rows[0].(map[string]any)
	text, _ := a1["text"].(string)
	if a1["id"] != "A1" || !strings.Contains(text, "J-2") || !strings.Contains(text, "waiting for J-1, which was rejected") {
		t.Fatalf("A1 = %v", a1)
	}
	gkStrings(body, func(s string) {
		if strings.HasPrefix(s, "job:") {
			t.Errorf("body holds a job: pointer %q", s)
		}
	})
	data, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	if err := contract.Validate("job-record", data); err != nil {
		t.Fatalf("the body on the Feed is not a job-record: %v", err)
	}
	// What the loop posts is what ReplanRejectJobBody builds.
	want, err := json.Marshal(ReplanRejectJobBody("J-3", "J-1", []string{"J-2"}))
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

func TestGuRjOnce(t *testing.T) {
	f := setup(t)
	f.job(t, "J-1", nil)
	f.job(t, "J-2", []any{"J-1"})
	guRjReject(t, f, "J-1")
	for i := 0; i < 4; i++ {
		f.tick(t)
	}
	replans, all := gkReplans(t, f)
	if len(replans) != 1 || replans["J-3"] == nil || replans["J-3"]["discovered_from"] != "J-1" {
		t.Fatalf("want one re-plan Job J-3 from J-1, got %v (Jobs %v)", replans, all)
	}
	if got := gkStatus(t, f, "J-2"); got == contract.Cancelled {
		t.Fatalf("J-2 is %s: the loop cancelled it", got)
	}
}

// A Rejected Job nothing waits on, and one whose only waiter holds a failed
// edge (which the rejection satisfies), get no new Job.
func TestGuRjNoDoneWaiter(t *testing.T) {
	for _, tc := range []struct {
		name  string
		after []any
		jobs  int
	}{
		{"no waiter", nil, 1},
		{"failed edge", []any{map[string]any{"job": "J-1", "kind": "failed", "why": "fallback"}}, 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := setup(t)
			f.job(t, "J-1", nil)
			if tc.after != nil {
				f.job(t, "J-2", tc.after)
			}
			guRjReject(t, f, "J-1")
			f.tick(t)
			f.tick(t)
			if got := gkStatus(t, f, "J-1"); got != contract.Rejected {
				t.Fatalf("J-1 is %s, want rejected", got)
			}
			if replans, all := gkReplans(t, f); len(replans) != 0 || len(all) != tc.jobs {
				t.Fatalf("want no new Job: %v (Jobs %v)", replans, all)
			}
		})
	}
}

// A Job cancelled by its worker and a Job rejected by a control are both seen in
// one tick: each gets its own re-plan Job, with its own id.
func TestGuRjSameTickAsWorkerCancel(t *testing.T) {
	f := gkRig(t, []any{"J-1"})
	f.job(t, "J-3", nil)
	f.job(t, "J-4", []any{"J-3"})
	guRjReject(t, f, "J-3")
	f.tick(t)
	replans, all := gkReplans(t, f)
	if len(replans) != 2 {
		t.Fatalf("want two re-plan Jobs, got %v (Jobs %v)", replans, all)
	}
	from := map[string]string{}
	var got []string
	for id, body := range replans {
		got = append(got, id)
		df, _ := body["discovered_from"].(string)
		from[df] = id
	}
	sort.Strings(got)
	if want := []string{"J-5", "J-6"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("re-plan Job ids = %v, want %v (Jobs %v)", got, want, all)
	}
	if len(from) != 2 || from["J-1"] == "" || from["J-3"] == "" {
		t.Fatalf("want one re-plan Job from J-1 and one from J-3, got %v", from)
	}
	if want := []string{"J-1", "J-2", "J-3", "J-4", "J-5", "J-6"}; !reflect.DeepEqual(all, want) {
		t.Fatalf("Jobs on the Feed = %v, want %v", all, want)
	}
}
