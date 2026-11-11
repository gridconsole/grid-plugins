package signal

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gridconsole/grid-plugins/internal/where"
)

const validNoteID = "S-01ARZ3NDEKTSV4RRFFQ69G5FAV"

// TestActValidate pins each rule of ActOptions.normalized to its message.
func TestActValidate(t *testing.T) {
	for _, tc := range []struct {
		name string
		id   string
		o    ActOptions
		want string
	}{
		{"invalid-id", "../escape", ActOptions{Act: "note"}, "invalid signal id"},                                                                 // act.go:48
		{"unknown-act", validNoteID, ActOptions{Act: "archive"}, `unknown signal act "archive"`},                                                  // act.go:53
		{"empty-act", validNoteID, ActOptions{}, `unknown signal act ""`},                                                                         // act.go:53
		{"later", validNoteID, ActOptions{Act: "note", Later: true}, "--later is not supported"},                                                  // act.go:56
		{"task-value-not-add", validNoteID, ActOptions{Act: "note", Task: "T-1"}, "--task is only valid for add"},                                 // act.go:59
		{"task-set-not-add", validNoteID, ActOptions{Act: "dismiss", TaskSet: true}, "--task is only valid for add"},                              // act.go:59
		{"until-value-not-snooze", validNoteID, ActOptions{Act: "note", Until: "2026-11-12T09:30"}, "--until is only valid for snooze"},           // act.go:62
		{"until-set-not-snooze", validNoteID, ActOptions{Act: "wake", UntilSet: true}, "--until is only valid for snooze"},                        // act.go:62
		{"add-without-task", validNoteID, ActOptions{Act: "add"}, "add requires --task T-<n>"},                                                    // act.go:65
		{"add-task-set-empty", validNoteID, ActOptions{Act: "add", TaskSet: true}, "add requires --task T-<n>"},                                   // act.go:65
		{"add-bad-task", validNoteID, ActOptions{Act: "add", Task: "T-x7"}, "add requires --task T-<n>"},                                          // act.go:65
		{"add-lowercase-task", validNoteID, ActOptions{Act: "add", Task: "t-7"}, "add requires --task T-<n>"},                                     // act.go:65
		{"add-trailing-text", validNoteID, ActOptions{Act: "add", Task: "T-7 "}, "add requires --task T-<n>"},                                     // act.go:65
		{"snooze-no-until", validNoteID, ActOptions{Act: "snooze"}, "snooze requires --until RFC3339 or YYYY-MM-DDTHH:MM"},                        // act.go:73
		{"snooze-until-set-empty", validNoteID, ActOptions{Act: "snooze", UntilSet: true}, "snooze requires --until RFC3339 or YYYY-MM-DDTHH:MM"}, // act.go:73
		{"snooze-bad-until", validNoteID, ActOptions{Act: "snooze", Until: "tomorrow"}, "snooze requires --until RFC3339 or YYYY-MM-DDTHH:MM"},    // act.go:73
		{"snooze-date-only", validNoteID, ActOptions{Act: "snooze", Until: "2026-11-12"}, "snooze requires --until RFC3339 or YYYY-MM-DDTHH:MM"},  // act.go:73
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.o.Validate(tc.id)
			var bad *BadInputError
			if !errors.As(err, &bad) {
				t.Fatalf("want *BadInputError, got %T %v", err, err)
			}
			if bad.Error() != tc.want || bad.Message != tc.want {
				t.Fatalf("got %q, want %q", bad.Error(), tc.want)
			}
		})
	}
}

// TestActValidateAccepts covers the valid acts and both snooze time forms.
func TestActValidateAccepts(t *testing.T) {
	for _, o := range []ActOptions{
		{Act: "turn"}, {Act: "note"}, {Act: "dismiss"}, {Act: "wake"}, {Act: "reopen"},
		{Act: "add", Task: "T-7"},
		{Act: "snooze", Until: "2026-11-12T09:30"},
		{Act: "snooze", Until: "2026-11-12T09:30:00Z"},
		{Act: "snooze", Until: "2026-11-12T09:30:00+02:00"},
	} {
		if err := o.Validate(validNoteID); err != nil {
			t.Errorf("%+v: %v", o, err)
		}
	}
	// The stored form is a local minute for both input forms.
	local, err := (ActOptions{Act: "snooze", Until: "2026-11-12T09:30"}).normalized(validNoteID)
	if err != nil || local.Until != "2026-11-12T09:30" {
		t.Fatalf("local form: %q %v", local.Until, err)
	}
	rfc, err := (ActOptions{Act: "snooze", Until: "2026-11-12T09:30:45Z"}).normalized(validNoteID)
	want := time.Date(2026, 11, 12, 9, 30, 45, 0, time.UTC).In(time.Local).Format("2006-01-02T15:04")
	if err != nil || rfc.Until != want {
		t.Fatalf("rfc form: %q want %q: %v", rfc.Until, want, err)
	}
}

func actNote(t *testing.T, s Store, source string) (Result, string) {
	t.Helper()
	r, err := s.Add(Input{Source: "mail", SourceID: source, Title: "Title", Body: "Body"})
	if err != nil {
		t.Fatal(err)
	}
	path, err := s.path(r.ID)
	if err != nil {
		t.Fatal(err)
	}
	return r, path
}

func readBytes(t *testing.T, path string) []byte {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func wantRefused(t *testing.T, err error, message string) *RefusedError {
	t.Helper()
	var refused *RefusedError
	if !errors.As(err, &refused) {
		t.Fatalf("want *RefusedError, got %T %v", err, err)
	}
	if refused.Error() != message {
		t.Fatalf("refusal %q, want %q", refused.Error(), message)
	}
	return refused
}

func TestActRefusals(t *testing.T) {
	t.Run("guest", func(t *testing.T) {
		s := fixture(t)
		r, path := actNote(t, s, "guest")
		before := readBytes(t, path)
		s.Resolver.Caller = where.Caller{Role: "guest"}
		_, err := s.Apply(r.ID, ActOptions{Act: "dismiss"}) // act.go:95
		wantRefused(t, err, "guests cannot act on signals")
		if string(readBytes(t, path)) != string(before) {
			t.Fatal("guest refusal changed the note")
		}
	})
	t.Run("missing-note", func(t *testing.T) {
		s := fixture(t)
		actNote(t, s, "missing")                                                  // creates the notes folder
		_, err := s.Apply("S-"+strings.Repeat("ab", 32), ActOptions{Act: "note"}) // act.go:116
		refused := wantRefused(t, err, "no such note")
		if !errors.Is(err, ErrNoNote) || refused.Cause != ErrNoNote || refused.Unwrap() != ErrNoNote {
			t.Fatalf("cause %v", refused.Cause)
		}
	})
	for _, tc := range []struct {
		name    string
		prepare []ActOptions
		act     ActOptions
		message string
	}{
		{"wake-new", nil, ActOptions{Act: "wake"}, "this note is not snoozed"},                                                                                 // act.go:124
		{"reopen-new", nil, ActOptions{Act: "reopen"}, "this note is not decided"},                                                                             // act.go:126
		{"reopen-snoozed", []ActOptions{{Act: "snooze", Until: "2026-11-12T09:30"}}, ActOptions{Act: "reopen"}, "this note is not decided"},                    // act.go:125-126
		{"wake-dismissed", []ActOptions{{Act: "dismiss"}}, ActOptions{Act: "wake"}, "this note is not snoozed"},                                                // act.go:123-124
		{"snooze-dismissed", []ActOptions{{Act: "dismiss"}}, ActOptions{Act: "snooze", Until: "2026-11-12T09:30"}, "this note is already decided (dismissed)"}, // act.go:129-130
		{"note-dismissed", []ActOptions{{Act: "dismiss"}}, ActOptions{Act: "note"}, "this note is already decided (dismissed)"},                                // act.go:130
		{"dismiss-kept", []ActOptions{{Act: "note"}}, ActOptions{Act: "dismiss"}, "this note is already decided (kept)"},                                       // act.go:130
		{"add-kept", []ActOptions{{Act: "note"}}, ActOptions{Act: "add", Task: "T-7"}, "this note is already decided (kept)"},                                  // act.go:130
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := fixture(t)
			r, path := actNote(t, s, tc.name)
			for _, p := range tc.prepare {
				if _, err := s.Apply(r.ID, p); err != nil {
					t.Fatal(err)
				}
			}
			before := readBytes(t, path)
			_, err := s.Apply(r.ID, tc.act)
			wantRefused(t, err, tc.message)
			if string(readBytes(t, path)) != string(before) {
				t.Fatal("refusal changed the note")
			}
		})
	}
	t.Run("bad-input-leaves-note", func(t *testing.T) {
		s := fixture(t)
		r, path := actNote(t, s, "bad")
		before := readBytes(t, path)
		_, err := s.Apply(r.ID, ActOptions{Act: "snooze", Until: "soon"})
		var bad *BadInputError
		if !errors.As(err, &bad) {
			t.Fatalf("got %T %v", err, err)
		}
		if string(readBytes(t, path)) != string(before) {
			t.Fatal("bad input changed the note")
		}
	})
}

func TestActTransitions(t *testing.T) {
	s := fixture(t)
	r, path := actNote(t, s, "transitions")
	step := func(o ActOptions, result, status string) Note {
		t.Helper()
		got, err := s.Apply(r.ID, o)
		if err != nil {
			t.Fatalf("%s: %v", o.Act, err)
		}
		if got.Result != result || got.Status != status {
			t.Fatalf("%s: %+v", o.Act, got)
		}
		n, err := read(path)
		if err != nil {
			t.Fatal(err)
		}
		if n.Status != status {
			t.Fatalf("%s: file status %q", o.Act, n.Status)
		}
		return n
	}
	n := step(ActOptions{Act: "snooze", Until: "2026-11-12T09:30"}, "updated", "snoozed") // act.go:144
	if n.Until != "2026-11-12T09:30" {
		t.Fatalf("until %q", n.Until)
	}
	n = step(ActOptions{Act: "wake"}, "updated", "new") // act.go:146
	if n.Until != "" || n.Became != "" {
		t.Fatalf("wake left until %q became %q", n.Until, n.Became)
	}
	step(ActOptions{Act: "dismiss"}, "updated", "dismissed") // act.go:142
	n = step(ActOptions{Act: "reopen"}, "updated", "new")    // act.go:148
	if n.Became != "" || n.Until != "" {
		t.Fatalf("reopen left became %q until %q", n.Became, n.Until)
	}
	// Reopen clears became on a note that an add had linked to a task.
	step(ActOptions{Act: "add", Task: "T-7"}, "updated", "done")
	if n, _ = read(path); n.Became != "T-7" {
		t.Fatalf("became %q", n.Became)
	}
	n = step(ActOptions{Act: "reopen"}, "updated", "new")
	if n.Became != "" {
		t.Fatalf("reopen kept became %q", n.Became)
	}
	// note twice: the second is unchanged and leaves the file alone.
	step(ActOptions{Act: "note"}, "updated", "kept")
	before := readBytes(t, path)
	step(ActOptions{Act: "note"}, "unchanged", "kept") // act.go:128
	if string(readBytes(t, path)) != string(before) {
		t.Fatal("unchanged note was rewritten")
	}
}

func TestActSnoozeStoresLocalMinute(t *testing.T) {
	s := fixture(t)
	r, path := actNote(t, s, "rfc")
	got, err := s.Apply(r.ID, ActOptions{Act: "snooze", Until: "2026-11-12T09:30:45Z"})
	want := time.Date(2026, 11, 12, 9, 30, 45, 0, time.UTC).In(time.Local).Format("2006-01-02T15:04")
	if err != nil || got.Until != want {
		t.Fatalf("%+v %v want %q", got, err, want)
	}
	if n, err := read(path); err != nil || n.Until != want {
		t.Fatalf("%+v %v", n, err)
	}
}

func TestActAddLinksTaskWithoutCreatingOne(t *testing.T) {
	s := fixture(t)
	r, path := actNote(t, s, "add")
	tasks := filepath.Join(s.GridHome, "projects", s.Project, "tasks")
	if err := os.MkdirAll(filepath.Join(tasks, "T-7"), 0o700); err != nil { // add needs T-7 to exist
		t.Fatal(err)
	}
	got, err := s.Apply(r.ID, ActOptions{Act: "add", Task: "T-7"}) // act.go:138
	if err != nil || got.Status != "done" || got.Became != "T-7" || got.Result != "updated" {
		t.Fatalf("%+v %v", got, err)
	}
	n, err := read(path)
	if err != nil || n.Status != "done" || n.Became != "T-7" {
		t.Fatalf("%+v %v", n, err)
	}
	entries, err := os.ReadDir(tasks)
	if err != nil || len(entries) != 1 || entries[0].Name() != "T-7" {
		t.Fatalf("add created task state: %v %v", entries, err)
	}
}
