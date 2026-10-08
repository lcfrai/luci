//go:build windows

package product

import (
	"errors"
	"os"
	"syscall"
)

func swarmWorkspaceSingleLink(file *os.File) error {
	var info syscall.ByHandleFileInformation
	if err := syscall.GetFileInformationByHandle(syscall.Handle(file.Fd()), &info); err != nil {
		return err
	}
	if info.NumberOfLinks != 1 {
		return errors.New("linked file is not exposed")
	}
	return nil
}
