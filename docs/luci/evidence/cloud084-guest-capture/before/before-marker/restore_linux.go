//go:build linux

package microvmguest

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"syscall"

	"github.com/lcfrai/luci/control-plane/internal/backend/e2bbe"
)

// CollectTask is the standalone root collector entry. Its only input is the
// exact prior authenticated launch identity. No task-provided path, command,
// success assertion or mutable ignore policy can choose its source of truth.
func CollectTask(ctx context.Context, expected e2bbe.CaptureIdentity) (e2bbe.CaptureReceipt, error) {
	if os.Geteuid() != 0 {
		return e2bbe.CaptureReceipt{}, ErrRefused
	}
	p, err := restoreCaptureInput(ctx, "/luci-work/attempt", expected, 0)
	if err != nil {
		return e2bbe.CaptureReceipt{}, err
	}
	frozen, err := FreezeTaskGroup(ctx, expected)
	if err != nil {
		return e2bbe.CaptureReceipt{}, err
	}
	return CaptureTaskOutput(ctx, p, frozen)
}

// privateHierarchy rejects even an intermediate link or a writable ancestor.
// uid is zero in production; tests use only their own dedicated private tree.
func privateHierarchy(directory string, uid uint32) (os.FileInfo, error) {
	if !filepath.IsAbs(directory) || filepath.Clean(directory) != directory {
		return nil, ErrRefused
	}
	var leaf os.FileInfo
	for current := directory; ; current = filepath.Dir(current) {
		info, err := os.Lstat(current)
		if err != nil {
			return nil, err
		}
		st, ok := info.Sys().(*syscall.Stat_t)
		if !ok || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 || info.Mode().Perm()&0022 != 0 || (st.Uid != uid && st.Uid != 0) {
			return nil, ErrRefused
		}
		if current == directory {
			if st.Uid != uid {
				return nil, ErrRefused
			}
			leaf = info
		}
		if filepath.Dir(current) == current {
			break
		}
	}
	return leaf, nil
}
func privateInput(root *os.Root, name string, uid uint32, limit int64) (*os.File, error) {
	info, err := root.Lstat(name)
	if err != nil {
		return nil, err
	}
	st, ok := info.Sys().(*syscall.Stat_t)
	if !ok || !info.Mode().IsRegular() || st.Uid != uid || st.Nlink != 1 || info.Mode().Perm()&0077 != 0 || info.Size() < 1 || info.Size() > limit {
		return nil, ErrRefused
	}
	f, err := root.Open(name)
	if err != nil {
		return nil, err
	}
	actual, err := f.Stat()
	if err != nil || !os.SameFile(info, actual) || actual.Size() != info.Size() {
		f.Close()
		return nil, ErrRefused
	}
	return f, nil
}
func privateJSON(ctx context.Context, root *os.Root, name string, uid uint32, limit int64, out any) error {
	f, err := privateInput(root, name, uid, limit)
	if err != nil {
		return err
	}
	defer f.Close()
	b, err := io.ReadAll(io.LimitReader(checkedReader{ctx, f}, limit+1))
	if err != nil {
		return err
	}
	if int64(len(b)) > limit {
		return ErrRefused
	}
	d := json.NewDecoder(bytes.NewReader(b))
	d.DisallowUnknownFields()
	if err = d.Decode(out); err != nil {
		return err
	}
	if d.Decode(new(any)) != io.EOF {
		return ErrRefused
	}
	return ctx.Err()
}
func restoreCaptureInput(ctx context.Context, directory string, expected e2bbe.CaptureIdentity, uid uint32) (Prepared, error) {
	var empty Prepared
	if err := ctx.Err(); err != nil {
		return empty, err
	}
	if e2bbe.ValidateCaptureIdentityShape(expected) != nil {
		return empty, ErrRefused
	}
	info, err := privateHierarchy(directory, uid)
	if err != nil {
		return empty, err
	}
	root, err := os.OpenRoot(directory)
	if err != nil {
		return empty, err
	}
	defer root.Close()
	var identity e2bbe.CaptureIdentity
	if err = privateJSON(ctx, root, kernelLaunchReceipt, uid, 4096, &identity); err != nil {
		return empty, err
	}
	if identity != expected {
		return empty, ErrRefused
	}
	archive, err := privateInput(root, "input.tar", uid, MaxArchiveBytes)
	if err != nil {
		return empty, err
	}
	defer archive.Close()
	binding := e2bbe.TransferBinding{Identity: expected.Identity, SourceDigest: expected.SourceDigest, ContextDigest: expected.ContextDigest, Direction: "input"}
	manifest, err := e2bbe.VerifyTransfer(checkedReader{ctx, archive}, binding, expected.InputBundleDigest)
	if err != nil {
		return empty, err
	}
	p := Prepared{directory: directory, archiveSHA256: expected.InputBundleDigest, manifest: manifest, identity: info}
	var marker struct {
		ArchiveSHA256 string                 `json:"archiveSha256"`
		Manifest      e2bbe.TransferManifest `json:"manifest"`
	}
	for _, name := range []string{"prepared.json", "launch-reserved.json"} {
		if err = privateJSON(ctx, root, name, uid, 1<<20, &marker); err != nil {
			return empty, err
		}
		if marker.ArchiveSHA256 != p.archiveSHA256 || !reflect.DeepEqual(marker.Manifest, p.manifest) {
			return empty, ErrRefused
		}
	}
	if err = validatePrepared(p); err != nil {
		return empty, err
	}
	// Writable implementation bytes intentionally need not equal the baseline;
	// capture records every original path's changed/deleted state after freezing.
	return p, ctx.Err()
}
