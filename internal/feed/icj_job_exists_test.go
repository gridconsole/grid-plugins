package feed

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/fslock"
)

// icjJob is the valid Job record fixture with its Job id and title set.
func icjJob(t *testing.T, id, title string) map[string]any {
	t.Helper()
	b, err := os.ReadFile("../../contract/fixtures/job-record/valid/basic.json")
	if err != nil {
		t.Fatal(err)
	}
	var body map[string]any
	if err := json.Unmarshal(b, &body); err != nil {
		t.Fatal(err)
	}
	body["id"], body["title"] = id, title
	return body
}

// icjOtherWriter is a second writer: another worker and another author.
func icjOtherWriter() Writer {
	w := testWriter()
	w.Author.ID = "sam"
	w.Worker = "sam@" + testMachine
	return w
}

// icjJobRecords returns the job records on the Feed that carry Job id id.
func icjJobRecords(t *testing.T, s Store, id string) []Entry {
	t.Helper()
	res, err := s.Read(Filter{Type: "job"})
	if err != nil {
		t.Fatal(err)
	}
	var out []Entry
	for _, e := range res.Records {
		if str(e.Body["id"]) == id {
			out = append(out, e)
		}
	}
	return out
}

func TestIcjJobExistsRefused(t *testing.T) {
	s := testStore(t)
	w1, w2 := testWriter(), icjOtherWriter()
	firstID, err := NewID()
	if err != nil {
		t.Fatal(err)
	}
	first, err := s.Post(Input{ID: firstID, Type: "job", Job: "J-2", Body: icjJob(t, "J-2", "Second A")}, w1)
	if err != nil {
		t.Fatal(err)
	}
	for _, leg := range []struct {
		name  string
		w     Writer
		title string
	}{{"w1 body B new record id", w1, "Second B"}, {"w2 body B", w2, "Second B"}, {"w1 body A new record id", w1, "Second A"}} {
		_, err := s.Post(Input{Type: "job", Job: "J-2", Body: icjJob(t, "J-2", leg.title)}, leg.w)
		if !errors.Is(err, ErrJobExists) || !errors.Is(err, ErrInvalidRecord) {
			t.Fatalf("%s: post = %v, want ErrJobExists and ErrInvalidRecord", leg.name, err)
		}
	}
	again, err := s.Post(Input{ID: firstID, Type: "job", Job: "J-2", Body: icjJob(t, "J-2", "Second A")}, w1)
	if err != nil || again.ID != first.ID || again.Seq != first.Seq || again.At != first.At {
		t.Fatalf("same request: %+v %v, want the first record %s", again, err, first.ID)
	}
	if got := icjJobRecords(t, s, "J-2"); len(got) != 1 || got[0].ID != first.ID || str(got[0].Body["title"]) != "Second A" {
		t.Fatalf("J-2 records = %+v, want only the first", got)
	}

	// A Feed that already holds two J-9 records (written before this check)
	// still reads; a third J-9 post is refused.
	bad := testStore(t)
	if err := os.MkdirAll(bad.Dir(), 0o700); err != nil {
		t.Fatal(err)
	}
	var seq Record
	for _, title := range []string{"Nine A", "Nine B"} {
		r, err := bad.record(Input{Type: "job", Job: "J-9", Body: icjJob(t, "J-9", title)}, w1)
		if err != nil {
			t.Fatal(err)
		}
		seq.Seq++
		r.Seq = seq.Seq
		rawAppend(t, bad, r)
	}
	if got := icjJobRecords(t, bad, "J-9"); len(got) != 2 {
		t.Fatalf("J-9 records = %d, want both raw records", len(got))
	}
	if _, err := bad.Post(Input{Type: "job", Job: "J-9", Body: icjJob(t, "J-9", "Nine C")}, w2); !errors.Is(err, ErrJobExists) {
		t.Fatalf("third J-9 post = %v, want ErrJobExists", err)
	}
	if got := icjJobRecords(t, bad, "J-9"); len(got) != 2 {
		t.Fatalf("J-9 records = %d after the refused post, want 2", len(got))
	}
}

func TestIcjOneWinner(t *testing.T) {
	// Two Stores on one task dir, two writers, released together.
	s := testStore(t)
	other := Store{TaskDir: s.TaskDir, TaskID: s.TaskID}
	bodyA, bodyB := icjJob(t, "J-3", "Three A"), icjJob(t, "J-3", "Three B")
	start := make(chan struct{})
	errs := make(chan error, 2)
	post := func(st Store, w Writer, body map[string]any) {
		<-start
		_, err := st.Post(Input{Type: "job", Job: "J-3", Body: body}, w)
		errs <- err
	}
	go post(s, testWriter(), bodyA)
	go post(other, icjOtherWriter(), bodyB)
	close(start)
	won, refused := 0, 0
	for i := 0; i < 2; i++ {
		switch err := <-errs; {
		case err == nil:
			won++
		case errors.Is(err, ErrJobExists):
			refused++
		default:
			t.Fatalf("post: %v", err)
		}
	}
	if won != 1 || refused != 1 {
		t.Fatalf("won=%d refused=%d, want one each", won, refused)
	}
	if got := icjJobRecords(t, s, "J-3"); len(got) != 1 {
		t.Fatalf("J-3 records = %d, want 1", len(got))
	}

	// The job lock is held between the check and the append.
	p := testStore(t)
	if err := os.MkdirAll(filepath.Join(p.Dir(), ".cache"), 0o700); err != nil {
		t.Fatal(err)
	}
	probe, err := fslock.OpenNoFollow(filepath.Join(p.Dir(), ".cache", "job.lock"), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	defer probe.Close()
	checked, resume := make(chan struct{}), make(chan struct{})
	p.afterJobCheck = func() { close(checked); <-resume }
	bodyC, bodyD := icjJob(t, "J-4", "Four A"), icjJob(t, "J-4", "Four B")
	first := make(chan error, 1)
	go func() { _, err := p.Post(Input{Type: "job", Job: "J-4", Body: bodyC}, testWriter()); first <- err }()
	released := false
	defer func() {
		if !released {
			close(resume)
		}
	}()
	select {
	case <-checked:
	case err := <-first:
		t.Fatalf("post did not reach the job hook: %v", err)
	}
	if err := fslock.TryLockExclusive(probe); !errors.Is(err, fslock.ErrLocked) {
		t.Errorf("job lock not held between check and append: %v", err)
	}
	fslock.Unlock(probe)
	second := make(chan error, 1)
	go func() {
		_, err := Store{TaskDir: p.TaskDir, TaskID: p.TaskID}.Post(Input{Type: "job", Job: "J-4", Body: bodyD}, icjOtherWriter())
		second <- err
	}()
	close(resume)
	released = true
	if err := <-first; err != nil {
		t.Fatal(err)
	}
	if err := <-second; !errors.Is(err, ErrJobExists) {
		t.Fatalf("second J-4 post = %v, want ErrJobExists", err)
	}
	if got := icjJobRecords(t, p, "J-4"); len(got) != 1 || str(got[0].Body["title"]) != "Four A" {
		t.Fatalf("J-4 records = %+v, want only Four A", got)
	}
}
