//go:build unix

package safefs

import (
	"crypto/sha256"
	"errors"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

// A pipe has no bytes to copy and reading one could block, so both tree copies refuse it; a missing source fails
// before any walk, and Skip leaves a named file out of the following copy.
func TestTreeCopiesRefuseWhatIsNotAFile(t *testing.T) {
	src := t.TempDir()
	if err := syscall.Mkfifo(filepath.Join(src, "pipe"), 0o600); err != nil {
		t.Fatal(err)
	}
	gone := filepath.Join(src, "gone")
	for name, run := range map[string]func() error{
		"CopyTree":                func() error { _, err := CopyTree(src, t.TempDir(), CopyOptions{}); return err },
		"CopyTreeFollowingLinks":  func() error { _, err := CopyTreeFollowingLinks(src, t.TempDir(), CopyOptions{}); return err },
		"CopyTree missing source": func() error { _, err := CopyTree(gone, t.TempDir(), CopyOptions{}); return err },
		"CopyTree missing, with progress": func() error {
			_, err := CopyTree(gone, t.TempDir(), CopyOptions{Progress: func(CopyProgress) {}})
			return err
		},
		"CopyTreeFollowingLinks missing": func() error { _, err := CopyTreeFollowingLinks(gone, t.TempDir(), CopyOptions{}); return err },
	} {
		err := run()
		if err == nil {
			t.Errorf("%s: no error", name)
		} else if strings.HasPrefix(name, "CopyTree") && !strings.Contains(name, "missing") && !strings.Contains(err.Error(), "not a regular file") {
			t.Errorf("%s: %v", name, err)
		}
	}
	skip := CopyOptions{Skip: func(name string) bool { return name == "pipe" }}
	if _, err := CopyTreeFollowingLinks(src, t.TempDir(), skip); err != nil {
		t.Errorf("a skipped pipe still failed: %v", err)
	}
}

func TestCopyFileAndHashFileRefuseAFifo(t *testing.T) {
	dir := t.TempDir()
	pipe := filepath.Join(dir, "pipe")
	if err := syscall.Mkfifo(pipe, 0o600); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 2)
	go func() { done <- CopyFile(pipe, filepath.Join(dir, "out"), CopyOptions{}) }()
	go func() { _, err := HashFile(pipe, sha256.New()); done <- err }()
	timer := time.NewTimer(2 * time.Second)
	defer timer.Stop()
	for range 2 {
		select {
		case err := <-done:
			if !errors.Is(err, ErrNotRegular) {
				t.Errorf("err = %v, want ErrNotRegular", err)
			}
		case <-timer.C:
			t.Fatal("blocked on a FIFO")
		}
	}
}
