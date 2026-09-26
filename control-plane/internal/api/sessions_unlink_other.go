//go:build !windows && !unix

package api

import (
	"errors"
	"os"
	"path/filepath"
)

// sessionRemoveIn has no way on this platform to delete in a directory without
// walking a path an agent can change after the caller's look, so it deletes
// nothing and says so. Forget then fails and keeps the record: never a
// deletion through a link (why: sessionForgetTranscripts).
func sessionRemoveIn(root, dir, name string) error {
	return &os.PathError{Op: "remove", Path: filepath.Join(root, dir, name), Err: errors.ErrUnsupported}
}
