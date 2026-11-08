package task

import (
	"errors"
	"fmt"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/contract"
)

func TestTaskTextRefusalSet(t *testing.T) {
	forbidden := []rune{'\u2028', '\u2029', '\ufdd0', '\ufdef', '\ufffe', '\uffff', '\U0010ffff'}
	for r := rune(0); r <= 0x9f; r++ {
		if (r <= 0x1f || r >= 0x7f) && r != '\t' && r != '\n' {
			forbidden = append(forbidden, r)
		}
	}
	for r := rune(0x202a); r <= 0x202e; r++ {
		forbidden = append(forbidden, r)
	}
	for r := rune(0x2066); r <= 0x2069; r++ {
		forbidden = append(forbidden, r)
	}
	values := []string{"bad\xfftext"}
	for _, r := range forbidden {
		values = append(values, "before"+string(r)+"after")
	}
	for _, value := range values {
		t.Run(fmt.Sprintf("%q", value), func(t *testing.T) {
			s := Store{}
			writes := map[string]func() error{
				"scalar":        func() error { _, err := encodeTaskScalar(value); return err },
				"session":       func() error { _, err := s.Attach(AttachOptions{SessionID: value}); return err },
				"attach author": func() error { _, err := s.Attach(AttachOptions{Who: contract.Author{ID: value}}); return err },
				"rename":        func() error { _, err := s.Rename(RenameOptions{Title: value}); return err },
				"decline":       func() error { _, err := s.Decline(DeclineOptions{Reason: value}); return err },
				"words":         func() error { return (NewOptions{Words: value}).Validate() },
				"workflow":      func() error { return (NewOptions{Words: "Valid", Workflow: value}).Validate() },
				"owner":         func() error { return (NewOptions{Words: "Valid", Who: value}).Validate() },
				"role":          func() error { return (NewOptions{Words: "Valid", Role: value}).Validate() },
				"history":       func() error { _, err := s.Note(WriteOptions{Note: value}); return err },
				"reason":        func() error { _, err := s.Change(WriteOptions{Note: value}); return err },
				"approval":      func() error { _, err := s.Approve(WriteOptions{Note: value}, ""); return err },
				"decision":      func() error { _, err := s.Decide(WriteOptions{Note: value}, "R-1", "deny"); return err },
				"actor": func() error {
					_, err := s.Decline(DeclineOptions{Reason: "Later", WriteOptions: WriteOptions{Who: contract.Author{ID: value}}})
					return err
				},
				"outcome": func() error { _, err := s.Deliver(DeliverOptions{Outcome: value, By: "worker"}); return err },
				"by":      func() error { _, err := s.Deliver(DeliverOptions{Outcome: "Done", By: value}); return err },
			}
			for field, write := range writes {
				t.Run(field, func(t *testing.T) {
					if err := write(); !errors.Is(err, ErrEditUsage) {
						t.Fatalf("text validation must precede task resolution: %v", err)
					}
				})
			}
		})
	}
	// Freeform task words retain supported layout and ordinary Unicode.
	if err := (NewOptions{Words: "café 🚀\tfirst\nsecond\ufdcf\ufdf0\ufffd\U0001fffd\U0010fffd"}).Validate(); err != nil {
		t.Fatal(err)
	}
}
