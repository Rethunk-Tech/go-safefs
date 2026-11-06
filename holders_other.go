//go:build !windows

package safefs

import "errors"

// Holders is Windows only: elsewhere an open file does not stop a move or delete.
func Holders([]string) ([]Holder, error) { return nil, errors.ErrUnsupported }
