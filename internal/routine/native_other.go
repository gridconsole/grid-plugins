//go:build !linux && !darwin && !windows

package routine

func NativeBackend() Backend { return Unsupported{} }

func NativePreviewOnly() bool { return false }
