package registry

import (
	"path/filepath"
	"reflect"
	"testing"
)

func TestImplementationContextSurvivesReopenWithoutSharingMutableInputs(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sessions.json")
	want := &ImplementationContext{ProjectID: "p-8c1d0e42", WorktreeID: "w-4a77b1c9", BriefRevision: 2, BriefDigest: "brief-start", PlanRevision: 3, PlanDigest: "plan-start", SpecificationRevision: 4, SpecificationDigest: "spec-start", DesignPlanRevision: 5, DesignPlanDigest: "design-start", TaskIDs: []string{"task-one", "task-two"}, TaskTitles: []string{"Build", "Verify"}, RequirementIDs: []string{"requirement-one", "requirement-two"}, SourceCommit: "commit-at-start", SourceDigest: "source-at-start", SourceDirty: true}
	sess := sampleSession()
	sess.ImplementationContext = want.Clone()
	store := newStore(t, path)
	if err := store.Create(sess); err != nil {
		t.Fatal(err)
	}
	// The original request remains caller-owned after Create.
	sess.ImplementationContext.TaskIDs[0] = "caller-changed"
	sess.ImplementationContext.TaskTitles[0] = "caller-changed"
	sess.ImplementationContext.RequirementIDs[0] = "caller-changed"
	sess.ImplementationContext.SourceDigest = "caller-changed"
	got, err := store.Get(sess.ID)
	if err != nil || !reflect.DeepEqual(got.ImplementationContext, want) {
		t.Fatalf("stored input changed after caller mutation: %v, %v", got.ImplementationContext, err)
	}
	// A UI/read consumer must not be able to mutate the private recorded inputs.
	got.ImplementationContext.TaskIDs[0] = "reader-changed"
	got.ImplementationContext.TaskTitles[0] = "reader-changed"
	got.ImplementationContext.RequirementIDs[0] = "reader-changed"
	got.ImplementationContext.PlanDigest = "reader-changed"
	reopened := newStore(t, path)
	got, err = reopened.Get(sess.ID)
	if err != nil || !reflect.DeepEqual(got.ImplementationContext, want) {
		t.Fatalf("reviewed inputs lost on reopen: %v, %v", got.ImplementationContext, err)
	}
	// Ordinary lifecycle changes preserve the original source snapshot.
	if _, err := reopened.Update(sess.ID, func(row *Session) error { row.Status = StatusExited; return nil }); err != nil {
		t.Fatal(err)
	}
	got, err = newStore(t, path).Get(sess.ID)
	if err != nil || got.Status != StatusExited || !reflect.DeepEqual(got.ImplementationContext, want) {
		t.Fatalf("lifecycle update changed reviewed inputs: %v, %v", got, err)
	}
}

func TestReviewedContextReceiptIsPrivateDurableAndIndependentlyCloned(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sessions.json")
	sess := sampleSession()
	want := &ReviewedContext{Version: 1, Digest: "captured-manifest-digest"}
	sess.ReviewedContext = want.Clone()
	store := newStore(t, path)
	if err := store.Create(sess); err != nil {
		t.Fatal(err)
	}
	sess.ReviewedContext.Digest = "caller-mutation"
	got, err := store.Get(sess.ID)
	if err != nil || !reflect.DeepEqual(got.ReviewedContext, want) {
		t.Fatal("stored reviewed receipt aliases the caller", err)
	}
	got.ReviewedContext.Version = 99
	reopened := newStore(t, path)
	got, err = reopened.Get(sess.ID)
	if err != nil || !reflect.DeepEqual(got.ReviewedContext, want) {
		t.Fatal("reviewed receipt did not survive reopen independently", err)
	}
	if _, err = reopened.Update(sess.ID, func(s *Session) error { s.Status = StatusExited; return nil }); err != nil {
		t.Fatal(err)
	}
	got, err = newStore(t, path).Get(sess.ID)
	if err != nil || got.Status != StatusExited || !reflect.DeepEqual(got.ReviewedContext, want) {
		t.Fatal("terminal state discarded original delivery evidence", err)
	}
}
