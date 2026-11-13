//go:build unix

package safefs

import (
	"crypto/sha256"
	"errors"
	"os"
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

func TestCloneRefusesAFifo(t *testing.T) {
	dir := t.TempDir()
	pipe := filepath.Join(dir, "pipe")
	if err := syscall.Mkfifo(pipe, 0o600); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		// Releases an open that did block, so the goroutine does not outlive the test.
		if w, err := os.OpenFile(pipe, os.O_WRONLY|syscall.O_NONBLOCK, 0); err == nil {
			_ = w.Close()
		}
	})
	done := make(chan error, 1)
	go func() { done <- Clone(pipe, filepath.Join(dir, "out")) }()
	select {
	case err := <-done:
		if !errors.Is(err, ErrNotRegular) {
			t.Errorf("err = %v, want ErrNotRegular", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Clone blocked on a FIFO")
	}
}

// A file swapped for a pipe between the walk's check and the open must be refused, not block the copy.
func TestTreeCopiesDoNotBlockOnAPipeSwappedIn(t *testing.T) {
	src, spare := t.TempDir(), t.TempDir()
	name := filepath.Join(src, "f")
	stop := make(chan struct{})
	swapped := make(chan struct{})
	go func() {
		defer close(swapped)
		for i := 0; ; i++ {
			select {
			case <-stop:
				return
			default:
			}
			tmp := filepath.Join(spare, "x")
			if i%2 == 0 {
				_ = os.WriteFile(tmp, []byte("data"), 0o600)
			} else {
				_ = syscall.Mkfifo(tmp, 0o600)
			}
			_ = os.Rename(tmp, name)
		}
	}()
	defer func() { close(stop); <-swapped }()
	for name, run := range map[string]func(dst string) error{
		"CopyTree":               func(dst string) error { _, err := CopyTree(src, dst, CopyOptions{}); return err },
		"CopyTreeFollowingLinks": func(dst string) error { _, err := CopyTreeFollowingLinks(src, dst, CopyOptions{}); return err },
	} {
		var copied, refused int
		for deadline := time.Now().Add(time.Second); time.Now().Before(deadline); {
			done := make(chan error, 1)
			dst := t.TempDir()
			go func() { done <- run(dst) }()
			select {
			case err := <-done:
				if err == nil {
					copied++
				} else {
					refused++
				}
			case <-time.After(2 * time.Second):
				t.Fatalf("%s blocked on a pipe", name)
			}
		}
		t.Logf("%s: %d copies, %d refusals", name, copied, refused)
	}
}

// The atomic writers create their file the way os.WriteFile does, so the process umask narrows the requested bits.
func TestAtomicWritersApplyTheUmask(t *testing.T) {
	defer syscall.Umask(syscall.Umask(0o022))
	path := filepath.Join(t.TempDir(), "f")
	if err := WriteFileAtomic(path, []byte("x"), 0o666); err != nil {
		t.Fatal(err)
	}
	fi, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := fi.Mode().Perm(); got != 0o644 {
		t.Fatalf("mode %v, want -rw-r--r--", got)
	}
}
