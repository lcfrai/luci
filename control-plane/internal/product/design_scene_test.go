package product

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/lcfrai/luci/control-plane/internal/assistance"
)

func designSceneFixture() ProjectDesignPlan {
	return ProjectDesignPlan{ProjectID: "p-scene", Screens: []DesignScreen{{ID: "screen-home", Name: "Home", Purpose: "Save work", States: []string{"Empty"}, Widths: []int{390}, RequirementIDs: []string{}}}, Tokens: []DesignToken{}, Assets: []DesignAsset{}, Status: "draft", Scene: &ProjectDesignScene{Version: 1, Frames: []DesignSceneFrame{{ID: "frame-home", Name: "Home phone", ScreenID: "screen-home", State: "Empty", Width: 390, Height: 844, Background: "#ffffff", Nodes: []DesignSceneNode{{ID: "node-title", Kind: "text", Name: "Title", X: 24, Y: 24, Width: 300, Height: 48, Text: "Saved work", Style: DesignSceneStyle{Fill: "transparent", Stroke: "transparent", Color: "#18181b", FontSize: 24, FontWeight: 600, FontFamily: "system", TextAlign: "left", Opacity: 100}, Behavior: DesignSceneBehavior{Kind: "none"}}}}}}}
}
func TestDesignSceneBoundedScalarAndReferenceValidation(t *testing.T) {
	for name, change := range map[string]func(*ProjectDesignPlan){
		"duplicate ID":   func(p *ProjectDesignPlan) { p.Scene.Frames[0].Nodes[0].ID = p.Scene.Frames[0].ID },
		"missing screen": func(p *ProjectDesignPlan) { p.Scene.Frames[0].ScreenID = "removed" },
		"missing state":  func(p *ProjectDesignPlan) { p.Scene.Frames[0].State = "Removed state" },
		"outside frame":  func(p *ProjectDesignPlan) { p.Scene.Frames[0].Nodes[0].X = 389 },
		"script style":   func(p *ProjectDesignPlan) { p.Scene.Frames[0].Nodes[0].Style.Fill = "url(javascript:alert(1))" },
		"opacity":        func(p *ProjectDesignPlan) { p.Scene.Frames[0].Nodes[0].Style.Opacity = 101 },
		"navigation URL": func(p *ProjectDesignPlan) {
			p.Scene.Frames[0].Nodes[0].Behavior = DesignSceneBehavior{Kind: "navigate", TargetFrameID: "https://example.invalid"}
		},
		"unbound image": func(p *ProjectDesignPlan) {
			p.Scene.Frames[0].Nodes[0].Kind = "image"
			p.Scene.Frames[0].Nodes[0].AssetID = "asset-image"
		},
		"oversized text": func(p *ProjectDesignPlan) { p.Scene.Frames[0].Nodes[0].Text = strings.Repeat("é", 2001) },
	} {
		t.Run(name, func(t *testing.T) {
			plan := designSceneFixture()
			change(&plan)
			if validateDesignPlan(plan) == nil {
				t.Fatal("invalid scene accepted")
			}
		})
	}
	plan := designSceneFixture()
	plan.Scene.Frames[0].Nodes[0].Text = "<script>alert('plain text only')</script>"
	if err := validateDesignPlan(plan); err != nil {
		t.Fatal("plain text should remain data", err)
	}
	plan.Scene.Frames = []DesignSceneFrame{}
	if err := validateDesignPlan(plan); err != nil {
		t.Fatal("explicit empty draft must remain valid", err)
	}
}
func TestDesignSceneAssistanceSchemaUsesIntegerWire(t *testing.T) {
	raw, err := assistance.StrictSchema(DesignSceneStyle{})
	if err != nil {
		t.Fatal(err)
	}
	var schema struct {
		Properties map[string]struct {
			Type string `json:"type"`
		} `json:"properties"`
	}
	if json.Unmarshal(raw, &schema) != nil {
		t.Fatal("schema unavailable")
	}
	for _, key := range []string{"strokeWidth", "radius", "fontSize", "fontWeight", "padding", "opacity"} {
		if schema.Properties[key].Type != "integer" {
			t.Fatal("wire scalar diverged", key, string(raw))
		}
	}
	for _, value := range []any{DesignSceneFrame{}, DesignSceneNode{}} {
		raw, _ = assistance.StrictSchema(value)
		if json.Unmarshal(raw, &schema) != nil || schema.Properties["width"].Type != "integer" || schema.Properties["height"].Type != "integer" {
			t.Fatal("geometry is not integer wire", string(raw))
		}
	}
	if json.Unmarshal([]byte(`{"version":1,"frames":[{"width":390.5}]}`), new(ProjectDesignScene)) == nil {
		t.Fatal("fractional wire geometry accepted")
	}
}
func TestDesignSceneLegacyOmissionAndAgentPreservation(t *testing.T) {
	old := designSceneFixture()
	old.Scene = nil
	raw, _ := json.Marshal(old)
	if strings.Contains(string(raw), `"scene"`) || strings.Contains(string(raw), `"content"`) {
		t.Fatal("legacy nil changed serialized fields", string(raw))
	}
	next := old
	if designSceneOmission(old, next) != nil {
		t.Fatal("legacy metadata-only amendment refused")
	}
	old = designSceneFixture()
	next = old
	next.Scene = nil
	if designSceneOmission(old, next) == nil {
		t.Fatal("present scene silently removed")
	}
	snapshot := OrchestratorContext{ProjectID: old.ProjectID, DesignPlan: old}
	answer := OrchestratorAnswer{Summary: "Amend the handoff", Proposal: &OrchestratorProposal{Target: "design-plan", Summary: "Change notes", DesignPlan: &next}}
	if ValidateOrchestratorAnswer(snapshot, answer) == nil {
		t.Fatal("current agent proposal dropped scene")
	}
	if validateStoredOrchestratorAnswer(snapshot, answer) != nil {
		t.Fatal("immutable old answer became unreadable")
	}
}
func TestDesignSceneUsesExistingCASApprovalHistoryAndCapture(t *testing.T) {
	_, mux := journeyFixture(t)
	p := createJourneyProject(t, mux, "typed-scene")
	base := "/v1/product/projects/" + p.ID
	spec := specificationFromBrief(Brief{ProjectID: p.ID, Purpose: "Saved work", Audience: "People"})
	spec.Requirements = []SpecificationRequirement{{ID: "AC-save", Title: "Save work", Priority: "must", Acceptance: "Saved work survives reopening"}}
	journeyRequest(t, mux, "POST", base+"/specification", map[string]any{"expectedRevision": 0, "expectedBriefRevision": 0, "specification": spec}, 200)
	journeyRequest(t, mux, "POST", base+"/specification/approve", map[string]int{"revision": 1, "briefRevision": 1}, 200)
	plan := designSceneFixture()
	plan.ProjectID = p.ID
	body := func(value ProjectDesignPlan, revision int) map[string]any {
		return map[string]any{"expectedRevision": revision, "expectedBriefRevision": 1, "expectedSpecificationRevision": 2, "plan": value}
	}
	decodeEvidenceResponse(t, journeyRequest(t, mux, "POST", base+"/design-plan", body(plan, 0), 200).Body.Bytes(), &plan)
	journeyRequest(t, mux, "POST", base+"/design-plan", body(plan, 0), 409)
	decodeEvidenceResponse(t, journeyRequest(t, mux, "POST", base+"/design-plan/approve", map[string]int{"revision": 1}, 200).Body.Bytes(), &plan)
	approvedDigest := plan.Digest
	if plan.Status != "approved" || len(approvedDigest) != 64 || !strings.Contains(string(renderDesignPlan(plan)), "node-title") {
		t.Fatal("scene not in approved handoff", plan)
	}
	next := plan
	next.Scene = nil
	journeyRequest(t, mux, "POST", base+"/design-plan", body(next, plan.Revision), 400)
	var nextScene ProjectDesignScene
	raw, _ := json.Marshal(plan.Scene)
	json.Unmarshal(raw, &nextScene)
	next.Scene = &nextScene
	next.Scene.Frames[0].Nodes[0].Text = "Changed scene"
	decodeEvidenceResponse(t, journeyRequest(t, mux, "POST", base+"/design-plan", body(next, plan.Revision), 200).Body.Bytes(), &next)
	if next.Status != "draft" || next.Digest != "" || plan.Digest != approvedDigest || plan.Scene.Frames[0].Nodes[0].Text != "Saved work" {
		t.Fatal("scene amendment changed approved snapshot")
	}
	var history struct {
		Revisions []ProjectDesignPlan `json:"revisions"`
	}
	decodeEvidenceResponse(t, journeyRequest(t, mux, "GET", base+"/design-plan/history", nil, 200).Body.Bytes(), &history)
	if len(history.Revisions) < 3 || history.Revisions[1].Digest != approvedDigest || history.Revisions[1].Scene.Frames[0].Nodes[0].Text != "Saved work" {
		t.Fatal("scene history lost exact approval", history)
	}
}
