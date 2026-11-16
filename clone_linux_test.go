//go:build linux

package safefs

import (
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/sys/unix"
)

func TestLinkFallbackOnFATRefusal(t *testing.T) {
	if !LinkUnsupported(unix.EPERM) {
		t.Fatal("EPERM from a link on FAT/exFAT must fall through to a copy")
	}
}

func TestMaterializeCloneKeepsSourceMode(t *testing.T) {
	src := t.TempDir()
	p := writeFile(t, src, "run.sh", "#!/bin/sh\n")
	if err := os.Chmod(p, 0o755); err != nil {
		t.Fatal(err)
	}
	m := Materializer{Tiers: []Tier{TierClone}}
	dst := t.TempDir()
	err := m.Tree(src, dst)
	if err != nil {
		if LinkUnsupported(err) {
			t.Skipf("no clone here: %v", err)
		}
		t.Fatal(err)
	}
	if st, err := os.Stat(filepath.Join(dst, "run.sh")); err != nil || st.Mode().Perm() != 0o755 {
		t.Fatalf("Tree mode = %v, %v; want 0755", st.Mode(), err)
	}
	one := filepath.Join(t.TempDir(), "run.sh")
	if err := m.File(p, one, "run.sh"); err != nil {
		t.Fatal(err)
	}
	if st, err := os.Stat(one); err != nil || st.Mode().Perm() != 0o755 {
		t.Fatalf("File mode = %v, %v; want 0755", st.Mode(), err)
	}
}
