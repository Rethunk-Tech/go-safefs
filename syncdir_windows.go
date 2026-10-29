//go:build windows

package safefs

// SyncDir does nothing: NTFS journals directory metadata and a directory handle cannot be flushed.
func SyncDir(string) error { return nil }
