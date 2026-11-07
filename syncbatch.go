package safefs

import (
	"errors"
	"os"
	"path/filepath"
)

// CopyMarker is the name of the file CopyTree and CopyTreeFollowingLinks keep in dst while a copy is unfinished.
// It is created and flushed before the first file is written and removed only after the whole copy is durable, so a
// crash or an error leaves it behind. Callers that sweep dst by name must leave it alone.
const CopyMarker = ".safefs-copy-incomplete"

// CopyInterrupted reports whether dst holds an unfinished tree copy: its files may be missing, short or hold
// unflushed bytes. Running the same copy again replaces every file and clears the marker.
func CopyInterrupted(dst string) bool {
	return pathExists(filepath.Join(dst, CopyMarker))
}

// syncBatch defers a tree copy's durability to one flush at the end. Files are renamed into place unsynced, so
// until flush returns, a crash may leave a renamed file short or empty; the marker is what says so.
type syncBatch struct {
	files []string
	dirs  []string
}

func (b *syncBatch) file(p string) { b.files = append(b.files, p) }
func (b *syncBatch) dir(p string)  { b.dirs = append(b.dirs, p) }

// begin writes (dst must exist) and flushes the marker, then flushes dst's parent so the marker's folder itself survives a crash.
func (b *syncBatch) begin(dst string) error {
	if err := WriteFileAtomic(filepath.Join(dst, CopyMarker), nil, 0o600); err != nil {
		return err
	}
	if syncWrites {
		return SyncDir(filepath.Dir(dst))
	}
	return nil
}

// finish makes every written file durable, then removes the marker and flushes its removal.
func (b *syncBatch) finish(dst string) error {
	if syncWrites {
		if err := b.flush(dst); err != nil {
			return err
		}
	}
	if err := Remove(filepath.Join(dst, CopyMarker)); err != nil {
		return err
	}
	if syncWrites {
		return SyncDir(dst)
	}
	return nil
}

// flush syncs the destination filesystem once where the OS can, else each written file and touched folder once.
func (b *syncBatch) flush(dst string) error {
	if err := syncFS(dst); err == nil {
		return nil
	}
	var errs []error
	for _, p := range b.files {
		errs = append(errs, syncFile(p))
	}
	for _, p := range b.dirs {
		errs = append(errs, SyncDir(p))
	}
	return errors.Join(errs...)
}

// syncFile flushes one finished file. os.File.Sync is FlushFileBuffers on Windows and F_FULLFSYNC on macOS, neither
// of which has a whole-filesystem form, so those platforms always take the per-file path of flush, once per file.
func syncFile(p string) error {
	f, err := OpenFile(p, os.O_RDWR, 0)
	if err != nil {
		return err
	}
	return errors.Join(f.Sync(), f.Close())
}
