package dockerbe

import (
	"encoding/binary"
	"testing"
	"unicode/utf16"
)

func TestDockerDesktopWSLInventoryRequiresExactRunningDistro(t *testing.T) {
	encode := func(value string) []byte {
		units := utf16.Encode([]rune(value))
		out := []byte{0xff, 0xfe}
		for _, unit := range units {
			out = binary.LittleEndian.AppendUint16(out, unit)
		}
		return out
	}
	for _, tc := range []struct {
		name, text string
		want       bool
	}{
		{"running", "docker-desktop\r\nUbuntu\r\n", true},
		{"absent", "Ubuntu\r\n", false},
		{"suffix", "docker-desktop-data\r\n", false},
		{"empty", "", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := dockerDesktopWSLRunning(encode(tc.text))
			if err != nil || got != tc.want {
				t.Fatal(got, err)
			}
		})
	}
	if _, err := dockerDesktopWSLRunning([]byte{0xff, 0xfe, 'a'}); err == nil {
		t.Fatal("truncated WSL response was accepted")
	}
}
