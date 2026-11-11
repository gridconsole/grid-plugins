package feed

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"testing"
	"time"
)

// questionsCachePersonGate is the body of a person-gated question addressed to
// "member" whose deadline is far away, so only controls and answers decide it.
func questionsCachePersonGate(gateID string, blocking bool) map[string]any {
	body := map[string]any{"question": "Proceed?", "who": "member", "gate": map[string]any{"id": gateID, "kind": "person", "default": "Wait", "at": "2099-01-01T00:00:00.000Z"}}
	if blocking {
		body["blocking"] = true
	}
	return body
}

func questionsCacheIDs(records []Record) string {
	ids := make([]string, 0, len(records))
	for _, r := range records {
		ids = append(ids, r.ID)
	}
	return strings.Join(ids, ",")
}

func TestQuestionsCachePersonAnswerRoles(t *testing.T) {
	for _, tc := range []struct {
		name string
		typ  string
		role string
		id   string
		want bool
	}{
		{"result is not an answer", "result", "owner", "p-1", false},
		{"owner", "answer", "owner", "p-1", true},
		{"member", "answer", "member", "p-1", true},
		{"lead", "answer", "lead", "p-1", true},
		{"guest named", "answer", "guest", "p-1", true},
		{"guest other", "answer", "guest", "p-2", false},
		{"worker", "answer", "worker", "p-1", false},
		{"empty role", "answer", "", "p-1", false},
		// NormalizeRole lowercases, trims and drops a "grid-" prefix.
		{"grid-owner", "answer", "grid-owner", "p-1", true},
		{"mixed case and spaces lead", "answer", " Lead ", "p-1", true},
		{"grid-guest named", "answer", "grid-guest", "p-1", true},
		{"grid-guest other", "answer", "grid-guest", "p-2", false},
		{"bare grid- prefix", "answer", "grid-", "p-1", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := Record{Type: tc.typ, Author: Author{ID: tc.id, Role: tc.role}}
			if got := PersonAnswer(p, "p-1"); got != tc.want {
				t.Fatalf("PersonAnswer(%s, role %q, id %q) = %v, want %v", tc.typ, tc.role, tc.id, got, tc.want)
			}
		})
	}
}

func TestQuestionsCacheAgedEvents(t *testing.T) {
	aged := func(id string, level int) Input {
		return Input{Type: "control", Re: id, Body: map[string]any{"target": id, "question_event": "aged", "text": "Priority raised", "level": level}}
	}
	for _, tc := range []struct {
		name       string
		blocking   bool
		levels     []int
		wantAging  int
		wantEvents int
	}{
		{"first level 1 on a blocking question", true, []int{1}, 1, 1},
		{"second level 1 is skipped", true, []int{1, 1}, 1, 1},
		{"level 1 on a non-blocking question is skipped", false, []int{1, 1}, 0, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := testStore(t)
			q := mustPost(t, s, Input{Type: "question", Body: questionsCachePersonGate("Q-1", tc.blocking)}, testWriter())
			task := taskWriter(t, s)
			for _, level := range tc.levels {
				mustPost(t, s, aged(q.ID, level), task)
			}
			read, err := s.Read(Filter{})
			if err != nil {
				t.Fatal(err)
			}
			_, decisions := FoldReadAt(read, time.Now())
			d := decisions[q.ID]
			if d.Aging != tc.wantAging || len(d.Events) != tc.wantEvents || d.State != "open" {
				t.Fatalf("aging=%d events=%d state=%s, want aging=%d events=%d state=open", d.Aging, len(d.Events), d.State, tc.wantAging, tc.wantEvents)
			}
		})
	}
}

func TestQuestionsCacheCancelledJob(t *testing.T) {
	s := testStore(t)
	task := taskWriter(t, s)
	mustPost(t, s, Input{Type: "job", Job: "J-1", Body: contractBody(t, "job", map[string]any{"title": "Example"})}, task)
	var ids []string
	for i := 0; i < 2; i++ {
		q := mustPost(t, s, Input{Type: "question", Job: "J-1", Body: questionsCachePersonGate(fmt.Sprintf("Q-%d", i+1), false)}, testWriter())
		ids = append(ids, q.ID)
	}
	sort.Strings(ids)
	fold := func() (map[string]bool, []string, []string) {
		read, err := s.Read(Filter{})
		if err != nil {
			t.Fatal(err)
		}
		state, _ := FoldReadAt(read, time.Now())
		open := map[string]bool{}
		for id, q := range state.Questions {
			open[id] = q.Open
		}
		return open, state.NeedsYou, state.Jobs["J-1"].OpenQuestions
	}
	open, needs, jobOpen := fold()
	if !slices.Equal(needs, ids) || !slices.Equal(jobOpen, ids) || !open[ids[0]] || !open[ids[1]] {
		t.Fatalf("live Job: needs=%v job=%v open=%v, want both of %v", needs, jobOpen, open, ids)
	}
	mustPost(t, s, Input{Type: "control", Body: map[string]any{"target": "J-1", "status": "cancelled", "reason": "scope withdrawn", "close_reason": "scope withdrawn"}}, task)
	open, needs, jobOpen = fold()
	if len(needs) != 0 || len(jobOpen) != 0 || open[ids[0]] || open[ids[1]] {
		t.Fatalf("cancelled Job: needs=%v job=%v open=%v, want none open", needs, jobOpen, open)
	}
}

func TestQuestionsCacheEventOrder(t *testing.T) {
	task := "task"
	ev := func(id string, lease *string, after ...string) Record {
		return Record{ID: id, Lease: lease, After: after}
	}
	for _, tc := range []struct {
		name   string
		events []Record
		want   string
	}{
		{"two task-lease events keep order", []Record{ev("a", &task), ev("b", &task)}, "a,b"},
		{"task-lease event waits for an earlier pending one", []Record{ev("a", &task, "c"), ev("b", &task), ev("c", nil)}, "c,a,b"},
		{"after waits for a later event", []Record{ev("a", nil, "b"), ev("b", nil)}, "b,a"},
		{"two-event cycle returns nothing", []Record{ev("a", nil, "b"), ev("b", nil, "a")}, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			done := make(chan []Record, 1)
			go func() { done <- questionEventOrder(tc.events) }()
			select {
			case got := <-done:
				if ids := questionsCacheIDs(got); ids != tc.want {
					t.Fatalf("order %q, want %q", ids, tc.want)
				}
			case <-time.After(5 * time.Second):
				t.Fatal("questionEventOrder did not return within 5s")
			}
		})
	}
}

func TestQuestionsCacheReaderRefusals(t *testing.T) {
	const invalid = "invalid reader cache; remove feed/.cache/reader.json to rebuild"
	cached := func(t *testing.T) (Store, string) {
		t.Helper()
		s := testStore(t)
		mustPost(t, s, message("example"), testWriter())
		if _, err := s.Read(Filter{}); err != nil {
			t.Fatal(err)
		}
		return s, filepath.Join(s.Dir(), ".cache", "reader.json")
	}
	for _, tc := range []struct{ name, content string }{
		{"not json", `{`},
		{"another version", `{"version":2,"task":"` + testTask + `","files":{}}`},
		{"another task", `{"version":1,"task":"01ARZ3NDEKTSV4RRFFQ69G5FAX","files":{}}`},
		{"files null", `{"version":1,"task":"` + testTask + `","files":null}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, path := cached(t)
			if err := os.WriteFile(path, []byte(tc.content), 0o600); err != nil {
				t.Fatal(err)
			}
			if _, err := s.Read(Filter{}); err == nil || err.Error() != invalid {
				t.Fatalf("got error %v, want %q", err, invalid)
			}
		})
	}
	t.Run("cached tenure file deleted", func(t *testing.T) {
		s, _ := cached(t)
		read, err := s.Read(Filter{})
		if err != nil || len(read.Records) != 1 {
			t.Fatalf("valid cache read: %v %d", err, len(read.Records))
		}
		name := read.Records[0].File
		if err := os.Remove(filepath.Join(s.Dir(), name)); err != nil {
			t.Fatal(err)
		}
		want := "cached tenure file is missing: " + name
		if _, err := s.Read(Filter{}); err == nil || err.Error() != want {
			t.Fatalf("got error %v, want %q", err, want)
		}
	})
}

func TestQuestionsCacheWireValid(t *testing.T) {
	s := testStore(t)
	mustPost(t, s, message("example"), testWriter())
	read, err := s.Read(Filter{})
	if err != nil || len(read.Records) != 1 {
		t.Fatalf("read: %v %d", err, len(read.Records))
	}
	data, err := os.ReadFile(filepath.Join(s.Dir(), read.Records[0].File))
	if err != nil {
		t.Fatal(err)
	}
	line := []byte(strings.TrimSpace(string(data)))
	var r Record
	if err := json.Unmarshal(line, &r); err != nil {
		t.Fatal(err)
	}
	if !wireValid(line, r, testTask) {
		t.Fatalf("stored line refused: %s", line)
	}
	if wireValid([]byte("not json"), r, testTask) {
		t.Fatal("a line that is not JSON was accepted")
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(line, &fields); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"id", "task", "type", "type_v", "author", "worker", "lease", "gen", "fence", "seq", "at", "re", "after", "job", "attempt", "wake", "body"} {
		t.Run("without "+key, func(t *testing.T) {
			cut := map[string]json.RawMessage{}
			for k, v := range fields {
				if k != key {
					cut[k] = v
				}
			}
			out, err := json.Marshal(cut)
			if err != nil {
				t.Fatal(err)
			}
			if wireValid(out, r, testTask) {
				t.Fatalf("line without %q was accepted", key)
			}
		})
	}
}
