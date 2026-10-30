//go:build !windows && !darwin

package safefs

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
)

func TestTrashMovesIntoTheFreedesktopTrashWithItsOrigin(t *testing.T) {
	data := t.TempDir()
	t.Setenv("XDG_DATA_HOME", data)
	for range 2 {
		dir := filepath.Join(data, "My Mod")
		if err := os.MkdirAll(dir, 0o750); err != nil {
			t.Fatal(err)
		}
		if err := Trash(dir); err != nil {
			t.Fatal(err)
		}
		if _, err := os.Stat(dir); !os.IsNotExist(err) {
			t.Fatalf("%s is still there", dir)
		}
	}
	info, err := ReadFile(filepath.Join(data, "Trash", "info", "My Mod.2.trashinfo"))
	if err != nil || !strings.Contains(string(info), "Path="+filepath.ToSlash(data)+"/My%20Mod\n") {
		t.Fatalf("second trashinfo = %q, %v", info, err)
	}
	if !IsDir(filepath.Join(data, "Trash", "files", "My Mod")) || !IsDir(filepath.Join(data, "Trash", "files", "My Mod.2")) {
		t.Fatal("trashed folders missing")
	}
}

func TestTrashCopiesAFolderFromAnotherFilesystem(t *testing.T) {
	data := t.TempDir()
	t.Setenv("XDG_DATA_HOME", data)
	src := filepath.Join("/dev/shm", "safefs-trash-test-"+strconv.Itoa(os.Getpid()))
	if err := os.Mkdir(src, 0o700); err != nil {
		t.Skip("no /dev/shm:", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(src) })
	var a, b syscall.Stat_t
	if syscall.Stat(src, &a) != nil || syscall.Stat(data, &b) != nil || a.Dev == b.Dev {
		t.Skip("/dev/shm and the test's temp folder are one filesystem")
	}
	dir := filepath.Join(src, "Mod")
	if err := os.MkdirAll(filepath.Join(dir, "src"), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "src", "Plugin.cs"), []byte("class P {}"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("src/Plugin.cs", filepath.Join(dir, "link")); err != nil {
		t.Fatal(err)
	}
	if err := Trash(dir); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(dir); !os.IsNotExist(err) {
		t.Fatalf("%s is still there", dir)
	}
	got, err := ReadFile(filepath.Join(data, "Trash", "files", "Mod", "src", "Plugin.cs"))
	if err != nil || string(got) != "class P {}" {
		t.Fatalf("trashed file = %q, %v", got, err)
	}
	if target, err := os.Readlink(filepath.Join(data, "Trash", "files", "Mod", "link")); err != nil || target != "src/Plugin.cs" {
		t.Fatalf("trashed link = %q, %v", target, err)
	}
}

func TestTrashIgnoresARelativeXDGDataHome(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_DATA_HOME", "relative/share")
	victim := filepath.Join(t.TempDir(), "victim")
	if err := os.Mkdir(victim, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := Trash(victim); err != nil {
		t.Fatal(err)
	}
	if !IsDir(filepath.Join(home, ".local", "share", "Trash", "files", "victim")) {
		t.Fatal("not trashed under the home's default data folder")
	}
}

// copyAll is what a cross-filesystem Trash runs; the plain file branch and its refusals need no second filesystem.
func TestCopyAllCopiesAFileAndRefusesWhatCannotBeCopied(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "f")
	if err := os.WriteFile(src, []byte("data"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := copyAll(src, filepath.Join(dir, "copy")); err != nil {
		t.Fatal(err)
	}
	if got, err := os.ReadFile(filepath.Join(dir, "copy")); err != nil || string(got) != "data" { //nolint:gosec // G304: a temp file this test made
		t.Fatalf("copy = %q, %v", got, err)
	}
	tree := filepath.Join(dir, "tree")
	if err := os.Mkdir(tree, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := syscall.Mkfifo(filepath.Join(tree, "pipe"), 0o600); err != nil {
		t.Fatal(err)
	}
	for name, c := range map[string]struct{ from, to string }{
		"missing source":     {filepath.Join(dir, "none"), filepath.Join(dir, "x1")},
		"destination exists": {src, filepath.Join(dir, "copy")},
		"folder exists":      {tree, dir},
	} {
		if copyAll(c.from, c.to) == nil {
			t.Errorf("%s: no error", name)
		}
	}
	if err := copyAll(tree, filepath.Join(dir, "treecopy")); err != nil { // a pipe has nothing to keep
		t.Fatal(err)
	}
	if _, err := os.Lstat(filepath.Join(dir, "treecopy", "pipe")); !os.IsNotExist(err) {
		t.Errorf("the pipe was copied: %v", err)
	}
}
