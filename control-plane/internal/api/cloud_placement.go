package api

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"math"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unicode"

	"github.com/lcfrai/luci/control-plane/internal/backend/e2bbe"
	"github.com/lcfrai/luci/control-plane/internal/management"
	"github.com/lcfrai/luci/control-plane/internal/product"
	"github.com/lcfrai/luci/control-plane/internal/registry"
)

// Hosts are installed by the authenticated worker's private operator config,
// never by a browser URL. The scheduler still selects project/task order.
type CloudTaskHost struct {
	ID           string
	Name         string
	ConfigDigest string
	HostKey      string
	Draining     bool
	Runtime      *CloudTaskRuntime
}
type CloudTaskPoolConfig struct {
	StateDir      string
	Scope         string
	DefaultHostID string
	Limits        e2bbe.Limits
}
type cloudPlacement struct {
	QueueID          string                       `json:"queueId"`
	Actor            management.ExecutionActorRef `json:"actor"`
	Request          e2bbe.Request                `json:"request"`
	RequestDigest    string                       `json:"requestDigest"`
	RunInputDigest   string                       `json:"runInputDigest"`
	HostID           string                       `json:"hostId"`
	HostFingerprint  string                       `json:"hostFingerprint"`
	Template         e2bbe.Template               `json:"template"`
	CreatedAt        time.Time                    `json:"createdAt"`
	AllocatedAt      time.Time                    `json:"allocatedAt,omitempty"`
	Admitted         bool                         `json:"admitted"`
	Retired          bool                         `json:"retired"`
	NoProviderEffect bool                         `json:"noProviderEffect"`
}
type cloudPlacementJournal struct {
	Version   int                       `json:"version"`
	Scope     string                    `json:"scope"`
	CreatedAt time.Time                 `json:"createdAt"`
	Rows      map[string]cloudPlacement `json:"rows"`
}
type CloudTaskPool struct {
	mu          sync.Mutex
	config      CloudTaskPoolConfig
	hosts       map[string]CloudTaskHost
	store       *cloudPlacementStore
	probeCursor atomic.Uint64
}
type cloudHostSnapshot struct {
	HostID      string
	Records     []e2bbe.Record
	Settled     []e2bbe.SettledSpend
	Unavailable bool
}
type cloudPoolUsage struct {
	Active, ProjectActive                                  int
	MemoryMiB, CPUMillis, EstimateMicros, ActorMonthMicros int64
	Unknown                                                bool
}

func cloudPoolAllows(usage cloudPoolUsage, limits e2bbe.Limits, budget int64, seconds int) bool {
	return !usage.Unknown && budget > 0 && seconds > 0 && seconds <= limits.MaxTaskSeconds && usage.Active <= limits.MaxConcurrent && usage.ProjectActive <= limits.MaxPerProject && usage.MemoryMiB <= limits.MemoryMiB && usage.CPUMillis <= limits.CPUMillis && usage.EstimateMicros <= limits.EstimatedCostMicros && usage.ActorMonthMicros <= budget
}
func cloudPlacementDigest(value any) string {
	raw, _ := json.Marshal(value)
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}
func cloudHostID(value string) bool {
	if len(value) < 1 || len(value) > 64 {
		return false
	}
	for _, r := range value {
		if !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '-') {
			return false
		}
	}
	return value[0] != '-' && value[len(value)-1] != '-'
}
func cloudHostFingerprint(host CloudTaskHost) string {
	limits, template := host.Runtime.Runtime.Controller.PlacementProfile()
	return cloudPlacementDigest(struct {
		ID, Config, Owner string
		Template          e2bbe.Template
		Limits            e2bbe.Limits
		Cost              int64
		Seconds           int
		Egress            []string
	}{host.ID, host.ConfigDigest, host.Runtime.Runtime.Controller.OwnerID(), template, limits, host.Runtime.EstimatedCostMicros, host.Runtime.MaxSeconds, host.Runtime.ModelEgressDomains})
}
func (p *CloudTaskPool) catalogueDigest() string {
	type hostBinding struct {
		ID, Fingerprint string
		Draining        bool
	}
	rows := []hostBinding{}
	for _, host := range p.hosts {
		rows = append(rows, hostBinding{host.ID, cloudHostFingerprint(host), host.Draining})
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].ID < rows[j].ID })
	return cloudPlacementDigest(struct {
		Scope, Default string
		Limits         e2bbe.Limits
		Hosts          []hostBinding
	}{p.config.Scope, p.config.DefaultHostID, p.config.Limits, rows})
}
func NewCloudTaskPool(config CloudTaskPoolConfig, hosts []CloudTaskHost) (*CloudTaskPool, error) {
	l := config.Limits
	if !cloudHostID(config.Scope) || !cloudHostID(config.DefaultHostID) || len(hosts) < 1 || len(hosts) > 8 || l.MaxConcurrent < 1 || l.MaxConcurrent > 1024 || l.MaxPerProject < 1 || l.MaxPerProject > l.MaxConcurrent || l.MemoryMiB < 128 || l.MemoryMiB > 1<<40 || l.CPUMillis < 1000 || l.CPUMillis > 1<<40 || l.EstimatedCostMicros < 1 || l.EstimatedCostMicros > 1<<50 || l.MaxTaskSeconds < 1 || l.MaxTaskSeconds > 86400 {
		return nil, e2bbe.ErrUnsupported
	}
	result := &CloudTaskPool{config: config, hosts: map[string]CloudTaskHost{}}
	controllers := map[*e2bbe.Controller]bool{}
	physicalHosts := map[string]bool{}
	for _, host := range hosts {
		if !cloudHostID(host.ID) || result.hosts[host.ID].Runtime != nil || host.Runtime == nil || host.Runtime.Pool != nil || host.Runtime.validate() != nil || len(host.ConfigDigest) != 64 || len(host.HostKey) != 64 || len(host.Name) < 1 || len(host.Name) > 120 || strings.IndexFunc(host.Name, func(r rune) bool { return unicode.IsControl(r) || unicode.In(r, unicode.Cf) }) >= 0 {
			return nil, e2bbe.ErrUnsupported
		}
		if _, err := hex.DecodeString(host.ConfigDigest); err != nil {
			return nil, e2bbe.ErrUnsupported
		}
		if _, err := hex.DecodeString(host.HostKey); err != nil {
			return nil, e2bbe.ErrUnsupported
		}
		controller := host.Runtime.Runtime.Controller
		// Two entries for the same authenticated physical node would invent
		// capacity. Ownership may be shared by one legitimate workspace controller.
		if controllers[controller] || physicalHosts[host.HostKey] {
			return nil, e2bbe.ErrConflict
		}
		controllers[controller] = true
		physicalHosts[host.HostKey] = true
		result.hosts[host.ID] = host
	}
	if result.hosts[config.DefaultHostID].Runtime == nil {
		return nil, e2bbe.ErrUnsupported
	}
	store, err := openCloudPlacementStore(config.StateDir, config.Scope)
	if err != nil {
		return nil, err
	}
	result.store = store
	return result, nil
}
func (p *CloudTaskPool) Close() error {
	if p == nil {
		return nil
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.store.Close()
}
func (p *CloudTaskPool) snapshots() []cloudHostSnapshot {
	ids := make([]string, 0, len(p.hosts))
	for id := range p.hosts {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	out := make([]cloudHostSnapshot, 0, len(ids))
	for _, id := range ids {
		controller := p.hosts[id].Runtime.Runtime.Controller
		rows, err := controller.OwnershipSnapshot()
		_, settled := controller.SpendSnapshot()
		out = append(out, cloudHostSnapshot{HostID: id, Records: rows, Settled: settled, Unavailable: err != nil})
	}
	return out
}
func cloudOriginalKey(owner, operation string) string { return owner + "\x00" + operation }
func cloudBillApplies(user, team string, actor management.ExecutionActorRef) bool {
	return user == actor.UserID || actor.TeamID != "" && team == actor.TeamID
}

// All reservations are charged before Allocate, including missing/unreachable
// originals. Materialized records and their reservation are one allocation.
// Settled provider estimates remain estimates; absence never clears a bill.
func cloudAggregateUsage(rows map[string]cloudPlacement, snapshots []cloudHostSnapshot, actor management.ExecutionActorRef, now time.Time) cloudPoolUsage {
	var usage cloudPoolUsage
	bound := map[string]cloudPlacement{}
	seen := map[string]bool{}
	month := now.UTC().Format("2006-01")
	add := func(template e2bbe.Template, cost int64, active bool, project, user, team string, at time.Time, effect bool) {
		if active {
			usage.Active++
			if project == actor.ProjectID {
				usage.ProjectActive++
			}
			usage.MemoryMiB += template.MemoryMiB
			usage.CPUMillis += template.CPUMillis
		}
		if active || effect {
			if cost < 1 || cost > math.MaxInt64-usage.EstimateMicros {
				usage.Unknown = true
			} else {
				usage.EstimateMicros += cost
			}
		}
		if !effect || !active && at.UTC().Format("2006-01") != month {
			return
		}
		if user == "" || cost < 1 || cost > 1<<50 {
			usage.Unknown = true
			return
		}
		if cloudBillApplies(user, team, actor) {
			if usage.ActorMonthMicros > math.MaxInt64-cost {
				usage.Unknown = true
				return
			}
			usage.ActorMonthMicros += cost
		}
	}
	for _, row := range rows {
		key := cloudOriginalKey(row.Request.Identity.OwnerID, row.Request.Identity.OperationID)
		if _, duplicate := bound[key]; duplicate {
			usage.Unknown = true
		}
		bound[key] = row
		at := row.CreatedAt
		if !row.AllocatedAt.IsZero() {
			at = row.AllocatedAt
		}
		add(row.Template, row.Request.EstimatedCostMicros, row.Admitted && !row.Retired, row.Actor.ProjectID, row.Actor.UserID, row.Actor.TeamID, at, row.Admitted && !row.NoProviderEffect)
	}
	for _, snapshot := range snapshots {
		if snapshot.Unavailable {
			usage.Unknown = true
		}
		for _, record := range snapshot.Records {
			key := cloudOriginalKey(record.Request.Identity.OwnerID, record.Request.Identity.OperationID)
			if seen[key] {
				usage.Unknown = true
				continue
			}
			seen[key] = true
			if row, ok := bound[key]; ok {
				if !row.Admitted || row.HostID != snapshot.HostID || row.RequestDigest != record.RequestDigest || row.Retired && record.ResourcesHeld || row.NoProviderEffect && record.SandboxID != "" {
					usage.Unknown = true
				}
				continue
			}
			add(record.Template, record.Request.EstimatedCostMicros, record.ResourcesHeld, record.Request.Identity.ProjectID, record.Request.BillingUserID, record.Request.BillingTeamID, record.CreatedAt, record.SandboxID != "" || record.ResourcesHeld)
		}
		for _, settled := range snapshot.Settled {
			// The controller's public budget snapshot has the original operation and
			// user, but legacy tombstones lack a team. A pool-owned row supplies that
			// attribution exactly; an unrelated legacy row remains user-attributed.
			matched := false
			for key, row := range bound {
				if row.HostID == snapshot.HostID && row.Request.Identity.OperationID == settled.OperationID {
					matched = true
					if seen[key] || settled.RequestDigest != row.RequestDigest {
						usage.Unknown = true
					}
					seen[key] = true
					break
				}
			}
			if matched {
				continue
			}
			if actor.TeamID != "" && settled.SandboxCreated && settled.CreatedAt.UTC().Format("2006-01") == month {
				usage.Unknown = true
			} // legacy tombstone has no team attribution
			add(e2bbe.Template{}, settled.EstimatedCostMicros, false, "", settled.BillingUserID, "", settled.CreatedAt, settled.SandboxCreated)
		}
	}
	return usage
}
func (p *CloudTaskPool) refreshLocked(journal *cloudPlacementJournal, snapshots []cloudHostSnapshot) error {
	for _, snapshot := range snapshots {
		if snapshot.Unavailable {
			return e2bbe.ErrUnknown
		}
	}
	for id, row := range journal.Rows {
		for _, snapshot := range snapshots {
			if snapshot.HostID != row.HostID {
				continue
			}
			for _, record := range snapshot.Records {
				if record.Request.Identity.OperationID != id || record.Request.Identity.OwnerID != row.Request.Identity.OwnerID {
					continue
				}
				if !row.Admitted || record.RequestDigest != row.RequestDigest || record.Request != row.Request || record.Template != row.Template {
					return e2bbe.ErrConflict
				}
				if row.Retired && (record.State != e2bbe.Absent || record.ResourcesHeld) || row.NoProviderEffect && record.SandboxID != "" {
					return e2bbe.ErrUnknown
				}
				if !row.AllocatedAt.IsZero() && !row.AllocatedAt.Equal(record.CreatedAt) {
					return e2bbe.ErrConflict
				}
				row.AllocatedAt = record.CreatedAt
				if record.State == e2bbe.Absent && !record.ResourcesHeld {
					row.Retired = true
					row.NoProviderEffect = record.SandboxID == ""
				}
			}
			for _, settled := range snapshot.Settled {
				if settled.OperationID == id {
					if !row.Admitted || settled.RequestDigest != row.RequestDigest {
						return e2bbe.ErrConflict
					}
					row.Retired = true
					row.NoProviderEffect = !settled.SandboxCreated
					if row.AllocatedAt.IsZero() {
						row.AllocatedAt = settled.CreatedAt
					}
				}
			}
		}
		journal.Rows[id] = row
	}
	return nil
}
func (p *CloudTaskPool) binding(operation string) (cloudPlacement, bool, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	journal, err := p.store.read()
	if err != nil {
		return cloudPlacement{}, false, err
	}
	row, ok := journal.Rows[operation]
	return row, ok, nil
}

// Missing records are useful negative evidence only for original operations
// minted after this pool's durable cutover. Legacy missing ownership is held.
func (p *CloudTaskPool) neverMaterialized(projectID, runID, operationID, childID, createdAt string, retire bool) (bool, error) {
	created, err := time.Parse(time.RFC3339Nano, createdAt)
	if err != nil {
		return false, e2bbe.ErrUnknown
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	journal, err := p.store.read()
	if err != nil {
		return false, err
	}
	row, bound := journal.Rows[childID]
	if bound {
		identity := row.Request.Identity
		if identity.ProjectID != projectID || identity.RunID != runID || identity.AttemptID != operationID || identity.OperationID != childID {
			return false, e2bbe.ErrConflict
		}
		if row.Retired {
			return true, nil
		}
		if _, err = p.runtime(row); err != nil {
			return false, err
		}
	} else if created.Before(journal.CreatedAt) {
		return false, e2bbe.ErrUnknown
	}
	_, present, err := p.originalRuntime(projectID, runID, operationID, childID)
	if err != nil || present {
		return false, err
	}
	if retire && bound {
		row.Retired = true
		row.NoProviderEffect = true
		journal.Rows[childID] = row
		if err = p.store.write(journal); err != nil {
			return false, err
		}
	}
	return true, nil
}
func (p *CloudTaskPool) runtime(row cloudPlacement) (*CloudTaskRuntime, error) {
	host, ok := p.hosts[row.HostID]
	if !ok || cloudHostFingerprint(host) != row.HostFingerprint {
		return nil, e2bbe.ErrUnknown
	}
	return host.Runtime, nil
}
func (p *CloudTaskPool) candidates(allowed []string) []CloudTaskHost {
	if len(allowed) == 0 {
		allowed = []string{p.config.DefaultHostID}
	}
	hosts := []CloudTaskHost{}
	for _, id := range allowed {
		if host, ok := p.hosts[id]; ok && !host.Draining {
			hosts = append(hosts, host)
		}
	}
	pending := map[string]int{}
	p.mu.Lock()
	journal, readErr := p.store.read()
	if readErr == nil {
		snapshots := p.snapshots()
		materialized := map[string]bool{}
		for _, snapshot := range snapshots {
			for _, row := range snapshot.Records {
				materialized[cloudOriginalKey(row.Request.Identity.OwnerID, row.Request.Identity.OperationID)] = true
			}
		}
		for _, row := range journal.Rows {
			if row.Admitted && !row.Retired && !materialized[cloudOriginalKey(row.Request.Identity.OwnerID, row.Request.Identity.OperationID)] {
				pending[row.HostID]++
			}
		}
	}
	p.mu.Unlock()
	if readErr != nil {
		return nil
	}
	slots := func(host CloudTaskHost) int {
		limits, template, usage := host.Runtime.Runtime.Controller.ConfiguredUsage()
		return minimumCloudSlots(limits.MaxConcurrent-usage.Active-pending[host.ID], int((limits.MemoryMiB-usage.MemoryMiB)/template.MemoryMiB)-pending[host.ID], int((limits.CPUMillis-usage.CPUMillis)/template.CPUMillis)-pending[host.ID])
	}
	sort.Slice(hosts, func(i, j int) bool {
		a, b := slots(hosts[i]), slots(hosts[j])
		if a != b {
			return a > b
		}
		return hosts[i].ID < hosts[j].ID
	})
	if len(hosts) > 0 && slots(hosts[0]) > 0 {
		available := hosts[:0]
		for _, host := range hosts {
			if slots(host) > 0 {
				available = append(available, host)
			}
		}
		return available
	}
	return nil
}
func (p *CloudTaskPool) reserve(ctx context.Context, host CloudTaskHost, queueID string, actor management.ExecutionActorRef, runInputDigest string, request e2bbe.Request, budget int64) (cloudPlacement, error) {
	if ctx.Err() != nil || queueID == "" || actor.Local || actor.UserID == "" || actor.ProjectID != request.Identity.ProjectID || actor.UserID != request.BillingUserID || actor.TeamID != request.BillingTeamID || len(runInputDigest) != 64 || host.Runtime == nil {
		return cloudPlacement{}, management.ErrExecutionAuthority
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	journal, err := p.store.read()
	if err != nil {
		return cloudPlacement{}, err
	}
	old, exists := journal.Rows[request.Identity.OperationID]
	if exists {
		if old.QueueID != queueID || old.Actor != actor || old.Request != request || old.RunInputDigest != runInputDigest || old.HostID != host.ID || old.HostFingerprint != cloudHostFingerprint(host) {
			return old, e2bbe.ErrConflict
		}
		if old.Admitted {
			return old, nil
		}
	}
	snapshots := p.snapshots()
	if err = p.refreshLocked(&journal, snapshots); err != nil {
		return cloudPlacement{}, err
	}
	if !exists && len(journal.Rows) >= 8192 {
		return cloudPlacement{}, e2bbe.ErrCapacity
	}
	_, template := host.Runtime.Runtime.Controller.PlacementProfile()
	row := cloudPlacement{QueueID: queueID, Actor: actor, Request: request, RequestDigest: cloudPlacementDigest(request), RunInputDigest: runInputDigest, HostID: host.ID, HostFingerprint: cloudHostFingerprint(host), Template: template, CreatedAt: time.Now().UTC()}
	if exists {
		row = old
	}
	materialized := false
	// Existing controller originals are adopted only as the exact same attempt,
	// never as a provider-created VM discovered by labels or a new run request.
	for _, snapshot := range snapshots {
		for _, record := range snapshot.Records {
			if record.Request.Identity.OperationID == request.Identity.OperationID {
				if snapshot.HostID != host.ID || record.Request != request {
					return cloudPlacement{}, e2bbe.ErrConflict
				}
				materialized = true
				row.AllocatedAt = record.CreatedAt
				row.Retired = record.State == e2bbe.Absent && !record.ResourcesHeld
				row.NoProviderEffect = row.Retired && record.SandboxID == ""
			}
		}
	}
	row.Admitted = true
	journal.Rows[request.Identity.OperationID] = row
	usage := cloudAggregateUsage(journal.Rows, snapshots, actor, time.Now().UTC())
	l := p.config.Limits
	if !cloudPoolAllows(usage, l, budget, request.MaxSeconds) {
		row.Admitted = materialized
		journal.Rows[request.Identity.OperationID] = row
		if err = p.store.write(journal); err != nil {
			return row, err
		}
		return row, e2bbe.ErrCapacity
	}
	if err = p.store.write(journal); err != nil {
		return cloudPlacement{}, err
	}
	return row, nil
}

// Called by every configured controller immediately before each Allocate
// effect, not just the UI dispatcher. No unreserved default/direct path can
// bypass aggregate slots or the freshly supplied current account/team budget.
func (p *CloudTaskPool) AuthorizeAllocation(ctx context.Context, hostID string, request e2bbe.Request, budget int64) error {
	if p == nil || ctx.Err() != nil {
		return e2bbe.ErrUnknown
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	journal, err := p.store.read()
	if err != nil {
		return err
	}
	row, ok := journal.Rows[request.Identity.OperationID]
	host, configured := p.hosts[hostID]
	if !ok || !configured || !row.Admitted || row.Retired || row.HostID != hostID || row.Request != request || row.RequestDigest != cloudPlacementDigest(request) || row.HostFingerprint != cloudHostFingerprint(host) || row.Actor.UserID != request.BillingUserID || row.Actor.TeamID != request.BillingTeamID {
		return management.ErrExecutionAuthority
	}
	snapshots := p.snapshots()
	if err = p.refreshLocked(&journal, snapshots); err != nil {
		return err
	}
	usage := cloudAggregateUsage(journal.Rows, snapshots, row.Actor, time.Now().UTC())
	l := p.config.Limits
	if !cloudPoolAllows(usage, l, budget, request.MaxSeconds) {
		return e2bbe.ErrCapacity
	}
	return p.store.write(journal)
}

func (a *API) cloudAttemptRuntime(projectID, runID, operationID, childID string) (*CloudTaskRuntime, error) {
	cloud := a.deps.CloudTasks
	if cloud == nil {
		return nil, e2bbe.ErrUnsupported
	}
	if cloud.Pool == nil {
		return cloud, nil
	}
	row, found, err := cloud.Pool.binding(childID)
	if err != nil {
		return nil, err
	}
	if !found {
		selected, present, err := cloud.Pool.originalRuntime(projectID, runID, operationID, childID)
		if err != nil {
			return nil, err
		}
		if !present {
			return nil, e2bbe.ErrUnknown
		}
		return selected, nil
	}
	identity := row.Request.Identity
	if identity.ProjectID != projectID || identity.RunID != runID || identity.AttemptID != operationID || identity.OperationID != childID {
		return nil, e2bbe.ErrConflict
	}
	return cloud.Pool.runtime(row)
}

// Only authoritative configured controller journals are examined. A missing
// retained placement or unreadable original is never inferred from a network
// timeout, provider labels, guest output or an untrusted browser argument.
func (p *CloudTaskPool) originalRuntime(projectID, runID, operationID, childID string) (*CloudTaskRuntime, bool, error) {
	var selected *CloudTaskRuntime
	for _, host := range p.hosts {
		records, err := host.Runtime.Runtime.Controller.OwnershipSnapshot()
		if err != nil {
			return nil, false, e2bbe.ErrUnknown
		}
		for _, record := range records {
			identity := record.Request.Identity
			if identity.OperationID != childID {
				continue
			}
			if identity.ProjectID != projectID || identity.RunID != runID || identity.AttemptID != operationID || identity.OwnerID != host.Runtime.Runtime.Controller.OwnerID() {
				return nil, false, e2bbe.ErrConflict
			}
			if selected != nil {
				return nil, false, e2bbe.ErrConflict
			}
			selected = host.Runtime
		}
		_, settled := host.Runtime.Runtime.Controller.SpendSnapshot()
		for _, record := range settled {
			if record.OperationID == childID {
				return nil, false, e2bbe.ErrUnknown
			}
		}
	}
	return selected, selected != nil, nil
}
func (a *API) chooseCloudRuntime(ctx context.Context, req product.SwarmDispatchRequest, run product.ProjectRun, op product.ProjectRunOperation) (*CloudTaskRuntime, *CloudTaskHost, error) {
	cloud := a.deps.CloudTasks
	if cloud == nil {
		return nil, nil, e2bbe.ErrUnsupported
	}
	if cloud.Pool == nil {
		if len(run.Input.Route.CloudHostIDs) > 0 {
			return nil, nil, e2bbe.ErrUnsupported
		}
		return cloud, nil, nil
	}
	row, found, err := cloud.Pool.binding(op.ChildID)
	if err != nil {
		return nil, nil, err
	}
	if found {
		if row.QueueID != req.QueueID || row.Actor != req.Actor || row.RunInputDigest != cloudPlacementDigest(run.Input) {
			return nil, nil, e2bbe.ErrConflict
		}
		selected, err := cloud.Pool.runtime(row)
		host := cloud.Pool.hosts[row.HostID]
		return selected, &host, err
	}
	if len(run.Input.Route.CloudHostIDs) > 0 && run.Input.Route.CloudPoolDigest != cloud.Pool.catalogueDigest() {
		return nil, nil, e2bbe.ErrConflict
	}
	original, present, err := cloud.Pool.originalRuntime(req.ProjectID, req.RunID, op.ID, op.ChildID)
	if err != nil {
		return nil, nil, err
	}
	if present {
		for _, host := range cloud.Pool.hosts {
			if host.Runtime == original {
				return original, &host, nil
			}
		}
	}
	candidates := cloud.Pool.candidates(run.Input.Route.CloudHostIDs)
	if len(candidates) == 0 {
		return nil, nil, e2bbe.ErrCapacity
	}
	// These are read-only prerequisite observations, not allocation attempts.
	// Rotate bounded probes so a slow configured host cannot starve other hosts.
	// Once reserve persists an original, this branch can never move it.
	bounded, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	current := func() bool {
		authority, err := a.authorizeExecution(bounded, req.Actor, run.Input.Route.Harness, true)
		return err == nil && authority.Policy.AllowCloudProvisioning && authority.Policy.MonthlyBudgetUSD > 0 && product.OrchestrationAuthorityMatches(run, authority)
	}
	if !current() {
		return nil, nil, management.ErrExecutionAuthority
	}
	connectionID := run.Input.Route.Connection.ID
	_, modelEnvironment, _, connectionErr := a.resolveSessionAgent(run.Input.Route.Harness, &connectionID, registry.Session{}, run.Input.Route.Model)
	if connectionErr != nil {
		return nil, nil, e2bbe.ErrUnsupported
	}
	start := int((cloud.Pool.probeCursor.Add(1) - 1) % uint64(len(candidates)))
	for i := 0; i < len(candidates); i++ {
		host := candidates[(start+i)%len(candidates)]
		if bounded.Err() != nil {
			return nil, nil, e2bbe.ErrCapacity
		}
		if !host.Runtime.allowsModelEndpoint(run.Input.Route.Harness, modelEnvironment) {
			continue
		}
		controller := host.Runtime.Runtime.Controller
		if !current() {
			return nil, nil, management.ErrExecutionAuthority
		}
		if controller.VerifyTemplateBuild(bounded) != nil {
			continue
		}
		if !current() {
			return nil, nil, management.ErrExecutionAuthority
		}
		if controller.VerifyNodeCapacity(bounded) != nil {
			continue
		}
		if !current() {
			return nil, nil, management.ErrExecutionAuthority
		}
		return host.Runtime, &host, nil
	}
	return nil, nil, e2bbe.ErrCapacity
}
