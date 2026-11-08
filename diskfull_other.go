//go:build !unix && !windows

package safefs

func platformDiskFull(error) bool { return false }
