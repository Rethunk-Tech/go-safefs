//go:build unix && !openbsd && !netbsd

package safefs

import "golang.org/x/sys/unix"

// FreeBytes returns the bytes available to an unprivileged writer on the volume holding path.
func FreeBytes(path string) (int64, error) {
	var st unix.Statfs_t
	if err := unix.Statfs(path, &st); err != nil {
		return 0, err
	}
	return freeBytes(st.Bsize, st.Bavail)
}
