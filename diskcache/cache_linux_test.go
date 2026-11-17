//go:build linux

package diskcache_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Rethunk-Tech/go-safefs/diskcache"
)

func TestPeekDoesNotFollowASymlinkAtTheEntryPath(t *testing.T) {
	t.Parallel()
	c, _ := newCache(t)
	body := `{"fetched":"2026-01-01T12:00:00Z","value":"leaked"}`
	if err := os.WriteFile(filepath.Join(c.Dir, "real"), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("real", c.Path("k")); err != nil {
		t.Fatal(err)
	}
	if e, ok := diskcache.Peek[string](c, "k"); ok {
		t.Fatalf("Peek followed a symlink at the entry path: %+v", e)
	}
	e, ok := diskcache.Peek[string](c, "real")
	if !ok || e.Value != "leaked" {
		t.Fatalf("regular entry = %+v, %v", e, ok)
	}
}

func TestPutDoesNotWriteThroughADestSymlink(t *testing.T) {
	t.Parallel()
	c, _ := newCache(t)
	victim := filepath.Join(t.TempDir(), "victim")
	if err := os.WriteFile(victim, []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(victim, c.Path("k")); err != nil {
		t.Fatal(err)
	}
	if err := diskcache.Put(c, "k", "new"); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(victim)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "keep" {
		t.Fatalf("Put wrote through a dest symlink: %q", got)
	}

	real := filepath.Join(c.Dir, "real")
	if err := os.Mkdir(real, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("real", filepath.Join(c.Dir, "sub")); err != nil {
		t.Fatal(err)
	}
	if err := diskcache.Put(c, "sub/k", 1); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(real, "k")); err == nil {
		t.Fatal("Put wrote through a dest dir symlink")
	}
}
