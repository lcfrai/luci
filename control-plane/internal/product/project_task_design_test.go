package product

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/lcfrai/luci/control-plane/internal/assistance"
	"github.com/lcfrai/luci/control-plane/internal/registry"
)

func taskFramesFixture() (journeyState, *registry.ImplementationContext) {
	state, binding := taskBriefFixture()
	visual := designSceneFixture()
	state.DesignPlan.Screens = visual.Screens
	state.DesignPlan.Scene = visual.Scene
	other := visual.Scene.Frames[0]
	other.ID, other.Name, other.Nodes = "frame-unassigned", "Unassigned screen", []DesignSceneNode{}
	state.DesignPlan.Scene.Frames = append(state.DesignPlan.Scene.Frames, other)
	state.DesignPlan.Digest = designPlanContentDigest(*state.DesignPlan)
	state.approvalAuthority.Design.ContentDigest = state.DesignPlan.Digest
	state.Plan.Tasks[1].DesignFrameIDs = []string{"frame-home"}
	state.Plan.Tasks[3].DesignFrameIDs = []string{"frame-unassigned"}
	state.Plan.DesignPlanRevision, state.Plan.DesignPlanDigest = state.DesignPlan.Revision, state.DesignPlan.Digest
	binding.DesignPlanDigest, binding.PlanDigest = state.DesignPlan.Digest, implementationDigest(*state.Plan)
	return state, binding
}

func TestTaskDesignFramesRemainOptionalAndEnterGeneratedSchema(t *testing.T) {
	state, _ := taskBriefFixture()
	legacy, _ := json.Marshal(*state.Plan)
	if strings.Contains(string(legacy), "designFrameIds") || strings.Contains(string(legacy), "designPlanRevision") || strings.Contains(string(legacy), "designPlanDigest") {
		t.Fatal("unlinked legacy plan gained serialized fields")
	}
	if err := validatePlan(*state.Plan); err != nil {
		t.Fatal("backend tasks acquired a visual requirement", err)
	}
	state.Plan.Tasks[0].DesignFrameIDs = []string{"frame-home"}
	copy := snapshotProjectTasks(state.Plan.Tasks)
	copy[0].DesignFrameIDs[0] = "changed"
	if state.Plan.Tasks[0].DesignFrameIDs[0] != "frame-home" {
		t.Fatal("task snapshot aliases frame selections")
	}
	raw, err := assistance.StrictSchema(ProjectTask{})
	if err != nil {
		t.Fatal(err)
	}
	var schema struct {
		Properties map[string]struct {
			Type string `json:"type"`
		} `json:"properties"`
	}
	if json.Unmarshal(raw, &schema) != nil || schema.Properties["designFrameIds"].Type != "array" {
		t.Fatal("typed model schema omitted the task field")
	}
}

func TestTaskDesignFramesRejectInvalidAndStaleReferences(t *testing.T) {
	for _, name := range []string{"duplicate", "unknown", "no-scene", "draft", "revision", "digest"} {
		t.Run(name, func(t *testing.T) {
			state, _ := taskFramesFixture()
			design := *state.DesignPlan
			switch name {
			case "duplicate":
				state.Plan.Tasks[1].DesignFrameIDs = []string{"frame-home", "frame-home"}
			case "unknown":
				state.Plan.Tasks[1].DesignFrameIDs = []string{"not-a-frame"}
			case "no-scene":
				design.Scene = nil
			case "draft":
				design.Status = "draft"
			case "revision":
				design.Revision++
			case "digest":
				design.Digest = strings.Repeat("e", 64)
			}
			if validateBoundTaskDesignFrames(*state.Plan, design) == nil {
				t.Fatal("invalid frame binding accepted")
			}
		})
	}
}

func TestTaskDesignFramesBindOnlyPrivateCurrentApproval(t *testing.T) {
	state, _ := taskFramesFixture()
	revision := state.DesignPlan.Revision
	if _, _, err := bindTaskDesignFrames(state, state.Plan.Tasks, nil); err == nil {
		t.Fatal("missing expected design revision accepted")
	}
	got, digest, err := bindTaskDesignFrames(state, state.Plan.Tasks, &revision)
	if err != nil || got != revision || digest != state.DesignPlan.Digest {
		t.Fatal("current approved frame binding lost", err)
	}
	state.approvalAuthority = nil
	if _, _, err := bindTaskDesignFrames(state, state.Plan.Tasks, &revision); err == nil {
		t.Fatal("public approved string substituted for private review")
	}
	if got, digest, err := bindTaskDesignFrames(state, []ProjectTask{{ID: "cli", Title: "CLI", Status: "todo"}}, nil); err != nil || got != 0 || digest != "" {
		t.Fatal("unlinked task required a design frame", err)
	}
}

func TestTaskDesignFrameCaptureKeepsFocusedReferencesAndCompleteHandoff(t *testing.T) {
	state, binding := taskFramesFixture()
	capture, err := captureReviewedContext(state, binding)
	if err != nil {
		t.Fatal(err)
	}
	var focused reviewedTaskBrief
	var full ProjectDesignPlan
	for _, file := range capture.Files {
		if file.Path == "LUCI.md" {
			focused = parsedTaskBrief(t, file.Data)
		}
		if file.Path == ".luci/design-handoff.json" {
			if err := json.Unmarshal(file.Data, &full); err != nil {
				t.Fatal(err)
			}
		}
	}
	if len(focused.DesignFrames) != 1 || focused.DesignFrames[0].ID != "frame-home" || focused.DesignFrames[0].ScreenID != "screen-home" || focused.DesignPlanDigest != binding.DesignPlanDigest || focused.DesignPlanRevision != binding.DesignPlanRevision || len(focused.SelectedTasks[0].DesignFrameIDs) != 1 {
		t.Fatal("focused task contract lost exact design references")
	}
	if full.Scene == nil || len(full.Scene.Frames) != 2 || len(full.Scene.Frames[0].Nodes) == 0 {
		t.Fatal("focusing frames dropped the complete handoff or component geometry")
	}
	state.Plan.Tasks[1].DesignFrameIDs = []string{"frame-unassigned"}
	if _, err := captureReviewedContext(state, binding); err == nil {
		t.Fatal("changed frame selection reused prior captured plan")
	}
}

func TestTaskDesignFrameNewApprovalRequiresExplicitPlanRebinding(t *testing.T) {
	state, binding := taskFramesFixture()
	request := *ImplementationContextSelection(binding)
	if _, err := implementationSnapshot(state, request, binding.WorktreeID); err != nil {
		t.Fatal(err)
	}
	state.DesignPlan.Revision++
	state.DesignPlan.Scene.Frames[0].Name = "Revised approved screen"
	state.DesignPlan.Digest = designPlanContentDigest(*state.DesignPlan)
	state.approvalAuthority.Design.Revision = state.DesignPlan.Revision
	state.approvalAuthority.Design.ContentDigest = state.DesignPlan.Digest
	request.DesignPlanRevision = state.DesignPlan.Revision
	if _, err := implementationSnapshot(state, request, binding.WorktreeID); err == nil {
		t.Fatal("same frame ID silently rebound to a new design")
	}
	state.Plan.DesignPlanRevision, state.Plan.DesignPlanDigest = state.DesignPlan.Revision, state.DesignPlan.Digest
	if _, err := implementationSnapshot(state, request, binding.WorktreeID); err != nil {
		t.Fatal("explicit current plan rebinding refused", err)
	}
}

func TestTaskDesignFrameImportKeepsLinksButRevokesApprovalBinding(t *testing.T) {
	state, _ := taskFramesFixture()
	if err := normalizeJourneyIdentity(&state, "p-imported-frames"); err != nil {
		t.Fatal(err)
	}
	if state.Plan.DesignPlanRevision != 0 || state.Plan.DesignPlanDigest != "" || state.Plan.Tasks[1].DesignFrameIDs[0] != "frame-home" || state.DesignPlan.Status != "draft" {
		t.Fatal("portable frame references retained an external approval")
	}
	if validateBoundTaskDesignFrames(*state.Plan, *state.DesignPlan) == nil {
		t.Fatal("imported task acquired design approval")
	}
}
