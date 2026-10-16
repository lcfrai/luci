//go:build windows

package api

import "golang.org/x/sys/windows"

func publishAssemblyCatalogObject(from, to string) error {
	source, err := windows.UTF16PtrFromString(sessionLongPath(from))
	if err != nil {
		return err
	}
	target, err := windows.UTF16PtrFromString(sessionLongPath(to))
	if err != nil {
		return err
	}
	// Deliberately no REPLACE_EXISTING. Another immutable object is never changed.
	return windows.MoveFileEx(source, target, windows.MOVEFILE_WRITE_THROUGH)
}
