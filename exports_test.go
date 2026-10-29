package safefs

import (
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// The exported wrappers a consumer calls, each through its public entry point; the tier and copy logic under them
// is covered by the tests of the unexported functions.

func TestMaterializerTreeAndFileCopyTheContent(t *testing.T) {
	src, dst := t.TempDir(), t.TempDir()
	seedTree(t, src)
	if err := (Materializer{}).Tree(src, dst); err != nil {
		t.Fatal(err)
	}
	want, got := readAll(t, src), readAll(t, dst)
	if len(got) != len(want) {
		t.Fatalf("Tree wrote %d files, want %d", len(got), len(want))
	}
	for k, v := range want {
		if got[k] != v {
			t.Fatalf("Tree %s = %q, want %q", k, got[k], v)
		}
	}

	one := filepath.Join(t.TempDir(), "readme.txt")
	if err := (Materializer{Tiers: []Tier{TierCopy}}).File(filepath.Join(src, "A", "readme.txt"), one, "A/readme.txt"); err != nil {
		t.Fatal(err)
	}
	if b, err := ReadFile(one); err != nil || string(b) != "readme" {
		t.Fatalf("File = %q, %v", b, err)
	}
}

func TestRootedCreateOpenFileAndChmod(t *testing.T) {
	path := filepath.Join(t.TempDir(), "f")
	f, err := Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString("one"); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	f, err = OpenFile(path, os.O_WRONLY|os.O_APPEND, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString("two"); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	if b, err := ReadFile(path); err != nil || string(b) != "onetwo" {
		t.Fatalf("content = %q, %v", b, err)
	}
	if err := Chmod(path, 0o400); err != nil {
		t.Fatal(err)
	}
	// Windows has only the read-only bit, so the exact mode is checked off Windows.
	if info, err := os.Stat(path); err != nil || runtime.GOOS != "windows" && info.Mode().Perm() != 0o400 {
		t.Fatalf("mode = %v, %v", info.Mode(), err)
	}
}

func TestRemoveAllRemovesATree(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "tree")
	seedTree(t, dir)
	if err := RemoveAll(dir); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(dir); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("tree still there: %v", err)
	}
}

func TestWriteStreamNoSyncReplacesTheFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "memo")
	for _, body := range []string{"first", "second"} {
		err := WriteStreamNoSync(path, 0o600, func(w io.Writer) error {
			_, err := io.WriteString(w, body)
			return err
		})
		if err != nil {
			t.Fatal(err)
		}
		if b, err := ReadFile(path); err != nil || string(b) != body {
			t.Fatalf("content = %q, %v; want %q", b, err, body)
		}
	}
}

func TestSizeEachNameCountsEveryHardLink(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a"), make([]byte, 10), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Link(filepath.Join(dir, "a"), filepath.Join(dir, "b")); err != nil {
		t.Skipf("no hard links here: %v", err)
	}
	if got, err := SizeEachName(dir); err != nil || got != 20 {
		t.Fatalf("SizeEachName = %d, %v; want 20", got, err)
	}
	if got, err := Size(dir); err != nil || got != 10 {
		t.Fatalf("Size = %d, %v; want 10", got, err)
	}
}

func TestCheckWritable(t *testing.T) {
	dir := t.TempDir()
	if err := CheckWritable(dir); err != nil {
		t.Fatalf("writable dir: %v", err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 0 {
		t.Fatalf("the probe was left behind: %v %v", entries, err)
	}
	if err := CheckWritable(filepath.Join(dir, "missing")); err != nil {
		t.Fatalf("a missing dir is the caller's to create, got %v", err)
	}
}

func TestFreeBytesReportsSpace(t *testing.T) {
	if n, err := FreeBytes(t.TempDir()); err != nil || n <= 0 {
		t.Fatalf("FreeBytes = %d, %v", n, err)
	}
}

func TestLinkDirReachesTheTarget(t *testing.T) {
	target := t.TempDir()
	if err := os.WriteFile(filepath.Join(target, "x"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(t.TempDir(), "link")
	if err := LinkDir(link, target); err != nil {
		t.Fatal(err)
	}
	if b, err := ReadFile(filepath.Join(link, "x")); err != nil || string(b) != "x" {
		t.Fatalf("through the link: %q, %v", b, err)
	}
}
