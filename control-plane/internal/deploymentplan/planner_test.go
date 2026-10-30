package deploymentplan

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestManifestPlannerBindsContextRevisionsWithoutClaimingApproval(t *testing.T) {
	b := exampleBlueprint()
	b.Source.Context = &ContextReference{BriefRevision: 1, SpecificationRevision: 3, SpecificationDigest: strings.Repeat("b", 64), DesignRevision: 4}
	first, err := (ManifestPlanner{}).Plan(context.Background(), b)
	if err != nil {
		t.Fatal(err)
	}
	b.Source.Context.DesignRevision++
	second, err := (ManifestPlanner{}).Plan(context.Background(), b)
	if err != nil || first.Digest == second.Digest || second.SourceVerified || second.EffectsApplied {
		t.Fatal("context change did not invalidate the unverified plan")
	}
	b.Source.Context.SpecificationRevision = 0
	if _, err := (ManifestPlanner{}).Plan(context.Background(), b); err == nil {
		t.Fatal("digest without saved revision accepted")
	}
}

func TestManifestPlannerCancellationHasNoApplyFallback(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	plan, err := (ManifestPlanner{}).Plan(ctx, exampleBlueprint())
	if !errors.Is(err, context.Canceled) || len(plan.Artifacts) != 0 {
		t.Fatal("cancelled planning produced artifacts")
	}
	if _, claimsEffects := any(ManifestPlanner{}).(Connector); claimsEffects {
		t.Fatal("plan-only connector advertises unimplemented observation/apply")
	}
}
