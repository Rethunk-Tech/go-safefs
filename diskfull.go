package safefs

import (
	"errors"
	"syscall"
)

// IsDiskFull reports whether err is a write that ran out of space, on any platform.
func IsDiskFull(err error) bool {
	return errors.Is(err, syscall.ENOSPC) || platformDiskFull(err)
}
