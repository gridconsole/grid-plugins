//go:build windows

package atomicfile

// SyncDir does nothing: Windows cannot open a folder to sync it, and NTFS
// journals the rename.
func SyncDir(string) error { return nil }
