//go:build linux

package safefs

import (
	"errors"

	"golang.org/x/sys/unix"
)

// Clone copies src to a new dst with FICLONE, sharing extents on a filesystem that supports it. dst is created
// exclusively with mode 0o600 and removed again on failure.
func Clone(src, dst string) error {
	in, err := Open(src)
	if err != nil {
		return err
	}
	defer func() { _ = in.Close() }()
	out, err := CreateExcl(dst, 0o600)
	if err != nil {
		return err
	}
	if err := unix.IoctlFileClone(int(out.Fd()), int(in.Fd())); err != nil {
		_ = out.Close()
		_ = Remove(dst)
		return err
	}
	return out.Close()
}

// LinkUnsupported reports an error that means the target cannot clone or link, so the caller falls back to a copy.
func LinkUnsupported(err error) bool {
	return errors.Is(err, unix.EXDEV) || errors.Is(err, unix.EOPNOTSUPP) || errors.Is(err, unix.EINVAL) ||
		errors.Is(err, unix.ENOTSUP) || errors.Is(err, unix.ENOSYS) || errors.Is(err, unix.ENOTTY) ||
		errors.Is(err, unix.EPERM)
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
	return st.Dev, nil
}
