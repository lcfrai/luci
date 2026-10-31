package product

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lcfrai/luci/control-plane/internal/management"
	"github.com/lcfrai/luci/control-plane/internal/previewruntime"
)

func runtimePreparationFixture(t *testing.T) (*Server, context.Context, management.ExecutionActorRef, RuntimePreparation) {
	t.Helper()
	s, p, cloud, ctx := azureBlueprintFixture(t)
	saved, err := s.SaveDeploymentBlueprint(ctx, p.ID, DeploymentBlueprintSave{IdempotencyKey: "runtime-template-save", Blueprint: cloud.Foundation.Blueprint}, allowBlueprintAccess)
	if err != nil {
		t.Fatal(err)
	}
	actor, err := management.CaptureDeploymentActor(ctx, p.ID)
	if err != nil {
		t.Fatal(err)
	}
	registry, err := s.SaveRuntimeRegistry(ctx, actor, RuntimeRegistrySave{Name: "Team registry", Repository: "registry.example/team/app", Username: "build-user", Password: "private-value-never-returned"})
	if err != nil {
		t.Fatal(err)
	}
	prepared, err := s.ReviewRuntimePreparation(ctx, actor, RuntimePreparationInput{RequestKey: "original-preparation-review", BlueprintRevision: saved.Saved.Revision, BlueprintDigest: saved.Saved.Digest, RegistryID: registry.ID, RegistryRevision: registry.Revision, Builds: []RuntimeBuildSelection{{ServiceID: "app", Dockerfile: "Dockerfile", MemoryMiB: 512, CPUMillis: 1000}}, MonthlyBudgetUSD: 50, AcknowledgeBuildAndPush: true})
	if err != nil {
		t.Fatal(err)
	}
	return s, ctx, actor, prepared
}
func TestRuntimePreparationRetainsOriginalPrivateConsentAndRegistryRevision(t *testing.T) {
	s, ctx, actor, row := runtimePreparationFixture(t)
	raw, _ := json.Marshal(row)
	if strings.Contains(string(raw), "private-value") || strings.Contains(string(raw), "build-user") {
		t.Fatal("credentials escaped into task/preparation response")
	}
	read, err := s.ReadRuntimePreparation(ctx, actor, row.Plan.ID)
	if err != nil || read.Plan.Digest != row.Plan.Digest || len(read.Builds) != 0 {
		t.Fatal("metadata-only review acquired or replaced work", err)
	}
	list, err := s.ListRuntimePreparations(ctx, actor)
	if err != nil || len(list) != 1 || list[0].ID != row.Plan.ID {
		t.Fatal(list, err)
	}
	reg := row.Plan.Registry
	if _, err = s.SaveRuntimeRegistry(ctx, actor, RuntimeRegistrySave{ID: reg.ID, ExpectedRevision: reg.Revision, Name: reg.Name, Repository: reg.Repository, Username: "build-user", Password: "rotated-private-value"}); err != nil {
		t.Fatal(err)
	}
	_, original, err := s.readRuntimePreparation(actor, row.Plan.ID)
	if err != nil {
		t.Fatal(err)
	}
	if s.runtimePreparationAuthority(ctx, original, true) == nil {
		t.Fatal("credential/descriptor revision rotation reused old consent")
	}
	if _, err = s.ReadRuntimePreparation(ctx, actor, row.Plan.ID); err != nil {
		t.Fatal("retained original history disappeared after setup changed", err)
	}
}
func TestRuntimePreparationMissingOriginalRuntimeReceiptCannotAllocateOrReplay(t *testing.T) {
	s, ctx, actor, row := runtimePreparationFixture(t)
	file, private, err := s.readRuntimePreparation(actor, row.Plan.ID)
	if err != nil {
		t.Fatal(err)
	}
	private.Builds = []RuntimeBuildAttempt{{ServiceID: "app", RunID: "run-original", CheckID: "check-original", Commit: strings.Repeat("a", 40), SourceDigest: strings.Repeat("b", 64), RunInputDigest: strings.Repeat("c", 64), CheckDigest: strings.Repeat("d", 64), Attempted: true}}
	private.Revision++
	if err = durableRunJSON(file, private); err != nil {
		t.Fatal(err)
	}
	calls := 0
	s.SetPreviewRuntime(previewruntime.Runtime{BuildAndPublish: func(context.Context, previewruntime.Request, previewruntime.Publication, func(previewruntime.Outcome) error) (previewruntime.Outcome, error) {
		calls++
		return previewruntime.Outcome{}, nil
	}})
	if _, err = s.ObserveRuntimePreparationService(ctx, actor, row.Plan.ID, "app"); err == nil {
		t.Fatal("missing original receipt was adopted")
	}
	if _, err = s.BuildRuntimePreparationService(ctx, actor, row.Plan.ID, "app", "different-run", "different-check"); err == nil {
		t.Fatal("retained attempt replayed under replacement source")
	}
	if calls != 0 {
		t.Fatal("new allocation after original attempt marker", calls)
	}
	_, after, err := s.readRuntimePreparation(actor, row.Plan.ID)
	if err != nil || after.Revision != private.Revision {
		t.Fatal("unknown original was rewritten", err)
	}
}
func TestRuntimePrerequisiteTaskProjectionPreservesLegacyClearAndExactAssignment(t *testing.T) {
	content := "export const ready = true;\n"
	sum := sha256.Sum256([]byte(content))
	refs := []TaskRuntimePrerequisite{{PreparationID: "prep-" + strings.Repeat("a", 32), PreparationDigest: strings.Repeat("b", 64), ServiceID: "app", Kind: "instrumentation", Artifacts: []TaskRuntimeArtifact{{Path: "telemetry/app.mjs", Content: content, SHA256: hex.EncodeToString(sum[:])}}, Dependencies: map[string]string{"@opentelemetry/api": "1.9.0"}}}
	if err := validateTaskRuntimePrerequisites(&refs); err != nil {
		t.Fatal(err)
	}
	old := []ProjectTask{{ID: "task-app", Title: "Task", Status: "todo", RuntimePrerequisites: &refs}}
	inherited := preserveTaskRuntimePrerequisites(old, []ProjectTask{{ID: "task-app", Title: "Renamed", Status: "todo"}})
	if inherited[0].RuntimePrerequisites == nil {
		t.Fatal("old client erased runtime inputs")
	}
	(*inherited[0].RuntimePrerequisites)[0].Artifacts[0].Content = "modified"
	if refs[0].Artifacts[0].Content != content {
		t.Fatal("snapshot aliases captured source")
	}
	if validateAssistedRuntimePrerequisites(old, inherited) == nil {
		t.Fatal("assistant invented artifact content")
	}
	moved := snapshotProjectTasks(old)
	moved[0].ID = "different-task"
	if validateAssistedRuntimePrerequisites(old, moved) == nil {
		t.Fatal("assistant reassigned exact prerequisite to different task")
	}
	empty := []TaskRuntimePrerequisite{}
	cleared := preserveTaskRuntimePrerequisites(old, []ProjectTask{{ID: "task-app", RuntimePrerequisites: &empty}})
	raw, _ := json.Marshal(cleared[0])
	if !strings.Contains(string(raw), "\"runtimePrerequisites\":[]") {
		t.Fatal("explicit clear lost", string(raw))
	}
	raw, _ = json.Marshal(ProjectTask{ID: "legacy"})
	if strings.Contains(string(raw), "runtimePrerequisites") {
		t.Fatal("legacy bytes changed")
	}
}
func TestRuntimeBuildVerifiesCommittedArtifactAndExactProductionDependency(t *testing.T) {
	root := t.TempDir()
	content := "export const sdk = true;\n"
	sum := sha256.Sum256([]byte(content))
	build := RuntimePreparedBuild{Selection: RuntimeBuildSelection{Dockerfile: "Dockerfile", PackageJSON: "package.json"}, Artifacts: []TaskRuntimeArtifact{{Path: "sdk.mjs", Content: content, SHA256: hex.EncodeToString(sum[:])}}, Dependencies: map[string]string{"@opentelemetry/api": "1.9.0"}}
	for name, raw := range map[string]string{"Dockerfile": "FROM already-reviewed\n", "sdk.mjs": content, "package.json": `{"dependencies":{"@opentelemetry/api":"1.9.0"}}`} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(raw), 0600); err != nil {
			t.Fatal(err)
		}
	}
	if err := verifyRuntimeBuildFiles(root, build); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "sdk.mjs"), []byte(content+"// modified\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if verifyRuntimeBuildFiles(root, build) == nil {
		t.Fatal("a task being marked done replaced exact committed file verification")
	}
	if err := os.WriteFile(filepath.Join(root, "sdk.mjs"), []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "package.json"), []byte(`{"dependencies":{"@opentelemetry/api":"^1.9.0"}}`), 0600); err != nil {
		t.Fatal(err)
	}
	if verifyRuntimeBuildFiles(root, build) == nil {
		t.Fatal("mutable dependency declaration accepted")
	}
}
func TestRuntimeDerivedBlueprintRequiresPositiveOriginalImagesAndPreservesTemplate(t *testing.T) {
	s, _, actor, row := runtimePreparationFixture(t)
	_, private, err := s.readRuntimePreparation(actor, row.Plan.ID)
	if err != nil {
		t.Fatal(err)
	}
	before := reviewDigest(private.Template)
	identity := previewruntime.Identity{ProjectID: actor.ProjectID, ReleaseID: row.Plan.ID + "-app", Target: "native-docker", Commit: strings.Repeat("c", 40), SourceDigest: strings.Repeat("d", 64)}
	private.Builds = []RuntimeBuildAttempt{{ServiceID: "app", RunInputDigest: strings.Repeat("e", 64), CheckDigest: strings.Repeat("f", 64), Commit: identity.Commit, Runtime: &previewruntime.Outcome{Identity: identity, Phase: "unknown", ResourcesHeld: true}}}
	if _, err = runtimePreparationDerivedBlueprint(private); err == nil {
		t.Fatal("unknown push became a usable image")
	}
	private.Builds[0].Runtime = &previewruntime.Outcome{Identity: identity, Phase: "published", ImmutableImage: row.Plan.Registry.Repository + "@sha256:" + strings.Repeat("a", 64), CleanupConfirmed: true}
	derived, err := runtimePreparationDerivedBlueprint(private)
	if err != nil {
		t.Fatal(err)
	}
	if derived.Source.Commit != identity.Commit || derived.Services[0].Image != private.Builds[0].Runtime.ImmutableImage || reviewDigest(private.Template) != before {
		t.Fatal("derived image rewrote original consent/template")
	}
	derived.Services[0].Port++
	if private.Template.Services[0].Port == derived.Services[0].Port {
		t.Fatal("derived topology aliases source")
	}
}

func TestRuntimePreparationAlwaysIncludesPackagingAndIndependentReviewerScopes(t *testing.T) {
	_, _, _, row := runtimePreparationFixture(t)
	if len(row.SuggestedTasks) != 2 {
		t.Fatal("non-instrumented build had no real packaging/reviewer tasks", row.SuggestedTasks)
	}
	coding, review := row.SuggestedTasks[0], row.SuggestedTasks[1]
	if coding.Role != "coding" || len(coding.WritePaths) != 1 || coding.WritePaths[0] != "Dockerfile" || coding.RuntimePrerequisites != nil || !strings.Contains(coding.Acceptance, "actual application") {
		t.Fatal("packaging scope missing or invented SDK requirement", coding)
	}
	if review.Role != "reviewer" || review.ReviewOfTaskID != coding.ID || len(review.WritePaths) != 0 {
		t.Fatal("independent review assignment was not preserved", review)
	}
}
