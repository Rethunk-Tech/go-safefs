//go:build unix

package safefs

import (
	"path/filepath"
	"strings"
	"syscall"
	"testing"
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
		"CopyTreeFollowingLinks":  func() error { return CopyTreeFollowingLinks(src, t.TempDir(), CopyOptions{}) },
		"CopyTree missing source": func() error { _, err := CopyTree(gone, t.TempDir(), CopyOptions{}); return err },
		"CopyTree missing, with progress": func() error {
			_, err := CopyTree(gone, t.TempDir(), CopyOptions{Progress: func(CopyProgress) {}})
			return err
		},
		"CopyTreeFollowingLinks missing": func() error { return CopyTreeFollowingLinks(gone, t.TempDir(), CopyOptions{}) },
	} {
		err := run()
		if err == nil {
			t.Errorf("%s: no error", name)
		} else if strings.HasPrefix(name, "CopyTree") && !strings.Contains(name, "missing") && !strings.Contains(err.Error(), "not a regular file") {
			t.Errorf("%s: %v", name, err)
		}
	}
	skip := CopyOptions{Skip: func(name string) bool { return name == "pipe" }}
	if err := CopyTreeFollowingLinks(src, t.TempDir(), skip); err != nil {
		t.Errorf("a skipped pipe still failed: %v", err)
	}
}
