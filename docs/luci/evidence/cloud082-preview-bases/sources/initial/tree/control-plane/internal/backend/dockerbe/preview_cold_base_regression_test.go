package dockerbe

import (
	"context"
	"testing"
)

// This test uses only the pre-CLOUD-082 API so it can demonstrate the original
// cold-base product gap independently of the new guarded build signature.
func TestPreviewColdBasePreparationNeedsNoManualDockerPull(t *testing.T) {
	b, f, spec := newPreviewProbe(t)
	delete(f.images, "node:fixture")
	if _, err := b.PreparePreview(context.Background(), spec); err != nil {
		t.Fatalf("a declared but uncached base blocked preview preparation: %v", err)
	}
	if len(f.buildOptions)+len(f.creates)+len(f.starts) != 0 {
		t.Fatal("read-only cold-base preparation mutated resources")
	}
}
