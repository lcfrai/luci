package product

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/lcfrai/luci/control-plane/internal/management"
)

func allowBlueprintAccess(context.Context, string, bool) error { return nil }

func TestSavedBlueprintPersistsExactTopologyAndIdempotentCASWithoutDeployment(t *testing.T) {
	s, p, cloud, ctx := azureBlueprintFixture(t)
	request := DeploymentBlueprintSave{IdempotencyKey: "blueprint-first-save", Blueprint: cloud.Foundation.Blueprint}
	view, err := s.SaveDeploymentBlueprint(ctx, p.ID, request, allowBlueprintAccess)
	if err != nil || view.Saved == nil || view.Saved.Revision != 1 || view.SourceVerified || view.EffectsApplied {
		t.Fatalf("save failed: %+v %v", view, err)
	}
	replayed, err := s.SaveDeploymentBlueprint(ctx, p.ID, request, allowBlueprintAccess)
	if err != nil || replayed.Saved.Digest != view.Saved.Digest {
		t.Fatal("exact retry did not read back the same revision", err)
	}
	view.Saved.Blueprint.Services[0].Image = "mutated-caller"
	read, err := s.ReadDeploymentBlueprint(ctx, p.ID, allowBlueprintAccess)
	if err != nil || read.Saved.Blueprint.Services[0].Image != request.Blueprint.Services[0].Image {
		t.Fatal("saved state was not independent of returned values", err)
	}
	request.IdempotencyKey = "competing-first-save"
	if _, err = s.SaveDeploymentBlueprint(ctx, p.ID, request, allowBlueprintAccess); !errors.Is(err, ErrBlueprintConflict) {
		t.Fatal("lost-update CAS was not refused", err)
	}
	if len(s.jobs) != 0 || len(s.plans) != 0 {
		t.Fatal("saving topology entered cloud execution")
	}
}

func TestSavedBlueprintFreshEditGrantRefusesWriteAndReadRevocationWithholdsBody(t *testing.T) {
	s, p, cloud, ctx := azureBlueprintFixture(t)
	calls := 0
	access := func(context.Context, string, bool) error {
		calls++
		if calls == 2 {
			return management.ErrExecutionAuthority
		}
		return nil
	}
	_, err := s.SaveDeploymentBlueprint(ctx, p.ID, DeploymentBlueprintSave{IdempotencyKey: "revoked-before-write", Blueprint: cloud.Foundation.Blueprint}, access)
	if !errors.Is(err, management.ErrExecutionAuthority) {
		t.Fatal("late edit revocation accepted", err)
	}
	view, err := s.ReadDeploymentBlueprint(ctx, p.ID, allowBlueprintAccess)
	if err != nil || view.Saved != nil {
		t.Fatal("refused write persisted", err)
	}
	if _, err = s.SaveDeploymentBlueprint(ctx, p.ID, DeploymentBlueprintSave{IdempotencyKey: "allowed-after-refusal", Blueprint: cloud.Foundation.Blueprint}, allowBlueprintAccess); err != nil {
		t.Fatal(err)
	}
	calls = 0
	if _, err = s.ReadDeploymentBlueprint(ctx, p.ID, access); !errors.Is(err, management.ErrExecutionAuthority) {
		t.Fatal("late read revocation disclosed saved data", err)
	}
}

func TestSavedBlueprintContextChangeKeepsOldRevisionReadableAndRefusesStaleSave(t *testing.T) {
	s, p, cloud, ctx := azureBlueprintFixture(t)
	request := DeploymentBlueprintSave{IdempotencyKey: "before-context-change", Blueprint: cloud.Foundation.Blueprint}
	view, err := s.SaveDeploymentBlueprint(ctx, p.ID, request, allowBlueprintAccess)
	if err != nil {
		t.Fatal(err)
	}
	state, err := s.readApprovedJourney(p.Path, p.ID)
	if err != nil {
		t.Fatal(err)
	}
	state.Brief.Revision++
	state.Brief.Purpose = "Changed project intent while preserving deployment topology"
	if err = writeJourney(p.Path, state); err != nil {
		t.Fatal(err)
	}
	request.ExpectedRevision, request.ExpectedDigest, request.IdempotencyKey = 1, view.Saved.Digest, "after-context-change"
	if _, err = s.SaveDeploymentBlueprint(ctx, p.ID, request, allowBlueprintAccess); !errors.Is(err, ErrBlueprintContextChanged) {
		t.Fatal("stale context saved", err)
	}
	read, err := s.ReadDeploymentBlueprint(ctx, p.ID, allowBlueprintAccess)
	if err != nil || read.Saved.Digest != view.Saved.Digest {
		t.Fatal("context amendment destroyed saved topology", err)
	}
}

func TestSavedBlueprintRejectsForeignProjectInlineCredentialsAndTamperedPrivateLedger(t *testing.T) {
	s, p, cloud, ctx := azureBlueprintFixture(t)
	request := DeploymentBlueprintSave{IdempotencyKey: "invalid-input-save", Blueprint: cloud.Foundation.Blueprint}
	request.Blueprint.ProjectID = "p-foreign"
	if _, err := s.SaveDeploymentBlueprint(ctx, p.ID, request, allowBlueprintAccess); err == nil {
		t.Fatal("foreign blueprint saved")
	}
	request.Blueprint.ProjectID = p.ID
	request.Blueprint.Services[0].Args = []string{"--api-key", "not-a-resource-reference"}
	if _, err := s.SaveDeploymentBlueprint(ctx, p.ID, request, allowBlueprintAccess); err == nil {
		t.Fatal("obvious inline secret flag stored")
	}
	request.Blueprint.Services[0].Args = nil
	if _, err := s.SaveDeploymentBlueprint(ctx, p.ID, request, allowBlueprintAccess); err != nil {
		t.Fatal(err)
	}
	path, ledger, err := s.readDeploymentBlueprintLedger(p)
	if err != nil {
		t.Fatal(err)
	}
	ledger.Revisions[0].Blueprint.Infrastructure.Reference = "changed-without-revision"
	if err = durableRunJSON(path, ledger); err != nil {
		t.Fatal(err)
	}
	if _, err = s.ReadDeploymentBlueprint(ctx, p.ID, allowBlueprintAccess); !errors.Is(err, ErrBlueprintUnavailable) {
		t.Fatal("tampered ledger disclosed", err)
	}
}

func TestSavedBlueprintRollingHistoryKeepsMonotonicCASAndRefusesTrimmedRetries(t *testing.T) {
	s, p, cloud, ctx := azureBlueprintFixture(t)
	first := DeploymentBlueprintSave{IdempotencyKey: "rolling-blueprint-1", Blueprint: cloud.Foundation.Blueprint}
	request := first
	var view DeploymentBlueprintView
	for revision := 1; revision <= 52; revision++ {
		request.IdempotencyKey = fmt.Sprintf("rolling-blueprint-%d", revision)
		var err error
		view, err = s.SaveDeploymentBlueprint(ctx, p.ID, request, allowBlueprintAccess)
		if err != nil || view.Saved.Revision != revision {
			t.Fatalf("revision %d failed: %v", revision, err)
		}
		request.ExpectedRevision, request.ExpectedDigest = view.Saved.Revision, view.Saved.Digest
	}
	_, ledger, err := s.readDeploymentBlueprintLedger(p)
	if err != nil || len(ledger.Revisions) != 50 || ledger.Revisions[0].Revision != 3 || ledger.Revisions[49].Revision != 52 {
		t.Fatal("rolling history reset numbering or exceeded bounds", err)
	}
	if _, err = s.SaveDeploymentBlueprint(ctx, p.ID, first, allowBlueprintAccess); !errors.Is(err, ErrBlueprintConflict) {
		t.Fatal("trimmed old retry overwrote the latest topology", err)
	}
	read, err := s.ReadDeploymentBlueprint(ctx, p.ID, allowBlueprintAccess)
	if err != nil || read.Saved.Revision != 52 {
		t.Fatal("latest revision lost after refused retry", err)
	}
}
