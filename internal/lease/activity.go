package lease

import "github.com/gridconsole/grid-plugins/internal/session"

// HookActivity records output observed by a hook without renewing a lease.
// Supervision alone calls session.Reconcile to write feed/.live after checking
// both pid and start time (feed-contract section 8).
func HookActivity(store session.Store, key session.Key, transcript string) error {
	_, err := store.Touch(key, transcript)
	return err
}
