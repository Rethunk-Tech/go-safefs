//go:build unix

package safefs

import (
	"crypto/sha256"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// Early returns a caller can hit: a path that is not there, a folder where a file is meant, a callback that fails.
func TestErrorPathsOfSmallHelpers(t *testing.T) {
	dir := t.TempDir()
	gone := filepath.Join(dir, "gone", "f")
	if _, err := HashFile(gone, sha256.New()); err == nil {
		t.Error("HashFile of a missing file")
	}
	if _, err := HashFile(dir, sha256.New()); err == nil {
		t.Error("HashFile of a folder")
	}
	if err := WalkFiles(filepath.Join(dir, "gone"), func(string, string, int64) error { return nil }); err == nil {
		t.Error("WalkFiles of a missing root")
	}
	writeFile(t, dir, "a", "x")
	boom := errors.New("boom")
	if err := WalkFiles(dir, func(string, string, int64) error { return boom }); !errors.Is(err, boom) {
		t.Errorf("WalkFiles callback error: %v", err)
	}
	if err := CheckWritable(filepath.Join(dir, "gone")); err != nil {
		t.Errorf("a missing folder is not a permission problem: %v", err)
	}
	if err := CheckWritable(filepath.Join(dir, "a")); err == nil {
		t.Error("a file is not a writable folder")
	}
	if Shared(filepath.Join(dir, "gone")) {
		t.Error("a missing path is not shared")
	}
	if err := Unshare(filepath.Join(dir, "gone")); err != nil {
		t.Errorf("unsharing nothing: %v", err)
	}
	link := filepath.Join(dir, "dirlink")
	if err := os.Symlink(dir, link); err != nil {
		t.Fatal(err)
	}
	if !Shared(link) {
		t.Error("a symlink is shared")
	}
	if err := Unshare(link); err != nil { // a link to a folder has no bytes to own
		t.Errorf("Unshare of a folder link: %v", err)
	}
	if err := Unshare(filepath.Join(dir, "dirlink", "gone")); err != nil {
		t.Errorf("Unshare below a link: %v", err)
	}
}
