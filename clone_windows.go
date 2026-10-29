//go:build windows

package safefs

import (
	"bytes"
	"encoding/binary"
	"errors"
	"os"

	"golang.org/x/sys/windows"
)

const fsctlDuplicateExtentsToFile = 0x98344

// Clone copies src to a new dst with FSCTL_DUPLICATE_EXTENTS_TO_FILE, sharing extents on a volume that supports it
// (ReFS). dst is created exclusively with mode 0o600 and removed again on failure.
func Clone(src, dst string) error {
	in, err := Open(src)
	if err != nil {
		return err
	}
	defer func() { _ = in.Close() }()
	st, err := in.Stat()
	if err != nil {
		return err
	}
	out, err := OpenFile(dst, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	if err := out.Truncate(st.Size()); err != nil {
		_ = out.Close()
		_ = Remove(dst)
		return err
	}
	if st.Size() == 0 {
		return out.Close()
	}
	var buf [32]byte
	var fd, sz bytes.Buffer
	_ = binary.Write(&fd, binary.LittleEndian, uint64(in.Fd()))
	_ = binary.Write(&sz, binary.LittleEndian, st.Size())
	copy(buf[0:8], fd.Bytes())
	copy(buf[24:32], sz.Bytes())
	var bytesReturned uint32
	err = windows.DeviceIoControl(
		windows.Handle(out.Fd()),
		fsctlDuplicateExtentsToFile,
		&buf[0],
		uint32(len(buf)),
		nil,
		0,
		&bytesReturned,
		nil,
	)
	if err != nil {
		_ = out.Close()
		_ = Remove(dst)
		return err
	}
	return out.Close()
}

// LinkUnsupported reports an error that means the target cannot clone or link, so the caller falls back to a copy.
func LinkUnsupported(err error) bool {
	return errors.Is(err, windows.ERROR_INVALID_FUNCTION) ||
		errors.Is(err, windows.ERROR_NOT_SUPPORTED) ||
		errors.Is(err, windows.ERROR_INVALID_PARAMETER) ||
		errors.Is(err, windows.ERROR_NOT_SAME_DEVICE) ||
		errors.Is(err, windows.ERROR_PRIVILEGE_NOT_HELD)
}

func isCrossDevice(err error) bool {
	return errors.Is(err, windows.ERROR_NOT_SAME_DEVICE)
}

// Device returns the serial number of the volume holding path.
func Device(path string) (uint64, error) {
	p, err := windows.UTF16PtrFromString(ExtendedPath(path))
	if err != nil {
		return 0, err
	}
	h, err := windows.CreateFile(p, windows.FILE_READ_ATTRIBUTES,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE, nil, windows.OPEN_EXISTING, windows.FILE_FLAG_BACKUP_SEMANTICS, 0)
	if err != nil {
		return 0, err
	}
	defer func() { _ = windows.CloseHandle(h) }()
	var info windows.ByHandleFileInformation
	if err := windows.GetFileInformationByHandle(h, &info); err != nil {
		return 0, err
	}
	return uint64(info.VolumeSerialNumber), nil
}
