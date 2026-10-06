//go:build windows

package api

import "golang.org/x/sys/windows"

func publishAssistanceReceipt(from, to string) error {
	source, err := windows.UTF16PtrFromString(sessionLongPath(from))
	if err != nil {
		return err
	}
	target, err := windows.UTF16PtrFromString(sessionLongPath(to))
	if err != nil {
		return err
	}
	// Windows has no supported directory fsync. Use the native write-through
	// replacement operation, following the import publication boundary.
	return windows.MoveFileEx(source, target, windows.MOVEFILE_REPLACE_EXISTING|windows.MOVEFILE_WRITE_THROUGH)
}
