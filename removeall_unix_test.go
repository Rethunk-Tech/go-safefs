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

const tmpfsChildEnv = "SAFEFS_TMPFS_CHILD"

// Bind mounts share a device id, so the mount-table miss path has to see a real filesystem (tmpfs) to refuse.
func TestATmpfsChildIsAMountEvenWithoutTheMountTable(t *testing.T) {
	if os.Getenv(tmpfsChildEnv) != "" {
		tmpfsChild(t, t.TempDir())
		return
	}
	if _, err := exec.LookPath("unshare"); err != nil {
		t.Skip("no unshare:", err)
	}
	// #nosec G204 G702 -- unshare running this test binary
	cmd := exec.CommandContext(t.Context(), "unshare", "-rm", os.Args[0], "-test.run=^TestATmpfsChildIsAMountEvenWithoutTheMountTable$", "-test.v")
	cmd.Env = append(os.Environ(), tmpfsChildEnv+"=1")
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

func tmpfsChild(t *testing.T, dir string) {
	tree := filepath.Join(dir, "tree")
	writeAt(t, filepath.Join(tree, "own"), "mine")
	m := filepath.Join(tree, "m")
	if err := os.Mkdir(m, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := syscall.Mount("tmpfs", m, "tmpfs", 0, ""); err != nil {
		t.Skip("cannot mount tmpfs:", err)
	}
	t.Cleanup(func() { _ = syscall.Unmount(m, 0) })

	info, err := os.Lstat(tree)
	if err != nil {
		t.Fatal(err)
	}
	if err := deviceChangesBelow(tree, info); !errors.Is(err, ErrMountInside) {
		t.Fatalf("tmpfs child: %v, want ErrMountInside", err)
	}

	link := filepath.Join(dir, "link")
	if err := os.Symlink(tree, link); err != nil {
		t.Fatal(err)
	}
	if !mountedDir(link, filepath.Join(link, "m")) {
		t.Fatal("a tmpfs under a linked src was not a mount")
	}
	if mountedDir(tree, tree) {
		t.Fatal("src itself counted as a mount")
	}

	// Hide mountinfo so checkDir falls through to device ids.
	if err := syscall.Mount("tmpfs", "/proc", "tmpfs", 0, ""); err != nil {
		t.Fatalf("hide mountinfo: %v", err)
	}
	if !mountedDir(tree, m) {
		t.Fatal("device-id fallback missed the tmpfs")
	}
	if err := checkDir(tree, info); !errors.Is(err, ErrMountInside) {
		t.Fatalf("without mountinfo: %v, want ErrMountInside", err)
	}
}

func TestASameDeviceTreeIsNotAMountInside(t *testing.T) {
	dir := t.TempDir()
	writeAt(t, filepath.Join(dir, "a", "f"), "x")
	info, err := os.Lstat(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := deviceChangesBelow(dir, info); err != nil {
		t.Fatalf("one device: %v", err)
	}
	if err := deviceChangesBelow(filepath.Join(dir, "missing"), info); err != nil {
		t.Fatalf("missing path: %v", err)
	}
	if err := deviceChangesBelow(dir, sysInfo{info, nil}); err != nil {
		t.Fatalf("non-stat FileInfo: %v", err)
	}
	if mountedDir(dir, dir) {
		t.Fatal("src itself counted as a mount")
	}

	locked := filepath.Join(dir, "locked")
	if err := os.Mkdir(locked, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(locked, 0o700) })
	if err := os.Chmod(locked, 0); err != nil {
		t.Fatal(err)
	}
	if err := deviceChangesBelow(dir, info); errors.Is(err, ErrMountInside) {
		t.Fatalf("unreadable child treated as a mount: %v", err)
	}

	link := filepath.Join(t.TempDir(), "link")
	if err := os.Symlink(dir, link); err != nil {
		t.Fatal(err)
	}
	if mountedDir(link, filepath.Join(link, "a")) {
		t.Fatal("a linked src's child looked like a mount")
	}
	if mountedDir(dir, t.TempDir()) {
		t.Fatal("an unrelated path looked like a mount")
	}

	// mountsBelow EvalSymlinks's the parent, so a missing leaf under a dangling name is a table miss.
	dangling := filepath.Join(dir, "gone")
	if err := os.Symlink("no-such", dangling); err != nil {
		t.Fatal(err)
	}
	ghost := filepath.Join(dangling, "tree")
	if mountedDir(ghost, filepath.Join(ghost, "m")) {
		t.Fatal("a dangling src looked like a mount")
	}
	if err := checkDir(ghost, info); err != nil {
		t.Fatalf("mount-table miss: %v", err)
	}
}

func TestAMismatchedDeviceIdIsAMountInside(t *testing.T) {
	dir := t.TempDir()
	writeAt(t, filepath.Join(dir, "a", "f"), "x")
	info, err := os.Lstat(dir)
	if err != nil {
		t.Fatal(err)
	}
	st, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		t.Fatal("Lstat Sys is not Stat_t")
	}
	foreign := *st
	foreign.Dev++
	if err := deviceChangesBelow(dir, sysInfo{info, &foreign}); !errors.Is(err, ErrMountInside) {
		t.Fatalf("mismatched device id: %v, want ErrMountInside", err)
	}
	if mounts, err := mountsBelow("/"); err == nil && len(mounts) > 0 {
		root, err := os.Lstat("/")
		if err != nil {
			t.Fatal(err)
		}
		if err := checkDir("/", root); !errors.Is(err, ErrMountInside) {
			t.Fatalf("root with %d mounts below: %v", len(mounts), err)
		}
	}
}

type sysInfo struct {
	os.FileInfo
	sys any
}

func (s sysInfo) Sys() any { return s.sys }
