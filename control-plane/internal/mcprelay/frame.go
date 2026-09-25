// Package mcprelay frames the canvas MCP relay: many byte streams carried
// over one WebSocket between sandboxd and the control plane.
//
// SHARED RULE, TWO SITES. This file exists twice, byte for byte:
// sandboxd/internal/mcprelay/frame.go and control-plane/internal/mcprelay/frame.go.
// The two modules share no code (sandboxd is its own module, vendored into
// the sandbox image), so the wire format is written once per module, as
// backend.MaxMessageBytes is. testdata/frames.golden, identical in both, pins
// the two codecs to one set of bytes, and the control plane's twin tests fail
// when either copy is edited alone. The framing is ours because
// hashicorp/yamux is MPL-2.0, outside OD-11 rule 3 (plan P5-C).
package mcprelay

import (
	"encoding/binary"
	"errors"
	"fmt"
	"unicode/utf8"
)

// Wire format, one frame per WebSocket BINARY message:
//
//	byte 0      kind
//	bytes 1-4   stream id, big-endian uint32 (odd: opened by sandboxd; 0: the channel itself)
//	bytes 5-    payload
//
// Kinds:
//
//	0x01 OPEN    sandboxd → cp   a new local connection; payload empty
//	0x02 DATA    both            payload 1..MaxPayload bytes of the stream
//	0x03 CLOSE   both            the sender will send no more on this stream (half-close); payload empty
//	0x04 RESET   both            abort the stream; payload a UTF-8 reason ≤ MaxReason bytes
//	0x05 WINDOW  both            payload a big-endian uint32: bytes the receiver has consumed and may be sent again
//	0x06 PING    both            stream 0; payload ≤ MaxPing bytes, echoed as PONG
//	0x07 PONG    both            stream 0
type Kind byte

const (
	KindOpen   Kind = 0x01
	KindData   Kind = 0x02
	KindClose  Kind = 0x03
	KindReset  Kind = 0x04
	KindWindow Kind = 0x05
	KindPing   Kind = 0x06
	KindPong   Kind = 0x07
)

const (
	HeaderBytes   = 5
	MaxPayload    = 32 << 10 // 32 KiB of stream data per frame
	MaxFrameBytes = HeaderBytes + MaxPayload
	InitialWindow = 256 << 10 // per stream: a sender may have this many unacknowledged bytes in flight
	WindowAck     = 64 << 10  // a receiver sends WINDOW once it has handed this many bytes to its reader
	MaxStreams    = 64        // a 65th OPEN is answered with RESET "too many streams"
	MaxReason     = 200       // bytes of a RESET's UTF-8 reason
	MaxPing       = 8         // bytes of a PING's payload, echoed in its PONG
	ChannelPath   = "/mcp-channel"
	ListenAddr    = "127.0.0.1:8731" // inside the sandbox; the URL the harness is given (task 19)
)

// Stream rules. The codec below pins the bytes; these pin what each side's
// channel does with them, and both sides' channels are written apart
// (sandboxd's channel.go, the control plane's task 19), so they are stated
// here, where both read them:
//
//   - Only sandboxd sends OPEN, with an odd id new to the channel. An OPEN
//     from the control plane is a protocol error: sandboxd closes the channel
//     with StatusProtocolError, as it does a malformed frame, and a text
//     message with StatusUnsupportedData.
//   - A stream ends at each side at a different moment, so a DATA, CLOSE,
//     RESET or WINDOW for a stream that has ended here, or that was never
//     opened, is dropped, not an error. In particular sandboxd queues RESET
//     but writes DATA, WINDOW and CLOSE directly, so a DATA or a WINDOW can
//     follow a RESET for the same stream.
//   - Flow control, per stream and direction: a sender never has more than
//     InitialWindow bytes unacknowledged, and a receiver sends WINDOW once it
//     has handed at least WindowAck bytes to its reader. DATA past the window,
//     a WINDOW for bytes never sent and DATA after CLOSE each reset the stream
//     with the matching reason below.
//   - A RESET's reason is one of the reasons below. sandboxd logs a peer's
//     reason only when it is one of them, and any other by its length, so G5
//     holds whatever the peer sends.
//   - A second channel supersedes the first, which sandboxd closes with status
//     4001. Only a holder of the attach token can attach one, so a peer
//     closed with 4001 that it did not cause should say so, not stop quietly.
const (
	reasonTooMany        = "too many streams"
	reasonWindow         = "window exceeded"
	reasonBadWindow      = "window acknowledges bytes never sent"
	reasonDataAfterClose = "data after close"
	reasonLocalRead      = "local read failed"
	reasonLocalWrite     = "local write failed"
	reasonUnexpectedOpen = "sandboxd opens every stream"
	reasonIDs            = "stream ids exhausted"
)

// knownReason reports whether r is one of the fixed reasons above.
func knownReason(r string) bool {
	switch r {
	case reasonTooMany, reasonWindow, reasonBadWindow, reasonDataAfterClose,
		reasonLocalRead, reasonLocalWrite, reasonUnexpectedOpen, reasonIDs:
		return true
	}
	return false
}

// Frame is one decoded message. A decoded Payload aliases the input slice.
type Frame struct {
	Kind    Kind
	Stream  uint32
	Payload []byte
}

// ErrFrame is wrapped by every Encode and Decode error.
var ErrFrame = errors.New("mcprelay: malformed frame")

func (k Kind) String() string {
	switch k {
	case KindOpen:
		return "OPEN"
	case KindData:
		return "DATA"
	case KindClose:
		return "CLOSE"
	case KindReset:
		return "RESET"
	case KindWindow:
		return "WINDOW"
	case KindPing:
		return "PING"
	case KindPong:
		return "PONG"
	}
	return fmt.Sprintf("Kind(0x%02x)", byte(k))
}

// Encode returns f's wire bytes. It refuses, with ErrFrame, every frame Decode
// would refuse: an oversized payload, a kind/stream mismatch, a payload the
// kind does not allow. A sender cannot put on the wire what its peer rejects.
func Encode(f Frame) ([]byte, error) {
	if err := check(f); err != nil {
		return nil, err
	}
	b := make([]byte, HeaderBytes+len(f.Payload))
	b[0] = byte(f.Kind)
	binary.BigEndian.PutUint32(b[1:HeaderBytes], f.Stream)
	copy(b[HeaderBytes:], f.Payload)
	return b, nil
}

// Decode parses one message. It returns ErrFrame on short input, an unknown
// kind, an oversized payload or any other frame Encode would refuse, and never
// panics on hostile input.
func Decode(b []byte) (Frame, error) {
	if len(b) < HeaderBytes {
		return Frame{}, fmt.Errorf("%w: %d bytes, shorter than the %d-byte header", ErrFrame, len(b), HeaderBytes)
	}
	f := Frame{Kind: Kind(b[0]), Stream: binary.BigEndian.Uint32(b[1:HeaderBytes]), Payload: b[HeaderBytes:]}
	if err := check(f); err != nil {
		return Frame{}, err
	}
	return f, nil
}

// WindowPayload is a WINDOW frame's payload for an increment of n bytes.
func WindowPayload(n uint32) []byte { return binary.BigEndian.AppendUint32(nil, n) }

// WindowIncrement reads a WINDOW frame's increment. f must have passed Decode.
func WindowIncrement(f Frame) uint32 { return binary.BigEndian.Uint32(f.Payload) }

func check(f Frame) error {
	n := len(f.Payload)
	if n > MaxPayload {
		return fmt.Errorf("%w: %d-byte payload exceeds %d", ErrFrame, n, MaxPayload)
	}
	switch f.Kind {
	case KindOpen, KindData, KindClose, KindReset, KindWindow:
		if f.Stream == 0 {
			return fmt.Errorf("%w: %s on stream 0", ErrFrame, f.Kind)
		}
	case KindPing, KindPong:
		if f.Stream != 0 {
			return fmt.Errorf("%w: %s on stream %d, not 0", ErrFrame, f.Kind, f.Stream)
		}
	default:
		return fmt.Errorf("%w: unknown kind 0x%02x", ErrFrame, byte(f.Kind))
	}
	switch f.Kind {
	case KindOpen, KindClose:
		if n != 0 {
			return fmt.Errorf("%w: %s carries no payload, got %d bytes", ErrFrame, f.Kind, n)
		}
	case KindData:
		if n == 0 {
			return fmt.Errorf("%w: empty DATA", ErrFrame)
		}
	case KindReset:
		if n > MaxReason || !utf8.Valid(f.Payload) {
			return fmt.Errorf("%w: RESET reason must be UTF-8 of at most %d bytes", ErrFrame, MaxReason)
		}
	case KindWindow:
		if n != 4 {
			return fmt.Errorf("%w: WINDOW payload is 4 bytes, got %d", ErrFrame, n)
		}
	case KindPing, KindPong:
		if n > MaxPing {
			return fmt.Errorf("%w: %s payload exceeds %d bytes", ErrFrame, f.Kind, MaxPing)
		}
	}
	return nil
}
