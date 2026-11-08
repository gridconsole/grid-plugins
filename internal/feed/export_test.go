package feed

import "testing"

// SetTaskIDHooks exposes the unexported seams only in the Feed test binary.
// Callers must not run in parallel with other tests that initialize identities.
func SetTaskIDHooks(t *testing.T, beforeLock, afterRead func()) {
	t.Helper()
	oldBefore, oldAfter := beforeTaskIDLock, afterTaskIDRead
	beforeTaskIDLock, afterTaskIDRead = beforeLock, afterRead
	t.Cleanup(func() {
		beforeTaskIDLock, afterTaskIDRead = oldBefore, oldAfter
	})
}
