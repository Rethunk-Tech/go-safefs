//go:build darwin

package safefs

import (
	"testing"

	"golang.org/x/sys/unix"
)

func TestLinkUnsupportedOnCloneRefusal(t *testing.T) {
	for _, e := range []error{unix.ENOTSUP, unix.EXDEV} {
		if !LinkUnsupported(e) {
			t.Fatalf("%v must fall through to a copy", e)
		}
	}
}
