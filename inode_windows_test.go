//go:build windows

package safefs

import (
	"os"
	"path/filepath"
	"testing"
)

func TestHardLinkIDWindows(t *testing.T) {
	dir := t.TempDir()
	a, b, c := filepath.Join(dir, "a"), filepath.Join(dir, "b"), filepath.Join(dir, "c")
	for _, p := range []string{a, c} {
		if err := os.WriteFile(p, []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if _, ok := HardLinkID(a, mustStat(t, a)); ok {
		t.Fatal("unlinked file reported as linked")
	}
	if err := os.Link(a, b); err != nil {
		t.Skipf("hard links unavailable: %v", err)
	}
	ka, okA := HardLinkID(a, mustStat(t, a))
	kb, okB := HardLinkID(b, mustStat(t, b))
	if !okA || !okB || ka != kb {
		t.Fatalf("names of one file got keys %v/%v (%v/%v)", ka, kb, okA, okB)
	}
	if kc, ok := HardLinkID(c, mustStat(t, c)); ok || kc == ka {
		t.Fatal("distinct file shares a key")
	}
}

func mustStat(t *testing.T, p string) os.FileInfo {
	t.Helper()
	info, err := os.Stat(p)
	if err != nil {
		t.Fatal(err)
	}
	return info
}
