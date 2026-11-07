//go:build unix && !linux

package safefs

import "errors"

// mountsBelow has no mount table to read here; the caller falls back to comparing device ids.
func mountsBelow(string) ([]string, error) { return nil, errors.ErrUnsupported }
