package mcprelay

// SHARED RULE, TWO SITES: this file is identical in sandboxd and the control
// plane, as frame.go and testdata/frames.golden are.

import (
	"bufio"
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"
	"math/rand"
	"os"
	"strings"
	"testing"
)

type goldenLine struct {
	no   int
	hex  string
	desc string
}

func readGolden(t *testing.T) []goldenLine {
	t.Helper()
	f, err := os.Open("testdata/frames.golden")
	if err != nil {
		t.Fatalf("open golden: %v", err)
	}
	defer f.Close()
	var out []goldenLine
	sc := bufio.NewScanner(f)
	for no := 1; sc.Scan(); no++ {
		line := strings.TrimRight(sc.Text(), "\r")
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		h, desc, ok := strings.Cut(line, " ")
		if !ok {
			t.Fatalf("golden line %d has no description: %q", no, line)
		}
		out = append(out, goldenLine{no: no, hex: h, desc: desc})
	}
	if err := sc.Err(); err != nil {
		t.Fatalf("read golden: %v", err)
	}
	return out
}

// describe renders f as the golden file writes it: %+v, with a non-empty
// printable-ASCII payload quoted. The text is independent of the codec, so a
// codec that round-trips its own mistake still fails against it.
func describe(f Frame) string {
	p := fmt.Sprint(f.Payload)
	if len(f.Payload) > 0 && printable(f.Payload) {
		p = fmt.Sprintf("%q", f.Payload)
	}
	return fmt.Sprintf("{Kind:%d Stream:%d Payload:%s}", byte(f.Kind), f.Stream, p)
}

func printable(b []byte) bool {
	for _, c := range b {
		if c < 0x20 || c > 0x7e {
			return false
		}
	}
	return true
}

func TestGolden(t *testing.T) {
	lines := readGolden(t)
	seen := map[Kind]bool{}
	for _, gl := range lines {
		raw, err := hex.DecodeString(gl.hex)
		if err != nil {
			t.Fatalf("line %d: bad hex: %v", gl.no, err)
		}
		f, err := Decode(raw)
		if err != nil {
			t.Errorf("line %d: Decode(%s): %v", gl.no, gl.hex, err)
			continue
		}
		if got := describe(f); got != gl.desc {
			t.Errorf("line %d: Decode(%s) = %s, golden says %s", gl.no, gl.hex, got, gl.desc)
		}
		enc, err := Encode(f)
		if err != nil {
			t.Errorf("line %d: Encode(%s): %v", gl.no, gl.desc, err)
			continue
		}
		if got := hex.EncodeToString(enc); got != gl.hex {
			t.Errorf("line %d: Encode(%s) = %s, golden says %s", gl.no, gl.desc, got, gl.hex)
		}
		seen[f.Kind] = true
	}
	for k := KindOpen; k <= KindPong; k++ {
		if !seen[k] {
			t.Errorf("the golden file has no %s frame", k)
		}
	}
}

// raw builds a frame's bytes by hand, bypassing Encode's checks.
func raw(k byte, stream uint32, payload []byte) []byte {
	b := []byte{k, byte(stream >> 24), byte(stream >> 16), byte(stream >> 8), byte(stream)}
	return append(b, payload...)
}

// badFrames are frames both Encode and Decode must refuse.
var badFrames = []struct {
	name string
	f    Frame
}{
	{"kind 0x09", Frame{Kind: 0x09, Stream: 1}},
	{"kind 0x00", Frame{Kind: 0x00, Stream: 1}},
	{"32 KiB + 1 payload", Frame{Kind: KindData, Stream: 1, Payload: make([]byte, MaxPayload+1)}},
	{"DATA on stream 0", Frame{Kind: KindData, Stream: 0, Payload: []byte("x")}},
	{"OPEN on stream 0", Frame{Kind: KindOpen, Stream: 0}},
	{"CLOSE on stream 0", Frame{Kind: KindClose, Stream: 0}},
	{"RESET on stream 0", Frame{Kind: KindReset, Stream: 0}},
	{"WINDOW on stream 0", Frame{Kind: KindWindow, Stream: 0, Payload: []byte{0, 0, 0, 1}}},
	{"PING on stream 1", Frame{Kind: KindPing, Stream: 1}},
	{"PONG on stream 2", Frame{Kind: KindPong, Stream: 2}},
	{"OPEN with a payload", Frame{Kind: KindOpen, Stream: 1, Payload: []byte("x")}},
	{"CLOSE with a payload", Frame{Kind: KindClose, Stream: 1, Payload: []byte("x")}},
	{"empty DATA", Frame{Kind: KindData, Stream: 1}},
	{"RESET reason of 201 bytes", Frame{Kind: KindReset, Stream: 1, Payload: bytes.Repeat([]byte("r"), MaxReason+1)}},
	{"RESET reason not UTF-8", Frame{Kind: KindReset, Stream: 1, Payload: []byte{0xff, 0xfe}}},
	{"WINDOW of 3 bytes", Frame{Kind: KindWindow, Stream: 1, Payload: []byte{0, 0, 1}}},
	{"WINDOW of 5 bytes", Frame{Kind: KindWindow, Stream: 1, Payload: []byte{0, 0, 0, 0, 1}}},
	{"PING of 9 bytes", Frame{Kind: KindPing, Stream: 0, Payload: make([]byte, MaxPing+1)}},
	{"PONG of 9 bytes", Frame{Kind: KindPong, Stream: 0, Payload: make([]byte, MaxPing+1)}},
}

func TestDecode_rejects(t *testing.T) {
	short := map[string][]byte{"empty": nil, "4-byte input": {byte(KindData), 0, 0, 1}}
	for name, b := range short {
		if _, err := Decode(b); !errors.Is(err, ErrFrame) {
			t.Errorf("%s: Decode err = %v, want ErrFrame", name, err)
		}
	}
	for _, tc := range badFrames {
		if _, err := Decode(raw(byte(tc.f.Kind), tc.f.Stream, tc.f.Payload)); !errors.Is(err, ErrFrame) {
			t.Errorf("%s: Decode err = %v, want ErrFrame", tc.name, err)
		}
	}
}

func TestEncode_rejects(t *testing.T) {
	for _, tc := range badFrames {
		if b, err := Encode(tc.f); !errors.Is(err, ErrFrame) || b != nil {
			t.Errorf("%s: Encode = %d bytes, %v; want nil, ErrFrame", tc.name, len(b), err)
		}
	}
}

// TestCodec_limitsAccepted pins each bound as inclusive, so an off-by-one
// check fails here rather than on a harness's largest write.
func TestCodec_limitsAccepted(t *testing.T) {
	good := []Frame{
		{Kind: KindData, Stream: 1, Payload: make([]byte, MaxPayload)},
		{Kind: KindReset, Stream: 3, Payload: bytes.Repeat([]byte("r"), MaxReason)},
		{Kind: KindReset, Stream: 3},
		{Kind: KindPing, Stream: 0, Payload: make([]byte, MaxPing)},
		{Kind: KindPong, Stream: 0},
		{Kind: KindData, Stream: 0xffffffff, Payload: []byte{0}},
	}
	for _, f := range good {
		b, err := Encode(f)
		if err != nil {
			t.Errorf("Encode(%s, %d bytes): %v", f.Kind, len(f.Payload), err)
			continue
		}
		if len(b) != HeaderBytes+len(f.Payload) {
			t.Errorf("Encode(%s): %d bytes, want %d", f.Kind, len(b), HeaderBytes+len(f.Payload))
		}
		got, err := Decode(b)
		if err != nil || got.Kind != f.Kind || got.Stream != f.Stream || !bytes.Equal(got.Payload, f.Payload) {
			t.Errorf("Decode(Encode(%s)) = %+v, %v", f.Kind, got.Kind, err)
		}
	}
}

// TestDecode_hostileNeverPanics feeds short random inputs: Decode never
// panics, and what it accepts re-encodes to the same bytes.
func TestDecode_hostileNeverPanics(t *testing.T) {
	r := rand.New(rand.NewSource(18))
	for i := 0; i < 20000; i++ {
		b := make([]byte, r.Intn(24))
		r.Read(b)
		if len(b) > 0 {
			b[0] = byte(r.Intn(10))
		}
		f, err := Decode(b)
		if err != nil {
			continue
		}
		enc, err := Encode(f)
		if err != nil || !bytes.Equal(enc, b) {
			t.Fatalf("Decode(%x) accepted %+v but Encode gave %x, %v", b, f, enc, err)
		}
	}
}
