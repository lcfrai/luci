package product

import (
	"encoding/json"
	"strings"
	"testing"
)

func metricImportFixture() *[]GTMMetricImport {
	v := []GTMMetricImport{{Version: 1, SourceName: "results.csv", SourceSHA256: strings.Repeat("a", 64), SourceBytes: 180, Rows: []GTMMetricRow{{Row: 2, Metric: "gross_revenue", Start: "2026-01-01", End: "2026-01-31", Value: 125000, Currency: "USD", Decimals: 2}}}}
	return &v
}
func TestGTMMetricImportsValidateUnitsBoundsAndLegacyEncoding(t *testing.T) {
	c := gtmFixtureContent()
	old, _ := json.Marshal(c)
	if strings.Contains(string(old), "metricImports") {
		t.Fatal("legacy encoding changed")
	}
	c.MetricImports = metricImportFixture()
	if err := validateGTM(c); err != nil {
		t.Fatal(err)
	}
	for _, mutate := range []func(*GTMMetricImport){
		func(b *GTMMetricImport) { b.Rows[0].Metric = "purchase_amount_inferred" },
		func(b *GTMMetricImport) { b.Rows[0].Metric = "visitors" },
		func(b *GTMMetricImport) { b.Rows[0].End = "2025-01-01" },
		func(b *GTMMetricImport) { b.Rows[0].Value = 1_000_000_000_001 },
		func(b *GTMMetricImport) {
			b.Rows = append(b.Rows, GTMMetricRow{Row: 3, Metric: "gross_revenue", Start: "2026-01-01", End: "2026-01-31", Currency: "USD", Decimals: 2})
		},
		func(b *GTMMetricImport) { b.SourceName = "../private.csv" },
	} {
		batch := metricImportFixture()
		mutate(&(*batch)[0])
		if validGTMMetricImports(batch) {
			t.Fatal("invalid aggregate accepted")
		}
	}
	batches := append(*metricImportFixture(), (*metricImportFixture())[0])
	if validGTMMetricImports(&batches) {
		t.Fatal("duplicate source accepted")
	}
}
func TestGTMImportedResultsCannotBeInventedChangedOrErasedByModel(t *testing.T) {
	saved := gtmFixtureContent()
	saved.MetricImports = metricImportFixture()
	snapshot := OrchestratorContext{GTM: &OrchestratorGTMContext{SourceDigest: strings.Repeat("b", 64), Content: saved, ArtifactLinks: []GTMArtifactLink{}}}
	same := saved
	if err := validateOrchestratorGTM(snapshot, &same); err != nil {
		t.Fatal(err)
	}
	removed := gtmFixtureContent()
	if validateOrchestratorGTM(snapshot, &removed) == nil {
		t.Fatal("omitted result erased")
	}
	changed := gtmFixtureContent()
	changed.MetricImports = metricImportFixture()
	(*changed.MetricImports)[0].Rows[0].Value++
	if validateOrchestratorGTM(snapshot, &changed) == nil {
		t.Fatal("invented amount accepted")
	}
	snapshot.GTM.Content = gtmFixtureContent()
	if validateOrchestratorGTM(snapshot, &saved) == nil {
		t.Fatal("model-created source accepted")
	}
}
func TestGTMMetricSaveReplayAndExplicitClear(t *testing.T) {
	_, h, id := controlTestHandler(t)
	base := "/v1/product/projects/" + id + "/distribute/gtm"
	var initial GTMView
	if err := json.Unmarshal(controlTestRequest(t, h, "GET", base, nil, 200).Body.Bytes(), &initial); err != nil {
		t.Fatal(err)
	}
	content := gtmFixtureContent()
	content.MetricImports = metricImportFixture()
	req := map[string]any{"expectedRevision": 0, "expectedDigest": "", "expectedSourceDigest": initial.SourceDigest, "idempotencyKey": "metric-import-save-123456", "content": content}
	var saved GTMView
	if err := json.Unmarshal(controlTestRequest(t, h, "POST", base, req, 200).Body.Bytes(), &saved); err != nil {
		t.Fatal(err)
	}
	controlTestRequest(t, h, "POST", base, req, 200)
	req["expectedRevision"], req["expectedDigest"], req["idempotencyKey"] = 1, saved.Workspace.Digest, "metric-import-next-123456"
	req["content"] = gtmFixtureContent()
	controlTestRequest(t, h, "POST", base, req, 422)
	empty := []GTMMetricImport{}
	content.MetricImports = &empty
	req["content"] = content
	var cleared GTMView
	if err := json.Unmarshal(controlTestRequest(t, h, "POST", base, req, 200).Body.Bytes(), &cleared); err != nil {
		t.Fatal(err)
	}
	if cleared.Workspace.Revision != 2 || len(gtmMetricImportsValue(cleared.Workspace.Content.MetricImports)) != 0 || len(cleared.History) != 2 {
		t.Fatal("explicit clear/history lost")
	}
}
