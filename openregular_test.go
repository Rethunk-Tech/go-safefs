package safefs

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestOpenRegularRefusesADirectory(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if _, err := OpenRegular(dir); !errors.Is(err, ErrNotRegular) {
		t.Fatalf("directory: err = %v", err)
	}
	file := filepath.Join(dir, "a")
	if err := os.WriteFile(file, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	f, err := OpenRegular(file)
	if err != nil {
		t.Fatal(err)
	}
	_ = f.Close()
}

func TestIsDirFollowsASymlinkOutOfItsFolder(t *testing.T) {
	t.Parallel()
	target := t.TempDir()
	link := filepath.Join(t.TempDir(), "lib")
	if err := os.Symlink(target, link); err != nil {
		t.Skip(err)
	}
	removeBeforeTempCleanup(t, link)
	if !IsDir(link) {
		t.Fatal("symlinked folder not seen as a directory")
	}
}

// A path with a trailing separator names the same file as without one, as it does for os.
func TestRootedCallsAcceptATrailingSeparator(t *testing.T) {
	t.Parallel()
	dir := filepath.Join(t.TempDir(), "a")
	if err := os.Mkdir(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	info, err := Stat(dir + string(filepath.Separator))
	if err != nil || !info.IsDir() {
		t.Fatalf("Stat(dir+sep) = %v, %v", info, err)
	}
}

func TestRootedCallsRefuseASymlinkedFinalElementLeadingOut(t *testing.T) {
	t.Parallel()
	skipWine(t, "Wine host symlinks are not Windows reparse points")
	outside := writeFile(t, t.TempDir(), "secret", "no")
	dir := t.TempDir()
	link := filepath.Join(dir, "link")
	if err := os.Symlink(outside, link); err != nil {
		t.Skip(err)
	}
	if b, err := ReadFile(link); err == nil {
		t.Fatalf("ReadFile followed the link out: %q", b)
	}
	if f, err := Open(link); err == nil {
		_ = f.Close()
		t.Fatal("Open followed the link out")
	}
	if _, err := Within(link, func(r *os.Root, name string) (fs.FileInfo, error) { return r.Stat(name) }); err == nil {
		t.Fatal("Within followed the link out")
	}
}

func TestWithinRootRefusesALinkedFolderOutTheRoot(t *testing.T) {
	skipWine(t, "Wine host symlinks are not Windows reparse points")
	root, evil := t.TempDir(), t.TempDir()
	if err := os.Symlink(evil, filepath.Join(root, "Foo")); err != nil {
		t.Skip(err)
	}
	write := func(r *os.Root, name string) (struct{}, error) {
		return struct{}{}, r.WriteFile(name, []byte("x"), 0o600)
	}
	target := filepath.Join(root, "Foo", "x.dll")
	if _, err := WithinRoot(root, target, write); err == nil {
		t.Fatal("WithinRoot wrote through a link that leaves the root")
	}
	if _, err := os.Stat(filepath.Join(evil, "x.dll")); err == nil {
		t.Fatal("the file landed outside the root")
	}
	if _, err := Within(target, write); err != nil {
		t.Fatalf("Within no longer follows the parent folder as documented: %v", err)
	}
	for _, p := range []string{root, filepath.Join(root, "..", "x"), filepath.Join(evil, "y")} {
		if _, err := WithinRoot(root, p, write); !errors.Is(err, ErrOutsideRoot) {
			t.Errorf("WithinRoot(%q) = %v, want ErrOutsideRoot", p, err)
		}
	}
	if err := os.MkdirAll(filepath.Join(root, "real", "sub"), 0o750); err != nil {
		t.Fatal(err)
	}
	if _, err := WithinRoot(root, filepath.Join(root, "real", "sub", "f"), write); err != nil {
		t.Fatalf("a path inside the root was refused: %v", err)
	}
}

func FuzzWithinRoot(f *testing.F) {
	for _, rel := range []string{"a", "a/b", "..", "../x", ".", "", "a/../b", "/abs", `a\b`, "real/sub/f"} {
		f.Add(rel)
	}
	f.Fuzz(func(t *testing.T, rel string) {
		if strings.IndexByte(rel, 0) >= 0 {
			return
		}
		root := t.TempDir()
		for _, path := range []string{filepath.Join(root, rel), rel} {
			_, err := WithinRoot(root, path, func(r *os.Root, name string) (struct{}, error) {
				if name != filepath.Base(name) || name == "." || name == ".." {
					t.Fatalf("callback name %q is not a single element", name)
				}
				return struct{}{}, r.WriteFile(name, []byte("x"), 0o600)
			})
			wantRel, rerr := filepath.Rel(filepath.Clean(root), filepath.Clean(path))
			inside := rerr == nil && wantRel != "." && filepath.IsLocal(wantRel)
			if errors.Is(err, ErrOutsideRoot) {
				if inside {
					t.Fatalf("WithinRoot(%q) refused a local path", path)
				}
				continue
			}
			if err != nil {
				continue
			}
			if !inside {
				t.Fatalf("WithinRoot(%q) accepted a path not below %q", path, root)
			}
			written := filepath.Join(root, wantRel)
			if !UnderRoot(root, written) {
				t.Fatalf("WithinRoot wrote %q outside %q", written, root)
			}
		}
	})
}
