package safefs

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"slices"
	"unsafe"

	"golang.org/x/sys/windows"
)

var shFileOperation = windows.NewLazySystemDLL("shell32.dll").NewProc("SHFileOperationW")

// shFileOpStruct is SHFILEOPSTRUCTW.
type shFileOpStruct struct {
	hwnd          uintptr
	wFunc         uint32
	from          *uint16
	to            *uint16
	flags         uint16
	aborted       int32
	nameMappings  uintptr
	progressTitle *uint16
}

const (
	foDelete          = 3
	fofSilent         = 0x4
	fofNoConfirmation = 0x10
	fofAllowUndo      = 0x40
	fofNoErrorUI      = 0x400
)

// Trash moves path to the Recycle Bin, where Explorer can restore it. An error wraps ErrInUse when another program
// holds the path or a file in it, and fs.ErrPermission when Windows denied access.
func Trash(path string) error {
	abs, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	from, err := windows.UTF16FromString(abs)
	if err != nil {
		return err
	}
	// The list of paths ends with a second NUL.
	from = append(from, 0)
	op := shFileOpStruct{wFunc: foDelete, from: &from[0], flags: fofSilent | fofNoConfirmation | fofAllowUndo | fofNoErrorUI}
	r, _, _ := shFileOperation.Call(uintptr(unsafe.Pointer(&op))) //nolint:gosec // G103: SHFileOperation takes a pointer to the op struct
	if r == 0 && op.aborted != 0 {
		r = deOpCancelled
	}
	if r != 0 {
		return fmt.Errorf("moving %s to the Recycle Bin failed: %w", abs, shFileOpError(r))
	}
	return nil
}

const deOpCancelled = 0x75

// shError is a failure SHFileOperation reported by one of its own codes.
type shError struct {
	code uintptr
	text string
	is   []error
}

func (e *shError) Error() string   { return fmt.Sprintf("%s (code 0x%X)", e.text, e.code) }
func (e *shError) Unwrap() []error { return e.is }

// shFileOpError reads SHFileOperation's result. Its own pre-Win32 DE_* codes overlap winerror.h values with other
// meanings, so they are read first; anything else is a Win32 error. The DE_* table and its meanings are Microsoft's:
// https://learn.microsoft.com/en-us/windows/win32/api/shellapi/nf-shellapi-shfileoperationw#return-value
func shFileOpError(code uintptr) error {
	inUse := []error{ErrInUse, fs.ErrPermission}
	de := map[uintptr]shError{
		0x71: {text: "the source and destination are the same file"},
		0x72: {text: "several sources were given but only one destination"},
		0x73: {text: "a rename named a destination in another folder"},
		0x74: {text: "it is the root of a drive, which cannot be moved"},
		0x75: {text: "the operation was cancelled"},
		0x76: {text: "the destination is inside the source"},
		// A folder below it watched for changes (an open directory handle) gives this code too.
		0x78: {text: "Windows denied access to it; a program may have it or a folder in it open", is: inUse},
		0x79: {text: "the path is longer than Windows allows"},
		0x7A: {text: "the operation named several destinations"},
		// A file in the folder held open without delete sharing gives this code too.
		0x7C:    {text: "Windows called the path invalid; a program may have a file in it open", is: []error{ErrInUse}},
		0x7D:    {text: "the source and destination are in the same folder"},
		0x7E:    {text: "the destination is an existing file"},
		0x80:    {text: "the destination is an existing folder"},
		0x81:    {text: "a file name is longer than Windows allows"},
		0x82:    {text: "the destination is a read-only CD", is: []error{fs.ErrPermission}},
		0x83:    {text: "the destination is a read-only DVD", is: []error{fs.ErrPermission}},
		0x84:    {text: "the destination is a writable CD that is not formatted"},
		0x85:    {text: "a file is too large for the destination"},
		0x86:    {text: "it is on a read-only CD", is: []error{fs.ErrPermission}},
		0x87:    {text: "it is on a read-only DVD", is: []error{fs.ErrPermission}},
		0x88:    {text: "it is on a writable CD"},
		0xB7:    {text: "a path grew longer than Windows allows during the operation"},
		0x402:   {text: "an unknown error, usually an invalid path"},
		0x10000: {text: "an unspecified error at the destination"},
		0x10074: {text: "the destination is the root of a drive"},
	}
	if e, ok := de[code]; ok {
		e.code = code
		return &e
	}
	errno := windows.Errno(code)
	// A program whose current folder is inside the folder gives ERROR_SHARING_VIOLATION; a rename of a folder with
	// open handles below it gives ERROR_ACCESS_DENIED, which errno already matches to fs.ErrPermission.
	if slices.Contains([]windows.Errno{windows.ERROR_SHARING_VIOLATION, windows.ERROR_LOCK_VIOLATION, windows.ERROR_ACCESS_DENIED}, errno) {
		return fmt.Errorf("%w: %w", ErrInUse, errno)
	}
	return errno
}
