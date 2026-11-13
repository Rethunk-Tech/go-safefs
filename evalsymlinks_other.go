//go:build !windows

package safefs

import (
	"os"
	"path/filepath"
)

// EvalSymlinks is filepath.EvalSymlinks; only Windows needs help with long paths.
func EvalSymlinks(p string) (string, error) { return filepath.EvalSymlinks(p) }

// heldUnder reports whether f, opened by the path p, holds a file under src. p must resolve under src to the very
// file f holds, so a link swapped between the open and the resolve cannot vouch for a file outside.
func heldUnder(src, p string, f *os.File) bool {
	root, err := EvalSymlinks(src)
	if err != nil {
		return false
	}
	resolved, err := EvalSymlinks(p)
	if err != nil || !UnderRoot(root, resolved) {
		return false
	}
	held, err := f.Stat()
	if err != nil {
		return false
	}
	at, err := os.Stat(resolved) //nolint:gosec // G703: the caller names the path; taking one is this package's contract
	return err == nil && os.SameFile(held, at)
}
