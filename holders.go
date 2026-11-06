package safefs

import "errors"

// ErrInUse is wrapped by an error from a move or delete that failed because another program has the path, or a file
// in it, open.
var ErrInUse = errors.New("in use by another program")

// Holder is a running program that has a file open.
type Holder struct {
	PID int
	// Name is the program's name as the system reports it, such as "Visual Studio Code"; its executable's name when it
	// has none.
	Name string
}
