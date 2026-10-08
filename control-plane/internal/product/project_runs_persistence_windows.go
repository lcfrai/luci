package product

import "golang.org/x/sys/windows"

// Directory fsync is unavailable on Windows. Use the native write-through
// replacement boundary with extended paths, as retained import receipts do.
func publishRunReceipt(from, to string) error {
	source, err := projectImportWindowsPath(from)
	if err != nil {
		return err
	}
	target, err := projectImportWindowsPath(to)
	if err != nil {
		return err
	}
	return windows.MoveFileEx(source, target, windows.MOVEFILE_REPLACE_EXISTING|windows.MOVEFILE_WRITE_THROUGH)
}
