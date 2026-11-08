//go:build unix

package safefs

import (
	"errors"
	"syscall"
)

func platformDiskFull(err error) bool {
	return errors.Is(err, syscall.EDQUOT)
}
