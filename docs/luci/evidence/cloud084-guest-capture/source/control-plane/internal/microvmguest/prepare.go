//go:build linux

// Package microvmguest stages one exact reviewed task in a newly allocated VM.
// It does not plan work, implement an agent engine, grant execution authority,
// or treat returned claims as independently verified evidence.
package microvmguest

import (
	"archive/tar"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/lcfrai/luci/control-plane/internal/backend/e2bbe"
)

var ErrRefused = errors.New("microVM bootstrap requires a fresh owned directory and exact reviewed input")

// MaxArchiveBytes includes bounded manifest and tar overhead, not just payload.
const MaxArchiveBytes int64 = e2bbe.MaxTransferBytes + e2bbe.MaxTransferFiles*1024 + (1 << 20) + 2048

// Prepared can only be obtained from successful verification. Its ownership
// and receipt are rechecked before permission changes or launch reservation.
type Prepared struct {
	directory     string
	archiveSHA256 string
	manifest      e2bbe.TransferManifest
	identity      os.FileInfo
}

func (p Prepared) Directory() string { return p.directory }
func (p Prepared) receipt() any {
	return struct {
		ArchiveSHA256 string                 `json:"archiveSha256"`
		Manifest      e2bbe.TransferManifest `json:"manifest"`
	}{p.archiveSHA256, p.manifest}
}
func validatePrepared(p Prepared) error {
	if p.identity == nil || !filepath.IsAbs(p.directory) || filepath.Clean(p.directory) != p.directory || filepath.Dir(p.directory) == p.directory {
		return ErrRefused
	}
	info, e := os.Lstat(p.directory)
	if e != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 || !os.SameFile(info, p.identity) {
		return ErrRefused
	}
	path := filepath.Join(p.directory, "prepared.json")
	info, e = os.Lstat(path)
	if e != nil || !info.Mode().IsRegular() || info.Size() > 1<<20 {
		return ErrRefused
	}
	want, e := json.Marshal(p.receipt())
	if e != nil {
		return ErrRefused
	}
	got, e := os.ReadFile(path)
	if e != nil || !bytes.Equal(got, append(want, '\n')) {
		return ErrRefused
	}
	return nil
}

type checkedReader struct {
	ctx context.Context
	r   io.Reader
}

func (r checkedReader) Read(b []byte) (int, error) {
	if e := r.ctx.Err(); e != nil {
		return 0, e
	}
	return r.r.Read(b)
}

// A durable prepared receipt is not proof that every directory entry survived
// a crash or that writable source still matches it. Recheck the bounded exact
// manifest immediately before reserving execution, using a confined root and
// refusing symbolic path components. No source history or credentials are read.
func verifyPreparedFiles(ctx context.Context, p Prepared) error {
	root, err := os.OpenRoot(p.directory)
	if err != nil {
		return ErrRefused
	}
	defer root.Close()
	for _, expected := range p.manifest.Files {
		if err := ctx.Err(); err != nil {
			return err
		}
		parts := strings.Split(expected.Path, "/")
		for i := range parts {
			info, err := root.Lstat(filepath.FromSlash(strings.Join(parts[:i+1], "/")))
			if err != nil || info.Mode()&os.ModeSymlink != 0 || (i < len(parts)-1 && !info.IsDir()) || (i == len(parts)-1 && (!info.Mode().IsRegular() || info.Size() != expected.Size)) {
				return ErrRefused
			}
		}
		file, err := root.Open(filepath.FromSlash(expected.Path))
		if err != nil {
			return ErrRefused
		}
		hash := sha256.New()
		n, readErr := io.Copy(hash, io.LimitReader(checkedReader{ctx, file}, expected.Size+1))
		closeErr := file.Close()
		if readErr != nil {
			return readErr
		}
		if closeErr != nil || n != expected.Size || hex.EncodeToString(hash.Sum(nil)) != expected.SHA256 {
			return ErrRefused
		}
	}
	return ctx.Err()
}

func writeJSON(path string, value any) error {
	f, e := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if e != nil {
		return e
	}
	if e = json.NewEncoder(f).Encode(value); e == nil {
		e = f.Sync()
	}
	ce := f.Close()
	if e != nil {
		return e
	}
	if ce != nil {
		return ce
	}
	parent, e := os.Open(filepath.Dir(path))
	if e != nil {
		return e
	}
	defer parent.Close()
	return parent.Sync()
}

// Prepare reserves a fresh directory before reading input. Failure leaves that
// private reservation for diagnosis: a repeated invocation cannot overwrite it
// or turn an uncertain earlier start into a second execution. The caller owns
// the directory's parent and must not let an untrusted worker rename it.
// Cancellation is cooperative between input reads; network callers must also
// cancel their transport. Nothing is executed by this function.
func Prepare(ctx context.Context, directory string, input io.Reader, binding e2bbe.TransferBinding, digest string) (Prepared, error) {
	var out Prepared
	if ctx.Err() != nil {
		return out, ctx.Err()
	}
	if input == nil || binding.Direction != "input" || !filepath.IsAbs(directory) || filepath.Clean(directory) != directory || filepath.Dir(directory) == directory {
		return out, ErrRefused
	}
	if e := os.Mkdir(directory, 0700); e != nil {
		return out, ErrRefused
	}
	if e := writeJSON(filepath.Join(directory, "reservation.json"), struct {
		Binding e2bbe.TransferBinding `json:"binding"`
		Archive string                `json:"archiveSha256"`
	}{binding, digest}); e != nil {
		return out, e
	}
	archive, e := os.OpenFile(filepath.Join(directory, "input.tar"), os.O_RDWR|os.O_CREATE|os.O_EXCL, 0600)
	if e != nil {
		return out, e
	}
	defer archive.Close()
	n, e := io.Copy(archive, io.LimitReader(checkedReader{ctx, input}, MaxArchiveBytes+1))
	if e != nil {
		return out, e
	}
	if n > MaxArchiveBytes {
		return out, e2bbe.ErrTransfer
	}
	if e = archive.Sync(); e != nil {
		return out, e
	}
	if _, e = archive.Seek(0, io.SeekStart); e != nil {
		return out, e
	}
	manifest, e := e2bbe.VerifyTransfer(checkedReader{ctx, archive}, binding, digest)
	if e != nil {
		return out, e
	}
	source, contextFiles := false, false
	for _, f := range manifest.Files {
		if len(f.Path) > 7 && f.Path[:7] == "source/" {
			source = true
		}
		if len(f.Path) > 8 && f.Path[:8] == "context/" {
			contextFiles = true
		}
	}
	if !source || !contextFiles {
		return out, ErrRefused
	}
	if _, e = archive.Seek(0, io.SeekStart); e != nil {
		return out, e
	}
	tr := tar.NewReader(checkedReader{ctx, archive})
	if _, e = tr.Next(); e != nil {
		return out, e
	}
	for _, f := range manifest.Files {
		h, e := tr.Next()
		if e != nil || h.Name != f.Path || h.Typeflag != tar.TypeReg || h.Size != f.Size {
			return out, e2bbe.ErrTransfer
		}
		target := filepath.Join(directory, filepath.FromSlash(f.Path))
		if e = os.MkdirAll(filepath.Dir(target), 0700); e != nil {
			return out, e
		}
		dst, e := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		if e != nil {
			return out, e
		}
		count, copyErr := io.Copy(dst, tr)
		if copyErr == nil {
			copyErr = dst.Sync()
		}
		closeErr := dst.Close()
		if copyErr != nil {
			return out, copyErr
		}
		if closeErr != nil {
			return out, closeErr
		}
		if count != f.Size {
			return out, e2bbe.ErrTransfer
		}
	}
	// Verify again from the same private descriptor before exposing any prepared
	// result. The caller cannot supply a different path between verify/extract.
	if _, e = archive.Seek(0, io.SeekStart); e != nil {
		return out, e
	}
	if _, e = e2bbe.VerifyTransfer(checkedReader{ctx, archive}, binding, digest); e != nil {
		return out, e
	}
	info, e := os.Lstat(directory)
	if e != nil {
		return out, e
	}
	out = Prepared{directory: directory, archiveSHA256: digest, manifest: manifest, identity: info}
	if e = writeJSON(filepath.Join(directory, "prepared.json"), out.receipt()); e != nil {
		return Prepared{}, e
	}
	return out, nil
}

// ReserveLaunch is an irreversible local intent marker, not a running-process
// receipt. A crash after this write is unknown and must never replay the engine.
func ReserveLaunch(ctx context.Context, prepared Prepared) error {
	if e := ctx.Err(); e != nil {
		return e
	}
	if e := validatePrepared(prepared); e != nil {
		return e
	}
	if e := verifyPreparedFiles(ctx, prepared); e != nil {
		return e
	}
	return writeJSON(filepath.Join(prepared.directory, "launch-reserved.json"), prepared.receipt())
}
