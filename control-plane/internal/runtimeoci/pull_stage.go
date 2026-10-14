package runtimeoci

import (
	"archive/tar"
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
	Version        int    `json:"version"`
	ManifestSHA256 string `json:"manifestSHA256"`
	StoreSHA256    string `json:"storeSHA256"`
	Repository     string `json:"repository"`
	ManifestDigest string `json:"manifestDigest"`
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
	for ancestor := parent; ; ancestor = filepath.Dir(ancestor) {
		info, err := os.Lstat(ancestor)
		if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return errors.New("runtime pull stage parent unavailable")
		}
		if filepath.Dir(ancestor) == ancestor {
			break
		}
	}
	if err := os.Mkdir(destination, 0700); err != nil {
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
		if err := root.MkdirAll(path.Dir(want.Path), 0700); err != nil {
			return err
		}
		f, err := root.OpenFile(want.Path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
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
	receipt := PullStageReceipt{Version: 1, ManifestSHA256: b.ManifestSHA256, StoreSHA256: b.Manifest.RegistryStore.SHA256, Repository: b.Manifest.RegistryStore.Repository, ManifestDigest: b.Manifest.Image.ManifestDigest}
	data, err := json.Marshal(receipt)
	if err != nil {
		return err
	}
	f, err := root.OpenFile("runtime-pull-stage.json", os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
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
	return protectPullPath(filepath.Join(destination, "runtime-pull-stage.json"), false)
}
