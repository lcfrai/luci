//go:build !linux

package main

import "testing"

func TestPDFPilotDriverUnsupportedPlatformRefusesOriginalAndStage(t *testing.T) {
	if _, err := readBoundedRegular(fontPath, fontBytes); err == nil { t.Fatal("unsupported original-file open accepted") }
	if _, err := newPilotStage("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"); err == nil { t.Fatal("unsupported writable stage accepted") }
}
