//go:build windows

package product

import (
	"fmt"
	"golang.org/x/sys/windows"
	"os"
	"unsafe"
)

func projectOverviewFileIdentity(file *os.File) (string, error) {
	var info windows.ByHandleFileInformation
	handle := windows.Handle(file.Fd())
	if err := windows.GetFileInformationByHandle(handle, &info); err != nil {
		return "", err
	}
	if info.NumberOfLinks != 1 || info.FileAttributes&(windows.FILE_ATTRIBUTE_REPARSE_POINT|windows.FILE_ATTRIBUTE_DIRECTORY) != 0 {
		return "", errOverviewQuestions
	}
	var identity struct {
		Volume uint64
		ID     [16]byte
	}
	if err := windows.GetFileInformationByHandleEx(handle, windows.FileIdInfo, (*byte)(unsafe.Pointer(&identity)), uint32(unsafe.Sizeof(identity))); err != nil {
		return "", err
	}
	if identity.ID == [16]byte{} {
		return "", errOverviewQuestions
	}
	return fmt.Sprintf("windows:%x:%x", identity.Volume, identity.ID), nil
}
