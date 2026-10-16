package api

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sync"

	"github.com/lcfrai/luci/control-plane/internal/backend"
	"github.com/lcfrai/luci/control-plane/internal/hostpath"
)

func assemblyIdentityDigest(identity backend.DocumentImageAssemblyIdentity) string {
	data, _ := json.Marshal(identity)
	digest := sha256.Sum256(data)
	return hex.EncodeToString(digest[:])
}

func assemblyOutcomeDigest(out documentImageAssemblyOutcome) string {
	out.ResourcesHeld = false
	data, err := json.Marshal(out)
	if err != nil {
		return ""
	}
	digest := sha256.Sum256(data)
	return hex.EncodeToString(digest[:])
}

// Private artifact/receipt files only; allocation authority and capacity stay in
// the existing shared execution ledger. No second ownership/revision store.
type assemblyPrivateCapture struct {
	g                  *executionAdmission
	identity           backend.DocumentImageAssemblyIdentity
	dir, path, receipt string
}
type assemblyPrivateReceipt struct {
	Outcome             documentImageAssemblyOutcome `json:"outcome"`
	CaptureExitVerified bool                         `json:"captureExitVerified"`
	// A past verified exit can survive unavailable current artifact bytes.
	RetainedCaptureExitVerified bool `json:"retainedCaptureExitVerified"`
}

func (g *executionAdmission) assemblyPrivateCapture(identity backend.DocumentImageAssemblyIdentity) (*assemblyPrivateCapture, error) {
	if !backend.ValidateDocumentImageAssemblyIdentity(identity, true) || identity.ScopeDigest != g.scope {
		return nil, backend.ErrLaunchFenceOwnership
	}
	root, err := g.prepareExecutionDirectory()
	if err != nil {
		return nil, err
	}
	parent := filepath.Join(root, "document-image-assembly")
	if err = os.Mkdir(parent, 0700); err != nil && !errors.Is(err, os.ErrExist) {
		return nil, err
	}
	if err = codexRegularPath(root, parent); err != nil {
		return nil, err
	}
	if err = codexPrivateDir(parent); err != nil {
		return nil, err
	}
	dir := filepath.Join(parent, assemblyIdentityDigest(identity))
	if err = os.Mkdir(dir, 0700); err != nil && !errors.Is(err, os.ErrExist) {
		return nil, err
	}
	if err = codexRegularPath(root, dir); err != nil {
		return nil, err
	}
	if _, err = hostpath.ResolveOutside(g.api.projectsDir(), dir); err != nil {
		return nil, err
	}
	if err = codexPrivateDir(filepath.Dir(dir)); err != nil {
		return nil, err
	}
	if err = codexPrivateDir(dir); err != nil {
		return nil, err
	}
	return &assemblyPrivateCapture{g: g, identity: identity, dir: dir, path: filepath.Join(dir, "capture.oci.partial"), receipt: filepath.Join(dir, "outcome.json")}, nil
}

type assemblyBoundedFile struct {
	mu       sync.Mutex
	file     *os.File
	written  int64
	closed   bool
	closeErr error
}

func (w *assemblyBoundedFile) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.closed || int64(len(p)) > backend.DocumentImageAssemblyOutputMaxBytes-w.written {
		return 0, errors.New("assembly capture closed or byte bound exceeded")
	}
	n, err := w.file.Write(p)
	w.written += int64(n)
	return n, err
}
func (w *assemblyBoundedFile) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.closed {
		return w.closeErr
	}
	w.closed = true
	w.closeErr = w.file.Sync()
	closeErr := w.file.Close()
	if w.closeErr == nil {
		w.closeErr = closeErr
	}
	return w.closeErr
}

func (s *assemblyPrivateCapture) open() (io.WriteCloser, error) {
	if err := codexRegularPath(filepath.Dir(s.dir), s.dir); err != nil {
		return nil, err
	}
	file, err := os.OpenFile(s.path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return nil, err
	}
	valid := false
	defer func() {
		if !valid {
			_ = file.Close()
		}
	}()
	opened, err := file.Stat()
	if err != nil {
		return nil, err
	}
	current, err := os.Lstat(s.path)
	if err != nil || !current.Mode().IsRegular() || !os.SameFile(current, opened) {
		return nil, backend.ErrLaunchFenceOwnership
	}
	if err = hostpath.OneName(s.path); err != nil {
		return nil, err
	}
	if err = codexPrivateFile(s.path); err != nil {
		return nil, err
	}
	valid = true
	return &assemblyBoundedFile{file: file}, nil
}

func (s *assemblyPrivateCapture) verify(ctx context.Context, capture backend.DocumentImageAssemblyCapture) error {
	if !checkHex(capture.Digest, 32) || capture.Bytes < 1 || capture.Bytes > backend.DocumentImageAssemblyOutputMaxBytes {
		return backend.ErrLaunchFenceOwnership
	}
	if err := codexRegularPath(s.dir, s.path); err != nil {
		return err
	}
	if err := hostpath.OneName(s.path); err != nil {
		return err
	}
	file, err := os.Open(s.path)
	if err != nil {
		return err
	}
	defer file.Close()
	opened, err := file.Stat()
	if err != nil || !opened.Mode().IsRegular() || opened.Size() != capture.Bytes {
		return backend.ErrLaunchFenceOwnership
	}
	hash := sha256.New()
	buf := make([]byte, 64<<10)
	var size int64
	for {
		if err = ctx.Err(); err != nil {
			return err
		}
		n, readErr := file.Read(buf)
		if n > 0 {
			size += int64(n)
			if size > capture.Bytes {
				return backend.ErrLaunchFenceOwnership
			}
			_, _ = hash.Write(buf[:n])
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return readErr
		}
	}
	current, err := os.Lstat(s.path)
	if err != nil || !os.SameFile(opened, current) || size != capture.Bytes || hex.EncodeToString(hash.Sum(nil)) != capture.Digest {
		return backend.ErrLaunchFenceOwnership
	}
	return ctx.Err()
}

func (s *assemblyPrivateCapture) publish(ctx context.Context, out documentImageAssemblyOutcome, exitVerified, retainedExitVerified bool) error {
	if out.Identity != s.identity {
		return backend.ErrLaunchFenceOwnership
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if out.Capture != (backend.DocumentImageAssemblyCapture{}) {
		if out.ExitCode == nil || *out.ExitCode != 0 || out.OOMKilled || !out.ReaderSettled {
			return backend.ErrLaunchFenceOwnership
		}
		if err := s.verify(ctx, out.Capture); err != nil {
			return err
		}
	}
	if out.Capture != (backend.DocumentImageAssemblyCapture{}) && !exitVerified {
		return backend.ErrLaunchFenceOwnership
	}
	if out.RetainedCapture != (backend.DocumentImageAssemblyCapture{}) {
		if out.Stage != "recovery" || out.Capture != (backend.DocumentImageAssemblyCapture{}) || !checkHex(out.RetainedCapture.Digest, 32) || out.RetainedCapture.Bytes < 1 || out.RetainedCapture.Bytes > backend.DocumentImageAssemblyOutputMaxBytes {
			return backend.ErrLaunchFenceOwnership
		}
	}
	data, err := json.Marshal(assemblyPrivateReceipt{Outcome: out, CaptureExitVerified: exitVerified, RetainedCaptureExitVerified: retainedExitVerified})
	if err != nil {
		return err
	}
	if len(data) > 16<<10 {
		return errors.New("assembly outcome byte bound")
	}
	if _, err = os.Lstat(s.receipt); err == nil {
		if err = codexRegularPath(s.dir, s.receipt); err != nil {
			return err
		}
		if err = hostpath.OneName(s.receipt); err != nil {
			return err
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	// Reuse the existing fsync + durable atomic private publication primitive.
	return writeExecutionLedger(s.receipt, data)
}

func (s *assemblyPrivateCapture) read() (assemblyPrivateReceipt, error) {
	var out assemblyPrivateReceipt
	data, err := codexRead(s.dir, s.receipt, 16<<10)
	if err != nil {
		return out, err
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err = decoder.Decode(&out); err != nil {
		return out, err
	}
	if decoder.Decode(new(any)) != io.EOF || out.Outcome.Identity != s.identity {
		return out, backend.ErrLaunchFenceOwnership
	}
	return out, nil
}
