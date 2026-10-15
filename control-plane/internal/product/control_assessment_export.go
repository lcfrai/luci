package product

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/xml"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const controlReportLimit = 16 << 20

// This native OOXML export uses inline string cells only. Imported values are
// never emitted as formulas, links or external references. It is a project
// assessment record, not a claim of certification or verified delivery.
func controlReportXLSX(ctx context.Context, projectID, state string, library TeamControlRevision, assessment ControlAssessment) ([]byte, error) {
	if len(library.Controls) > controlRowLimit || len(assessment.Results) != len(library.Controls) {
		return nil, errControlWorkbook
	}
	results := make(map[string]ControlAssessmentResult, len(assessment.Results))
	for _, result := range assessment.Results {
		if _, duplicate := results[result.ControlID]; duplicate {
			return nil, errControlWorkbook
		}
		results[result.ControlID] = result
	}
	var sheet bytes.Buffer
	sheet.WriteString(`<?xml version="1.0" encoding="UTF-8"?><worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><sheetData>`)
	rows := [][]string{
		{"Project", projectID}, {"Assessment revision", strconv.Itoa(assessment.Revision)},
		{"Assessment status", state}, {"Library revision", strconv.Itoa(library.Revision)},
		{"Library SHA-256", library.Digest}, {"Imported workbook SHA-256", library.SourceSHA256},
		{"Project context SHA-256", assessment.ContextDigest},
		{"Assessment actor", assessment.ActorID}, {"Assessment recorded at", assessment.At},
		{"Review notice", "Reviewer observations only. Unknown or stale evidence is not verified compliance."},
		{"Control ID", "Requirement", "Category", "Applicability", "Owner", "Evidence expected", "Assessment", "Reviewer note", "Document segments", "Source sheet", "Source row"},
	}
	for _, control := range library.Controls {
		result, ok := results[control.ID]
		if !ok {
			return nil, errControlWorkbook
		}
		links := make([]string, 0, len(result.EvidenceRefs))
		for _, link := range result.EvidenceRefs {
			links = append(links, link.DocumentID+" / "+link.SegmentID)
		}
		rows = append(rows, []string{control.ID, control.Requirement, control.Category, control.Applicability, control.Owner, control.EvidenceExpected, result.Status, result.Note, strings.Join(links, "; "), control.Sheet, strconv.Itoa(control.Row)})
	}
	for rowIndex, row := range rows {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		fmt.Fprintf(&sheet, `<row r="%d">`, rowIndex+1)
		for column, value := range row {
			if len(value) > 32767 || strings.ContainsRune(value, 0) {
				return nil, errControlWorkbook
			}
			fmt.Fprintf(&sheet, `<c r="%s%d" t="inlineStr"><is><t xml:space="preserve">`, reportColumn(column), rowIndex+1)
			if err := xml.EscapeText(&sheet, []byte(value)); err != nil {
				return nil, err
			}
			sheet.WriteString(`</t></is></c>`)
		}
		sheet.WriteString(`</row>`)
		if sheet.Len() > controlReportLimit {
			return nil, errControlWorkbook
		}
	}
	sheet.WriteString(`</sheetData></worksheet>`)
	entries := []struct{ name, body string }{
		{`[Content_Types].xml`, `<?xml version="1.0" encoding="UTF-8"?><Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Default Extension="xml" ContentType="application/xml"/><Override PartName="/xl/workbook.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml"/><Override PartName="/xl/worksheets/sheet1.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.worksheet+xml"/></Types>`},
		{`_rels/.rels`, `<?xml version="1.0" encoding="UTF-8"?><Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="xl/workbook.xml"/></Relationships>`},
		{`xl/workbook.xml`, `<?xml version="1.0" encoding="UTF-8"?><workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><sheets><sheet name="Assessment" sheetId="1" r:id="rId1"/></sheets></workbook>`},
		{`xl/_rels/workbook.xml.rels`, `<?xml version="1.0" encoding="UTF-8"?><Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/sheet1.xml"/></Relationships>`},
		{`xl/worksheets/sheet1.xml`, sheet.String()},
	}
	var output bytes.Buffer
	archive := zip.NewWriter(&output)
	for _, entry := range entries {
		if ctx.Err() != nil {
			archive.Close()
			return nil, ctx.Err()
		}
		header := &zip.FileHeader{Name: entry.name, Method: zip.Deflate}
		header.SetModTime(time.Unix(0, 0).UTC())
		writer, err := archive.CreateHeader(header)
		if err != nil {
			archive.Close()
			return nil, err
		}
		if _, err = writer.Write([]byte(entry.body)); err != nil {
			archive.Close()
			return nil, err
		}
	}
	if err := archive.Close(); err != nil || output.Len() > controlReportLimit {
		return nil, errControlWorkbook
	}
	return output.Bytes(), nil
}

func reportColumn(index int) string {
	if index < 26 {
		return string(rune('A' + index))
	}
	return string([]rune{rune('A' + index/26 - 1), rune('A' + index%26)})
}

func (s *Server) exportControlAssessment(w http.ResponseWriter, r *http.Request) {
	p, access, ok := s.assessmentReadAccess(w, r, false)
	if !ok {
		return
	}
	s.mu.RLock()
	_, saved, err := s.readControlAssessment(p)
	contextDigest, contextErr := s.currentAssessmentContext(p)
	s.mu.RUnlock()
	if err != nil || contextErr != nil || saved.Revision == 0 || saved.TeamID != access.Team.ID {
		journeyError(w, 409, "assessment_unavailable", "Save and review an assessment before exporting it.")
		return
	}
	s.teamControlsMu.Lock()
	_, ledger, libraryErr := s.readTeamControls(access.Team.ID)
	s.teamControlsMu.Unlock()
	if libraryErr != nil {
		journeyError(w, 409, "library_unavailable", "The source library cannot be verified.")
		return
	}
	var source *TeamControlRevision
	for index := range ledger.Revisions {
		row := &ledger.Revisions[index]
		if row.Revision == saved.LibraryRevision && row.Digest == saved.LibraryDigest {
			source = row
			break
		}
	}
	if source == nil {
		journeyError(w, 409, "library_unavailable", "The exact source library revision is no longer retained.")
		return
	}
	state := "current"
	latest := latestControls(ledger)
	if latest == nil || latest.Revision != saved.LibraryRevision || latest.Digest != saved.LibraryDigest || contextDigest != saved.ContextDigest {
		state = "stale"
	}
	data, err := controlReportXLSX(r.Context(), p.ID, state, *source, saved)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			return
		}
		journeyError(w, 409, "report_unavailable", "The bounded assessment report could not be produced.")
		return
	}
	sum := sha256.Sum256(data)
	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	w.Header().Set("Content-Disposition", `attachment; filename="luci-control-assessment.xlsx"`)
	w.Header().Set("X-Content-SHA256", hex.EncodeToString(sum[:]))
	w.Header().Set("X-Luci-Assessment-State", state)
	w.Header().Set("Content-Length", strconv.Itoa(len(data)))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}
