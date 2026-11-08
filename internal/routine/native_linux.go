//go:build linux

package routine

func NativeBackend() Backend { return Systemd{} }

func NativePreviewOnly() bool { return false }
