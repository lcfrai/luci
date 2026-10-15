package product

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"encoding/xml"
	"errors"
	"io"
	"strings"
	"testing"
)

func artifactZipContent(t *testing.T, data []byte) string {
	t.Helper()
	archive, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil || len(archive.File) < 3 {
		t.Fatal("export is not a native document", err)
	}
	var content string
	for _, file := range archive.File {
		if strings.HasSuffix(file.Name, "document.xml") || strings.HasSuffix(file.Name, "sheet1.xml") || (strings.HasPrefix(file.Name, "ppt/slides/slide") && strings.HasSuffix(file.Name, ".xml")) {
			reader, e := file.Open()
			if e != nil {
				t.Fatal(e)
			}
			body, e := io.ReadAll(reader)
			reader.Close()
			if e != nil {
				t.Fatal(e)
			}
			if strings.HasPrefix(file.Name, "ppt/slides/") {
				decoder := xml.NewDecoder(bytes.NewReader(body))
				insideText := false
				for {
					token, e := decoder.Token()
					if errors.Is(e, io.EOF) {
						break
					}
					if e != nil {
						t.Fatal("invalid presentation XML", e)
					}
					switch value := token.(type) {
					case xml.StartElement:
						insideText = value.Name.Local == "t"
					case xml.EndElement:
						if value.Name.Local == "t" {
							insideText = false
						}
					case xml.CharData:
						if insideText {
							content += string(value)
						}
					}
				}
			} else {
				content += string(body)
			}
		}
	}
	if content == "" {
		t.Fatal("document contains no saved text")
	}
	return content
}

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
	controlTestRequest(t, h, "POST", base+"/prepare", map[string]string{"kind": "risk", "runId": "missing-run"}, 409)
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
	invalidXML := append([]DistributeArtifactSection{}, sections...)
	invalidXML[0].Body = "invalid\x01office text"
	controlTestRequest(t, h, "POST", artifactURL+"/revise", map[string]any{"expectedRevision": 1, "expectedDigest": saved.Artifact.Digest, "expectedSourceDigest": saved.Artifact.SourceDigest, "idempotencyKey": "artifact-invalid-xml-123456", "title": saved.Artifact.Title, "sections": invalidXML, "gaps": saved.Artifact.Gaps, "assumptions": saved.Artifact.Assumptions}, 422)
	sections[0].Body += " Reviewed source-specific risk."
	revised := controlTestRequest(t, h, "POST", artifactURL+"/revise", map[string]any{"expectedRevision": 1, "expectedDigest": saved.Artifact.Digest, "expectedSourceDigest": saved.Artifact.SourceDigest, "idempotencyKey": "artifact-revise-request-123456", "title": saved.Artifact.Title, "sections": sections, "gaps": saved.Artifact.Gaps, "assumptions": saved.Artifact.Assumptions}, 200)
	if err = json.Unmarshal(revised.Body.Bytes(), &saved); err != nil || saved.Artifact.Revision != 2 || saved.Artifact.State != "draft" {
		t.Fatal(revised.Body.String(), err)
	}
	review := controlTestRequest(t, h, "POST", artifactURL+"/review", map[string]any{"expectedRevision": 2, "expectedDigest": saved.Artifact.Digest, "idempotencyKey": "artifact-review-request-123456"}, 200)
	if err = json.Unmarshal(review.Body.Bytes(), &saved); err != nil || saved.Artifact.Revision != 3 || saved.Artifact.State != "reviewed" || saved.Artifact.Review == nil {
		t.Fatal(review.Body.String(), err)
	}
	for _, format := range []string{"docx", "pptx", "xlsx"} {
		exported := controlTestRequest(t, h, "GET", artifactURL+"/exports/"+format+"?digest="+saved.Artifact.Digest, nil, 200)
		content := artifactZipContent(t, exported.Body.Bytes())
		for _, value := range []string{"Saved product purpose", "Reviewed source-specific risk.", "No exact implementation-check receipt", "Source reference"} {
			if !strings.Contains(content, value) {
				t.Fatalf("%s export lost %q", format, value)
			}
		}
		if strings.Contains(content, "<f>") || strings.Contains(content, "TargetMode=\"External\"") {
			t.Fatal("export introduced executable spreadsheet or remote reference", format)
		}
	}
	controlTestRequest(t, h, "GET", artifactURL+"/exports/docx?digest="+strings.Repeat("0", 64), nil, 409)
	controlTestRequest(t, h, "GET", artifactURL+"/exports/pdf?digest="+saved.Artifact.Digest, nil, 422)
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
	staleExport := controlTestRequest(t, h, "GET", artifactURL+"/exports/docx?digest="+saved.Artifact.Digest, nil, 200)
	if !strings.Contains(artifactZipContent(t, staleExport.Body.Bytes()), "outdated") {
		t.Fatal("stale document export hid changed source")
	}
	controlTestRequest(t, h, "POST", artifactURL+"/review", map[string]any{"expectedRevision": 3, "expectedDigest": saved.Artifact.Digest, "idempotencyKey": "artifact-stale-review-123456"}, 409)
	_, retained, err := s.readArtifactLedger(p)
	if err != nil || len(retained.Records) != 1 || len(retained.Records[0].Revisions) != 3 {
		t.Fatal("stale refusal changed private history", err)
	}
}

func TestDistributePresentationKeepsBoundedUnicodeText(t *testing.T) {
	long := strings.Repeat("界", 400) + "& final"
	lines := []artifactExportLine{{Label: "Risk", Value: long}}
	pages, err := artifactSlideRows(lines)
	if err != nil || len(pages) < 2 {
		t.Fatal("long unicode text was not paginated", err, len(pages))
	}
	var retained strings.Builder
	for _, page := range pages {
		if len(page) > 7 {
			t.Fatal("slide exceeded row limit")
		}
		for _, row := range page {
			if len([]rune(row)) > 48 {
				t.Fatal("slide exceeded rune width")
			}
			retained.WriteString(row)
		}
	}
	if retained.String() != "Risk: "+long {
		t.Fatal("pagination lost source text")
	}
	data, err := artifactPPTX(context.Background(), lines)
	if err != nil || !strings.Contains(artifactZipContent(t, data), "& final") {
		t.Fatal("native presentation did not retain escaped text", err)
	}
	if _, err := artifactSlideRows([]artifactExportLine{{Label: "Risk", Value: strings.Repeat("a", 48*7*1024)}}); err == nil {
		t.Fatal("oversized presentation did not refuse")
	}
}

func TestDistributeArtifactSelectsOnlyExactAcceptedCheckAndRelease(t *testing.T) {
	check := ProjectRunCheckEvidence{CheckID: "check-one", Commit: strings.Repeat("a", 40), SourceDigest: strings.Repeat("b", 64), OriginalAvailable: true}
	release := ProjectRunReleaseEvidence{ReleaseID: "release-one", CheckID: check.CheckID, Commit: check.Commit, SourceDigest: check.SourceDigest}
	run := ProjectRun{ID: "run-one", Revision: 5, State: "complete", Checks: []ProjectRunCheckEvidence{check}, Releases: []ProjectRunReleaseEvidence{release}, Acceptance: &ProjectRunAcceptance{CheckID: check.CheckID, ReleaseID: release.ReleaseID, AcceptedAt: "2026-10-15T00:00:00Z"}}
	selected, err := acceptedArtifactEvidence(run)
	if err != nil || selected.RunID != "run-one" || selected.RunRevision != 5 || selected.CheckDigest != reviewDigest(check) || selected.ReleaseDigest != reviewDigest(release) {
		t.Fatal("accepted snapshot lost exact immutable evidence", selected, err)
	}
	run.Acceptance.ReleaseID = "another-release"
	if _, err := acceptedArtifactEvidence(run); err == nil {
		t.Fatal("unjoined release accepted")
	}
	run.Acceptance.ReleaseID = release.ReleaseID
	run.Checks[0].OriginalAvailable = false
	if _, err := acceptedArtifactEvidence(run); err == nil {
		t.Fatal("missing original check accepted")
	}
	run.Checks[0].OriginalAvailable = true
	run.State = "awaiting-release"
	if _, err := acceptedArtifactEvidence(run); err == nil {
		t.Fatal("unaccepted run accepted")
	}
}

func TestDistributeArtifactCapacityRefusalDoesNotPrepareOrPersist(t *testing.T) {
	s, h, id := controlTestHandler(t)
	s.SetDistributeWorkAdmission(func(context.Context) (func(), error) { return nil, errors.New("held host permit") })
	base := "/v1/product/projects/" + id + "/distribute/artifacts"
	controlTestRequest(t, h, "POST", base+"/prepare", map[string]string{"kind": "risk"}, 503)
	p, err := s.resolveProject(id)
	if err != nil {
		t.Fatal(err)
	}
	_, ledger, err := s.readArtifactLedger(p)
	if err != nil || len(ledger.Records) != 0 {
		t.Fatal("capacity refusal changed private artifact history", err)
	}
}

func TestDistributeArtifactKindsKeepDistinctEvidenceAndUnknowns(t *testing.T) {
	_, h, id := controlTestHandler(t)
	for _, tc := range []struct{ kind, section, gap string }{
		{"risk", "risk-register", "No exact implementation-check"},
		{"go-to-market", "launch-readiness", "No observed demand"},
		{"proposal", "scope", "No exact implementation-check"},
		{"pricing", "commercial-basis", "A real price quote"},
		{"presentation", "narrative", "No exact implementation-check"},
	} {
		response := controlTestRequest(t, h, "POST", "/v1/product/projects/"+id+"/distribute/artifacts/prepare", map[string]string{"kind": tc.kind}, 200)
		var prepared struct {
			Draft DistributeArtifact `json:"draft"`
		}
		if err := json.Unmarshal(response.Body.Bytes(), &prepared); err != nil {
			t.Fatal(err)
		}
		foundSection, foundGap := false, false
		for _, part := range prepared.Draft.Sections {
			if part.ID == tc.section {
				foundSection = true
			}
		}
		for _, gap := range prepared.Draft.Gaps {
			if strings.Contains(gap, tc.gap) {
				foundGap = true
			}
		}
		if !foundSection || !foundGap || prepared.Draft.Source.Checks != "not-selected" || prepared.Draft.Source.Release != "not-selected" {
			t.Fatalf("%s draft omitted its source/unknowns: %+v", tc.kind, prepared.Draft)
		}
	}
}
