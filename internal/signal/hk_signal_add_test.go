package signal

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/where"
)

// hkSigNote makes one open note in a fresh temp home and returns its id and path.
func hkSigNote(t *testing.T) (Store, string, string) {
	t.Helper()
	s := fixture(t)
	r, path := actNote(t, s, "hk-add")
	return s, r.ID, path
}

func TestHkSigAddUnknown(t *testing.T) {
	s, id, path := hkSigNote(t)
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.Apply(id, ActOptions{Act: "add", Task: "T-99"})
	var refused *RefusedError
	if !errors.As(err, &refused) {
		t.Fatalf("want *RefusedError, got %T %v", err, err)
	}
	if !errors.Is(refused.Cause, where.ErrTaskNotFound) {
		t.Fatalf("cause %v, want where.ErrTaskNotFound", refused.Cause)
	}
	if !strings.Contains(refused.Message, "T-99") {
		t.Fatalf("message %q does not name T-99", refused.Message)
	}
	after, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatalf("note changed after a refused add: %v", err)
	}
	if _, err := os.Stat(filepath.Join(s.GridHome, "projects", s.Project, "tasks")); !os.IsNotExist(err) {
		t.Fatalf("refused add created task state: %v", err)
	}
}

func TestHkSigAddKnown(t *testing.T) {
	s, id, path := hkSigNote(t)
	if err := os.MkdirAll(filepath.Join(s.GridHome, "projects", s.Project, "tasks", "T-7"), 0o700); err != nil {
		t.Fatal(err)
	}
	got, err := s.Apply(id, ActOptions{Act: "add", Task: "T-7"})
	if err != nil || got.Status != "done" || got.Became != "T-7" || got.Result != "updated" {
		t.Fatalf("%+v %v", got, err)
	}
	n, err := read(path)
	if err != nil || n.Status != "done" || n.Became != "T-7" {
		t.Fatalf("%+v %v", n, err)
	}
}
