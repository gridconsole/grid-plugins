package feed

import (
	"errors"
	"strings"
	"testing"
)

// A caller's body that is only {"ref":"bodies/..."} is refused at Post: the
// read drops one that names no stored body of its record, so the post would
// vanish. The Feed's own spill of a body over one line still round-trips.
func TestPostRefusesCallerRefBody(t *testing.T) {
	own, err := NewID()
	if err != nil {
		t.Fatal(err)
	}
	other, err := NewID()
	if err != nil {
		t.Fatal(err)
	}
	for name, in := range map[string]Input{
		"another id": {Type: "knowledge", Body: map[string]any{"ref": "bodies/" + other + ".json"}},
		"own id":     {ID: own, Type: "knowledge", Body: map[string]any{"ref": "bodies/" + own + ".json"}},
		"not a ulid": {Type: "knowledge", Body: map[string]any{"ref": "bodies/missing.json"}},
	} {
		t.Run(name, func(t *testing.T) {
			s := testStore(t)
			_, err := s.Post(in, testWriter())
			if !errors.Is(err, ErrInvalidRecord) || !strings.Contains(err.Error(), "must be inline") {
				t.Fatalf("post of a bare ref body: %v, want an invalid record that says the body must be inline", err)
			}
			read, err := s.Read(Filter{})
			if err != nil || len(read.Records) != 0 || len(read.Bad) != 0 {
				t.Fatalf("records %d bad %v err %v, want nothing written", len(read.Records), read.Bad, err)
			}
		})
	}
	t.Run("spilled body round-trips", func(t *testing.T) {
		s := testStore(t)
		in := largeKnowledge("Big")
		posted := mustPost(t, s, in, testWriter())
		again := in
		again.ID = posted.ID
		if retry := mustPost(t, s, again, testWriter()); retry.ID != posted.ID || str(retry.Body["text"]) != in.Body["text"] {
			t.Fatalf("retry of the spilled post: %s, body text %d bytes", retry.ID, len(str(retry.Body["text"])))
		}
		read, err := s.Read(Filter{})
		if err != nil || len(read.Records) != 1 {
			t.Fatalf("read: %d records, %v", len(read.Records), err)
		}
		if e := read.Records[0]; e.Verdict != "counted" || str(e.Body["text"]) != in.Body["text"] {
			t.Fatalf("verdict %q, body text %d bytes", e.Verdict, len(str(e.Body["text"])))
		}
	})
}
