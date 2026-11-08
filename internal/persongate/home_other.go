//go:build !linux && !darwin

package persongate

import "os/user"

func accountHome(*user.User) error { return nil }
