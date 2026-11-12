package proof

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// proofAddBuildSource writes a source file for Add in its own folder.
func proofAddBuildSource(t *testing.T, name string, data []byte) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	return path
}

// proofAddBuildFolder returns the task's proof/<id> folder.
func proofAddBuildFolder(s Store, id string) string {
	return filepath.Join(s.TaskDir, "proof", id)
}

// proofAddBuildMetadata writes proof/<id>/proof.json by hand.
func proofAddBuildMetadata(t *testing.T, s Store, id string, content []byte) {
	t.Helper()
	if err := os.MkdirAll(proofAddBuildFolder(s, id), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(proofAddBuildFolder(s, id), "proof.json"), content, 0644); err != nil {
		t.Fatal(err)
	}
}

// proofAddBuildEvidence writes an evidence file next to proof.json and returns
// a valid record for it, with the right sha256.
func proofAddBuildEvidence(t *testing.T, s Store, id, name, content string) Record {
	t.Helper()
	if err := os.MkdirAll(proofAddBuildFolder(s, id), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(proofAddBuildFolder(s, id), name), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	return Record{
		Kind:   "log",
		By:     "grid",
		At:     "2026-01-02T03:04:05Z",
		SHA256: fmt.Sprintf("%x", sha256.Sum256([]byte(content))),
		File:   "proof/" + id + "/" + name,
	}
}

func proofAddBuildRecords(t *testing.T, s Store, id string, records ...Record) {
	t.Helper()
	b, err := json.Marshal(records)
	if err != nil {
		t.Fatal(err)
	}
	proofAddBuildMetadata(t, s, id, b)
}

// proofAddBuildNoWrite fails when proof/<id> holds anything.
func proofAddBuildNoWrite(t *testing.T, s Store, id string) {
	t.Helper()
	entries, err := os.ReadDir(proofAddBuildFolder(s, id))
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("proof/%s holds %d entries after a refused Add", id, len(entries))
	}
}

// Row 2: an unlisted extension is stored as .bin; an empty file is refused.
func TestProofAddBuildExtensionAndSize(t *testing.T) {
	if MaxBytes != 20<<20 {
		t.Fatalf("MaxBytes = %d, want 20 MiB", MaxBytes)
	}
	t.Run("unlisted extension is stored as bin", func(t *testing.T) {
		s := fixture(t)
		data := []byte("raw dump bytes")
		r, err := s.Add(Input{Item: "1", File: proofAddBuildSource(t, "x.dat", data), Kind: "log", By: "grid"})
		if err != nil {
			t.Fatal(err)
		}
		if !strings.HasSuffix(r.File, ".bin") {
			t.Fatalf("File = %q, want a .bin name", r.File)
		}
		stored, err := os.ReadFile(filepath.Join(s.TaskDir, filepath.FromSlash(r.File)))
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(stored, data) {
			t.Fatalf("stored %q, want %q", stored, data)
		}
	})
	t.Run("empty file is refused", func(t *testing.T) {
		s := fixture(t)
		_, err := s.Add(Input{Item: "1", File: proofAddBuildSource(t, "x.log", nil), Kind: "log", By: "grid"})
		if err == nil || err.Error() != "proof must contain 1 byte to 20 MiB" {
			t.Fatalf("got %v", err)
		}
		if _, err := os.Stat(filepath.Join(proofAddBuildFolder(s, "i-1"), "proof.json")); !os.IsNotExist(err) {
			t.Fatalf("proof.json after a refused empty file: %v", err)
		}
	})
}

// Row 3: the file type must match the screenshot and video kinds.
func TestProofAddBuildKindAndType(t *testing.T) {
	for _, tc := range []struct {
		name, kind, file, message string
	}{
		{"screenshot with txt", "screenshot", "shot.txt", "screenshot file must be png, jpg, webp or gif"},
		{"video with png", "video", "clip.png", "video file must be mp4, webm or gif"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := fixture(t)
			_, err := s.Add(Input{Item: "1", File: proofAddBuildSource(t, tc.file, []byte("bytes")), Kind: tc.kind, By: "grid"})
			if err == nil || err.Error() != tc.message {
				t.Fatalf("got %v, want %q", err, tc.message)
			}
			proofAddBuildNoWrite(t, s, "i-1")
		})
	}
}

// Row 4: a set Now gives the record's At, in UTC, in the result and on disk.
func TestProofAddBuildNow(t *testing.T) {
	s := fixture(t)
	instant := time.Date(2026, 3, 4, 15, 4, 5, 123456789, time.FixedZone("plus0530", 5*3600+30*60))
	s.Now = func() time.Time { return instant }
	want := instant.UTC().Format(time.RFC3339Nano)
	if want != "2026-03-04T09:34:05.123456789Z" {
		t.Fatalf("want = %q", want)
	}
	r, err := s.Add(Input{Item: "1", Text: "checks pass", Kind: "log", By: "grid"})
	if err != nil {
		t.Fatal(err)
	}
	if r.At != want {
		t.Fatalf("Record.At = %q, want %q", r.At, want)
	}
	b, err := os.ReadFile(filepath.Join(proofAddBuildFolder(s, "i-1"), "proof.json"))
	if err != nil {
		t.Fatal(err)
	}
	var saved []Record
	if err = json.Unmarshal(b, &saved); err != nil {
		t.Fatal(err)
	}
	if len(saved) != 1 || saved[0].At != want {
		t.Fatalf("proof.json = %s, want one record at %q", b, want)
	}
}

// Row 5: readRecords through Add and Build.
func TestProofAddBuildReadRecords(t *testing.T) {
	for _, tc := range []struct {
		name, content, message string
		prefix                 bool
	}{
		{"not json", "{", "invalid proof.json", true},
		{"null", "null", "proof.json must be an array", false},
	} {
		check := func(t *testing.T, err error) {
			t.Helper()
			if err == nil {
				t.Fatal("no error")
			}
			if tc.prefix && !strings.HasPrefix(err.Error(), tc.message) {
				t.Fatalf("got %v, want a start of %q", err, tc.message)
			}
			if !tc.prefix && err.Error() != tc.message {
				t.Fatalf("got %v, want %q", err, tc.message)
			}
		}
		t.Run(tc.name+" through Add", func(t *testing.T) {
			s := fixture(t)
			proofAddBuildMetadata(t, s, "i-1", []byte(tc.content))
			_, err := s.Add(Input{Item: "1", Text: "checks pass", Kind: "log", By: "grid"})
			check(t, err)
		})
		t.Run(tc.name+" through Build", func(t *testing.T) {
			s := fixture(t)
			proofAddBuildMetadata(t, s, "i-1", []byte(tc.content))
			_, err := s.Build()
			check(t, err)
		})
	}
}

// Row 6: Build re-reads every record and refuses a hand-written bad one.
func TestProofAddBuildRefusals(t *testing.T) {
	for _, tc := range []struct {
		name    string
		change  func(*Record)
		message string
	}{
		{"unknown kind", func(r *Record) { r.Kind = "photo" }, "invalid proof metadata for i-1"},
		{"empty by", func(r *Record) { r.By = "" }, "invalid proof metadata for i-1"},
		{"newline in note", func(r *Record) { r.Note = "one\ntwo" }, "invalid proof metadata for i-1"},
		{"bad time", func(r *Record) { r.At = "yesterday" }, "invalid proof time for i-1"},
		{"other item folder", func(r *Record) { r.File = "proof/i-2/a.txt" }, "invalid proof file for i-1"},
		{"metadata file", func(r *Record) { r.File = "proof/i-1/proof.json" }, "invalid proof file for i-1"},
		{"parent path", func(r *Record) { r.File = "../a.txt" }, "invalid proof file for i-1"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := fixture(t)
			r := proofAddBuildEvidence(t, s, "i-1", "a.txt", "evidence")
			tc.change(&r)
			proofAddBuildRecords(t, s, "i-1", r)
			_, err := s.Build()
			if err == nil || err.Error() != tc.message {
				t.Fatalf("got %v, want %q", err, tc.message)
			}
		})
	}
	t.Run("valid record is accepted", func(t *testing.T) {
		s := fixture(t)
		proofAddBuildRecords(t, s, "i-1", proofAddBuildEvidence(t, s, "i-1", "a.txt", "evidence"))
		idx, err := s.Build()
		if err != nil {
			t.Fatal(err)
		}
		for _, item := range idx.Items {
			if item.ID == "i-1" && len(item.Evidence) != 1 {
				t.Fatalf("i-1 evidence = %v", item.Evidence)
			}
		}
	})
}

// Row 7: records with the same At come back in File order.
func TestProofAddBuildSortByFile(t *testing.T) {
	s := fixture(t)
	b := proofAddBuildEvidence(t, s, "i-1", "b.txt", "second")
	a := proofAddBuildEvidence(t, s, "i-1", "a.txt", "first")
	if a.At != b.At {
		t.Fatalf("At differs: %q and %q", a.At, b.At)
	}
	proofAddBuildRecords(t, s, "i-1", b, a)
	idx, err := s.Build()
	if err != nil {
		t.Fatal(err)
	}
	var files []string
	for _, item := range idx.Items {
		if item.ID == "i-1" {
			for _, r := range item.Evidence {
				files = append(files, r.File)
			}
		}
	}
	want := []string{"proof/i-1/a.txt", "proof/i-1/b.txt"}
	if strings.Join(files, ",") != strings.Join(want, ",") {
		t.Fatalf("order = %v, want %v", files, want)
	}
}
