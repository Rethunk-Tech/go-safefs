package safefs

import (
	"os"
	"path/filepath"
	"testing"
)

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
