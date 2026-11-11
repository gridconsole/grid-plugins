package signal

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func wantBadInput(t *testing.T, err error, message string) {
	t.Helper()
	var bad *BadInputError
	if !errors.As(err, &bad) {
		t.Fatalf("want *BadInputError, got %T %v", err, err)
	}
	if bad.Message != message {
		t.Fatalf("message %q, want %q", bad.Message, message)
	}
}

func notRefused(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		t.Fatal("want an error, got nil")
	}
	var refused *RefusedError
	if errors.As(err, &refused) {
		t.Fatalf("want an I/O error, got a refusal: %v", err)
	}
}

// TestValidateMoveRefusals covers move.go:23-29 and keeps the id check as contrast.
func TestValidateMoveRefusals(t *testing.T) {
	wantBadInput(t, ValidateMove(validNoteID, "example", ""), "move requires --to <project|->") // move.go:24
	wantBadInput(t, ValidateMove(validNoteID, "../x", "second"), "invalid project name")        // move.go:28, from
	wantBadInput(t, ValidateMove(validNoteID, "example", "../x"), "invalid project name")       // move.go:28, to
	wantBadInput(t, ValidateMove("S-"+strings.Repeat("g", 64), "example", "second"), "invalid signal id")
	wantBadInput(t, ValidateMove("S-"+strings.Repeat("A", 64), "example", "second"), "invalid signal id")
	wantBadInput(t, ValidateMove("S-abc", "example", "second"), "invalid signal id") // signal.go:118
}

// TestMoveRefusals covers move.go:71-81 on a real moved-from note.
func TestMoveRefusals(t *testing.T) {
	t.Run("source-is-a-directory", func(t *testing.T) {
		s, r, path, _ := moveFixture(t)
		if err := os.Remove(path); err != nil {
			t.Fatal(err)
		}
		if err := os.Mkdir(path, 0o755); err != nil {
			t.Fatal(err)
		}
		_, err := s.Move(r.ID, "example", "second") // move.go:71-73
		notRefused(t, err)
		if !strings.Contains(err.Error(), "regular file") {
			t.Fatalf("error %q", err)
		}
	})
	t.Run("decided", func(t *testing.T) {
		s, r, path, _ := moveFixture(t)
		if _, err := s.Apply(r.ID, ActOptions{Act: "dismiss"}); err != nil {
			t.Fatal(err)
		}
		before := readBytes(t, path)
		_, err := s.Move(r.ID, "example", "second") // move.go:74-76
		wantRefused(t, err, "this note is already decided (dismissed)")
		if !bytes.Equal(readBytes(t, path), before) {
			t.Fatal("refusal changed the note")
		}
	})
	t.Run("same-project", func(t *testing.T) {
		s, r, path, before := moveFixture(t)
		_, err := s.Move(r.ID, "example", "example") // move.go:77-79
		wantRefused(t, err, "the note is already there")
		if !bytes.Equal(readBytes(t, path), before) {
			t.Fatal("refusal changed the note")
		}
	})
	t.Run("id-already-in-target", func(t *testing.T) {
		s, r, path, before := moveFixture(t)
		target := s
		target.Project = "second"
		dest, err := target.path(r.ID)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(dest, before, 0o600); err != nil {
			t.Fatal(err)
		}
		_, err = s.Move(r.ID, "example", "second") // move.go:80-82
		wantRefused(t, err, "a note with this id is already there")
		if !bytes.Equal(readBytes(t, path), before) || !bytes.Equal(readBytes(t, dest), before) {
			t.Fatal("refusal changed a note")
		}
	})
}

// TestRelocateStatusRefusals calls relocateStatus directly with exact bytes.
func TestRelocateStatusRefusals(t *testing.T) {
	for _, tc := range []struct {
		name, in, old, status string
		want                  string // output bytes when wantErr is empty
		wantErr               string // exact message; "*" means any error
	}{
		{"same-status", "unchanged, not even front matter", "new", "new", "unchanged, not even front matter", ""},               // move.go:154
		{"crlf-no-closing", "---\r\nstatus: new\r\n", "new", "placed", "", "signal has no closing front matter"},                // move.go:161
		{"lf-no-closing", "---\nstatus: new\n", "new", "placed", "", "signal has no closing front matter"},                      // move.go:168
		{"yaml-error", "---\nstatus: [\n---\n", "new", "placed", "", "*"},                                                       // move.go:172
		{"not-a-mapping", "---\n- a\n---\n", "new", "placed", "", "signal metadata is invalid"},                                 // move.go:175
		{"single-quoted-doubled", "---\nstatus: 'ne''w'\n---\n", "ne'w", "placed", "---\nstatus: 'placed'\n---\n", ""},          // move.go:212
		{"alias", "---\nx: &s new\nstatus: *s\n---\n", "new", "placed", "---\nx: &s new\nstatus: placed\n---\n", ""},            // move.go:221
		{"old-longer-than-file", "---\nstatus: new\n---\n", strings.Repeat("a", 200), "placed", "", "signal status is invalid"}, // move.go:241
		{"old-shorter-than-real", "---\nstatus: new\n---\n", "ne", "placed", "", "signal status is invalid"},                    // move.go:257
		{"no-status", "---\ntitle: t\n---\n", "new", "placed", "", "signal status is missing"},                                  // move.go:262
	} {
		t.Run(tc.name, func(t *testing.T) {
			in := []byte(tc.in)
			got, err := relocateStatus(in, tc.old, tc.status)
			if tc.wantErr != "" {
				if err == nil || (tc.wantErr != "*" && err.Error() != tc.wantErr) {
					t.Fatalf("err %v, want %q", err, tc.wantErr)
				}
				if got != nil {
					t.Fatalf("output %q with an error", got)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if string(got) != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
			if string(in) != tc.in {
				t.Fatalf("input changed to %q", in)
			}
		})
	}
	t.Run("same-status-returns-the-slice", func(t *testing.T) {
		in := []byte("abc")
		got, err := relocateStatus(in, "new", "new")
		if err != nil || len(got) != 3 || &got[0] != &in[0] {
			t.Fatalf("got %q, %v", got, err)
		}
	})
}

// TestValidateRefusals covers signal.go:71-73, :93-95 and :99-101.
func TestValidateRefusals(t *testing.T) {
	for _, tc := range []struct {
		name string
		in   Input
		want string
	}{
		{"source-not-utf8", Input{Source: "a\xffb", Title: "t"}, "source must be UTF-8 text of at most 128 bytes"},
		{"source-too-long", Input{Source: strings.Repeat("a", 129), Title: "t"}, "source must be UTF-8 text of at most 128 bytes"},
		{"title-too-long", Input{Source: "s", Title: strings.Repeat("a", 1025)}, "title must be UTF-8 text of at most 1024 bytes"},
		{"body-too-long", Input{Source: "s", Title: "t", Body: strings.Repeat("a", MaxBytes+1)}, fmt.Sprintf("body must be UTF-8 text of at most %d bytes", MaxBytes)},
		{"body-not-utf8", Input{Source: "s", Title: "t", Body: "a\xffb"}, fmt.Sprintf("body must be UTF-8 text of at most %d bytes", MaxBytes)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := Validate(tc.in); err == nil || err.Error() != tc.want {
				t.Fatalf("err %v, want %q", err, tc.want)
			}
		})
	}
}

func notesDir(t *testing.T, s Store) string {
	t.Helper()
	dir := filepath.Join(s.GridHome, "projects", s.Project, "notes")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	return dir
}

// TestAddAndReadRefusals covers signal.go:124-126, :148-150, :187-189, :197-199, :248-255.
func TestAddAndReadRefusals(t *testing.T) {
	t.Run("bad-project-name", func(t *testing.T) {
		s := fixture(t)
		s.Project = "../x"
		if _, err := s.Add(Input{Source: "mail", Title: "t"}); err == nil { // signal.go:124
			t.Fatal("Add accepted a bad project name")
		}
		if _, err := os.Stat(filepath.Join(s.GridHome, "projects", "example", "notes")); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("a notes folder was made: %v", err)
		}
	})
	t.Run("bad-id-path", func(t *testing.T) {
		s := fixture(t)
		if _, err := s.path("bad"); err == nil || err.Error() != "invalid signal id" { // signal.go:149
			t.Fatalf("err %v", err)
		}
	})
	t.Run("no-front-matter", func(t *testing.T) {
		s := fixture(t)
		path := filepath.Join(notesDir(t, s), validNoteID+".md")
		if err := os.WriteFile(path, []byte("plain text, no header\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		_, err := s.Apply(validNoteID, ActOptions{Act: "dismiss"}) // signal.go:188
		notRefused(t, err)
		if err.Error() != "signal has no front matter" {
			t.Fatalf("err %v", err)
		}
		if string(readBytes(t, path)) != "plain text, no header\n" {
			t.Fatal("the note changed")
		}
	})
	t.Run("header-yaml-error", func(t *testing.T) {
		s := fixture(t)
		text := "---\nid: [\n---\nbody\n"
		path := filepath.Join(notesDir(t, s), validNoteID+".md")
		if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
			t.Fatal(err)
		}
		_, err := s.Apply(validNoteID, ActOptions{Act: "dismiss"}) // signal.go:198
		notRefused(t, err)
		for _, other := range []string{"signal has no front matter", "signal has no closing front matter", "signal metadata is invalid"} {
			if err.Error() == other {
				t.Fatalf("got the %q refusal, want a YAML error", other)
			}
		}
		if string(readBytes(t, path)) != text {
			t.Fatal("the note changed")
		}
	})
	t.Run("id-conflicts-with-other-source", func(t *testing.T) {
		s := fixture(t)
		sum := sha256.Sum256([]byte("mail" + "\x00" + "message-1"))
		id := "S-" + hex.EncodeToString(sum[:])
		text := "---\nid: " + id + "\nsource: other\nsource_id: message-1\nstatus: new\ncaptured: 2026-11-11T09:00:00Z\nupdated: 2026-11-11T09:00:00Z\ntier: external\nbody_kind: data\n---\nbody\n"
		path := filepath.Join(notesDir(t, s), id+".md")
		if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
			t.Fatal(err)
		}
		_, err := s.Add(Input{Source: "mail", SourceID: "message-1", Title: "t"}) // signal.go:249
		if err == nil || err.Error() != "signal id conflicts with existing source" {
			t.Fatalf("err %v", err)
		}
		if string(readBytes(t, path)) != text {
			t.Fatal("the note changed")
		}
	})
	t.Run("id-path-is-a-directory", func(t *testing.T) {
		s := fixture(t)
		sum := sha256.Sum256([]byte("mail" + "\x00" + "message-2"))
		id := "S-" + hex.EncodeToString(sum[:])
		dir := notesDir(t, s)
		if err := os.Mkdir(filepath.Join(dir, id+".md"), 0o755); err != nil {
			t.Fatal(err)
		}
		_, err := s.Add(Input{Source: "mail", SourceID: "message-2", Title: "t"}) // signal.go:254
		notRefused(t, err)
		entries, rerr := os.ReadDir(dir)
		if rerr != nil || len(entries) != 1 || !entries[0].IsDir() {
			t.Fatalf("notes folder holds %v, %v", entries, rerr)
		}
		sub, rerr := os.ReadDir(filepath.Join(dir, id+".md"))
		if rerr != nil || len(sub) != 0 {
			t.Fatalf("the directory holds %v, %v", sub, rerr)
		}
	})
}

// TestApplyStoreRefusals covers act.go:101-103, :118-120 and :158-160.
func TestApplyStoreRefusals(t *testing.T) {
	t.Run("project-not-registered", func(t *testing.T) {
		s := fixture(t)
		s.Project = "nowhere"
		_, err := s.Apply(validNoteID, ActOptions{Act: "dismiss"}) // act.go:101
		if err == nil || !strings.Contains(err.Error(), `project "nowhere" is not registered`) {
			t.Fatalf("err %v", err)
		}
	})
	t.Run("note-is-a-directory", func(t *testing.T) {
		s := fixture(t)
		r, path := actNote(t, s, "dir")
		if err := os.Remove(path); err != nil {
			t.Fatal(err)
		}
		if err := os.Mkdir(path, 0o755); err != nil {
			t.Fatal(err)
		}
		_, err := s.Apply(r.ID, ActOptions{Act: "dismiss"}) // act.go:118
		notRefused(t, err)
	})
	for name, title := range map[string]string{"literal-tab": "title: Ti\ttle", "escaped-tab": `title: "Ti\ttle"`} {
		t.Run("turn-title-"+name, func(t *testing.T) {
			s := fixture(t)
			r, path := actNote(t, s, "tab")
			text := string(readBytes(t, path))
			if !strings.Contains(text, "title: Title\n") {
				t.Fatalf("note has no title line:\n%s", text)
			}
			edited := strings.Replace(text, "title: Title\n", title+"\n", 1)
			if err := os.WriteFile(path, []byte(edited), 0o600); err != nil {
				t.Fatal(err)
			}
			_, err := s.Apply(r.ID, ActOptions{Act: "turn", Cwd: s.GridHome, Who: "alex", Role: "owner"}) // act.go:158
			if err == nil || err.Error() != "title must be one line" {
				t.Fatalf("err %v", err)
			}
			if string(readBytes(t, path)) != edited {
				t.Fatal("the note changed")
			}
			tasks, _ := filepath.Glob(filepath.Join(s.GridHome, "projects", s.Project, "tasks", "*"))
			if len(tasks) != 0 {
				t.Fatalf("a task was made: %v", tasks)
			}
		})
	}
}
