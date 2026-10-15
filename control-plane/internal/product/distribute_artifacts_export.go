package product

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/management"
)

const distributeExportLimit = 16 << 20

type artifactExportLine struct{ Label, Value string }

// Every saved claim, source reference and limitation becomes an ordinary text
// line. Exporters neither evaluate formula syntax nor infer missing evidence.
func artifactExportLines(row DistributeArtifact, freshness string) []artifactExportLine {
	lines := []artifactExportLine{{"Title", row.Title}, {"Kind", row.Kind}, {"Review state", row.State}, {"Source freshness", freshness}, {"Artifact revision", strconv.Itoa(row.Revision)}, {"Artifact SHA-256", row.Digest}, {"Source SHA-256", row.SourceDigest}}
	add := func(label, value string) { lines = append(lines, artifactExportLine{label, value}) }
	add("Brief", fmt.Sprintf("revision %d · %s", row.Source.BriefRevision, row.Source.BriefDigest))
	add("Specification", fmt.Sprintf("revision %d · %s", row.Source.SpecificationRevision, row.Source.SpecificationDigest))
	add("Design", fmt.Sprintf("revision %d · %s", row.Source.DesignRevision, row.Source.DesignDigest))
	add("Plan", fmt.Sprintf("revision %d · %s", row.Source.PlanRevision, row.Source.PlanDigest))
	if row.Source.TeamID != "" {
		add("Team controls", fmt.Sprintf("%s · revision %d · %s", row.Source.TeamID, row.Source.LibraryRevision, row.Source.LibraryDigest))
		add("Project assessment", fmt.Sprintf("revision %d · %s", row.Source.AssessmentRevision, row.Source.AssessmentDigest))
	}
	if selected := row.Source.SelectedEvidence; selected != nil {
		add("Accepted run", selected.RunID)
		add("Exact check", selected.CheckID+" · "+selected.CheckDigest)
		add("Observed release", selected.ReleaseID+" · "+selected.ReleaseDigest)
		add("Candidate commit", selected.Commit)
		add("Accepted at", selected.AcceptedAt)
	} else {
		add("Implementation check", "Not selected")
		add("Release evidence", "Not selected")
	}
	if row.Review != nil {
		add("Reviewed by", row.Review.Actor.DisplayName)
		add("Reviewed at", row.Review.At)
		add("Reviewed draft SHA-256", row.Review.Digest)
	}
	add("Use", "Private project review record. A draft or stale source is not a verified delivery, public release, price quote or compliance certification.")
	for _, section := range row.Sections {
		add("Section: "+section.Heading, section.Body)
		for _, bullet := range section.Bullets {
			add("Key point", bullet)
		}
		for _, ref := range section.SourceRefs {
			add("Source reference", ref)
		}
	}
	for _, gap := range row.Gaps {
		add("Evidence gap", gap)
	}
	for _, assumption := range row.Assumptions {
		add("Assumption", assumption)
	}
	return lines
}

func artifactEscape(value string) (string, error) {
	if !validArtifactXMLText(value) {
		return "", errors.New("export text contains an invalid character")
	}
	var out bytes.Buffer
	if err := xml.EscapeText(&out, []byte(value)); err != nil {
		return "", err
	}
	return out.String(), nil
}

func artifactZip(ctx context.Context, parts []struct{ name, body string }) ([]byte, error) {
	var output bytes.Buffer
	archive := zip.NewWriter(&output)
	for _, part := range parts {
		if err := ctx.Err(); err != nil {
			_ = archive.Close()
			return nil, err
		}
		if len(part.name) > 200 || len(part.body) > distributeExportLimit || output.Len() > distributeExportLimit {
			_ = archive.Close()
			return nil, errArtifact
		}
		header := &zip.FileHeader{Name: part.name, Method: zip.Deflate}
		header.SetModTime(time.Unix(0, 0).UTC())
		writer, err := archive.CreateHeader(header)
		if err != nil {
			_ = archive.Close()
			return nil, err
		}
		if _, err = writer.Write([]byte(part.body)); err != nil {
			_ = archive.Close()
			return nil, err
		}
	}
	if err := archive.Close(); err != nil {
		return nil, err
	}
	if output.Len() > distributeExportLimit {
		return nil, errArtifact
	}
	return output.Bytes(), nil
}

func artifactXLSX(ctx context.Context, lines []artifactExportLine) ([]byte, error) {
	var sheet bytes.Buffer
	sheet.WriteString(`<?xml version="1.0" encoding="UTF-8"?><worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><sheetData>`)
	for i, line := range lines {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if len(line.Label) > 32767 || len(line.Value) > 32767 {
			return nil, errArtifact
		}
		label, err := artifactEscape(line.Label)
		if err != nil {
			return nil, err
		}
		value, err := artifactEscape(line.Value)
		if err != nil {
			return nil, err
		}
		fmt.Fprintf(&sheet, `<row r="%d"><c r="A%d" t="inlineStr"><is><t xml:space="preserve">%s</t></is></c><c r="B%d" t="inlineStr"><is><t xml:space="preserve">%s</t></is></c></row>`, i+1, i+1, label, i+1, value)
		if sheet.Len() > distributeExportLimit {
			return nil, errArtifact
		}
	}
	sheet.WriteString(`</sheetData></worksheet>`)
	parts := []struct{ name, body string }{
		{`[Content_Types].xml`, `<?xml version="1.0" encoding="UTF-8"?><Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Default Extension="xml" ContentType="application/xml"/><Override PartName="/xl/workbook.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml"/><Override PartName="/xl/worksheets/sheet1.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.worksheet+xml"/></Types>`},
		{`_rels/.rels`, `<?xml version="1.0" encoding="UTF-8"?><Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="xl/workbook.xml"/></Relationships>`},
		{`xl/workbook.xml`, `<?xml version="1.0" encoding="UTF-8"?><workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><sheets><sheet name="Artifact" sheetId="1" r:id="rId1"/></sheets></workbook>`},
		{`xl/_rels/workbook.xml.rels`, `<?xml version="1.0" encoding="UTF-8"?><Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/sheet1.xml"/></Relationships>`},
		{`xl/worksheets/sheet1.xml`, sheet.String()},
	}
	return artifactZip(ctx, parts)
}

func artifactDOCX(ctx context.Context, lines []artifactExportLine) ([]byte, error) {
	var doc bytes.Buffer
	doc.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?><w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:body>`)
	for _, line := range lines {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		for i, part := range strings.Split(line.Value, "\n") {
			label := line.Label
			if i > 0 {
				label = "Continuation"
			}
			text, err := artifactEscape(label + ": " + part)
			if err != nil {
				return nil, err
			}
			doc.WriteString(`<w:p><w:r><w:t xml:space="preserve">` + text + `</w:t></w:r></w:p>`)
			if doc.Len() > distributeExportLimit {
				return nil, errArtifact
			}
		}
	}
	doc.WriteString(`<w:sectPr/></w:body></w:document>`)
	parts := []struct{ name, body string }{
		{`[Content_Types].xml`, `<?xml version="1.0" encoding="UTF-8"?><Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Default Extension="xml" ContentType="application/xml"/><Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/></Types>`},
		{`_rels/.rels`, `<?xml version="1.0" encoding="UTF-8"?><Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/></Relationships>`},
		{`word/document.xml`, doc.String()},
	}
	return artifactZip(ctx, parts)
}

func (s *Server) exportDistributeArtifact(w http.ResponseWriter, r *http.Request) {
	if !artifactAccess(w, r, false) {
		return
	}
	p, ok := s.requestProject(w, r)
	if !ok {
		return
	}
	format := r.PathValue("format")
	if format != "docx" && format != "xlsx" {
		journeyError(w, 422, "artifact_format_unavailable", "This format has no verified native exporter yet. Choose DOCX or XLSX.")
		return
	}
	digest := r.URL.Query().Get("digest")
	if !hexDigest(digest) {
		journeyError(w, 400, "artifact_digest_required", "Open the exact saved artifact before exporting its revision.")
		return
	}
	s.mu.RLock()
	s.teamControlsMu.Lock()
	_, ledger, err := s.readArtifactLedger(p)
	if err != nil {
		s.teamControlsMu.Unlock()
		s.mu.RUnlock()
		artifactProblem(w, err)
		return
	}
	record := artifactRecord(&ledger, r.PathValue("artifactId"))
	if record == nil {
		s.teamControlsMu.Unlock()
		s.mu.RUnlock()
		journeyError(w, 404, "artifact_unavailable", "This artifact is not retained in the current project.")
		return
	}
	row := record.Revisions[len(record.Revisions)-1]
	access, scoped := management.TeamAccessFromContext(r.Context())
	if row.Source.TeamID != "" && (!scoped || access.Team.ID != row.Source.TeamID || !access.Permissions.Read || access.ProjectID != p.ID) {
		s.teamControlsMu.Unlock()
		s.mu.RUnlock()
		journeyError(w, 403, "artifact_scope_changed", "Current team access does not cover this artifact.")
		return
	}
	if row.Digest != digest {
		s.teamControlsMu.Unlock()
		s.mu.RUnlock()
		journeyError(w, 409, "artifact_changed", "The saved artifact revision changed. Review the current version before exporting.")
		return
	}
	selectedRun := ""
	if row.Source.SelectedEvidence != nil {
		selectedRun = row.Source.SelectedEvidence.RunID
	}
	source, _, _, _, sourceErr := s.artifactSource(r.Context(), p, selectedRun)
	s.teamControlsMu.Unlock()
	s.mu.RUnlock()
	freshness := "unavailable"
	if sourceErr == nil {
		freshness = artifactCurrentView(row, source)["freshness"].(string)
	}
	lines := artifactExportLines(row, freshness)
	var data []byte
	if format == "docx" {
		data, err = artifactDOCX(r.Context(), lines)
	} else {
		data, err = artifactXLSX(r.Context(), lines)
	}
	if err != nil {
		artifactProblem(w, err)
		return
	}
	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="luci-%s-r%d.%s"`, row.ID, row.Revision, format))
	if format == "docx" {
		w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.wordprocessingml.document")
	} else {
		w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	}
	w.Header().Set("Content-Length", strconv.Itoa(len(data)))
	_, _ = w.Write(data)
}
