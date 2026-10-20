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
	"path/filepath"
	"reflect"
	"strings"
	"unicode/utf8"
)

// DocumentPackagedManifest is a separate, externally pinned prebuilt profile.
// It deliberately does not accept the sandbox runtime manifest or its harness
// probes as evidence for a document tool. The installer owner supplies the
// SHA-256 of these exact manifest bytes independently of the bundle.
type DocumentPackagedManifest struct {
	Version int `json:"version"`
	Kind string `json:"kind"`
	ToolProtocol int `json:"toolProtocol"`
	Provenance struct {
		AssemblyReceiptSHA256 string `json:"assemblyReceiptSha256"`
		InputClosureSHA256 string `json:"inputClosureSha256"`
		CaptureSHA256 string `json:"captureSha256"`
		CaptureBytes int64 `json:"captureBytes"`
		NoticeClosureSHA256 string `json:"noticeClosureSha256"`
		NoticeClosureFileName string `json:"noticeClosureFileName"`
		NoticeClosureBytes int64 `json:"noticeClosureBytes"`
		SourceCommit string `json:"sourceCommit"`
		SourceDigest string `json:"sourceDigest"`
		SourceRetentionDigest string `json:"sourceRetentionDigest"`
	} `json:"provenance"`
	Archive struct {
		FileName string `json:"fileName"`
		SHA256 string `json:"sha256"`
		Bytes int64 `json:"bytes"`
		Members []Member `json:"members"`
	} `json:"archive"`
	Image struct {
		ID string `json:"id"`
		ConfigPath string `json:"configPath"`
		LayerPath string `json:"layerPath"`
		DiffID string `json:"diffId"`
	} `json:"image"`
	ExtractorSHA256 string `json:"extractorSha256"`
}

// DocumentBundle is deliberately separate from Bundle. The latter's manifest,
// receipts and verifier describe the sandbox harness and cannot attest to a
// document extractor. The manifest bytes remain held with the original archive
// descriptor for re-verification immediately before ImageLoad.
type DocumentBundle struct {
	Manifest DocumentPackagedManifest
	ManifestSHA256 string
	Archive *os.File
	manifestBytes []byte
	noticeBytes []byte
}

func (b *DocumentBundle) Close() error {
	if b == nil || b.Archive == nil { return nil }
	return b.Archive.Close()
}

func (b *DocumentBundle) VerifyArchive() error { return VerifyDocumentArchive(b) }

func validDocumentManifest(m DocumentPackagedManifest) bool {
	if m.Version != 1 || m.Kind != "luci-document-prebuilt-docker-save" || m.ToolProtocol != 1 ||
		!digestPattern.MatchString(m.Provenance.AssemblyReceiptSHA256) || !digestPattern.MatchString(m.Provenance.InputClosureSHA256) ||
		!digestPattern.MatchString(m.Provenance.CaptureSHA256) || m.Provenance.CaptureBytes < 1024 || m.Provenance.CaptureBytes > 2<<30 ||
		!digestPattern.MatchString(m.Provenance.NoticeClosureSHA256) || m.Provenance.NoticeClosureFileName != "document-runtime-notices-v1.json" ||
		m.Provenance.NoticeClosureBytes < 1 || m.Provenance.NoticeClosureBytes > maxMetadata || !commitPattern.MatchString(m.Provenance.SourceCommit) ||
		!digestPattern.MatchString(m.Provenance.SourceDigest) || !digestPattern.MatchString(m.Provenance.SourceRetentionDigest) ||
		m.Archive.FileName != "document-image-linux-amd64.tar" || m.Archive.Bytes < 1024 || m.Archive.Bytes > MaxArchiveBytes ||
		!digestPattern.MatchString(m.Archive.SHA256) || !imagePattern.MatchString(m.Image.ID) ||
		m.Image.ConfigPath != strings.TrimPrefix(m.Image.ID, "sha256:")+".json" ||
		!safePath(m.Image.LayerPath) || !strings.HasSuffix(m.Image.LayerPath, "/layer.tar") ||
		m.Image.DiffID != "sha256:"+strings.TrimPrefix(m.Image.DiffID, "sha256:") || !imagePattern.MatchString(m.Image.DiffID) ||
		!digestPattern.MatchString(m.ExtractorSHA256) || len(m.Archive.Members) != 3 {
		return false
	}
	want := map[string]bool{"manifest.json":false, m.Image.ConfigPath:false, m.Image.LayerPath:false}
	if len(want) != 3 { return false }
	for i, member := range m.Archive.Members {
		if !safePath(member.Path) || member.Bytes < 1 || !digestPattern.MatchString(member.SHA256) || i > 0 && m.Archive.Members[i-1].Path >= member.Path {
			return false
		}
		if _, ok := want[member.Path]; !ok || want[member.Path] { return false }
		want[member.Path] = true
		if member.Path == m.Image.ConfigPath && member.SHA256 != strings.TrimPrefix(m.Image.ID,"sha256:") || member.Path == m.Image.LayerPath && member.SHA256 != strings.TrimPrefix(m.Image.DiffID,"sha256:") { return false }
	}
	for _, found := range want { if !found { return false } }
	return true
}

// Docker rootfs tar writers may spell a directory "usr/" and a member
// "./opt/luci/...". Normalize only those two spellings; every other alias,
// absolute path, traversal segment or duplicate normalized name is refused.
func documentLayerName(raw string, directory bool) (string, bool) {
	if !utf8.ValidString(raw) || raw == "" || len(raw) > 1024 || strings.HasPrefix(raw, "/") || strings.ContainsRune(raw, '\\') {
		return "", false
	}
	for _, r := range raw { if r < 32 || r == 127 { return "", false } }
	for strings.HasPrefix(raw, "./") { raw = strings.TrimPrefix(raw, "./") }
	if directory { raw = strings.TrimSuffix(raw, "/") } else if strings.HasSuffix(raw, "/") { return "", false }
	if raw == "" { return "", false }
	for _, part := range strings.Split(raw, "/") { if part == "" || part == "." || part == ".." { return "", false } }
	return raw, true
}

// OpenDocument holds the original archive descriptor and checks the complete
// Docker-save stream before any daemon call. A manifest inside the package is
// not a trust root: its exact bytes must match the separately supplied pin.
func OpenDocument(root, pinnedManifestSHA256 string) (*DocumentBundle, error) {
	if !digestPattern.MatchString(pinnedManifestSHA256) { return nil,errors.New("document manifest pin missing") }
	rootInfo, err := os.Lstat(root)
	if err != nil || !rootInfo.IsDir() || rootInfo.Mode()&os.ModeSymlink != 0 { return nil,errors.New("document bundle root invalid") }
	raw, err := readRegular(root, "document-runtime-manifest-v1.json", maxMetadata)
	if err != nil || sumHex(raw) != pinnedManifestSHA256 { return nil,errors.New("document manifest does not match trusted pin") }
	var m DocumentPackagedManifest
	if decodeStrict(raw,&m) != nil || !validDocumentManifest(m) { return nil,errors.New("document manifest invalid") }
	notice, err := readRegular(root,m.Provenance.NoticeClosureFileName,maxMetadata)
	if err != nil || int64(len(notice)) != m.Provenance.NoticeClosureBytes || sumHex(notice) != m.Provenance.NoticeClosureSHA256 {
		return nil,errors.New("document notice closure does not match pinned package")
	}
	path := filepath.Join(root,m.Archive.FileName)
	if err=rejectLinkedPath(root,m.Archive.FileName);err!=nil{return nil,err}
	before, err := os.Lstat(path)
	if err != nil || !before.Mode().IsRegular() || before.Size() != m.Archive.Bytes { return nil,errors.New("document archive unavailable") }
	f, err := os.Open(path)
	if err != nil { return nil,err }
	opened, err := f.Stat()
	if err != nil || !os.SameFile(before,opened) { f.Close(); return nil,errors.New("document archive identity changed") }
	b := &DocumentBundle{Archive:f, Manifest:m, ManifestSHA256:pinnedManifestSHA256, manifestBytes:raw, noticeBytes:notice}
	if err = b.VerifyArchive(); err != nil { b.Close(); return nil,err }
	return b,nil
}

func VerifyDocumentArchive(b *DocumentBundle) error {
	if b == nil || b.Archive == nil || sumHex(b.manifestBytes) != b.ManifestSHA256 || sumHex(b.noticeBytes) != b.Manifest.Provenance.NoticeClosureSHA256 || int64(len(b.noticeBytes)) != b.Manifest.Provenance.NoticeClosureBytes { return errors.New("document bundle pin changed") }
	var m DocumentPackagedManifest
	if decodeStrict(b.manifestBytes,&m) != nil || !validDocumentManifest(m) || !reflect.DeepEqual(m,b.Manifest) { return errors.New("document bundle identity changed") }
	if _,err := b.Archive.Seek(0,io.SeekStart); err != nil { return err }
	defer b.Archive.Seek(0,io.SeekStart)
	whole := sha256.New()
	limited := io.LimitReader(b.Archive,m.Archive.Bytes+1)
	r := tar.NewReader(io.TeeReader(limited,whole))
	var saved, config []byte
	var extractorFound bool
	seenLayer := make(map[string]bool)
	layerNamesBytes := 0
	for _,want := range m.Archive.Members {
		h,err := r.Next()
		if err != nil || h.Name != want.Path || h.Size != want.Bytes || h.Typeflag != tar.TypeReg && h.Typeflag != tar.TypeRegA || h.Linkname != "" || h.PAXRecords != nil { return errors.New("document archive member invalid") }
		hash := sha256.New()
		if want.Path == m.Image.LayerPath {
			boundedLayer := &io.LimitedReader{R:r,N:h.Size}
			layer := tar.NewReader(io.TeeReader(boundedLayer,hash))
			var consumed int64
			for {
				item,e := layer.Next()
				if e == io.EOF { break }
				if e != nil || item.Size < 0 || item.Size > h.Size-consumed { return errors.New("document layer invalid") }
				name, valid := documentLayerName(item.Name, item.Typeflag == tar.TypeDir)
				layerNamesBytes += len(item.Name)
				if !valid || seenLayer[name] || len(seenLayer) >= 100000 || layerNamesBytes > 16<<20 { return errors.New("document layer name invalid or duplicated") }
				seenLayer[name] = true
				if item.Typeflag != tar.TypeReg && item.Typeflag != tar.TypeRegA && item.Typeflag != tar.TypeDir && item.Typeflag != tar.TypeSymlink || item.Typeflag != tar.TypeReg && item.Typeflag != tar.TypeRegA && item.Size != 0 { return errors.New("document layer member type invalid") }
				if item.Typeflag == tar.TypeSymlink && (name == "opt" || name == "opt/luci" || name == "opt/luci/document_extractor.py") { return errors.New("document tool path is linked") }
				consumed += item.Size
				if name == "opt/luci/document_extractor.py" {
					if extractorFound || item.Typeflag != tar.TypeReg || item.Size < 1 || item.Size > maxMetadata { return errors.New("document extractor invalid") }
					toolHash := sha256.New()
					if _,e = io.CopyN(toolHash,layer,item.Size); e != nil || hex.EncodeToString(toolHash.Sum(nil)) != m.ExtractorSHA256 { return errors.New("document extractor changed") }
					extractorFound = true
				} else if _,e = io.CopyN(io.Discard,layer,item.Size); e != nil { return e }
			}
			// Tar readers may stop before padding. Finish hashing the exact outer
			// member, including all trailing bytes, before comparing its DiffID.
			if _,err = io.Copy(hash,boundedLayer); err != nil { return err }
		} else {
			if want.Bytes > maxMetadata { return errors.New("document save metadata oversized") }
			data := make([]byte,want.Bytes)
			if _,err = io.ReadFull(io.TeeReader(r,hash),data); err != nil { return err }
			if want.Path == "manifest.json" { saved = data } else { config = data }
		}
		if hex.EncodeToString(hash.Sum(nil)) != want.SHA256 { return errors.New("document archive member changed") }
	}
	if _,err := r.Next(); err != io.EOF { return errors.New("document archive extra member") }
	if _,err := io.Copy(whole,limited); err != nil { return err }
	info,err := b.Archive.Stat()
	if err != nil || info.Size() != m.Archive.Bytes || hex.EncodeToString(whole.Sum(nil)) != m.Archive.SHA256 || !extractorFound { return errors.New("document archive digest or extractor changed") }
	var save []struct{ Config string `json:"Config"`; RepoTags []string `json:"RepoTags"`; Layers []string `json:"Layers"` }
	if uniqueJSON(json.NewDecoder(bytes.NewReader(saved))) != nil || decodeStrict(saved,&save) != nil || len(save) != 1 || save[0].Config != m.Image.ConfigPath || len(save[0].RepoTags) != 0 || !equalStrings(save[0].Layers,[]string{m.Image.LayerPath}) { return errors.New("document save index invalid") }
	var cfg struct { OS string `json:"os"`; Architecture string `json:"architecture"`; Config struct { User string `json:"User"`; Entrypoint []string `json:"Entrypoint"`; WorkingDir string `json:"WorkingDir"`; Labels map[string]string `json:"Labels"`; Volumes map[string]json.RawMessage `json:"Volumes"`; ExposedPorts map[string]json.RawMessage `json:"ExposedPorts"` } `json:"config"`; RootFS struct { DiffIDs []string `json:"diff_ids"` } `json:"rootfs"` }
	// Docker image configs contain additional engine-owned fields (history,
	// created, container_config). Reject duplicate keys, then inspect the
	// security-relevant fields without requiring a vendor-specific full shape.
	if uniqueJSON(json.NewDecoder(bytes.NewReader(config))) != nil || json.Unmarshal(config,&cfg) != nil || cfg.OS != "linux" || cfg.Architecture != "amd64" ||
		cfg.Config.User != "1001:1001" || cfg.Config.WorkingDir != "/tmp" ||
		!equalStrings(cfg.Config.Entrypoint,[]string{"/usr/bin/python3","-I","/opt/luci/document_extractor.py"}) ||
		cfg.Config.Labels["org.opencontainers.image.revision"] != m.Provenance.SourceCommit ||
		cfg.Config.Labels["luci.document-source"] != m.Provenance.SourceDigest ||
		cfg.Config.Labels["luci.document-source-retention"] != m.Provenance.SourceRetentionDigest ||
		len(cfg.Config.Volumes) != 0 || len(cfg.Config.ExposedPorts) != 0 || !equalStrings(cfg.RootFS.DiffIDs,[]string{m.Image.DiffID}) { return errors.New("document image config invalid") }
	return nil
}
