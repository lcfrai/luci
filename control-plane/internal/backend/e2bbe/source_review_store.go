package e2bbe

import (
	"archive/tar"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
)

// ReadStoredTaskReport returns the optional untrusted upstream handoff only
// after both complete source archives have passed exact binding validation.
// Missing means no report was captured, never that checks passed or questions
// were answered. The returned bytes still require product schema/review.
func (s *DirectoryStore) ReadStoredTaskReport(ctx context.Context, req Request, receipt CaptureReceipt) ([]byte, error) {
	if _, err := s.ValidateStoredReturnedSource(ctx, req, receipt); err != nil {
		return nil, err
	}
	if receipt.ReportSHA256 == "" {
		return nil, nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil, ErrClosed
	}
	f, err := s.root.Open("artifact-" + receipt.ArchiveSHA256 + ".tar")
	if err != nil {
		return nil, err
	}
	defer f.Close()
	reader := tar.NewReader(contextReader{ctx: ctx, r: f})
	for {
		header, err := reader.Next()
		if err == io.EOF {
			return nil, ErrTransfer
		}
		if err != nil {
			return nil, err
		}
		if header.Name != "output/handoff.json" {
			continue
		}
		if header.Size < 1 || header.Size > 64<<10 || header.Typeflag != tar.TypeReg {
			return nil, ErrTransfer
		}
		body, err := io.ReadAll(io.LimitReader(reader, 64<<10+1))
		if err != nil || int64(len(body)) != header.Size {
			return nil, ErrTransfer
		}
		hash := sha256.Sum256(body)
		if hex.EncodeToString(hash[:]) != receipt.ReportSHA256 {
			return nil, ErrTransfer
		}
		return body, nil
	}
}

// ValidateStoredReturnedSource reads both complete, hash-bound archives from
// controller-owned storage and verifies the returned source inventory/diff.
// The caller must separately establish a current actor, exact capture/native
// exit and the reviewed source/context task reservation. No files are applied.
func (s *DirectoryStore) ValidateStoredReturnedSource(ctx context.Context, req Request, receipt CaptureReceipt) (SourceChanges, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return SourceChanges{}, ErrClosed
	}
	if req.Identity.OwnerID != s.config.OwnerID || !validRequest(req) || !digestPattern.MatchString(receipt.ArchiveSHA256) ||
		receipt.ArchiveBytes <= 0 || receipt.ArchiveBytes > MaxCaptureArchiveBytes {
		return SourceChanges{}, ErrConflict
	}
	inputBinding := TransferBinding{Identity: req.Identity, SourceDigest: req.SourceDigest, ContextDigest: req.ContextDigest, Direction: "input"}
	outputBinding := inputBinding
	outputBinding.Direction = "output"
	input, err := s.verifyTransfer(ctx, "artifact-"+req.BundleDigest+".tar", inputBinding, req.BundleDigest)
	if err != nil {
		return SourceChanges{}, err
	}
	outputName := "artifact-" + receipt.ArchiveSHA256 + ".tar"
	output, err := s.verifyTransfer(ctx, outputName, outputBinding, receipt.ArchiveSHA256)
	if err != nil {
		return SourceChanges{}, err
	}
	info, err := s.root.Lstat(outputName)
	if err != nil || !info.Mode().IsRegular() || info.Size() != receipt.ArchiveBytes {
		return SourceChanges{}, ErrTransfer
	}
	f, err := s.root.Open(outputName)
	if err != nil {
		return SourceChanges{}, err
	}
	defer f.Close()
	reader := tar.NewReader(contextReader{ctx: ctx, r: f})
	if _, err = reader.Next(); err != nil { // manifest.json, already verified above
		return SourceChanges{}, ErrTransfer
	}
	var sourceBytes, changeBytes []byte
	for _, file := range output.Files {
		header, e := reader.Next()
		if e != nil || header.Name != file.Path {
			return SourceChanges{}, ErrTransfer
		}
		if file.Path == "output/source-manifest.json" || file.Path == "output/changes.json" {
			if file.Size > maxManifestBytes {
				return SourceChanges{}, ErrTransfer
			}
			body, e := io.ReadAll(io.LimitReader(reader, maxManifestBytes+1))
			if e != nil || int64(len(body)) != file.Size {
				return SourceChanges{}, ErrTransfer
			}
			if file.Path == "output/source-manifest.json" {
				sourceBytes = body
			} else {
				changeBytes = body
			}
		} else if _, e := io.Copy(io.Discard, reader); e != nil {
			return SourceChanges{}, e
		}
	}
	if _, err = reader.Next(); !errors.Is(err, io.EOF) {
		return SourceChanges{}, ErrTransfer
	}
	return ValidateReturnedSource(req, input, output, receipt, sourceBytes, changeBytes)
}
