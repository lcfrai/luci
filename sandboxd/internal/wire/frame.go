// Package wire defines the frame protocol spoken between the desktop app,
// the control plane and sandboxd. It deliberately knows nothing about
// terminal semantics: payloads are opaque bytes.
package wire

import (
	"encoding/binary"
	"errors"
)

// Kind identifies what a frame carries.
type Kind uint8

const (
	Stdout Kind = 0x00 // server -> client: raw PTY output
	Stdin  Kind = 0x01 // client -> server: raw keystrokes
	Resize Kind = 0x02 // client -> server: ResizePayload
	Exit   Kind = 0x03 // server -> client: payload is the exit reason
)

// ErrEmptyFrame is returned by Decode when given a zero-length frame.
var ErrEmptyFrame = errors.New("wire: empty frame")

// Encode prefixes payload with its kind.
func Encode(k Kind, payload []byte) []byte {
	out := make([]byte, 0, len(payload)+1)
	out = append(out, byte(k))
	return append(out, payload...)
}

// Decode splits a frame into its kind and payload. The returned payload
// aliases frame; copy it if you need to retain it.
func Decode(frame []byte) (Kind, []byte, error) {
	if len(frame) == 0 {
		return 0, nil, ErrEmptyFrame
	}
	return Kind(frame[0]), frame[1:], nil
}

// ResizePayload is the body of a Resize frame.
type ResizePayload struct {
	Cols uint16
	Rows uint16
}

// MarshalBinary encodes the payload as four big-endian bytes.
func (r ResizePayload) MarshalBinary() ([]byte, error) {
	b := make([]byte, 4)
	binary.BigEndian.PutUint16(b[0:2], r.Cols)
	binary.BigEndian.PutUint16(b[2:4], r.Rows)
	return b, nil
}

// UnmarshalBinary decodes a payload produced by MarshalBinary.
func (r *ResizePayload) UnmarshalBinary(b []byte) error {
	if len(b) < 4 {
		return errors.New("wire: resize payload too short")
	}
	r.Cols = binary.BigEndian.Uint16(b[0:2])
	r.Rows = binary.BigEndian.Uint16(b[2:4])
	return nil
}
