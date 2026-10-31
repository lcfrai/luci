package deploymentkube

import (
	"context"
	"encoding/json"
	"github.com/lcfrai/luci/control-plane/internal/deploymentplan"
	"io"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"math"
	"net/http"
	"net/url"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"time"
)

type StoredTelemetrySource struct {
	ServiceID     string `json:"serviceId"`
	Profile       string `json:"profile"`
	DeploymentUID string `json:"deploymentUid"`
	ServiceUID    string `json:"serviceUid"`
	PodUID        string `json:"podUid"`
	ConfigUID     string `json:"configUid"`
	ConfigVersion string `json:"configVersion"`
}
type storageQuerySession struct {
	client      *http.Client
	base        *url.URL
	source      StoredTelemetrySource
	fingerprint string
	close       func() error
}

func (c *Connector) storageQuery(ctx context.Context, operation Operation, target Target, resources []desiredResource, serviceID string, budget *int) (storageQuerySession, error) {
	var service *deploymentplan.Service
	for i := range operation.Request.Blueprint.Services {
		row := &operation.Request.Blueprint.Services[i]
		if row.ID == serviceID && row.TelemetryStorage != nil {
			service = row
		}
	}
	if service == nil || c.authorize(ctx, operation.Request, false) != nil {
		return storageQuerySession{}, ErrRefused
	}
	before, err := c.telemetryComponent(ctx, target, operation, resources, *service, budget)
	if err != nil || before.omitted || len(before.pods) != 1 {
		return storageQuerySession{}, ErrRefused
	}
	var config *desiredResource
	var receipt *ResourceReceipt
	for i := range resources {
		if resources[i].configMap != nil && resources[i].configMap.Labels["luci.ai/component"] == service.ID {
			config = &resources[i]
			receipt = &operation.Resources[i]
		}
	}
	if config == nil || receipt == nil || !receipt.Attempted || receipt.ObservedUID == "" {
		return storageQuerySession{}, ErrRefused
	}
	actual, err := target.Client.CoreV1().ConfigMaps(target.Namespace).Get(ctx, config.name, metav1.GetOptions{})
	if err != nil || actual == nil || string(actual.UID) != receipt.ObservedUID || !objectOwned(actual, operation.Request, *config, true) || !reflect.DeepEqual(actual.Data, config.configMap.Data) || len(actual.BinaryData) != 0 || c.authorize(ctx, operation.Request, false) != nil {
		return storageQuerySession{}, ErrRefused
	}
	ref := service.TelemetryStorage.QueryTLS
	var binding *ReferenceSnapshot
	for i := range operation.Request.References {
		r := &operation.Request.References[i]
		if r.Kind == "secret" && r.Name == ref.ResourceName {
			binding = r
		}
	}
	if binding == nil || c.authorize(ctx, operation.Request, false) != nil {
		return storageQuerySession{}, ErrRefused
	}
	secret, err := target.Client.CoreV1().Secrets(target.Namespace).Get(ctx, ref.ResourceName, metav1.GetOptions{})
	if err != nil || secret == nil || secret.DeletionTimestamp != nil || string(secret.UID) != binding.UID || secret.ResourceVersion != binding.ResourceVersion || c.authorize(ctx, operation.Request, false) != nil {
		return storageQuerySession{}, ErrRefused
	}
	server := deploymentplan.ResourceName(operation.Request.Blueprint.ProjectID, service.ID) + "." + target.Namespace + ".svc"
	client, closeTunnel, err := storageHTTPClient(ctx, target, before.pods[0].Name, server, service.Port, secret.Data[ref.CAKey], secret.Data[ref.CertKey], secret.Data[ref.KeyKey])
	if err != nil {
		return storageQuerySession{}, ErrRefused
	}
	base, _ := url.Parse("https://" + server + ":" + strconv.Itoa(service.Port))
	source := StoredTelemetrySource{ServiceID: service.ID, Profile: service.TelemetryStorage.Profile, DeploymentUID: string(before.deployment.UID), ServiceUID: string(before.service.UID), PodUID: string(before.pods[0].UID), ConfigUID: string(actual.UID), ConfigVersion: actual.ResourceVersion}
	return storageQuerySession{client: client, base: base, source: source, fingerprint: digest([]string{before.fingerprint, string(actual.UID), actual.ResourceVersion, binding.MetadataDigest}), close: closeTunnel}, nil
}
func (c *Connector) storageQueryStillCurrent(ctx context.Context, operation Operation, target Target, resources []desiredResource, source storageQuerySession, budget *int) error {
	// Do not open a second tunnel: only repeat original owned specs/UIDs and
	// current private references after the already bounded read and close.
	var service deploymentplan.Service
	for _, s := range operation.Request.Blueprint.Services {
		if s.ID == source.source.ServiceID {
			service = s
		}
	}
	component, err := c.telemetryComponent(ctx, target, operation, resources, service, budget)
	if err != nil || component.omitted || len(component.pods) != 1 || string(component.pods[0].UID) != source.source.PodUID {
		return ErrRefused
	}
	var desired *desiredResource
	for i := range resources {
		r := &resources[i]
		if r.configMap != nil && r.configMap.Labels["luci.ai/component"] == service.ID {
			desired = r
		}
	}
	if desired == nil {
		return ErrRefused
	}
	actual, err := target.Client.CoreV1().ConfigMaps(target.Namespace).Get(ctx, desired.name, metav1.GetOptions{})
	if err != nil || actual == nil || string(actual.UID) != source.source.ConfigUID || actual.ResourceVersion != source.source.ConfigVersion || !objectOwned(actual, operation.Request, *desired, true) || !reflect.DeepEqual(actual.Data, desired.configMap.Data) || len(actual.BinaryData) != 0 {
		return ErrRefused
	}
	var ref ReferenceSnapshot
	for _, r := range operation.Request.References {
		if r.Kind == "secret" && r.Name == service.TelemetryStorage.QueryTLS.ResourceName {
			ref = r
		}
	}
	if digest([]string{component.fingerprint, string(actual.UID), actual.ResourceVersion, ref.MetadataDigest}) != source.fingerprint || c.validateReferences(ctx, target, operation.Request, nil, false) != nil || c.authorize(ctx, operation.Request, false) != nil {
		return ErrRefused
	}
	return nil
}
func storageGet(ctx context.Context, session storageQuerySession, path string, params url.Values, value any) error {
	requestURL := *session.base
	requestURL.Path = path
	requestURL.RawQuery = params.Encode()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL.String(), nil)
	if err != nil {
		return ErrRefused
	}
	request.Header.Set("Accept", "application/json")
	response, err := session.client.Do(request)
	if err != nil {
		return ErrRefused
	}
	raw, e := io.ReadAll(io.LimitReader(response.Body, telemetryBodyLimit+1))
	closed := response.Body.Close()
	if e != nil || closed != nil || len(raw) > telemetryBodyLimit || response.StatusCode != 200 || ctx.Err() != nil {
		return ErrRefused
	}
	decoder := json.NewDecoder(strings.NewReader(string(raw)))
	if decoder.Decode(value) != nil || decoder.Decode(new(any)) != io.EOF {
		return ErrRefused
	}
	return nil
}
func parseStoredProductVector(value prometheusVector, request Request, collector deploymentplan.Service, instances map[string]string) ([]ProductEventSample, bool, error) {
	if value.Status != "success" || value.Data.Type != "vector" || len(value.Data.Results) > telemetrySampleLimit+1 || len(value.Warnings) > 4 {
		return nil, false, ErrRefused
	}
	for _, warning := range value.Warnings {
		if len(warning) > 512 {
			return nil, false, ErrRefused
		}
	}
	allowed := map[string]bool{}
	for _, e := range collector.Collector.Events {
		allowed[e.ID] = true
	}
	rows := []ProductEventSample{}
	seen := map[string]bool{}
	omitted := len(value.Warnings) > 0 || len(value.Data.Results) > telemetrySampleLimit
	if len(value.Data.Results) > telemetrySampleLimit {
		value.Data.Results = value.Data.Results[:telemetrySampleLimit]
	}
	for _, r := range value.Data.Results {
		if len(r.Metric) > 12 || len(r.Value) != 2 || r.Metric["__name__"] != deploymentplan.ProductEventMetric || r.Metric["service_namespace"] != request.Identity.ProjectID {
			return nil, false, ErrRefused
		}
		service, instance, event, outcome := r.Metric["service_name"], r.Metric["service_instance_id"], r.Metric["event"], r.Metric["outcome"]
		if !allowed[event] || (outcome != "success" && outcome != "failure") {
			return nil, false, ErrRefused
		}
		if service == "" || instance == "" || len(service) > 63 || len(instance) > 128 {
			return nil, false, ErrRefused
		}
		original, exists := instances[instance]
		if !exists || original != service {
			omitted = true
			continue
		} // retained old Pods are not current/lifetime user outcomes
		var number string
		if json.Unmarshal(r.Value[1], &number) != nil {
			return nil, false, ErrRefused
		}
		count, e := strconv.ParseFloat(number, 64)
		timestamp, e2 := strconv.ParseFloat(string(r.Value[0]), 64)
		if e != nil || e2 != nil || math.IsNaN(count) || math.IsInf(count, 0) || count < 0 || count > 9007199254740991 || math.Trunc(count) != count || math.IsNaN(timestamp) || math.IsInf(timestamp, 0) || timestamp < 946684800 || timestamp > float64(time.Now().Add(time.Minute).Unix()) {
			return nil, false, ErrRefused
		}
		sample := ProductEventSample{ServiceID: service, InstanceUID: instance, EventID: event, Outcome: outcome, ReportedCount: count}
		if seen[sampleKey(sample)] {
			return nil, false, ErrRefused
		}
		seen[sampleKey(sample)] = true
		rows = append(rows, sample)
	}
	sort.Slice(rows, func(i, j int) bool { return sampleKey(rows[i]) < sampleKey(rows[j]) })
	return rows, omitted, nil
}

type prometheusVector struct {
	Status   string   `json:"status"`
	Warnings []string `json:"warnings"`
	Data     struct {
		Type    string `json:"resultType"`
		Results []struct {
			Metric map[string]string `json:"metric"`
			Value  []json.RawMessage `json:"value"`
		} `json:"result"`
	} `json:"data"`
}

func (c *Connector) ReadStoredTelemetry(ctx context.Context, identity Identity) (Operation, error) {
	ctx, cancel := context.WithTimeout(ctx, 90*time.Second)
	defer cancel()
	operation, target, resources, err := c.operationsInput(ctx, identity)
	if err != nil {
		return Operation{}, ErrRefused
	}
	request := operation.Request
	if c.validateReferences(ctx, target, request, nil, false) != nil {
		return Operation{}, ErrRefused
	}
	budget := 0
	before, err := c.telemetryRuntime(ctx, target, operation, resources, &budget)
	if err != nil || before.collector.Collector.OwnedDestination == nil {
		return Operation{}, ErrRefused
	}
	session, err := c.storageQuery(ctx, operation, target, resources, before.collector.Collector.OwnedDestination.MetricsServiceID, &budget)
	if err != nil {
		return Operation{}, ErrRefused
	}
	query := deploymentplan.ProductEventMetric + "{service_namespace=" + strconv.Quote(identity.ProjectID) + "}"
	var vector prometheusVector
	readErr := storageGet(ctx, session, "/api/v1/query", url.Values{"query": {query}, "timeout": {"5s"}, "limit": {"257"}}, &vector)
	closeErr := session.close()
	if readErr != nil || closeErr != nil || c.authorize(ctx, request, false) != nil {
		return Operation{}, ErrRefused
	}
	samples, omitted, err := parseStoredProductVector(vector, request, before.collector, before.instances)
	if err != nil {
		return Operation{}, ErrRefused
	}
	latest, err := c.target(ctx, request)
	if err != nil || latest.NamespaceUID != operation.NamespaceUID || c.storageQueryStillCurrent(ctx, operation, latest, resources, session, &budget) != nil {
		return Operation{}, ErrRefused
	}
	after, err := c.telemetryRuntime(ctx, latest, operation, resources, &budget)
	if err != nil || after.fingerprint != before.fingerprint {
		return Operation{}, ErrRefused
	}
	observation := TelemetryObservation{Storage: &session.source, State: "recorded", CollectorID: before.collector.ID, ServiceUID: string(before.service.UID), DeploymentUID: string(before.deployment.UID), CollectorPodUID: string(before.pod.UID), ApplicationPodUIDs: before.instanceUIDs, PlanDigest: identity.PlanDigest, SourceCommit: request.Blueprint.Source.Commit, ObservedAt: time.Now().UTC().Format(time.RFC3339Nano), Samples: samples, Omitted: omitted || before.omitted}
	if observation.Omitted {
		observation.State = "partial"
	} else if len(samples) == 0 {
		observation.State = "no-data"
	}
	observation.Continuity = telemetryContinuity(operation.Telemetry, observation)
	observation.EvidenceDigest = telemetryDigest(observation)
	if validateTelemetryObservation(&observation, request, operation.Resources) != nil || c.authorize(ctx, request, false) != nil {
		return Operation{}, ErrRefused
	}
	operation.Telemetry = &observation
	if c.save(ctx, &operation, operation.State) != nil {
		return Operation{}, ErrRefused
	}
	latest, err = c.target(ctx, request)
	if err != nil || latest.NamespaceUID != operation.NamespaceUID || c.validateReferences(ctx, latest, request, nil, false) != nil || c.authorize(ctx, request, false) != nil {
		return Operation{}, ErrRefused
	}
	return operation, nil
}

func validStoredTelemetrySource(v *StoredTelemetrySource, request Request, resources []ResourceReceipt) bool {
	if v == nil {
		return true
	}
	if v.Profile != deploymentplan.PrometheusStorageProfile || v.PodUID == "" || len(v.PodUID) > 128 || v.ConfigUID == "" || len(v.ConfigUID) > 128 || v.ConfigVersion == "" || len(v.ConfigVersion) > 128 {
		return false
	}
	var service *deploymentplan.Service
	for i := range request.Blueprint.Services {
		s := &request.Blueprint.Services[i]
		if s.ID == v.ServiceID && s.TelemetryStorage != nil && s.TelemetryStorage.Profile == v.Profile {
			service = s
		}
	}
	if service == nil {
		return false
	}
	name := deploymentplan.ResourceName(request.Identity.ProjectID, service.ID)
	ds, svc, config := false, false, false
	for _, r := range resources {
		if !r.Attempted {
			continue
		}
		ds = ds || r.Kind == "Deployment" && r.Name == name && r.ObservedUID == v.DeploymentUID
		svc = svc || r.Kind == "Service" && r.Name == name && r.ObservedUID == v.ServiceUID
		config = config || r.Kind == "ConfigMap" && r.Name == name+"-config" && r.ObservedUID == v.ConfigUID
	}
	return ds && svc && config
}

// The existing Tempo parser and exact three-attribute TraceQL query are reused;
// only its private transport changes to the original owned store's mTLS tunnel.
func (c *Connector) ReadOwnedOperationalTraces(ctx context.Context, identity Identity, query TraceQuery) (OperationalTraces, error) {
	ctx, cancel := context.WithTimeout(ctx, 90*time.Second)
	defer cancel()
	if !query.OwnedBackend {
		return OperationalTraces{}, ErrRefused
	}
	operation, target, resources, err := c.operationsInput(ctx, identity)
	if err != nil || c.validateReferences(ctx, target, operation.Request, nil, false) != nil {
		return OperationalTraces{}, ErrRefused
	}
	serviceID := ""
	for _, s := range operation.Request.Blueprint.Services {
		if s.Collector != nil && s.Collector.OwnedDestination != nil {
			serviceID = s.Collector.OwnedDestination.TracesServiceID
		}
	}
	if serviceID == "" {
		return OperationalTraces{}, ErrRefused
	}
	budget := 0
	session, err := c.storageQuery(ctx, operation, target, resources, serviceID, &budget)
	if err != nil {
		return OperationalTraces{}, ErrRefused
	}
	reader := &tempoHTTPReader{base: session.base, client: session.client, authorize: func(call context.Context) error { return c.authorize(call, operation.Request, false) }}
	result, readErr := c.ReadOperationalTraces(ctx, identity, query, reader)
	closeErr := session.close()
	if readErr != nil || closeErr != nil {
		return OperationalTraces{}, ErrRefused
	}
	latest, err := c.target(ctx, operation.Request)
	if err != nil || latest.NamespaceUID != operation.NamespaceUID || c.storageQueryStillCurrent(ctx, operation, latest, resources, session, &budget) != nil {
		return OperationalTraces{}, ErrRefused
	}
	result.Storage = &session.source
	result.EvidenceDigest = ""
	result.EvidenceDigest = digest(result)
	return result, nil
}
