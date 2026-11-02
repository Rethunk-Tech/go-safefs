package safefs

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestCopyTreeCopiesSymlinkFilesThatStayInside(t *testing.T) {
	src := t.TempDir()
	dst := t.TempDir()
	writeFile(t, src, "real.png", "png")
	if err := os.Mkdir(filepath.Join(src, "assets"), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(src, "real.png"), filepath.Join(src, "assets", "foo.png")); err != nil {
		t.Fatal(err)
	}
	if _, err := CopyTree(src, dst, CopyOptions{}); err != nil {
		t.Fatal(err)
	}
	got, err := ReadFile(filepath.Join(dst, "assets", "foo.png"))
	if err != nil || string(got) != "png" {
		t.Fatalf("copied symlink = %q, %v", got, err)
	}
}

func TestCopyTreeRejectsASymlinkFileThatEscapes(t *testing.T) {
	src := t.TempDir()
	outside := filepath.Join(t.TempDir(), "secret")
	if err := WriteFile(outside, []byte("no"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(src, "link")); err != nil {
		t.Fatal(err)
	}
	_, err := CopyTree(src, t.TempDir(), CopyOptions{})
	if err == nil || !strings.Contains(err.Error(), "escapes") {
		t.Fatalf("escape = %v", err)
	}
}

func TestCopyTreeRejectsARelativeSymlinkThatEscapes(t *testing.T) {
	parent := t.TempDir()
	src := filepath.Join(parent, "src")
	if err := os.Mkdir(src, 0o750); err != nil {
		t.Fatal(err)
	}
	writeFile(t, parent, "secret", "no")
	if err := os.Symlink(filepath.Join("..", "secret"), filepath.Join(src, "link")); err != nil {
		t.Fatal(err)
	}
	if _, err := CopyTree(src, t.TempDir(), CopyOptions{}); err == nil || !strings.Contains(err.Error(), "escapes") {
		t.Fatalf("escape = %v", err)
	}
}

func TestCopyTreeSkipsASymlinkDirectory(t *testing.T) {
	src := t.TempDir()
	outside := t.TempDir()
	writeFile(t, outside, "secret.txt", "no")
	writeFile(t, src, "ok.txt", "yes")
	if err := os.Symlink(outside, filepath.Join(src, "Innocent")); err != nil {
		t.Fatal(err)
	}
	dst := t.TempDir()
	if _, err := CopyTree(src, dst, CopyOptions{}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(filepath.Join(dst, "Innocent")); !os.IsNotExist(err) {
		t.Fatalf("symlink dir was copied: %v", err)
	}
	got, err := ReadFile(filepath.Join(dst, "ok.txt"))
	if err != nil || string(got) != "yes" {
		t.Fatalf("regular file = %q, %v", got, err)
	}
}

func TestUnderRoot(t *testing.T) {
	if !UnderRoot("/", "/etc") {
		t.Fatal("path under filesystem root")
	}
	if !UnderRoot("/data", "/data") {
		t.Fatal("path equal to root")
	}
	if UnderRoot("/data", "/data-other") {
		t.Fatal("sibling prefix")
	}
	if UnderRoot("/data", "/data/../etc") || !UnderRoot("/data", "/data/a/../b") {
		t.Error("UnderRoot must judge the cleaned path")
	}
	if UnderRoot("/data", "/etc") {
		t.Fatal("path outside root")
	}
}

func TestCopyTreeReportsSkippedLinkedFolders(t *testing.T) {
	src, outside := t.TempDir(), t.TempDir()
	if err := os.Symlink(outside, filepath.Join(src, "link")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	skipped, err := CopyTree(src, t.TempDir(), CopyOptions{})
	if err != nil || len(skipped) != 1 {
		t.Fatalf("skipped = %v, err = %v", skipped, err)
	}
}

func TestCopyFileReplacesInPlaceAndLeavesNoTemp(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "src.txt", "new")
	writeFile(t, dir, "dst.txt", "old")
	link := filepath.Join(dir, "linked.txt")
	if err := os.Link(filepath.Join(dir, "dst.txt"), link); err != nil {
		t.Skip("no hard links here")
	}
	if err := CopyFile(filepath.Join(dir, "src.txt"), filepath.Join(dir, "dst.txt"), CopyOptions{}); err != nil {
		t.Fatal(err)
	}
	if b, _ := ReadFile(filepath.Join(dir, "dst.txt")); string(b) != "new" {
		t.Fatalf("dst = %q", b)
	}
	if b, _ := ReadFile(link); string(b) != "old" {
		t.Fatalf("the copy wrote through a link: %q", b)
	}
	if _, err := os.Lstat(filepath.Join(dir, "dst.txt.safefs-tmp")); err == nil {
		t.Fatal("the temp file stays")
	}
	if err := CopyFile(filepath.Join(dir, "missing.txt"), filepath.Join(dir, "dst.txt"), CopyOptions{}); err == nil {
		t.Fatal("a missing source copied")
	}
	if b, _ := ReadFile(filepath.Join(dir, "dst.txt")); string(b) != "new" {
		t.Fatalf("a failed copy changed dst: %q", b)
	}
}

func TestCopyFileReplacesAStaleTempOfItsSuffix(t *testing.T) {
	dir := t.TempDir()
	src := writeFile(t, dir, "src.txt", "new")
	dst := filepath.Join(dir, "dst.txt")
	stale := writeFile(t, dir, "dst.txt.part", "leftover from a crash")
	if err := CopyFile(src, dst, CopyOptions{TempSuffix: ".part"}); err != nil {
		t.Fatal(err)
	}
	if b, _ := ReadFile(dst); string(b) != "new" {
		t.Fatalf("dst = %q", b)
	}
	if _, err := os.Lstat(stale); !os.IsNotExist(err) {
		t.Fatalf("the temp stays: %v", err)
	}
}

func TestCopyTreeFollowingLinksWritesTheTargetAndSkips(t *testing.T) {
	src, outside := t.TempDir(), t.TempDir()
	target := writeFile(t, outside, "real.bin", "bytes")
	writeFile(t, src, "plain.txt", "plain")
	writeFile(t, src, "__marker", "bookkeeping")
	if err := os.Symlink(target, filepath.Join(src, "linked.bin")); err != nil {
		t.Skip(err)
	}
	dst := t.TempDir()
	opts := CopyOptions{Skip: func(name string) bool { return name == "__marker" }}
	if err := CopyTreeFollowingLinks(src, dst, opts); err != nil {
		t.Fatal(err)
	}
	if b, _ := ReadFile(filepath.Join(dst, "linked.bin")); string(b) != "bytes" {
		t.Fatalf("linked.bin = %q", b)
	}
	if IsFile(filepath.Join(dst, "__marker")) || !IsFile(filepath.Join(dst, "plain.txt")) {
		t.Fatal("Skip did not leave out exactly the marker")
	}
	if fi, err := os.Lstat(filepath.Join(dst, "linked.bin")); err != nil || fi.Mode()&os.ModeSymlink != 0 {
		t.Fatalf("the link was copied as a link: %v", err)
	}
}

func TestCopyTreeFollowingLinksReportsADanglingLink(t *testing.T) {
	src := t.TempDir()
	if err := os.Symlink(filepath.Join(src, "gone"), filepath.Join(src, "dangling.bin")); err != nil {
		t.Skip(err)
	}
	err := CopyTreeFollowingLinks(src, t.TempDir(), CopyOptions{})
	var dangling *DanglingLinkError
	if !errors.As(err, &dangling) || dangling.Rel != "dangling.bin" || !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("err = %v", err)
	}
}

func TestCopyTreeProgressAndFirstEntry(t *testing.T) {
	src := t.TempDir()
	writeFile(t, src, "a/one", "1")
	writeFile(t, src, "b/two", "22")
	var order []string
	var last CopyProgress
	opts := CopyOptions{
		First:    "b",
		Progress: func(p CopyProgress) { last = p },
		Put: func(from, to, rel string) error {
			order = append(order, rel)
			return CopyFile(from, to, CopyOptions{})
		},
	}
	if _, err := CopyTree(src, t.TempDir(), opts); err != nil {
		t.Fatal(err)
	}
	if want := []string{filepath.Join("b", "two"), filepath.Join("a", "one")}; !slices.Equal(order, want) {
		t.Fatalf("order = %v, want %v", order, want)
	}
	if last.Files != 2 || last.TotalFiles != 2 || last.Bytes != 3 || last.TotalBytes != 3 {
		t.Fatalf("progress = %+v", last)
	}
}

func TestDanglingLinkErrorNamesTheLink(t *testing.T) {
	err := &DanglingLinkError{Rel: "a/link", Err: os.ErrNotExist}
	if !strings.Contains(err.Error(), "a/link") || !errors.Is(err, os.ErrNotExist) {
		t.Errorf("%v", err)
	}
}
