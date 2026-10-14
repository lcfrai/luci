package product

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/lcfrai/luci/control-plane/internal/runtimeoci"
)

const maxWorkspaceCoreBytes int64 = 300 << 20
const maxWorkspaceCoreExpandedBytes int64 = 1 << 30

type workspaceCompanion struct {
	core            *os.File
	metadata        *os.File
	metadataPath    string
	metadataHash    string
	metadataSize    int64
	bundle          *runtimeoci.Bundle
	coreHash        string
	coreSize        int64
	markerHash      string
	payloadHash     string
	payloadSize     int64
	payloadFileName string
	verifyArchive   func() error
}

func (c *workspaceCompanion) close() {
	if c == nil {
		return
	}
	if c.core != nil {
		_ = c.core.Close()
	}
	if c.metadata != nil {
		_ = c.metadata.Close()
		_ = os.Remove(c.metadataPath)
	}
	if c.bundle != nil {
		_ = c.bundle.Close()
	}
}

func (s *Server) openWorkspaceCompanion() (*workspaceCompanion, error) {
	dir, err := s.runtimeSource()
	if err != nil {
		return nil, err
	}
	bundle, err := runtimeoci.Open(dir)
	if err != nil {
		return nil, fmt.Errorf("the installed runtime companion cannot be verified: %w", err)
	}
	result := &workspaceCompanion{bundle: bundle}
	defer func() {
		if err != nil {
			result.close()
		}
	}()
	info, e := os.Lstat(s.cloudBundle)
	if e != nil || !info.Mode().IsRegular() || info.Size() < 1 || info.Size() > maxWorkspaceCoreBytes {
		err = errors.New("the bundled Linux core is missing, linked or oversized")
		return nil, err
	}
	result.core, err = os.Open(s.cloudBundle)
	if err != nil {
		return nil, err
	}
	opened, e := result.core.Stat()
	if e != nil || !os.SameFile(info, opened) {
		err = errors.New("the bundled Linux core changed before inspection")
		return nil, err
	}
	result.markerHash, err = scanWorkspaceCore(result.core, bundle)
	if err != nil {
		return nil, err
	}
	result.metadata, result.metadataPath, result.metadataHash, result.metadataSize, err = buildWorkspaceMetadata(bundle, filepath.Join(s.stateDir, "workspace-transfer-staging"))
	if err != nil {
		return nil, err
	}
	result.coreHash, err = hashWorkspaceFile(result.core, info.Size(), "")
	result.coreSize = info.Size()
	result.payloadHash = bundle.Manifest.Archive.SHA256
	result.payloadSize = bundle.Manifest.Archive.Bytes
	result.payloadFileName = bundle.Manifest.Archive.FileName
	result.verifyArchive = bundle.VerifyArchive
	return result, err
}

// The core intentionally excludes the large payload. Verify that its marker,
// runtime source, receipts and runtime-only grants match the outer verified
// distribution before any SSH/SCP call. Other app licences are package-owned.
func scanWorkspaceCore(file *os.File, bundle *runtimeoci.Bundle) (string, error) {
	if file == nil || bundle == nil {
		return "", errors.New("bundled Linux core unavailable")
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return "", errors.New("bundled Linux core unavailable")
	}
	gz, err := gzip.NewReader(io.LimitReader(file, maxWorkspaceCoreBytes+1))
	if err != nil {
		return "", err
	}
	defer gz.Close()
	wanted := map[string]runtimeoci.Member{}
	for _, source := range bundle.Manifest.SourceFiles {
		wanted["runtime/"+source.Path] = source.Member
	}
	wanted["runtime/source-receipt.json"] = runtimeoci.Member{SHA256: bundle.SourceReceiptSHA256}
	seen := map[string]bool{}
	var expanded int64
	var markerHash string
	reader := tar.NewReader(gz)
	for count := 0; ; count++ {
		header, nextErr := reader.Next()
		if nextErr == io.EOF {
			break
		}
		if nextErr != nil || count >= 20000 || header == nil || header.Name == "" || path.IsAbs(header.Name) || path.Clean(header.Name) != strings.TrimSuffix(header.Name, "/") || strings.HasPrefix(header.Name, "../") || header.Name == ".." || header.Linkname != "" {
			return "", errors.New("bundled Linux core has unsafe or excessive members")
		}
		if header.Typeflag == tar.TypeDir {
			continue
		}
		if header.Typeflag != tar.TypeReg && header.Typeflag != tar.TypeRegA || header.Size < 0 || expanded > maxWorkspaceCoreExpandedBytes-header.Size || seen[header.Name] {
			return "", errors.New("bundled Linux core contains linked, duplicate or oversized content")
		}
		expanded += header.Size
		seen[header.Name] = true
		if header.Name == "runtime/"+bundle.Manifest.Archive.FileName || header.Name == "runtime/runtime-oci-manifest-v1.json" || header.Name == "runtime/runtime-build-receipt.json" || header.Name == "runtime/runtime-source-reuse-v1.json" || header.Name == "runtime/runtime-prior-manifest-v1.json" || header.Name == "runtime/runtime-prior-build-receipt.json" || strings.HasPrefix(header.Name, "licenses/runtime-image/") || strings.HasPrefix(header.Name, "runtime/sandboxd/") && wanted[header.Name].SHA256 == "" || strings.HasPrefix(header.Name, "runtime/images/sandbox/") && wanted[header.Name].SHA256 == "" {
			return "", errors.New("bundled Linux core has unexpected runtime content")
		}
		if header.Name == "runtime/runtime-payload-companion.json" {
			if header.Size > 4096 {
				return "", errors.New("runtime companion marker oversized")
			}
			data, readErr := io.ReadAll(io.LimitReader(reader, header.Size+1))
			if readErr != nil || int64(len(data)) != header.Size || runtimeoci.VerifyCompanionMarker(data, bundle) != nil {
				return "", errors.New("runtime companion marker does not match the installed payload")
			}
			markerHash = hashWorkspaceBytes(data)
			continue
		}
		if expected, ok := wanted[header.Name]; ok {
			if expected.Bytes != 0 && expected.Bytes != header.Size {
				return "", errors.New("bundled runtime member length changed")
			}
			hash := sha256.New()
			if _, readErr := io.CopyN(hash, reader, header.Size); readErr != nil || hex.EncodeToString(hash.Sum(nil)) != expected.SHA256 {
				return "", errors.New("bundled runtime member digest changed")
			}
		} else if _, readErr := io.CopyN(io.Discard, reader, header.Size); readErr != nil {
			return "", readErr
		}
	}
	var trailing [32 * 1024]byte
	for {
		n, readErr := gz.Read(trailing[:])
		if expanded > maxWorkspaceCoreExpandedBytes-int64(n) {
			return "", errors.New("bundled Linux core expanded beyond its bound")
		}
		expanded += int64(n)
		for _, value := range trailing[:n] {
			if value != 0 {
				return "", errors.New("bundled Linux core has nonzero trailing content")
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil || n == 0 {
			return "", errors.New("bundled Linux core did not end cleanly")
		}
	}
	if markerHash == "" {
		return "", errors.New("bundled Linux core has no verified runtime companion marker")
	}
	for name := range wanted {
		if !seen[name] {
			return "", fmt.Errorf("bundled Linux core lacks required runtime member %s", name)
		}
	}
	return markerHash, nil
}

// The producer deliberately omits runtime metadata and image grants from its
// small management core. Transfer only their already verified bytes in a
// separate bounded tar, rather than copying an unbounded outer package.
func buildWorkspaceMetadata(bundle *runtimeoci.Bundle, stagingDir string) (*os.File, string, string, int64, error) {
	if bundle == nil || bundle.Archive == nil {
		return nil, "", "", 0, errors.New("verified runtime metadata unavailable")
	}
	runtimeRoot := filepath.Dir(bundle.Archive.Name())
	packageRoot := filepath.Dir(runtimeRoot)
	wanted := map[string]string{
		"runtime/runtime-oci-manifest-v1.json": bundle.ManifestSHA256,
		"runtime/runtime-build-receipt.json":   bundle.Manifest.Producer.BuildReceiptSHA256,
	}
	if bundle.Manifest.Producer.ReuseEvidenceSHA256 != "" {
		wanted["runtime/runtime-source-reuse-v1.json"] = bundle.Manifest.Producer.ReuseEvidenceSHA256
		reuseBytes, err := readWorkspaceMetadataMember(packageRoot, "runtime/runtime-source-reuse-v1.json", bundle.Manifest.Producer.ReuseEvidenceSHA256)
		if err != nil {
			return nil, "", "", 0, err
		}
		var reuse struct {
			PriorManifestSHA256     string `json:"priorManifestSHA256"`
			PriorBuildReceiptSHA256 string `json:"priorBuildReceiptSHA256"`
		}
		if json.Unmarshal(reuseBytes, &reuse) != nil || len(reuse.PriorManifestSHA256) != 64 || len(reuse.PriorBuildReceiptSHA256) != 64 {
			return nil, "", "", 0, errors.New("runtime reuse originals unavailable")
		}
		wanted["runtime/runtime-prior-manifest-v1.json"] = reuse.PriorManifestSHA256
		wanted["runtime/runtime-prior-build-receipt.json"] = reuse.PriorBuildReceiptSHA256
	}
	for _, grant := range bundle.Manifest.Licenses {
		wanted[grant.Path] = grant.SHA256
	}
	if err := os.MkdirAll(stagingDir, 0700); err != nil {
		return nil, "", "", 0, err
	}
	if info, err := os.Lstat(stagingDir); err != nil || !info.IsDir() || info.Mode().Perm()&0077 != 0 {
		return nil, "", "", 0, errors.New("private workspace staging directory unavailable")
	}
	file, err := os.CreateTemp(stagingDir, "verified-runtime-metadata-*.part")
	if err != nil {
		return nil, "", "", 0, err
	}
	ok := false
	defer func() {
		if !ok {
			_ = file.Close()
			_ = os.Remove(file.Name())
		}
	}()
	if err = file.Chmod(0600); err != nil {
		return nil, "", "", 0, err
	}
	hash := sha256.New()
	archive := tar.NewWriter(io.MultiWriter(file, hash))
	names := make([]string, 0, len(wanted))
	for name := range wanted {
		names = append(names, name)
	}
	sort.Strings(names)
	var contentBytes int64
	for _, name := range names {
		data, readErr := readWorkspaceMetadataMember(packageRoot, name, wanted[name])
		if readErr != nil || contentBytes > 64<<20-int64(len(data)) {
			return nil, "", "", 0, errors.New("verified runtime metadata changed or oversized")
		}
		contentBytes += int64(len(data))
		if err = archive.WriteHeader(&tar.Header{Name: name, Mode: 0644, Typeflag: tar.TypeReg, Size: int64(len(data))}); err != nil {
			return nil, "", "", 0, err
		}
		if _, err = archive.Write(data); err != nil {
			return nil, "", "", 0, err
		}
	}
	if err = archive.Close(); err != nil {
		return nil, "", "", 0, err
	}
	if err = file.Sync(); err != nil {
		return nil, "", "", 0, err
	}
	info, err := file.Stat()
	if err != nil || info.Size() < 1 || info.Size() > 72<<20 {
		return nil, "", "", 0, errors.New("verified runtime metadata tar oversized")
	}
	if _, err = file.Seek(0, io.SeekStart); err != nil {
		return nil, "", "", 0, err
	}
	ok = true
	return file, file.Name(), hex.EncodeToString(hash.Sum(nil)), info.Size(), nil
}

func readWorkspaceMetadataMember(root, name, expectedHash string) ([]byte, error) {
	if path.IsAbs(name) || path.Clean(name) != name || strings.HasPrefix(name, "../") || len(expectedHash) != 64 {
		return nil, errors.New("invalid runtime metadata member")
	}
	current := root
	for _, part := range strings.Split(name, "/") {
		current = filepath.Join(current, part)
		info, err := os.Lstat(current)
		if err != nil || info.Mode()&os.ModeSymlink != 0 {
			return nil, errors.New("linked runtime metadata unavailable")
		}
	}
	info, err := os.Lstat(current)
	if err != nil || !info.Mode().IsRegular() || info.Size() < 0 || info.Size() > 64<<20 {
		return nil, errors.New("runtime metadata unavailable or oversized")
	}
	file, err := os.Open(current)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	opened, err := file.Stat()
	if err != nil || !os.SameFile(info, opened) {
		return nil, errors.New("runtime metadata changed")
	}
	data, err := io.ReadAll(io.LimitReader(file, info.Size()+1))
	if err != nil || int64(len(data)) != info.Size() || hashWorkspaceBytes(data) != expectedHash {
		return nil, errors.New("runtime metadata digest changed")
	}
	return data, nil
}

func hashWorkspaceBytes(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

func hashWorkspaceFile(file *os.File, expectedSize int64, expectedHash string) (string, error) {
	if file == nil || expectedSize < 0 {
		return "", errors.New("verified workspace file unavailable")
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return "", errors.New("verified workspace file unavailable")
	}
	h := sha256.New()
	n, err := io.Copy(h, io.LimitReader(file, expectedSize+1))
	actual := hex.EncodeToString(h.Sum(nil))
	if err != nil || n != expectedSize || expectedHash != "" && actual != expectedHash {
		return "", errors.New("verified workspace file changed")
	}
	return actual, nil
}

func stageWorkspaceFile(ctx context.Context, source *os.File, expectedSize int64, expectedHash, stagingDir, prefix string) (string, error) {
	if source == nil || expectedSize <= 0 || len(expectedHash) != 64 {
		return "", errors.New("verified workspace file unavailable")
	}
	if info, err := source.Stat(); err != nil || !info.Mode().IsRegular() || info.Size() != expectedSize {
		return "", errors.New("verified workspace file changed")
	}
	if err := os.MkdirAll(stagingDir, 0700); err != nil {
		return "", err
	}
	if info, err := os.Lstat(stagingDir); err != nil || !info.IsDir() || info.Mode().Perm()&0077 != 0 {
		return "", errors.New("private workspace staging directory unavailable")
	}
	staged, err := os.CreateTemp(stagingDir, prefix+"-*.part")
	if err != nil {
		return "", err
	}
	name := staged.Name()
	success := false
	defer func() {
		_ = staged.Close()
		if !success {
			_ = os.Remove(name)
		}
	}()
	if err = staged.Chmod(0600); err != nil {
		return "", err
	}
	if _, err = source.Seek(0, io.SeekStart); err != nil {
		return "", err
	}
	h := sha256.New()
	buffer := make([]byte, 256<<10)
	var copied int64
	for copied < expectedSize {
		if err = ctx.Err(); err != nil {
			return "", err
		}
		count := int64(len(buffer))
		if expectedSize-copied < count {
			count = expectedSize - copied
		}
		n, readErr := source.Read(buffer[:count])
		if readErr != nil || n == 0 {
			return "", errors.New("verified workspace file ended during staging")
		}
		if _, err = staged.Write(buffer[:n]); err != nil {
			return "", err
		}
		_, _ = h.Write(buffer[:n])
		copied += int64(n)
	}
	var extra [1]byte
	if n, readErr := source.Read(extra[:]); n != 0 || readErr != io.EOF {
		return "", errors.New("verified workspace file changed during staging")
	}
	if hex.EncodeToString(h.Sum(nil)) != expectedHash {
		return "", errors.New("workspace staging input changed")
	}
	if err = staged.Sync(); err != nil {
		return "", err
	}
	if err = staged.Close(); err != nil {
		return "", err
	}
	success = true
	return name, nil
}
