package api

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"

	"github.com/lcfrai/luci/control-plane/internal/backend"
	"github.com/lcfrai/luci/control-plane/internal/previewruntime"
)

type sourcePreparationHookFixture struct {
	*sourcePreviewBackendFixture
	afterPrepare func()
}

func (f *sourcePreparationHookFixture) PreparePreview(ctx context.Context, spec backend.PreviewSpec) (backend.PreparedPreview, error) {
	prepared, err := f.sourcePreviewBackendFixture.PreparePreview(ctx, spec)
	if err == nil && f.afterPrepare != nil {
		f.afterPrepare()
	}
	return prepared, err
}

func TestSourceFirstAdmissionRefusalAfterPrepareDoesNotRetainPhantomAllocation(t *testing.T) {
	for _, drift := range []string{"source", "authority"} {
		t.Run(drift, func(t *testing.T) {
			a, f, req := previewRuntimeFixture(t)
			req.Kind = previewruntime.ReviewedSourceKind
			ledgerPath := a.execution.path
			lockPath := filepath.Join(filepath.Dir(ledgerPath), "leases.lock")
			for _, path := range []string{ledgerPath, lockPath} {
				if _, err := os.Lstat(path); !os.IsNotExist(err) {
					t.Fatalf("fresh fixture reservation path %q is not absent: %v", path, err)
				}
			}
			var sourceOK, authorityOK atomic.Bool
			sourceOK.Store(true)
			authorityOK.Store(true)
			a.deps.ExecutionReady = authorityOK.Load
			req.ValidateSource = func(context.Context) error {
				if !sourceOK.Load() {
					return errors.New("synthetic reviewed source changed")
				}
				return nil
			}
			a.deps.Backend = &sourcePreparationHookFixture{sourcePreviewBackendFixture: &sourcePreviewBackendFixture{f}, afterPrepare: func() {
				if drift == "source" {
					sourceOK.Store(false)
				} else {
					authorityOK.Store(false)
				}
			}}
			var published previewruntime.Outcome
			out, err := a.startPreview(context.Background(), req, func(value previewruntime.Outcome) error { published = value; return nil })
			if err == nil || out.Phase != "blocked" || out.ReasonCode != "preview_admission_refused" || out.Retained() || out.ResourcesHeld || published.Retained() || published.ResourcesHeld || f.creates+f.starts+f.builds != 0 {
				t.Fatal("proven pre-reservation refusal retained a phantom allocation", out, err)
			}
			if a.execution.path != ledgerPath {
				t.Fatal("pre-reservation refusal changed the exact reservation store")
			}
			for _, path := range []string{ledgerPath, lockPath} {
				if _, err := os.Lstat(path); !os.IsNotExist(err) {
					t.Fatalf("pre-reservation refusal created reservation state %q: %v", path, err)
				}
			}
			// A never-created store is unavailable, not an observed zero total.
			observed := a.execution.reservationObservation(context.Background())
			if observed.Status != "unavailable" || observed.executionReservationView != nil {
				t.Fatalf("absent reservation store became a numeric observation: %+v", observed)
			}
		})
	}
}

func TestSourceReadmissionRefusalPreservesAlreadyCommittedReservation(t *testing.T) {
	for _, drift := range []string{"source", "authority"} {
		t.Run(drift, func(t *testing.T) {
			a, f, req := previewRuntimeFixture(t)
			req.Kind = previewruntime.ReviewedSourceKind
			a.deps.Backend = &sourcePreviewBackendFixture{f}
			var sourceOK, authorityOK atomic.Bool
			sourceOK.Store(true)
			authorityOK.Store(true)
			a.deps.ExecutionReady = authorityOK.Load
			req.ValidateSource = func(context.Context) error {
				if !sourceOK.Load() {
					return errors.New("synthetic source changed after reservation")
				}
				return nil
			}
			out, err := a.startPreview(context.Background(), req, func(value previewruntime.Outcome) error {
				if value.Phase == "reserved" {
					if drift == "source" {
						sourceOK.Store(false)
					} else {
						authorityOK.Store(false)
					}
				}
				return nil
			})
			if err == nil || !out.Retained() || !out.ResourcesHeld || out.AllocationID == "" || f.creates+f.starts+f.builds != 0 {
				t.Fatal("readmission refusal dropped an existing exact reservation", out, err)
			}
			previewRetained(t, a, 1)
			sourceOK.Store(true)
			authorityOK.Store(true)
			out, err = a.stopPreview(context.Background(), out.Identity, func(previewruntime.Outcome) error { return nil })
			if err != nil || out.ResourcesHeld || !out.CleanupConfirmed || f.removes != 0 {
				t.Fatal("known pre-create reservation could not settle normally", out, err)
			}
			previewRetained(t, a, 0)
		})
	}
}

func TestSourceReservationAttemptErrorRetainsUncertaintyWithoutErrorTextInference(t *testing.T) {
	a, f, req := previewRuntimeFixture(t)
	req.Kind = previewruntime.ReviewedSourceKind
	originalPath := a.execution.path
	blocker := filepath.Join(t.TempDir(), "synthetic-ledger-blocker")
	if err := os.WriteFile(blocker, []byte("inert fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	a.deps.Backend = &sourcePreparationHookFixture{sourcePreviewBackendFixture: &sourcePreviewBackendFixture{f}, afterPrepare: func() { a.execution.path = filepath.Join(blocker, "leases.json") }}
	out, err := a.startPreview(context.Background(), req, func(previewruntime.Outcome) error { return nil })
	a.execution.path = originalPath
	if err == nil || out.Phase != "unknown" || out.ReasonCode != "preview_reservation_unconfirmed" || !out.Retained() || !out.ResourcesHeld || out.ContainerID != "" || f.creates+f.starts+f.builds != 0 {
		t.Fatal("reservation-attempt failure was inferred to be settled or unallocated", out, err)
	}
}
