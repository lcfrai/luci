//go:build !windows

package api

import "os"

func codexPrivateFile(path string) error { return os.Chmod(path, 0600) }
func codexPrivateDir(path string) error  { return os.Chmod(path, 0700) }
