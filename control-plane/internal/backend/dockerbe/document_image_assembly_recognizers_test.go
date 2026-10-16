package dockerbe

import (
	"context"
	"strings"
	"testing"

	"github.com/lcfrai/luci/control-plane/internal/backend"
)

func TestAssemblySharedRecognizersRequireExactPurposeAndInertConfiguration(t *testing.T) {
	b, f, spec := newAssemblyProbe(t)
	spec.Identity.EngineID, spec.Identity.InstanceID = f.info.ID, b.instance
	labels := assemblyLabels(spec.Identity)
	if containerResourceKind(labels) != "document-image-assembly" {
		t.Fatal("valid assembly kind unavailable")
	}
	for _, key := range []string{backend.LabelManaged, checkLabel, documentLabel, "luci.preview"} {
		mixed := map[string]string{}
		for k, v := range labels {
			mixed[k] = v
		}
		mixed[key] = "true"
		if containerResourceKind(mixed) != "other" {
			t.Fatal("mixed purpose deduplicated as managed assembly/worker", key)
		}
	}
	bad := assemblyLabels(spec.Identity)
	delete(bad, "luci.assembly-source")
	if containerResourceKind(bad) != "other" {
		t.Fatal("incomplete assembly identity recognized")
	}
	req := backend.DocumentImageAssemblyFenceRequest{Identity: spec.Identity, IntentID: strings.Repeat("a", 32)}
	proof, err := b.FenceDocumentImageAssembly(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	row := f.rows[proof.FenceRef]
	if !inertFenceContainer(row) {
		t.Fatal("valid exact assembler inert fence rejected by shared recognizer")
	}
	row.HostConfig.Privileged = true
	if inertFenceContainer(row) {
		t.Fatal("unsafe marker exempted by assembly labels")
	}
	if len(f.starts) != 0 {
		t.Fatal("read recognition/fencing executed helper")
	}
}
