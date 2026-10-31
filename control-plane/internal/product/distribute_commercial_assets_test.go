package product

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
	"unicode/utf8"
)

func TestCommercialDocumentProjectionKeepsValidLongUTF8WithinExistingBounds(t *testing.T) {
	s, mux := journeyFixture(t)
	p := createJourneyProject(t, mux, "commercial-long-text")
	h := coordinationHandler(s, "alice")
	content := gtmFixtureContent()
	long := strings.Repeat("界<", 999)
	content.Positioning, content.Offer = long, long
	segment := content.Segments[0]
	segment.Name, segment.Problem, segment.Value = long, long, long
	content.Segments = []GTMSegment{segment, segment}
	content.Segments[1].ID = "segment-second"
	experiment := &content.Experiments[0]
	experiment.Title, experiment.Hypothesis, experiment.Method, experiment.SuccessCriterion, experiment.Result = long, long, long, long, long
	content.CommercialModel = &CommercialModelInput{Currency: "AUD", MinorUnitDigits: 2, Interval: "month", PricePerUnitMinor: 2950, VariableCostPerUnitMinor: 500, FixedCostMinor: 10000, Units: 1000, Assumptions: []string{"Demand is an untested scenario"}}
	if err := validateGTM(content); err != nil {
		t.Fatal("long source fixture is not a valid saved strategy", err)
	}
	saved := launchStrategyForTest(t, h, p.ID, content)
	for _, kind := range []string{"go-to-market", "pricing", "support-guide"} {
		t.Run(kind, func(t *testing.T) {
			draft, _ := launchPreparedForTest(t, h, p.ID, kind)
			encoded, err := json.Marshal(draft)
			if err != nil || len(encoded) > 256<<10 || !validArtifactContent(draft) {
				t.Fatal("valid detailed source exceeded existing artifact bounds", kind, len(encoded), err)
			}
			notice := false
			for _, gap := range draft.Gaps {
				if strings.Contains(gap, "Bounded document projection:") && strings.Contains(gap, saved.Workspace.Digest) {
					notice = true
				}
			}
			if !notice || draft.Source.GTMRevision != saved.Workspace.Revision || draft.Source.GTMDigest != saved.Workspace.Digest {
				t.Fatal("projection hid shortening or changed its exact source")
			}
			for _, section := range draft.Sections {
				if !utf8.ValidString(section.Body) || strings.ContainsRune(section.Body, '\ufffd') || len(section.Body) > 8192 || len(section.Bullets) > 100 {
					t.Fatal("invalid projected body")
				}
				for _, bullet := range section.Bullets {
					if !utf8.ValidString(bullet) || strings.ContainsRune(bullet, '\ufffd') || len(bullet) > 8192 {
						t.Fatal("invalid projected bullet")
					}
				}
			}
		})
	}
	project, err := s.resolveProject(p.ID)
	if err != nil {
		t.Fatal(err)
	}
	_, ledger, err := s.readGTMLedger(project)
	if err != nil || len(ledger.Revisions) != 1 || ledger.Revisions[0].Digest != saved.Workspace.Digest || reviewDigest(ledger.Revisions[0].Content) != reviewDigest(content) {
		t.Fatal("display projection mutated the saved strategy", err)
	}
}

func TestGTMDocumentUsesSavedChannelsMilestonesAndReportedExperiments(t *testing.T) {
	s, mux := journeyFixture(t)
	p := createJourneyProject(t, mux, "commercial-strategy")
	h := coordinationHandler(s, "alice")
	content := gtmFixtureContent()
	content.Milestones[0].TargetDate = "2026-12-01"
	content.Experiments[0].Status = "recorded"
	content.Experiments[0].Result = "Three consenting participants reported interest."
	saved := launchStrategyForTest(t, h, p.ID, content)
	draft, _ := launchPreparedForTest(t, h, p.ID, "go-to-market")
	raw, _ := json.Marshal(draft)
	text := string(raw)
	for _, want := range []string{content.Positioning, content.Offer, content.Segments[0].Name, content.Channels[0].Name, content.Channels[0].Hypothesis, content.Milestones[0].TargetDate, content.Experiments[0].Result, content.PricingAssumptions[0], saved.Workspace.Digest} {
		if !strings.Contains(text, want) {
			t.Fatal("saved strategy source omitted", want)
		}
	}
	if !validArtifactContent(draft) || draft.Source.GTMRevision != saved.Workspace.Revision || launchArtifactKind(draft.Kind) || strings.Contains(text, "No observed demand, channel decision") || strings.Contains(text, "Channel: unconfirmed") {
		t.Fatal("strategy claims missing saved choices or acquired public authority")
	}
}

func TestCommercialPricingCapturesExactGTMScenarioAndRefusesStaleSave(t *testing.T) {
	s, mux := journeyFixture(t)
	p := createJourneyProject(t, mux, "commercial-pricing")
	h := coordinationHandler(s, "alice")
	base := "/v1/product/projects/" + p.ID + "/distribute/artifacts"
	for _, kind := range []string{"pricing", "support-guide"} {
		controlTestRequest(t, h, "POST", base+"/prepare", map[string]string{"kind": kind}, 409)
	}
	content := gtmFixtureContent()
	content.CommercialModel = &CommercialModelInput{Currency: "AUD", MinorUnitDigits: 2, Interval: "month", PricePerUnitMinor: 100, VariableCostPerUnitMinor: 200, FixedCostMinor: 50, Units: 2, Assumptions: []string{"Two units is an untested demand scenario"}}
	saved := launchStrategyForTest(t, h, p.ID, content)
	draft, prepared := launchPreparedForTest(t, h, p.ID, "pricing")
	if !validArtifactContent(draft) || draft.Source.GTMRevision != saved.Workspace.Revision || draft.Source.GTMDigest != saved.Workspace.Digest || draft.SourceDigest != reviewDigest(draft.Source) {
		t.Fatal("Pricing lost exact current scenario", draft)
	}
	raw, _ := json.Marshal(draft)
	text := string(raw)
	for _, want := range []string{"AUD 1.00", "AUD 2.00", "AUD 0.50", "AUD -2.50", "AUD -1.00", "not established", content.CommercialModel.Assumptions[0], "gtm:1:" + saved.Workspace.Digest} {
		if !strings.Contains(text, want) {
			t.Fatal("scenario amount or attribution omitted", want)
		}
	}
	if strings.Contains(text, "No rate, unit, currency") || strings.Contains(text, "Price and currency: not selected") || launchArtifactKind("pricing") {
		t.Fatal("saved values remained unknown or acquired publishing authority")
	}
	content.CommercialModel.PricePerUnitMinor = 150
	launchStrategyForTest(t, h, p.ID, content)
	controlTestRequest(t, h, "POST", base, map[string]any{"kind": "pricing", "prepareDigest": prepared, "idempotencyKey": "stale-pricing-request"}, 409)
}

func TestSupportGuideUsesExistingEditableHistoryWithoutInventingTerms(t *testing.T) {
	s, mux := journeyFixture(t)
	p := createJourneyProject(t, mux, "commercial-support")
	h := coordinationHandler(s, "alice")
	strategy := launchStrategyForTest(t, h, p.ID, gtmFixtureContent())
	draft, prepared := launchPreparedForTest(t, h, p.ID, "support-guide")
	raw, _ := json.Marshal(draft)
	for _, want := range []string{"Draft choice:", "No support hours", "No entitlement", "No commercial scenario", "gtm:1:" + strategy.Workspace.Digest} {
		if !strings.Contains(string(raw), want) {
			t.Fatal("support choice became a commitment or lost provenance", want)
		}
	}
	if !validArtifactContent(draft) || launchArtifactKind(draft.Kind) || draft.Source.SelectedEvidence != nil {
		t.Fatal("support draft acquired public or accepted-release authority")
	}
	row := launchSavedForTest(t, h, p.ID, "support-guide", prepared, "support-save-request")
	sections := append([]DistributeArtifactSection{}, row.Sections...)
	found := false
	for i := range sections {
		if sections[i].ID == "support-contact" {
			sections[i].Body = "Contact the reviewed support address supplied with this release. Availability remains to be agreed."
			found = true
		}
	}
	if !found {
		t.Fatal("guide has no editable contact answer")
	}
	base := "/v1/product/projects/" + p.ID + "/distribute/artifacts/" + row.ID
	var edited, reviewed struct {
		Artifact DistributeArtifact `json:"artifact"`
	}
	response := controlTestRequest(t, h, "POST", base+"/revise", map[string]any{"expectedRevision": row.Revision, "expectedDigest": row.Digest, "expectedSourceDigest": row.SourceDigest, "idempotencyKey": "support-edit-request", "title": row.Title, "sections": sections, "gaps": row.Gaps, "assumptions": row.Assumptions}, 200)
	if err := json.Unmarshal(response.Body.Bytes(), &edited); err != nil {
		t.Fatal(err)
	}
	response = controlTestRequest(t, h, "POST", base+"/review", map[string]any{"expectedRevision": edited.Artifact.Revision, "expectedDigest": edited.Artifact.Digest, "idempotencyKey": "support-review-request"}, 200)
	if err := json.Unmarshal(response.Body.Bytes(), &reviewed); err != nil {
		t.Fatal(err)
	}
	if reviewed.Artifact.State != "reviewed" || reviewed.Artifact.Revision != 3 || reviewed.Artifact.SourceDigest != row.SourceDigest || reviewed.Artifact.Review == nil {
		t.Fatal("support did not reuse exact-source existing review history")
	}
	exported := controlTestRequest(t, h, "GET", base+"/exports/docx?digest="+reviewed.Artifact.Digest, nil, 200)
	if !strings.Contains(artifactZipContent(t, exported.Body.Bytes()), "Availability remains to be agreed.") {
		t.Fatal("edited support answer missing from ordinary export")
	}
}

func TestLegacyUnboundCommercialDocumentsRetainExactDigestReadEditReviewAndExport(t *testing.T) {
	for _, kind := range []string{"pricing", "go-to-market"} {
		t.Run(kind, func(t *testing.T) {
			s, mux := journeyFixture(t)
			created := createJourneyProject(t, mux, "legacy-pricing")
			h := coordinationHandler(s, "alice")
			p, err := s.resolveProject(created.ID)
			if err != nil {
				t.Fatal(err)
			}
			source, state, library, assessment, err := s.artifactSource(orchestratorActor(t, "alice"), p, "")
			if err != nil {
				t.Fatal(err)
			}
			legacy := generatedArtifact(kind, p.ID, source, state, library, assessment)
			legacy.ID = newID("artifact-")
			legacy.Revision = 1
			legacy.Actor = ContextActor{ID: "alice", DisplayName: "alice"}
			legacy.At = time.Now().UTC().Format(time.RFC3339Nano)
			legacy.RequestKeyDigest = reviewDigest("original-legacy-pricing-key")
			legacy.RequestDigest = reviewDigest("original-legacy-pricing-request")
			legacy.Digest = artifactDigest(legacy)
			original, _ := json.Marshal(legacy)
			path, ledger, err := s.artifactLedgerPath(p)
			if err != nil {
				t.Fatal(err)
			}
			ledger.Records = append(ledger.Records, distributeArtifactRecord{ID: legacy.ID, Revisions: []DistributeArtifact{legacy}})
			if err = writeArtifactLedger(path, ledger); err != nil {
				t.Fatal(err)
			}
			launchStrategyForTest(t, h, p.ID, gtmFixtureContent())
			base := "/v1/product/projects/" + p.ID + "/distribute/artifacts/" + legacy.ID
			var read struct {
				Artifact  DistributeArtifact   `json:"artifact"`
				Freshness string               `json:"freshness"`
				History   []DistributeArtifact `json:"history"`
			}
			response := controlTestRequest(t, h, "GET", base, nil, 200)
			if err = json.Unmarshal(response.Body.Bytes(), &read); err != nil {
				t.Fatal(err)
			}
			same, _ := json.Marshal(read.Artifact)
			if string(same) != string(original) || read.Freshness != "current" || read.Artifact.Source.GTMRevision != 0 {
				t.Fatal("new GTM rewrote or made legacy pricing unreadable")
			}
			sections := append([]DistributeArtifactSection{}, legacy.Sections...)
			sections[0].Body += " A retained legacy clarification."
			response = controlTestRequest(t, h, "POST", base+"/revise", map[string]any{"expectedRevision": 1, "expectedDigest": legacy.Digest, "expectedSourceDigest": legacy.SourceDigest, "idempotencyKey": "legacy-pricing-edit", "title": legacy.Title, "sections": sections, "gaps": legacy.Gaps, "assumptions": legacy.Assumptions}, 200)
			if err = json.Unmarshal(response.Body.Bytes(), &read); err != nil {
				t.Fatal(err)
			}
			response = controlTestRequest(t, h, "POST", base+"/review", map[string]any{"expectedRevision": 2, "expectedDigest": read.Artifact.Digest, "idempotencyKey": "legacy-pricing-review"}, 200)
			if err = json.Unmarshal(response.Body.Bytes(), &read); err != nil {
				t.Fatal(err)
			}
			exported := controlTestRequest(t, h, "GET", base+"/exports/docx?digest="+read.Artifact.Digest, nil, 200)
			if !strings.Contains(artifactZipContent(t, exported.Body.Bytes()), "retained legacy clarification") {
				t.Fatal("legacy export lost edited content")
			}
			_, saved, err := s.readArtifactLedger(p)
			if err != nil || len(saved.Records) != 1 || len(saved.Records[0].Revisions) != 3 {
				t.Fatal("legacy history changed", err)
			}
			originalAgain, _ := json.Marshal(saved.Records[0].Revisions[0])
			if string(originalAgain) != string(original) {
				t.Fatal("legacy first revision or digest mutated")
			}
		})
	}
}
