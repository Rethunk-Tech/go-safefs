package safefs

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func underWine() bool {
	return runtime.GOOS == "windows" && os.Getenv("WINEPREFIX") != ""
}

// skipWine skips tests that need a Windows kernel service or reparse-point semantics Wine does not provide.
func skipWine(t *testing.T, reason string) {
	t.Helper()
	if underWine() {
		t.Skip(reason)
	}
}

// removeBeforeTempCleanup deletes paths before t.TempDir's RemoveAll, so a junction or symlink cannot make that cleanup fail.
func removeBeforeTempCleanup(t *testing.T, paths ...string) {
	t.Helper()
	t.Cleanup(func() {
		for _, p := range paths {
			_ = os.Remove(p)
		}
	})
}

// writeFile writes body at the slash-separated rel under root, creating folders, and returns the full path.
func writeFile(t testing.TB, root, rel, body string) string {
	t.Helper()
	p := filepath.Join(root, filepath.FromSlash(rel))
	writeAt(t, p, body)
	return p
}

// writeAt writes body at path, creating its folder.
func writeAt(t testing.TB, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}
