package product

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"github.com/lcfrai/luci/control-plane/internal/deploymentplan"
	"github.com/lcfrai/luci/control-plane/internal/management"
	"strings"
)

type RuntimeTelemetryBlobBinding struct {
	JobID               string `json:"jobId"`
	PlanDigest          string `json:"planDigest"`
	AccountResourceID   string `json:"accountResourceId"`
	AccountCreatedAt    string `json:"accountCreatedAt"`
	ContainerResourceID string `json:"containerResourceId"`
	ServiceID           string `json:"serviceId"`
	SecretName          string `json:"secretName"`
	SecretKey           string `json:"secretKey"`
	GenerationID        string `json:"generationId"`
}

func (s *Server) runtimeBlobJob(actor management.ExecutionActorRef, id string) (CloudJob, error) {
	s.mu.RLock()
	job, ok := s.jobs[id]
	s.mu.RUnlock()
	if !ok || job.Kind != "telemetry-storage" || job.Status != "resources_ready" || job.DeleteAttempted || !job.Owned || job.TelemetryStorage == nil || job.TelemetryStorage.State != "resources_ready" || job.Plan.Foundation == nil || job.Plan.Foundation.Actor != actor || job.Plan.Foundation.TelemetryStorage == nil || !validFoundationPlan(job.Plan) {
		return CloudJob{}, ErrRuntimePreparation
	}
	return job, nil
}
func (s *Server) reviewRuntimeBlobSecret(ctx context.Context, actor management.ExecutionActorRef, row storedRuntimePreparation, jobID string) (*RuntimeTelemetryBlobBinding, *RuntimeSecretPlan, *runtimePrivateSecret, error) {
	if jobID == "" {
		return nil, nil, nil, nil
	}
	if !prototypeReleaseID.MatchString(jobID) || s.kubernetesAuthority(ctx, actor, true, true, row.Plan.MonthlyBudgetUSD) != nil {
		return nil, nil, nil, ErrRuntimePreparation
	}
	job, err := s.runtimeBlobJob(actor, jobID)
	if err != nil {
		return nil, nil, nil, err
	}
	observed, err := s.observeTelemetryStorage(ctx, job)
	if err != nil || observed.State != "resources_ready" || observed.AccountResourceID != job.TelemetryStorage.AccountResourceID || observed.AccountCreatedAt != job.TelemetryStorage.AccountCreatedAt {
		return nil, nil, nil, ErrRuntimePreparation
	}
	spec := job.Plan.Foundation.TelemetryStorage.Spec
	var service *deploymentplan.Service
	for i := range row.Template.Services {
		candidate := &row.Template.Services[i]
		v := candidate.TelemetryStorage
		if v != nil && v.Profile == deploymentplan.TempoStorageProfile && v.Azure != nil && v.Azure.AccountName == spec.AccountName && v.Azure.ContainerName == spec.ContainerName {
			if service != nil {
				return nil, nil, nil, ErrRuntimePreparation
			}
			service = candidate
		}
	}
	if service == nil {
		return nil, nil, nil, ErrRuntimePreparation
	}
	ref := service.TelemetryStorage.Azure.AccountKey
	if s.kubernetesAuthority(ctx, actor, true, true, row.Plan.MonthlyBudgetUSD) != nil {
		return nil, nil, nil, ErrRuntimePreparation
	}
	// The stdout is consumed privately, never returned, included in an error or
	// hashed into public evidence. No key generation/rotation/IAM call occurs.
	raw, err := s.cloudJobCommand(ctx, job, "az", []string{"storage", "account", "keys", "list", "--resource-group", job.Name, "--account-name", spec.AccountName, "--output", "json"}, "")
	if err != nil || len(raw) > 16<<10 || s.kubernetesAuthority(ctx, actor, true, true, row.Plan.MonthlyBudgetUSD) != nil {
		return nil, nil, nil, ErrRuntimePreparation
	}
	var keys []struct {
		Name        string `json:"keyName"`
		Permissions string `json:"permissions"`
		Value       string `json:"value"`
	}
	if json.Unmarshal([]byte(raw), &keys) != nil || len(keys) != 2 {
		return nil, nil, nil, ErrRuntimePreparation
	}
	value := ""
	for _, key := range keys {
		if key.Name == "key1" && key.Permissions == "FULL" {
			if value != "" {
				return nil, nil, nil, ErrRuntimePreparation
			}
			decoded, e := base64.StdEncoding.DecodeString(key.Value)
			if e != nil || len(decoded) != 64 || strings.ContainsAny(key.Value, "\r\n\x00") {
				return nil, nil, nil, ErrRuntimePreparation
			}
			value = key.Value
		}
	}
	if value == "" {
		return nil, nil, nil, ErrRuntimePreparation
	}
	latest, err := s.observeTelemetryStorage(ctx, job)
	if err != nil || latest.State != "resources_ready" || latest.AccountResourceID != observed.AccountResourceID || latest.AccountCreatedAt != observed.AccountCreatedAt || latest.ContainerResourceID != observed.ContainerResourceID || latest.ContainerVersion != observed.ContainerVersion {
		return nil, nil, nil, ErrRuntimePreparation
	}
	if s.kubernetesAuthority(ctx, actor, true, true, row.Plan.MonthlyBudgetUSD) != nil {
		return nil, nil, nil, ErrRuntimePreparation
	}
	generation, err := kubeOpaqueID("secret-")
	if err != nil {
		return nil, nil, nil, err
	}
	binding := &RuntimeTelemetryBlobBinding{JobID: job.ID, PlanDigest: job.Plan.Foundation.PlanDigest, AccountResourceID: observed.AccountResourceID, AccountCreatedAt: observed.AccountCreatedAt, ContainerResourceID: observed.ContainerResourceID, ServiceID: service.ID, SecretName: ref.ResourceName, SecretKey: ref.Key, GenerationID: generation}
	plan := &RuntimeSecretPlan{Name: ref.ResourceName, Kind: "telemetry-storage-key", Keys: []string{ref.Key}, GenerationID: generation, ServiceID: service.ID}
	private := &runtimePrivateSecret{Name: ref.ResourceName, Type: "Opaque", GenerationID: generation, Data: map[string][]byte{ref.Key: []byte(value)}}
	return binding, plan, private, nil
}
func validRuntimeBlobBinding(row storedRuntimePreparation) bool {
	if row.Plan.Resources == nil {
		return true
	}
	v := row.Plan.Resources.TelemetryBlob
	if v == nil {
		for _, s := range row.Plan.Resources.Secrets {
			if s.Kind == "telemetry-storage-key" {
				return false
			}
		}
		return true
	}
	if !prototypeReleaseID.MatchString(v.JobID) || !hexDigest(v.PlanDigest) || !feedbackTimestamp(v.AccountCreatedAt) || !validConnectionToken(v.AccountResourceID, 1024, false) || !validConnectionToken(v.ContainerResourceID, 1200, false) || !validConnectionToken(v.GenerationID, 80, false) {
		return false
	}
	service := runtimeTemplateService(row, v.ServiceID)
	if service == nil || service.TelemetryStorage == nil || service.TelemetryStorage.Azure == nil || service.TelemetryStorage.Azure.AccountKey.ResourceName != v.SecretName || service.TelemetryStorage.Azure.AccountKey.Key != v.SecretKey {
		return false
	}
	count := 0
	for _, s := range row.Plan.Resources.Secrets {
		if s.Kind == "telemetry-storage-key" {
			if s.Name != v.SecretName || s.ServiceID != v.ServiceID || s.GenerationID != v.GenerationID || len(s.Keys) != 1 || s.Keys[0] != v.SecretKey {
				return false
			}
			count++
		}
	}
	return count == 1
}
func (s *Server) runtimeBlobCurrent(ctx context.Context, row storedRuntimePreparation, remote bool) error {
	if row.Plan.Resources == nil || row.Plan.Resources.TelemetryBlob == nil {
		return nil
	}
	v := row.Plan.Resources.TelemetryBlob
	if !validRuntimeBlobBinding(row) {
		return ErrRuntimePreparation
	}
	job, err := s.runtimeBlobJob(row.Plan.Actor, v.JobID)
	if err != nil || job.Plan.Foundation.PlanDigest != v.PlanDigest || job.TelemetryStorage.AccountResourceID != v.AccountResourceID || job.TelemetryStorage.AccountCreatedAt != v.AccountCreatedAt || job.TelemetryStorage.ContainerResourceID != v.ContainerResourceID {
		return ErrRuntimePreparation
	}
	if remote {
		observed, e := s.observeTelemetryStorage(ctx, job)
		if e != nil || observed.State != "resources_ready" || observed.AccountResourceID != v.AccountResourceID || observed.AccountCreatedAt != v.AccountCreatedAt || observed.ContainerResourceID != v.ContainerResourceID {
			return ErrRuntimePreparation
		}
	}
	return s.kubernetesAuthority(ctx, row.Plan.Actor, false, true, 0)
}
