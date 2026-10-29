//go:build !unix && !windows

package safefs

import "os"

// HardLinkID reports no sharing: a hard-linked file is copied once per name.
func HardLinkID(string, os.FileInfo) (FileID, bool) { return FileID{}, false }
