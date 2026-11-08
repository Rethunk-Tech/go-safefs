package safefs

import (
	"fmt"
	"testing"

	"golang.org/x/sys/windows"
)

func TestIsDiskFullWindows(t *testing.T) {
	if !IsDiskFull(windows.ERROR_DISK_FULL) {
		t.Fatal("ERROR_DISK_FULL")
	}
	if !IsDiskFull(windows.ERROR_HANDLE_DISK_FULL) {
		t.Fatal("ERROR_HANDLE_DISK_FULL")
	}
	if !IsDiskFull(fmt.Errorf("write: %w", windows.ERROR_DISK_FULL)) {
		t.Fatal("wrapped ERROR_DISK_FULL")
	}
}
