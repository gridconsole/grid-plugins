//go:build !linux && !darwin && !windows

package session

import (
	"errors"
	"time"
)

// ProcessStart is unavailable on hosts with no Linux, macOS or Windows process identity.
func ProcessStart(int) (time.Time, error) {
	return time.Time{}, errors.New("task lease needs a Unix host for process identity")
}

// ProbeLocal leaves unsupported process checks unknown.
func ProbeLocal(Live) ProcessState { return Unknown }
