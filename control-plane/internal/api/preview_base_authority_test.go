package api

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/lcfrai/luci/control-plane/internal/backend"
	"github.com/lcfrai/luci/control-plane/internal/management"
	"github.com/lcfrai/luci/control-plane/internal/previewruntime"
)

func TestPreviewBaseMutationsRecheckAuthoritySourceAndExactCharge(t *testing.T) {
	for _, mode := range []string{"unchanged", "revoked", "policy-reduced", "changed-source", "cancelled", "changed-lease", "unacknowledged-pull"} {
		t.Run(mode, func(t *testing.T) {
			a, f, req := previewRuntimeFixture(t)
			revoked, changedSource, reduced := false, false, false
			if mode == "policy-reduced" {
				req.Actor = management.ExecutionActorRef{UserID: "owner", ProjectID: req.ProjectID}
			}
			a.deps.AuthorizeDeployment = func(context.Context, management.ExecutionActorRef, bool) (management.ExecutionAuthority, error) {
				if revoked {
					return management.ExecutionAuthority{}, management.ErrExecutionAuthority
				}
				policy := management.Policy{CheckMemoryMiB: 1024, CheckCPUMillis: 1000, MaxConcurrentChecksPerUser: 1}
				if reduced {
					policy.CheckMemoryMiB = 512
				}
				return management.ExecutionAuthority{Policy: policy}, nil
			}
			req.ValidateSource = func(context.Context) error {
				if changedSource {
					return errors.New("reviewed source changed after the first base")
				}
				return nil
			}
			pulls, builds := 0, 0
			f.guardedBuild = func(ctx context.Context, p backend.PreparedPreview, guard func(context.Context) error) (backend.PreviewBuildResult, error) {
				if guard == nil {
					t.Fatal("no coordinator mutation guard")
				}
				if err := guard(ctx); err != nil {
					t.Fatal("first admitted base refused", err)
				}
				pulls++
				first, err := a.execution.previewLease(ctx, p.Spec.Identity.AllocationID)
				if err != nil || first.State != "building" {
					t.Fatal("base pull did not retain the exact active build", first, err)
				}
				previewRetained(t, a, 1)
				if mode == "unacknowledged-pull" {
					return backend.PreviewBuildResult{}, errors.New("first pull lost its final acknowledgement")
				}
				revoked, changedSource = mode == "revoked", mode == "changed-source"
				reduced = mode == "policy-reduced"
				checkCtx := ctx
				if mode == "cancelled" {
					var cancel context.CancelFunc
					checkCtx, cancel = context.WithCancel(ctx)
					cancel()
				}
				if mode == "changed-lease" {
					if err := a.execution.mutate(ctx, func(ledger *executionLeaseLedger) error {
						for i := range ledger.Previews {
							if ledger.Previews[i].ID == first.ID {
								ledger.Previews[i].Revision++
							}
						}
						return nil
					}); err != nil {
						t.Fatal(err)
					}
				}
				before, err := a.execution.previewLease(ctx, first.ID)
				if err != nil {
					t.Fatal(err)
				}
				err = guard(checkCtx)
				after, readErr := a.execution.previewLease(ctx, first.ID)
				if readErr != nil || after != before {
					t.Fatal("mutation guard altered the held reservation", readErr)
				}
				if mode != "unchanged" {
					if err == nil {
						t.Fatal("second base crossed an invalid authority/source/allocation")
					}
					// The first fake pull is explicitly acknowledged; nothing else
					// was issued. A changed durable lease still prevents its release.
					return backend.PreviewBuildResult{Settled: true}, err
				}
				if err != nil {
					t.Fatal(err)
				}
				pulls++
				if err = guard(ctx); err != nil {
					t.Fatal("final build guard", err)
				}
				builds++
				return backend.PreviewBuildResult{Settled: true, ImageID: "sha256:" + strings.Repeat("f", 64)}, nil
			}
			out, err := a.startPreview(context.Background(), req, func(previewruntime.Outcome) error { return nil })
			if mode == "unchanged" {
				if err != nil || out.Phase != "running" || pulls != 2 || builds != 1 || f.creates != 1 || f.starts != 1 {
					t.Fatal("unchanged reviewed build did not progress exactly once", out, err, pulls, builds)
				}
				previewRetained(t, a, 1)
				return
			}
			if err == nil || pulls != 1 || builds != 0 || f.creates != 0 || f.starts != 0 {
				t.Fatal("invalid second operation progressed", out, err, pulls, builds)
			}
			if mode == "changed-lease" || mode == "unacknowledged-pull" {
				previewRetained(t, a, 1)
			} else {
				previewRetained(t, a, 0)
				if out.ResourcesHeld || !out.CleanupConfirmed || out.Phase != "failed" {
					t.Fatal("acknowledged preparation did not settle truthfully", out)
				}
			}
		})
	}
}
