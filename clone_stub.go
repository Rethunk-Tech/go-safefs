//go:build !linux && !windows

package safefs

import (
	"errors"
	"syscall"
)

// Clone is unsupported here; the error is errors.ErrUnsupported through errors.Is, so a caller falls back to a copy.
func Clone(string, string) error { return syscall.EOPNOTSUPP }

// LinkUnsupported reports an error that means the target cannot clone or link, so the caller falls back to a copy.
// The errors arrive wrapped in an *os.LinkError, so they are matched through the chain.
func LinkUnsupported(err error) bool {
	return errors.Is(err, syscall.EXDEV) || errors.Is(err, syscall.EOPNOTSUPP) || errors.Is(err, syscall.EINVAL) ||
		errors.Is(err, syscall.EPERM)
}

func isCrossDevice(err error) bool { return errors.Is(err, syscall.EXDEV) }

// Device is unsupported here.
func Device(string) (uint64, error) { return 0, syscall.EOPNOTSUPP }
