package dockerbe

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"testing"

	"github.com/lcfrai/luci/control-plane/internal/backend"
)

func TestReviewedRuntimePlatformBindsOnlySupportedBuildAndPreservesLegacyDigest(t *testing.T) {
	_, _, spec := newPreviewProbe(t)
	if !validPreviewSpec(spec) {
		t.Fatal("legacy fixture invalid")
	}
	prepared := backend.PreparedPreview{Spec: spec, ContextDigest: "context"}
	raw, _ := json.Marshal(struct {
		Identity            backend.PreviewIdentity
		Context, Dockerfile string
		Bases               []backend.PreviewBaseImage
	}{spec.Identity, prepared.ContextDigest, spec.Dockerfile, prepared.BasePlan})
	sum := sha256.Sum256(raw)
	legacy := previewBasePlanDigest(prepared)
	if legacy != hex.EncodeToString(sum[:]) {
		t.Fatal("omitted platform changed historical recipe digest")
	}
	prepared.Spec.Platform = "linux/amd64"
	if !validPreviewSpec(prepared.Spec) || previewBasePlanDigest(prepared) == legacy {
		t.Fatal("supported architecture did not bind recipe")
	}
	prepared.Spec.Platform = "linux/arm64"
	if validPreviewSpec(prepared.Spec) {
		t.Fatal("unsupported emulation/fallback admitted")
	}
	prepared.Spec.Platform = "linux/amd64"
	prepared.Spec.Kind = "preview"
	if validPreviewSpec(prepared.Spec) {
		t.Fatal("build platform widened serving protocol")
	}
}
