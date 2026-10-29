package safefs

import (
	"os"
	"path/filepath"
	"testing"
)

func TestUnshareGivesALinkedFileItsOwnCopy(t *testing.T) {
	source, target := t.TempDir(), t.TempDir()
	orig := filepath.Join(source, "tool.dll")
	if err := os.WriteFile(orig, []byte("orig"), 0o600); err != nil {
		t.Fatal(err)
	}
	sym, hard, own := filepath.Join(target, "sym.dll"), filepath.Join(target, "hard.dll"), filepath.Join(target, "own.dll")
	if err := os.Symlink(orig, sym); err != nil {
		t.Fatal(err)
	}
	if err := os.Link(orig, hard); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(own, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if !Shared(sym) || !Shared(hard) || Shared(own) {
		t.Fatalf("Shared: sym %v hard %v own %v", Shared(sym), Shared(hard), Shared(own))
	}
	for _, p := range []string{sym, hard, own} {
		if err := Unshare(p); err != nil {
			t.Fatal(err)
		}
		if Shared(p) {
			t.Fatalf("%s still shared", p)
		}
		if err := os.WriteFile(p, []byte("data"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if b, _ := ReadFile(orig); string(b) != "orig" {
		t.Fatalf("source file changed to %q", b)
	}
}
