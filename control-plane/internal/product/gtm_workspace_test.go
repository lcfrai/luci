package product

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lcfrai/luci/control-plane/internal/management"
)

func gtmFixtureContent() GTMContent {
	return GTMContent{Positioning: "A focused workspace for local teams", Offer: "A small paid pilot", PricingAssumptions: []string{"Demand is untested"}, Segments: []GTMSegment{{ID: "segment-team", Name: "Local teams", Problem: "Repeated handoffs", Value: "Keep the context", RequirementIDs: []string{}}}, Competitors: []GTMCompetitor{}, Channels: []GTMChannel{{ID: "channel-pilot", Name: "Direct pilot", Hypothesis: "Teams will try a scoped pilot", SegmentIDs: []string{"segment-team"}}}, Milestones: []GTMMilestone{{ID: "milestone-learn", Title: "Learn", Outcome: "Record pilot feedback", DependsOn: []string{}}}, Experiments: []GTMExperiment{{ID: "experiment-pilot", Title: "Pilot interest", Hypothesis: "Teams value continuity", Method: "Ask consenting pilot participants", SuccessCriterion: "Record explicit interest", SegmentID: "segment-team", ChannelID: "channel-pilot", Status: "planned"}}, ArtifactLinks: []GTMArtifactLink{}}
}

func TestGTMValidatesStableLinksAndBoundedDrafts(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mutate func(*GTMContent)
	}{
		{"duplicate identity", func(c *GTMContent) { c.Channels[0].ID = c.Segments[0].ID }},
		{"missing segment", func(c *GTMContent) { c.Channels[0].SegmentIDs = []string{"missing"} }},
		{"missing experiment channel", func(c *GTMContent) { c.Experiments[0].ChannelID = "missing" }},
		{"milestone cycle", func(c *GTMContent) {
			c.Milestones = append(c.Milestones, GTMMilestone{ID: "milestone-next", DependsOn: []string{"milestone-learn"}})
			c.Milestones[0].DependsOn = []string{"milestone-next"}
		}},
		{"unknown status", func(c *GTMContent) { c.Experiments[0].Status = "published" }},
		{"unbound artifact", func(c *GTMContent) {
			c.ArtifactLinks = []GTMArtifactLink{{ID: "artifact-one", Revision: 1, Digest: "unknown"}}
		}},
		{"control text", func(c *GTMContent) { c.Positioning = "unsafe\x00text" }},
		{"too many segments", func(c *GTMContent) { c.Segments = make([]GTMSegment, 51) }},
		{"missing arrays", func(c *GTMContent) { c.ArtifactLinks = nil }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := gtmFixtureContent()
			tc.mutate(&c)
			if validateGTM(c) == nil {
				t.Fatal("invalid GTM record accepted")
			}
		})
	}
	if err := validateGTM(gtmFixtureContent()); err != nil {
		t.Fatal(err)
	}
}

func TestGTMWorkspaceRevisionReplayAndSourceCAS(t *testing.T) {
	s, h, id := controlTestHandler(t)
	base := "/v1/product/projects/" + id + "/distribute/gtm"
	var initial GTMView
	if err := json.Unmarshal(controlTestRequest(t, h, "GET", base, nil, 200).Body.Bytes(), &initial); err != nil {
		t.Fatal(err)
	}
	if initial.Workspace != nil || initial.Freshness != "none" || !hexDigest(initial.SourceDigest) {
		t.Fatal("missing workspace became a verified strategy")
	}
	request := map[string]any{"expectedRevision": 0, "expectedDigest": "", "expectedSourceDigest": initial.SourceDigest, "idempotencyKey": "gtm-first-save-request-123456", "content": gtmFixtureContent()}
	var saved GTMView
	json.Unmarshal(controlTestRequest(t, h, "POST", base, request, 200).Body.Bytes(), &saved)
	if saved.Workspace == nil || saved.Workspace.Revision != 1 || saved.Workspace.Actor.ID != "admin" || saved.Workspace.Digest != gtmDigest(*saved.Workspace) {
		t.Fatal("saved revision lost identity")
	}
	var replay GTMView
	json.Unmarshal(controlTestRequest(t, h, "POST", base, request, 200).Body.Bytes(), &replay)
	if replay.Freshness != "readback" || replay.Workspace.Digest != saved.Workspace.Digest {
		t.Fatal("lost response duplicated or changed the save")
	}
	changed := gtmFixtureContent()
	changed.Offer = "Another offer"
	request["content"] = changed
	controlTestRequest(t, h, "POST", base, request, 409)
	request["idempotencyKey"] = "gtm-stale-save-request-123456"
	controlTestRequest(t, h, "POST", base, request, 409)
	request["expectedRevision"], request["expectedDigest"] = 1, saved.Workspace.Digest
	request["expectedSourceDigest"] = strings.Repeat("a", 64)
	controlTestRequest(t, h, "POST", base, request, 409)
	request["expectedSourceDigest"] = initial.SourceDigest
	var next GTMView
	json.Unmarshal(controlTestRequest(t, h, "POST", base, request, 200).Body.Bytes(), &next)
	if next.Workspace.Revision != 2 || len(next.History) != 2 || next.Workspace.Content.Offer != "Another offer" {
		t.Fatal("second revision not retained")
	}
	p, err := s.resolveProject(id)
	if err != nil {
		t.Fatal(err)
	}
	_, ledger, err := s.readGTMLedger(p)
	if err != nil || len(ledger.Revisions) != 2 || ledger.Revisions[0].Content.Offer != "A small paid pilot" {
		t.Fatal("prior strategy was overwritten", err)
	}
}

func TestGTMLinksOnlyExactRetainedProjectArtifactsAndRequirements(t *testing.T) {
	_, h, id := controlTestHandler(t)
	base := "/v1/product/projects/" + id
	var view GTMView
	json.Unmarshal(controlTestRequest(t, h, "GET", base+"/distribute/gtm", nil, 200).Body.Bytes(), &view)
	content := gtmFixtureContent()
	content.Segments[0].RequirementIDs = []string{"AC-not-retained"}
	req := map[string]any{"expectedRevision": 0, "expectedDigest": "", "expectedSourceDigest": view.SourceDigest, "idempotencyKey": "gtm-artifact-request-123456", "content": content}
	controlTestRequest(t, h, "POST", base+"/distribute/gtm", req, 422)
	var prepared struct{ PrepareDigest string }
	json.Unmarshal(controlTestRequest(t, h, "POST", base+"/distribute/artifacts/prepare", map[string]string{"kind": "go-to-market"}, 200).Body.Bytes(), &prepared)
	var artifact struct{ Artifact DistributeArtifact }
	json.Unmarshal(controlTestRequest(t, h, "POST", base+"/distribute/artifacts", map[string]string{"kind": "go-to-market", "prepareDigest": prepared.PrepareDigest, "idempotencyKey": "gtm-linked-artifact-save-123456"}, 201).Body.Bytes(), &artifact)
	content = gtmFixtureContent()
	content.ArtifactLinks = []GTMArtifactLink{{ID: artifact.Artifact.ID, Revision: artifact.Artifact.Revision, Digest: strings.Repeat("b", 64)}}
	req["content"] = content
	controlTestRequest(t, h, "POST", base+"/distribute/gtm", req, 422)
	content.ArtifactLinks[0].Digest = artifact.Artifact.Digest
	req["content"] = content
	controlTestRequest(t, h, "POST", base+"/distribute/gtm", req, 200)
}

func TestGTMReadAndEditAuthorityRemainSeparate(t *testing.T) {
	s, mux := journeyFixture(t)
	p := createJourneyProject(t, mux, "gtm-authority")
	base := "/v1/product/projects/" + p.ID + "/distribute/gtm"
	controlTestRequest(t, mux, "GET", base, nil, 401)
	viewer := contextHistoryActorHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mux.ServeHTTP(w, r.WithContext(management.WithTeamAccess(r.Context(), management.TeamAccess{ProjectID: p.ID, Permissions: management.TeamPermissions{Read: true}})))
	}), management.User{ID: "viewer", Username: "viewer", Role: "member"})
	controlTestRequest(t, viewer, "POST", base, map[string]any{}, 403)
	_, ledger, err := s.readGTMLedger(p)
	if err != nil || len(ledger.Revisions) != 0 {
		t.Fatal("refused edit persisted", err)
	}
	wrong := contextHistoryActorHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.SetPathValue("id", p.ID)
		s.getGTMWorkspace(w, r.WithContext(management.WithTeamAccess(r.Context(), management.TeamAccess{ProjectID: "p-other", Permissions: management.TeamPermissions{Read: true}})))
	}), management.User{ID: "viewer", Username: "viewer", Role: "member"})
	controlTestRequest(t, wrong, "GET", base, nil, 403)
}

func TestGTMCommercialProjectionIsDerivedAndNeverStoredAsRevenueEvidence(t *testing.T) {
	_, h, id := controlTestHandler(t)
	base := "/v1/product/projects/" + id + "/distribute/gtm"
	var initial GTMView
	json.Unmarshal(controlTestRequest(t, h, "GET", base, nil, 200).Body.Bytes(), &initial)
	content := gtmFixtureContent()
	content.CommercialModel = &CommercialModelInput{Currency: "usd", MinorUnitDigits: 2, Interval: "month", PricePerUnitMinor: 1000, VariableCostPerUnitMinor: 200, FixedCostMinor: 1600, Units: 3, Assumptions: []string{"  Three pilot customers, not actual sales  "}}
	req := map[string]any{"expectedRevision": 0, "expectedDigest": "", "expectedSourceDigest": initial.SourceDigest, "idempotencyKey": "gtm-commercial-save-123456", "content": content}
	var view GTMView
	json.Unmarshal(controlTestRequest(t, h, "POST", base, req, 200).Body.Bytes(), &view)
	if view.CommercialModel == nil || view.CommercialModel.RevenueMinor != 3000 || view.CommercialModel.OperatingProfitMinor != 800 || view.Workspace.Content.CommercialModel.Currency != "USD" || view.Workspace.Content.CommercialModel.Assumptions[0] != "Three pilot customers, not actual sales" {
		t.Fatal("scenario inputs/projection drift")
	}
	encoded, _ := json.Marshal(view.Workspace.Content)
	if strings.Contains(string(encoded), "revenueMinor") {
		t.Fatal("derived figure stored as an authored fact")
	}
}

func TestGTMContextChangesRemainExplicitAndUnavailableSourceRetainsSavedContent(t *testing.T) {
	s, h, id := controlTestHandler(t)
	base := "/v1/product/projects/" + id
	var initial GTMView
	json.Unmarshal(controlTestRequest(t, h, "GET", base+"/distribute/gtm", nil, 200).Body.Bytes(), &initial)
	req := map[string]any{"expectedRevision": 0, "expectedDigest": "", "expectedSourceDigest": initial.SourceDigest, "idempotencyKey": "gtm-context-first-request-123456", "content": gtmFixtureContent()}
	var saved GTMView
	json.Unmarshal(controlTestRequest(t, h, "POST", base+"/distribute/gtm", req, 200).Body.Bytes(), &saved)
	controlTestRequest(t, h, "POST", base+"/brief", map[string]any{"revision": initial.Source.BriefRevision, "purpose": "A changed project purpose", "audience": "Local teams", "criteria": []string{}}, 200)
	var changed GTMView
	json.Unmarshal(controlTestRequest(t, h, "GET", base+"/distribute/gtm", nil, 200).Body.Bytes(), &changed)
	if changed.Freshness != "outdated" || changed.SourceDigest == initial.SourceDigest || changed.Workspace.Digest != saved.Workspace.Digest {
		t.Fatal("changed context silently rebound strategy")
	}
	p, err := s.resolveProject(id)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(p.Path, ".luci", "state.json"), []byte("{"), 0600); err != nil {
		t.Fatal(err)
	}
	var retained GTMView
	json.Unmarshal(controlTestRequest(t, h, "GET", base+"/distribute/gtm", nil, 200).Body.Bytes(), &retained)
	if retained.Freshness != "unavailable" || retained.SourceDigest != "" || retained.Workspace.Digest != saved.Workspace.Digest {
		t.Fatal("unavailable source erased or verified retained strategy")
	}
}
