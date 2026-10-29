//go:build darwin

package safefs

import "errors"

// Trash is unsupported on darwin: the freedesktop trash would be a folder Finder never shows.
func Trash(string) error { return errors.ErrUnsupported }
