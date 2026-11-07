package safefs

import (
	"errors"
	"os"
	"time"
)

// ErrMountInside is RemoveAll's refusal of a tree with another filesystem mounted below it.
var ErrMountInside = errors.New("a mount point lies inside the folder")

const renameRetryWindow = 2 * time.Second

// Rename is os.Rename that rides out the short Windows window in which an antivirus scanner or the search indexer
// holds a freshly written file open. Elsewhere it is os.Rename.
func Rename(from, to string) error {
	return retry(func() error { return os.Rename(from, to) }, transientRename, renameRetryWindow) //nolint:gosec // G703: the caller names the path; taking one is this package's contract
}

// retry runs op until it succeeds, fails with a non-transient error, or window has passed, backing off between tries.
func retry(op func() error, transient func(error) bool, window time.Duration) error {
	start := time.Now()
	for delay := time.Millisecond; ; delay *= 2 {
		err := op()
		if err == nil || !transient(err) || time.Since(start)+delay > window {
			return err
		}
		time.Sleep(delay)
	}
}

// RemoveAll is os.RemoveAll with the same antivirus/indexer retry as Rename; a repeat pass removes whatever the
// first one could not.
//
// On Unix it refuses with ErrMountInside, before deleting anything, a tree that has a mount point (a bind mount, a
// FUSE or portal mount) below path: those expose files that are not the tree's own.
func RemoveAll(path string) error {
	if err := checkNoMountsInside(path); err != nil {
		return err
	}
	return retry(func() error { return os.RemoveAll(path) }, transientRename, renameRetryWindow) //nolint:gosec // G703: the caller names the path; taking one is this package's contract
}

// Remove is os.Remove with the same antivirus/indexer retry as Rename.
func Remove(path string) error {
	return retry(func() error { return os.Remove(path) }, transientRename, renameRetryWindow) //nolint:gosec // G703: the caller names the path; taking one is this package's contract
}
