package product

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"path"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

const (
	controlWorkbookLimit = 2 << 20
	controlExpandedLimit = 8 << 20
	controlMemberLimit   = 128
	controlSheetLimit    = 8
	controlRowLimit      = 2000
	controlColumnLimit   = 64
	controlCellLimit     = 4096
)

var errControlWorkbook = errors.New("This workbook cannot be safely imported. Use a small XLSX without formulas, macros, links or external data.")
var controlCellAddress = regexp.MustCompile(`^([A-Z]{1,2})([1-9][0-9]{0,6})$`)

type controlWorkbook struct {
	Sheets []controlSheet `json:"sheets"`
	Issues []string       `json:"issues"`
}
type controlSheet struct {
	Name    string          `json:"name"`
	Headers []controlHeader `json:"headers"`
	Rows    []controlRow    `json:"rows"`
}
type controlHeader struct {
	Column string `json:"column"`
	Name   string `json:"name"`
}
type controlRow struct {
	Number int               `json:"number"`
	Cells  map[string]string `json:"cells"`
}

type xlsxWorkbook struct {
	Sheets []struct {
		Name  string `xml:"name,attr"`
		RelID string `xml:"id,attr"`
	} `xml:"sheets>sheet"`
}
type xlsxRelationships struct {
	Entries []struct {
		ID     string `xml:"Id,attr"`
		Target string `xml:"Target,attr"`
		Mode   string `xml:"TargetMode,attr"`
		Type   string `xml:"Type,attr"`
	} `xml:"Relationship"`
}
type xlsxStrings struct {
	Items []struct {
		Text string `xml:"t"`
		Runs []struct {
			Text string `xml:"t"`
		} `xml:"r"`
	} `xml:"si"`
}
type xlsxSheet struct {
	Rows []struct {
		Number int `xml:"r,attr"`
		Cells  []struct {
			Ref     string  `xml:"r,attr"`
			Type    string  `xml:"t,attr"`
			Value   string  `xml:"v"`
			Inline  string  `xml:"is>t"`
			Formula *string `xml:"f"`
		} `xml:"c"`
	} `xml:"sheetData>row"`
	Hyperlinks []struct{} `xml:"hyperlinks>hyperlink"`
}

// All ZIP members are inspected before XML decoding. OOXML is treated as data:
// relationships cannot fetch files or URLs and formulas are reported but never
// evaluated. The aggregate expanded limit applies even to unused members.
func parseControlWorkbook(ctx context.Context, raw []byte) (controlWorkbook, error) {
	var result controlWorkbook
	if len(raw) == 0 || len(raw) > controlWorkbookLimit {
		return result, errControlWorkbook
	}
	archive, err := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
	if err != nil || len(archive.File) == 0 || len(archive.File) > controlMemberLimit {
		return result, errControlWorkbook
	}
	members := make(map[string][]byte, len(archive.File))
	total := uint64(0)
	for _, file := range archive.File {
		if ctx.Err() != nil {
			return result, ctx.Err()
		}
		name := file.Name
		_, duplicate := members[name]
		if name == "" || len(name) > 256 || strings.Contains(name, "\\") || strings.HasPrefix(name, "/") || path.Clean(name) != name || strings.HasSuffix(name, "/") || file.Mode()&^(0666) != 0 || duplicate {
			return result, errControlWorkbook
		}
		lower := strings.ToLower(name)
		if strings.Contains(lower, "vbaproject") || strings.Contains(lower, "externallink") || strings.Contains(lower, "embeddings/") || strings.Contains(lower, "connections.xml") || strings.HasSuffix(lower, ".bin") {
			return result, errControlWorkbook
		}
		total += file.UncompressedSize64
		if file.UncompressedSize64 > controlExpandedLimit || total > controlExpandedLimit {
			return result, errControlWorkbook
		}
		reader, openErr := file.Open()
		if openErr != nil {
			return result, errControlWorkbook
		}
		body, readErr := io.ReadAll(io.LimitReader(reader, int64(file.UncompressedSize64)+1))
		closeErr := reader.Close()
		if readErr != nil || closeErr != nil || uint64(len(body)) != file.UncompressedSize64 {
			return result, errControlWorkbook
		}
		members[name] = body
	}
	for name, body := range members {
		if strings.HasSuffix(strings.ToLower(name), ".rels") {
			var links xlsxRelationships
			if xml.Unmarshal(body, &links) != nil {
				return result, errControlWorkbook
			}
			for _, link := range links.Entries {
				if link.Mode != "" {
					return result, errControlWorkbook
				}
			}
		}
	}
	if bytes.Contains(bytes.ToLower(members["[Content_Types].xml"]), []byte("macroenabled")) {
		return result, errControlWorkbook
	}
	var workbook xlsxWorkbook
	var rels xlsxRelationships
	var shared xlsxStrings
	if xml.Unmarshal(members["xl/workbook.xml"], &workbook) != nil || xml.Unmarshal(members["xl/_rels/workbook.xml.rels"], &rels) != nil || len(workbook.Sheets) == 0 || len(workbook.Sheets) > controlSheetLimit {
		return result, errControlWorkbook
	}
	if body := members["xl/sharedStrings.xml"]; len(body) > 0 && xml.Unmarshal(body, &shared) != nil {
		return result, errControlWorkbook
	}
	if len(shared.Items) > controlRowLimit*controlColumnLimit {
		return result, errControlWorkbook
	}
	byID := map[string]string{}
	for _, rel := range rels.Entries {
		if rel.Mode != "" || strings.HasPrefix(rel.Target, "/") || strings.Contains(rel.Target, "\\") || strings.Contains(rel.Target, "..") || strings.Contains(rel.Target, ":") {
			return result, errControlWorkbook
		}
		if strings.HasSuffix(rel.Type, "/worksheet") {
			name := path.Clean(path.Join("xl", rel.Target))
			if rel.ID == "" || byID[rel.ID] != "" || !strings.HasPrefix(name, "xl/worksheets/") {
				return result, errControlWorkbook
			}
			byID[rel.ID] = name
		}
	}
	seenSheets := map[string]bool{}
	for _, selected := range workbook.Sheets {
		if ctx.Err() != nil {
			return result, ctx.Err()
		}
		if selected.Name == "" || len(selected.Name) > 100 || seenSheets[selected.Name] || byID[selected.RelID] == "" {
			return result, errControlWorkbook
		}
		seenSheets[selected.Name] = true
		var parsed xlsxSheet
		if xml.Unmarshal(members[byID[selected.RelID]], &parsed) != nil || len(parsed.Rows) > controlRowLimit || len(parsed.Hyperlinks) != 0 {
			return result, errControlWorkbook
		}
		sheet := controlSheet{Name: selected.Name, Headers: []controlHeader{}, Rows: []controlRow{}}
		prior := 0
		for _, row := range parsed.Rows {
			if row.Number < 1 || row.Number > controlRowLimit || row.Number <= prior || len(row.Cells) > controlColumnLimit {
				return result, errControlWorkbook
			}
			prior = row.Number
			cells := map[string]string{}
			for _, cell := range row.Cells {
				match := controlCellAddress.FindStringSubmatch(cell.Ref)
				if match == nil {
					return result, errControlWorkbook
				}
				col := match[1]
				n, _ := strconv.Atoi(match[2])
				_, duplicateCell := cells[col]
				if n != row.Number || columnIndex(col) > controlColumnLimit || duplicateCell {
					return result, errControlWorkbook
				}
				value := cell.Value
				switch cell.Type {
				case "", "n", "str":
				case "inlineStr":
					value = cell.Inline
				case "s":
					index, e := strconv.Atoi(value)
					if e != nil || index < 0 || index >= len(shared.Items) {
						return result, errControlWorkbook
					}
					value = shared.Items[index].Text
					for _, run := range shared.Items[index].Runs {
						value += run.Text
					}
				default:
					return result, errControlWorkbook
				}
				if cell.Formula != nil {
					result.Issues = append(result.Issues, fmt.Sprintf("%s!%s contains a formula", selected.Name, cell.Ref))
				}
				if len(value) > controlCellLimit || strings.ContainsRune(value, 0) {
					return result, errControlWorkbook
				}
				cells[col] = value
			}
			if row.Number == 1 {
				for col, value := range cells {
					sheet.Headers = append(sheet.Headers, controlHeader{col, value})
				}
				sort.Slice(sheet.Headers, func(i, j int) bool {
					return columnIndex(sheet.Headers[i].Column) < columnIndex(sheet.Headers[j].Column)
				})
			} else {
				sheet.Rows = append(sheet.Rows, controlRow{row.Number, cells})
			}
		}
		result.Sheets = append(result.Sheets, sheet)
	}
	return result, nil
}

func columnIndex(value string) int {
	n := 0
	for _, letter := range value {
		n = n*26 + int(letter-'A'+1)
	}
	return n
}
