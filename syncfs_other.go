//go:build !linux

package safefs

import "errors"

func syncFS(string) error { return errors.ErrUnsupported }
