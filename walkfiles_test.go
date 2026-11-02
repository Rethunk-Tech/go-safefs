package safefs

import (
	"crypto/sha512"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

func TestWalkFilesGivesSlashPathsAndSizesOfRegularFilesOnly(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "a", "b"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "a", "b", "f.txt"), []byte("abc"), 0o600); err != nil {
		t.Fatal(err)
	}
	got := map[string]int64{}
	if err := WalkFiles(root, func(rel, _ string, size int64) error { got[rel] = size; return nil }); err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got["a/b/f.txt"] != 3 {
		t.Fatalf("WalkFiles = %v", got)
	}
}

func TestHashFileHashesTheFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "f")
	if err := os.WriteFile(path, []byte("abc"), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := HashFile(path, sha512.New())
	if err != nil || got[:16] != "ddaf35a193617aba" {
		t.Fatalf("HashFile = %s, %v", got, err)
	}
}

func TestWalkRegularSkipsLinksAndToleratesAMissingRoot(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	if err := os.WriteFile(filepath.Join(outside, "secret"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "d"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "d", "f"), []byte("a"), 0o600); err != nil {
		t.Fatal(err)
	}
	// Symlink creation needs privilege on Windows; the regular-file and missing-root cases still run.
	linked := os.Symlink(outside, filepath.Join(root, "link")) == nil
	var got []string
	collect := func(p string, _ fs.DirEntry) error { got = append(got, filepath.Base(p)); return nil }
	if err := WalkRegular(root, collect); err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0] != "f" {
		t.Fatalf("WalkRegular (links %v) = %v", linked, got)
	}
	if err := WalkRegular(filepath.Join(root, "nope"), collect); err != nil {
		t.Fatalf("missing root: %v", err)
	}
}

func TestReadFileIfExists(t *testing.T) {
	dir := t.TempDir()
	if data, ok, err := ReadFileIfExists(filepath.Join(dir, "nope")); data != nil || ok || err != nil {
		t.Fatalf("missing = %q, %v, %v", data, ok, err)
	}
	p := filepath.Join(dir, "f")
	if err := os.WriteFile(p, []byte("hi"), 0o600); err != nil {
		t.Fatal(err)
	}
	if data, ok, err := ReadFileIfExists(p); string(data) != "hi" || !ok || err != nil {
		t.Fatalf("present = %q, %v, %v", data, ok, err)
	}
	if _, ok, err := ReadFileIfExists(dir); ok || err == nil {
		t.Fatalf("directory = %v, %v; want an error", ok, err)
	}
}
