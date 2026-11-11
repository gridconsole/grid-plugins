package feed

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// jwRiNewID is a caller-chosen record id no other record has.
const jwRiNewID = "01ARZ3NDEKTSV4RRFFQ69G5FAX"

// jwRiOther is a second writer tenure on the same Feed.
func jwRiOther() Writer {
	w := testWriter()
	w.Worker = "other@" + testMachine
	w.Author.ID = "other"
	return w
}

// jwRiFile returns the bytes of the tenure file of w.
func jwRiFile(t *testing.T, s Store, w Writer) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(s.Dir(), writerFile(w.Worker, w.Fence)))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// jwRiUsed fails unless err is the record_id_used refusal of id.
func jwRiUsed(t *testing.T, err error, id string) {
	t.Helper()
	var used *RecordIDUsedError
	var invalid *InvalidRecordError
	if !errors.As(err, &used) || used.ID != id {
		t.Fatalf("want RecordIDUsedError for %s, got %v", id, err)
	}
	if !errors.Is(err, ErrInvalidRecord) || !errors.As(err, &invalid) {
		t.Fatalf("record_id_used is not an invalid record rejection (exit 3): %v", err)
	}
	if err.Error() != "record id already used for a different post" {
		t.Fatalf("text = %q", err.Error())
	}
}

// Row 1: a record of another type with the id of an existing record is refused
// and the Feed keeps its bytes.
func TestJwRiOtherTypeRefused(t *testing.T) {
	s := testStore(t)
	w := testWriter()
	first := mustPost(t, s, message("first"), w)
	before := jwRiFile(t, s, w)
	in := Input{ID: first.ID, Type: "knowledge", Body: map[string]any{"title": "Example", "text": "note"}}
	if r, err := s.Post(in, jwRiOther()); err == nil {
		t.Fatalf("second record with id %s accepted as %s %s", first.ID, r.Type, r.ID)
	} else {
		jwRiUsed(t, err, first.ID)
	}
	if after := jwRiFile(t, s, w); !bytes.Equal(before, after) {
		t.Fatalf("Feed file changed:\n%s\n%s", before, after)
	}
	other, err := os.ReadFile(filepath.Join(s.Dir(), writerFile(jwRiOther().Worker, 0)))
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		t.Fatal(err)
	}
	if len(other) != 0 {
		t.Fatalf("refused post wrote %q", other)
	}
	got, err := s.ReadOnly(Filter{})
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Records) != 1 || got.Records[0].Type != "post" {
		t.Fatalf("records after refusal: %+v", got.Records)
	}
}

// Row 2: the same writer tenure keeps its retry and its 287 refusal; another
// tenure repeating the same request is refused with record_id_used.
func TestJwRiTenure(t *testing.T) {
	s := testStore(t)
	w := testWriter()
	first := mustPost(t, s, message("first"), w)

	in := message("first")
	in.ID = first.ID
	again, err := s.Post(in, w)
	if err != nil || again.ID != first.ID || again.Seq != first.Seq {
		t.Fatalf("same tenure retry: %+v %v", again, err)
	}

	changed := message("changed")
	changed.ID = first.ID
	_, err = s.Post(changed, w)
	var used *RecordIDUsedError
	if !errors.Is(err, ErrInvalidRecord) || errors.As(err, &used) || err.Error() != "record id already used for a different post" {
		t.Fatalf("same tenure different request: %v", err)
	}

	_, err = s.Post(in, jwRiOther())
	jwRiUsed(t, err, first.ID)
}

// Row 3: new ids are accepted from every tenure.
func TestJwRiNewIDs(t *testing.T) {
	s := testStore(t)
	mustPost(t, s, message("first"), testWriter())
	mustPost(t, s, message("second"), jwRiOther())
	in := Input{ID: jwRiNewID, Type: "knowledge", Body: map[string]any{"title": "Example", "text": "note"}}
	if r, err := s.Post(in, jwRiOther()); err != nil || r.ID != jwRiNewID {
		t.Fatalf("%+v %v", r, err)
	}
	got, err := s.ReadOnly(Filter{})
	if err != nil || len(got.Records) != 3 {
		t.Fatalf("records: %d %v", len(got.Records), err)
	}
}
