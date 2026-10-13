//go:build linux

package microvmguest

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/backend/e2bbe"
)

const capturePolicy = e2bbe.SourceSnapshotPolicy
const maxCaptureEntries = 16384
const maxCaptureDirectories = 1024

type sourceSnapshot = e2bbe.SourceSnapshot
type sourceChange = e2bbe.SourceChange
type captureExclusion = e2bbe.SourceExclusion
type changeSnapshot = e2bbe.SourceChanges

func hashBytes(b []byte) string { sum := sha256.Sum256(b); return hex.EncodeToString(sum[:]) }

// Fixed controller-reviewed exclusions apply only to NEW paths. A worker's
// changed .gitignore cannot hide any originally transferred file or deletion.
func captureExcluded(name string, dir bool) bool {
	parts := strings.Split(name, "/")
	for index, part := range parts {
		if part == ".git" {
			return true
		}
		if dir || index < len(parts)-1 {
			switch part {
			case "node_modules", ".venv", "venv", "__pycache__", ".next", ".nuxt", ".cache", ".pytest_cache", "coverage":
				return true
			}
		}
	}
	base := path.Base(name)
	return !dir && (base == ".env" || strings.HasPrefix(base, ".env.") && base != ".env.example" && base != ".env.sample" || base == "id_rsa" || base == "id_ed25519")
}
func sourceFile(root *os.Root, name string, expected os.FileInfo, uid uint32) (*os.File, error) {
	if !expected.Mode().IsRegular() {
		return nil, ErrRefused
	}
	st, ok := expected.Sys().(*syscall.Stat_t)
	if !ok || st.Nlink != 1 || st.Uid != uid {
		return nil, ErrRefused
	}
	f, e := root.Open(name)
	if e != nil {
		return nil, e
	}
	info, e := f.Stat()
	if e != nil || !os.SameFile(expected, info) || info.Size() != expected.Size() || !info.Mode().IsRegular() {
		f.Close()
		return nil, ErrRefused
	}
	return f, nil
}

// Bounded batches avoid reading a huge dependency directory into RAM. The task
// stays frozen throughout this scan; no guest Git executable or hooks run.
func snapshotTaskSource(ctx context.Context, p Prepared, frozen FrozenTask, uid uint32) (sourceSnapshot, changeSnapshot, []e2bbe.Payload, error) {
	snapshot := sourceSnapshot{Version: 1, Policy: capturePolicy, InputBundleSHA256: p.archiveSHA256, Files: []e2bbe.TransferFile{}}
	changes := changeSnapshot{Version: 1, Policy: capturePolicy, InputBundleSHA256: p.archiveSHA256, Changes: []sourceChange{}, Exclusions: []captureExclusion{}}
	if frozen.identity.Identity != p.manifest.Binding.Identity || frozen.identity.InputBundleDigest != p.archiveSHA256 || frozen.identity.SourceDigest != p.manifest.Binding.SourceDigest || frozen.identity.ContextDigest != p.manifest.Binding.ContextDigest {
		return snapshot, changes, nil, ErrRefused
	}
	if e := validatePrepared(p); e != nil {
		return snapshot, changes, nil, e
	}
	if e := frozen.verifyFrozen(ctx); e != nil {
		return snapshot, changes, nil, e
	}
	sourceInfo, e := os.Lstat(filepath.Join(p.directory, "source"))
	if e != nil {
		return snapshot, changes, nil, e
	}
	sourceStat, ok := sourceInfo.Sys().(*syscall.Stat_t)
	if !ok || !sourceInfo.IsDir() || sourceInfo.Mode()&os.ModeSymlink != 0 || sourceStat.Uid != uid {
		return snapshot, changes, nil, ErrRefused
	}
	root, e := os.OpenRoot(filepath.Join(p.directory, "source"))
	if e != nil {
		return snapshot, changes, nil, e
	}
	defer root.Close()
	originals := map[string]e2bbe.TransferFile{}
	parents := map[string]bool{}
	for _, file := range p.manifest.Files {
		if strings.HasPrefix(file.Path, "source/") {
			name := strings.TrimPrefix(file.Path, "source/")
			file.Path = name
			originals[name] = file
			for parent := path.Dir(name); parent != "."; parent = path.Dir(parent) {
				parents[parent] = true
			}
		}
	}
	infos := map[string]os.FileInfo{}
	stack := []string{"."}
	entries, dirs := 0, 0
	total := int64(0)
	for len(stack) > 0 {
		if e = ctx.Err(); e != nil {
			return snapshot, changes, nil, e
		}
		name := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		dirs++
		if dirs > maxCaptureDirectories {
			return snapshot, changes, nil, e2bbe.ErrTransfer
		}
		directory, e := root.Open(name)
		if e != nil {
			return snapshot, changes, nil, e
		}
		for {
			batch, readErr := directory.ReadDir(128)
			for _, entry := range batch {
				entries++
				if entries > maxCaptureEntries {
					directory.Close()
					return snapshot, changes, nil, e2bbe.ErrTransfer
				}
				rel := path.Join(name, entry.Name())
				info, err := entry.Info()
				if err != nil {
					directory.Close()
					return snapshot, changes, nil, err
				}
				_, wasOriginal := originals[rel]
				// .git is intentionally never output or used to determine source claims.
				excluded := captureExcluded(rel, entry.IsDir()) && !wasOriginal && !parents[rel]
				if excluded {
					changes.Exclusions = append(changes.Exclusions, captureExclusion{rel, "fixed-new-path-exclusion"})
					continue
				}
				if info.Mode()&os.ModeSymlink != 0 {
					directory.Close()
					return snapshot, changes, nil, ErrRefused
				}
				if entry.IsDir() {
					st, ok := info.Sys().(*syscall.Stat_t)
					if !ok || st.Uid != uid || st.Dev != sourceStat.Dev {
						directory.Close()
						return snapshot, changes, nil, ErrRefused
					}
					stack = append(stack, rel)
					continue
				}
				if len(snapshot.Files) >= e2bbe.MaxTransferFiles-2 || info.Size() < 0 || total+info.Size() > e2bbe.MaxTransferBytes-(1<<20) {
					directory.Close()
					return snapshot, changes, nil, e2bbe.ErrTransfer
				}
				stat, ok := info.Sys().(*syscall.Stat_t)
				if !ok || stat.Dev != sourceStat.Dev {
					directory.Close()
					return snapshot, changes, nil, ErrRefused
				}
				file, err := sourceFile(root, rel, info, uid)
				if err != nil {
					directory.Close()
					return snapshot, changes, nil, err
				}
				h := sha256.New()
				n, err := io.Copy(h, io.LimitReader(checkedReader{ctx, file}, info.Size()+1))
				closeErr := file.Close()
				if err != nil || closeErr != nil || n != info.Size() {
					directory.Close()
					return snapshot, changes, nil, ErrRefused
				}
				mode := uint32(0644)
				if info.Mode()&0111 != 0 {
					mode = 0755
				}
				snapshot.Files = append(snapshot.Files, e2bbe.TransferFile{Path: rel, Size: n, SHA256: hex.EncodeToString(h.Sum(nil)), Mode: mode})
				infos[rel] = info
				total += n
			}
			if errors.Is(readErr, io.EOF) {
				break
			}
			if readErr != nil {
				directory.Close()
				return snapshot, changes, nil, readErr
			}
		}
		if e = directory.Close(); e != nil {
			return snapshot, changes, nil, e
		}
	}
	sort.Slice(snapshot.Files, func(i, j int) bool { return snapshot.Files[i].Path < snapshot.Files[j].Path })
	sort.Slice(changes.Exclusions, func(i, j int) bool { return changes.Exclusions[i].Path < changes.Exclusions[j].Path })
	for _, file := range snapshot.Files {
		before, exists := originals[file.Path]
		after := file
		if !exists {
			changes.Changes = append(changes.Changes, sourceChange{Path: file.Path, Kind: "added", After: &after})
		} else if before != file {
			changes.Changes = append(changes.Changes, sourceChange{Path: file.Path, Kind: "modified", Before: &before, After: &after})
		}
		delete(originals, file.Path)
	}
	for _, before := range originals {
		value := before
		changes.Changes = append(changes.Changes, sourceChange{Path: before.Path, Kind: "deleted", Before: &value})
	}
	sort.Slice(changes.Changes, func(i, j int) bool { return changes.Changes[i].Path < changes.Changes[j].Path })
	payloads := make([]e2bbe.Payload, 0, len(snapshot.Files)+2)
	for _, file := range snapshot.Files {
		transfer := file
		transfer.Path = "output/source/" + file.Path
		name, info := file.Path, infos[file.Path]
		payloads = append(payloads, e2bbe.Payload{TransferFile: transfer, Open: func() (io.ReadCloser, error) {
			if e := frozen.verifyFrozen(ctx); e != nil {
				return nil, e
			}
			confined, e := os.OpenRoot(filepath.Join(p.directory, "source"))
			if e != nil {
				return nil, e
			}
			handle, e := sourceFile(confined, name, info, uid)
			_ = confined.Close()
			if e != nil {
				return nil, e
			}
			return &contextReadCloser{ctx: ctx, ReadCloser: handle}, nil
		}})
	}
	if e = frozen.verifyFrozen(ctx); e != nil {
		return snapshot, changes, nil, e
	}
	return snapshot, changes, payloads, nil
}

type contextReadCloser struct {
	ctx context.Context
	io.ReadCloser
}

func (r *contextReadCloser) Read(b []byte) (int, error) {
	if e := r.ctx.Err(); e != nil {
		return 0, e
	}
	return r.ReadCloser.Read(b)
}
func metadataPayload(name string, b []byte) e2bbe.Payload {
	return e2bbe.Payload{TransferFile: e2bbe.TransferFile{Path: name, Size: int64(len(b)), SHA256: hashBytes(b), Mode: 0644}, Open: func() (io.ReadCloser, error) { return io.NopCloser(bytes.NewReader(b)), nil }}
}

// CaptureTaskOutput is root-only and writes fixed private transport artifacts.
// It cannot grant native-success, integration, deployment or VM charge release.
// A partial archive/intent is deliberately retained on any error, never replayed.
func CaptureTaskOutput(ctx context.Context, p Prepared, frozen FrozenTask) (e2bbe.CaptureReceipt, error) {
	if os.Geteuid() != 0 {
		return e2bbe.CaptureReceipt{}, ErrRefused
	}
	info, e := os.Lstat("/luci-transfer")
	if e != nil {
		return e2bbe.CaptureReceipt{}, e
	}
	st, ok := info.Sys().(*syscall.Stat_t)
	if !ok || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 || st.Uid != 0 || info.Mode().Perm()&0022 != 0 {
		return e2bbe.CaptureReceipt{}, ErrRefused
	}
	return captureTaskOutput(ctx, p, frozen, "/luci-transfer", 1001)
}
func captureTaskOutput(ctx context.Context, p Prepared, frozen FrozenTask, destination string, uid uint32) (e2bbe.CaptureReceipt, error) {
	var receipt e2bbe.CaptureReceipt
	if e := frozen.verifyFrozen(ctx); e != nil {
		return receipt, e
	}
	if e := validatePrepared(p); e != nil {
		return receipt, e
	}
	bounded, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	ctx = bounded
	identityBytes, e := json.Marshal(frozen.identity)
	if e != nil {
		return receipt, e
	}
	captureID := "capture-" + hashBytes(identityBytes)[:24]
	if e = writeJSON(filepath.Join(p.directory, "capture-intent.json"), struct {
		CaptureID string                `json:"captureId"`
		Identity  e2bbe.CaptureIdentity `json:"identity"`
	}{captureID, frozen.identity}); e != nil {
		return receipt, e
	}
	snapshot, changes, payloads, e := snapshotTaskSource(ctx, p, frozen, uid)
	if e != nil {
		return receipt, e
	}
	sourceBytes, e := json.Marshal(snapshot)
	if e != nil {
		return receipt, e
	}
	changes.SourceManifestSHA256 = hashBytes(sourceBytes)
	changeBytes, e := json.Marshal(changes)
	if e != nil {
		return receipt, e
	}
	if len(sourceBytes)+len(changeBytes) > 1<<20 {
		return receipt, e2bbe.ErrTransfer
	}
	payloads = append(payloads, metadataPayload("output/source-manifest.json", sourceBytes), metadataPayload("output/changes.json", changeBytes))
	file, e := os.OpenFile(filepath.Join(destination, "output.tar"), os.O_RDWR|os.O_CREATE|os.O_EXCL, 0600)
	if e != nil {
		return receipt, e
	}
	defer file.Close()
	binding := p.manifest.Binding
	binding.Direction = "output"
	_, hash, e := e2bbe.WriteTransfer(file, binding, payloads)
	if e != nil {
		return receipt, e
	}
	if e = file.Sync(); e != nil {
		return receipt, e
	}
	info, e := file.Stat()
	if e != nil {
		return receipt, e
	}
	if _, e = file.Seek(0, io.SeekStart); e != nil {
		return receipt, e
	}
	if _, e = e2bbe.VerifyTransfer(checkedReader{ctx, file}, binding, hash); e != nil {
		return receipt, e
	}
	if e = frozen.verifyFrozen(ctx); e != nil {
		return receipt, e
	}
	if e = file.Chmod(0400); e != nil {
		return receipt, e
	}
	if e = file.Sync(); e != nil {
		return receipt, e
	}
	if e = file.Close(); e != nil {
		return receipt, e
	}
	receipt = e2bbe.CaptureReceipt{Version: 1, CaptureID: captureID, Identity: frozen.identity, ArchiveSHA256: hash, ArchiveBytes: info.Size(), CapturedAt: time.Now().UTC(), SourceManifestSHA256: hashBytes(sourceBytes), ChangeManifestSHA256: hashBytes(changeBytes)}
	if e = writeJSON(filepath.Join(destination, "output.json"), receipt); e != nil {
		return e2bbe.CaptureReceipt{}, e
	}
	return receipt, nil
}
