//go:build windows

package management

import (
	"context"
	"errors"
	"runtime"

	"golang.org/x/sys/windows"
)

// A named operating-system mutex also serializes independent Luci instances.
// Windows mutex ownership belongs to the thread that acquired it.
func acquireDependencyHostInstallation(ctx context.Context) (func(), error) {
	runtime.LockOSThread()
	name, _ := windows.UTF16PtrFromString(`Global\Luci.DependencyInstaller.v1`)
	handle, err := windows.CreateMutex(nil, false, name)
	if err != nil && !errors.Is(err, windows.ERROR_ALREADY_EXISTS) {
		runtime.UnlockOSThread()
		return nil, err
	}
	for {
		if err = ctx.Err(); err != nil {
			windows.CloseHandle(handle)
			runtime.UnlockOSThread()
			return nil, err
		}
		status, waitErr := windows.WaitForSingleObject(handle, 100)
		if waitErr != nil || (status != windows.WAIT_OBJECT_0 && status != windows.WAIT_ABANDONED && status != uint32(windows.WAIT_TIMEOUT)) {
			windows.CloseHandle(handle)
			runtime.UnlockOSThread()
			return nil, errors.New("could not acquire the computer's installation lock")
		}
		if status == windows.WAIT_OBJECT_0 || status == windows.WAIT_ABANDONED {
			if err := ctx.Err(); err != nil {
				_ = windows.ReleaseMutex(handle)
				_ = windows.CloseHandle(handle)
				runtime.UnlockOSThread()
				return nil, err
			}
			return func() {
				_ = windows.ReleaseMutex(handle)
				_ = windows.CloseHandle(handle)
				runtime.UnlockOSThread()
			}, nil
		}
	}
}
