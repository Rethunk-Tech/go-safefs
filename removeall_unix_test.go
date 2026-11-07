//go:build linux

package safefs

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

const mountChildEnv = "SAFEFS_MOUNT_CHILD"

// RemoveAll must not empty a filesystem mounted below the folder, bind mounts included. The mount is made in a
// private namespace (unshare -rm), so the host's mount table is untouched; the child is this test binary.
func TestRemoveAllRefusesAMountInside(t *testing.T) {
	if os.Getenv(mountChildEnv) != "" {
		mountChild(t, t.TempDir())
		return
	}
	if _, err := exec.LookPath("unshare"); err != nil {
		t.Skip("no unshare:", err)
	}
	// #nosec G204 G702 -- unshare running this test binary
	cmd := exec.CommandContext(t.Context(), "unshare", "-rm", os.Args[0], "-test.run=^TestRemoveAllRefusesAMountInside$", "-test.v")
	cmd.Env = append(os.Environ(), mountChildEnv+"=1")
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

func mountChild(t *testing.T, dir string) {
	kept := filepath.Join(dir, "kept")
	tree := filepath.Join(dir, "tree")
	writeAt(t, filepath.Join(kept, "precious"), "real")
	writeAt(t, filepath.Join(tree, "own"), "mine")
	if err := os.Mkdir(filepath.Join(tree, "m"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := syscall.Mount(kept, filepath.Join(tree, "m"), "", syscall.MS_BIND, ""); err != nil {
		t.Skip("cannot mount here:", err)
	}
	err := RemoveAll(tree)
	if !errors.Is(err, ErrMountInside) {
		t.Fatalf("RemoveAll = %v, want ErrMountInside", err)
	}
	if !IsFile(filepath.Join(tree, "own")) || !IsFile(filepath.Join(kept, "precious")) {
		t.Fatal("a refused RemoveAll deleted something")
	}
	_ = syscall.Unmount(filepath.Join(tree, "m"), 0)
	if err := RemoveAll(tree); err != nil {
		t.Fatalf("RemoveAll without the mount: %v", err)
	}
}
