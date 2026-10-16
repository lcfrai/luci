package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"io"
	"strings"
	"testing"
	"time"
)

func frameFixture(role string) driverRequest {
	input := strings.Repeat("c", 64)
	if role == roleRender { input = fontDigest }
	return driverRequest{role: role, attempt: strings.Repeat("a", 32), allocation: strings.Repeat("b", 64), inputDigest: input}
}

// Test-only inverse parser proves the emitted complete wire, including exact
// ordered lengths/digests and EOF. Product capture needs its own reviewed parser.
func decodeTestFrame(t *testing.T, data []byte, request driverRequest, role byte, kinds ...byte) [][]byte {
	t.Helper()
	reader := bytes.NewReader(data)
	magic := make([]byte, len(frameMagic))
	if _, err := io.ReadFull(reader, magic); err != nil || string(magic) != frameMagic {
		t.Fatal("missing exact frame start")
	}
	header := make([]byte, 1+32+32+2)
	pageCount := byte(0)
	if role == 'D' { pageCount = 1 }
	if _, err := io.ReadFull(reader, header); err != nil || header[0] != role || header[65] != pageCount || int(header[66]) != len(kinds) {
		t.Fatal("role or record count differs")
	}
	allocation, _ := hex.DecodeString(request.allocation)
	input, _ := hex.DecodeString(request.inputDigest)
	if !bytes.Equal(header[1:33], allocation) || !bytes.Equal(header[33:65], input) {
		t.Fatal("allocation or input binding differs")
	}
	var payloads [][]byte
	for _, kind := range kinds {
		var record [1+4+32]byte
		if _, err := io.ReadFull(reader, record[:]); err != nil || record[0] != kind {
			t.Fatal("record order differs")
		}
		size := binary.BigEndian.Uint32(record[1:5])
		if size == 0 || size > maxPNG {
			t.Fatal("record length differs")
		}
		payload := make([]byte, int(size))
		if _, err := io.ReadFull(reader, payload); err != nil {
			t.Fatal("record truncated")
		}
		digest := sha256.Sum256(payload)
		if !bytes.Equal(record[5:], digest[:]) {
			t.Fatal("record digest differs")
		}
		payloads = append(payloads, payload)
	}
	end := make([]byte, len(frameEnd))
	if _, err := io.ReadFull(reader, end); err != nil || string(end) != frameEnd || reader.Len() != 0 {
		t.Fatal("frame end or EOF differs")
	}
	return payloads
}

func TestPDFPilotDriverFrameBindsExactRolesRecordsAndEOF(t *testing.T) {
	for _, fixture := range []struct {
		role string
		records []frameRecord
		wireRole byte
		kinds []byte
	}{
		{roleRender, []frameRecord{{kind: recordPDF, data: []byte("%PDF-1.7\n")}}, 'R', []byte{recordPDF}},
		{roleRead, []frameRecord{{kind: recordText, data: []byte("日本語\n")}, {kind: recordPNG, data: []byte("png")}}, 'D', []byte{recordText, recordPNG}},
	} {
		var out bytes.Buffer
		request := frameFixture(fixture.role)
		if err := writeFrame(&out, request, fixture.records); err != nil {
			t.Fatal(err)
		}
		payloads := decodeTestFrame(t, out.Bytes(), request, fixture.wireRole, fixture.kinds...)
		for i := range payloads {
			if !bytes.Equal(payloads[i], fixture.records[i].data) {
				t.Fatal("framed original bytes differ")
			}
		}
	}
}

type shortFrameWriter struct{}
func (shortFrameWriter) Write([]byte) (int, error) { return 0, nil }

func TestPDFPilotDriverFrameRefusesBadOrderOverrunAndIncompleteWrite(t *testing.T) {
	r := frameFixture(roleRead)
	for _, records := range [][]frameRecord{
		{{kind: recordPNG, data: []byte("png")}, {kind: recordText, data: []byte("text")}},
		{{kind: recordText, data: []byte("text")}},
		{{kind: recordText, data: bytes.Repeat([]byte{'x'}, maxText+1)}, {kind: recordPNG, data: []byte("png")}},
	} {
		var out bytes.Buffer
		if writeFrame(&out, r, records) == nil || out.Len() != 0 {
			t.Fatal("malformed frame emitted bytes")
		}
	}
	if writeFrame(shortFrameWriter{}, r, []frameRecord{{kind: recordText, data: []byte("text")}, {kind: recordPNG, data: []byte("png")}}) == nil {
		t.Fatal("short output write was accepted")
	}
}

type blockedFrameWriter struct { release <-chan struct{} }
func (w blockedFrameWriter) Write(p []byte) (int, error) { <-w.release; return len(p), nil }

func TestPDFPilotDriverFrameCancellationDoesNotWaitForBlockedCapture(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	release := make(chan struct{})
	done := make(chan error, 1)
	go func() { done <- emitFrame(ctx, blockedFrameWriter{release}, frameFixture(roleRender), []frameRecord{{kind: recordPDF, data: []byte("pdf")}}) }()
	cancel()
	select {
	case err := <-done:
		if err == nil { t.Fatal("cancelled frame became accepted") }
	case <-time.After(time.Second):
		close(release)
		t.Fatal("cancelled frame waited for blocked capture")
	}
	close(release)
}
