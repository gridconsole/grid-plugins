//go:build !linux && !darwin

package session

import (
	"errors"
	"time"
)

// ProcessStart is unavailable on hosts without supported Unix process identity.
func ProcessStart(int) (time.Time, error) {
	return time.Time{}, errors.New("task lease needs a Unix host for process identity")
}

// ProbeLocal leaves unsupported process checks unknown.
func ProbeLocal(Live) ProcessState { return Unknown }
