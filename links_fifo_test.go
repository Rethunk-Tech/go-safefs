//go:build !windows

package safefs

import (
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
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Unshare blocked opening a pipe")
	}
}
