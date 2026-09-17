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
