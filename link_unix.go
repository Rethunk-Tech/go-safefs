//go:build !windows

package safefs

import "os"

// LinkDir makes link point at the folder target.
func LinkDir(link, target string) error { return os.Symlink(target, link) }
