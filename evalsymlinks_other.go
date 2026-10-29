//go:build !windows

package safefs

import "path/filepath"

// EvalSymlinks is filepath.EvalSymlinks; only Windows needs help with long paths.
func EvalSymlinks(p string) (string, error) { return filepath.EvalSymlinks(p) }
