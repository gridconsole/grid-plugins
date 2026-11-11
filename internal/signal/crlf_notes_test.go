package signal

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/project"
)

// crlfNote adds a note through Store.Add, then rewrites its file with every
// "\n" turned into "\r\n", as an editor on Windows would save it.
func crlfNote(t *testing.T, s Store, in Input) (Result, string, []byte) {
	t.Helper()
	r, err := s.Add(in)
	if err != nil {
		t.Fatal(err)
	}
	path, err := s.path(r.ID)
	if err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	crlf := bytes.ReplaceAll(b, []byte("\n"), []byte("\r\n"))
	if err := os.WriteFile(path, crlf, 0o600); err != nil {
		t.Fatal(err)
	}
	return r, path, crlf
}

func TestCRLFNoteDismiss(t *testing.T) {
	s := fixture(t)
	r, path, _ := crlfNote(t, s, Input{Source: "mail", SourceID: "m-1", Title: "Windows note", Body: "line one\nline two\n"})
	got, err := s.Dismiss(r.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != "dismissed" {
		t.Fatalf("status %q", got.Status)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(b, []byte("---\n")) {
		t.Fatalf("rewritten note does not open with an LF front matter: %q", b[:8])
	}
}

func TestCRLFNoteAddIsDuplicate(t *testing.T) {
	s := fixture(t)
	in := Input{Source: "mail", SourceID: "m-2", Title: "Windows note", Body: "text\n"}
	r, _, _ := crlfNote(t, s, in)
	dup, err := s.Add(in)
	if err != nil {
		t.Fatal(err)
	}
	if dup.Result != "duplicate" || dup.ID != r.ID {
		t.Fatalf("%+v", dup)
	}
}

func TestCRLFNoteMove(t *testing.T) {
	s := fixture(t)
	if err := project.Write(s.GridHome, project.Record{Name: "second"}); err != nil {
		t.Fatal(err)
	}
	r, _, crlf := crlfNote(t, s, Input{Source: "mail", SourceID: "m-3", Title: "Move the Windows note", Body: "a\nb\n"})
	_, wantBody, ok := bytes.Cut(crlf[5:], []byte("\r\n---\r\n"))
	if !ok || string(wantBody) != "a\r\nb\r\n" {
		t.Fatalf("test setup: body %q", wantBody)
	}
	if _, err := s.Move(r.ID, "example", "second"); err != nil {
		t.Fatal(err)
	}
	to := s
	to.Project = "second"
	path, err := to.path(r.ID)
	if err != nil {
		t.Fatal(err)
	}
	n, err := read(path)
	if err != nil {
		t.Fatal(err)
	}
	if n.Title != "Move the Windows note" || n.Source != "mail" || n.SourceID != "m-3" {
		t.Fatalf("%+v", n)
	}
	if n.Body != string(wantBody) {
		t.Fatalf("body %q, want %q", n.Body, wantBody)
	}
}

func TestBodyCRLFIsData(t *testing.T) {
	s := fixture(t)
	r, err := s.Add(Input{Source: "mail", SourceID: "m-4", Title: "LF header, CRLF body", Body: "a\r\nb\r\n"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Dismiss(r.ID); err != nil {
		t.Fatal(err)
	}
	path, err := s.path(r.ID)
	if err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	_, body, ok := bytes.Cut(b[4:], []byte("\n---\n"))
	if !ok || string(body) != "a\r\nb\r\n" {
		t.Fatalf("body %q", body)
	}
}

func TestCRLFNoteStillRefused(t *testing.T) {
	s := fixture(t)
	r, path, crlf := crlfNote(t, s, Input{Source: "mail", SourceID: "m-5", Title: "Refuse me", Body: "x\n"})
	write := func(b []byte) {
		t.Helper()
		if err := os.WriteFile(path, b, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	t.Run("wrong tier", func(t *testing.T) {
		write(bytes.Replace(crlf, []byte("tier: external"), []byte("tier: person-team"), 1))
		if _, err := read(path); err == nil || err.Error() != "signal metadata is invalid" {
			t.Fatalf("%v", err)
		}
	})
	t.Run("file name is not the id", func(t *testing.T) {
		write(crlf)
		other := strings.TrimSuffix(path, r.ID+".md") + "S-other.md"
		if err := os.WriteFile(other, crlf, 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := read(other); err == nil || err.Error() != "signal metadata is invalid" {
			t.Fatalf("%v", err)
		}
	})
	t.Run("no closing line", func(t *testing.T) {
		write([]byte("---\r\nid: " + r.ID + "\r\ntier: external\r\nbody text\r\n"))
		if _, err := read(path); err == nil || err.Error() != "signal has no closing front matter" {
			t.Fatalf("%v", err)
		}
	})
	t.Run("mixed endings do not close a CRLF header", func(t *testing.T) {
		write([]byte("---\r\nid: " + r.ID + "\r\ntier: external\n---\nbody\n"))
		if _, err := read(path); err == nil || err.Error() != "signal has no closing front matter" {
			t.Fatalf("%v", err)
		}
	})
	t.Run("over MaxBytes", func(t *testing.T) {
		write(append(append([]byte{}, crlf...), bytes.Repeat([]byte("x"), MaxBytes)...))
		if _, err := read(path); err == nil || !strings.Contains(err.Error(), "at most") {
			t.Fatalf("%v", err)
		}
	})
}
