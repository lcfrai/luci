//go:build !windows

package resourcebudget

import (
	"context"
	"errors"
	"os"
)

func probeLocalStorage(context.Context, string) (LocalStorageObservation, error) {
	return LocalStorageObservation{}, errors.New("Windows Docker Desktop storage confirmation is not required on this platform")
}
func localStorageRecordPrivate(f *os.File, info os.FileInfo) bool {
	return info.Mode().IsRegular() && info.Mode().Perm()&0077 == 0 && localStorageSingleLink(info)
}
