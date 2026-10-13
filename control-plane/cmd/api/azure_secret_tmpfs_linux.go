//go:build linux

package main

import "golang.org/x/sys/unix"

func azureSecretTmpfs(path string) bool {
	var stat unix.Statfs_t
	return unix.Statfs(path, &stat) == nil && stat.Type == unix.TMPFS_MAGIC
}
