//go:build !windows

package safefs

func transientRename(error) bool { return false }
