//go:build !windows

package safefs

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

// Opening a pipe for reading blocks until a writer shows up, so Unshare of a link to one must not open it blindly.
func TestUnshareOfALinkToAPipeReturnsInsteadOfBlocking(t *testing.T) {
	dir := t.TempDir()
	fifo := filepath.Join(dir, "pipe")
	if err := syscall.Mkfifo(fifo, 0o600); err != nil {
		t.Skip(err)
	}
	link := filepath.Join(dir, "link")
	if err := os.Symlink(fifo, link); err != nil {
		t.Skip(err)
	}
	done := make(chan error, 1)
	go func() { done <- Unshare(link) }()
	select {
	case err := <-done:
		t.Log(err)
	case <-time.After(2 * time.Second):
		t.Fatal("Unshare blocked opening a pipe")
	}
}

func TestReadFileCappedRefusesAPipeWithoutBlockingAndCapsAFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	pipe := filepath.Join(dir, "pipe")
	if err := syscall.Mkfifo(pipe, 0o600); err != nil {
		t.Skip(err)
	}
	if _, err := ReadFileCapped(pipe, 10); !errors.Is(err, ErrNotRegular) {
		t.Fatalf("pipe: err = %v", err)
	}
	file := writeFile(t, dir, "f", "abcd")
	if b, err := ReadFileCapped(file, 4); err != nil || string(b) != "abcd" {
		t.Fatalf("at the limit: %q, %v", b, err)
	}
	if _, err := ReadFileCapped(file, 3); !errors.Is(err, ErrTooLarge) {
		t.Fatalf("over the limit: err = %v", err)
	}
}
