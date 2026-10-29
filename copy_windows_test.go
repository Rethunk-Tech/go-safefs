//go:build windows

package safefs

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestCopyTreeSkipsAJunction(t *testing.T) {
	src, outside := t.TempDir(), t.TempDir()
	writeFile(t, outside, "secret", "x")
	writeFile(t, src, "a.txt", "a")
	junc := filepath.Join(src, "junc")
	if out, err := exec.CommandContext(t.Context(), "cmd", "/c", "mklink", "/J", junc, outside).CombinedOutput(); err != nil { //nolint:gosec // G204: cmd /c mklink with temp dirs this test created
		t.Skipf("junction not permitted: %v %s", err, out)
	}
	dst := t.TempDir()
	skipped, err := CopyTree(src, dst, CopyOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(skipped) != 1 || skipped[0] != junc {
		t.Fatalf("skipped = %v", skipped)
	}
	if _, err := os.Stat(filepath.Join(dst, "a.txt")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dst, "junc")); err == nil {
		t.Fatal("junction was copied")
	}
}
