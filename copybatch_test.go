package safefs

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func withSyncWrites(t testing.TB) {
	t.Helper()
	old := syncWrites
	syncWrites = true
	t.Cleanup(func() { syncWrites = old })
}

func TestCopyTreeCopiesEveryByteAndModeAndLeavesNoMarker(t *testing.T) {
	withSyncWrites(t)
	src, dst := t.TempDir(), filepath.Join(t.TempDir(), "out")
	want := map[string]string{"a.txt": "alpha", "d/b.bin": string(bytes.Repeat([]byte{7}, 1<<20)), "d/e/empty": ""}
	for rel, body := range want {
		writeFile(t, src, rel, body)
	}
	if runtime.GOOS != "windows" {
		if err := os.Chmod(filepath.Join(src, "a.txt"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := CopyTree(src, dst, CopyOptions{}); err != nil {
		t.Fatal(err)
	}
	for rel, body := range want {
		got, err := os.ReadFile(filepath.Join(dst, filepath.FromSlash(rel)))
		if err != nil || string(got) != body {
			t.Fatalf("%s: %d bytes, %v; want %d", rel, len(got), err, len(body))
		}
	}
	if runtime.GOOS != "windows" {
		if st, err := os.Stat(filepath.Join(dst, "a.txt")); err != nil || st.Mode().Perm() != 0o755 {
			t.Fatalf("mode = %v, %v", st.Mode(), err)
		}
	}
	if CopyInterrupted(dst) {
		t.Fatal("marker left after a finished copy")
	}
}

func TestCopyTreeInterruptedCopyIsDetectedAndRepaired(t *testing.T) {
	src, dst := t.TempDir(), filepath.Join(t.TempDir(), "out")
	for i := range 5 {
		writeFile(t, src, fmt.Sprintf("f%d", i), fmt.Sprintf("body%d", i))
	}
	errCrash := errors.New("power cut")
	n := 0
	crash := CopyOptions{Put: func(from, to, _ string) error {
		if n++; n == 3 {
			return errCrash
		}
		return CopyFile(from, to, CopyOptions{})
	}}
	if _, err := CopyTree(src, dst, crash); !errors.Is(err, errCrash) {
		t.Fatalf("err = %v", err)
	}
	if !CopyInterrupted(dst) {
		t.Fatal("interrupted copy not detected")
	}
	if _, err := CopyTree(src, dst, CopyOptions{}); err != nil {
		t.Fatal(err)
	}
	if CopyInterrupted(dst) {
		t.Fatal("marker survived the repair")
	}
	for i := range 5 {
		got, err := os.ReadFile(filepath.Join(dst, fmt.Sprintf("f%d", i)))
		if err != nil || string(got) != fmt.Sprintf("body%d", i) {
			t.Fatalf("f%d = %q, %v", i, got, err)
		}
	}
}

func TestCopyTreeFollowingLinksKeepsTheMarkerUntilDone(t *testing.T) {
	src, dst := t.TempDir(), filepath.Join(t.TempDir(), "out")
	writeFile(t, src, "x", "x")
	writeFile(t, dst, CopyMarker, "")
	if !CopyInterrupted(dst) {
		t.Fatal("marker not seen")
	}
	if _, err := CopyTreeFollowingLinks(src, dst, CopyOptions{}); err != nil {
		t.Fatal(err)
	}
	if CopyInterrupted(dst) {
		t.Fatal("marker survived")
	}
}

func TestSyncBatchFlushFallsBackToPerFileSync(t *testing.T) {
	dir := t.TempDir()
	p := writeFile(t, dir, "f", "x")
	b := syncBatch{files: []string{p}, dirs: []string{dir}}
	if err := b.flush(filepath.Join(dir, "missing")); err != nil {
		t.Fatal(err) // syncFS fails on a missing dir, so the per-file path ran
	}
}

// BenchmarkSmallFiles copies 2000 small files; SAFEFS_BENCH_DIR must name a folder on a real disk, since a tmpfs
// makes fsync free.
func BenchmarkSmallFiles(b *testing.B) {
	withSyncWrites(b)
	base := os.Getenv("SAFEFS_BENCH_DIR")
	if base == "" {
		base = b.TempDir()
	}
	src := filepath.Join(base, "src")
	for i := range 2000 {
		writeFile(b, src, fmt.Sprintf("d%d/f%d", i%20, i), "some small mod file body")
	}
	b.Cleanup(func() { _ = RemoveAll(src) })
	run := func(name string, copyAll func(dst string) error) {
		b.Run(name, func(b *testing.B) {
			for b.Loop() {
				dst := filepath.Join(base, "dst")
				if err := copyAll(dst); err != nil {
					b.Fatal(err)
				}
				_ = RemoveAll(dst)
			}
		})
	}
	run("batched", func(dst string) error { _, err := CopyTree(src, dst, CopyOptions{}); return err })
	run("perFile", func(dst string) error {
		_, err := CopyTree(src, dst, CopyOptions{Put: func(from, to, rel string) error {
			if err := os.MkdirAll(filepath.Dir(to), 0o750); err != nil {
				return err
			}
			return CopyFile(from, to, CopyOptions{})
		}})
		return err
	})
}
