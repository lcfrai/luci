package api

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"io"
	"sync"
	"time"
)

// Raw observations only: no native reader, file, completion, retained ref or
// adoption capability. Full bundle/coverage semantics require the later owner.
const debianBinaryRawWireMax = 20 << 20
const debianBinaryRawStderrMax = 64 << 10
const debianBinaryRawScratchMax = 64 << 10
const debianBinaryRawBundleMax = 4 << 20
const debianBinaryRawCallsMax = 65536
const debianBinaryRawMagic = "LUCI-DBBE1\n"

var errDebianBinaryRaw = errors.New("binary raw observation incomplete")

type debianBinaryRawStream uint8
const (
	debianBinaryRawStdout debianBinaryRawStream = 1
	debianBinaryRawStderr debianBinaryRawStream = 2
)

type debianBinaryRawExpected struct {
	OperationID, LoanID, SourceBindingSHA256, SelectionSHA256 string
	SignedIndexes assemblyCatalogRef
	Selection assemblyDebianBinarySelectionV1
	Indexes [4]assemblyDebianObjectRef
}

type debianBinaryRawRef struct {
	Kind uint8 `json:"kind"`
	Part uint8 `json:"part"`
	Bytes int `json:"bytes"`
	SHA256 string `json:"sha256"`
}

type debianBinaryRawCounters struct {
	State string `json:"state"`
	CompressedReadBytes int64 `json:"compressedReadBytes"`
	ReadCalls int `json:"readCalls"`
	ExpandedBytes int64 `json:"expandedBytes"`
	Paragraphs int `json:"paragraphs"`
}

type debianBinaryRawClassDebit struct {
	Bytes int64 `json:"bytes"`
	Calls int `json:"calls"`
}

type debianBinaryRawSummary struct {
	Schema int `json:"schema"`
	State string `json:"state"`
	OperationID string `json:"operationId"`
	LoanID string `json:"loanId"`
	SourceBindingSHA256 string `json:"sourceBindingSha256"`
	SelectionSHA256 string `json:"selectionSha256"`
	SignedIndexes assemblyCatalogRef `json:"signedIndexes"`
	RawOutputs []debianBinaryRawRef `json:"rawOutputs"`
	DecodedIndexes []assemblyDebianBinaryDecodedIndexV1 `json:"decodedIndexes"`
	Counters debianBinaryRawCounters `json:"counters"`
	SeekCalls int `json:"seekCalls"`
	ClassDebits struct {
		Index debianBinaryRawClassDebit `json:"index"`
		Small debianBinaryRawClassDebit `json:"small"`
	} `json:"classDebits"`
	PackageCount int `json:"packageCount"`
	PackageBytes int64 `json:"packageBytes"`
	SourceArchiveBinding string `json:"sourceArchiveBinding"`
	DependencyClosure string `json:"dependencyClosure"`
	ExpectedInstalledFiles string `json:"expectedInstalledFiles"`
	ObservedInstalledOutput string `json:"observedInstalledOutput"`
	ServerEligibility string `json:"serverEligibility"`
}

// Copies of the decoder, tickets and views share this same lifetime/quota.
type debianBinaryRawDecoder struct { state *debianBinaryRawState }
type debianBinaryRawRead struct {
	state *debianBinaryRawState
	serial uint32
	stream debianBinaryRawStream
	requested int
}
type debianBinaryRawView struct { state *debianBinaryRawState; generation uint32 }
type debianBinaryRawObservation struct {
	State string
	Frames, StdoutDebit, StderrDebit int
	ReadAttempts uint32
	Pending bool
	Refs [23]debianBinaryRawRef
}
type debianBinaryRawState struct {
	mu sync.Mutex
	expected debianBinaryRawExpected
	created, deadline, lastAt time.Time
	failed, retired bool
	pending *debianBinaryRawRead
	calls, generation uint32
	stdoutDebit, stderrDebit int
	stdoutEOF, stderrEOF bool
	scratch [debianBinaryRawScratchMax]byte
	scratchAt, scratchEnd int
	magicAt, headerAt int
	header [38]byte
	// One fixed arena, allocated lazily once. Never append/grow, allocate a
	// second frame arena, or let a view expose this backing array.
	arena []byte
	length, bodyAt, frames, bundleBytes int
	ready bool
	refs [23]debianBinaryRawRef
}

func newDebianBinaryRawDecoder(e debianBinaryRawExpected, created, deadline time.Time) (debianBinaryRawDecoder, error) {
	if !checkLogicalID(e.OperationID) || !checkLogicalID(e.LoanID) || !assemblyCatalogHex(e.SourceBindingSHA256) || !assemblyCatalogHex(e.SelectionSHA256) || !validAssemblyCatalogRef(e.SignedIndexes) || e.SignedIndexes.Kind != "signed-index-bindings" || checkAssemblyDebianBinarySelection(e.Selection) != nil || created.IsZero() || !deadline.After(created) || deadline.Sub(created) > 300*time.Second {
		return debianBinaryRawDecoder{}, errDebianBinaryRaw
	}
	total := int64(0)
	for i, r := range e.Indexes {
		if !checkAssemblyDebianObject(r, "xz-index", 16<<20) || i > 0 && r.SHA256 <= e.Indexes[i-1].SHA256 || r.Bytes > assemblyDebianBinaryIndexBytes-total { return debianBinaryRawDecoder{}, errDebianBinaryRaw }
		total += r.Bytes
	}
	if total != assemblyDebianBinaryIndexBytes { return debianBinaryRawDecoder{}, errDebianBinaryRaw }
	// Fixed174/at most6 IDs and existing field limits bound container overhead;
	// separately cap retained expected text at1MiB. No frame arena exists yet.
	metadataBytes := len(e.OperationID)+len(e.LoanID)+len(e.SourceBindingSHA256)+len(e.SelectionSHA256)+len(e.SignedIndexes.Digest)+len(e.SignedIndexes.Kind)+len(e.Selection.DependencyLockSHA256)+len(e.Selection.AcquisitionManifestSHA256)+len(e.Selection.CandidatePolicySHA256)
	for _, r := range e.Indexes { metadataBytes += len(r.Role)+len(r.SHA256) }
	for _, r := range e.Selection.Packages {
		metadataBytes += len(r.Name)+len(r.Version)+len(r.Architecture)+len(r.Filename)+len(r.SHA256)
		for _, id := range r.AllowedCandidateIDs { metadataBytes += len(id) }
	}
	if metadataBytes > 1<<20 { return debianBinaryRawDecoder{}, errDebianBinaryRaw }
	// Do not retain caller-mutable slices/candidate IDs.
	rows := make([]assemblyDebianBinaryExpectedTupleV1, len(e.Selection.Packages))
	for i, row := range e.Selection.Packages {
		rows[i] = row
		rows[i].AllowedCandidateIDs = append([]string(nil), row.AllowedCandidateIDs...)
	}
	e.Selection.Packages = rows
	return debianBinaryRawDecoder{&debianBinaryRawState{expected: e, created: created, deadline: deadline, lastAt: created}}, nil
}

func (s *debianBinaryRawState) current(at time.Time) bool {
	if s.failed || s.retired || at.IsZero() || at.Before(s.created) || at.Before(s.lastAt) || !at.Before(s.deadline) { return false }
	s.lastAt = at
	return true
}
func (s *debianBinaryRawState) refuse() error {
	s.failed = true; s.generation++; s.ready = false
	return errDebianBinaryRaw
}

// Begin/observe are a pure model of a read boundary, not permission to perform
// IO. The later native owner must bind them to its actual original call.
func (d debianBinaryRawDecoder) beginRead(stream debianBinaryRawStream, maximum int, at time.Time) (debianBinaryRawRead, error) {
	if d.state == nil { return debianBinaryRawRead{}, errDebianBinaryRaw }
	s := d.state; s.mu.Lock(); defer s.mu.Unlock()
	if !s.current(at) || s.pending != nil || maximum < 1 || maximum > debianBinaryRawScratchMax || s.calls >= debianBinaryRawCallsMax || stream != debianBinaryRawStdout && stream != debianBinaryRawStderr { return debianBinaryRawRead{}, s.refuse() }
	if stream == debianBinaryRawStdout && (s.stdoutEOF || s.ready || s.scratchAt != s.scratchEnd) || stream == debianBinaryRawStderr && s.stderrEOF { return debianBinaryRawRead{}, s.refuse() }
	debit, limit := &s.stdoutDebit, debianBinaryRawWireMax
	if stream == debianBinaryRawStderr { debit, limit = &s.stderrDebit, debianBinaryRawStderrMax }
	if maximum > limit-*debit { return debianBinaryRawRead{}, s.refuse() }
	if stream == debianBinaryRawStdout && s.frames == 23 && maximum != 1 { return debianBinaryRawRead{}, s.refuse() }
	*debit += maximum; s.calls++
	r := debianBinaryRawRead{s, s.calls, stream, maximum}; s.pending = &r
	return r, nil
}

func (d debianBinaryRawDecoder) observeRead(r debianBinaryRawRead, data []byte, readErr error, at time.Time) error {
	if d.state == nil { return errDebianBinaryRaw }
	s := d.state; s.mu.Lock(); defer s.mu.Unlock()
	if !s.current(at) || r.state != s || s.pending == nil || *s.pending != r || len(data) > r.requested { return s.refuse() }
	// Partial data+error (including EOF) is deliberately not a short-success
	// refund. Keep the full attempt charged and never accept those bytes.
	if readErr != nil && (readErr != io.EOF || len(data) != 0 || r.requested != 1) { return s.refuse() }
	if readErr == nil && len(data) == 0 { return s.refuse() }
	if readErr == io.EOF {
		if r.stream == debianBinaryRawStdout {
			if s.frames != 23 || s.ready || s.scratchAt != s.scratchEnd || s.headerAt != 0 { return s.refuse() }
			s.stdoutEOF = true
		} else { s.stderrEOF = true }
		s.pending = nil
		return nil
	}
	actual := len(data); if actual < 1 { actual = 1 }
	if r.stream == debianBinaryRawStderr {
		s.stderrDebit -= r.requested-actual; s.pending = nil
		return nil // Never retain native stderr/content in a raw result.
	}
	if s.frames == 23 { return s.refuse() }
	copy(s.scratch[:], data); s.scratchAt = 0; s.scratchEnd = len(data)
	if err := s.pump(); err != nil { return err }
	s.stdoutDebit -= r.requested-actual; s.pending = nil
	return nil
}

func (s *debianBinaryRawState) pump() error {
	for s.scratchAt < s.scratchEnd && !s.ready {
		if s.frames == 23 { return s.refuse() }
		if s.magicAt < len(debianBinaryRawMagic) {
			if s.scratch[s.scratchAt] != debianBinaryRawMagic[s.magicAt] { return s.refuse() }
			s.magicAt++; s.scratchAt++; continue
		}
		if s.headerAt < len(s.header) {
			n := copy(s.header[s.headerAt:], s.scratch[s.scratchAt:s.scratchEnd]); s.headerAt += n; s.scratchAt += n
			if s.headerAt != len(s.header) { continue }
			kind, part := uint8(1+s.frames%2), uint8(s.frames/2)
			if s.frames == 22 { kind, part = 3, 255 }
			s.length = int(binary.LittleEndian.Uint32(s.header[2:6]))
			limit := debianBinaryRawScratchMax; if kind == 1 { limit = debianBinaryRawBundleMax }
			if s.header[0] != kind || s.header[1] != part || s.length < 1 || s.length > limit || kind == 1 && s.length > (16<<20)-s.bundleBytes { return s.refuse() }
			if kind == 1 { s.bundleBytes += s.length }
			if s.arena == nil { s.arena = make([]byte, debianBinaryRawBundleMax) }
			s.bodyAt = 0
		}
		remaining := s.length-s.bodyAt; available := s.scratchEnd-s.scratchAt
		if remaining > available { remaining = available }
		copy(s.arena[s.bodyAt:s.bodyAt+remaining], s.scratch[s.scratchAt:s.scratchAt+remaining]); s.bodyAt += remaining; s.scratchAt += remaining
		if s.bodyAt != s.length { continue }
		body := s.arena[:s.length]; digest := sha256.Sum256(body)
		for i, b := range digest { if s.header[i+6] != b { return s.refuse() } }
		ref := debianBinaryRawRef{s.header[0], s.header[1], s.length, hex.EncodeToString(digest[:])}
		if ref.Kind == 2 && s.checkPart(body, int(ref.Part)) != nil || ref.Kind == 3 && s.checkSummary(body) != nil { return s.refuse() }
		s.refs[s.frames] = ref; s.frames++; s.generation++; s.ready = true
	}
	return nil
}

func (s *debianBinaryRawState) checkPart(body []byte, ordinal int) error {
	var p assemblyDebianBinaryPartV1
	if decodeAssemblyDebianRaw(body, &p) != nil { return errDebianBinaryRaw }
	e := s.expected; first := ordinal*16; count := 16; if ordinal == 10 { count = 14 }
	if p.Schema != 1 || p.RecordType != "luci-debian-binary-binding-part-v1" || p.OperationID != e.OperationID || p.Part != ordinal || p.FirstLockOrdinal != first || p.SignedIndexes != e.SignedIndexes || p.DependencyLockSHA256 != e.Selection.DependencyLockSHA256 || len(p.Bindings) != count { return errDebianBinaryRaw }
	// Raw scope/tuple consistency only. Opaque bundle paragraphs, span contents,
	// claim equality and full existing coverage validation remain later work.
	for i, b := range p.Bindings {
		want := e.Selection.Packages[first+i]
		if b.LockOrdinal != want.Ordinal || b.Name != want.Name || b.Version != want.Version || b.Architecture != want.Architecture || b.Filename != want.Filename || b.DebObject != (assemblyDebianObjectRef{"deb-package", want.SHA256, want.Bytes}) || len(b.Matches) < 1 || len(b.Matches) > 4 { return errDebianBinaryRaw }
		bundle := s.refs[s.frames-1]
		object := assemblyDebianObjectRef{"binary-binding-bundle", bundle.SHA256, int64(bundle.Bytes)}
		if !debianBinaryRawSpan(b.Control, object) || !debianBinaryRawSpan(b.SelectedFieldClaims, object) { return errDebianBinaryRaw }
		seen := [4]bool{}
		for _, m := range b.Matches {
			index := -1; for k, r := range e.Indexes { if m.CompressedIndex == r { index = k } }
			if index < 0 || seen[index] || !assemblyCatalogHex(m.DecodedIndexSHA256) || m.DecodedIndexBytes < 1 || m.DecodedIndexBytes > 128<<20 || !debianBinaryRawSpan(m.Paragraph, object) || m.Paragraph.Bytes > m.DecodedIndexBytes || m.ParagraphOffset < 0 || m.ParagraphOffset > m.DecodedIndexBytes-m.Paragraph.Bytes || len(m.IndexBindingOrdinals) < 1 || len(m.IndexBindingOrdinals) > 12 { return errDebianBinaryRaw }
			seen[index] = true; previous := -1
			for _, v := range m.IndexBindingOrdinals { if v <= previous || v > 11 { return errDebianBinaryRaw }; previous = v }
		}
	}
	return nil
}

func debianBinaryRawSpan(v assemblyDebianBinarySpanV1, bundle assemblyDebianObjectRef) bool {
	return v.Bundle == bundle && v.Offset >= 0 && v.Bytes > 0 && v.Bytes <= bundle.Bytes && v.Offset <= bundle.Bytes-v.Bytes && assemblyCatalogHex(v.SHA256)
}

func (s *debianBinaryRawState) checkSummary(body []byte) error {
	var p debianBinaryRawSummary
	if decodeAssemblyDebianRaw(body, &p) != nil { return errDebianBinaryRaw }
	e := s.expected
	if p.Schema != 1 || p.State != "complete-observations-not-adopted" || p.OperationID != e.OperationID || p.LoanID != e.LoanID || p.SourceBindingSHA256 != e.SourceBindingSHA256 || p.SelectionSHA256 != e.SelectionSHA256 || p.SignedIndexes != e.SignedIndexes || len(p.RawOutputs) != 22 || len(p.DecodedIndexes) != 4 || p.SeekCalls != 359 || p.PackageCount != 174 || p.PackageBytes != assemblyDebianBinaryBytes || p.SourceArchiveBinding != "not-evaluated" || p.DependencyClosure != "not-evaluated" || p.ExpectedInstalledFiles != "not-derived" || p.ObservedInstalledOutput != "not-observed" || p.ServerEligibility != "not-evaluated-server-owned" { return errDebianBinaryRaw }
	for i, ref := range p.RawOutputs { if ref != s.refs[i] { return errDebianBinaryRaw } }
	expanded := int64(0)
	for i, v := range p.DecodedIndexes {
		if v.Compressed != e.Indexes[i] || !assemblyCatalogHex(v.DecodedSHA256) || v.DecodedBytes < 1 || v.DecodedBytes > 128<<20 || v.DecodedBytes > (256<<20)-expanded { return errDebianBinaryRaw }
		expanded += v.DecodedBytes
	}
	c := p.Counters
	if c.State != "complete-observations-not-adopted" || c.CompressedReadBytes < 2*(assemblyDebianBinaryBytes+assemblyDebianBinaryIndexBytes) || c.CompressedReadBytes > 320<<20 || c.ReadCalls < 359 || c.ReadCalls > 50000 || c.ExpandedBytes < expanded || c.ExpandedBytes > 512<<20 || c.Paragraphs < 174 || c.Paragraphs > 250000 { return errDebianBinaryRaw }
	for i, v := range []debianBinaryRawClassDebit{p.ClassDebits.Index, p.ClassDebits.Small} {
		bytes, calls := int64(19044408), 768; if i == 1 { bytes, calls = 393216, 16 }
		if v.Bytes < 1 || v.Bytes > bytes || v.Calls < 1 || v.Calls > calls { return errDebianBinaryRaw }
	}
	return nil
}

func (d debianBinaryRawDecoder) nextFrame() (debianBinaryRawView, error) {
	if d.state == nil { return debianBinaryRawView{}, errDebianBinaryRaw }
	s := d.state; s.mu.Lock(); defer s.mu.Unlock()
	if s.failed || s.retired || !s.ready { return debianBinaryRawView{}, errDebianBinaryRaw }
	return debianBinaryRawView{s, s.generation}, nil
}
func (v debianBinaryRawView) copyPayload(offset int, dst []byte) (int, error) {
	if v.state == nil { return 0, errDebianBinaryRaw }
	s := v.state; s.mu.Lock(); defer s.mu.Unlock()
	if s.failed || s.retired || !s.ready || s.generation != v.generation || offset < 0 || offset > s.length || len(dst) > debianBinaryRawScratchMax { return 0, errDebianBinaryRaw }
	return copy(dst, s.arena[offset:s.length]), nil
}
func (v debianBinaryRawView) descriptor() (debianBinaryRawRef, error) {
	if v.state == nil { return debianBinaryRawRef{}, errDebianBinaryRaw }
	s := v.state; s.mu.Lock(); defer s.mu.Unlock()
	if s.failed || s.retired || !s.ready || s.generation != v.generation { return debianBinaryRawRef{}, errDebianBinaryRaw }
	return s.refs[s.frames-1], nil
}
func (d debianBinaryRawDecoder) advance(v debianBinaryRawView, at time.Time) error {
	if d.state == nil { return errDebianBinaryRaw }
	s := d.state; s.mu.Lock(); defer s.mu.Unlock()
	if !s.current(at) || v.state != s || v.generation != s.generation || !s.ready { return s.refuse() }
	s.generation++; s.ready = false; s.length = 0; s.bodyAt = 0; s.headerAt = 0
	// Reuse the single exact4MiB arena. Scratch remainder is already copied,
	// and processing it cannot allocate a second current frame backing.
	return s.pump()
}
func (d debianBinaryRawDecoder) retire() {
	if d.state == nil { return }
	s := d.state; s.mu.Lock(); defer s.mu.Unlock()
	s.retired = true; s.generation++; s.ready = false
}
func (d debianBinaryRawDecoder) observation() debianBinaryRawObservation {
	if d.state == nil { return debianBinaryRawObservation{State: "incomplete-raw-observations"} }
	s := d.state; s.mu.Lock(); defer s.mu.Unlock()
	phase := "incomplete-raw-observations"
	if !s.failed && !s.retired && s.pending == nil && !s.ready && s.stdoutEOF && s.stderrEOF && s.frames == 23 { phase = "raw-framing-complete-not-native-or-semantic-proof" }
	return debianBinaryRawObservation{phase, s.frames, s.stdoutDebit, s.stderrDebit, s.calls, s.pending != nil, s.refs}
}
