//go:build netbsd

package safefs

import "golang.org/x/sys/unix"

// FreeBytes returns the bytes available to an unprivileged writer on the volume holding path.
func FreeBytes(path string) (int64, error) {
	var st unix.Statvfs_t
	if err := unix.Statvfs(path, &st); err != nil {
		return 0, err
	}
	return freeBytes(st.Frsize, st.Bavail)
}
