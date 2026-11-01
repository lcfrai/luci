package api

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/backend/e2bbe"
	"github.com/lcfrai/luci/control-plane/internal/management"
)

// Operator configuration supplies the only permitted image/network/identity
// template. A project may narrow these limits, never provide a provider URL.
type AzureElasticOperator struct {
	Disabled              bool   `json:"disabled,omitempty"`
	ParentHostID          string `json:"parentHostId"`
	ProfileDigest         string `json:"profileDigest"`
	BootstrapSeconds      int    `json:"bootstrapSeconds"`
	MinHostSeconds        int    `json:"minHostSeconds"`
	MaxHosts              int    `json:"maxHosts"`
	HostCostMicrosPerHour int64  `json:"hostCostMicrosPerHour"`
	MaxHostSeconds        int    `json:"maxHostSeconds"`
	BudgetMicros          int64  `json:"budgetMicros"`
}
type CloudElasticPolicy struct {
	ID              string                       `json:"id"`
	Actor           management.ExecutionActorRef `json:"actor"`
	Harness         string                       `json:"harness"`
	Revision        int                          `json:"revision"`
	ProfileDigest   string                       `json:"profileDigest"`
	CatalogueDigest string                       `json:"catalogueDigest"`
	MaxHosts        int                          `json:"maxHosts"`
	HostSeconds     int                          `json:"hostSeconds"`
	IdleSeconds     int                          `json:"idleSeconds"`
	BudgetMicros    int64                        `json:"budgetMicros"`
	ExpiresAt       time.Time                    `json:"expiresAt"`
	Enabled         bool                         `json:"enabled"`
}
type AzureElasticHost struct {
	ID               string                       `json:"id"`
	PolicyID         string                       `json:"policyId"`
	OperatorDigest   string                       `json:"operatorDigest"`
	PolicySnapshot   CloudElasticPolicy           `json:"policy"`
	OperatorSnapshot AzureElasticOperator         `json:"operator"`
	Actor            management.ExecutionActorRef `json:"actor"`
	CreatedAt        time.Time                    `json:"createdAt"`
	IdleSince        time.Time                    `json:"idleSince"`
	ExpiresAt        time.Time                    `json:"expiresAt"`
	EstimateMicros   int64                        `json:"estimateMicros"`
	// Original individual resource requests are persisted before any Azure write.
	ScaleSetID             string          `json:"scaleSetId"`
	ResourceID             string          `json:"resourceId"`
	OwnershipMarker        string          `json:"ownershipMarker"`
	NetworkRequest         json.RawMessage `json:"networkRequest,omitempty"`
	VMRequest              json.RawMessage `json:"vmRequest,omitempty"`
	NICDeleteAttempted     bool            `json:"nicDeleteAttempted"`
	NICResourceID          string          `json:"nicResourceId"`
	NICUID                 string          `json:"nicUid"`
	DiskID                 string          `json:"diskId"`
	DiskUID                string          `json:"diskUid"`
	NetworkAttempted       bool            `json:"networkAttempted"`
	CreateRejected         bool            `json:"createRejected"`
	NetworkRejected        bool            `json:"networkRejected"`
	NetworkARMOperation    string          `json:"networkArmOperation,omitempty"`
	VMARMOperation         string          `json:"vmArmOperation,omitempty"`
	NetworkRequestID       string          `json:"networkRequestId,omitempty"`
	VMRequestID            string          `json:"vmRequestId,omitempty"`
	ModelDigest            string          `json:"modelDigest"`
	ScaleAttempted         bool            `json:"scaleAttempted"`
	InstanceID             string          `json:"instanceId"`
	VMID                   string          `json:"vmId"`
	NICID                  string          `json:"nicId"`
	PrivateIP              string          `json:"privateIp"`
	NodeID                 string          `json:"nodeId"`
	TemplateID             string          `json:"templateId"`
	BuildID                string          `json:"buildId"`
	ConfigDigest           string          `json:"configDigest"`
	State                  string          `json:"state"`
	DeleteAttempted        bool            `json:"deleteAttempted"`
	Retired                bool            `json:"retired"`
	PendingProductOutcomes bool            `json:"pendingProductOutcomes,omitempty"`
}
type cloudElasticLedger struct {
	Policies map[string]CloudElasticPolicy `json:"policies"`
	Hosts    map[string]AzureElasticHost   `json:"hosts"`
	Reviews  map[string]cloudElasticReview `json:"reviews,omitempty"`
}

// The worker implements these fixed Azure/bootstrap operations. Step performs
// one recorded phase; it never allocates a task or starts an upstream harness.
type AzureElasticDriver interface {
	Prepare(context.Context, AzureElasticHost) (AzureElasticHost, error)
	CreateNetwork(context.Context, AzureElasticHost, func() error) (AzureElasticHost, error)
	CreateVM(context.Context, AzureElasticHost, func() error) (AzureElasticHost, error)
	Observe(context.Context, AzureElasticHost) (AzureElasticHost, error)
	Publish(context.Context, AzureElasticHost, func() error) (AzureElasticHost, error)
	Open(context.Context, AzureElasticHost, func(context.Context, e2bbe.Request, int64) error) (CloudTaskHost, func(), error)
	Delete(context.Context, AzureElasticHost) error
	Absent(context.Context, AzureElasticHost) (bool, bool, error)
	DeleteNIC(context.Context, AzureElasticHost) error
	Idle(context.Context, AzureElasticHost) (bool, error)
}
type cloudElasticRuntime struct {
	operator AzureElasticOperator
	driver   AzureElasticDriver
	slots    int
	// Immutable maps are replaced under p.mu. Readers never range a mutating map.
	cursor  uint64
	closes  map[string]func()
	working bool
}

func elasticDigest(value string, n int) bool {
	if len(value) != n {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}
func validElasticPolicy(p CloudElasticPolicy) bool {
	return p.ID == elasticPolicyKey(p.Actor) && !p.Actor.Local && p.Actor.UserID != "" && p.Actor.ProjectID != "" && p.Revision > 0 && p.Revision < int(^uint(0)>>1) && (p.Harness == "codex" || p.Harness == "claude-code") && elasticDigest(p.ProfileDigest, 64) && elasticDigest(p.CatalogueDigest, 64) && p.MaxHosts > 0 && p.HostSeconds > 0 && int64(p.HostSeconds) <= int64((1<<63-1)/int64(time.Second)) && p.IdleSeconds > 0 && p.IdleSeconds <= p.HostSeconds && p.BudgetMicros > 0 && p.BudgetMicros <= 1<<50 && !p.ExpiresAt.IsZero()
}
func elasticPolicyFits(p CloudElasticPolicy, op AzureElasticOperator) bool {
	return validElasticPolicy(p) && !op.Disabled && p.ProfileDigest == op.ProfileDigest && p.MaxHosts <= op.MaxHosts && p.HostSeconds >= op.MinHostSeconds && p.HostSeconds <= op.MaxHostSeconds && p.BudgetMicros <= op.BudgetMicros
}
func elasticHostCost(seconds int, rate int64) (int64, bool) {
	if seconds < 1 || rate < 1 || int64(seconds) > (math.MaxInt64-3599)/rate {
		return 0, false
	}
	return (int64(seconds)*rate + 3599) / 3600, true
}
func validCloudElastic(j *cloudElasticLedger) bool {
	if j == nil {
		return true
	}
	if j.Policies == nil || j.Hosts == nil {
		return false
	}
	for id, p := range j.Policies {
		if id != p.ID || !validElasticPolicy(p) {
			return false
		}
	}
	for id, r := range j.Reviews {
		if id != r.ReviewID || !strings.HasPrefix(id, "capacity-") || !elasticDigest(strings.TrimPrefix(id, "capacity-"), 32) || !validElasticPolicy(r.Policy) || r.Digest != cloudPlacementDigest(r.Policy) {
			return false
		}
	}
	for id, h := range j.Hosts {
		current, ok := j.Policies[h.PolicyID]
		original := h.PolicySnapshot
		operator := h.OperatorSnapshot
		cost, validCost := elasticHostCost(original.HostSeconds, operator.HostCostMicrosPerHour)
		if id != h.ID || !strings.HasPrefix(id, "host-") || !elasticDigest(strings.TrimPrefix(id, "host-"), 32) || !ok || h.Actor != current.Actor || h.Actor != original.Actor || h.PolicyID != original.ID || !validElasticPolicy(original) || !elasticPolicyFits(original, operator) || h.OperatorDigest != operator.ProfileDigest || !elasticDigest(h.OwnershipMarker, 32) || h.CreatedAt.IsZero() || !h.ExpiresAt.Equal(h.CreatedAt.Add(time.Duration(original.HostSeconds)*time.Second)) || h.ExpiresAt.After(original.ExpiresAt) || !validCost || cost != h.EstimateMicros || cost > 1<<50 {
			return false
		}
		switch h.State {
		case "reserved", "network-unknown", "vm-pending", "scale-unknown", "booting", "ready", "draining", "delete-unknown", "retired":
		default:
			return false
		}
		if h.NetworkAttempted && (h.ResourceID == "" || h.NICResourceID == "" || !elasticDigest(h.ModelDigest, 64) || len(h.NetworkRequest) == 0 || len(h.NetworkRequest) > 128<<10 || len(h.VMRequest) == 0 || len(h.VMRequest) > 128<<10 || !json.Valid(h.NetworkRequest) || !json.Valid(h.VMRequest)) {
			return false
		}
		if h.ScaleAttempted && (!h.NetworkAttempted || h.NICUID == "") || h.DeleteAttempted && !h.NetworkAttempted || h.NICDeleteAttempted && !h.DeleteAttempted || h.Retired && h.NetworkAttempted && !h.NetworkRejected && !h.DeleteAttempted {
			return false
		}
		if h.State == "ready" && (h.VMID == "" || h.PrivateIP == "" || h.NodeID == "" || h.TemplateID == "" || h.BuildID == "" || !elasticDigest(h.ConfigDigest, 64)) {
			return false
		}
	}
	return true
}

func (p *CloudTaskPool) ConfigureAzureElasticity(operator AzureElasticOperator, driver AzureElasticDriver, pendingTaskSlots ...int) error {
	if p == nil || driver == nil || !cloudHostID(operator.ParentHostID) || len(operator.ProfileDigest) != 64 || operator.MaxHosts < 1 || operator.BootstrapSeconds < 1 || operator.MaxHostSeconds <= operator.BootstrapSeconds || int64(operator.MaxHostSeconds) > int64((1<<63-1)/int64(time.Second)) || operator.HostCostMicrosPerHour < 1 || operator.HostCostMicrosPerHour > 1<<50 || operator.BudgetMicros < 1 || operator.BudgetMicros > 1<<50 || int64(operator.MaxHostSeconds) > (math.MaxInt64-3599)/operator.HostCostMicrosPerHour {
		return e2bbe.ErrUnsupported
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	parent, ok := p.hosts[operator.ParentHostID]
	if !ok || parent.Runtime.Runtime.Controller.Provider() != "azure" || p.elastic != nil {
		return e2bbe.ErrUnsupported
	}
	j, err := p.store.read()
	if err != nil {
		return err
	}
	if j.Elastic == nil {
		j.Elastic = &cloudElasticLedger{Policies: map[string]CloudElasticPolicy{}, Hosts: map[string]AzureElasticHost{}, Reviews: map[string]cloudElasticReview{}}
		if err = p.store.write(j); err != nil {
			return err
		}
	}
	operator.MinHostSeconds = operator.BootstrapSeconds + parent.Runtime.MaxSeconds
	if !operator.Disabled && operator.MaxHostSeconds < operator.MinHostSeconds {
		return e2bbe.ErrUnsupported
	}
	slots := 1
	if len(pendingTaskSlots) == 1 {
		slots = pendingTaskSlots[0]
	}
	if slots < 1 || slots > p.config.Limits.MaxConcurrent {
		return e2bbe.ErrUnsupported
	}
	p.elastic = &cloudElasticRuntime{operator: operator, driver: driver, slots: slots, closes: map[string]func(){}}
	return nil
}

func (p *CloudTaskPool) allHosts() map[string]CloudTaskHost {
	p.dynamicMu.RLock()
	defer p.dynamicMu.RUnlock()
	result := make(map[string]CloudTaskHost, len(p.hosts)+len(p.dynamicHosts))
	for id, h := range p.hosts {
		result[id] = h
	}
	for id, h := range p.dynamicHosts {
		result[id] = h
	}
	return result
}
func (p *CloudTaskPool) publishElasticHost(host CloudTaskHost) {
	p.dynamicMu.Lock()
	defer p.dynamicMu.Unlock()
	if p.dynamicHosts == nil {
		p.dynamicHosts = map[string]CloudTaskHost{}
	}
	p.dynamicHosts[host.ID] = host
}
func (p *CloudTaskPool) elasticUsage(j cloudPlacementJournal, actor management.ExecutionActorRef, now time.Time) (global, month int64, unknown bool) {
	if j.Elastic == nil {
		return
	}
	for _, h := range j.Elastic.Hosts {
		if h.NetworkRejected || h.Retired && !h.NetworkAttempted {
			continue
		}
		if global > math.MaxInt64-h.EstimateMicros {
			return 0, 0, true
		}
		global += h.EstimateMicros
		if cloudBillApplies(h.Actor.UserID, h.Actor.TeamID, actor) && (!h.Retired || h.CreatedAt.UTC().Format("2006-01") == now.UTC().Format("2006-01")) {
			if month > math.MaxInt64-h.EstimateMicros {
				return 0, 0, true
			}
			month += h.EstimateMicros
		}
		// Passing a host's reviewed billing window never certifies it absent.
		if !h.Retired && !now.Before(h.ExpiresAt) {
			unknown = true
		}
	}
	return
}
func (p *CloudTaskPool) aggregateUsage(j cloudPlacementJournal, snapshots []cloudHostSnapshot, actor management.ExecutionActorRef, now time.Time) cloudPoolUsage {
	u := cloudAggregateUsage(j.Rows, snapshots, actor, now)
	g, m, unknown := p.elasticUsage(j, actor, now)
	if u.EstimateMicros > math.MaxInt64-g || u.ActorMonthMicros > math.MaxInt64-m {
		u.Unknown = true
		return u
	}
	u.EstimateMicros += g
	u.ActorMonthMicros += m
	u.Unknown = u.Unknown || unknown
	return u
}
func elasticPolicyKey(actor management.ExecutionActorRef) string {
	return "elastic-" + cloudPlacementDigest(actor)[:32]
}
func (p *CloudTaskPool) elasticCandidates(actor management.ExecutionActorRef, allowed []string) []CloudTaskHost {
	result := p.candidates(allowed)
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.elastic == nil {
		return result
	}
	j, err := p.store.read()
	if err != nil || j.Elastic == nil {
		return result
	}
	policy, ok := j.Elastic.Policies[elasticPolicyKey(actor)]
	if !ok || !policy.Enabled || policy.Actor != actor || !time.Now().Before(policy.ExpiresAt) || policy.ProfileDigest != p.elastic.operator.ProfileDigest || policy.CatalogueDigest != p.catalogueDigest() {
		return result
	}
	permitted := len(allowed) == 0 && p.config.DefaultHostID == p.elastic.operator.ParentHostID
	for _, id := range allowed {
		permitted = permitted || id == p.elastic.operator.ParentHostID
	}
	if !permitted {
		return result
	}
	ready := []AzureElasticHost{}
	for _, h := range j.Elastic.Hosts {
		if h.PolicyID == policy.ID && !h.Retired && h.State == "ready" {
			ready = append(ready, h)
		}
	}
	sort.Slice(ready, func(i, j int) bool {
		if !ready[i].CreatedAt.Equal(ready[j].CreatedAt) {
			return ready[i].CreatedAt.Before(ready[j].CreatedAt)
		}
		return ready[i].ID < ready[j].ID
	})
	entitled := map[string]bool{}
	for _, h := range ready[:min(len(ready), policy.MaxHosts)] {
		entitled[h.ID] = true
	}
	for id, h := range j.Elastic.Hosts {
		if !entitled[id] || h.OperatorDigest != policy.ProfileDigest || h.PolicyID != policy.ID || h.State != "ready" || h.Retired || !time.Now().Add(time.Duration(p.hosts[p.elastic.operator.ParentHostID].Runtime.MaxSeconds)*time.Second).Before(h.ExpiresAt) {
			continue
		}
		if host, ok := p.allHosts()[id]; ok && !host.Draining {
			result = append(result, host)
		}
	}
	return result
}

// Called by the existing scheduler only after its ordinary queued admission.
// A denied/no-demand call never mints a capacity request.
func (a *API) requestCloudExpansion(ctx context.Context, actor management.ExecutionActorRef, harness string) error {
	cloud := a.deps.CloudTasks
	if cloud == nil || cloud.Pool == nil {
		return e2bbe.ErrCapacity
	}
	p := cloud.Pool
	current, err := a.authorizeExecution(ctx, actor, harness, true)
	if err != nil || !current.Policy.AllowCloudProvisioning || current.Policy.MonthlyBudgetUSD <= 0 {
		return management.ErrExecutionAuthority
	}
	if p.elastic == nil || p.elastic.operator.Disabled {
		return e2bbe.ErrCapacity
	}
	demand, err := a.deps.Product.CloudHostDemand(ctx, actor, harness, p.elastic.operator.ParentHostID, p.catalogueDigest())
	if err != nil {
		return err
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.elastic == nil || p.elastic.operator.Disabled {
		return e2bbe.ErrCapacity
	}
	j, err := p.store.read()
	if err != nil || j.Elastic == nil {
		return e2bbe.ErrUnknown
	}
	policy, ok := j.Elastic.Policies[elasticPolicyKey(actor)]
	now := time.Now().UTC()
	if !ok || !policy.Enabled || policy.Actor != actor || policy.Harness != harness || policy.ProfileDigest != p.elastic.operator.ProfileDigest || policy.CatalogueDigest != p.catalogueDigest() || !now.Add(time.Duration(policy.HostSeconds)*time.Second).Before(policy.ExpiresAt) {
		return management.ErrExecutionAuthority
	}
	active, own, pendingSlots := 0, 0, 0
	var ownSpend int64
	for _, h := range j.Elastic.Hosts {
		if h.PolicyID == policy.ID && !h.NetworkRejected && !(h.Retired && !h.NetworkAttempted) {
			if ownSpend > math.MaxInt64-h.EstimateMicros {
				return e2bbe.ErrCapacity
			}
			ownSpend += h.EstimateMicros
		}
		if !h.Retired {
			active++
			if h.PolicyID == policy.ID {
				own++
				if h.OperatorDigest == policy.ProfileDigest && h.State != "ready" && h.State != "draining" && h.State != "delete-unknown" && time.Now().Before(h.CreatedAt.Add(time.Duration(p.elastic.operator.BootstrapSeconds)*time.Second)) {
					pendingSlots += p.elastic.slots
				}
			}
		}
	}
	unbound := 0
	for _, work := range demand {
		if row, ok := j.Rows[work.ChildID]; !ok || !row.Admitted {
			unbound++
		}
	}
	if unbound <= pendingSlots {
		return e2bbe.ErrCapacity
	}
	if int64(policy.HostSeconds) > (math.MaxInt64-3599)/p.elastic.operator.HostCostMicrosPerHour {
		return e2bbe.ErrCapacity
	}
	cost, validCost := elasticHostCost(policy.HostSeconds, p.elastic.operator.HostCostMicrosPerHour)
	if !validCost {
		return e2bbe.ErrCapacity
	}
	usage := p.aggregateUsage(j, p.snapshots(), actor, now)
	if active >= p.elastic.operator.MaxHosts || own >= policy.MaxHosts || cost < 1 || ownSpend > policy.BudgetMicros-cost || usage.Unknown || usage.EstimateMicros > p.elastic.operator.BudgetMicros-cost || usage.EstimateMicros > p.config.Limits.EstimatedCostMicros-cost || usage.ActorMonthMicros > cloudPolicyBudget(current.Policy.MonthlyBudgetUSD)-cost {
		return e2bbe.ErrCapacity
	}
	var raw, marker [16]byte
	if _, err = rand.Read(raw[:]); err != nil {
		return err
	}
	if _, err = rand.Read(marker[:]); err != nil {
		return err
	}
	id := "host-" + hex.EncodeToString(raw[:])
	j.Elastic.Hosts[id] = AzureElasticHost{ID: id, PolicyID: policy.ID, PolicySnapshot: policy, OperatorSnapshot: p.elastic.operator, OperatorDigest: p.elastic.operator.ProfileDigest, OwnershipMarker: hex.EncodeToString(marker[:]), Actor: actor, CreatedAt: now, ExpiresAt: now.Add(time.Duration(policy.HostSeconds) * time.Second), EstimateMicros: cost, State: "reserved"}
	return p.store.write(j)
}

// One original phase per existing observer tick. Provider calls never hold p.mu.
func (a *API) advanceCloudElasticity(ctx context.Context) error {
	cloud := a.deps.CloudTasks
	if cloud == nil || cloud.Pool == nil {
		return nil
	}
	p := cloud.Pool
	p.mu.Lock()
	if p.elastic == nil || p.elastic.working {
		p.mu.Unlock()
		return nil
	}
	j, err := p.store.read()
	if err != nil || j.Elastic == nil {
		p.mu.Unlock()
		return err
	}
	ids := []string{}
	for id, h := range j.Elastic.Hosts {
		if !h.Retired || h.PendingProductOutcomes {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	if len(ids) == 0 {
		p.mu.Unlock()
		return nil
	}
	p.elastic.cursor++
	index := int(p.elastic.cursor % uint64(len(ids)))
	h := j.Elastic.Hosts[ids[index]]
	policy := j.Elastic.Policies[h.PolicyID]
	p.elastic.working = true
	driver := p.elastic.driver
	p.mu.Unlock()
	defer func() { p.mu.Lock(); p.elastic.working = false; p.mu.Unlock() }()
	authorize := func() error {
		c, e := a.authorizeExecution(ctx, h.Actor, policy.Harness, true)
		if e != nil || !c.Policy.AllowCloudProvisioning || c.Policy.MonthlyBudgetUSD <= 0 {
			return management.ErrExecutionAuthority
		}
		demand, e := a.deps.Product.CloudHostDemand(ctx, h.Actor, policy.Harness, p.elastic.operator.ParentHostID, p.catalogueDigest())
		if e != nil {
			return e
		}
		p.mu.Lock()
		defer p.mu.Unlock()
		current, e := p.store.read()
		if e != nil || current.Elastic == nil {
			return e2bbe.ErrUnknown
		}
		unbound := 0
		for _, work := range demand {
			if row, ok := current.Rows[work.ChildID]; !ok || !row.Admitted {
				unbound++
			}
		}
		pending := []AzureElasticHost{}
		for _, candidate := range current.Elastic.Hosts {
			if candidate.PolicyID == h.PolicyID && !candidate.Retired && candidate.State != "ready" && candidate.State != "draining" && candidate.State != "delete-unknown" && candidate.OperatorDigest == h.OperatorDigest && time.Now().Before(candidate.CreatedAt.Add(time.Duration(p.elastic.operator.BootstrapSeconds)*time.Second)) {
				pending = append(pending, candidate)
			}
		}
		sort.Slice(pending, func(i, j int) bool {
			if !pending[i].CreatedAt.Equal(pending[j].CreatedAt) {
				return pending[i].CreatedAt.Before(pending[j].CreatedAt)
			}
			return pending[i].ID < pending[j].ID
		})
		rank := -1
		for i, candidate := range pending {
			if candidate.ID == h.ID {
				rank = i
				break
			}
		}
		if rank < 0 || rank >= unbound || rank*p.elastic.slots >= unbound {
			return e2bbe.ErrCapacity
		}
		currentPolicy, exists := current.Elastic.Policies[h.PolicyID]
		if !exists {
			return e2bbe.ErrUnknown
		}
		hard, retainedCost := 0, int64(0)
		for _, original := range current.Elastic.Hosts {
			if original.PolicyID != h.PolicyID || original.NetworkRejected || original.Retired && !original.NetworkAttempted {
				continue
			}
			if retainedCost > math.MaxInt64-original.EstimateMicros {
				return e2bbe.ErrCapacity
			}
			retainedCost += original.EstimateMicros
			if !original.Retired && (original.State == "ready" || original.State == "draining" || original.State == "delete-unknown" || original.OperatorDigest != h.OperatorDigest) {
				hard++
			}
		}
		if hard >= currentPolicy.MaxHosts || rank >= currentPolicy.MaxHosts-hard || retainedCost > currentPolicy.BudgetMicros {
			return e2bbe.ErrCapacity
		}
		latest, ok := current.Elastic.Policies[policy.ID]
		if h.ConfigDigest == "" && time.Now().After(h.CreatedAt.Add(time.Duration(p.elastic.operator.BootstrapSeconds)*time.Second)) {
			return management.ErrExecutionAuthority
		}
		if !ok || !latest.Enabled || latest.Actor != h.Actor || p.elastic.operator.Disabled || h.OperatorDigest != p.elastic.operator.ProfileDigest || latest.ProfileDigest != p.elastic.operator.ProfileDigest || latest.CatalogueDigest != p.catalogueDigest() || !time.Now().Before(latest.ExpiresAt) || !time.Now().Before(h.ExpiresAt) {
			return management.ErrExecutionAuthority
		}
		usage := p.aggregateUsage(current, p.snapshots(), h.Actor, time.Now().UTC())
		if usage.Unknown || usage.ActorMonthMicros > cloudPolicyBudget(c.Policy.MonthlyBudgetUSD) || usage.EstimateMicros > p.config.Limits.EstimatedCostMicros {
			return e2bbe.ErrCapacity
		}
		return nil
	}
	save := func(next AzureElasticHost) error {
		p.mu.Lock()
		defer p.mu.Unlock()
		latest, e := p.store.read()
		if e != nil {
			return e
		}
		old, ok := latest.Elastic.Hosts[h.ID]
		if !ok || cloudPlacementDigest(old) != cloudPlacementDigest(h) {
			return e2bbe.ErrConflict
		}
		latest.Elastic.Hosts[h.ID] = next
		if e = p.store.write(latest); e == nil {
			h = next
		}
		return e
	}
	if h.Retired {
		if !h.PendingProductOutcomes {
			return nil
		}
		if a.deps.AuthorizeResearch == nil || a.deps.AuthorizeResearch(ctx, h.Actor, true) != nil {
			return management.ErrExecutionAuthority
		}
		host, closeHost, e := driver.Open(ctx, h, func(context.Context, e2bbe.Request, int64) error { return management.ErrExecutionAuthority })
		if e != nil {
			return e
		}
		host.Draining = true
		p.publishElasticHost(host)
		defer func() { p.dynamicMu.Lock(); delete(p.dynamicHosts, h.ID); p.dynamicMu.Unlock(); closeHost() }()
		rows, e := host.Runtime.Runtime.Controller.OwnershipSnapshot()
		if e != nil {
			return e
		}
		count := 0
		for _, record := range rows {
			if record.State != e2bbe.Absent || record.ResourcesHeld {
				return e2bbe.ErrUnknown
			}
			if record.ProductOutcome == "" && count < 4 {
				_ = a.observeOneCloudTask(ctx, record)
				count++
			}
		}
		rows, e = host.Runtime.Runtime.Controller.OwnershipSnapshot()
		if e != nil {
			return e
		}
		next := h
		next.PendingProductOutcomes = false
		for _, record := range rows {
			next.PendingProductOutcomes = next.PendingProductOutcomes || record.ProductOutcome == ""
		}
		p.mu.Lock()
		latest, e := p.store.read()
		if e == nil {
			_, settled := host.Runtime.Runtime.Controller.SpendSnapshot()
			e = p.refreshLocked(&latest, []cloudHostSnapshot{{HostID: h.ID, Records: rows, Settled: settled}})
			if e == nil {
				latest.Elastic.Hosts[h.ID] = next
				e = p.store.write(latest)
			}
		}
		p.mu.Unlock()
		return e
	}
	if h.State == "reserved" {
		if !policy.Enabled || time.Now().After(policy.ExpiresAt) || authorize() != nil {
			next := h
			next.State = "retired"
			next.Retired = true
			return save(next)
		}
		next, e := driver.Prepare(ctx, h)
		if e != nil {
			return e
		}
		next.NetworkAttempted = true
		next.State = "network-unknown"
		if e = save(next); e != nil {
			return e
		}
		next, e = driver.CreateNetwork(ctx, h, authorize)
		persist := save(next)
		return errors.Join(e, persist)
	}
	if h.State == "network-unknown" || h.State == "scale-unknown" || h.State == "booting" {
		// Observing an already attempted original needs no renewed spending grant.
		next, e := driver.Observe(ctx, h)
		if e != nil {
			return e
		}
		if e = save(next); e != nil {
			return e
		}
		if next.NetworkRejected {
			next.State = "retired"
			next.Retired = true
			return save(next)
		}
		if authorize() != nil {
			if next.NICUID != "" && (!next.ScaleAttempted || next.VMID != "") {
				next.State = "draining"
				return save(next)
			}
			return nil
		}
		if next.NICUID != "" && !next.ScaleAttempted {
			next.State = "vm-pending"
			return save(next)
		}
		if next.NodeID == "" {
			return nil
		}
		next, e = driver.Publish(ctx, h, authorize)
		if e != nil {
			return e
		}
		if e = save(next); e != nil {
			return e
		}
		if next.BuildID == "" {
			return nil
		}
		host, closeHost, e := driver.Open(ctx, next, func(c context.Context, r e2bbe.Request, b int64) error { return p.AuthorizeAllocation(c, h.ID, r, b) })
		if e != nil {
			return e
		}
		if e = authorize(); e != nil {
			closeHost()
			return e
		}
		next.State = "ready"
		next.ConfigDigest = host.ConfigDigest
		next.IdleSince = time.Now().UTC()
		if e = save(next); e != nil {
			closeHost()
			return e
		}
		p.publishElasticHost(host)
		p.mu.Lock()
		p.elastic.closes[host.ID] = closeHost
		p.mu.Unlock()
		return nil
	}
	if h.State == "vm-pending" {
		if authorize() != nil {
			next := h
			next.State = "draining"
			return save(next)
		}
		next := h
		next.ScaleAttempted = true
		next.State = "scale-unknown"
		if err = save(next); err != nil {
			return err
		}
		next, err = driver.CreateVM(ctx, h, authorize)
		return errors.Join(err, save(next))
	}
	if h.State == "ready" || h.State == "draining" {
		host, ok := p.allHosts()[h.ID]
		if h.State == "draining" && h.ConfigDigest == "" {
			idle, e := driver.Idle(ctx, h)
			if e != nil || !idle {
				return e
			}
			next := h
			next.State = "delete-unknown"
			next.DeleteAttempted = true
			if e = save(next); e != nil {
				return e
			}
			return driver.Delete(ctx, h)
		}
		if !ok { // Restart restores the same private original, never another host.
			var closeHost func()
			host, closeHost, err = driver.Open(ctx, h, func(c context.Context, r e2bbe.Request, b int64) error { return p.AuthorizeAllocation(c, h.ID, r, b) })
			if err != nil {
				return err
			}
			host.Draining = h.State == "draining" || h.Retired
			p.publishElasticHost(host)
			p.mu.Lock()
			p.elastic.closes[h.ID] = closeHost
			p.mu.Unlock()
		}
		rows, e := host.Runtime.Runtime.Controller.OwnershipSnapshot()
		if e != nil {
			return e
		}
		busy := false
		for _, r := range rows {
			busy = busy || r.ResourcesHeld || r.State != e2bbe.Absent
		}
		p.mu.Lock()
		current, e := p.store.read()
		if e == nil {
			_, settled := host.Runtime.Runtime.Controller.SpendSnapshot()
			e = p.refreshLocked(&current, []cloudHostSnapshot{{HostID: h.ID, Records: rows, Settled: settled}})
			if e == nil {
				e = p.store.write(current)
			}
		}
		if e == nil {
			for _, r := range current.Rows {
				if r.HostID == h.ID && r.Admitted && !r.Retired {
					busy = true
				}
			}
		}
		p.mu.Unlock()
		if e != nil {
			return e
		}
		next := h
		if busy {
			next.IdleSince = time.Time{}
			return save(next)
		}
		if next.IdleSince.IsZero() {
			next.IdleSince = time.Now().UTC()
			return save(next)
		}
		due := p.elastic.operator.Disabled || h.OperatorDigest != p.elastic.operator.ProfileDigest || !policy.Enabled || time.Now().After(policy.ExpiresAt) || time.Now().After(h.ExpiresAt) || time.Since(next.IdleSince) >= time.Duration(policy.IdleSeconds)*time.Second
		if !due {
			return nil
		}
		if h.State != "draining" {
			p.mu.Lock()
			latest, e := p.store.read()
			if e != nil {
				p.mu.Unlock()
				return e
			}
			for _, r := range latest.Rows {
				if r.HostID == h.ID && r.Admitted && !r.Retired {
					p.mu.Unlock()
					return nil
				}
			}
			old, found := latest.Elastic.Hosts[h.ID]
			if !found || cloudPlacementDigest(old) != cloudPlacementDigest(h) {
				p.mu.Unlock()
				return e2bbe.ErrConflict
			}
			next.State = "draining"
			host.Draining = true
			p.publishElasticHost(host)
			latest.Elastic.Hosts[h.ID] = next
			e = p.store.write(latest)
			p.mu.Unlock()
			return e
		}
		// The draining publication fences new reservations; final original task
		// settlement was checked above. DELETE is sent once, observation recovers it.
		next.State = "delete-unknown"
		next.DeleteAttempted = true
		if e = save(next); e != nil {
			return e
		}
		return driver.Delete(ctx, h)
	}
	if h.State == "delete-unknown" {
		vmAbsent, allAbsent, e := driver.Absent(ctx, h)
		if e != nil {
			return e
		}
		if vmAbsent && !allAbsent && !h.NICDeleteAttempted {
			next := h
			next.NICDeleteAttempted = true
			if e = save(next); e != nil {
				return e
			}
			return driver.DeleteNIC(ctx, h)
		}
		if !allAbsent {
			return nil
		}
		next := h
		next.Retired = true
		next.State = "retired"
		if host, ok := p.allHosts()[h.ID]; ok {
			rows, e := host.Runtime.Runtime.Controller.OwnershipSnapshot()
			if e != nil {
				return e
			}
			for _, record := range rows {
				if record.State != e2bbe.Absent || record.ResourcesHeld {
					return e2bbe.ErrUnknown
				}
				next.PendingProductOutcomes = next.PendingProductOutcomes || record.ProductOutcome == ""
			}
			p.mu.Lock()
			latest, e := p.store.read()
			if e == nil {
				_, settled := host.Runtime.Runtime.Controller.SpendSnapshot()
				e = p.refreshLocked(&latest, []cloudHostSnapshot{{HostID: h.ID, Records: rows, Settled: settled}})
				if e == nil {
					e = p.store.write(latest)
				}
			}
			p.mu.Unlock()
			if e != nil {
				return e
			}
		} else if h.ConfigDigest != "" {
			next.PendingProductOutcomes = true
		}
		if e = save(next); e != nil {
			return e
		}
		p.detachElasticHost(h.ID)
		return nil
	}
	return errors.New("unrecognized retained cloud host phase")
}

func (p *CloudTaskPool) detachElasticHost(id string) {
	p.dynamicMu.Lock()
	delete(p.dynamicHosts, id)
	p.dynamicMu.Unlock()
	p.mu.Lock()
	closeHost := p.elastic.closes[id]
	delete(p.elastic.closes, id)
	p.mu.Unlock()
	if closeHost != nil {
		closeHost()
	}
}
func (p *CloudTaskPool) terminalRecord(projectID, runID, operationID, childID string) (e2bbe.Record, bool, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	j, err := p.store.read()
	if err != nil {
		return e2bbe.Record{}, false, err
	}
	row, ok := j.Rows[childID]
	if !ok || row.Terminal == nil {
		return e2bbe.Record{}, false, nil
	}
	id := row.Request.Identity
	if id.ProjectID != projectID || id.RunID != runID || id.AttemptID != operationID || id.OperationID != childID {
		return e2bbe.Record{}, false, e2bbe.ErrConflict
	}
	return *row.Terminal, true, nil
}
