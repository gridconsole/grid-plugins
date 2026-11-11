package signal

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/where"
)

// iySigRefused returns the RefusedError in err, failing when there is none or
// when its message is not message.
func iySigRefused(t *testing.T, err error, message string) *RefusedError {
	t.Helper()
	var refused *RefusedError
	if !errors.As(err, &refused) {
		t.Fatalf("want *RefusedError, got %T %v", err, err)
	}
	if refused.Message != message || refused.Error() != message {
		t.Fatalf("message %q, want %q", refused.Message, message)
	}
	return refused
}

// iySigGuest makes s a guest caller, as act_lane_test.go and move_test.go do.
func iySigGuest(s Store) Store {
	s.Resolver.Caller = where.Caller{Role: "guest"}
	return s
}

// TestIySigStore drives each internal/signal refusal through the Store and
// checks the stable Code it carries.
func TestIySigStore(t *testing.T) {
	missing := "S-" + strings.Repeat("ab", 32)
	cases := []struct {
		name, message, code string
		run                 func(t *testing.T) error
	}{
		{"guest-apply", "guests cannot act on signals", CodeGuestRefused, func(t *testing.T) error {
			s := fixture(t)
			r, _ := actNote(t, s, "iy-guest")
			_, err := iySigGuest(s).Apply(r.ID, ActOptions{Act: "dismiss"})
			return err
		}},
		{"guest-move", "guests cannot act on signals", CodeGuestRefused, func(t *testing.T) error {
			s, r, _, _ := moveFixture(t)
			_, err := iySigGuest(s).Move(r.ID, "example", "second")
			return err
		}},
		{"apply-missing-note", "no such note", CodeNoSuchNote, func(t *testing.T) error {
			s := fixture(t)
			actNote(t, s, "iy-missing") // creates the notes folder
			_, err := s.Apply(missing, ActOptions{Act: "note"})
			return err
		}},
		{"move-missing-note", "no such note", CodeNoSuchNote, func(t *testing.T) error {
			s, _, _, _ := moveFixture(t)
			_, err := s.Move(missing, "example", "second")
			return err
		}},
		{"add-unknown-task", "no task T-99", CodeNoSuchTask, func(t *testing.T) error {
			s := fixture(t)
			r, _ := actNote(t, s, "iy-add")
			_, err := s.Apply(r.ID, ActOptions{Act: "add", Task: "T-99"})
			return err
		}},
		{"move-decided", "this note is already decided (dismissed)", CodeNoteState, func(t *testing.T) error {
			s, r, _, _ := moveFixture(t)
			if _, err := s.Apply(r.ID, ActOptions{Act: "dismiss"}); err != nil {
				t.Fatal(err)
			}
			_, err := s.Move(r.ID, "example", "second")
			return err
		}},
		{"move-same-project", "the note is already there", CodeNoteExists, func(t *testing.T) error {
			s, r, _, _ := moveFixture(t)
			_, err := s.Move(r.ID, "example", "example")
			return err
		}},
		{"move-id-in-target", "a note with this id is already there", CodeNoteExists, func(t *testing.T) error {
			s, r, _, before := moveFixture(t)
			dest := iySigTargetPath(t, s, r.ID)
			if err := os.WriteFile(dest, before, 0o600); err != nil {
				t.Fatal(err)
			}
			_, err := s.Move(r.ID, "example", "second")
			return err
		}},
		{"move-notes-full", "notes already holds 4000 files", CodeNotesFull, func(t *testing.T) error {
			s, r, _, _ := moveFixture(t)
			dir := filepath.Dir(iySigTargetPath(t, s, r.ID))
			for i := range 4000 {
				if err := os.WriteFile(filepath.Join(dir, fmt.Sprintf("iy-%04d.md", i)), nil, 0o600); err != nil {
					t.Fatal(err)
				}
			}
			_, err := s.Move(r.ID, "example", "second")
			return err
		}},
		{"move-unknown-project", "no such project", CodeNoSuchProject, func(t *testing.T) error {
			s, r, _, _ := moveFixture(t)
			_, err := s.Move(r.ID, "example", "nosuch")
			return err
		}},
	}
	// The act refusals of act_lane_test.go TestActRefusals, each on a fresh note.
	for _, tc := range []struct {
		name    string
		prepare []ActOptions
		act     ActOptions
		message string
	}{
		{"wake-new", nil, ActOptions{Act: "wake"}, "this note is not snoozed"},
		{"reopen-new", nil, ActOptions{Act: "reopen"}, "this note is not decided"},
		{"reopen-snoozed", []ActOptions{{Act: "snooze", Until: "2026-11-12T09:30"}}, ActOptions{Act: "reopen"}, "this note is not decided"},
		{"wake-dismissed", []ActOptions{{Act: "dismiss"}}, ActOptions{Act: "wake"}, "this note is not snoozed"},
		{"snooze-dismissed", []ActOptions{{Act: "dismiss"}}, ActOptions{Act: "snooze", Until: "2026-11-12T09:30"}, "this note is already decided (dismissed)"},
		{"note-dismissed", []ActOptions{{Act: "dismiss"}}, ActOptions{Act: "note"}, "this note is already decided (dismissed)"},
		{"dismiss-kept", []ActOptions{{Act: "note"}}, ActOptions{Act: "dismiss"}, "this note is already decided (kept)"},
		{"add-kept", []ActOptions{{Act: "note"}}, ActOptions{Act: "add", Task: "T-7"}, "this note is already decided (kept)"},
	} {
		cases = append(cases, struct {
			name, message, code string
			run                 func(t *testing.T) error
		}{"act-" + tc.name, tc.message, CodeNoteState, func(t *testing.T) error {
			s := fixture(t)
			r, _ := actNote(t, s, tc.name)
			for _, p := range tc.prepare {
				if _, err := s.Apply(r.ID, p); err != nil {
					t.Fatal(err)
				}
			}
			_, err := s.Apply(r.ID, tc.act)
			return err
		}})
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := iySigRefused(t, tc.run(t), tc.message).Code; got != tc.code {
				t.Fatalf("code %q, want %q", got, tc.code)
			}
		})
	}
}

// iySigTargetPath is the path id would have in the "second" project of
// moveFixture; its notes folder is created.
func iySigTargetPath(t *testing.T, s Store, id string) string {
	t.Helper()
	target := s
	target.Project = "second"
	dest, err := target.path(id)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		t.Fatal(err)
	}
	return dest
}
