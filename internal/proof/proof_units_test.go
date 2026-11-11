package proof

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/plan"
)

func TestLevelMapsEachProofKind(t *testing.T) {
	for _, tc := range []struct{ proof, want string }{
		{"", "check"},
		{"check", "check"},
		{"log", "log"},
		{"screenshot", "screenshot"},
		{"video", "video"},
	} {
		got, err := Level(plan.Item{ID: "i-1", Proof: tc.proof})
		if err != nil || got != tc.want {
			t.Errorf("Level(%q) = %q, %v; want %q", tc.proof, got, err, tc.want)
		}
	}
}

func TestLevelUnknownKindErrors(t *testing.T) {
	for _, bad := range []string{"magic", "Screenshot", " log", "audio"} {
		got, err := Level(plan.Item{ID: "i-7", Proof: bad})
		if err == nil || got != "" {
			t.Errorf("Level(%q) = %q, %v; want an error and no level", bad, got, err)
			continue
		}
		if !strings.Contains(err.Error(), "i-7") || !strings.Contains(err.Error(), "unknown proof level") {
			t.Errorf("Level(%q) error %q should name the item and the level", bad, err)
		}
	}
}

func TestReadEvidenceRegularFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "note.txt")
	if err := os.WriteFile(path, []byte("evidence bytes"), 0600); err != nil {
		t.Fatal(err)
	}
	b, err := ReadEvidence(path)
	if err != nil || string(b) != "evidence bytes" {
		t.Fatalf("got %q, %v", b, err)
	}
}

func TestReadEvidenceMissingAndDirectory(t *testing.T) {
	dir := t.TempDir()
	if _, err := ReadEvidence(filepath.Join(dir, "absent.txt")); !os.IsNotExist(err) {
		t.Errorf("missing file: want a not-exist error, got %v", err)
	}
	if _, err := ReadEvidence(dir); err == nil || !strings.Contains(err.Error(), "regular file") {
		t.Errorf("directory: want a regular-file error, got %v", err)
	}
}

func TestReadEvidenceTooLarge(t *testing.T) {
	path := filepath.Join(t.TempDir(), "big.log")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err = f.Truncate(MaxBytes + 1); err != nil {
		f.Close()
		t.Skip(err)
	}
	f.Close()
	if _, err = ReadEvidence(path); err == nil || !strings.Contains(err.Error(), "20 MiB") {
		t.Fatalf("want a size error, got %v", err)
	}
}

// A symlink pointing outside the task is refused by ReadEvidence, and Add with
// that file as its source writes nothing into the task, as TestSymlinkProof
// expects of a symlinked proof folder.
func TestReadEvidenceSymlinkOutsideTask(t *testing.T) {
	s := fixture(t)
	outside := filepath.Join(t.TempDir(), "secret.txt")
	if err := os.WriteFile(outside, []byte("outside"), 0600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(t.TempDir(), "link.txt")
	if err := os.Symlink(outside, link); err != nil {
		t.Skip(err)
	}
	if _, err := ReadEvidence(link); err == nil {
		t.Fatal("ReadEvidence followed a symlink")
	}
	if _, err := s.Add(Input{Item: "1", File: link, Kind: "log", By: "grid"}); err == nil {
		t.Fatal("Add followed a symlink source")
	}
	if _, err := os.Stat(filepath.Join(s.TaskDir, "proof", "i-1", "proof.json")); err == nil {
		t.Fatal("Add left a record after a refused symlink")
	}
	idx, err := s.Build()
	if err != nil {
		t.Fatal(err)
	}
	if err = idx.Check(); err == nil {
		t.Fatal("refused symlink counted as proof")
	}
}

func TestRebuildAfterDeletingIndexMatchesBuild(t *testing.T) {
	s := fixture(t)
	if _, err := s.Add(Input{Item: "1", Text: "checks pass", By: "grid", Kind: "log"}); err != nil {
		t.Fatal(err)
	}
	shot := filepath.Join(t.TempDir(), "image.png")
	if err := os.WriteFile(shot, []byte("image bytes"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Add(Input{Item: "2", File: shot, Kind: "screenshot", By: "grid"}); err != nil {
		t.Fatal(err)
	}
	first, err := s.Rebuild()
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(s.TaskDir, "proof", "index.json")
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.Remove(path); err != nil {
		t.Fatal(err)
	}
	built, err := s.Build()
	if err != nil {
		t.Fatal(err)
	}
	if _, err = os.Stat(path); err == nil {
		t.Fatal("Build wrote the index")
	}
	again, err := s.Rebuild()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(first, again) || !reflect.DeepEqual(built, again) {
		t.Fatalf("index differs:\nfirst %+v\nbuild %+v\nrebuild %+v", first, built, again)
	}
	after, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatalf("rebuilt index file differs: %v", err)
	}
}

func TestIndexCheck(t *testing.T) {
	full := Index{Items: []Item{{ID: "i-1", Level: "check", Proven: true}, {ID: "i-2", Level: "screenshot", Proven: true}}}
	if err := full.Check(); err != nil {
		t.Errorf("full index: %v", err)
	}
	if err := (Index{}).Check(); err == nil || !strings.Contains(err.Error(), "no items") {
		t.Errorf("empty index: %v", err)
	}
	missing := Index{Items: []Item{{ID: "i-1", Level: "check", Proven: true}, {ID: "i-2", Level: "screenshot"}}}
	err := missing.Check()
	if err == nil || !strings.Contains(err.Error(), "i-2 lacks screenshot proof") || strings.Contains(err.Error(), "i-1") {
		t.Errorf("missing proof: want i-2 named, got %v", err)
	}
}

func TestIndexCheckOnBuiltIndex(t *testing.T) {
	s := fixture(t)
	if _, err := s.Add(Input{Item: "1", Text: "pass", By: "grid", Kind: "log"}); err != nil {
		t.Fatal(err)
	}
	idx, err := s.Build()
	if err != nil {
		t.Fatal(err)
	}
	if err = idx.Check(); err == nil || !strings.Contains(err.Error(), "i-2 lacks screenshot proof") {
		t.Fatalf("want i-2 named, got %v", err)
	}
}
