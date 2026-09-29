//go:build windows

package hostpath

import "syscall"

// linkCount is the file's NumberOfLinks as GetFileInformationByHandle reports
// it, from a handle opened the way os.SameFile opens its own: no data access,
// following a link to the file it names, FILE_FLAG_BACKUP_SEMANTICS so that a
// directory opens too.
func linkCount(p string) (uint64, error) {
	name, err := syscall.UTF16PtrFromString(p)
	if err != nil {
		return 0, err
	}
	share := uint32(syscall.FILE_SHARE_READ | syscall.FILE_SHARE_WRITE | syscall.FILE_SHARE_DELETE)
	h, err := syscall.CreateFile(name, 0, share, nil, syscall.OPEN_EXISTING, syscall.FILE_FLAG_BACKUP_SEMANTICS, 0)
	if err != nil {
		return 0, err
	}
	defer syscall.CloseHandle(h)
	var info syscall.ByHandleFileInformation
	if err := syscall.GetFileInformationByHandle(h, &info); err != nil {
		return 0, err
	}
	return uint64(info.NumberOfLinks), nil
}
