package api

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"
)

// Tiny synthetic raw wire and tuple metadata only. The one-byte opaque bundles
// deliberately cannot pass semantic bundle/coverage validation. No original
// package preimages, files, native clocks/handles, completion or adoption.
func binaryRawExpectedFixture() (debianBinaryRawExpected, time.Time) {
	e := debianBinaryRawExpected{OperationID: "raw-operation", LoanID: "raw-loan", SourceBindingSHA256: strings.Repeat("b", 64), SelectionSHA256: strings.Repeat("c", 64), SignedIndexes: assemblyCatalogRef{Schema: 1, Kind: "signed-index-bindings", Digest: strings.Repeat("d", 64), Bytes: 128}}
	e.Selection = assemblyDebianBinarySelectionV1{Schema: 1, DependencyLockSHA256: assemblyDebianLockSHA, AcquisitionManifestSHA256: assemblyDebianManifestSHA, CandidatePolicySHA256: assemblyDebianPolicySourceSHA}
	remaining := assemblyDebianBinaryBytes
	for i := 0; i < 174; i++ {
		n := assemblyDebianBinaryBytes/174; if i == 173 { n = remaining }; remaining -= n
		e.Selection.Packages = append(e.Selection.Packages, assemblyDebianBinaryExpectedTupleV1{Ordinal: i, Name: fmt.Sprintf("pkg-%03d", i), Version: "1", Architecture: "amd64", Filename: fmt.Sprintf("pool/test/pkg-%03d.deb", i), Bytes: n, SHA256: strings.Repeat("e", 64), AllowedCandidateIDs: []string{"current/bookworm"}})
	}
	remaining = assemblyDebianBinaryIndexBytes
	for i := range e.Indexes {
		n := assemblyDebianBinaryIndexBytes/4; if i == 3 { n = remaining }; remaining -= n
		e.Indexes[i] = assemblyDebianObjectRef{"xz-index", fmt.Sprintf("%064x", i+1), n}
	}
	return e, time.Unix(1700000000, 0).UTC()
}
func binaryRawDecoderFixture(t *testing.T) (debianBinaryRawDecoder, debianBinaryRawExpected, time.Time) {
	t.Helper(); e, now := binaryRawExpectedFixture()
	d, err := newDebianBinaryRawDecoder(e, now, now.Add(300*time.Second)); if err != nil { t.Fatal(err) }
	return d, e, now
}
func binaryRawTestFrame(kind, part uint8, body []byte) ([]byte, debianBinaryRawRef) {
	header := make([]byte, 38); header[0], header[1] = kind, part
	binary.LittleEndian.PutUint32(header[2:6], uint32(len(body))); digest := sha256.Sum256(body); copy(header[6:], digest[:])
	return append(header, body...), debianBinaryRawRef{kind, part, len(body), hex.EncodeToString(digest[:])}
}
func binaryRawTestWire(t *testing.T, e debianBinaryRawExpected, changePart func(*assemblyDebianBinaryPartV1), changeSummary func(*debianBinaryRawSummary)) []byte {
	t.Helper(); var wire bytes.Buffer; wire.WriteString(debianBinaryRawMagic); refs := []debianBinaryRawRef{}
	for part := 0; part < 11; part++ {
		bundle := []byte{byte(part+1)}; frame, ref := binaryRawTestFrame(1, uint8(part), bundle); wire.Write(frame); refs = append(refs, ref)
		object := assemblyDebianObjectRef{"binary-binding-bundle", ref.SHA256, int64(ref.Bytes)}
		span := assemblyDebianBinarySpanV1{Bundle: object, Offset: 0, Bytes: 1, SHA256: ref.SHA256}
		p := assemblyDebianBinaryPartV1{Schema: 1, RecordType: "luci-debian-binary-binding-part-v1", OperationID: e.OperationID, Part: part, SignedIndexes: e.SignedIndexes, DependencyLockSHA256: e.Selection.DependencyLockSHA256, FirstLockOrdinal: part*16}
		count := 16; if part == 10 { count = 14 }
		for i := 0; i < count; i++ {
			r := e.Selection.Packages[part*16+i]
			p.Bindings = append(p.Bindings, assemblyDebianBinaryBindingV1{LockOrdinal: r.Ordinal, Name: r.Name, Version: r.Version, Architecture: r.Architecture, Filename: r.Filename, DebObject: assemblyDebianObjectRef{"deb-package", r.SHA256, r.Bytes}, Matches: []assemblyDebianBinaryMatchV1{{IndexBindingOrdinals: []int{0}, CompressedIndex: e.Indexes[0], DecodedIndexSHA256: strings.Repeat("f", 64), DecodedIndexBytes: 1, ParagraphOffset: 0, Paragraph: span}}, Control: span, SelectedFieldClaims: span})
		}
		if changePart != nil { changePart(&p) }
		raw, err := json.Marshal(p); if err != nil { t.Fatal(err) }; frame, ref = binaryRawTestFrame(2, uint8(part), raw); wire.Write(frame); refs = append(refs, ref)
	}
	p := debianBinaryRawSummary{Schema: 1, State: "complete-observations-not-adopted", OperationID: e.OperationID, LoanID: e.LoanID, SourceBindingSHA256: e.SourceBindingSHA256, SelectionSHA256: e.SelectionSHA256, SignedIndexes: e.SignedIndexes, RawOutputs: refs, Counters: debianBinaryRawCounters{State: "complete-observations-not-adopted", CompressedReadBytes: 2*(assemblyDebianBinaryBytes+assemblyDebianBinaryIndexBytes)+400000, ReadCalls: 1000, ExpandedBytes: 1024, Paragraphs: 200}, SeekCalls: 359, PackageCount: 174, PackageBytes: assemblyDebianBinaryBytes, SourceArchiveBinding: "not-evaluated", DependencyClosure: "not-evaluated", ExpectedInstalledFiles: "not-derived", ObservedInstalledOutput: "not-observed", ServerEligibility: "not-evaluated-server-owned"}
	for _, r := range e.Indexes { p.DecodedIndexes = append(p.DecodedIndexes, assemblyDebianBinaryDecodedIndexV1{Compressed: r, DecodedSHA256: strings.Repeat("f", 64), DecodedBytes: 1}) }
	p.ClassDebits.Index = debianBinaryRawClassDebit{19000000, 700}; p.ClassDebits.Small = debianBinaryRawClassDebit{300000, 8}
	if changeSummary != nil { changeSummary(&p) }
	raw, err := json.Marshal(p); if err != nil { t.Fatal(err) }; frame, _ := binaryRawTestFrame(3, 255, raw); wire.Write(frame)
	return wire.Bytes()
}
func binaryRawDrain(d debianBinaryRawDecoder, now time.Time) error {
	for {
		view, err := d.nextFrame(); if err != nil { return nil }
		if _, err = view.descriptor(); err != nil { return err }
		if err = d.advance(view, now); err != nil { return err }
	}
}
func binaryRawFeed(d debianBinaryRawDecoder, data []byte, chunk int, now time.Time) error {
	for len(data) > 0 {
		if err := binaryRawDrain(d, now); err != nil { return err }
		n := chunk; if n > len(data) { n = len(data) }
		r, err := d.beginRead(debianBinaryRawStdout, n, now); if err != nil { return err }
		if err = d.observeRead(r, data[:n], nil, now); err != nil { return err }; data = data[n:]
	}
	return binaryRawDrain(d, now)
}
func binaryRawEOF(d debianBinaryRawDecoder, stream debianBinaryRawStream, now time.Time) error {
	r, err := d.beginRead(stream, 1, now); if err != nil { return err }
	return d.observeRead(r, nil, io.EOF, now)
}

func TestDebianBinaryRawFragmentedSequenceIsOnlyRawObservation(t *testing.T) {
	d, e, now := binaryRawDecoderFixture(t); raw := binaryRawTestWire(t, e, nil, nil)
	if len(raw) > 256<<10 { t.Fatal("fixture ceased to be tiny", len(raw)) }
	if err := binaryRawFeed(d, raw, 47, now); err != nil { t.Fatal(err) }
	if d.observation().State != "incomplete-raw-observations" { t.Fatal("missing EOF became complete") }
	if binaryRawEOF(d, debianBinaryRawStdout, now) != nil || binaryRawEOF(d, debianBinaryRawStderr, now) != nil { t.Fatal("exact EOF observation refused") }
	got := d.observation()
	if got.State != "raw-framing-complete-not-native-or-semantic-proof" || got.Frames != 23 || got.StdoutDebit != len(raw)+1 || got.StderrDebit != 1 || got.Pending { t.Fatal(got) }
	// One-byte bundles are framing-consistent synthetic data, NOT semantic
	// records/preimages. The later existing bundle validator must reject them.
	if _, err := decodeAssemblyDebianBinaryBundle([]byte{1}, assemblyDebianObjectRef{"binary-binding-bundle", got.Refs[0].SHA256, 1}, 0); err == nil { t.Fatal("tiny opaque fixture accidentally became semantic evidence") }
}

func TestDebianBinaryRawBorrowCopyExpiresAcrossSharedAdvanceAndRetire(t *testing.T) {
	d, _, now := binaryRawDecoderFixture(t); frame, _ := binaryRawTestFrame(1, 0, []byte{7})
	input := append([]byte(debianBinaryRawMagic), frame...); r, err := d.beginRead(debianBinaryRawStdout, len(input), now); if err != nil { t.Fatal(err) }
	if d.observeRead(r, input, nil, now) != nil { t.Fatal("frame refused") }; input[len(input)-1] = 99
	v, err := d.nextFrame(); if err != nil { t.Fatal(err) }; dst := []byte{0}
	if n, err := v.copyPayload(0, dst); err != nil || n != 1 || dst[0] != 7 { t.Fatal("input alias escaped", n, err, dst) }
	dst[0] = 55; if _, err := v.copyPayload(0, dst); err != nil || dst[0] != 7 { t.Fatal("output copy modified arena") }
	copyDecoder := d; if copyDecoder.advance(v, now) != nil { t.Fatal("shared advance failed") }
	if _, err := v.copyPayload(0, dst); err == nil { t.Fatal("expired payload view usable") }
	if _, err := v.descriptor(); err == nil { t.Fatal("expired descriptor usable") }
	copyDecoder.retire(); if _, err := d.beginRead(debianBinaryRawStderr, 1, now); err == nil { t.Fatal("decoder copy reset retirement") }
}

func TestDebianBinaryRawPartialErrorsAndWrongTicketsKeepFullDebit(t *testing.T) {
	for _, kind := range []string{"partial-error", "partial-eof", "failed-empty", "empty-success", "foreign-ticket", "oversized-observation", "late-return", "backward-capture"} {
		t.Run(kind, func(t *testing.T) {
			d, _, now := binaryRawDecoderFixture(t); began := now; if kind == "backward-capture" { began = now.Add(time.Second) }
			r, err := d.beginRead(debianBinaryRawStdout, 8, began); if err != nil { t.Fatal(err) }
			data, readErr, at := []byte("L"), error(nil), now
			switch kind {
			case "partial-error": readErr = errors.New("synthetic read error")
			case "partial-eof": readErr = io.EOF
			case "failed-empty": data = nil; readErr = errors.New("synthetic read error")
			case "empty-success": data = nil
			case "foreign-ticket": r.serial++
			case "oversized-observation": data = []byte("123456789")
			case "late-return": at = now.Add(300*time.Second)
			}
			if d.observeRead(r, data, readErr, at) == nil { t.Fatal("uncertain read accepted") }
			copyDecoder := d; got := copyDecoder.observation()
			if got.StdoutDebit != 8 || got.ReadAttempts != 1 || !got.Pending || got.State != "incomplete-raw-observations" { t.Fatal("uncertain debit refunded", got) }
			if _, err := copyDecoder.beginRead(debianBinaryRawStderr, 1, now); err == nil { t.Fatal("copied quota escaped sticky failure") }
		})
	}
}

func TestDebianBinaryRawCombinedReadBudgetAndPendingBoundary(t *testing.T) {
	d, _, now := binaryRawDecoderFixture(t); r, err := d.beginRead(debianBinaryRawStderr, 4, now); if err != nil { t.Fatal(err) }
	if d.observeRead(r, []byte{'x'}, nil, now) != nil || d.observation().StderrDebit != 1 { t.Fatal("short stderr debit") }
	r, err = d.beginRead(debianBinaryRawStdout, 4, now); if err != nil { t.Fatal(err) }
	if _, err := d.beginRead(debianBinaryRawStderr, 1, now); err == nil { t.Fatal("parallel pending read admitted") }
	if d.observation().StdoutDebit != 4 || d.observation().ReadAttempts != 2 { t.Fatal("pending boundary refunded") }
	d, _, now = binaryRawDecoderFixture(t); d.state.calls = debianBinaryRawCallsMax-1
	r, err = d.beginRead(debianBinaryRawStdout, 1, now); if err != nil || d.observeRead(r, []byte{'L'}, nil, now) != nil { t.Fatal("last read attempt refused") }
	if _, err := d.beginRead(debianBinaryRawStderr, 1, now); err == nil || d.observation().ReadAttempts != debianBinaryRawCallsMax { t.Fatal("stderr reset combined quota") }
	for _, stream := range []debianBinaryRawStream{debianBinaryRawStdout, debianBinaryRawStderr} {
		d, _, now = binaryRawDecoderFixture(t)
		if stream == debianBinaryRawStdout { d.state.stdoutDebit = debianBinaryRawWireMax-1 } else { d.state.stderrDebit = debianBinaryRawStderrMax-1 }
		if _, err := d.beginRead(stream, 2, now); err == nil { t.Fatal("pending bytes exceeded stream cap") }
	}
}

func TestDebianBinaryRawMalformedHeadersRefuseBeforeArena(t *testing.T) {
	for _, kind := range []string{"magic", "kind", "part", "empty", "oversize"} {
		t.Run(kind, func(t *testing.T) {
			d, _, now := binaryRawDecoderFixture(t); header := make([]byte, 38); header[0] = 1; binary.LittleEndian.PutUint32(header[2:6], 1)
			magic := []byte(debianBinaryRawMagic)
			switch kind { case "magic": magic[0] = '!'; case "kind": header[0] = 2; case "part": header[1] = 1; case "empty": binary.LittleEndian.PutUint32(header[2:6], 0); case "oversize": binary.LittleEndian.PutUint32(header[2:6], debianBinaryRawBundleMax+1) }
			if binaryRawFeed(d, append(magic, header...), 13, now) == nil || d.state.arena != nil { t.Fatal("bad header allocated frame arena") }
		})
	}
	for _, kind := range []string{"hash", "duplicate"} {
		d, _, now := binaryRawDecoderFixture(t); frame, _ := binaryRawTestFrame(1, 0, []byte{1}); frame[len(frame)-1] = 2
		if kind == "duplicate" { frame, _ = binaryRawTestFrame(1, 0, []byte{1}); frame = append(frame, frame...) }
		if binaryRawFeed(d, append([]byte(debianBinaryRawMagic), frame...), 64, now) == nil { t.Fatal("hash/repeated frame accepted", kind) }
	}
}

func TestDebianBinaryRawScratchRemainderReusesOneArena(t *testing.T) {
	d, e, now := binaryRawDecoderFixture(t); raw := binaryRawTestWire(t, e, nil, nil)
	var backing *byte
	for len(raw) > 0 {
		if binaryRawDrain(d, now) != nil { t.Fatal("scratch advance failed") }
		n := debianBinaryRawScratchMax; if n > len(raw) { n = len(raw) }
		r, err := d.beginRead(debianBinaryRawStdout, n, now); if err != nil || d.observeRead(r, raw[:n], nil, now) != nil { t.Fatal("multi-frame observation refused") }
		if len(d.state.arena) != debianBinaryRawBundleMax || cap(d.state.arena) != debianBinaryRawBundleMax { t.Fatal("arena capacity grew") }
		if backing == nil { backing = &d.state.arena[0] } else if backing != &d.state.arena[0] { t.Fatal("next frame allocated second arena") }
		raw = raw[n:]
	}
	if binaryRawDrain(d, now) != nil || d.observation().Frames != 23 { t.Fatal("lost copied scratch remainder") }
}

func TestDebianBinaryRawScopeReferencesAndCountersReject(t *testing.T) {
	for _, kind := range []string{"operation", "loan", "selection", "source", "index", "raw-ref", "counter", "class", "seek", "eligibility", "schema", "part-scope", "span"} {
		t.Run(kind, func(t *testing.T) {
			d, e, now := binaryRawDecoderFixture(t)
			raw := binaryRawTestWire(t, e, func(p *assemblyDebianBinaryPartV1) { if kind == "part-scope" { p.OperationID = "foreign" }; if kind == "span" { p.Bindings[0].Control.Bundle.SHA256 = strings.Repeat("a", 64) } }, func(p *debianBinaryRawSummary) {
				switch kind { case "operation": p.OperationID = "foreign"; case "loan": p.LoanID = "foreign"; case "selection": p.SelectionSHA256 = strings.Repeat("a", 64); case "source": p.SourceBindingSHA256 = strings.Repeat("a", 64); case "index": p.DecodedIndexes[0].Compressed.SHA256 = strings.Repeat("a", 64); case "raw-ref": p.RawOutputs[0].Bytes++; case "counter": p.Counters.ReadCalls = 50001; case "class": p.ClassDebits.Index.Bytes = 19044409; case "seek": p.SeekCalls = 358; case "eligibility": p.ServerEligibility = "verified"; case "schema": p.Schema = 2 }
			})
			if binaryRawFeed(d, raw, 4096, now) == nil || d.observation().State != "incomplete-raw-observations" { t.Fatal("foreign/malformed raw binding accepted") }
		})
	}
}

func TestDebianBinaryRawEOFAndTrailingKeepIncomplete(t *testing.T) {
	d, _, now := binaryRawDecoderFixture(t)
	if binaryRawEOF(d, debianBinaryRawStdout, now) == nil || d.observation().StdoutDebit != 1 { t.Fatal("premature EOF accepted/refunded") }
	d, e, now := binaryRawDecoderFixture(t); raw := binaryRawTestWire(t, e, nil, nil)
	if binaryRawFeed(d, append(raw, '!'), 4096, now) == nil { t.Fatal("trailing byte accepted") }
	d, e, now = binaryRawDecoderFixture(t); raw = binaryRawTestWire(t, e, nil, nil)
	if binaryRawFeed(d, raw, 4096, now) != nil || binaryRawEOF(d, debianBinaryRawStdout, now) != nil { t.Fatal("raw stdout refused") }
	if d.observation().State != "incomplete-raw-observations" { t.Fatal("missing stderr EOF invented completeness") }
	if binaryRawEOF(d, debianBinaryRawStderr, now) != nil { t.Fatal("stderr EOF refused") }
	if _, err := d.beginRead(debianBinaryRawStdout, 1, now); err == nil || d.observation().State != "incomplete-raw-observations" { t.Fatal("post-EOF read did not invalidate completeness") }
}

func TestDebianBinaryRawExpectedMetadataAndRetirementAreShared(t *testing.T) {
	d, e, now := binaryRawDecoderFixture(t)
	e.Selection.Packages[0].AllowedCandidateIDs[0] = "foreign"; e.Selection.Packages[0].Name = "changed"
	if d.state.expected.Selection.Packages[0].Name == "changed" || d.state.expected.Selection.Packages[0].AllowedCandidateIDs[0] != "current/bookworm" { t.Fatal("mutable expected selection escaped") }
	copyDecoder := d; r, err := d.beginRead(debianBinaryRawStderr, 7, now); if err != nil { t.Fatal(err) }
	copyDecoder.retire()
	if d.observeRead(r, []byte{'x'}, nil, now) == nil || d.observation().StderrDebit != 7 || !d.observation().Pending { t.Fatal("retirement lost pending charged boundary") }
	e, now = binaryRawExpectedFixture(); e.Selection.Schema = 2
	if _, err := newDebianBinaryRawDecoder(e, now, now.Add(300*time.Second)); err == nil { t.Fatal("invalid selection metadata accepted") }
}

func TestDebianBinaryRawDuplicateAndUnknownJSONRefuseAtPart(t *testing.T) {
	for _, key := range []string{"schema", "unknown"} {
		t.Run(key, func(t *testing.T) {
			d, e, now := binaryRawDecoderFixture(t); raw := binaryRawTestWire(t, e, nil, nil)
			// First bundle is the one-byte synthetic payload. Change the first
			// part JSON, recompute its wire hash, and demand refusal there rather
			// than accidentally relying on a later summary raw-ref mismatch.
			start := len(debianBinaryRawMagic)+38+1
			size := int(binary.LittleEndian.Uint32(raw[start+2:start+6])); end := start+38+size
			body := append([]byte(fmt.Sprintf("{%q:1,", key)), raw[start+39:end]...)
			frame, _ := binaryRawTestFrame(2, 0, body)
			changed := append(append(append([]byte(nil), raw[:start]...), frame...), raw[end:]...)
			if binaryRawFeed(d, changed, 4096, now) == nil || d.observation().Frames != 1 { t.Fatal("closed JSON key checks did not refuse first raw part") }
		})
	}
}
