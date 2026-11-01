package api

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/backend/e2bbe"
	"github.com/lcfrai/luci/control-plane/internal/httpx"
	"github.com/lcfrai/luci/control-plane/internal/management"
)

type cloudElasticReview struct {
	ReviewID string             `json:"reviewId"`
	Policy   CloudElasticPolicy `json:"policy"`
	Digest   string             `json:"digest"`
}
type cloudElasticView struct {
	Configured bool                   `json:"configured"`
	Reason     string                 `json:"reason,omitempty"`
	Operator   *AzureElasticOperator  `json:"operator,omitempty"`
	Policy     *CloudElasticPolicy    `json:"policy,omitempty"`
	Hosts      []cloudElasticHostView `json:"hosts"`
	Spend      *cloudElasticSpend     `json:"spend,omitempty"`
}
type cloudElasticSpend struct {
	PolicyUsedMicros        int64 `json:"policyUsedMicros"`
	PolicyRemainingMicros   int64 `json:"policyRemainingMicros"`
	OperatorUsedMicros      int64 `json:"operatorUsedMicros"`
	OperatorRemainingMicros int64 `json:"operatorRemainingMicros"`
	Unknown                 bool  `json:"unknown"`
	Cumulative              bool  `json:"cumulative"`
}
type cloudElasticHostView struct {
	ID             string    `json:"id"`
	State          string    `json:"state"`
	EstimateMicros int64     `json:"estimateMicros"`
	CreatedAt      time.Time `json:"createdAt"`
	ExpiresAt      time.Time `json:"expiresAt"`
	Retired        bool      `json:"retired"`
}
type cloudElasticInput struct {
	Harness          string    `json:"harness"`
	ExpectedRevision int       `json:"expectedRevision"`
	MaxHosts         int       `json:"maxHosts"`
	HostSeconds      int       `json:"hostSeconds"`
	IdleSeconds      int       `json:"idleSeconds"`
	BudgetMicros     int64     `json:"budgetMicros"`
	ExpiresAt        time.Time `json:"expiresAt"`
	ReviewDigest     string    `json:"reviewDigest,omitempty"`
	ReviewID         string    `json:"reviewId,omitempty"`
	AcknowledgeCost  bool      `json:"acknowledgeCost"`
	Disable          bool      `json:"disable,omitempty"`
}

func (a *API) cloudElasticAuthority(ctx context.Context, actor management.ExecutionActorRef, harness string) error {
	if actor.Local || actor.UserID == "" || a.deps.AuthorizeExecutionControl == nil || a.deps.AuthorizeExecutionControl(ctx, actor) != nil {
		return management.ErrExecutionAuthority
	}
	current, err := a.authorizeExecution(ctx, actor, harness, true)
	if err != nil || !current.Policy.AllowCloudProvisioning || current.Policy.MonthlyBudgetUSD <= 0 {
		return management.ErrExecutionAuthority
	}
	return nil
}
func (a *API) handleCloudElasticity(w http.ResponseWriter, r *http.Request) {
	refuse := func() {
		httpx.WriteProblem(w, 409, "cloud_capacity_review_refused", "The capacity review is no longer valid. Read current limits and prepare a new review; no policy write was attempted.", nil)
	}
	fail := func() {
		httpx.WriteProblem(w, 409, "cloud_capacity_changed", "Cloud capacity authority, reviewed limits or original host state changed. Refresh; an uncertain provisioning request is never repeated.", nil)
	}
	actor, err := management.CaptureExecutionControlActor(r.Context(), r.PathValue("id"))
	if err != nil || a.deps.AuthorizeExecutionControl == nil || a.deps.AuthorizeExecutionControl(r.Context(), actor) != nil || a.deps.AuthorizeResearch == nil || a.deps.AuthorizeResearch(r.Context(), actor, true) != nil {
		httpx.WriteProblem(w, 403, "cloud_capacity_forbidden", "Current project control is required.", nil)
		return
	}
	cloud := a.deps.CloudTasks
	if cloud == nil || cloud.Pool == nil || cloud.Pool.elastic == nil {
		if r.Method == "GET" {
			httpx.WriteJSON(w, 200, cloudElasticView{Reason: "Connect an Azure workspace prepared for automatic hosts in Setup before reviewing capacity.", Hosts: []cloudElasticHostView{}})
		} else {
			fail()
		}
		return
	}
	p := cloud.Pool
	if r.Method == "GET" {
		// Secrets protects private infrastructure observations even without a write.
		// The retained owner may inspect/disable capacity after new execution is revoked.
		p.mu.Lock()
		j, e := p.store.read()
		view := cloudElasticView{Configured: !p.elastic.operator.Disabled, Operator: &p.elastic.operator, Hosts: []cloudElasticHostView{}}
		spendInvalid := false
		if p.elastic.operator.Disabled {
			view.Operator = nil
			view.Reason = "Provisioning is unavailable; original capacity and cleanup records remain retained."
		}
		if e == nil && j.Elastic != nil {
			if policy, ok := j.Elastic.Policies[elasticPolicyKey(actor)]; ok {
				view.Policy = &policy
			}
			for _, h := range j.Elastic.Hosts {
				if h.Actor == actor {
					view.Hosts = append(view.Hosts, cloudElasticHostView{h.ID, h.State, h.EstimateMicros, h.CreatedAt, h.ExpiresAt, h.Retired})
				}
			}
		}
		if e == nil && j.Elastic != nil {
			usage := p.aggregateUsage(j, p.snapshots(), actor, time.Now().UTC())
			spend := cloudElasticSpend{OperatorUsedMicros: usage.EstimateMicros, OperatorRemainingMicros: max(int64(0), min(p.elastic.operator.BudgetMicros, p.config.Limits.EstimatedCostMicros)-usage.EstimateMicros), Unknown: usage.Unknown, Cumulative: true}
			for _, h := range j.Elastic.Hosts {
				if h.Actor == actor && !h.NetworkRejected && !(h.Retired && !h.NetworkAttempted) {
					if spend.PolicyUsedMicros > (1<<50)-h.EstimateMicros {
						spend.Unknown = true
						spendInvalid = true
						break
					}
					spend.PolicyUsedMicros += h.EstimateMicros
				}
			}
			if view.Policy != nil {
				spend.PolicyRemainingMicros = max(int64(0), view.Policy.BudgetMicros-spend.PolicyUsedMicros)
			}
			if spend.OperatorUsedMicros < 0 || spend.OperatorUsedMicros > 1<<50 {
				spendInvalid = true
			}
			view.Spend = &spend
		}
		p.mu.Unlock()
		if e != nil || spendInvalid || a.deps.AuthorizeExecutionControl(r.Context(), actor) != nil || a.deps.AuthorizeResearch(r.Context(), actor, true) != nil {
			fail()
			return
		}
		httpx.WriteJSON(w, 200, view)
		return
	}
	var input cloudElasticInput
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&input) != nil || decoder.Decode(new(any)) != io.EOF {
		fail()
		return
	}
	if input.Disable || r.Pattern == "POST /v1/product/projects/{id}/cloud-capacity/automation/disable" {
		// Revoked new allocation rights do not prevent turning off future capacity.
		p.mu.Lock()
		j, e := p.store.read()
		if e == nil && j.Elastic == nil {
			e = e2bbe.ErrConflict
		}
		if e == nil {
			policy, ok := j.Elastic.Policies[elasticPolicyKey(actor)]
			if !ok || policy.Actor != actor || (policy.Revision != input.ExpectedRevision && !(!policy.Enabled && policy.Revision == input.ExpectedRevision+1)) {
				e = e2bbe.ErrConflict
			} else {
				if policy.Enabled {
					policy.Enabled = false
					policy.Revision++
				}
				j.Elastic.Policies[policy.ID] = policy
				e = p.store.write(j)
			}
		}
		p.mu.Unlock()
		if e != nil {
			fail()
			return
		}
		httpx.WriteJSON(w, 200, map[string]bool{"disabled": true})
		return
	}
	if r.Method != "PUT" && a.cloudElasticAuthority(r.Context(), actor, input.Harness) != nil {
		refuse()
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	j, err := p.store.read()
	if err != nil || j.Elastic == nil {
		fail()
		return
	}
	op := p.elastic.operator
	old := j.Elastic.Policies[elasticPolicyKey(actor)]
	if r.Method == "PUT" {
		retained, ok := j.Elastic.Reviews[input.ReviewID]
		if !ok || retained.ReviewID != input.ReviewID || retained.Digest != input.ReviewDigest || retained.Policy.Actor != actor || !input.AcknowledgeCost {
			refuse()
			return
		}
		if old.ID == retained.Policy.ID && cloudPlacementDigest(old) == retained.Digest {
			httpx.WriteJSON(w, 200, retained)
			return
		}
		if !elasticPolicyFits(retained.Policy, op) {
			refuse()
			return
		}
		current, e := a.authorizeExecution(r.Context(), actor, retained.Policy.Harness, true)
		if e != nil || a.cloudElasticAuthority(r.Context(), actor, retained.Policy.Harness) != nil || retained.Policy.BudgetMicros > cloudPolicyBudget(current.Policy.MonthlyBudgetUSD) || !time.Now().Before(retained.Policy.ExpiresAt) || retained.Policy.ProfileDigest != op.ProfileDigest || retained.Policy.CatalogueDigest != p.catalogueDigest() || old.Revision != retained.Policy.Revision-1 || input.ExpectedRevision != old.Revision {
			refuse()
			return
		}
		j.Elastic.Policies[retained.Policy.ID] = retained.Policy
		if err = p.store.write(j); err != nil {
			fail()
			return
		}
		httpx.WriteJSON(w, 200, retained)
		return
	}
	if op.Disabled {
		refuse()
		return
	}
	current, err := a.authorizeExecution(r.Context(), actor, input.Harness, true)
	if err != nil || old.Revision != input.ExpectedRevision || input.MaxHosts < 1 || input.MaxHosts > op.MaxHosts || input.HostSeconds < op.MinHostSeconds || input.HostSeconds > op.MaxHostSeconds || input.IdleSeconds < 1 || input.IdleSeconds > input.HostSeconds || input.BudgetMicros < 1 || input.BudgetMicros > op.BudgetMicros || input.BudgetMicros > cloudPolicyBudget(current.Policy.MonthlyBudgetUSD) || !input.ExpiresAt.After(time.Now().Add(time.Duration(input.HostSeconds)*time.Second)) {
		refuse()
		return
	}
	policy := CloudElasticPolicy{ID: elasticPolicyKey(actor), Actor: actor, Harness: input.Harness, Revision: old.Revision + 1, ProfileDigest: op.ProfileDigest, CatalogueDigest: p.catalogueDigest(), MaxHosts: input.MaxHosts, HostSeconds: input.HostSeconds, IdleSeconds: input.IdleSeconds, BudgetMicros: input.BudgetMicros, ExpiresAt: input.ExpiresAt, Enabled: true}
	if !elasticPolicyFits(policy, op) {
		refuse()
		return
	}
	var raw [16]byte
	if _, err = rand.Read(raw[:]); err != nil {
		fail()
		return
	}
	review := cloudElasticReview{ReviewID: "capacity-" + hex.EncodeToString(raw[:]), Policy: policy, Digest: cloudPlacementDigest(policy)}
	if r.Method != "POST" {
		fail()
		return
	}
	if j.Elastic.Reviews == nil {
		j.Elastic.Reviews = map[string]cloudElasticReview{}
	}
	j.Elastic.Reviews[review.ReviewID] = review
	if err = p.store.write(j); err != nil {
		fail()
		return
	}
	httpx.WriteJSON(w, 200, review)
}
