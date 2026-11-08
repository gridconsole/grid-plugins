//go:build darwin

package routine

import "os"

func NativeBackend() Backend { return Launchd{UID: os.Getuid()} }

func NativePreviewOnly() bool { return false }
