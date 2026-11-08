//go:build windows

package atomicfile

// syncDir does nothing: Windows cannot open a folder to sync it, and NTFS
// journals the rename.
func syncDir(string) error { return nil }
