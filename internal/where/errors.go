package where

import "errors"

// ErrTaskNotFound identifies a task that is missing or unavailable to the caller.
var ErrTaskNotFound = errors.New("task not found")

// LookupError retains a lookup's human diagnostic independently of its identity.
type LookupError struct {
	Kind    error
	Message string
}

func (e *LookupError) Error() string { return e.Message }
func (e *LookupError) Unwrap() error { return e.Kind }
