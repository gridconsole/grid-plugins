//go:build windows

package routine

func NativeBackend() Backend { return Schtasks{} }

// The shared lock implementation deliberately refuses native Windows writes.
// Render a reviewable Task Scheduler plan without mutating files or OS state.
func NativePreviewOnly() bool { return true }
