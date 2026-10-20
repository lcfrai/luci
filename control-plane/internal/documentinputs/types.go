// Package documentinputs makes bounded observations of retained document-image
// inputs. It has no filesystem, process, network, signature, or grant authority.
// A caller must separately bind the supplied streams to owned original inputs
// and verify signed-index authenticity before using these observations.
package documentinputs

import (
	"bytes"
	"context"
	"crypto/sha256"
	"debug/elf"
	"encoding/hex"
	"errors"
	"io"
	"strings"
	"time"
	"unicode/utf8"
)

var ErrInvalid = errors.New("document input is unavailable, malformed, or exceeds inspection bounds")

const (
	maxWheelBytes int64 = 128 << 20
	maxWheelExpanded int64 = 512 << 20
	maxBaseBytes int64 = 128 << 20
	maxBaseExpanded int64 = 2 << 30
	maxSourceIndexBytes int64 = 64 << 20
	maxSourceCleartextBytes int64 = 128 << 20
	maxSourceArchiveBytes int64 = 128 << 20
	maxSelectedSourceArchiveBytes int64 = 512 << 20
	maxNoticeBytes int64 = 32 << 20
	maxELFBytes int64 = 32 << 20
	maxMembers = 200000
	maxCalls = 131072
	readBlock = 64 << 10
)

type Pin struct {
	SHA256 string `json:"sha256"`
	Bytes int64 `json:"bytes"`
}

type Package struct {
	Name string `json:"name"`
	Version string `json:"version"`
	Architecture string `json:"architecture"`
	MultiArch string `json:"multiArch,omitempty"`
	Source string `json:"source,omitempty"`
	BuiltUsing []string `json:"builtUsing"`
	StaticBuiltUsing []string `json:"staticBuiltUsing"`
	Depends []string `json:"depends"`
	PreDepends []string `json:"preDepends"`
	Provides []string `json:"provides"`
	Breaks []string `json:"breaks"`
	Conflicts []string `json:"conflicts"`
}

type File struct {
	Path string `json:"path"`
	Owner string `json:"owner,omitempty"`
	Kind string `json:"kind"` // regular, directory, symlink, or hardlink
	Bytes int64 `json:"bytes"`
	SHA256 string `json:"sha256,omitempty"`
	Target string `json:"target,omitempty"`
	Architecture string `json:"architecture,omitempty"`
	Needed []string `json:"needed,omitempty"`
}

type Notice struct {
	Path string `json:"path"`
	Owner string `json:"owner,omitempty"`
	Bytes int64 `json:"bytes"`
	SHA256 string `json:"sha256"`
	Contents []byte `json:"contents"` // original grant/license bytes, never inferred text
}

type Component struct {
	Path string `json:"path"`
	Architecture string `json:"architecture"`
	Needed []string `json:"needed"`
}

type WheelOwner struct {
	ID string `json:"id"`
	Version string `json:"version"`
	InstalledOwner string `json:"installedOwner"` // exact expected *.dist-info directory
}

type Wheel struct {
	ID string `json:"id"`
	Version string `json:"version"`
	RequiresDist []string `json:"requiresDist"`
	RequiresPython string `json:"requiresPython,omitempty"`
	Tags []string `json:"tags"`
	RootIsPurelib bool `json:"rootIsPurelib"`
	Components []Component `json:"components"`
	Files []File `json:"files"`
	Notices []Notice `json:"notices"`
}

type Base struct {
	Packages []Package `json:"packages"`
	Files []File `json:"files"`
	Notices []Notice `json:"notices"`
}

type SourceArchive struct {
	FileName string `json:"fileName"`
	Bytes int64 `json:"bytes"`
	SHA256 string `json:"sha256"`
}

type Source struct {
	Name string `json:"name"`
	Version string `json:"version"`
	Archives []SourceArchive `json:"archives"`
}

// SourcesBinding is an API-owned observation of a separately authenticated
// Debian release/index relation. This package verifies cleartext and archive
// bytes only; it neither authenticates signatures nor trusts a bool claim.
type SourcesBinding struct {
	SignedReleaseSHA256 string `json:"signedReleaseSha256"`
	IndexPath string `json:"indexPath"`
	CompressedIndex Pin `json:"compressedIndex"`
	Cleartext Pin `json:"cleartext"`
}

type SourceArchiveInput struct {
	SourceName string
	SourceVersion string
	FileName string
	Pin Pin
	Reader io.ReaderAt
}

func validHex(value string) bool {
	if len(value) != 64 { return false }
	for _, c := range value { if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f') { return false } }
	return true
}

func validPin(pin Pin, max int64) bool { return pin.Bytes > 0 && pin.Bytes <= max && validHex(pin.SHA256) }

func checkedContext(ctx context.Context, duration time.Duration) (context.Context, context.CancelFunc, error) {
	if ctx == nil || ctx.Err() != nil { return nil, nil, ErrInvalid }
	bounded, cancel := context.WithTimeout(ctx, duration)
	return bounded, cancel, nil
}

func cleanName(name string, allowDotPrefix bool) (string, bool) {
	if !utf8.ValidString(name) || len(name) == 0 || len(name) > 1024 || strings.HasPrefix(name,"/") || strings.ContainsRune(name,'\\') { return "",false }
	for _, r := range name { if r < 32 || r == 127 { return "",false } }
	if allowDotPrefix { for strings.HasPrefix(name,"./") { name = strings.TrimPrefix(name,"./") } }
	name = strings.TrimSuffix(name,"/")
	if name == "" { return "",false }
	for _, part := range strings.Split(name,"/") { if part == "" || part == "." || part == ".." { return "",false } }
	return name,true
}

func hashBytes(raw []byte) string { digest := sha256.Sum256(raw); return hex.EncodeToString(digest[:]) }

type boundedReaderAt struct {
	ctx context.Context
	reader io.ReaderAt
	calls int
	bytes int64
	maxBytes int64
}

func (r *boundedReaderAt) ReadAt(p []byte, off int64) (int,error) {
	if len(p)==0{return 0,nil}
	if r.ctx.Err() != nil || r.calls >= maxCalls || off < 0 || int64(len(p)) > r.maxBytes-r.bytes { return 0,ErrInvalid }
	r.calls++
	n,err := r.reader.ReadAt(p,off)
	r.bytes += int64(n)
	if n < 0 || n > len(p) || r.ctx.Err() != nil { return 0,ErrInvalid }
	return n,err
}

func hashReaderAt(ctx context.Context, reader io.ReaderAt, size int64, budget *boundedReaderAt) (string,error) {
	if reader == nil || size < 1 { return "",ErrInvalid }
	h := sha256.New()
	var block [readBlock]byte
	for off := int64(0); off < size; {
		if ctx.Err() != nil { return "",ErrInvalid }
		need := int64(len(block)); if size-off < need { need = size-off }
		n,err := budget.ReadAt(block[:int(need)],off)
		if n != int(need) || err != nil && err != io.EOF { return "",ErrInvalid }
		_,_ = h.Write(block[:n]); off += int64(n)
	}
	return hex.EncodeToString(h.Sum(nil)),nil
}

func nativeComponent(raw []byte) (string,[]string,error) {
	if len(raw) < 6 || string(raw[:4]) != "\x7fELF" || raw[5] != byte(elf.ELFDATA2LSB) { return "",nil,ErrInvalid }
	if int64(len(raw)) > maxELFBytes { return "",nil,ErrInvalid }
	parsed,err := elf.NewFile(bytes.NewReader(raw))
	if err != nil || parsed.Class != elf.ELFCLASS64 || parsed.Data != elf.ELFDATA2LSB || parsed.Machine != elf.EM_X86_64 || parsed.Type != elf.ET_DYN && parsed.Type != elf.ET_EXEC { return "",nil,ErrInvalid }
	needed,err := parsed.ImportedLibraries(); if err != nil { return "",nil,ErrInvalid }
	if len(needed) > 512 { return "",nil,ErrInvalid }
	for _, name := range needed { if _,ok := cleanName(name,false); !ok || strings.Contains(name,"/") { return "",nil,ErrInvalid } }
	return "amd64",needed,nil
}

// elfCapture retains bytes only after the first four bytes prove ELF. Archive
// readers may return fewer than four bytes in any individual successful read.
type elfCapture struct {
	prefix [4]byte
	prefixBytes int
	decided bool
	isELF bool
	raw bytes.Buffer
}

func (c *elfCapture) write(part []byte) error {
	if !c.decided {
		need:=len(c.prefix)-c.prefixBytes
		if need>len(part){need=len(part)}
		copy(c.prefix[c.prefixBytes:],part[:need]);c.prefixBytes+=need;part=part[need:]
		if c.prefixBytes==len(c.prefix) {
			c.decided=true;c.isELF=string(c.prefix[:])=="\x7fELF"
			if c.isELF {_,_=c.raw.Write(c.prefix[:])}
		}
	}
	if c.isELF {
		if int64(c.raw.Len())+int64(len(part))>maxELFBytes{return ErrInvalid}
		_,_=c.raw.Write(part)
	}
	return nil
}
