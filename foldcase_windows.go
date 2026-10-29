package safefs

import "strings"

// FoldCase maps path to the form two spellings of one location share: Windows paths ignore case, so D:\Library
// and d:\library are one folder.
func FoldCase(path string) string { return strings.ToLower(path) }
