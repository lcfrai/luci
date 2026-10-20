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

// A slide has seven short text rows. Splitting by rune keeps every saved
// character, including non-ASCII text, while bounding the visible layout.
// Long fields continue on further slides rather than disappearing in a
// clipped textbox or an Office autosize implementation detail.
func artifactSlideRows(lines []artifactExportLine) ([][]string, error) {
	const width, height, slideLimit = 48, 7, 1024
	var slides [][]string
	for _, line := range lines {
		if !validArtifactXMLText(line.Label) || !validArtifactXMLText(line.Value) {
			return nil, errArtifact
		}
		runes := []rune(line.Label + ": " + line.Value)
		for start := 0; start < len(runes); start += width {
			if len(slides) == 0 || len(slides[len(slides)-1]) == height {
				if len(slides) == slideLimit {
					return nil, errArtifact
				}
				slides = append(slides, []string{})
			}
			end := start + width
			if end > len(runes) {
				end = len(runes)
			}
			slides[len(slides)-1] = append(slides[len(slides)-1], string(runes[start:end]))
		}
	}
	return slides, nil
}

func artifactPPTX(ctx context.Context, lines []artifactExportLine) ([]byte, error) {
	slides, err := artifactSlideRows(lines)
	if err != nil {
		return nil, err
	}
	var contentTypes, presentation, presentationRels bytes.Buffer
	contentTypes.WriteString(`<?xml version="1.0" encoding="UTF-8"?><Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Default Extension="xml" ContentType="application/xml"/><Override PartName="/ppt/presentation.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.presentation.main+xml"/>`)
	presentation.WriteString(`<?xml version="1.0" encoding="UTF-8"?><p:presentation xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><p:sldIdLst>`)
	presentationRels.WriteString(`<?xml version="1.0" encoding="UTF-8"?><Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">`)
	parts := []struct{ name, body string }{
		{`_rels/.rels`, `<?xml version="1.0" encoding="UTF-8"?><Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="ppt/presentation.xml"/></Relationships>`},
	}
	for i, rows := range slides {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		n := i + 1
		fmt.Fprintf(&contentTypes, `<Override PartName="/ppt/slides/slide%d.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.slide+xml"/>`, n)
		fmt.Fprintf(&presentation, `<p:sldId id="%d" r:id="rId%d"/>`, 255+n, n)
		fmt.Fprintf(&presentationRels, `<Relationship Id="rId%d" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slide" Target="slides/slide%d.xml"/>`, n, n)
		var slide bytes.Buffer
		slide.WriteString(`<?xml version="1.0" encoding="UTF-8"?><p:sld xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"><p:cSld><p:spTree><p:nvGrpSpPr><p:cNvPr id="1" name=""/><p:cNvGrpSpPr/><p:nvPr/></p:nvGrpSpPr><p:grpSpPr/><p:sp><p:nvSpPr><p:cNvPr id="2" name="Artifact text"/><p:cNvSpPr/><p:nvPr/></p:nvSpPr><p:spPr><a:xfrm><a:off x="457200" y="457200"/><a:ext cx="11277600" cy="5943600"/></a:xfrm></p:spPr><p:txBody><a:bodyPr wrap="square"/><a:lstStyle/>`)
		for _, row := range rows {
			value, err := artifactEscape(row)
			if err != nil {
				return nil, err
			}
			slide.WriteString(`<a:p><a:pPr/><a:r><a:rPr lang="en-US" sz="1800"/><a:t>` + value + `</a:t></a:r><a:endParaRPr lang="en-US" sz="1800"/></a:p>`)
		}
		slide.WriteString(`</p:txBody></p:sp></p:spTree></p:cSld><p:clrMapOvr><a:masterClrMapping/></p:clrMapOvr></p:sld>`)
		if slide.Len() > distributeExportLimit {
			return nil, errArtifact
		}
		parts = append(parts, struct{ name, body string }{fmt.Sprintf("ppt/slides/slide%d.xml", n), slide.String()})
	}
	contentTypes.WriteString(`</Types>`)
	presentation.WriteString(`</p:sldIdLst><p:sldSz cx="12192000" cy="6858000"/><p:notesSz cx="6858000" cy="9144000"/></p:presentation>`)
	presentationRels.WriteString(`</Relationships>`)
	parts = append(parts,
		struct{ name, body string }{`[Content_Types].xml`, contentTypes.String()},
		struct{ name, body string }{`ppt/presentation.xml`, presentation.String()},
		struct{ name, body string }{`ppt/_rels/presentation.xml.rels`, presentationRels.String()},
	)
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
	if format != "docx" && format != "xlsx" && format != "pptx" && format != "pdf" {
		journeyError(w, 422, "artifact_format_unavailable", "Choose an available DOCX, PPTX, XLSX or PDF export.")
		return
	}
	if format == "pdf" && !artifactPDFAssetsAvailable() {
		journeyError(w, 503, "artifact_pdf_assets_unavailable", "The required PDF font bundle could not be verified. Your saved artifact is unchanged.")
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
	for _, line := range lines {
		if !validArtifactXMLText(line.Label) || !validArtifactXMLText(line.Value) {
			journeyError(w, 422, "artifact_export_unavailable", "This retained artifact contains text Office XML cannot preserve. Revise the text in a new draft before exporting.")
			return
		}
	}
	var data []byte
	if format == "docx" {
		data, err = artifactDOCX(r.Context(), lines)
	} else if format == "xlsx" {
		data, err = artifactXLSX(r.Context(), lines)
	} else if format == "pptx" {
		data, err = artifactPPTX(r.Context(), lines)
	} else {
		data, err = artifactPDF(r.Context(), lines)
	}
	if err != nil {
		if format == "pdf" {
			journeyError(w, 422, "artifact_pdf_unavailable", "The artifact could not be preserved within PDF font, layout or size limits. Your saved content is unchanged; use an Office export or revise the draft.")
			return
		}
		artifactProblem(w, err)
		return
	}
	if err := r.Context().Err(); err != nil {
		return
	}
	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="luci-%s-r%d.%s"`, row.ID, row.Revision, format))
	if format == "docx" {
		w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.wordprocessingml.document")
	} else if format == "xlsx" {
		w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	} else if format == "pptx" {
		w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.presentationml.presentation")
	} else {
		w.Header().Set("Content-Type", "application/pdf")
	}
	w.Header().Set("Content-Length", strconv.Itoa(len(data)))
	_, _ = w.Write(data)
}

func (s *Server) exportDistributePDFArtifact(w http.ResponseWriter, r *http.Request) {
	r.SetPathValue("format", "pdf")
	s.exportDistributeArtifact(w, r)
}

func artifactExportFormats() []string {
	formats := []string{"docx", "pptx", "xlsx"}
	if artifactPDFAssetsAvailable() {
		formats = append(formats, "pdf")
	}
	return formats
}
