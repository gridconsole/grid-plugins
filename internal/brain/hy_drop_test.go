package brain

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
)

var hyDrAna = Origin{Kind: "person", ID: "ana"}

// hyDrNote accepts the sample entry and returns the store, the note's path and
// its bytes.
func hyDrNote(t *testing.T) (Store, string, []byte) {
	t.Helper()
	s := testStore(t)
	accepted(t, s, sample())
	path := filepath.Join(s.GridHome, "projects", s.Project, "knowledge", "conventions", "checks.md")
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return s, path, b
}

func hyDrEntries(t *testing.T, s Store) int {
	t.Helper()
	idx, err := s.Index()
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range idx.Topics {
		if row.Topic == "conventions" {
			return row.Entries
		}
	}
	t.Fatal("no conventions row")
	return 0
}

func hyDrHits(t *testing.T, s Store) int {
	t.Helper()
	hits, err := s.Search("checks", "", 5)
	if err != nil {
		t.Fatal(err)
	}
	return len(hits)
}

func TestHyDrDropAndRestore(t *testing.T) {
	s, path, before := hyDrNote(t)
	rev := NoteRev(before)
	versions := versionsOf(t, s, "conventions", "checks")
	if hyDrHits(t, s) != 1 || hyDrEntries(t, s) != 1 {
		t.Fatal("the accepted entry is not listed before the drop")
	}
	res, err := s.Drop("project", "conventions", "checks", OpDrop, rev, hyDrAna, "out of date")
	if err != nil || res.Status != "dropped" || res.Rev != rev || res.Version != versions[len(versions)-1].N {
		t.Fatalf("drop: %+v, %v", res, err)
	}
	after, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(after, before) {
		t.Fatalf("the note changed: %v", err)
	}
	if got := versionsOf(t, s, "conventions", "checks"); !reflect.DeepEqual(got, versions) {
		t.Fatalf("versions %+v, want %+v", got, versions)
	}
	if hyDrHits(t, s) != 0 || hyDrEntries(t, s) != 0 {
		t.Fatal("a dropped entry is still listed")
	}
	e, err := s.Show("brain:project/checks")
	if err != nil || !e.Dropped {
		t.Fatalf("show: %+v, %v", e, err)
	}
	c := newestChange(t, s)
	if c.Status != "dropped" || c.OriginKind != "person" || c.OriginID != "ana" || c.Reason != "out of date" ||
		c.Topic != "conventions" || c.Name != "checks" || c.Version != res.Version || c.Hash != auditHash(before) {
		t.Fatalf("change: %+v", c)
	}

	res, err = s.Drop("project", "conventions", "checks", OpRestore, rev, hyDrAna, "")
	if err != nil || res.Status != "restored" {
		t.Fatalf("restore: %+v, %v", res, err)
	}
	if hyDrHits(t, s) != 1 || hyDrEntries(t, s) != 1 {
		t.Fatal("a restored entry is not listed")
	}
	if e, err := s.Show("brain:project/checks"); err != nil || e.Dropped {
		t.Fatalf("show after restore: %+v, %v", e, err)
	}
	if c := newestChange(t, s); c.Status != "restored" || c.OriginID != "ana" {
		t.Fatalf("change: %+v", c)
	}

	// A drop straight after a restore still writes its own line, even in one millisecond.
	if _, err := s.Drop("project", "conventions", "checks", OpDrop, rev, hyDrAna, "out of date"); err != nil {
		t.Fatal(err)
	}
	if n := strings.Count(readLog(t, s), "| dropped |"); n != 2 {
		t.Fatalf("%d dropped lines, want 2:\n%s", n, readLog(t, s))
	}
	if hyDrHits(t, s) != 0 {
		t.Fatal("the second drop did not hide the entry")
	}
}

func TestHyDrRefusals(t *testing.T) {
	s, path, before := hyDrNote(t)
	rev := NoteRev(before)
	log := readLog(t, s)
	check := func(name string, err error, want error) {
		t.Helper()
		if !errors.Is(err, want) {
			t.Fatalf("%s: %v, want %v", name, err, want)
		}
		if after, _ := os.ReadFile(path); !bytes.Equal(after, before) {
			t.Fatalf("%s changed the note", name)
		}
		if readLog(t, s) != log {
			t.Fatalf("%s changed the log", name)
		}
	}
	_, err := s.Drop("project", "conventions", "checks", OpDrop, "0000000000000000", hyDrAna, "")
	check("stale rev", err, ErrConflict)
	var conflict *ConflictError
	if !errors.As(err, &conflict) || conflict.Rev != rev {
		t.Fatalf("conflict %v does not carry rev %s", err, rev)
	}
	_, err = s.Drop("project", "conventions", "nothing", OpDrop, rev, hyDrAna, "")
	check("missing note", err, ErrNoSuchNote)
	_, err = s.Drop("project", "conventions", "checks", OpRestore, rev, hyDrAna, "")
	check("restore of a note never dropped", err, ErrDropRefused)
	for _, topic := range []string{"machines", "skills"} {
		_, err = s.Drop("project", topic, "checks", OpDrop, rev, hyDrAna, "")
		check(topic, err, ErrDropRefused)
	}
	_, err = s.Drop("project", "conventions", "TOPIC", OpDrop, rev, hyDrAna, "")
	check("TOPIC", err, ErrDropRefused)

	// A superseded entry cannot be dropped.
	n, err := Parse(before)
	if err != nil {
		t.Fatal(err)
	}
	n.SupersededBy = "brain:project/other"
	b, err := Encode(n)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, b, 0o600); err != nil {
		t.Fatal(err)
	}
	before = b
	_, err = s.Drop("project", "conventions", "checks", OpDrop, NoteRev(b), hyDrAna, "")
	check("superseded", err, ErrDropRefused)
	if err := os.WriteFile(path, before, 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestHyDrDropTwiceIsRefused(t *testing.T) {
	s, _, before := hyDrNote(t)
	rev := NoteRev(before)
	if _, err := s.Drop("project", "conventions", "checks", OpDrop, rev, hyDrAna, ""); err != nil {
		t.Fatal(err)
	}
	log := readLog(t, s)
	_, err := s.Drop("project", "conventions", "checks", OpDrop, rev, hyDrAna, "")
	var refused *RefusedError
	if !errors.Is(err, ErrDropRefused) || !errors.As(err, &refused) || refused.Reason != "note is already dropped" {
		t.Fatalf("second drop: %v", err)
	}
	if readLog(t, s) != log {
		t.Fatal("the refused drop changed the log")
	}
}

func TestHyDrConcurrent(t *testing.T) {
	s, _, before := hyDrNote(t)
	rev := NoteRev(before)
	logBefore := strings.Count(readLog(t, s), "| dropped |")
	start := make(chan struct{})
	var wg sync.WaitGroup
	errs := make([]error, 2)
	for i := range errs {
		root, err := s.scope("project", false)
		if err != nil {
			t.Fatal(err)
		}
		defer root.Close()
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			_, errs[i] = DropWith(root, "conventions", "checks", OpDrop, rev, hyDrAna, "")
		}()
	}
	close(start)
	wg.Wait()
	nilCount, refused := 0, 0
	for _, err := range errs {
		switch {
		case err == nil:
			nilCount++
		case errors.Is(err, ErrDropRefused):
			refused++
		default:
			t.Fatalf("unexpected error: %v", err)
		}
	}
	if nilCount != 1 || refused != 1 {
		t.Fatalf("%d succeeded and %d refused, want 1 and 1: %v", nilCount, refused, errs)
	}
	if n := strings.Count(readLog(t, s), "| dropped |") - logBefore; n != 1 {
		t.Fatalf("%d new dropped lines, want 1", n)
	}
}
