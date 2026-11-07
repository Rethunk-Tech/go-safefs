package safefs

import (
	"path/filepath"
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
