package product

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestDistributeArtifactPrepareSaveReviewAndSourceCAS(t *testing.T) {
	s, h, id := controlTestHandler(t)
	p, err := s.resolveProject(id)
	if err != nil {
		t.Fatal(err)
	}
	base := "/v1/product/projects/" + id + "/distribute/artifacts"
	_, before, err := s.readArtifactLedger(p)
	if err != nil || len(before.Records) != 0 {
		t.Fatal("preparation fixture already has artifacts", err)
	}
	prepared := controlTestRequest(t, h, "POST", base+"/prepare", map[string]string{"kind": "risk"}, 200)
	var proposal struct {
		Draft         DistributeArtifact `json:"draft"`
		PrepareDigest string             `json:"prepareDigest"`
	}
	if err = json.Unmarshal(prepared.Body.Bytes(), &proposal); err != nil || !hexDigest(proposal.PrepareDigest) || proposal.Draft.Kind != "risk" || len(proposal.Draft.Gaps) < 2 || len(proposal.Draft.Sections) == 0 {
		t.Fatal(prepared.Body.String(), err)
	}
	_, after, err := s.readArtifactLedger(p)
	if err != nil || len(after.Records) != 0 {
		t.Fatal("effects-free preparation persisted", err)
	}
	request := map[string]any{"kind": "risk", "prepareDigest": proposal.PrepareDigest, "idempotencyKey": "artifact-save-request-123456"}
	var saved struct {
		Artifact  DistributeArtifact `json:"artifact"`
		Freshness string             `json:"freshness"`
	}
	response := controlTestRequest(t, h, "POST", base, request, 201)
	if err = json.Unmarshal(response.Body.Bytes(), &saved); err != nil || saved.Artifact.Revision != 1 || saved.Freshness != "current" || saved.Artifact.State != "draft" {
		t.Fatal(response.Body.String(), err)
	}
	controlTestRequest(t, h, "POST", base, request, 200)
	artifactURL := base + "/" + saved.Artifact.ID
	var read struct {
		History []DistributeArtifact `json:"history"`
	}
	if err = json.Unmarshal(controlTestRequest(t, h, "GET", artifactURL, nil, 200).Body.Bytes(), &read); err != nil || len(read.History) != 1 {
		t.Fatal(err, len(read.History))
	}
	sections := append([]DistributeArtifactSection{}, saved.Artifact.Sections...)
	changedRefs := append([]DistributeArtifactSection{}, sections...)
	changedRefs[0].SourceRefs = []string{"invented:source"}
	controlTestRequest(t, h, "POST", artifactURL+"/revise", map[string]any{"expectedRevision": 1, "expectedDigest": saved.Artifact.Digest, "expectedSourceDigest": saved.Artifact.SourceDigest, "idempotencyKey": "artifact-forged-source-123456", "title": saved.Artifact.Title, "sections": changedRefs, "gaps": saved.Artifact.Gaps, "assumptions": saved.Artifact.Assumptions}, 422)
	sections[0].Body += " Reviewed source-specific risk."
	revised := controlTestRequest(t, h, "POST", artifactURL+"/revise", map[string]any{"expectedRevision": 1, "expectedDigest": saved.Artifact.Digest, "expectedSourceDigest": saved.Artifact.SourceDigest, "idempotencyKey": "artifact-revise-request-123456", "title": saved.Artifact.Title, "sections": sections, "gaps": saved.Artifact.Gaps, "assumptions": saved.Artifact.Assumptions}, 200)
	if err = json.Unmarshal(revised.Body.Bytes(), &saved); err != nil || saved.Artifact.Revision != 2 || saved.Artifact.State != "draft" {
		t.Fatal(revised.Body.String(), err)
	}
	review := controlTestRequest(t, h, "POST", artifactURL+"/review", map[string]any{"expectedRevision": 2, "expectedDigest": saved.Artifact.Digest, "idempotencyKey": "artifact-review-request-123456"}, 200)
	if err = json.Unmarshal(review.Body.Bytes(), &saved); err != nil || saved.Artifact.Revision != 3 || saved.Artifact.State != "reviewed" || saved.Artifact.Review == nil {
		t.Fatal(review.Body.String(), err)
	}
	controlTestRequest(t, h, "POST", artifactURL+"/review", map[string]any{"expectedRevision": 3, "expectedDigest": saved.Artifact.Digest, "idempotencyKey": "artifact-duplicate-review-123456"}, 409)
	path, err := teamControlPath(s, "team-test-control")
	if err != nil {
		t.Fatal(err)
	}
	control := TeamControl{ID: "SEC-1", Requirement: "Require distinct reviewed evidence", Sheet: "Controls", Row: 2, Unmapped: map[string]string{}, UnmappedHeaders: map[string]string{}}
	revision := TeamControlRevision{Revision: 1, Digest: reviewDigest([]TeamControl{control}), SourceSHA256: strings.Repeat("a", 64), Controls: []TeamControl{control}, KeyDigest: strings.Repeat("b", 64)}
	if err = writeTeamControls(path, teamControlLedger{Version: 1, TeamID: "team-test-control", Revisions: []TeamControlRevision{revision}}); err != nil {
		t.Fatal(err)
	}
	var stale struct {
		Freshness string `json:"freshness"`
	}
	if err = json.Unmarshal(controlTestRequest(t, h, "GET", artifactURL, nil, 200).Body.Bytes(), &stale); err != nil || stale.Freshness != "outdated" {
		t.Fatal("new team import did not stale artifact", err, stale)
	}
	controlTestRequest(t, h, "POST", artifactURL+"/review", map[string]any{"expectedRevision": 3, "expectedDigest": saved.Artifact.Digest, "idempotencyKey": "artifact-stale-review-123456"}, 409)
	_, retained, err := s.readArtifactLedger(p)
	if err != nil || len(retained.Records) != 1 || len(retained.Records[0].Revisions) != 3 {
		t.Fatal("stale refusal changed private history", err)
	}
}
