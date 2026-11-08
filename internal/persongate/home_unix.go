//go:build linux || darwin

package persongate

import (
	"errors"
	"os/user"
)

// The pure-Go Linux Current implementation can fall back to HOME if the UID
// is absent from the account database. Verify it against LookupId to fail closed.
func accountHome(u *user.User) error {
	account, err := user.LookupId(u.Uid)
	if err != nil {
		return err
	}
	if account.HomeDir != u.HomeDir {
		return errors.New("account home could not be verified")
	}
	return nil
}
