//go:build !windows

package safefs

// FoldCase returns path unchanged: Linux and macOS installs are compared as spelled.
func FoldCase(path string) string { return path }
