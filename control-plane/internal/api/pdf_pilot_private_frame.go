package api

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"io"

	"github.com/lcfrai/luci/control-plane/internal/backend"
)

// This is a pure, bounded capture candidate decoder. It neither owns the
// selected attach nor confirms child exit, durable host staging, current
// authority, semantic PDF/text/PNG validity or exact-CID retirement. A context
// check cannot interrupt a blocked Reader.Read: its selected attach owner must
// close the connection independently. No goroutine is started here.
const (
	pdfPilotFrameStart = "LUCI-PDF-PILOT/1\n"
	pdfPilotFrameEnd = "LUCI-PDF-PILOT-END/1\n"
	pdfPilotRawMaximum = 34<<20 + 16<<10
	pdfPilotReadMaximum = 64 << 10
	pdfPilotCallMaximum = 65536
)

var errPDFPilotFrame = errors.New("pdf pilot frame refused")

type pdfPilotFrameExpected struct {
	role         backend.PDFPilotRole
	scopeDigest  string
	attemptID    string
	allocationID string
	ref          string
	inputDigest  string
}

// Copy every expected scalar before the first underlying Read. The complete
// lease check is shape/chronology validation, not source or actor authority.
func pdfPilotExpectedFrame(lease pdfPilotLease, role backend.PDFPilotRole) (pdfPilotFrameExpected, error) {
	if !validPDFPilotLease(lease) {
		return pdfPilotFrameExpected{}, errPDFPilotFrame
	}
	request := lease.Request
	var child pdfPilotChildLease
	var input string
	switch role {
	case backend.PDFPilotRenderer:
		child, input = lease.Renderer, request.FontDigest
	case backend.PDFPilotReader:
		if lease.Renderer.State != "retired" || !checkHex(lease.RendererOutputDigest, 32) {
			return pdfPilotFrameExpected{}, errPDFPilotFrame
		}
		child, input = lease.Reader, lease.RendererOutputDigest
	default:
		return pdfPilotFrameExpected{}, errPDFPilotFrame
	}
	allocation := backend.PDFPilotAllocationID(request.ScopeDigest, request.AttemptID, role)
	if allocation == "" || child.AllocationID != allocation || child.Identity == nil || child.Identity.AllocationID != allocation ||
		!checkHex(child.Ref, 32) || !checkHex(input, 32) {
		return pdfPilotFrameExpected{}, errPDFPilotFrame
	}
	return pdfPilotFrameExpected{role: role, scopeDigest: request.ScopeDigest, attemptID: request.AttemptID,
		allocationID: allocation, ref: child.Ref, inputDigest: input}, nil
}

type pdfPilotFrameCandidate struct {
	Role         backend.PDFPilotRole
	ScopeDigest  string
	AttemptID    string
	AllocationID string
	Ref          string
	InputDigest  string
	PageCount    byte
	PDF          []byte
	Text         []byte
	PNG          []byte
	PDFDigest    string
	TextDigest   string
	PNGDigest    string
}

// The raw allowance includes all Docker 8-byte headers and data, plus one
// charged EOF probe. Each requested upper bound is reserved before Read and
// reconciled only after a valid short read. Failed/EOF/no-progress calls count.
type pdfPilotRawReader struct {
	ctx    context.Context
	reader io.Reader
	used   uint64
	calls  uint32
	eof    bool
	failed bool
}

func (r *pdfPilotRawReader) refuse() error {
	r.failed = true
	return errPDFPilotFrame
}

func (r *pdfPilotRawReader) read(p []byte) (int, error) {
	if r.failed || r.ctx == nil || r.reader == nil || r.ctx.Err() != nil || len(p) == 0 || len(p) > pdfPilotReadMaximum {
		return 0, r.refuse()
	}
	if r.eof {
		return 0, io.EOF
	}
	if r.calls >= pdfPilotCallMaximum || uint64(len(p)) > pdfPilotRawMaximum-r.used {
		return 0, r.refuse()
	}
	r.calls++
	r.used += uint64(len(p))
	n, err := r.reader.Read(p)
	if n < 0 || n > len(p) {
		return 0, r.refuse()
	}
	charge := n
	if charge == 0 {
		charge = 1
	}
	r.used -= uint64(len(p) - charge)
	if err == io.EOF {
		r.eof = true
	} else if err != nil || n == 0 {
		return 0, r.refuse()
	}
	return n, err
}

func (r *pdfPilotRawReader) exact(p []byte) error {
	for len(p) > 0 {
		chunk := min(len(p), pdfPilotReadMaximum)
		n, err := r.read(p[:chunk])
		if n > 0 {
			p = p[n:]
		}
		if (err != nil && err != io.EOF) || n == 0 {
			return r.refuse()
		}
	}
	return nil
}

func (r *pdfPilotRawReader) expectEOF() error {
	if r.failed || r.ctx == nil || r.ctx.Err() != nil {
		return r.refuse()
	}
	if r.eof {
		return nil
	}
	var probe [1]byte
	n, err := r.read(probe[:])
	if n == 0 && err == io.EOF {
		return nil
	}
	return r.refuse()
}

// Logical bytes may span any number of bounded stdout Docker frames. Header
// parsing never allocates from an untrusted declared frame length.
type pdfPilotLogicalReader struct {
	raw       *pdfPilotRawReader
	remaining uint64
}

func (r *pdfPilotLogicalReader) exact(p []byte) error {
	for len(p) > 0 {
		if r.remaining == 0 {
			var header [8]byte
			if err := r.raw.exact(header[:]); err != nil {
				return err
			}
			length := uint64(binary.BigEndian.Uint32(header[4:]))
			if header[0] != 1 || header[1] != 0 || header[2] != 0 || header[3] != 0 ||
				length == 0 || length > pdfPilotRawMaximum-r.raw.used {
				return r.raw.refuse()
			}
			r.remaining = length
		}
		chunk := min(len(p), pdfPilotReadMaximum, int(r.remaining))
		if err := r.raw.exact(p[:chunk]); err != nil {
			return err
		}
		r.remaining -= uint64(chunk)
		p = p[chunk:]
	}
	return nil
}

func (r *pdfPilotLogicalReader) expectEOF() error {
	if r.remaining != 0 {
		return r.raw.refuse()
	}
	return r.raw.expectEOF()
}

func pdfPilotRecordLimit(role backend.PDFPilotRole, index int, kind byte) int {
	if role == backend.PDFPilotRenderer && index == 0 && kind == 'P' {
		return backend.PDFPilotMaxPDFBytes
	}
	if role == backend.PDFPilotReader && index == 0 && kind == 'T' {
		return backend.PDFPilotMaxReaderTextBytes
	}
	if role == backend.PDFPilotReader && index == 1 && kind == 'N' {
		return backend.PDFPilotMaxReaderPNGBytes
	}
	return 0
}

// readPDFPilotFrameCandidate consumes one complete frame and transport EOF.
// Retained bytes consist only of the role's exact payloads; all other parsing
// uses <=64 KiB per underlying Read and fixed small headers. An error returns
// no candidate even when a partial payload was read.
func readPDFPilotFrameCandidate(ctx context.Context, source io.Reader, lease pdfPilotLease, role backend.PDFPilotRole) (pdfPilotFrameCandidate, error) {
	expected, err := pdfPilotExpectedFrame(lease, role)
	if err != nil || ctx == nil || source == nil {
		return pdfPilotFrameCandidate{}, errPDFPilotFrame
	}
	raw := &pdfPilotRawReader{ctx: ctx, reader: source}
	logical := &pdfPilotLogicalReader{raw: raw}
	var magic [len(pdfPilotFrameStart)]byte
	if logical.exact(magic[:]) != nil || string(magic[:]) != pdfPilotFrameStart {
		return pdfPilotFrameCandidate{}, raw.refuse()
	}
	var fixed [1 + 32 + 32 + 2]byte
	if logical.exact(fixed[:]) != nil {
		return pdfPilotFrameCandidate{}, raw.refuse()
	}
	wireRole, page, count := byte('R'), byte(0), byte(1)
	if role == backend.PDFPilotReader {
		wireRole, page, count = 'D', 1, 2
	}
	allocation, _ := hex.DecodeString(expected.allocationID)
	input, _ := hex.DecodeString(expected.inputDigest)
	if fixed[0] != wireRole || subtle.ConstantTimeCompare(fixed[1:33], allocation) != 1 ||
		subtle.ConstantTimeCompare(fixed[33:65], input) != 1 || fixed[65] != page || fixed[66] != count {
		return pdfPilotFrameCandidate{}, raw.refuse()
	}
	candidate := pdfPilotFrameCandidate{Role: role, ScopeDigest: expected.scopeDigest, AttemptID: expected.attemptID,
		AllocationID: expected.allocationID, Ref: expected.ref, InputDigest: expected.inputDigest, PageCount: page}
	retained, maximum := 0, backend.PDFPilotMaxPDFBytes
	if role == backend.PDFPilotReader {
		maximum = backend.PDFPilotMaxReaderTextBytes + backend.PDFPilotMaxReaderPNGBytes
	}
	for index := 0; index < int(count); index++ {
		var header [1 + 4 + 32]byte
		if logical.exact(header[:]) != nil {
			return pdfPilotFrameCandidate{}, raw.refuse()
		}
		limit := pdfPilotRecordLimit(role, index, header[0])
		length := uint64(binary.BigEndian.Uint32(header[1:5]))
		if limit == 0 || length == 0 || length > uint64(limit) || length > uint64(maximum-retained) || length > uint64(pdfPilotRawMaximum)-raw.used {
			return pdfPilotFrameCandidate{}, raw.refuse()
		}
		payload := make([]byte, int(length))
		digest := sha256.New()
		for offset := 0; offset < len(payload); {
			end := min(offset+pdfPilotReadMaximum, len(payload))
			if logical.exact(payload[offset:end]) != nil {
				return pdfPilotFrameCandidate{}, raw.refuse()
			}
			_, _ = digest.Write(payload[offset:end])
			offset = end
		}
		actual := digest.Sum(nil)
		if subtle.ConstantTimeCompare(header[5:], actual) != 1 {
			return pdfPilotFrameCandidate{}, raw.refuse()
		}
		retained += len(payload)
		switch header[0] {
		case 'P':
			candidate.PDF, candidate.PDFDigest = payload, hex.EncodeToString(actual)
		case 'T':
			candidate.Text, candidate.TextDigest = payload, hex.EncodeToString(actual)
		case 'N':
			candidate.PNG, candidate.PNGDigest = payload, hex.EncodeToString(actual)
		}
	}
	var end [len(pdfPilotFrameEnd)]byte
	if logical.exact(end[:]) != nil || !bytes.Equal(end[:], []byte(pdfPilotFrameEnd)) || logical.expectEOF() != nil {
		return pdfPilotFrameCandidate{}, raw.refuse()
	}
	return candidate, nil
}
