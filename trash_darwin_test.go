//go:build darwin

package safefs

import (
	"errors"
	"testing"
)

func TestTrashIsUnsupportedOnDarwin(t *testing.T) {
	if err := Trash(t.TempDir()); !errors.Is(err, errors.ErrUnsupported) {
		t.Fatalf("Trash = %v, want errors.ErrUnsupported", err)
	}
}
