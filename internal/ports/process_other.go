//go:build !linux && !darwin

package ports

import (
	"errors"
	"time"
)

// Unsupported platforms cannot prove that an owner is dead.
func Alive(pid int, group bool) bool { return true }

func networkUnsupported(error) bool { return false }

// Stop is unavailable: Windows job object supervision is not implemented.
func Stop(int, time.Duration) error {
	return errors.New("dev environments require macOS or Linux; use WSL2")
}

func ProcessMatches(Record) (bool, error) {
	return false, errors.New("dev environments require macOS or Linux; use WSL2")
}
func StopRecord(Record, time.Duration) error {
	return errors.New("dev environments require macOS or Linux; use WSL2")
}
