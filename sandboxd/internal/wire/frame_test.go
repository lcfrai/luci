package wire_test

import (
	"bytes"
	"testing"

	"github.com/kpmg-au-emu/panacea-app/sandboxd/internal/wire"
)

func TestEncodeDecodeRoundTrip(t *testing.T) {
	cases := []struct {
		name    string
		kind    wire.Kind
		payload []byte
	}{
		{"stdout", wire.Stdout, []byte("hello\x1b[0m")},
		{"stdin", wire.Stdin, []byte("ls -la\n")},
		{"empty payload", wire.Stdin, []byte{}},
		{"binary safe", wire.Stdout, []byte{0x00, 0xff, 0x1b, 0x5b}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gotKind, gotPayload, err := wire.Decode(wire.Encode(tc.kind, tc.payload))
			if err != nil {
				t.Fatalf("Decode returned error: %v", err)
			}
			if gotKind != tc.kind {
				t.Errorf("kind = %v, want %v", gotKind, tc.kind)
			}
			if !bytes.Equal(gotPayload, tc.payload) {
				t.Errorf("payload = %q, want %q", gotPayload, tc.payload)
			}
		})
	}
}

func TestDecodeEmptyFrameIsError(t *testing.T) {
	if _, _, err := wire.Decode(nil); err != wire.ErrEmptyFrame {
		t.Fatalf("err = %v, want ErrEmptyFrame", err)
	}
}

func TestResizePayloadRoundTrip(t *testing.T) {
	in := wire.ResizePayload{Cols: 120, Rows: 40}
	b, err := in.MarshalBinary()
	if err != nil {
		t.Fatalf("MarshalBinary: %v", err)
	}
	var out wire.ResizePayload
	if err := out.UnmarshalBinary(b); err != nil {
		t.Fatalf("UnmarshalBinary: %v", err)
	}
	if out != in {
		t.Errorf("got %+v, want %+v", out, in)
	}
}

func TestResizePayloadRejectsShortInput(t *testing.T) {
	var out wire.ResizePayload
	if err := out.UnmarshalBinary([]byte{0x01}); err == nil {
		t.Fatal("expected error for short input, got nil")
	}
}

func TestDecodePayloadAliasesInputFrame(t *testing.T) {
	frame := wire.Encode(wire.Stdout, []byte("abc"))
	_, payload, err := wire.Decode(frame)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	// Mutating the frame must be visible through the payload: they share memory.
	frame[1] = 'X'
	if payload[0] != 'X' {
		t.Fatal("payload does not alias frame; the documented contract changed")
	}
}

func FuzzDecode(f *testing.F) {
	f.Add([]byte{})
	f.Add([]byte{0x00})
	f.Add([]byte{0x02, 0x00, 0x78, 0x00, 0x28})
	f.Fuzz(func(t *testing.T, data []byte) {
		// Must never panic. An error return is a valid outcome.
		_, _, _ = wire.Decode(data)
	})
}

func FuzzResizePayloadUnmarshal(f *testing.F) {
	f.Add([]byte{0x00, 0x78, 0x00, 0x28})
	f.Add([]byte{0x01})
	f.Fuzz(func(t *testing.T, data []byte) {
		var rp wire.ResizePayload
		_ = rp.UnmarshalBinary(data)
	})
}
