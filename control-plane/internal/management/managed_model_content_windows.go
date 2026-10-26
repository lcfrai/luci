//go:build windows

package management

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"github.com/lcfrai/luci/control-plane/internal/hostpath"
	"golang.org/x/sys/windows"
	"io"
	"os"
	"path/filepath"
	"strings"
)

type managedModelDescriptor struct {
	Digest string `json:"digest"`
	Size   int64  `json:"size"`
}
type managedModelManifest struct {
	Version int                      `json:"schemaVersion"`
	Config  managedModelDescriptor   `json:"config"`
	Layers  []managedModelDescriptor `json:"layers"`
}

func managedModelManifestCandidates(root, model string) ([]string, error) {
	name, tag, has := strings.Cut(model, ":")
	if !has {
		tag = "latest"
	}
	if name == "" || tag == "" || name == "." || name == ".." || tag == "." || tag == ".." || strings.ContainsAny(name+tag, "\\/\x00\r\n ") || strings.Contains(tag, ":") {
		return nil, errors.New("installed library model name invalid")
	}
	return []string{filepath.Join(root, "manifests-v2", "ollama.com", "library", name, tag), filepath.Join(root, "manifests", "registry.ollama.ai", "library", name, tag)}, nil
}
func pinManagedContentFile(ctx context.Context, path string, max int64) (*managedModelPinnedFile, []byte, error) {
	if e := ctx.Err(); e != nil {
		return nil, nil, e
	}
	if e := managedModelOrdinaryPath(path, false); e != nil {
		return nil, nil, e
	}
	if e := hostpath.OneName(path); e != nil {
		return nil, nil, e
	}
	p, e := windows.UTF16PtrFromString(path)
	if e != nil {
		return nil, nil, e
	}
	h, e := windows.CreateFile(p, windows.GENERIC_READ, windows.FILE_SHARE_READ, nil, windows.OPEN_EXISTING, windows.FILE_ATTRIBUTE_NORMAL, 0)
	if e != nil {
		return nil, nil, e
	}
	pin := &managedModelPinnedFile{handle: h, path: path}
	// The outer owner retains pin on EVERY post-open error and closes once.
	var duplicate windows.Handle
	if e = windows.DuplicateHandle(windows.CurrentProcess(), h, windows.CurrentProcess(), &duplicate, 0, false, windows.DUPLICATE_SAME_ACCESS); e != nil {
		return pin, nil, e
	}
	f := os.NewFile(uintptr(duplicate), path)
	defer func() {
		if f.Close() != nil {
			pin.cleanupUnknown = true
		}
	}()
	info, e := f.Stat()
	if e != nil || !info.Mode().IsRegular() || info.Size() < 1 || info.Size() > max {
		return pin, nil, errors.New("installed model content size invalid")
	}
	pin.size = info.Size()
	sum := sha256.New()
	buf := make([]byte, 1<<20)
	var small []byte
	var total int64
	for {
		if e = ctx.Err(); e != nil {
			return pin, nil, e
		}
		n, readErr := f.Read(buf)
		if n > 0 {
			total += int64(n)
			if total > max {
				return pin, nil, errors.New("installed model content exceeded bound")
			}
			sum.Write(buf[:n])
			if max <= 256<<10 {
				small = append(small, buf[:n]...)
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return pin, nil, readErr
		}
	}
	if total != pin.size {
		return pin, nil, errors.New("installed model content changed")
	}
	pin.digest = hex.EncodeToString(sum.Sum(nil))
	return pin, small, nil
}
func pinManagedModelContent(ctx context.Context, root, model string) (pins []*managedModelPinnedFile, digest string, result error) {
	paths, e := managedModelManifestCandidates(root, model)
	if e != nil {
		return nil, "", e
	}
	selected := ""
	for _, p := range paths {
		_, e = os.Lstat(p)
		if e == nil {
			if selected != "" {
				return nil, "", errors.New("ambiguous installed model manifests")
			}
			selected = p
		} else if !errors.Is(e, os.ErrNotExist) {
			return nil, "", e
		}
	}
	if selected == "" {
		return nil, "", errors.New("selected installed model manifest missing")
	}
	pin, raw, e := pinManagedContentFile(ctx, selected, 256<<10)
	if pin != nil {
		pins = append(pins, pin)
	}
	if e != nil {
		return pins, "", e
	}
	if pin.cleanupUnknown {
		return pins, "", errors.New("original model reader closure unknown")
	}
	digest = pin.digest
	var manifest managedModelManifest
	if json.Unmarshal(raw, &manifest) != nil || manifest.Version != 2 || len(manifest.Layers) < 1 || len(manifest.Layers) > 16 {
		return pins, "", errors.New("installed model manifest invalid")
	}
	descriptors := append([]managedModelDescriptor{manifest.Config}, manifest.Layers...)
	seen := map[string]bool{}
	var bytes int64
	for _, d := range descriptors {
		hash, ok := strings.CutPrefix(d.Digest, "sha256:")
		if !ok || len(hash) != 64 || strings.Trim(hash, "0123456789abcdef") != "" || d.Size < 1 || d.Size > 6<<30 {
			return pins, "", errors.New("installed model descriptor invalid")
		}
		if seen[hash] {
			continue
		}
		seen[hash] = true
		bytes += d.Size
		if bytes > 6<<30 {
			return pins, "", errors.New("installed model exceeds supported content bound")
		}
		p, _, e := pinManagedContentFile(ctx, filepath.Join(root, "blobs", "sha256-"+hash), d.Size)
		if p != nil {
			pins = append(pins, p)
		}
		if e != nil {
			return pins, "", e
		}
		if p.cleanupUnknown || p.size != d.Size || p.digest != hash {
			return pins, "", errors.New("installed model content digest or reader closure differs")
		}
	}
	return pins, digest, nil
}
