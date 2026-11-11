package orchestrator

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/feed"
)

const kbSfAt = "internal/auth/token.go:42"

// kbSfFound posts a found record on job the way the fixture writer (role lead)
// does. The re field is a record id, so the task id stands in for one.
func kbSfFound(t *testing.T, f *fixture, job, kind, text string) feed.Record {
	t.Helper()
	return f.post(t, feed.Input{Type: "found", Body: map[string]any{"job": job, "re": taskID, "at": kbSfAt, "text": text, "kind": kind, "class": nil}}, f.s.Writer)
}

// kbSfJobs is the id of every Job the Feed holds and the body of each.
func kbSfJobs(t *testing.T, f *fixture) ([]string, map[string]map[string]any) {
	t.Helper()
	snap, err := Load(f.s.Feed)
	if err != nil {
		t.Fatal(err)
	}
	var all []string
	for _, it := range snap.Items {
		all = append(all, it.ID)
	}
	return all, snap.Bodies
}

// kbSfWant fails unless the Feed holds exactly the Jobs in want.
func kbSfWant(t *testing.T, f *fixture, when string, want ...string) map[string]map[string]any {
	t.Helper()
	all, bodies := kbSfJobs(t, f)
	if !reflect.DeepEqual(all, want) {
		t.Fatalf("%s: Jobs on the Feed = %v, want %v", when, all, want)
	}
	return bodies
}

// kbSfA1 is the text of A1 of a decoded job body.
func kbSfA1(t *testing.T, body map[string]any) string {
	t.Helper()
	rows, _ := body["acceptance"].([]any)
	if len(rows) != 1 {
		t.Fatalf("acceptance = %v, want one row", body["acceptance"])
	}
	a1, _ := rows[0].(map[string]any)
	text, _ := a1["text"].(string)
	if a1["id"] != "A1" {
		t.Fatalf("A1 = %v", a1)
	}
	return text
}

// kbSfRestart is a new Store on the folder of f: nothing of the first one is kept.
func kbSfRestart(f *fixture) Store {
	s := f.s
	s.Feed = feed.Store{TaskDir: f.s.Feed.TaskDir, TaskID: taskID}
	return s
}

func TestKbSfOneJobPerSecurityFound(t *testing.T) {
	f := setup(t)
	f.job(t, "J-1", nil)
	found := kbSfFound(t, f, "J-1", "security", "The token is written to the log in clear.\nIt stays in the log for ever.")
	bodies := kbSfWant(t, f, "before the tick", "J-1")
	f.tick(t)
	bodies = kbSfWant(t, f, "after one tick", "J-1", "J-2")
	body := bodies["J-2"]
	if body["role"] != "orchestrator" {
		t.Fatalf("role = %v, want orchestrator", body["role"])
	}
	if want := "Security finding on J-1: The token is written to the log in clear. It stays in the log for ever."; body["title"] != want {
		t.Fatalf("title = %v, want %q", body["title"], want)
	}
	text := kbSfA1(t, body)
	for _, want := range []string{found.ID, kbSfAt, "J-1", "fix Job", "cancel", "person"} {
		if !strings.Contains(text, want) {
			t.Errorf("A1 %q lacks %q", text, want)
		}
	}
	// The re-plan of J-1 must still be able to post: the security Job is not its re-plan.
	if body["discovered_from"] != nil {
		t.Errorf("discovered_from = %v, want null", body["discovered_from"])
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
	// What the pass posts is what SecurityFoundJobBody builds.
	want, err := json.Marshal(SecurityFoundJobBody("J-2", found.ID, "J-1", kbSfAt, "The token is written to the log in clear.\nIt stays in the log for ever."))
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

func TestKbSfJobBodyIsOneLine(t *testing.T) {
	long := strings.Repeat("a very long finding ", 40) + "\n\tend"
	body := SecurityFoundJobBody("J-5", "01ARZ3NDEKTSV4RRFFQ69G5FB0", "J-3", kbSfAt, long)
	title, _ := body["title"].(string)
	if !strings.HasPrefix(title, "Security finding on J-3: a very long finding") || strings.ContainsAny(title, "\n\r\t") || len([]rune(title)) > len("Security finding on J-3: ")+kbSfTitleMax {
		t.Fatalf("title = %q", title)
	}
	data, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	if err := contract.Validate("job-record", data); err != nil {
		t.Fatalf("not a job-record: %v", err)
	}
}

func TestKbSfSecondTickPostsNothing(t *testing.T) {
	f := setup(t)
	f.job(t, "J-1", nil)
	first := kbSfFound(t, f, "J-1", "security", "first finding")
	f.tick(t)
	kbSfWant(t, f, "after the first tick", "J-1", "J-2")
	for i := 0; i < 3; i++ {
		f.tick(t)
	}
	kbSfWant(t, f, "after three more ticks", "J-1", "J-2")
	// A restart: a new Store on the same folder reads the Feed, not memory.
	restarted := kbSfRestart(f)
	if _, err := restarted.Tick(); err != nil {
		t.Fatal(err)
	}
	bodies := kbSfWant(t, f, "after a restart", "J-1", "J-2")
	if text := kbSfA1(t, bodies["J-2"]); !strings.Contains(text, first.ID) {
		t.Fatalf("J-2 A1 %q does not hold the first found id %s", text, first.ID)
	}
	// A second security found on the same Job is its own finding and gets its own Job.
	second := kbSfFound(t, f, "J-1", "security", "second finding")
	f.tick(t)
	bodies = kbSfWant(t, f, "after the second found", "J-1", "J-2", "J-3")
	if text := kbSfA1(t, bodies["J-3"]); !strings.Contains(text, second.ID) || strings.Contains(text, first.ID) {
		t.Fatalf("J-3 A1 %q must hold %s and not %s", text, second.ID, first.ID)
	}
	if text := kbSfA1(t, bodies["J-2"]); strings.Contains(text, second.ID) {
		t.Fatalf("J-2 A1 %q holds the second found id", text)
	}
	f.tick(t)
	if _, err := kbSfRestart(f).Tick(); err != nil {
		t.Fatal(err)
	}
	kbSfWant(t, f, "after both founds were posted", "J-1", "J-2", "J-3")
}

func TestKbSfOnlySecurity(t *testing.T) {
	f := setup(t)
	f.job(t, "J-1", nil)
	kbSfFound(t, f, "J-1", "bug", "a bug, not a security finding")
	kbSfFound(t, f, "J-9", "security", "a security finding on a Job the task does not hold")
	f.tick(t)
	kbSfWant(t, f, "after a bug and an unknown Job", "J-1")
	// A found whose body is a ref, not read yet, is a wake in the fold but holds no kind.
	snap, err := Load(f.s.Feed)
	if err != nil {
		t.Fatal(err)
	}
	const refID = "01ARZ3NDEKTSV4RRFFQ69G5FB1"
	snap.Records[refID] = feed.Record{ID: refID, Type: "found", Body: feed.Body{"ref": "bodies/" + refID + ".json"}}
	snap.State.Wakes = append(snap.State.Wakes, refID)
	if err := f.s.kbSecurityFound(snap); err != nil {
		t.Fatal(err)
	}
	kbSfWant(t, f, "after a found with a ref body", "J-1")
	// The same tick does post for a security found on J-1, and only for that one.
	real := kbSfFound(t, f, "J-1", "security", "a real security finding")
	f.tick(t)
	bodies := kbSfWant(t, f, "after a security found on J-1", "J-1", "J-2")
	if text := kbSfA1(t, bodies["J-2"]); !strings.Contains(text, real.ID) {
		t.Fatalf("J-2 A1 %q does not hold %s", text, real.ID)
	}
}

func TestKbSfReplanStillPosts(t *testing.T) {
	f := setup(t)
	f.job(t, "J-1", nil)
	f.job(t, "J-2", []any{"J-1"})
	found := kbSfFound(t, f, "J-1", "security", "a security finding on J-1")
	f.tick(t)
	bodies := kbSfWant(t, f, "after the security found", "J-1", "J-2", "J-3")
	if text := kbSfA1(t, bodies["J-3"]); !strings.Contains(text, found.ID) {
		t.Fatalf("J-3 A1 %q does not hold %s", text, found.ID)
	}
	guRjReject(t, f, "J-1")
	f.tick(t)
	bodies = kbSfWant(t, f, "after J-1 was rejected", "J-1", "J-2", "J-3", "J-4")
	replan := bodies["J-4"]
	if replan["role"] != "orchestrator" || replan["discovered_from"] != "J-1" {
		t.Fatalf("J-4 role %v, discovered_from %v, want the re-plan of J-1", replan["role"], replan["discovered_from"])
	}
	if title := replan["title"]; title != "Re-plan after J-1 was rejected: J-2 wait on it" {
		t.Fatalf("J-4 title = %v", title)
	}
	if bodies["J-3"]["discovered_from"] != nil {
		t.Fatalf("J-3 discovered_from = %v, want null", bodies["J-3"]["discovered_from"])
	}
	// Each of the two is posted once.
	f.tick(t)
	kbSfWant(t, f, "after one more tick", "J-1", "J-2", "J-3", "J-4")
}
