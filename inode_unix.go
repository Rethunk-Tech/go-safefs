//go:build unix

package safefs

import (
	"os"
	"syscall"
)

// HardLinkID returns the inode key of a file with more than one hard link.
func HardLinkID(_ string, info os.FileInfo) (FileID, bool) {
	st, ok := info.Sys().(*syscall.Stat_t)
	if !ok || st.Nlink < 2 {
		return FileID{}, false
	}
	return FileID{u64(st.Dev), u64(st.Ino), 0}, true
}

// u64 widens a stat field whose width differs per platform.
func u64[T ~uint32 | ~uint64 | ~int32 | ~int64](v T) uint64 { return uint64(v) }
