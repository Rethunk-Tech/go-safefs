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

// flush is Flush over the batch's files.
func (b *syncBatch) flush(dst string) error { return Flush(dst, b.files, b.dirs) }

// Flush makes files written without a per-file fsync (CopyFileNoSync) durable together: one syncfs(2) on the
// filesystem holding dir where the OS has it, else each file and each folder in dirs once. Pass every folder the
// writes created or renamed into, since a file's rename lives in its folder.
func Flush(dir string, files, dirs []string) error {
	if err := syncFS(dir); err == nil {
		return nil
	}
	var errs []error
	for _, p := range files {
		errs = append(errs, syncFile(p))
	}
	for _, p := range dirs {
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
