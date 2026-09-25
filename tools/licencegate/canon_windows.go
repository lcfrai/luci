//go:build windows

package main

import "syscall"

const caseInsensitivePaths = true

// longPath is canon's one Windows line: GetLongPathName expands 8.3 short
// elements (C:\Users\AU-DEV~1 -> C:\Users\au-dev-dmackie1) and keeps a
// junction as it is. It needs the path to exist.
func longPath(p string) (string, error) {
	in, err := syscall.UTF16PtrFromString(p)
	if err != nil {
		return "", err
	}
	buf := make([]uint16, 260)
	for {
		n, err := syscall.GetLongPathName(in, &buf[0], uint32(len(buf)))
		if err != nil {
			return "", err
		}
		if n <= uint32(len(buf)) {
			return syscall.UTF16ToString(buf[:n]), nil
		}
		buf = make([]uint16, n)
	}
}
