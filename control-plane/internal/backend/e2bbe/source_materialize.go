package e2bbe

import (
	"archive/tar"
	"context"
	"errors"
	"io"
	"os"
	"path"
	"runtime"
	"strings"
)

// MaterializeStoredReturnedSource writes a fully verified source snapshot only
// into a caller-owned empty private staging directory. It never writes a Git
// checkout or treats the guest's output as integrated source. The caller owns
// staging cleanup and must recheck task/actor authority before Git import.
func (s *DirectoryStore) MaterializeStoredReturnedSource(ctx context.Context, req Request, receipt CaptureReceipt, staging string) (SourceChanges, error) {
	changes, err := s.ValidateStoredReturnedSource(ctx, req, receipt)
	if err != nil {
		return SourceChanges{}, err
	}
	info, err := os.Lstat(staging)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 || runtime.GOOS != "windows" && info.Mode().Perm()&0077 != 0 {
		return SourceChanges{}, ErrTransfer
	}
	root, err := os.OpenRoot(staging)
	if err != nil {
		return SourceChanges{}, err
	}
	defer root.Close()
	dir, err := root.Open(".")
	if err != nil {
		return SourceChanges{}, err
	}
	entries, readErr := dir.ReadDir(1)
	_ = dir.Close()
	if readErr != nil && !errors.Is(readErr, io.EOF) || len(entries) != 0 {
		return SourceChanges{}, ErrTransfer
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return SourceChanges{}, ErrClosed
	}
	binding := TransferBinding{Identity: req.Identity, SourceDigest: req.SourceDigest, ContextDigest: req.ContextDigest, Direction: "output"}
	name := "artifact-" + receipt.ArchiveSHA256 + ".tar"
	manifest, err := s.verifyTransfer(ctx, name, binding, receipt.ArchiveSHA256)
	if err != nil {
		return SourceChanges{}, err
	}
	f, err := s.root.Open(name)
	if err != nil {
		return SourceChanges{}, err
	}
	defer f.Close()
	reader := tar.NewReader(contextReader{ctx: ctx, r: f})
	if _, err = reader.Next(); err != nil { // verified manifest.json
		return SourceChanges{}, ErrTransfer
	}
	for _, file := range manifest.Files {
		header, nextErr := reader.Next()
		if nextErr != nil || header.Name != file.Path {
			return SourceChanges{}, ErrTransfer
		}
		if !strings.HasPrefix(file.Path, "output/source/") {
			continue
		}
		rel := strings.TrimPrefix(file.Path, "output/source/")
		if !validTransferPath("source/"+rel, "input") {
			return SourceChanges{}, ErrTransfer
		}
		parent := path.Dir(rel)
		if parent != "." {
			if err = root.MkdirAll(parent, 0700); err != nil {
				return SourceChanges{}, err
			}
		}
		out, openErr := root.OpenFile(rel, os.O_WRONLY|os.O_CREATE|os.O_EXCL, os.FileMode(file.Mode))
		if openErr != nil {
			return SourceChanges{}, openErr
		}
		n, copyErr := io.CopyN(out, reader, file.Size)
		if copyErr == nil {
			copyErr = out.Chmod(os.FileMode(file.Mode))
		}
		if copyErr == nil {
			copyErr = out.Sync()
		}
		closeErr := out.Close()
		if copyErr != nil || closeErr != nil || n != file.Size {
			return SourceChanges{}, ErrTransfer
		}
	}
	if _, err = reader.Next(); !errors.Is(err, io.EOF) {
		return SourceChanges{}, ErrTransfer
	}
	return changes, nil
}
