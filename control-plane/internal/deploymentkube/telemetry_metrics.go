package deploymentkube

import (
	"bufio"
	"fmt"
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/lcfrai/luci/control-plane/internal/deploymentplan"
)

const telemetryBodyLimit = 1 << 20
const telemetrySampleLimit = 256

type ProductEventSample struct {
	ServiceID     string  `json:"serviceId"`
	InstanceUID   string  `json:"instanceUid"` // current owned application Pod, not a person
	EventID       string  `json:"eventId"`
	Outcome       string  `json:"outcome"`
	ReportedCount float64 `json:"reportedCount"`
}
type TelemetryObservation struct {
	Storage            *StoredTelemetrySource `json:"storage,omitempty"`
	State              string                 `json:"state"` // recorded, partial, no-data
	CollectorID        string                 `json:"collectorId"`
	ServiceUID         string                 `json:"serviceUid"`
	DeploymentUID      string                 `json:"deploymentUid"`
	CollectorPodUID    string                 `json:"collectorPodUid"`
	ApplicationPodUIDs []string               `json:"applicationPodUids"`
	PlanDigest         string                 `json:"planDigest"`
	SourceCommit       string                 `json:"sourceCommit"`
	ObservedAt         string                 `json:"observedAt"`
	Continuity         string                 `json:"continuity"` // baseline, continued, reset; never an inferred interval/lifetime total
	Samples            []ProductEventSample   `json:"samples"`
	Omitted            bool                   `json:"omitted"`
	EvidenceDigest     string                 `json:"evidenceDigest"`
}

var metricLabel = regexp.MustCompile(`^([a-zA-Z_][a-zA-Z0-9_]*)="((?:[^"\\]|\\["\\n])*)"(?:,|$)`)

func sampleKey(row ProductEventSample) string {
	return row.ServiceID + "/" + row.InstanceUID + "/" + row.EventID + "/" + row.Outcome
}
func telemetryDigest(row TelemetryObservation) string { row.EvidenceDigest = ""; return digest(row) }
func telemetryContinuity(previous *TelemetryObservation, current TelemetryObservation) string {
	if previous == nil {
		return "baseline"
	}
	if !jsonEqual(previous.Storage, current.Storage) {
		return "reset"
	}
	if previous.CollectorPodUID != current.CollectorPodUID || !jsonEqual(previous.ApplicationPodUIDs, current.ApplicationPodUIDs) || len(previous.Samples) != len(current.Samples) || previous.Omitted || current.Omitted {
		return "reset"
	}
	old := map[string]float64{}
	for _, row := range previous.Samples {
		old[sampleKey(row)] = row.ReportedCount
	}
	for _, row := range current.Samples {
		count, found := old[sampleKey(row)]
		if !found || row.ReportedCount < count {
			return "reset"
		}
	}
	return "continued"
}

// Decode only the one generated aggregate metric. Other telemetry never
// enters Luci. Unknown/extra labels or malformed matching metrics are refused,
// not silently retained as text or interpreted as identities/business evidence.
func parseProductMetrics(raw []byte, request Request, collector deploymentplan.Service, instances map[string]string) ([]ProductEventSample, bool, error) {
	if len(raw) > telemetryBodyLimit || !utf8.Valid(raw) {
		return nil, false, ErrRefused
	}
	allowedEvents := map[string]bool{}
	for _, event := range collector.Collector.Events {
		allowedEvents[event.ID] = true
	}
	allowedServices := map[string]bool{}
	for _, service := range request.Blueprint.Services {
		if service.Telemetry != nil && service.Telemetry.ResourceID == collector.ID {
			allowedServices[service.ID] = true
		}
	}
	out := []ProductEventSample{}
	seen := map[string]bool{}
	omitted := false
	matched := 0
	scanner := bufio.NewScanner(strings.NewReader(string(raw)))
	scanner.Buffer(make([]byte, 4096), 16<<10)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if !strings.HasPrefix(line, deploymentplan.ProductEventMetric) {
			continue
		}
		matched++
		if matched > telemetrySampleLimit {
			omitted = true
			break
		}
		prefix := deploymentplan.ProductEventMetric + "{"
		if !strings.HasPrefix(line, prefix) {
			return nil, false, ErrRefused
		}
		end := strings.LastIndex(line, "}")
		if end < len(prefix) {
			return nil, false, ErrRefused
		}
		labels := map[string]string{}
		remaining := line[len(prefix):end]
		for remaining != "" {
			match := metricLabel.FindStringSubmatch(remaining)
			if match == nil || len(labels) >= 12 {
				return nil, false, ErrRefused
			}
			name := match[1]
			if _, exists := labels[name]; exists {
				return nil, false, ErrRefused
			}
			value, err := strconv.Unquote("\"" + match[2] + "\"")
			if err != nil || len(value) > 256 || strings.ContainsAny(value, "\r\n\x00") {
				return nil, false, ErrRefused
			}
			labels[name] = value
			remaining = remaining[len(match[0]):]
		}
		fields := strings.Fields(line[end+1:])
		if len(fields) != 1 {
			return nil, false, ErrRefused
		}
		value, err := strconv.ParseFloat(fields[0], 64)
		if err != nil || math.IsNaN(value) || math.IsInf(value, 0) || value < 0 || value > 9007199254740991 || math.Trunc(value) != value {
			return nil, false, ErrRefused
		}
		for name := range labels {
			switch name {
			case "job", "instance", "event", "outcome", "luci_project", "luci_plan", "service_name", "service_namespace", "service_instance_id":
			default:
				return nil, false, ErrRefused
			}
		}
		service := strings.TrimPrefix(labels["job"], request.Blueprint.ProjectID+"/")
		if labels["job"] != request.Blueprint.ProjectID+"/"+service {
			return nil, false, ErrRefused
		}
		if !allowedServices[service] || !allowedEvents[labels["event"]] || labels["outcome"] != "success" && labels["outcome"] != "failure" || labels["luci_project"] != request.Blueprint.ProjectID || labels["luci_plan"] != request.Identity.PlanDigest {
			return nil, false, ErrRefused
		}
		if labels["service_name"] != "" && labels["service_name"] != service || labels["service_namespace"] != "" && labels["service_namespace"] != request.Blueprint.ProjectID || labels["service_instance_id"] != "" && labels["service_instance_id"] != labels["instance"] {
			return nil, false, ErrRefused
		}
		instance := labels["instance"]
		if instances[instance] != service {
			omitted = true
			continue
		} // expired/replaced Pod cache is not current work
		row := ProductEventSample{ServiceID: service, InstanceUID: instance, EventID: labels["event"], Outcome: labels["outcome"], ReportedCount: value}
		key := sampleKey(row)
		if seen[key] {
			return nil, false, ErrRefused
		}
		seen[key] = true
		if len(out) >= telemetrySampleLimit {
			omitted = true
			continue
		}
		out = append(out, row)
	}
	if scanner.Err() != nil {
		return nil, false, ErrRefused
	}
	sort.Slice(out, func(i, j int) bool { return sampleKey(out[i]) < sampleKey(out[j]) })
	return out, omitted, nil
}
func validateTelemetryObservation(row *TelemetryObservation, request Request, resources []ResourceReceipt) error {
	if row == nil {
		return nil
	}
	collector := deploymentplan.ManagedCollector(request.Blueprint, row.CollectorID)
	if !validStoredTelemetrySource(row.Storage, request, resources) || row.Storage != nil && (collector == nil || collector.Collector.OwnedDestination == nil || collector.Collector.OwnedDestination.MetricsServiceID != row.Storage.ServiceID) {
		return ErrRefused
	}
	if collector == nil || row.PlanDigest != request.Identity.PlanDigest || row.SourceCommit != request.Blueprint.Source.Commit || row.ServiceUID == "" || len(row.ServiceUID) > 128 || row.DeploymentUID == "" || len(row.DeploymentUID) > 128 || row.CollectorPodUID == "" || len(row.CollectorPodUID) > 128 || len(row.ApplicationPodUIDs) > 128 || len(row.Samples) > telemetrySampleLimit || row.Samples == nil || row.EvidenceDigest != telemetryDigest(*row) {
		return ErrRefused
	}
	if row.State != "recorded" && row.State != "partial" && row.State != "no-data" || row.State == "recorded" && (row.Omitted || len(row.Samples) == 0) || row.State == "partial" && !row.Omitted || row.State == "no-data" && (row.Omitted || len(row.Samples) != 0) {
		return ErrRefused
	}
	name := deploymentplan.ResourceName(request.Blueprint.ProjectID, collector.ID)
	serviceBound, deploymentBound := false, false
	for _, resource := range resources {
		if resource.Name != name || !resource.Attempted {
			continue
		}
		serviceBound = serviceBound || resource.Kind == "Service" && resource.ObservedUID == row.ServiceUID
		deploymentBound = deploymentBound || resource.Kind == "Deployment" && resource.ObservedUID == row.DeploymentUID
	}
	if !serviceBound || !deploymentBound {
		return ErrRefused
	}
	if row.Continuity != "baseline" && row.Continuity != "continued" && row.Continuity != "reset" {
		return ErrRefused
	}
	if _, err := time.Parse(time.RFC3339Nano, row.ObservedAt); err != nil {
		return ErrRefused
	}
	instances := map[string]bool{}
	for _, id := range row.ApplicationPodUIDs {
		if id == "" || len(id) > 128 || instances[id] {
			return ErrRefused
		}
		instances[id] = true
	}
	keys := map[string]bool{}
	for _, sample := range row.Samples {
		service := false
		for _, s := range request.Blueprint.Services {
			service = service || s.ID == sample.ServiceID && s.Telemetry != nil && s.Telemetry.ResourceID == collector.ID
		}
		event := false
		for _, e := range collector.Collector.Events {
			event = event || e.ID == sample.EventID
		}
		if !service || !event || !instances[sample.InstanceUID] || sample.Outcome != "success" && sample.Outcome != "failure" || sample.ReportedCount < 0 || sample.ReportedCount > 9007199254740991 || math.IsNaN(sample.ReportedCount) || math.IsInf(sample.ReportedCount, 0) || math.Trunc(sample.ReportedCount) != sample.ReportedCount || keys[sampleKey(sample)] {
			return ErrRefused
		}
		keys[sampleKey(sample)] = true
	}
	return nil
}

func TelemetrySummary(row TelemetryObservation) string {
	// An explicit human can use this attributed aggregate in project discussion;
	// it does not grant Apply or infer sales/users/revenue from reported counts.
	return fmt.Sprintf("Recorded product-event samples at %s from Collector %s, plan %s, source %s. State %s; counter continuity %s; omitted=%t. These are reported cumulative occurrences, not unique users, funnels, revenue or lifetime totals.", row.ObservedAt, row.CollectorID, row.PlanDigest, row.SourceCommit, row.State, row.Continuity, row.Omitted)
}
