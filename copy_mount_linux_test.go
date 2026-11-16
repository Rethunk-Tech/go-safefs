//go:build linux

package safefs

import (
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
	"testing"
)

const copyMountChildEnv = "SAFEFS_COPY_MOUNT_CHILD"

func TestCopyTreeSkipsAMountInside(t *testing.T) {
	if os.Getenv(copyMountChildEnv) != "" {
		copyMountChild(t)
		return
	}
	if _, err := exec.LookPath("unshare"); err != nil {
		t.Skip("no unshare:", err)
	}
	// #nosec G204 G702 -- unshare running this test binary
	cmd := exec.CommandContext(t.Context(), "unshare", "-rm", os.Args[0], "-test.run=^TestCopyTreeSkipsAMountInside$", "-test.v")
	cmd.Env = append(os.Environ(), copyMountChildEnv+"=1")
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

func copyMountChild(t *testing.T) {
	kept := t.TempDir()
	src := t.TempDir()
	writeAt(t, filepath.Join(kept, "precious"), "real")
	writeAt(t, filepath.Join(src, "own"), "mine")
	mount := filepath.Join(src, "m")
	if err := os.Mkdir(mount, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := syscall.Mount(kept, mount, "", syscall.MS_BIND, ""); err != nil {
		t.Skip("cannot mount here:", err)
	}
	t.Cleanup(func() { _ = syscall.Unmount(mount, 0) })
	for _, tc := range []struct {
		name string
		fn   func(src, dst string, opts CopyOptions) ([]string, error)
	}{
		{"CopyTree", CopyTree},
		{"CopyTreeFollowingLinks", CopyTreeFollowingLinks},
	} {
		dst := t.TempDir()
		skipped, err := tc.fn(src, dst, CopyOptions{})
		if err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		if !IsFile(filepath.Join(dst, "own")) {
			t.Fatalf("%s: own was not copied", tc.name)
		}
		if IsFile(filepath.Join(dst, "m", "precious")) {
			t.Fatalf("%s: copied a file through a mount", tc.name)
		}
		if !slices.Contains(skipped, mount) {
			t.Fatalf("%s: skipped = %v, want %s", tc.name, skipped, mount)
		}
	}
}

const copyMountLinkChildEnv = "SAFEFS_COPY_MOUNT_LINK_CHILD"

func TestCopyTreeSkipsAMountInsideALinkedSource(t *testing.T) {
	if os.Getenv(copyMountLinkChildEnv) != "" {
		copyMountLinkChild(t)
		return
	}
	if _, err := exec.LookPath("unshare"); err != nil {
		t.Skip("no unshare:", err)
	}
	// #nosec G204 G702 -- unshare running this test binary
	cmd := exec.CommandContext(t.Context(), "unshare", "-rm", os.Args[0], "-test.run=^TestCopyTreeSkipsAMountInsideALinkedSource$", "-test.v")
	cmd.Env = append(os.Environ(), copyMountLinkChildEnv+"=1")
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

func copyMountLinkChild(t *testing.T) {
	kept := t.TempDir()
	tree := t.TempDir()
	writeAt(t, filepath.Join(kept, "precious"), "real")
	writeAt(t, filepath.Join(tree, "own"), "mine")
	mount := filepath.Join(tree, "m")
	if err := os.Mkdir(mount, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := syscall.Mount(kept, mount, "", syscall.MS_BIND, ""); err != nil {
		t.Skip("cannot mount here:", err)
	}
	t.Cleanup(func() { _ = syscall.Unmount(mount, 0) })
	src := filepath.Join(t.TempDir(), "alias")
	if err := os.Symlink(tree, src); err != nil {
		t.Skip("cannot symlink here:", err)
	}
	for _, tc := range []struct {
		name string
		fn   func(src, dst string, opts CopyOptions) ([]string, error)
	}{
		{"CopyTree", CopyTree},
		{"CopyTreeFollowingLinks", CopyTreeFollowingLinks},
	} {
		dst := t.TempDir()
		skipped, err := tc.fn(src, dst, CopyOptions{})
		if err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		if !IsFile(filepath.Join(dst, "own")) {
			t.Fatalf("%s: own was not copied", tc.name)
		}
		if IsFile(filepath.Join(dst, "m", "precious")) {
			t.Fatalf("%s: copied a file through a mount", tc.name)
		}
		if want := filepath.Join(src, "m"); !slices.Contains(skipped, want) {
			t.Fatalf("%s: skipped = %v, want %s", tc.name, skipped, want)
		}
	}
}
