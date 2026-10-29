package safefs

import "golang.org/x/sys/windows"

func linkCount(path string) (uint64, error) {
	p, err := windows.UTF16PtrFromString(ExtendedPath(path))
	if err != nil {
		return 0, err
	}
	h, err := windows.CreateFile(p, 0, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE, nil,
		windows.OPEN_EXISTING, windows.FILE_FLAG_OPEN_REPARSE_POINT|windows.FILE_FLAG_BACKUP_SEMANTICS, 0)
	if err != nil {
		return 0, err
	}
	defer func() { _ = windows.CloseHandle(h) }()
	var info windows.ByHandleFileInformation
	if err := windows.GetFileInformationByHandle(h, &info); err != nil {
		return 0, err
	}
	return uint64(info.NumberOfLinks), nil
}
