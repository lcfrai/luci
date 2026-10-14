package runtimeoci

import (
	"archive/tar"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path"
	"path/filepath"
	"time"
)

type PullStageReceipt struct {
	Version         int    `json:"version"`
	ManifestSHA256  string `json:"manifestSHA256"`
	StoreSHA256     string `json:"storeSHA256"`
	Repository      string `json:"repository"`
	ManifestDigest  string `json:"manifestDigest"`
	AllowlistSHA256 string `json:"allowlistSHA256"`
	AllowlistBytes  int    `json:"allowlistBytes"`
}

type pullAllowlist struct {
	Version        int    `json:"version"`
	Repository     string `json:"repository"`
	ManifestDigest string `json:"manifestDigest"`
	ManifestBytes  int64  `json:"manifestBytes"`
	ConfigDigest   string `json:"configDigest"`
	ConfigBytes    int64  `json:"configBytes"`
	Layers         []struct {
		Digest string `json:"digest"`
		Bytes  int64  `json:"bytes"`
	} `json:"layers"`
}

func (b *PullBundle) pullAllowlistBytes() ([]byte, error) {
	m := b.Manifest
	a := pullAllowlist{Version: 1, Repository: m.RegistryStore.Repository, ManifestDigest: m.Image.ManifestDigest, ConfigDigest: m.Image.ID, ConfigBytes: m.Image.ConfigBytes}
	for _, member := range m.RegistryStore.Members {
		if member.Path == pullBlobPath(m.Image.ManifestDigest) {
			a.ManifestBytes = member.Bytes
		}
	}
	if a.ManifestBytes < 1 || a.ManifestBytes > maxMetadata {
		return nil, errors.New("runtime pull manifest blob unavailable")
	}
	for _, layer := range m.Image.Layers {
		a.Layers = append(a.Layers, struct {
			Digest string `json:"digest"`
			Bytes  int64  `json:"bytes"`
		}{Digest: layer.CompressedDigest, Bytes: layer.CompressedBytes})
	}
	data, err := json.Marshal(a)
	if err != nil || len(data) > 1<<20 {
		return nil, errors.New("runtime pull allowlist unavailable")
	}
	return data, nil
}

// ReadPullStage binds a completed private staging receipt and allowlist to
// this held package. A missing or changed receipt is never inferred from a
// directory's existence.
func (b *PullBundle) ReadPullStage(destination string) (PullStageReceipt, error) {
	var receipt PullStageReceipt
	if b == nil || !filepath.IsAbs(destination) {
		return receipt, errors.New("runtime pull stage identity unavailable")
	}
	data, err := readRegular(destination, "runtime-pull-stage.json", 4096)
	if err != nil || decodeStrict(data, &receipt) != nil {
		return PullStageReceipt{}, errors.New("runtime pull stage receipt unavailable")
	}
	allowlist, err := b.pullAllowlistBytes()
	if err != nil || receipt.Version != 1 || receipt.ManifestSHA256 != b.ManifestSHA256 || receipt.StoreSHA256 != b.Manifest.RegistryStore.SHA256 || receipt.Repository != b.Manifest.RegistryStore.Repository || receipt.ManifestDigest != b.Manifest.Image.ManifestDigest || receipt.AllowlistBytes != len(allowlist) || receipt.AllowlistSHA256 != sumHex(allowlist) {
		return PullStageReceipt{}, errors.New("runtime pull stage receipt differs from package")
	}
	actual, err := readRegular(destination, "runtime-pull-allowlist.json", 1<<20)
	if err != nil || !bytes.Equal(actual, allowlist) {
		return PullStageReceipt{}, errors.New("runtime pull stage allowlist changed")
	}
	return receipt, nil
}

// VerifyPullStage reopens every staged member under the private root. The
// helper receives this directory only after this check and a fresh daemon
// admission; it never trusts a prior path-only receipt.
func (b *PullBundle) VerifyPullStage(destination string) (PullStageReceipt, error) {
	receipt, err := b.ReadPullStage(destination)
	if err != nil {
		return receipt, err
	}
	wanted := map[string]Member{}
	for _, member := range b.Manifest.RegistryStore.Members {
		wanted[member.Path] = member
	}
	for _, member := range b.Manifest.RegistryStore.Members {
		if !safePullPath(member.Path) || rejectLinkedPath(destination, filepath.FromSlash(member.Path)) != nil {
			return PullStageReceipt{}, errors.New("runtime pull stage member linked or missing")
		}
		name := filepath.Join(destination, filepath.FromSlash(member.Path))
		before, err := os.Lstat(name)
		if err != nil || !before.Mode().IsRegular() || before.Size() != member.Bytes {
			return PullStageReceipt{}, errors.New("runtime pull stage member size changed")
		}
		f, err := os.Open(name)
		if err != nil {
			return PullStageReceipt{}, err
		}
		opened, statErr := f.Stat()
		if statErr != nil || !os.SameFile(before, opened) {
			_ = f.Close()
			return PullStageReceipt{}, errors.New("runtime pull stage member replaced")
		}
		hash := sha256.New()
		n, copyErr := io.Copy(hash, io.LimitReader(f, member.Bytes+1))
		closeErr := f.Close()
		if copyErr != nil || closeErr != nil || n != member.Bytes || hex.EncodeToString(hash.Sum(nil)) != member.SHA256 {
			return PullStageReceipt{}, errors.New("runtime pull stage member bytes changed")
		}
	}
	err = filepath.WalkDir(destination, func(name string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return errors.New("runtime pull stage contains link")
		}
		if entry.IsDir() {
			return nil
		}
		relative, err := filepath.Rel(destination, name)
		if err != nil || !entry.Type().IsRegular() {
			return errors.New("runtime pull stage has non-file")
		}
		path := filepath.ToSlash(relative)
		if path != "runtime-pull-stage.json" && path != "runtime-pull-allowlist.json" {
			if _, ok := wanted[path]; !ok {
				return errors.New("runtime pull stage has extra member")
			}
		}
		return nil
	})
	if err != nil {
		return PullStageReceipt{}, err
	}
	return receipt, nil
}

// StageStore extracts only previously verified compressed registry members to
// a fresh private directory. The caller must durably own destination and have
// admitted the stage's disk and host demand before calling this method.
func (b *PullBundle) StageStore(destination string) error {
	if b == nil || b.Store == nil || !filepath.IsAbs(destination) {
		return errors.New("runtime pull stage destination invalid")
	}
	if err := b.VerifyPullPayload(); err != nil {
		return err
	}
	parent := filepath.Dir(destination)
	if err := validatePullPrivateParent(parent); err != nil {
		return err
	}
	for ancestor := parent; ; ancestor = filepath.Dir(ancestor) {
		info, err := os.Lstat(ancestor)
		if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return errors.New("runtime pull stage parent unavailable")
		}
		if filepath.Dir(ancestor) == ancestor {
			break
		}
	}
	if err := os.Mkdir(destination, 0755); err != nil {
		return errors.New("runtime pull stage must be fresh")
	}
	if err := protectPullPath(destination, true); err != nil {
		return err
	}
	root, err := os.OpenRoot(destination)
	if err != nil {
		return err
	}
	defer root.Close()
	if _, err := b.Store.Seek(0, io.SeekStart); err != nil {
		return err
	}
	whole := sha256.New()
	tee := io.TeeReader(io.LimitReader(b.Store, b.Manifest.RegistryStore.Bytes+1), whole)
	reader := tar.NewReader(tee)
	deadline := time.Now().Add(pullTotalDeadline)
	for _, want := range b.Manifest.RegistryStore.Members {
		header, err := reader.Next()
		if err != nil || header.Name != want.Path || header.Typeflag != tar.TypeReg && header.Typeflag != tar.TypeRegA || header.Size != want.Bytes || header.Linkname != "" || header.PAXRecords != nil || time.Now().After(deadline) {
			return errors.New("runtime pull staged member changed")
		}
		if err := root.MkdirAll(path.Dir(want.Path), 0755); err != nil {
			return err
		}
		f, err := root.OpenFile(want.Path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
		if err != nil {
			return err
		}
		hash := sha256.New()
		n, copyErr := copyPullBound(io.MultiWriter(f, hash), io.LimitReader(reader, header.Size), header.Size, deadline)
		syncErr := f.Sync()
		closeErr := f.Close()
		if copyErr != nil || syncErr != nil || closeErr != nil || n != header.Size || hex.EncodeToString(hash.Sum(nil)) != want.SHA256 {
			return errors.New("runtime pull staged bytes changed")
		}
		if err := protectPullPath(filepath.Join(destination, filepath.FromSlash(want.Path)), false); err != nil {
			return err
		}
	}
	if _, err := reader.Next(); err != io.EOF {
		return errors.New("runtime pull stage has extra member")
	}
	var tail [32 << 10]byte
	for {
		n, err := tee.Read(tail[:])
		for _, value := range tail[:n] {
			if value != 0 {
				return errors.New("runtime pull stage trailing bytes changed")
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil || n == 0 {
			return errors.New("runtime pull stage tail unavailable")
		}
	}
	if hex.EncodeToString(whole.Sum(nil)) != b.Manifest.RegistryStore.SHA256 {
		return errors.New("runtime pull stage archive changed")
	}
	allowlist, err := b.pullAllowlistBytes()
	if err != nil {
		return err
	}
	allowlistHash := sha256.Sum256(allowlist)
	allowlistFile, err := root.OpenFile("runtime-pull-allowlist.json", os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	_, err = allowlistFile.Write(allowlist)
	if err == nil {
		err = allowlistFile.Sync()
	}
	allowlistCloseErr := allowlistFile.Close()
	if err != nil || allowlistCloseErr != nil {
		return errors.New("runtime pull allowlist could not be saved")
	}
	if err := protectPullPath(filepath.Join(destination, "runtime-pull-allowlist.json"), false); err != nil {
		return err
	}
	receipt := PullStageReceipt{Version: 1, ManifestSHA256: b.ManifestSHA256, StoreSHA256: b.Manifest.RegistryStore.SHA256, Repository: b.Manifest.RegistryStore.Repository, ManifestDigest: b.Manifest.Image.ManifestDigest, AllowlistSHA256: hex.EncodeToString(allowlistHash[:]), AllowlistBytes: len(allowlist)}
	data, err := json.Marshal(receipt)
	if err != nil {
		return err
	}
	f, err := root.OpenFile("runtime-pull-stage.json", os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	_, err = f.Write(data)
	if err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err != nil || closeErr != nil {
		return errors.New("runtime pull stage receipt could not be saved")
	}
	if err := protectPullPath(filepath.Join(destination, "runtime-pull-stage.json"), false); err != nil {
		return err
	}
	if err := exposePullReadOnlyTree(destination); err != nil {
		return err
	}
	// File fsyncs above protect the bytes. On platforms that support a
	// directory flush, persist the directory entries too. The caller always
	// rechecks this receipt and every file before a later Docker effect.
	return syncPullDirectory(destination)
}
