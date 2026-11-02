//go:build darwin

package safefs

import (
	"errors"

	"golang.org/x/sys/unix"
)

// Clone copies src to a new dst with clonefile(2), sharing blocks on APFS. clonefile creates dst itself and fails
// with EEXIST when it exists, so nothing is overwritten. dst is then set to mode 0o600, as on Linux, and removed again
// if that fails.
func Clone(src, dst string) error {
	// Open refuses a symlinked src; CLONE_NOFOLLOW keeps the clone from following one swapped in afterwards.
	in, err := Open(src)
	if err != nil {
		return err
	}
	_ = in.Close()
	if err := unix.Clonefile(src, dst, unix.CLONE_NOFOLLOW); err != nil {
		return err
	}
	// clonefile copies src's mode and has no mode argument.
	if err := unix.Fchmodat(unix.AT_FDCWD, dst, 0o600, unix.AT_SYMLINK_NOFOLLOW); err != nil {
		_ = unix.Unlink(dst)
		return err
	}
	return nil
}

// LinkUnsupported reports an error that means the target cannot clone or link, so the caller falls back to a copy.
func LinkUnsupported(err error) bool {
	return errors.Is(err, unix.EXDEV) || errors.Is(err, unix.ENOTSUP) || errors.Is(err, unix.EINVAL) ||
		errors.Is(err, unix.ENOSYS) || errors.Is(err, unix.EPERM)
}

func isCrossDevice(err error) bool {
	return errors.Is(err, unix.EXDEV)
}

// Device returns the id of the device holding path.
func Device(path string) (uint64, error) {
	var st unix.Stat_t
	if err := unix.Stat(path, &st); err != nil {
		return 0, err
	}
	return uint64(st.Dev), nil
}
