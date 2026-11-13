//go:build !windows && !darwin

package safefs

import (
	"os"
	"path/filepath"
	"testing"
)

// A file in Trash/files whose .trashinfo is gone is still someone's data; renaming onto it would overwrite it.
func TestTrashNeverOverwritesAnOrphanedTrashedFile(t *testing.T) {
	data := t.TempDir()
	t.Setenv("XDG_DATA_HOME", data)
	files := filepath.Join(data, "Trash", "files")
	if err := os.MkdirAll(files, 0o700); err != nil {
		t.Fatal(err)
	}
	orphan := filepath.Join(files, "note.txt")
	if err := os.WriteFile(orphan, []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}
	src := filepath.Join(data, "note.txt")
	if err := os.WriteFile(src, []byte("new"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := Trash(src); err != nil {
		t.Fatal(err)
	}
	if b, _ := ReadFile(orphan); string(b) != "old" {
		t.Fatalf("orphan overwritten: %q", b)
	}
	if b, _ := ReadFile(orphan + ".2"); string(b) != "new" {
		t.Fatalf("trashed copy = %q", b)
	}
}
