package api

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/backend"
	"github.com/lcfrai/luci/control-plane/internal/management"
)

func pdfPilotFrameTestLease(role backend.PDFPilotRole) pdfPilotLease {
	request := pdfPilotRequest{
		AttemptID: strings.Repeat("a", 32), ScopeDigest: strings.Repeat("b", 64),
		Actor: management.ExecutionActorRef{ProjectID: "project", Local: true},
		ApprovalRevision: 1, ApprovalDigest: strings.Repeat("5", 64),
		EngineID: "selected-engine", InstanceID: "selected-instance", SourceCommit: strings.Repeat("c", 40),
		ImageID: "sha256:" + strings.Repeat("d", 64), ImageReceiptDigest: strings.Repeat("e", 64),
		PackageReceiptDigest: strings.Repeat("f", 64), FontDigest: strings.Repeat("1", 64),
		FontReceiptDigest: strings.Repeat("2", 64), RendererReceipt: strings.Repeat("3", 64),
		ReaderReceipt: strings.Repeat("4", 64),
	}
	started := time.Date(2026, 10, 16, 0, 0, 0, 0, time.UTC)
	lease := pdfPilotLease{
		Request: request, Deadlines: newPDFPilotDeadlines(started), ChargeBytes: backend.PDFPilotMemoryBytes,
		HostCaptureBytes: backend.PDFPilotHostCaptureBytes, StageDiskBytes: backend.PDFPilotStageDiskBytes,
		SourceBindingDigest: request.ApprovalDigest,
		Renderer: pdfPilotChildLease{AllocationID: backend.PDFPilotAllocationID(request.ScopeDigest, request.AttemptID, backend.PDFPilotRenderer)},
		Reader: pdfPilotChildLease{AllocationID: backend.PDFPilotAllocationID(request.ScopeDigest, request.AttemptID, backend.PDFPilotReader), State: "reserved"},
	}
	identity := backend.PDFPilotIdentity{
		Protocol: backend.PDFPilotProtocol, Role: backend.PDFPilotRenderer, ScopeDigest: request.ScopeDigest,
		AttemptID: request.AttemptID, AllocationID: lease.Renderer.AllocationID,
		ActorDigest: pdfPilotActorDigest(request.Actor), ApprovalDigest: request.ApprovalDigest,
		EngineID: request.EngineID, InstanceID: request.InstanceID, SourceCommit: request.SourceCommit,
		ImageID: request.ImageID, ImageReceiptDigest: request.ImageReceiptDigest,
		BinaryReceiptDigest: request.RendererReceipt, PackageReceiptDigest: request.PackageReceiptDigest,
		OriginalFontDigest: request.FontDigest, OriginalFontReceipt: request.FontReceiptDigest,
		OutputStageDigest: strings.Repeat("6", 64), CommandDigest: strings.Repeat("7", 64),
	}
	lease.Renderer.Identity, lease.Renderer.Ref, lease.Renderer.State = &identity, strings.Repeat("8", 64), "running"
	lease.Renderer.StartIntentAt = started.Add(2 * time.Second)
	if role == backend.PDFPilotReader {
		lease.Renderer.State = "retired"
		lease.Renderer.ExitObservedAt = started.Add(20 * time.Second)
		lease.Renderer.CaptureAt = started.Add(25 * time.Second)
		lease.Renderer.RetiredAt = started.Add(30 * time.Second)
		lease.Renderer.RetirementDigest = strings.Repeat("c", 64)
		lease.RendererOutputDigest = strings.Repeat("9", 64)
		lease.RendererOutputBytes = 1024
		lease.RendererFileIdentity = strings.Repeat("a", 64)
		reader := identity
		reader.Role, reader.AllocationID = backend.PDFPilotReader, lease.Reader.AllocationID
		reader.BinaryReceiptDigest, reader.OutputStageDigest = request.ReaderReceipt, strings.Repeat("b", 64)
		reader.RendererOutputDigest = lease.RendererOutputDigest
		lease.Reader.Identity, lease.Reader.Ref, lease.Reader.State = &reader, strings.Repeat("d", 64), "running"
		lease.Reader.StartIntentAt = started.Add(45 * time.Second)
	}
	return lease
}

type pdfPilotTestRecord struct {
	kind byte
	payload []byte
}

func pdfPilotTestLogicalFrame(lease pdfPilotLease, role backend.PDFPilotRole, records []pdfPilotTestRecord) []byte {
	var frame bytes.Buffer
	frame.WriteString(pdfPilotFrameStart)
	if role == backend.PDFPilotRenderer {
		frame.WriteByte('R')
	} else {
		frame.WriteByte('D')
	}
	allocation, _ := hex.DecodeString(backend.PDFPilotAllocationID(lease.Request.ScopeDigest, lease.Request.AttemptID, role))
	frame.Write(allocation)
	input := lease.Request.FontDigest
	page := byte(0)
	if role == backend.PDFPilotReader {
		input, page = lease.RendererOutputDigest, 1
	}
	inputBytes, _ := hex.DecodeString(input)
	frame.Write(inputBytes)
	frame.Write([]byte{page, byte(len(records))})
	for _, record := range records {
		var header [1 + 4 + 32]byte
		header[0] = record.kind
		binary.BigEndian.PutUint32(header[1:5], uint32(len(record.payload)))
		hash := sha256.Sum256(record.payload)
		copy(header[5:], hash[:])
		frame.Write(header[:])
		frame.Write(record.payload)
	}
	frame.WriteString(pdfPilotFrameEnd)
	return frame.Bytes()
}

func pdfPilotTestMux(logical []byte, frameBytes int) []byte {
	var raw bytes.Buffer
	for len(logical) > 0 {
		count := min(len(logical), frameBytes)
		var header [8]byte
		header[0] = 1
		binary.BigEndian.PutUint32(header[4:], uint32(count))
		raw.Write(header[:])
		raw.Write(logical[:count])
		logical = logical[count:]
	}
	return raw.Bytes()
}

type pdfPilotTestReader struct {
	data []byte
	limit int
	calls int
	onRead func()
	endError error
}

func (r *pdfPilotTestReader) Read(p []byte) (int, error) {
	r.calls++
	if r.onRead != nil {
		r.onRead()
		r.onRead = nil
	}
	if len(r.data) == 0 {
		if r.endError != nil {
			return 0, r.endError
		}
		return 0, io.EOF
	}
	n := min(len(p), len(r.data), r.limit)
	copy(p, r.data[:n])
	r.data = r.data[n:]
	if len(r.data) == 0 && r.endError == io.EOF {
		return n, io.EOF
	}
	return n, nil
}

func TestPDFPilotPrivateFrameFragmentedRendererBindsImmutableLease(t *testing.T) {
	lease := pdfPilotFrameTestLease(backend.PDFPilotRenderer)
	if !validPDFPilotLease(lease) {
		t.Fatal("test lease must have the complete source-bound shape")
	}
	pdf := []byte("not yet semantically checked PDF bytes")
	logical := pdfPilotTestLogicalFrame(lease, backend.PDFPilotRenderer, []pdfPilotTestRecord{{'P', pdf}})
	reader := &pdfPilotTestReader{data: pdfPilotTestMux(logical, 5), limit: 1}
	reader.onRead = func() {
		lease.Request.FontDigest = strings.Repeat("0", 64)
		lease.Renderer.AllocationID = strings.Repeat("0", 64)
	}
	candidate, err := readPDFPilotFrameCandidate(context.Background(), reader, lease, backend.PDFPilotRenderer)
	if err != nil || candidate.Role != backend.PDFPilotRenderer || candidate.ScopeDigest != strings.Repeat("b", 64) ||
		candidate.AttemptID != strings.Repeat("a", 32) || candidate.Ref != strings.Repeat("8", 64) ||
		candidate.InputDigest != strings.Repeat("1", 64) || candidate.AllocationID == lease.Renderer.AllocationID ||
		!bytes.Equal(candidate.PDF, pdf) || len(candidate.Text) != 0 || len(candidate.PNG) != 0 || reader.calls < 100 {
		t.Fatalf("fragmented bound candidate failed: err=%v calls=%d", err, reader.calls)
	}
	got := sha256.Sum256(pdf)
	if candidate.PDFDigest != hex.EncodeToString(got[:]) {
		t.Fatal("renderer candidate digest differs from retained exact payload")
	}
}

func TestPDFPilotPrivateFrameReaderRecordsAreCandidatesOnly(t *testing.T) {
	lease := pdfPilotFrameTestLease(backend.PDFPilotReader)
	if !validPDFPilotLease(lease) {
		t.Fatal("serial reader test lease must be valid")
	}
	text, png := []byte("unverified text"), []byte("unverified PNG")
	logical := pdfPilotTestLogicalFrame(lease, backend.PDFPilotReader, []pdfPilotTestRecord{{'T', text}, {'N', png}})
	candidate, err := readPDFPilotFrameCandidate(context.Background(), bytes.NewReader(pdfPilotTestMux(logical, len(logical))), lease, backend.PDFPilotReader)
	if err != nil || candidate.PageCount != 1 || !bytes.Equal(candidate.Text, text) || !bytes.Equal(candidate.PNG, png) || len(candidate.PDF) != 0 {
		t.Fatalf("reader candidate framing failed: %v", err)
	}
	for _, value := range []struct{ payload []byte; digest string }{{text, candidate.TextDigest}, {png, candidate.PNGDigest}} {
		sum := sha256.Sum256(value.payload)
		if value.digest != hex.EncodeToString(sum[:]) {
			t.Fatal("reader candidate digest differs from retained payload")
		}
	}
}

func TestPDFPilotPrivateFrameRefusesAlteredAndIncompleteWire(t *testing.T) {
	lease := pdfPilotFrameTestLease(backend.PDFPilotRenderer)
	base := pdfPilotTestLogicalFrame(lease, backend.PDFPilotRenderer, []pdfPilotTestRecord{{'P', []byte("PDF")}})
	const fixedOffset = len(pdfPilotFrameStart)
	const recordOffset = fixedOffset + 1 + 32 + 32 + 2
	mutate := func(change func([]byte)) []byte {
		changed := bytes.Clone(base)
		change(changed)
		return pdfPilotTestMux(changed, len(changed))
	}
	cases := []struct { name string; raw []byte }{
		{"role", mutate(func(v []byte) { v[fixedOffset] = 'D' })},
		{"allocation", mutate(func(v []byte) { v[fixedOffset+1] ^= 1 })},
		{"input", mutate(func(v []byte) { v[fixedOffset+33] ^= 1 })},
		{"page", mutate(func(v []byte) { v[fixedOffset+65] = 1 })},
		{"count", mutate(func(v []byte) { v[fixedOffset+66] = 2 })},
		{"record-order", mutate(func(v []byte) { v[recordOffset] = 'N' })},
		{"hash", mutate(func(v []byte) { v[recordOffset+5] ^= 1 })},
		{"invalid-length", mutate(func(v []byte) { binary.BigEndian.PutUint32(v[recordOffset+1:], ^uint32(0)) })},
		{"end", mutate(func(v []byte) { v[len(v)-2] ^= 1 })},
		{"trailing", pdfPilotTestMux(append(bytes.Clone(base), 'X'), len(base)+1)},
		{"truncated-payload", pdfPilotTestMux(base[:recordOffset+37+2], len(base))},
		{"truncated-docker-header", []byte{1, 0, 0}},
	}
	stderr := pdfPilotTestMux(base, len(base))
	stderr[0] = 2
	cases = append(cases, struct { name string; raw []byte }{"stderr", stderr})
	unknown := pdfPilotTestMux(base, len(base))
	unknown[0] = 3
	cases = append(cases, struct { name string; raw []byte }{"unknown-stream", unknown})
	reserved := pdfPilotTestMux(base, len(base))
	reserved[2] = 1
	cases = append(cases, struct { name string; raw []byte }{"docker-reserved", reserved})
	oversize := pdfPilotTestMux(base, len(base))
	binary.BigEndian.PutUint32(oversize[4:], ^uint32(0))
	cases = append(cases, struct { name string; raw []byte }{"docker-declared-length", oversize})
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			candidate, err := readPDFPilotFrameCandidate(context.Background(), bytes.NewReader(tc.raw), lease, backend.PDFPilotRenderer)
			if err == nil || candidate.Role != "" || candidate.ScopeDigest != "" || candidate.AttemptID != "" ||
				candidate.AllocationID != "" || candidate.Ref != "" ||
				candidate.PDF != nil || candidate.Text != nil || candidate.PNG != nil {
				t.Fatalf("malformed wire produced a candidate: %v", err)
			}
		})
	}
}

func TestPDFPilotPrivateFrameBudgetAndFailureAreSticky(t *testing.T) {
	underlying := &pdfPilotTestReader{data: []byte("data"), limit: 4}
	raw := &pdfPilotRawReader{ctx: context.Background(), reader: underlying, used: pdfPilotRawMaximum - 1}
	if _, err := raw.read(make([]byte, 2)); err == nil || underlying.calls != 0 || !raw.failed {
		t.Fatal("raw upper bound was not reserved before underlying IO")
	}
	if _, err := raw.read(make([]byte, 1)); err == nil || underlying.calls != 0 {
		t.Fatal("failed raw reader resumed IO")
	}
	raw = &pdfPilotRawReader{ctx: context.Background(), reader: underlying, calls: pdfPilotCallMaximum}
	if _, err := raw.read(make([]byte, 1)); err == nil || underlying.calls != 0 {
		t.Fatal("call budget permitted another underlying IO")
	}
	noProgress := &pdfPilotTestReader{limit: 0, data: []byte("x")}
	raw = &pdfPilotRawReader{ctx: context.Background(), reader: noProgress}
	if _, err := raw.read(make([]byte, 1)); err == nil || raw.calls != 1 || raw.used != 1 || !raw.failed {
		t.Fatal("zero-progress call was not charged and refused")
	}
	failed := &pdfPilotTestReader{limit: 1, endError: errors.New("transport error")}
	raw = &pdfPilotRawReader{ctx: context.Background(), reader: failed}
	if _, err := raw.read(make([]byte, 1)); err == nil || raw.calls != 1 || raw.used != 1 || !raw.failed {
		t.Fatal("failed underlying read was not charged and retained")
	}
}

func TestPDFPilotPrivateFrameEOFAndCancellation(t *testing.T) {
	lease := pdfPilotFrameTestLease(backend.PDFPilotRenderer)
	logical := pdfPilotTestLogicalFrame(lease, backend.PDFPilotRenderer, []pdfPilotTestRecord{{'P', []byte("PDF")}})
	rawBytes := pdfPilotTestMux(logical, len(logical))
	withEOF := &pdfPilotTestReader{data: rawBytes, limit: len(rawBytes), endError: io.EOF}
	if _, err := readPDFPilotFrameCandidate(context.Background(), withEOF, lease, backend.PDFPilotRenderer); err != nil {
		t.Fatalf("exact bytes with simultaneous EOF refused: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	before := &pdfPilotTestReader{data: rawBytes, limit: 1}
	if _, err := readPDFPilotFrameCandidate(ctx, before, lease, backend.PDFPilotRenderer); err == nil || before.calls != 0 {
		t.Fatal("cancelled decode reached the underlying reader")
	}
	ctx, cancel = context.WithCancel(context.Background())
	after := &pdfPilotTestReader{data: rawBytes, limit: 1, onRead: cancel}
	if _, err := readPDFPilotFrameCandidate(ctx, after, lease, backend.PDFPilotRenderer); err == nil || after.calls != 1 {
		t.Fatal("cancel after a partial read did not refuse further IO")
	}
	invalid := lease
	invalid.Renderer.AllocationID = strings.Repeat("0", 64)
	before = &pdfPilotTestReader{data: rawBytes, limit: 1}
	if _, err := readPDFPilotFrameCandidate(context.Background(), before, invalid, backend.PDFPilotRenderer); err == nil || before.calls != 0 {
		t.Fatal("invalid expected lease reached the underlying reader")
	}
}
