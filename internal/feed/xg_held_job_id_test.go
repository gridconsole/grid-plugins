package feed

import (
	"errors"
	"testing"
)

// xgPostHeldJob posts a job record that waits for a record nobody has posted,
// and checks that Read shows it held. It returns the stored record and the id it
// waits for.
func xgPostHeldJob(t *testing.T, s Store, w Writer, id, title string) (Record, string) {
	t.Helper()
	missing, err := NewID()
	if err != nil {
		t.Fatal(err)
	}
	rec := mustPost(t, s, xgHeldInput(t, "", id, title, missing), w)
	res, err := s.Read(Filter{Type: "job"})
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range res.Records {
		if e.ID == rec.ID {
			if e.Verdict != "held" || len(e.Waiting) != 1 || e.Waiting[0] != missing {
				t.Fatalf("%s: verdict %q waiting %v, want held on %s", id, e.Verdict, e.Waiting, missing)
			}
			return rec, missing
		}
	}
	t.Fatalf("%s: record %s not on the Feed", id, rec.ID)
	return rec, missing
}

// xgHeldInput is a job post for Job id id that waits for record after. A fresh
// body map is built each call, so a retry is built the way a client builds it.
func xgHeldInput(t *testing.T, recordID, id, title, after string) Input {
	t.Helper()
	return Input{ID: recordID, Type: "job", Job: id, Body: icjJob(t, id, title), After: []string{after}}
}

// xgWantJobExists checks err is the Job id refusal naming record post.
func xgWantJobExists(t *testing.T, err error, jobID, post string) {
	t.Helper()
	if !errors.Is(err, ErrJobExists) || !errors.Is(err, ErrInvalidRecord) {
		t.Fatalf("post = %v, want ErrJobExists and ErrInvalidRecord", err)
	}
	var exists *JobExistsError
	if !errors.As(err, &exists) || exists.Job != jobID || exists.Post != post {
		t.Fatalf("post = %v, want JobExistsError for %s naming post %s", err, jobID, post)
	}
}

// TestXgHeldJobIDRefused, TestXgHeldRetryAllowed and TestXgHeldThenLands are one
// story told in three steps on one Feed: a held J-7, a second writer refused, the
// same request retried, then the awaited record landing.
func TestXgHeldJobIDRefused(t *testing.T) {
	s := testStore(t)
	held, _ := xgPostHeldJob(t, s, testWriter(), "J-7", "Seven A")
	_, err := s.Post(Input{Type: "job", Job: "J-7", Body: icjJob(t, "J-7", "Seven B")}, icjOtherWriter())
	xgWantJobExists(t, err, "J-7", held.ID)
	if got := icjJobRecords(t, s, "J-7"); len(got) != 1 || got[0].ID != held.ID {
		t.Fatalf("J-7 records = %+v, want only the held one", got)
	}
}

func TestXgHeldRetryAllowed(t *testing.T) {
	s := testStore(t)
	w := testWriter()
	held, missing := xgPostHeldJob(t, s, w, "J-7", "Seven A")
	again, err := s.Post(xgHeldInput(t, held.ID, "J-7", "Seven A", missing), w)
	if err != nil || again.ID != held.ID || again.Seq != held.Seq || again.At != held.At {
		t.Fatalf("same request: %+v %v, want the held record %s", again, err, held.ID)
	}
	// The same Job id with a new record id is a second record, not a retry.
	_, err = s.Post(xgHeldInput(t, "", "J-7", "Seven A", missing), w)
	xgWantJobExists(t, err, "J-7", held.ID)
	if got := icjJobRecords(t, s, "J-7"); len(got) != 1 || got[0].ID != held.ID {
		t.Fatalf("J-7 records = %+v, want only the held one", got)
	}
}

func TestXgHeldThenLands(t *testing.T) {
	s := testStore(t)
	w1, w2 := testWriter(), icjOtherWriter()
	held, missing := xgPostHeldJob(t, s, w1, "J-7", "Seven A")
	_, err := s.Post(Input{Type: "job", Job: "J-7", Body: icjJob(t, "J-7", "Seven B")}, w2)
	xgWantJobExists(t, err, "J-7", held.ID)
	if _, err := s.Post(xgHeldInput(t, held.ID, "J-7", "Seven A", missing), w1); err != nil {
		t.Fatal(err)
	}
	late := message("the awaited record")
	late.ID = missing
	mustPost(t, s, late, w1)
	got := icjJobRecords(t, s, "J-7")
	if len(got) != 1 || got[0].ID != held.ID || str(got[0].Body["title"]) != "Seven A" {
		t.Fatalf("J-7 records = %+v, want only A's", got)
	}
	if got[0].Verdict != "counted" || len(got[0].Waiting) != 0 {
		t.Fatalf("A's record: verdict %q waiting %v, want counted", got[0].Verdict, got[0].Waiting)
	}
}

func TestXgCountedStillRefused(t *testing.T) {
	s := testStore(t)
	counted := mustPost(t, s, Input{Type: "job", Job: "J-8", Body: icjJob(t, "J-8", "Eight A")}, testWriter())
	missing, err := NewID()
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.Post(xgHeldInput(t, "", "J-8", "Eight B", missing), icjOtherWriter())
	xgWantJobExists(t, err, "J-8", counted.ID)
	if got := icjJobRecords(t, s, "J-8"); len(got) != 1 || got[0].ID != counted.ID {
		t.Fatalf("J-8 records = %+v, want only the counted one", got)
	}
}

// A second held record for a held Job id is refused too: two held records would
// race to be the one the fold keeps once both dependencies land.
func TestXgHeldVsHeldRefused(t *testing.T) {
	s := testStore(t)
	held, _ := xgPostHeldJob(t, s, testWriter(), "J-9", "Nine A")
	other, err := NewID()
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.Post(xgHeldInput(t, "", "J-9", "Nine B", other), icjOtherWriter())
	xgWantJobExists(t, err, "J-9", held.ID)
	if got := icjJobRecords(t, s, "J-9"); len(got) != 1 || got[0].ID != held.ID {
		t.Fatalf("J-9 records = %+v, want only the first held one", got)
	}
}
