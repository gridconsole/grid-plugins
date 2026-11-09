package signal

import (
	"errors"
	"fmt"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/where"
)

func TestNotesProjectErrorCauses(t *testing.T) {
	for _, tc := range []struct {
		name, project, message string
		cause                  error
	}{
		{"unregistered", "absent", `project "absent" is not registered; run grid project add`, where.ErrProjectUnregistered},
		{"personal-refused", "personal", `project "personal" is not registered`, where.ErrPersonalRefused},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := fixture(t)
			s.Project = tc.project
			s.Resolver.PersonalAllowed = func(where.Caller) bool { return false }
			_, err := s.notes()
			if !errors.Is(err, tc.cause) {
				t.Errorf("notes error lacks typed cause: %v", err)
			}
			if err == nil || err.Error() != tc.message {
				t.Errorf("notes message changed: %v", err)
			}
		})
	}
}

// Keep the cause but replace all display text, as a future wording change might.
type rewordedProjectError struct{ error }

func (rewordedProjectError) Error() string   { return "destination unavailable" }
func (e rewordedProjectError) Unwrap() error { return e.error }

func TestMoveProjectErrorReworded(t *testing.T) {
	for _, cause := range []error{where.ErrProjectUnregistered, where.ErrPersonalRefused} {
		err := moveNotesError(rewordedProjectError{fmt.Errorf("lookup: %w", cause)})
		var refused *RefusedError
		if !errors.As(err, &refused) || refused.Message != "no such project" {
			t.Fatalf("reworded cause lost refusal: %v", err)
		}
	}
	plain := errors.New(`project "absent" is not registered`)
	if got := moveNotesError(plain); got != plain {
		t.Fatalf("untyped text classified as refusal: %v", got)
	}
}
