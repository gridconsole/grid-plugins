package where

import "errors"

// ErrPersonalRefused means the caller cannot resolve the machine owner's Personal project.
var ErrPersonalRefused = errors.New("this caller cannot use the Personal project")

// DefaultPersonalAllowed allows the local CLI (an empty role) and the owner.
// A host or the workflow may replace this default through Resolver.PersonalAllowed.
func DefaultPersonalAllowed(role string) bool {
	return role == "" || role == "owner"
}
