package safefs

import (
	"crypto/rand"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"
)

// The temp file of every write here is "<base>.<random>.tmp" beside path. Callers sweep leftovers by that shape, so
// it is part of the contract.

// WriteFileAtomic replaces path with data through a temp file and rename, so a crash never leaves it truncated. As with
// os.WriteFile, the umask narrows perm.
func WriteFileAtomic(path string, data []byte, perm os.FileMode) error {
	return WriteStreamAtomic(path, perm, func(w io.Writer) error {
		_, err := w.Write(data)
		return err
	})
}

// WriteStreamAtomic replaces path with write's output through a temp file, fsync, and rename, so a crash never leaves it
// truncated. write seeing a failure, or returning one, removes the temp file and leaves path unchanged.
func WriteStreamAtomic(path string, perm os.FileMode, write func(io.Writer) error) error {
	return writeStream(path, perm, true, write)
}

// WriteStreamNoSync is WriteStreamAtomic without the fsyncs, for a file that only memoises a computation: a crash may
// leave it torn, so the reader must treat an unreadable file as absent. Thousands of small fsyncs cost seconds.
func WriteStreamNoSync(path string, perm os.FileMode, write func(io.Writer) error) error {
	return writeStream(path, perm, false, write)
}

// syncWrites is off under go test: a test cannot observe an fsync, and the thousands its fixtures write cost the
// suite seconds on a real disk.
var syncWrites = !testing.Testing()

func writeStream(path string, perm os.FileMode, durable bool, write func(io.Writer) error) (err error) {
	durable = durable && syncWrites
	f, err := createTemp(path, perm)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			err = errors.Join(err, Remove(f.Name()))
		}
	}()
	if err = write(f); err != nil {
		return errors.Join(err, f.Close())
	}
	if durable {
		if err = f.Sync(); err != nil {
			return errors.Join(err, f.Close())
		}
	}
	if err = f.Close(); err != nil {
		return err
	}
	if err = Rename(f.Name(), path); err != nil {
		return err
	}
	if durable {
		_ = SyncDir(filepath.Dir(path)) // best effort: the rename already landed
	}
	return nil
}

// createTemp opens a new "<base>.<random>.tmp" beside path with perm passed to the create itself, so the umask
// narrows it exactly as os.WriteFile would; os.CreateTemp plus Chmod would set the bits verbatim. The name carries
// 130 random bits, so O_EXCL failing means something is wrong rather than a collision to retry.
func createTemp(path string, perm os.FileMode) (*os.File, error) {
	return os.OpenFile(path+"."+rand.Text()+".tmp", os.O_RDWR|os.O_CREATE|os.O_EXCL, perm)
}
