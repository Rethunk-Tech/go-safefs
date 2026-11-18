//go:build unix

package safefs

import (
	"errors"
	"math"
	"testing"

	"golang.org/x/sys/unix"
)

func TestUnusableBlockCountsAreZeroOrAnError(t *testing.T) {
	if _, err := freeBytes(int64(0), int64(10)); !errors.Is(err, unix.EINVAL) {
		t.Fatalf("zero block size: %v, want EINVAL", err)
	}
	n, err := freeBytes(int64(4096), int64(-1))
	if err != nil || n != 0 {
		t.Fatalf("negative available blocks: %d, %v", n, err)
	}
	n, err = freeBytes(int64(2), int64(math.MaxInt64))
	if err != nil || n != math.MaxInt64 {
		t.Fatalf("overflow: %d, %v", n, err)
	}
}
