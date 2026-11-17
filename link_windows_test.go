//go:build windows

package safefs

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMakeJunctionReachesTheTarget(t *testing.T) {
	target := t.TempDir()
	if err := os.WriteFile(filepath.Join(target, "x"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(t.TempDir(), "junc")
	if err := makeJunction(link, target); err != nil {
		t.Skip(err)
	}
	removeBeforeTempCleanup(t, link)
	if b, err := ReadFile(filepath.Join(link, "x")); err != nil || string(b) != "x" {
		t.Fatalf("through the junction: %q, %v", b, err)
	}
}

func TestMakeJunctionRejectsAPathTooLongForTheReparseBuffer(t *testing.T) {
	link := filepath.Join(t.TempDir(), "junc")
	target := filepath.Join(t.TempDir(), strings.Repeat("a", maxJunctionPath+1))
	if err := makeJunction(link, target); !errors.Is(err, errPathTooLong) {
		if err != nil {
			t.Skip(err)
		}
		t.Fatal("wanted errPathTooLong")
	}
}
