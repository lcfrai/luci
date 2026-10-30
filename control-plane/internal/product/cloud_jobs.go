package product

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/management"
	"github.com/lcfrai/luci/control-plane/internal/project"
)

// This typed refusal is local admission, not provider authentication failure.
type foundationAdmissionRefusal struct{ cause error }

func (e *foundationAdmissionRefusal) Error() string {
	return "The current foundation plan or deployment access changed."
}
func (e *foundationAdmissionRefusal) Unwrap() error { return e.cause }

type CloudJob struct {
	ID         string    `json:"id"`
	PlanID     string    `json:"planId"`
	Provider   string    `json:"provider"`
	Name       string    `json:"name"`
	Region     string    `json:"region"`
	Domain     string    `json:"domain"`
	Status     string    `json:"status"`
	Message    string    `json:"message"`
	PublicIP   string    `json:"publicIp"`
	ResourceID string    `json:"resourceId"`
	CreatedAt  time.Time `json:"createdAt"`
	UpdatedAt  time.Time `json:"updatedAt"`
	Plan       CloudPlan `json:"-"`
	Owned      bool      `json:"-"`
}
type savedCloudJob struct {
	Job   CloudJob  `json:"job"`
	Plan  CloudPlan `json:"plan"`
	Owned bool      `json:"owned"`
}

func (s *Server) initCloudJobs() error {
	s.jobs = map[string]CloudJob{}
	data, err := os.ReadFile(filepath.Join(s.stateDir, "cloud-jobs.json"))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if len(data) > 16<<20 {
		return errors.New("cloud job history exceeds its limit")
	}
	var records []savedCloudJob
	if err = json.Unmarshal(data, &records); err != nil {
		return err
	}
	for _, record := range records {
		j := record.Job
		j.Plan = record.Plan
		if j.Plan.Foundation != nil && (!validFoundationPlan(j.Plan) || j.PlanID != j.Plan.ID || j.Provider != "azure") {
			return errAzureBlueprint
		}
		j.Owned = record.Owned
		if j.Status == "creating" || j.Status == "deleting" {
			j.Status = "unknown"
			j.Message = "Luci restarted during this operation. Refresh provider status before retrying."
		}
		s.jobs[j.ID] = j
	}
	return nil
}
func (s *Server) saveJobsLocked() error {
	records := []savedCloudJob{}
	for _, j := range s.jobs {
		records = append(records, savedCloudJob{j, j.Plan, j.Owned})
	}
	return atomicJSON(filepath.Join(s.stateDir, "cloud-jobs.json"), records)
}
func (s *Server) registerCloudJobs(mux Registrar) {
	mux.HandleFunc("GET /v1/setup/cloud/jobs", s.listCloudJobs)
	mux.HandleFunc("POST /v1/setup/cloud/create", s.createCloudJob)
	mux.HandleFunc("POST /v1/setup/cloud/jobs/{jobId}/refresh", s.refreshCloudJob)
	mux.HandleFunc("POST /v1/setup/cloud/jobs/{jobId}/delete", s.deleteCloudJob)
	mux.HandleFunc("POST /v1/setup/cloud/jobs/{jobId}/dns", s.checkCloudDNS)
}
func (s *Server) listCloudJobs(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	jobs := []CloudJob{}
	for _, j := range s.jobs {
		jobs = append(jobs, j)
	}
	s.mu.RUnlock()
	visible := []CloudJob{}
	for _, job := range jobs {
		if s.foundationJobReadable(r.Context(), job) {
			visible = append(visible, job)
		}
	}
	for _, job := range visible {
		if !s.foundationJobReadable(r.Context(), job) {
			foundationProblem(w, errAzureBlueprint)
			return
		}
	}
	writeJSON(w, 200, map[string]any{"jobs": visible})
}
func (s *Server) createCloudJob(w http.ResponseWriter, r *http.Request) {
	var req struct {
		PlanID                     string `json:"planId"`
		AcknowledgeCosts           bool   `json:"acknowledgeCosts"`
		AcknowledgeUnverifiedPrice bool   `json:"acknowledgeUnverifiedPrice"`
	}
	if !readJSON(w, r, &req) {
		return
	}
	if !req.AcknowledgeCosts {
		problem(w, 400, "cost_acknowledgement_required", "Accept the reviewed cloud resources and ongoing billing before creating them.")
		return
	}
	s.mu.RLock()
	inspected, found := s.plans[req.PlanID]
	existing := false
	for _, job := range s.jobs {
		if job.PlanID == req.PlanID {
			existing = true
		}
	}
	s.mu.RUnlock()
	var foundationProject project.Project
	if found && inspected.Foundation != nil {
		var err error
		foundationProject, err = s.authorizeFoundationPlan(r.Context(), inspected, !existing, true)
		if err != nil {
			foundationProblem(w, err)
			return
		}
	}
	s.mu.Lock()
	plan, ok := s.plans[req.PlanID]
	if !ok {
		s.mu.Unlock()
		problem(w, 404, "plan_not_found", "Generate a fresh cloud plan first.")
		return
	}
	if _, policy, managed := management.ActorFromContext(r.Context()); managed && (!policy.AllowCloudProvisioning || plan.MonthlyBudgetUSD > policy.MonthlyBudgetUSD || (plan.Estimate.Monthly != nil && *plan.Estimate.Monthly > policy.MonthlyBudgetUSD)) {
		s.mu.Unlock()
		problem(w, 403, "budget_policy", "This saved plan exceeds the current control-plane provisioning or budget policy. Generate a new plan after reviewing the policy.")
		return
	}
	for _, j := range s.jobs {
		if j.PlanID == plan.ID {
			s.mu.Unlock()
			s.writeFoundationJobResponse(w, r, j, 200, j)
			return
		}
	}
	if plan.Foundation != nil {
		if !validFoundationPlan(plan) || inspected.Foundation == nil || plan.Foundation.PlanDigest != inspected.Foundation.PlanDigest {
			s.mu.Unlock()
			foundationProblem(w, errAzureBlueprint)
			return
		}
		if err := s.recheckFoundationContextLocked(foundationProject, plan); err != nil {
			s.mu.Unlock()
			foundationProblem(w, err)
			return
		}
	}
	if time.Since(plan.CreatedAt) > 30*time.Minute {
		s.mu.Unlock()
		problem(w, 409, "plan_expired", "Generate a fresh plan and quote before creating resources.")
		return
	}
	if !plan.Estimate.Verified && !req.AcknowledgeUnverifiedPrice {
		s.mu.Unlock()
		problem(w, 400, "quote_required", "This quote is incomplete. Acknowledge that uncertainty separately or obtain a provider quote.")
		return
	}
	if plan.Estimate.Monthly != nil && *plan.Estimate.Monthly > plan.MonthlyBudgetUSD {
		s.mu.Unlock()
		problem(w, 409, "budget_exceeded", "This machine exceeds the reviewed budget. Generate a new plan with a smaller machine or an explicitly increased budget.")
		return
	}
	job := CloudJob{ID: newID("cj-"), PlanID: plan.ID, Provider: plan.Provider, Name: plan.Name, Region: plan.Region, Domain: plan.Domain, Plan: plan, Status: "creating", Message: "Validating your account and checking that this name is unused.", CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()}
	s.jobs[job.ID] = job
	if err := s.saveJobsLocked(); err != nil {
		delete(s.jobs, job.ID)
		s.mu.Unlock()
		problem(w, 500, "save_failed", "Could not save the cloud operation; nothing was created.")
		return
	}
	s.mu.Unlock()
	go s.provisionCloud(job)
	s.writeFoundationJobResponse(w, r, job, 202, job)
}
func (s *Server) updateJob(j CloudJob, status, message string) {
	s.mu.Lock()
	j.Status = status
	j.Message = message
	j.UpdatedAt = time.Now().UTC()
	s.jobs[j.ID] = j
	_ = s.saveJobsLocked()
	s.mu.Unlock()
}
func (s *Server) provisionCloud(j CloudJob) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	var admissionRefusal *foundationAdmissionRefusal
	defer func() {
		if admissionRefusal != nil {
			// Preserve the latest retained identity/ownership and uncertain costs.
			// Earlier commands may have succeeded; there is no rollback or absence claim.
			saved := s.jobsSnapshot(j.ID)
			if saved.ID != "" {
				s.updateJob(saved, "unknown", "The current foundation plan or deployment access changed. No further provider command was issued after that refusal. Earlier operations or resources may exist; inspect the retained job before any new attempt.")
			}
		}
	}()
	dir := filepath.Join(s.stateDir, "cloud", j.ID)
	if err := os.MkdirAll(dir, 0700); err != nil {
		s.updateJob(j, "failed", "Could not prepare the cloud template.")
		return
	}
	for _, f := range j.Plan.Files {
		if filepath.Base(f.Name) != f.Name {
			s.updateJob(j, "failed", "The template has an invalid filename.")
			return
		}
		if err := os.WriteFile(filepath.Join(dir, f.Name), []byte(f.Content), 0600); err != nil {
			s.updateJob(j, "failed", "Could not save the cloud template.")
			return
		}
	}
	run := func(exe string, args ...string) (string, error) {
		if err := s.admitFoundationProvider(ctx, j); err != nil {
			admissionRefusal = &foundationAdmissionRefusal{cause: err}
			return "", admissionRefusal
		}
		return s.runner(ctx, exe, args, dir)
	}
	if j.Provider == "aws" {
		if _, err := run("aws", "sts", "get-caller-identity", "--output", "json"); err != nil {
			s.updateJob(j, "failed", "AWS is not signed in. Connect your account in setup.")
			return
		}
		// describe error is not proof a stack does not exist. Listing succeeds or creation is refused.
		raw, err := run("aws", "cloudformation", "list-stacks", "--region", j.Region, "--output", "json")
		if err != nil {
			s.updateJob(j, "failed", "Could not check existing AWS stacks; creation was refused.")
			return
		}
		var existing struct {
			Stacks []struct {
				Name   string `json:"StackName"`
				Status string `json:"StackStatus"`
			} `json:"StackSummaries"`
		}
		if json.Unmarshal([]byte(raw), &existing) != nil {
			s.updateJob(j, "failed", "AWS returned an unreadable stack list.")
			return
		}
		for _, e := range existing.Stacks {
			if e.Name == j.Name && e.Status != "DELETE_COMPLETE" {
				s.updateJob(j, "failed", "An AWS stack already uses this name. Choose a new name.")
				return
			}
		}
		if _, err = run("aws", "cloudformation", "validate-template", "--template-body", "file://template.json", "--region", j.Region); err != nil {
			s.updateJob(j, "failed", "AWS rejected the template. Review it before creation.")
			return
		}
		raw, err = run("aws", "cloudformation", "create-stack", "--stack-name", j.Name, "--template-body", "file://template.json", "--region", j.Region, "--tags", "Key=managed-by,Value=luci", "Key=luci-job,Value="+j.ID, "--client-request-token", j.ID, "--output", "json")
		if err != nil {
			s.updateJob(j, "unknown", "AWS creation returned an error. Refresh status before retrying; resources may exist.")
			return
		}
		var result struct {
			StackID string `json:"StackId"`
		}
		if json.Unmarshal([]byte(raw), &result) != nil || result.StackID == "" {
			s.updateJob(j, "unknown", "AWS did not return a stack identity. Refresh before retrying.")
			return
		}
		j.ResourceID = result.StackID
		j.Owned = true
	} else {
		if _, err := run("az", "account", "show", "--output", "json"); err != nil {
			s.updateJob(j, "failed", "Azure is not signed in. Use the account connection in setup.")
			return
		}
		raw, err := run("az", "group", "exists", "--name", j.Name)
		if err != nil || strings.TrimSpace(raw) != "false" {
			s.updateJob(j, "failed", "This resource group exists or could not be checked. Choose a new group name.")
			return
		}
		raw, err = run("az", "group", "create", "--name", j.Name, "--location", j.Region, "--tags", "managed-by=luci", "luci-job="+j.ID, "--output", "json")
		if err != nil {
			s.updateJob(j, "unknown", "Azure group creation returned an error. Refresh status before retrying.")
			return
		}
		var group struct {
			ID string `json:"id"`
		}
		if json.Unmarshal([]byte(raw), &group) != nil || group.ID == "" {
			s.updateJob(j, "unknown", "Azure did not return a resource identity. Refresh before retrying.")
			return
		}
		j.ResourceID = group.ID
		j.Owned = true
		s.updateJob(j, "creating", "Resource group created. Validating the machine template.")
		if _, err = run("az", "deployment", "group", "validate", "--resource-group", j.Name, "--template-file", "template.json"); err != nil {
			s.updateJob(j, "failed", "Azure rejected the template. The empty resource group remains; remove it with Delete resources.")
			return
		}
		if _, err = run("az", "deployment", "group", "create", "--resource-group", j.Name, "--name", j.Name, "--template-file", "template.json", "--no-wait"); err != nil {
			s.updateJob(j, "unknown", "Azure creation returned an error. Refresh before retrying; the group still exists.")
			return
		}
	}
	s.updateJob(j, "provisioning", "Resources are being created. Refresh progress, then connect the server through SSH.")
}
func (s *Server) jobForRequest(w http.ResponseWriter, r *http.Request) (CloudJob, bool) {
	s.mu.RLock()
	j, ok := s.jobs[r.PathValue("jobId")]
	s.mu.RUnlock()
	if ok && !s.foundationJobReadable(r.Context(), j) {
		foundationProblem(w, errAzureBlueprint)
		return CloudJob{}, false
	}
	if !ok {
		problem(w, 404, "job_not_found", "Cloud operation not found.")
	}
	return j, ok
}
func (s *Server) refreshCloudJob(w http.ResponseWriter, r *http.Request) {
	j, ok := s.jobForRequest(w, r)
	if !ok {
		return
	}
	if j.Status == "creating" {
		s.writeFoundationJobResponse(w, r, j, 200, j)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	exe := "aws"
	stackName := j.Name
	if j.ResourceID != "" {
		stackName = j.ResourceID
	}
	args := []string{"cloudformation", "describe-stacks", "--stack-name", stackName, "--region", j.Region, "--output", "json"}
	if j.Provider == "azure" {
		exe = "az"
		exists, checkErr := s.runner(ctx, "az", []string{"group", "exists", "--name", j.Name}, "")
		if checkErr != nil {
			problem(w, 502, "provider_unavailable", "Could not check resource-group progress.")
			return
		}
		if strings.TrimSpace(exists) == "false" {
			if j.Status == "deleting" || j.Status == "deleted" {
				s.updateJob(j, "deleted", "Azure confirmed the resource group has been removed.")
				s.writeFoundationJobResponse(w, r, j, 200, s.jobsSnapshot(j.ID))
				return
			}
			problem(w, 409, "resource_not_found", "The resource group is not present. Nothing was created or adopted.")
			return
		}
		groupRaw, groupErr := s.runner(ctx, "az", []string{"group", "show", "--name", j.Name, "--output", "json"}, "")
		var group struct {
			ID   string            `json:"id"`
			Tags map[string]string `json:"tags"`
		}
		if groupErr != nil || json.Unmarshal([]byte(groupRaw), &group) != nil || group.Tags["luci-job"] != j.ID || (j.ResourceID != "" && group.ID != j.ResourceID) {
			problem(w, 409, "ownership_required", "This resource group does not match Luci's operation. It was not adopted.")
			return
		}
		j.ResourceID = group.ID
		j.Owned = true
		args = []string{"deployment", "group", "show", "--resource-group", j.Name, "--name", j.Name, "--output", "json"}
	}
	raw, err := s.runner(ctx, exe, args, "")
	if err != nil {
		problem(w, 502, "provider_unavailable", "Could not read cloud progress. Your existing resources were not changed.")
		return
	}
	var status string
	if j.Provider == "aws" {
		var data struct {
			Stacks []struct {
				ID     string `json:"StackId"`
				Status string `json:"StackStatus"`
				Tags   []struct {
					Key   string
					Value string
				}
				Outputs []struct {
					Key   string `json:"OutputKey"`
					Value string `json:"OutputValue"`
				}
			} `json:"Stacks"`
		}
		if json.Unmarshal([]byte(raw), &data) != nil || len(data.Stacks) != 1 {
			problem(w, 502, "provider_unavailable", "AWS returned an unreadable result.")
			return
		}
		v := data.Stacks[0]
		status = v.Status
		owned := false
		for _, tag := range v.Tags {
			if tag.Key == "luci-job" && tag.Value == j.ID {
				owned = true
				j.Owned = true
				j.ResourceID = v.ID
			}
		}
		if !owned {
			problem(w, 409, "ownership_required", "The returned stack does not match Luci's operation. It was not adopted.")
			return
		}
		for _, o := range v.Outputs {
			if o.Key == "PublicIp" && net.ParseIP(o.Value) != nil {
				j.PublicIP = o.Value
			}
		}
		if status == "CREATE_COMPLETE" {
			j.Status = "resources_ready"
		} else if strings.Contains(status, "FAILED") || strings.Contains(status, "ROLLBACK") {
			j.Status = "failed"
		} else if status == "DELETE_COMPLETE" {
			j.Status = "deleted"
		} else {
			j.Status = "provisioning"
		}
	} else {
		var data struct {
			Properties struct {
				State   string `json:"provisioningState"`
				Outputs map[string]struct {
					Value string `json:"value"`
				} `json:"outputs"`
			} `json:"properties"`
		}
		if json.Unmarshal([]byte(raw), &data) != nil {
			problem(w, 502, "provider_unavailable", "Azure returned an unreadable result.")
			return
		}
		status = data.Properties.State
		if ip := data.Properties.Outputs["publicIp"].Value; net.ParseIP(ip) != nil {
			j.PublicIP = ip
		}
		switch status {
		case "Succeeded":
			j.Status = "resources_ready"
		case "Failed", "Canceled":
			j.Status = "failed"
		default:
			j.Status = "provisioning"
		}
	}
	s.updateJob(j, j.Status, "Provider status: "+status+". Server installation and application health are checked separately.")
	s.mu.RLock()
	j = s.jobs[j.ID]
	s.mu.RUnlock()
	s.writeFoundationJobResponse(w, r, j, 200, j)
}
func (s *Server) deleteCloudJob(w http.ResponseWriter, r *http.Request) {
	j, ok := s.jobForRequest(w, r)
	if !ok {
		return
	}
	var req struct {
		ConfirmName string `json:"confirmName"`
	}
	if !readJSON(w, r, &req) {
		return
	}
	if req.ConfirmName != j.Name || !j.Owned {
		problem(w, 409, "ownership_required", "Type the resource name and refresh its ownership before deleting it.")
		return
	}
	if j.Status == "creating" || j.Status == "deleting" {
		problem(w, 409, "operation_running", "Wait for the current operation before deleting resources.")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	if j.Provider == "azure" {
		raw, err := s.runner(ctx, "az", []string{"group", "show", "--name", j.Name, "--output", "json"}, "")
		var group struct {
			ID   string            `json:"id"`
			Tags map[string]string `json:"tags"`
		}
		if err != nil || json.Unmarshal([]byte(raw), &group) != nil || group.ID != j.ResourceID || group.Tags["luci-job"] != j.ID {
			problem(w, 409, "ownership_required", "The resource group no longer matches Luci's ownership record.")
			return
		}
	} else {
		raw, err := s.runner(ctx, "aws", []string{"cloudformation", "describe-stacks", "--stack-name", j.ResourceID, "--region", j.Region, "--output", "json"}, "")
		var data struct {
			Stacks []struct {
				ID   string `json:"StackId"`
				Tags []struct {
					Key   string
					Value string
				}
			} `json:"Stacks"`
		}
		verified := false
		if err == nil && json.Unmarshal([]byte(raw), &data) == nil {
			for _, v := range data.Stacks {
				for _, tag := range v.Tags {
					if v.ID == j.ResourceID && tag.Key == "luci-job" && tag.Value == j.ID {
						verified = true
					}
				}
			}
		}
		if !verified {
			problem(w, 409, "ownership_required", "The stack no longer matches Luci's ownership record.")
			return
		}
	}
	exe := "aws"
	args := []string{"cloudformation", "delete-stack", "--stack-name", j.ResourceID, "--region", j.Region}
	if j.Provider == "azure" {
		exe = "az"
		args = []string{"group", "delete", "--name", j.Name, "--yes", "--no-wait"}
	}
	if j.Plan.Foundation != nil {
		if _, err := s.authorizeFoundationPlan(ctx, j.Plan, false, true); err != nil {
			foundationProblem(w, err)
			return
		}
	}
	if _, err := s.runner(ctx, exe, args, ""); err != nil {
		problem(w, 502, "delete_failed", "Deletion was not confirmed. Check the provider before retrying.")
		return
	}
	s.updateJob(j, "deleting", "Deletion requested. Confirm in your cloud console; billing ends only after resources are removed.")
	s.writeFoundationJobResponse(w, r, j, 202, s.jobsSnapshot(j.ID))
}
func (s *Server) jobsSnapshot(id string) CloudJob {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.jobs[id]
}
func (s *Server) checkCloudDNS(w http.ResponseWriter, r *http.Request) {
	j, ok := s.jobForRequest(w, r)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	ips, err := net.DefaultResolver.LookupHost(ctx, j.Domain)
	matched := false
	for _, ip := range ips {
		if ip == j.PublicIP && ip != "" {
			matched = true
		}
	}
	message := fmt.Sprintf("Add an A record for %s pointing to %s, then check again.", j.Domain, j.PublicIP)
	if matched {
		message = "DNS matches your server. Deploy an application to let Coolify provision HTTPS, then verify its release health."
	}
	if err != nil {
		ips = []string{}
	}
	s.writeFoundationJobResponse(w, r, j, 200, map[string]any{"domain": j.Domain, "expectedIp": j.PublicIP, "addresses": ips, "ready": matched, "message": message})
}
