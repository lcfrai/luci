package main

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"io"
)

const (
	frameMagic = "LUCI-PDF-PILOT/1\n"
	frameEnd = "LUCI-PDF-PILOT-END/1\n"
	frameRawMax = 34<<20 + 16<<10
	recordPDF byte = 'P'
	recordText byte = 'T'
	recordPNG byte = 'N'
)

type frameRecord struct {
	kind byte
	data []byte
}

func validFrame(r driverRequest, records []frameRecord) bool {
	if !validHex(r.attempt, 16) || !validHex(r.allocation, 32) || !validHex(r.inputDigest, 32) {
		return false
	}
	if r.role == roleRender {
		return r.inputDigest == fontDigest && len(records) == 1 && records[0].kind == recordPDF && len(records[0].data) > 0 && len(records[0].data) <= maxPDF
	}
	return r.role == roleRead && len(records) == 2 && records[0].kind == recordText && records[1].kind == recordPNG &&
		len(records[0].data) > 0 && len(records[0].data) <= maxText && len(records[1].data) > 0 && len(records[1].data) <= maxPNG
}

func writeFull(w io.Writer, data []byte) error {
	for len(data) != 0 {
		n, err := w.Write(data)
		if n < 0 || n > len(data) || err != nil {
			return errRefused
		}
		if n == 0 {
			return io.ErrShortWrite
		}
		data = data[n:]
	}
	return nil
}

// Only framed, complete result bytes reach stdout. Docker multiplex headers
// and any extra stream/EOF are independently bounded by the future API parser.
func writeFrame(w io.Writer, r driverRequest, records []frameRecord) error {
	if w == nil || !validFrame(r, records) {
		return errRefused
	}
	role := byte('R')
	if r.role == roleRead {
		role = 'D'
	}
	allocation, err := hex.DecodeString(r.allocation)
	if err != nil {
		return errRefused
	}
	input, err := hex.DecodeString(r.inputDigest)
	if err != nil {
		return errRefused
	}
	size := len(frameMagic) + 1 + 32 + 32 + 2 + len(frameEnd)
	for _, record := range records {
		size += 1 + 4 + 32 + len(record.data)
	}
	if size > frameRawMax {
		return errRefused
	}
	pageCount := byte(0)
	if r.role == roleRead {
		// runRole reaches this frame only after bounded pdfinfo proved one page.
		pageCount = 1
	}
	if writeFull(w, []byte(frameMagic)) != nil || writeFull(w, []byte{role}) != nil ||
		writeFull(w, allocation) != nil || writeFull(w, input) != nil || writeFull(w, []byte{pageCount, byte(len(records))}) != nil {
		return errRefused
	}
	for _, record := range records {
		var header [1 + 4 + 32]byte
		header[0] = record.kind
		binary.BigEndian.PutUint32(header[1:5], uint32(len(record.data)))
		digest := sha256.Sum256(record.data)
		copy(header[5:], digest[:])
		if writeFull(w, header[:]) != nil || writeFull(w, record.data) != nil {
			return errRefused
		}
	}
	return writeFull(w, []byte(frameEnd))
}

// A blocked attach writer cannot delay the role's deadline. The process exits
// on error, while the owning controller's independent watchdog retires its
// exact CID. The blocked goroutine cannot outlive this one container process.
func emitFrame(ctx context.Context, w io.Writer, r driverRequest, records []frameRecord) error {
	if ctx.Err() != nil {
		return errRefused
	}
	done := make(chan error, 1)
	go func() { done <- writeFrame(w, r, records) }()
	select {
	case err := <-done:
		if ctx.Err() != nil {
			return errRefused
		}
		return err
	case <-ctx.Done():
		return errRefused
	}
}
