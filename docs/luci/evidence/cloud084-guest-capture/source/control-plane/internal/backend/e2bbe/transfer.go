package e2bbe

import (
	"archive/tar"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path"
	"sort"
	"strings"
	"time"
)

const MaxTransferBytes int64 = 64 << 20
const MaxTransferFiles = 4096
const maxManifestBytes = 1 << 20

// TransferBinding binds data to one reviewed attempt. No manifest grants
// execution authority, promises checks passed, or requests automatic merging.
type TransferBinding struct {
	Identity      Identity `json:"identity"`
	SourceDigest  string   `json:"sourceDigest"`
	ContextDigest string   `json:"contextDigest"`
	Direction     string   `json:"direction"` // input or output
}
type TransferFile struct {
	Path   string `json:"path"`
	Size   int64  `json:"size"`
	SHA256 string `json:"sha256"`
	Mode   uint32 `json:"mode"` // Git's regular-file modes: 0644 or 0755, never set-id.
}
type TransferManifest struct {
	Version int             `json:"version"`
	Binding TransferBinding `json:"binding"`
	Files   []TransferFile  `json:"files"`
}

// Payload opens a caller-selected immutable byte stream. The caller captures
// authorized files under its existing source lock; this package discovers no
// directories, reads no credentials, executes no code and extracts no archive.
type Payload struct {
	TransferFile
	Open func() (io.ReadCloser, error)
}

var ErrTransfer = errors.New("task transfer is unsafe, changed, incomplete or exceeds its bounds")

func validBinding(b TransferBinding) bool {
	return validRequest(Request{Identity: b.Identity, SourceDigest: b.SourceDigest, ContextDigest: b.ContextDigest, BundleDigest: strings.Repeat("0", 64), MaxSeconds: 1, EstimatedCostMicros: 1}) && (b.Direction == "input" || b.Direction == "output")
}
func validTransferPath(name, direction string) bool {
	if name == "" || len(name) > 240 || path.Clean(name) != name || strings.ContainsAny(name, "\\:\x00*?\"<>|") || strings.HasPrefix(name, "/") {
		return false
	}
	if direction == "input" {
		if !strings.HasPrefix(name, "source/") && !strings.HasPrefix(name, "context/") {
			return false
		}
	} else if !strings.HasPrefix(name, "output/") {
		return false
	}
	for _, r := range name {
		if r < 32 || r > 126 {
			return false
		}
	}
	for _, part := range strings.Split(name, "/") {
		if part == "" || part == "." || part == ".." || strings.EqualFold(part, ".git") || strings.HasSuffix(part, ".") || strings.HasSuffix(part, " ") {
			return false
		}
		base := strings.ToUpper(strings.SplitN(part, ".", 2)[0])
		if base == "CON" || base == "PRN" || base == "AUX" || base == "NUL" || len(base) == 4 && (strings.HasPrefix(base, "COM") || strings.HasPrefix(base, "LPT")) && base[3] >= '0' && base[3] <= '9' {
			return false
		}
	}
	return true
}
func validateManifest(m TransferManifest) error {
	if m.Version != 1 || !validBinding(m.Binding) || len(m.Files) < 1 || len(m.Files) > MaxTransferFiles {
		return ErrTransfer
	}
	total := int64(0)
	seen := map[string]bool{}
	directories := map[string]bool{}
	previous := ""
	for _, f := range m.Files {
		key := strings.ToLower(f.Path)
		if !validTransferPath(f.Path, m.Binding.Direction) || f.Size < 0 || f.Size > MaxTransferBytes || !digestPattern.MatchString(f.SHA256) || (f.Mode != 0644 && f.Mode != 0755) || (strings.HasPrefix(f.Path, "context/") && f.Mode != 0644) || seen[key] || directories[key] || f.Path <= previous {
			return ErrTransfer
		}
		for parent := path.Dir(key); parent != "."; parent = path.Dir(parent) {
			if seen[parent] {
				return ErrTransfer
			}
			directories[parent] = true
		}
		seen[key] = true
		previous = f.Path
		total += f.Size
		if total > MaxTransferBytes {
			return ErrTransfer
		}
	}
	return nil
}
func WriteTransfer(w io.Writer, binding TransferBinding, payloads []Payload) (TransferManifest, string, error) {
	p := append([]Payload(nil), payloads...)
	sort.Slice(p, func(i, j int) bool { return p[i].Path < p[j].Path })
	m := TransferManifest{Version: 1, Binding: binding}
	for _, v := range p {
		if v.Open == nil {
			return m, "", ErrTransfer
		}
		m.Files = append(m.Files, v.TransferFile)
	}
	if e := validateManifest(m); e != nil {
		return m, "", e
	}
	body, e := json.Marshal(m)
	if e != nil || len(body) > maxManifestBytes {
		return m, "", ErrTransfer
	}
	hash := sha256.New()
	tw := tar.NewWriter(io.MultiWriter(w, hash))
	writeHeader := func(name string, size int64, mode uint32) error {
		return tw.WriteHeader(&tar.Header{Name: name, Size: size, Mode: int64(mode), Typeflag: tar.TypeReg, Format: tar.FormatUSTAR, ModTime: time.Unix(0, 0)})
	}
	if e = writeHeader("manifest.json", int64(len(body)), 0600); e != nil {
		return m, "", e
	}
	if _, e = tw.Write(body); e != nil {
		return m, "", e
	}
	for _, v := range p {
		r, e := v.Open()
		if e != nil {
			return m, "", e
		}
		if r == nil {
			return m, "", ErrTransfer
		}
		if e = writeHeader(v.Path, v.Size, v.Mode); e != nil {
			r.Close()
			return m, "", e
		}
		h := sha256.New()
		n, e := io.CopyN(io.MultiWriter(tw, h), r, v.Size)
		var extra [1]byte
		more, endErr := r.Read(extra[:])
		closeErr := r.Close()
		if e != nil || n != v.Size || more != 0 || endErr != io.EOF || closeErr != nil || hex.EncodeToString(h.Sum(nil)) != v.SHA256 {
			return m, "", ErrTransfer
		}
	}
	if e = tw.Close(); e != nil {
		return m, "", e
	}
	return m, hex.EncodeToString(hash.Sum(nil)), nil
}

// VerifyTransfer validates input/returned bytes without extracting to a repo.
// It consumes a bounded complete archive and does not trust tar/PAX paths,
// command claims, modes, file ownership or caller-selected manifest fields.
func VerifyTransfer(r io.Reader, expected TransferBinding, expectedArchiveDigest string) (TransferManifest, error) {
	var m TransferManifest
	if !validBinding(expected) || !digestPattern.MatchString(expectedArchiveDigest) {
		return m, ErrTransfer
	}
	// Metadata/padding overhead is bounded separately from the 64MiB payload.
	lr := &io.LimitedReader{R: r, N: MaxTransferBytes + MaxTransferFiles*1024 + maxManifestBytes + 2048}
	hash := sha256.New()
	stream := io.TeeReader(lr, hash)
	tr := tar.NewReader(stream)
	header, e := tr.Next()
	if e != nil || header.Name != "manifest.json" || header.Typeflag != tar.TypeReg || header.Size < 1 || header.Size > maxManifestBytes || len(header.PAXRecords) > 0 || header.Format != tar.FormatUSTAR {
		return m, ErrTransfer
	}
	body, e := io.ReadAll(io.LimitReader(tr, maxManifestBytes+1))
	if e != nil {
		return m, ErrTransfer
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&m) != nil || decoder.Decode(new(any)) != io.EOF || m.Binding != expected || validateManifest(m) != nil {
		return m, ErrTransfer
	}
	for _, f := range m.Files {
		h, e := tr.Next()
		if e != nil || h.Name != f.Path || h.Typeflag != tar.TypeReg || h.Size != f.Size || h.Mode != int64(f.Mode) || len(h.PAXRecords) > 0 || h.Format != tar.FormatUSTAR || h.Linkname != "" {
			return m, ErrTransfer
		}
		digest := sha256.New()
		n, e := io.Copy(digest, tr)
		if e != nil || n != f.Size || hex.EncodeToString(digest.Sum(nil)) != f.SHA256 {
			return m, ErrTransfer
		}
	}
	if _, e = tr.Next(); e != io.EOF {
		return m, ErrTransfer
	}
	// Accept only archive zero padding, bounded to tar's conventional trailer.
	tail, e := io.ReadAll(io.LimitReader(stream, 1025))
	if e != nil || len(tail) > 1024 {
		return m, ErrTransfer
	}
	for _, v := range tail {
		if v != 0 {
			return m, ErrTransfer
		}
	}
	if lr.N <= 0 || hex.EncodeToString(hash.Sum(nil)) != expectedArchiveDigest {
		return m, fmt.Errorf("%w: archive digest", ErrTransfer)
	}
	return m, nil
}
