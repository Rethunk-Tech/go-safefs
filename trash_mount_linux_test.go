//go:build linux

package safefs

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
)

const trashMountChildEnv = "SAFEFS_TRASH_MOUNT_CHILD"

func TestTrashRefusesAMountInside(t *testing.T) {
	if os.Getenv(trashMountChildEnv) != "" {
		trashMountChild(t)
		return
	}
	if _, err := exec.LookPath("unshare"); err != nil {
		t.Skip("no unshare:", err)
	}
	// #nosec G204 G702 -- unshare running this test binary
	cmd := exec.CommandContext(t.Context(), "unshare", "-rm", os.Args[0], "-test.run=^TestTrashRefusesAMountInside$", "-test.v")
	cmd.Env = append(os.Environ(), trashMountChildEnv+"=1")
	out, err := cmd.CombinedOutput()
	if err != nil && strings.Contains(string(out), "unshare:") {
		t.Skipf("no private mount namespace: %v: %s", err, out)
	}
	if err != nil {
		t.Fatalf("child: %v\n%s", err, out)
	}
	if strings.Contains(string(out), "--- SKIP") {
		t.Skipf("child skipped: %s", out)
	}
}

func trashMountChild(t *testing.T) {
	data := t.TempDir()
	t.Setenv("XDG_DATA_HOME", data)
	src := filepath.Join("/dev/shm", "safefs-trash-mount-"+strconv.Itoa(os.Getpid()))
	if err := os.Mkdir(src, 0o700); err != nil {
		t.Skip("no /dev/shm:", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(src) })
	var a, b syscall.Stat_t
	if syscall.Stat(src, &a) != nil || syscall.Stat(data, &b) != nil || a.Dev == b.Dev {
		t.Skip("/dev/shm and the test's temp folder are one filesystem")
	}
	kept := filepath.Join(data, "kept")
	tree := filepath.Join(src, "tree")
	writeAt(t, filepath.Join(kept, "precious"), "real")
	writeAt(t, filepath.Join(tree, "own"), "mine")
	if err := os.Mkdir(filepath.Join(tree, "m"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := syscall.Mount(kept, filepath.Join(tree, "m"), "", syscall.MS_BIND, ""); err != nil {
		t.Skip("cannot mount here:", err)
	}
	err := Trash(tree)
	if !errors.Is(err, ErrMountInside) {
		t.Fatalf("Trash = %v, want ErrMountInside", err)
	}
	if !IsFile(filepath.Join(tree, "own")) || !IsFile(filepath.Join(kept, "precious")) {
		t.Fatal("a refused Trash deleted something")
	}
	_ = syscall.Unmount(filepath.Join(tree, "m"), 0)
}
