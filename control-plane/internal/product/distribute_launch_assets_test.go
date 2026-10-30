package product

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
)

func launchStrategyForTest(t *testing.T, h http.Handler, projectID string, content GTMContent) GTMView {
	t.Helper()
	base := "/v1/product/projects/" + projectID + "/distribute/gtm"
	var current, next GTMView
	if err := json.Unmarshal(controlTestRequest(t, h, "GET", base, nil, 200).Body.Bytes(), &current); err != nil {
		t.Fatal(err)
	}
	revision, digest := 0, ""
	if current.Workspace != nil {
		revision, digest = current.Workspace.Revision, current.Workspace.Digest
	}
	request := map[string]any{"expectedRevision": revision, "expectedDigest": digest, "expectedSourceDigest": current.SourceDigest, "idempotencyKey": fmt.Sprintf("launch-strategy-save-%d", revision), "content": content}
	if err := json.Unmarshal(controlTestRequest(t, h, "POST", base, request, 200).Body.Bytes(), &next); err != nil {
		t.Fatal(err)
	}
	return next
}
func launchPreparedForTest(t *testing.T, h http.Handler, id, kind string) (DistributeArtifact, string) {
	t.Helper()
	var result struct {
		Draft         DistributeArtifact `json:"draft"`
		PrepareDigest string             `json:"prepareDigest"`
	}
	if err := json.Unmarshal(controlTestRequest(t, h, "POST", "/v1/product/projects/"+id+"/distribute/artifacts/prepare", map[string]any{"kind": kind, "runId": ""}, 200).Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	return result.Draft, result.PrepareDigest
}
func launchSavedForTest(t *testing.T, h http.Handler, id, kind, prepareDigest, key string) DistributeArtifact {
	t.Helper()
	var result struct {
		Artifact DistributeArtifact `json:"artifact"`
	}
	if err := json.Unmarshal(controlTestRequest(t, h, "POST", "/v1/product/projects/"+id+"/distribute/artifacts", map[string]any{"kind": kind, "runId": "", "prepareDigest": prepareDigest, "idempotencyKey": key}, 201).Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	return result.Artifact
}

func TestLaunchAssetsRequireSavedCurrentGTMWithoutChangingLegacySource(t *testing.T) {
	s, mux := journeyFixture(t)
	p := createJourneyProject(t, mux, "launch-saved-strategy")
	h := coordinationHandler(s, "alice")
	base := "/v1/product/projects/" + p.ID + "/distribute/artifacts/prepare"
	controlTestRequest(t, h, "POST", base, map[string]any{"kind": "landing-copy"}, 409)
	legacy, _ := launchPreparedForTest(t, h, p.ID, "proposal")
	raw, _ := json.Marshal(legacy.Source)
	if strings.Contains(string(raw), "gtmRevision") || strings.Contains(string(raw), "gtmDigest") {
		t.Fatal("legacy source encoding changed")
	}
	oldJSON := fmt.Sprintf(`{"briefRevision":%d,"briefDigest":%q,"specificationRevision":%d,"specificationDigest":%q,"designRevision":%d,"designDigest":%q,"planRevision":%d,"planDigest":%q,"checks":"not-selected","release":"not-selected"}`, legacy.Source.BriefRevision, legacy.Source.BriefDigest, legacy.Source.SpecificationRevision, legacy.Source.SpecificationDigest, legacy.Source.DesignRevision, legacy.Source.DesignDigest, legacy.Source.PlanRevision, legacy.Source.PlanDigest)
	if string(raw) != oldJSON {
		t.Fatal("adding optional GTM fields changed exact legacy JSON bytes")
	}
	pair := legacy.Source
	pair.GTMRevision = 1
	if validArtifactSource(pair) {
		t.Fatal("partial GTM source binding accepted")
	}
	legacy.Source.GTMRevision = 1
	legacy.Source.GTMDigest = strings.Repeat("a", 64)
	if validArtifactContent(legacy) {
		t.Fatal("unrelated legacy kind acquired a GTM dependency")
	}
	launchStrategyForTest(t, h, p.ID, gtmFixtureContent())
	project, _ := s.resolveProject(p.ID)
	state, err := s.readApprovedJourney(project.Path, project.ID)
	if err != nil {
		t.Fatal(err)
	}
	state.Brief.Revision++
	state.Brief.Purpose = "A changed source contract"
	if err := writeJourney(project.Path, state); err != nil {
		t.Fatal(err)
	}
	controlTestRequest(t, h, "POST", base, map[string]any{"kind": "landing-copy"}, 409)
	latest, _ := launchPreparedForTest(t, h, p.ID, "proposal")
	if latest.Source.GTMRevision != 0 || latest.Source.GTMDigest != "" {
		t.Fatal("unrelated artifact acquired GTM freshness dependency")
	}
}

func TestLaunchAssetsCaptureExactStrategyAndScenarioAsDrafts(t *testing.T) {
	for _, kind := range []string{"landing-copy", "launch-brief", "channel-copy"} {
		t.Run(kind, func(t *testing.T) {
			s, mux := journeyFixture(t)
			p := createJourneyProject(t, mux, "launch-"+kind)
			h := coordinationHandler(s, "alice")
			content := gtmFixtureContent()
			content.CommercialModel = &CommercialModelInput{Currency: "AUD", MinorUnitDigits: 2, Interval: "month", PricePerUnitMinor: 2950, VariableCostPerUnitMinor: 500, FixedCostMinor: 10000, Units: 1000, Assumptions: []string{"One thousand units is an untested scenario"}}
			strategy := launchStrategyForTest(t, h, p.ID, content)
			draft, _ := launchPreparedForTest(t, h, p.ID, kind)
			if draft.Kind != kind || draft.State != "draft" || draft.Review != nil || draft.Source.GTMRevision != 1 || draft.Source.GTMDigest != strategy.Workspace.Digest || draft.SourceDigest != reviewDigest(draft.Source) {
				t.Fatal("launch draft lost exact source or acquired review", draft)
			}
			raw, _ := json.Marshal(draft)
			text := string(raw)
			for _, want := range []string{content.Positioning, content.Offer, "AUD 29.50 per month", content.PricingAssumptions[0], content.CommercialModel.Assumptions[0], "gtm:1:" + strategy.Workspace.Digest} {
				if !strings.Contains(text, want) {
					t.Fatal("saved source omitted", want)
				}
			}
			if !validArtifactContent(draft) || strings.Contains(text, "1000 customers") || strings.Contains(text, "revenue result: 29500") {
				t.Fatal("scenario became claimed demand/revenue")
			}
			found := false
			for _, line := range artifactExportLines(draft, "current") {
				if line.Label == "GTM strategy" && strings.Contains(line.Value, strategy.Workspace.Digest) {
					found = true
				}
			}
			if !found {
				t.Fatal("export omitted exact strategy revision")
			}
		})
	}
}

func TestLaunchAssetPrepareSaveEditReviewAndReplayUseExistingLedger(t *testing.T) {
	s, mux := journeyFixture(t)
	p := createJourneyProject(t, mux, "launch-edit-review")
	h := coordinationHandler(s, "alice")
	launchStrategyForTest(t, h, p.ID, gtmFixtureContent())
	_, prepare := launchPreparedForTest(t, h, p.ID, "landing-copy")
	row := launchSavedForTest(t, h, p.ID, "landing-copy", prepare, "launch-save-original")
	base := "/v1/product/projects/" + p.ID + "/distribute/artifacts"
	var replay struct {
		Artifact  DistributeArtifact `json:"artifact"`
		Freshness string             `json:"freshness"`
	}
	json.Unmarshal(controlTestRequest(t, h, "POST", base, map[string]any{"kind": "landing-copy", "runId": "", "prepareDigest": prepare, "idempotencyKey": "launch-save-original"}, 200).Body.Bytes(), &replay)
	if replay.Artifact.ID != row.ID || replay.Artifact.Revision != 1 || replay.Freshness != "readback" {
		t.Fatal("lost response replay duplicated asset")
	}
	sections := append([]DistributeArtifactSection{}, row.Sections...)
	sections[0].Body = "A human-reviewed headline draft"
	var edited struct {
		Artifact DistributeArtifact `json:"artifact"`
	}
	request := map[string]any{"expectedRevision": row.Revision, "expectedDigest": row.Digest, "expectedSourceDigest": row.SourceDigest, "idempotencyKey": "launch-edit-original", "title": row.Title, "sections": sections, "gaps": row.Gaps, "assumptions": row.Assumptions}
	json.Unmarshal(controlTestRequest(t, h, "POST", base+"/"+row.ID+"/revise", request, 200).Body.Bytes(), &edited)
	if edited.Artifact.Revision != 2 || edited.Artifact.State != "draft" || edited.Artifact.Review != nil || edited.Artifact.SourceDigest != row.SourceDigest || edited.Artifact.Sections[0].SourceRefs[0] != row.Sections[0].SourceRefs[0] {
		t.Fatal("edit changed source/review instead of content")
	}
	var reviewed struct {
		Artifact DistributeArtifact `json:"artifact"`
	}
	json.Unmarshal(controlTestRequest(t, h, "POST", base+"/"+row.ID+"/review", map[string]any{"expectedRevision": 2, "expectedDigest": edited.Artifact.Digest, "idempotencyKey": "launch-review-original"}, 200).Body.Bytes(), &reviewed)
	if reviewed.Artifact.State != "reviewed" || reviewed.Artifact.Review == nil || reviewed.Artifact.Review.Digest != edited.Artifact.Digest {
		t.Fatal("explicit review not bound to edited draft")
	}
	project, _ := s.resolveProject(p.ID)
	_, ledger, err := s.readArtifactLedger(project)
	if err != nil || len(ledger.Records) != 1 || len(ledger.Records[0].Revisions) != 3 {
		t.Fatal("did not reuse immutable existing artifact history", err)
	}
}

func TestLaunchAssetGTMChangesRejectStalePreparedSaveAndReview(t *testing.T) {
	s, mux := journeyFixture(t)
	p := createJourneyProject(t, mux, "launch-source-cas")
	h := coordinationHandler(s, "alice")
	content := gtmFixtureContent()
	launchStrategyForTest(t, h, p.ID, content)
	_, legacyPrepare := launchPreparedForTest(t, h, p.ID, "proposal")
	legacy := launchSavedForTest(t, h, p.ID, "proposal", legacyPrepare, "legacy-launch-independent")
	_, oldPrepare := launchPreparedForTest(t, h, p.ID, "channel-copy")
	content.Offer = "A revised offer"
	launchStrategyForTest(t, h, p.ID, content)
	base := "/v1/product/projects/" + p.ID + "/distribute/artifacts"
	var legacyObserved struct {
		Artifact  DistributeArtifact `json:"artifact"`
		Freshness string             `json:"freshness"`
	}
	json.Unmarshal(controlTestRequest(t, h, "GET", base+"/"+legacy.ID, nil, 200).Body.Bytes(), &legacyObserved)
	if legacyObserved.Freshness != "current" || legacyObserved.Artifact.Source.GTMRevision != 0 {
		t.Fatal("GTM edits made an unrelated legacy artifact stale")
	}
	controlTestRequest(t, h, "POST", base, map[string]any{"kind": "channel-copy", "runId": "", "prepareDigest": oldPrepare, "idempotencyKey": "stale-launch-save"}, 409)
	_, currentPrepare := launchPreparedForTest(t, h, p.ID, "channel-copy")
	row := launchSavedForTest(t, h, p.ID, "channel-copy", currentPrepare, "current-launch-save")
	content.Positioning = "A different positioning"
	launchStrategyForTest(t, h, p.ID, content)
	var observed struct {
		Artifact  DistributeArtifact `json:"artifact"`
		Freshness string             `json:"freshness"`
	}
	json.Unmarshal(controlTestRequest(t, h, "GET", base+"/"+row.ID, nil, 200).Body.Bytes(), &observed)
	if observed.Freshness != "outdated" || observed.Artifact.Source.GTMRevision != 2 || observed.Artifact.Source.GTMDigest != row.Source.GTMDigest {
		t.Fatal("stale asset silently rebound to new GTM")
	}
	controlTestRequest(t, h, "POST", base+"/"+row.ID+"/review", map[string]any{"expectedRevision": row.Revision, "expectedDigest": row.Digest, "idempotencyKey": "stale-launch-review"}, 409)
}

func TestLaunchCopyKeepsMissingInputsAndExactMoneyExplicit(t *testing.T) {
	if launchMoney(2950, 2) != "29.50" || launchMoney(7, 0) != "7" || launchMoney(1, 6) != "0.000001" {
		t.Fatal("money formatting rounded the saved integer")
	}
	base := DistributeArtifact{Kind: "channel-copy", State: "draft", Sections: []DistributeArtifactSection{}, Assumptions: []string{}, Gaps: []string{}}
	content := emptyGTMContent()
	draft := generatedLaunchAsset(base, GTMWorkspace{Revision: 1, Digest: strings.Repeat("a", 64), Content: content})
	if len(draft.Gaps) < 4 || len(draft.Sections) < 3 || draft.Sections[1].Body != "" {
		t.Fatal("missing copy inputs became fabricated claims")
	}
	content.Positioning = strings.Repeat("P", 4000)
	content.Offer = strings.Repeat("O", 4000)
	draft = generatedLaunchAsset(base, GTMWorkspace{Revision: 1, Digest: strings.Repeat("a", 64), Content: content})
	if !strings.Contains(draft.Sections[1].Body, content.Positioning) || !strings.Contains(draft.Sections[1].Body, content.Offer) || len(draft.Sections[1].Body) > 8192 {
		t.Fatal("bounded copy was truncated or overflowed")
	}
}
