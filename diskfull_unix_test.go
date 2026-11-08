//go:build unix

package safefs

import (
	"fmt"
	"syscall"
	"testing"
)

func TestIsDiskFullQuota(t *testing.T) {
	if !IsDiskFull(syscall.EDQUOT) {
		t.Fatal("EDQUOT")
	}
	if !IsDiskFull(fmt.Errorf("write: %w", syscall.EDQUOT)) {
		t.Fatal("wrapped EDQUOT")
	}
}
