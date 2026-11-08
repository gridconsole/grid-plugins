package feed

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
)

func gatedQuestion(id string) Input {
	return Input{Type: "question", Body: map[string]any{
		"question": "Approve release?", "who": "owner", "gate": map[string]any{"id": id, "kind": "person"},
	}}
}

func TestGateReuse(t *testing.T) {
	for _, existing := range []string{"ordinary", "spilled", "held"} {
		t.Run(existing, func(t *testing.T) {
			s, w := testStore(t), testWriter()
			in := gatedQuestion("Q-1")
			if existing == "spilled" {
				in.Body["question"] = strings.Repeat("x", MaxLine)
			}
			if existing == "held" {
				in.After = []string{"01ARZ3NDEKTSV4RRFFQ69G5FAX"}
			}
			first := mustPost(t, s, in, w)
			in.ID = first.ID
			if retry := mustPost(t, s, in, w); retry.ID != first.ID || retry.Seq != first.Seq {
				t.Fatalf("identical record retry changed: %+v", retry)
			}
			for _, change := range []string{"identical", "kind", "question", "who", "default", "job", "wake"} {
				for _, writer := range []Writer{w, {Author: Author{ID: "agent", Role: "member"}, Worker: "agent@" + testMachine}} {
					attempt := gatedQuestion("Q-1")
					attempt.Body["question"] = in.Body["question"]
					switch change {
					case "kind":
						attempt.Body["gate"] = map[string]any{"id": "Q-1", "kind": "timer", "at": "2000-01-01T00:00:00.000Z"}
					case "question", "who":
						attempt.Body[change] = "changed"
					case "default":
						attempt.Body["gate"].(map[string]any)["default"] = "Proceed"
					case "job":
						attempt.Job = "J-2"
					case "wake":
						attempt.Wake = first.ID + "/1"
					}
					if _, err := s.Post(attempt, writer); !errors.Is(err, ErrInvalidRecord) || !strings.Contains(err.Error(), "gate id Q-1 already exists") {
						t.Errorf("%s from %s: %v", change, writer.Worker, err)
					}
				}
			}
			r, err := s.Read(Filter{})
			if err != nil || len(r.Records) != 1 {
				t.Fatalf("refusals wrote records: %d %v", len(r.Records), err)
			}
			// Uniqueness is task-scoped and applies only to named gates.
			mustPost(t, testStore(t), gatedQuestion("Q-1"), w)
			mustPost(t, s, gatedQuestion("Q-2"), w)
			for range 2 {
				plain := gatedQuestion("")
				delete(plain.Body, "gate")
				mustPost(t, s, plain, w)
			}
		})
	}
}

func TestGateConcurrentWriters(t *testing.T) {
	s := testStore(t)
	const writers = 16
	start := make(chan struct{})
	results := make(chan error, writers)
	var wg sync.WaitGroup
	for i := range writers {
		wg.Go(func() {
			<-start
			w := testWriter()
			w.Author.ID = fmt.Sprintf("agent%d", i)
			w.Author.Role = "member"
			w.Worker = w.Author.ID + "@" + testMachine
			_, err := s.Post(gatedQuestion("Q-1"), w)
			results <- err
		})
	}
	close(start)
	wg.Wait()
	close(results)
	accepted := 0
	for err := range results {
		if err == nil {
			accepted++
		} else if !strings.Contains(err.Error(), "gate id Q-1 already exists") {
			t.Errorf("unexpected refusal: %v", err)
		}
	}
	r, err := s.Read(Filter{})
	if err != nil || accepted != 1 || len(r.Records) != 1 {
		t.Fatalf("accepted=%d records=%d err=%v", accepted, len(r.Records), err)
	}
}
