package safefs

import (
	"os"
	"path/filepath"

	"golang.org/x/sys/windows"
)

// EvalSymlinks is filepath.EvalSymlinks that also works on paths past MAX_PATH.
func EvalSymlinks(p string) (string, error) {
	abs, err := filepath.Abs(p)
	if err != nil {
		return filepath.EvalSymlinks(p)
	}
	extended := ExtendedPath(abs)
	resolved, err := filepath.EvalSymlinks(extended)
	if extended == abs {
		return resolved, err
	}
	return trimExtended(resolved), err
}

// heldUnder reports whether f holds a file under src, judged by the final path of f's own handle against that of src,
// so no later lookup of a path an attacker can swap decides it.
func heldUnder(src, _ string, f *os.File) bool {
	d, err := os.Open(src) //nolint:gosec // G703: the caller names the path; taking one is this package's contract
	if err != nil {
		return false
	}
	defer func() { _ = d.Close() }()
	root, err := finalPath(d)
	if err != nil {
		return false
	}
	held, err := finalPath(f)
	return err == nil && UnderRoot(root, held)
}

// finalPath is the normalized \\?\ path of the file f holds, every link and junction resolved.
func finalPath(f *os.File) (string, error) {
	var buf [windows.MAX_LONG_PATH]uint16
	// Flags 0 are FILE_NAME_NORMALIZED and VOLUME_NAME_DOS.
	n, err := windows.GetFinalPathNameByHandle(windows.Handle(f.Fd()), &buf[0], uint32(len(buf)), 0)
	if err != nil {
		return "", err
	}
	if n > uint32(len(buf)) {
		return "", windows.ERROR_INSUFFICIENT_BUFFER
	}
	return windows.UTF16ToString(buf[:n]), nil
}
