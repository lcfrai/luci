package api

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/lcfrai/luci/control-plane/internal/backend"
	"github.com/lcfrai/luci/control-plane/internal/previewruntime"
)

// Only the typed backend is substituted. These tests prove lease/authority/CAS
// behavior; they make no actual Docker, Node, HTTP or source-quality claim.
type sourcePreviewBackendFixture struct{ *previewBackendFixture }

func (f *sourcePreviewBackendFixture) PreparePreview(ctx context.Context, s backend.PreviewSpec) (backend.PreparedPreview, error) {
	p, err := f.previewBackendFixture.PreparePreview(ctx, s)
	if err != nil {
		return p, err
	}
	if s.Kind == backend.ReviewedSourcePreview {
		p.Spec.Identity.SnapshotDigest = strings.Repeat("d", 64)
		p.Spec.Identity.CommandDigest = strings.Repeat("e", 64)
		p.Spec.Identity.EnvironmentDigest = strings.Repeat("9", 64)
		p.Spec.Identity.RuntimeImageID = "sha256:" + strings.Repeat("f", 64)
		p.Spec.ImageID = p.Spec.Identity.RuntimeImageID
		p.Spec.ContextDigest = p.Spec.Identity.SnapshotDigest
		p.ContextDigest = p.Spec.ContextDigest
		p.SourcePayload = []byte("synthetic payload; not a real server")
	}
	return p, nil
}

func TestReviewedSourceUsesOneServingLeaseAndRetiresAcrossReopen(t *testing.T) {
	a, f, req := previewRuntimeFixture(t)
	a.deps.Backend = &sourcePreviewBackendFixture{f}
	req.Kind = previewruntime.ReviewedSourceKind
	var phases []string
	publish := func(out previewruntime.Outcome) error { phases = append(phases, out.Phase); return nil }
	out, err := a.startPreview(context.Background(), req, publish)
	if err != nil || out.Kind != req.Kind || out.SnapshotDigest == "" || out.BuildAllocationID != "" || out.ContainerID == "" || !out.ResourcesHeld || f.builds != 0 || f.creates != 1 || f.starts != 1 {
		t.Fatal("static serving bypassed its one-lease lifecycle", out, err)
	}
	for _, phase := range phases {
		if phase == "building" || phase == "built" {
			t.Fatal("source serving claimed an application image build")
		}
	}
	previewRetained(t, a, 1)
	a.execution = newExecutionAdmission(a)
	observed, err := a.observePreview(context.Background(), out.Identity)
	if err != nil || observed.Phase != "running" || observed.Identity != out.Identity {
		t.Fatal("reopen replaced or lost source identity", observed, err)
	}
	changed := out.Identity
	changed.Kind = ""
	if _, err := a.observePreview(context.Background(), changed); err == nil {
		t.Fatal("source preview accepted legacy image-build identity")
	}
	changed = out.Identity
	changed.SnapshotDigest = strings.Repeat("a", 64)
	if _, err := a.stopPreview(context.Background(), changed, publish); err == nil || f.removes != 0 {
		t.Fatal("changed static snapshot could retire a resource")
	}
	out, err = a.stopPreview(context.Background(), out.Identity, publish)
	if err != nil || out.ResourcesHeld || !out.CleanupConfirmed || out.Phase != "finished" || f.removes != 1 {
		t.Fatal("static source retirement not positively settled", out, err)
	}
	previewRetained(t, a, 0)
}

func TestReviewedSourceLostCreateRetainsUnknownLeaseWithoutReplay(t *testing.T) {
	a, f, req := previewRuntimeFixture(t)
	a.deps.Backend = &sourcePreviewBackendFixture{f}
	req.Kind = previewruntime.ReviewedSourceKind
	f.createError = errors.New("synthetic lost create acknowledgement")
	pub := func(previewruntime.Outcome) error { return nil }
	out, err := a.startPreview(context.Background(), req, pub)
	if err == nil || out.Phase != "unknown" || !out.ResourcesHeld || out.BuildAllocationID != "" || out.AllocationID == "" || out.ContainerID != "" || f.creates != 1 || f.starts != 0 || f.builds != 0 {
		t.Fatal(out, err)
	}
	previewRetained(t, a, 1)
	a.execution = newExecutionAdmission(a)
	if _, err := a.stopPreview(context.Background(), out.Identity, pub); err == nil || f.removes != 0 {
		t.Fatal("unknown source Create was settled without an exact CID")
	}
	if _, err := a.startPreview(context.Background(), previewruntime.Request{Identity: out.Identity, Actor: req.Actor, ValidateSource: req.ValidateSource}, pub); err == nil || f.creates != 1 {
		t.Fatal("retained source allocation replayed Create")
	}
	previewRetained(t, a, 1)
}

func TestReviewedSourceCancellationBeforeStartPreservesExactCleanup(t *testing.T) {
	a, f, req := previewRuntimeFixture(t)
	a.deps.Backend = &sourcePreviewBackendFixture{f}
	req.Kind = previewruntime.ReviewedSourceKind
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	out, err := a.startPreview(ctx, req, func(value previewruntime.Outcome) error {
		if value.Phase == "created" {
			cancel()
		}
		return nil
	})
	if err == nil || out.ContainerID == "" || !out.ResourcesHeld || f.starts != 0 || f.creates != 1 || f.builds != 0 {
		t.Fatal("cancelled source allocation lost exact retained ownership", out, err)
	}
	previewRetained(t, a, 1)
	out, err = a.stopPreview(context.Background(), out.Identity, func(previewruntime.Outcome) error { return nil })
	if err != nil || !out.CleanupConfirmed || out.ResourcesHeld || f.removes != 1 {
		t.Fatal(out, err)
	}
	previewRetained(t, a, 0)
}

func TestReviewedSourceRefusesEnvironmentDockerfileAndStaleAuthority(t *testing.T) {
	for _, name := range []string{"environment", "dockerfile", "source", "kind"} {
		t.Run(name, func(t *testing.T) {
			a, f, req := previewRuntimeFixture(t)
			a.deps.Backend = &sourcePreviewBackendFixture{f}
			req.Kind = previewruntime.ReviewedSourceKind
			switch name {
			case "environment":
				req.Environment = map[string]string{"SYNTHETIC": "private"}
			case "dockerfile":
				req.Dockerfile = "Dockerfile"
			case "source":
				req.ValidateSource = func(context.Context) error { return errors.New("reviewed commit changed") }
			case "kind":
				req.Kind = "unknown"
			}
			if _, err := a.startPreview(context.Background(), req, func(previewruntime.Outcome) error { return nil }); err == nil {
				t.Fatal("unsupported/stale source request accepted")
			}
			if f.creates+f.starts+f.builds != 0 {
				t.Fatal("refused source request allocated resources")
			}
			previewRetained(t, a, 0)
		})
	}
}

func TestReviewedSourceWaitingAdmissionHasNoPhantomOwnedAllocation(t *testing.T) {
	a, f, req := previewRuntimeFixture(t)
	a.deps.Backend = &sourcePreviewBackendFixture{f}
	req.Kind = previewruntime.ReviewedSourceKind
	a.deps.ResourceSnapshot = nil
	out, err := a.startPreview(context.Background(), req, func(previewruntime.Outcome) error { return nil })
	if err != nil || out.Phase != "blocked" || out.Retained() || out.ResourcesHeld || f.creates+f.starts+f.builds != 0 {
		t.Fatal("waiting source admission invented an owned allocation", out, err)
	}
	previewRetained(t, a, 0)
}
