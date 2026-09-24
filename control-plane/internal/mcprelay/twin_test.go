package mcprelay

// The control plane's copy of the relay framing has a twin in the sandboxd
// module (frame.go's SHARED RULE, TWO SITES). These tests live on this side
// only: the control plane's tests run from the monorepo, where the twin is
// always present, while sandboxd is also built alone inside the image.

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// twinDir is sandboxd's copy, from this package's directory.
const twinDir = "../../../sandboxd/internal/mcprelay"

func TestGoldenFilesIdentical(t *testing.T) { sameAsTwin(t, "testdata/frames.golden") }

// TestFrameSourceIdentical widens the golden file's guard to the codec and
// its tests: the golden lines pin encodings, but a check that only one copy
// makes (DATA on stream 0, say) would drift past them.
func TestFrameSourceIdentical(t *testing.T) {
	sameAsTwin(t, "frame.go")
	sameAsTwin(t, "frame_test.go")
}

func sameAsTwin(t *testing.T, rel string) {
	t.Helper()
	mine, err := os.ReadFile(rel)
	if err != nil {
		t.Fatalf("read %s: %v", rel, err)
	}
	twinPath := filepath.Join(twinDir, filepath.FromSlash(rel))
	twin, err := os.ReadFile(twinPath)
	if err != nil {
		t.Fatalf("read the sandboxd twin %s: %v", twinPath, err)
	}
	if !bytes.Equal(mine, twin) {
		t.Errorf("%s differs from %s: the relay framing is one rule in two modules; edit both copies together", rel, twinPath)
	}
}
