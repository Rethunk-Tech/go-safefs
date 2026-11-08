package safefs

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"syscall"
	"testing"
)

func TestIsDiskFull(t *testing.T) {
	if IsDiskFull(nil) {
		t.Fatal("nil")
	}
	if !IsDiskFull(syscall.ENOSPC) {
		t.Fatal("ENOSPC")
	}
	if !IsDiskFull(fmt.Errorf("write: %w", syscall.ENOSPC)) {
		t.Fatal("wrapped ENOSPC")
	}
	if !IsDiskFull(&os.PathError{Op: "write", Path: "f", Err: syscall.ENOSPC}) {
		t.Fatal("PathError ENOSPC")
	}
	if IsDiskFull(fs.ErrPermission) {
		t.Fatal("permission")
	}
	if IsDiskFull(errors.New("plain")) {
		t.Fatal("plain")
	}
}
