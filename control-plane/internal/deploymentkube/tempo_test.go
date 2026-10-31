package deploymentkube

import (
	"encoding/json"
	"strconv"
	"strings"
	"testing"
	"time"
)

func tempoTraceFixture(t *testing.T, start, end time.Time, namespace string) tempoTrace {
	t.Helper()
	attribute := func(key, value string) map[string]any {
		return map[string]any{"key": key, "value": map[string]any{"stringValue": value}}
	}
	span := map[string]any{"traceId": strings.Repeat("1", 32), "spanId": strings.Repeat("2", 16), "startTimeUnixNano": strconv.FormatInt(start.UnixNano(), 10), "endTimeUnixNano": strconv.FormatInt(end.UnixNano(), 10), "status": map[string]any{"code": 2}}
	data := map[string]any{"resourceSpans": []any{map[string]any{"resource": map[string]any{"attributes": []any{attribute("service.namespace", namespace), attribute("service.name", "api"), attribute("service.instance.id", "pod-original")}}, "scopeSpans": []any{map[string]any{"spans": []any{span}}}}}}
	raw, err := json.Marshal(data)
	if err != nil {
		t.Fatal(err)
	}
	var trace tempoTrace
	if err = json.Unmarshal(raw, &trace); err != nil {
		t.Fatal(err)
	}
	return trace
}

func TestTempoWholeTraceSkipsValidOutsideWindowSpans(t *testing.T) {
	now := time.Date(2026, 10, 31, 4, 0, 0, 0, time.UTC)
	start := now.Add(-15 * time.Minute)
	older := tempoTraceFixture(t, now.Add(-2*time.Hour), now.Add(-90*time.Minute), "project-original")
	result, err := summarizeTempoTrace(older, strings.Repeat("1", 32), "project-original", "api", "pod-original", start, now)
	if err != nil || result.SpanCount != 0 {
		t.Fatal("valid old parent refused whole trace", result, err)
	}
	crossing := tempoTraceFixture(t, now.Add(-2*time.Hour), now.Add(-5*time.Minute), "project-original")
	result, err = summarizeTempoTrace(crossing, strings.Repeat("1", 32), "project-original", "api", "pod-original", start, now)
	if err != nil || result.SpanCount != 1 || result.ErrorSpans != 1 {
		t.Fatal("long valid overlapping span refused", result, err)
	}
}
func TestTempoNeverWidensMissingInstanceAttribution(t *testing.T) {
	now := time.Date(2026, 10, 31, 4, 0, 0, 0, time.UTC)
	trace := tempoTraceFixture(t, now.Add(-time.Minute), now, "foreign-project")
	result, err := summarizeTempoTrace(trace, strings.Repeat("1", 32), "project-original", "api", "pod-original", now.Add(-time.Hour), now)
	if err != nil || result.SpanCount != 0 {
		t.Fatal("foreign service namespace was adopted", result, err)
	}
	if _, ok := otlpHex(strings.Repeat("0", 32), 16); ok {
		t.Fatal("invalid all-zero trace identity accepted")
	}
}
