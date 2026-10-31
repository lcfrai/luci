package product

import (
	"errors"
	"reflect"
	"strings"
	"time"
	"unicode/utf8"
)

// Imported aggregates are user-reported evidence. A file digest attributes the
// source selected by the person; it does not authenticate an external provider.
type GTMMetricRow struct {
	Row      int    `json:"row"`
	Metric   string `json:"metric"`
	Start    string `json:"start"`
	End      string `json:"end"`
	Value    int64  `json:"value"`
	Currency string `json:"currency"`
	Decimals int    `json:"decimals"`
}
type GTMMetricImport struct {
	Version      int            `json:"version"`
	SourceName   string         `json:"sourceName"`
	SourceSHA256 string         `json:"sourceSha256"`
	SourceBytes  int            `json:"sourceBytes"`
	Rows         []GTMMetricRow `json:"rows"`
}

func gtmMetricImportsValue(value *[]GTMMetricImport) []GTMMetricImport {
	if value == nil {
		return nil
	}
	return *value
}

func validGTMMetricImports(value *[]GTMMetricImport) bool {
	if value == nil {
		return true
	}
	if *value == nil {
		return false
	}
	imports := *value
	if len(imports) > 8 {
		return false
	}
	sources := map[string]bool{}
	total := 0
	for _, batch := range imports {
		if batch.Version != 1 || !hexDigest(batch.SourceSHA256) || sources[batch.SourceSHA256] || batch.SourceBytes < 1 || batch.SourceBytes > 128*1024 || len(batch.SourceName) < 1 || len(batch.SourceName) > 120 || !utf8.ValidString(batch.SourceName) || strings.TrimSpace(batch.SourceName) != batch.SourceName || strings.ContainsAny(batch.SourceName, "/\\") || !validArtifactStoredText(batch.SourceName) || strings.ContainsRune(batch.SourceName, 127) || strings.ContainsAny(batch.SourceName, "\r\n\t") || len(batch.Rows) < 1 || len(batch.Rows) > 128 {
			return false
		}
		sources[batch.SourceSHA256] = true
		total += len(batch.Rows)
		if total > 128 {
			return false
		}
		keys := map[string]bool{}
		priorRow := 1
		for _, row := range batch.Rows {
			if row.Row <= priorRow || row.Row > 129 || row.Value < 0 || row.Value > 1_000_000_000_000 {
				return false
			}
			priorRow = row.Row
			start, e1 := time.Parse("2006-01-02", row.Start)
			end, e2 := time.Parse("2006-01-02", row.End)
			if e1 != nil || e2 != nil || end.Before(start) || end.Sub(start) > 3660*24*time.Hour {
				return false
			}
			switch row.Metric {
			case "visitors", "signups", "activations", "purchases":
				if row.Currency != "" || row.Decimals != 0 || row.Value > 1_000_000_000 {
					return false
				}
			case "gross_revenue", "refunds", "spend":
				if len(row.Currency) != 3 || row.Decimals < 0 || row.Decimals > 6 {
					return false
				}
				for _, c := range row.Currency {
					if c < 'A' || c > 'Z' {
						return false
					}
				}
			default:
				return false
			}
			key := row.Metric + "|" + row.Start + "|" + row.End + "|" + row.Currency
			if keys[key] {
				return false
			}
			keys[key] = true
		}
	}
	return true
}

func preserveGTMMetricImports(saved, proposed *[]GTMMetricImport) error {
	if len(gtmMetricImportsValue(saved)) == 0 && len(gtmMetricImportsValue(proposed)) == 0 {
		return nil
	}
	if !reflect.DeepEqual(saved, proposed) {
		return errors.New("Luci cannot create, alter or remove imported results. Preserve the exact saved imports; manage them in Go to market.")
	}
	return nil
}
