package artifact

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func wantText(t *testing.T, err error, text string) {
	t.Helper()
	if err == nil || err.Error() != text {
		t.Fatalf("error: got %v, want %q", err, text)
	}
}

func TestTaskRefusals(t *testing.T) {
	const link = "https://example.com/x"

	t.Run("no task", func(t *testing.T) {
		_, err := Store{TaskPath: ""}.Link(link, "")
		wantText(t, err, "a task is required")
		_, err = Store{TaskPath: ""}.List()
		wantText(t, err, "a task is required")
	})

	t.Run("task does not exist", func(t *testing.T) {
		missing := filepath.Join(t.TempDir(), "missing")
		_, err := Store{TaskPath: missing}.Link(link, "")
		if !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("Link: got %v, want os.ErrNotExist", err)
		}
		if _, err := os.Lstat(missing); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("Link created the task folder: %v", err)
		}
	})

	t.Run("task is a regular file", func(t *testing.T) {
		file := filepath.Join(t.TempDir(), "task")
		const body = "not a folder\n"
		if err := os.WriteFile(file, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
		_, err := Store{TaskPath: file}.Link(link, "")
		wantText(t, err, "task is not a directory")
		_, err = Store{TaskPath: file}.List()
		wantText(t, err, "task is not a directory")
		b, rerr := os.ReadFile(file)
		if rerr != nil || string(b) != body {
			t.Fatalf("file changed: %q, %v", b, rerr)
		}
		if _, err := os.Lstat(filepath.Join(file, "artifacts")); err == nil {
			t.Fatal("an artifacts folder exists below the file")
		}
	})
}

func TestBrokenRecords(t *testing.T) {
	// linked makes a real record with Link and returns the store, its id and the
	// path of the record file.
	linked := func(t *testing.T) (Store, string, string) {
		t.Helper()
		s := Store{TaskPath: t.TempDir()}
		r, err := s.Link("https://example.com/x", "x")
		if err != nil {
			t.Fatal(err)
		}
		return s, r.ID, filepath.Join(s.TaskPath, "artifacts", r.ID+".json")
	}
	// rewrite changes the stored record and returns the file's new bytes.
	rewrite := func(t *testing.T, path string, change func(*Record)) []byte {
		t.Helper()
		b, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		var r Record
		if err := json.Unmarshal(b, &r); err != nil {
			t.Fatal(err)
		}
		change(&r)
		b, err = json.MarshalIndent(r, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, b, 0o644); err != nil {
			t.Fatal(err)
		}
		return b
	}
	// refused checks that Publish and List give text for the broken record and
	// that the record file still holds want.
	refused := func(t *testing.T, s Store, id, path string, want []byte, text func(string) bool) {
		t.Helper()
		_, err := s.Publish(id)
		if err == nil || !text(err.Error()) {
			t.Fatalf("Publish: got %v", err)
		}
		_, lerr := s.List()
		if lerr == nil || lerr.Error() != err.Error() {
			t.Fatalf("List: got %v, want the Publish error %v", lerr, err)
		}
		got, rerr := os.ReadFile(path)
		if rerr != nil || string(got) != string(want) {
			t.Fatalf("record changed: %q, %v", got, rerr)
		}
	}
	exact := func(text string) func(string) bool { return func(got string) bool { return got == text } }

	t.Run("no record for a valid id", func(t *testing.T) {
		s, id, _ := linked(t)
		other := "A-" + strings.Repeat("0", 32)
		if other == id {
			t.Fatal("test id collides with the real one")
		}
		if _, err := s.Publish(other); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("Publish: got %v, want os.ErrNotExist", err)
		}
	})

	t.Run("record is a directory", func(t *testing.T) {
		s, id, path := linked(t)
		if err := os.Remove(path); err != nil {
			t.Fatal(err)
		}
		if err := os.Mkdir(path, 0o755); err != nil {
			t.Fatal(err)
		}
		_, err := s.Publish(id)
		wantText(t, err, "artifact record must be a regular file")
		_, err = s.List()
		wantText(t, err, "artifact record must be a regular file")
		if info, err := os.Lstat(path); err != nil || !info.IsDir() {
			t.Fatalf("the directory is gone: %v", err)
		}
	})

	t.Run("record is not JSON", func(t *testing.T) {
		s, id, path := linked(t)
		if err := os.WriteFile(path, []byte("not json"), 0o644); err != nil {
			t.Fatal(err)
		}
		refused(t, s, id, path, []byte("not json"), func(got string) bool { return strings.HasPrefix(got, "artifact "+id+": ") })
	})

	t.Run("id field differs", func(t *testing.T) {
		s, id, path := linked(t)
		want := rewrite(t, path, func(r *Record) { r.ID = "A-" + strings.Repeat("1", 32) })
		refused(t, s, id, path, want, exact("invalid artifact record "+id))
	})

	t.Run("kind other", func(t *testing.T) {
		s, id, path := linked(t)
		want := rewrite(t, path, func(r *Record) { r.Kind = "other" })
		refused(t, s, id, path, want, exact("invalid artifact record "+id))
	})

	t.Run("file name with a slash", func(t *testing.T) {
		s, id, path := linked(t)
		want := rewrite(t, path, func(r *Record) { r.File = "a/b" })
		refused(t, s, id, path, want, exact("artifact "+id+": invalid file name"))
	})
}

func TestImportSourceLabelOutsideMissingProject(t *testing.T) {
	source := sourceFile(t, "report.txt", 3)
	project := filepath.Join(t.TempDir(), "no-such-project")
	if _, err := os.Lstat(project); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("project folder exists: %v", err)
	}
	r, err := Store{TaskPath: t.TempDir(), ProjectFolder: project}.Import(source, "")
	if err != nil {
		t.Fatal(err)
	}
	if !regexp.MustCompile(`^report\.txt#[0-9a-f]{16}$`).MatchString(r.Source) {
		t.Fatalf("Source: got %q, want report.txt#<16 hex>", r.Source)
	}
}
