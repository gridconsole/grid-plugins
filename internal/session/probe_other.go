//go:build !linux && !darwin

package session

// ProbeLocal leaves unsupported process checks unknown.
func ProbeLocal(Live) ProcessState { return Unknown }
