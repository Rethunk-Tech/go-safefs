package safefs

import (
	"os"
	"path/filepath"
	"testing"
)

func TestUnshareGivesALinkedFileItsOwnCopy(t *testing.T) {
	source, target := t.TempDir(), t.TempDir()
	orig := filepath.Join(source, "tool.bin")
	if err := os.WriteFile(orig, []byte("orig"), 0o600); err != nil {
		t.Fatal(err)
	}
	hard, own := filepath.Join(target, "hard.bin"), filepath.Join(target, "own.bin")
	if err := os.Link(orig, hard); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(own, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	paths := []string{hard, own}
	if !underWine() {
		sym := filepath.Join(target, "sym.bin")
		if err := os.Symlink(orig, sym); err != nil {
			t.Fatal(err)
		}
		if !Shared(sym) {
			t.Fatal("Shared: symlink not shared")
		}
		paths = append([]string{sym}, paths...)
	}
	if !Shared(hard) || Shared(own) {
		t.Fatalf("Shared: hard %v own %v", Shared(hard), Shared(own))
	}
	for _, p := range paths {
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
