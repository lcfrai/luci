package e2bbe

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"maps"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"sort"
	"sync"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/statelease"
)

type journal struct {
	Version int               `json:"version"`
	OwnerID string            `json:"ownerId"`
	Scope   string            `json:"scope"`
	Records map[string]Record `json:"records"`
	// Settled retains exact no-replay and estimated-spend evidence after an
	// absent VM has been joined to the authoritative product run. Its full
	// record remains in the prior immutable off-host journal checkpoint.
	Settled map[string]settledAttempt `json:"settled,omitempty"`
}

type settledAttempt struct {
	RequestDigest       string    `json:"requestDigest"`
	AttemptDigest       string    `json:"attemptDigest"`
	RecordDigest        string    `json:"recordDigest"`
	JournalDigest       string    `json:"journalDigest"`
	EstimatedCostMicros int64     `json:"estimatedCostMicros"`
	BillingUserID       string    `json:"billingUserId,omitempty"`
	CreatedAt           time.Time `json:"createdAt"`
	SandboxCreated      bool      `json:"sandboxCreated"`
}

// SettledSpend is budget evidence only. It cannot be used as an allocation,
// process observation, provider billing settlement or source-review receipt.
type SettledSpend struct {
	OperationID         string
	BillingUserID       string
	CreatedAt           time.Time
	EstimatedCostMicros int64
	SandboxCreated      bool
}

const maxSettledAttempts = 8192
const settledCompactionWatermark = 3 << 20
const settledCompactionTarget = 2 << 20

func attemptDigest(i Identity) string {
	return digest([3]string{i.ProjectID, i.TaskID, i.AttemptID})
}

type Controller struct {
	mu               sync.Mutex
	config           Config
	client           *providerClient
	journal          journal
	root             *os.Root
	lease            *os.File
	busy             map[string]bool
	nativeObservers  map[string]context.CancelFunc
	closed           bool
	stopped          chan struct{}
	ctx              context.Context
	cancel           context.CancelFunc
	workers          sync.WaitGroup
	checkpoint       CheckpointWriter
	checkpointDigest string
	// A restored journal is ownership evidence, not proof that its VMs still
	// match the provider. New Create effects wait for exact readback of every
	// retained attempt. This set is process-local; the journal remains durable.
	restoredPending map[string]bool
	restoredAfter   string
}

func New(c Config) (*Controller, error) { return newController(c, nil, context.Background()) }

// OwnerID is the configured controller ownership scope, never a provider
// credential or a task-selected identity.
func (c *Controller) OwnerID() string { return c.config.OwnerID }

func (c *Controller) Region() string { return c.config.Deployment.Region }

func (c *Controller) Provider() string { return c.config.Deployment.Provider }

func (c *Controller) EgressDomains() []string {
	return append([]string{}, c.config.EgressAllow...)
}

func supportedEmbedInstanceType(instance string) bool {
	switch instance {
	case "m8i.xlarge", "m8i.2xlarge", "m8i.4xlarge", "m8i.8xlarge":
		return true
	}
	return false
}

// Capacity is an informational snapshot. Allocate rechecks the authoritative
// journal and current actor/provider prerequisites under its own lock.
func (c *Controller) Capacity(projectID string) (Limits, Template, Usage, int, []Record, []SettledSpend) {
	c.mu.Lock()
	defer c.mu.Unlock()
	rows := make([]Record, 0, len(c.journal.Records))
	projectActive := 0
	for _, row := range c.journal.Records {
		rows = append(rows, row)
		if row.ResourcesHeld && row.Request.Identity.ProjectID == projectID {
			projectActive++
		}
	}
	usage, _ := c.usageLocked()
	return c.config.Limits, c.config.Template, usage, projectActive, rows, c.settledSpendLocked()
}

func (c *Controller) settledSpendLocked() []SettledSpend {
	out := make([]SettledSpend, 0, len(c.journal.Settled))
	for op, row := range c.journal.Settled {
		out = append(out, SettledSpend{OperationID: op, BillingUserID: row.BillingUserID, CreatedAt: row.CreatedAt, EstimatedCostMicros: row.EstimatedCostMicros, SandboxCreated: row.SandboxCreated})
	}
	return out
}

// SpendSnapshot reads live and compacted estimates under one journal lock.
func (c *Controller) SpendSnapshot() ([]Record, []SettledSpend) {
	c.mu.Lock()
	defer c.mu.Unlock()
	rows := make([]Record, 0, len(c.journal.Records))
	for _, row := range c.journal.Records {
		rows = append(rows, row)
	}
	return rows, c.settledSpendLocked()
}

func egressDigest(domains []string) string {
	ordered := append([]string{}, domains...)
	slices.Sort(ordered)
	return digest(ordered)
}

func newController(c Config, restored []byte, restoreContext context.Context) (*Controller, error) {
	if !ValidEgressDomains(c.EgressAllow) {
		return nil, ErrUnsupported
	}
	c.EgressAllow = append([]string{}, c.EgressAllow...)
	if !validID(c.OwnerID) || !validDeployment(c) || c.Checkpoint == nil || c.Authorize == nil {
		return nil, ErrUnsupported
	}
	if c.Deployment.Provider == "azure" && !digestPattern.MatchString(c.GatewayReceiptDigest) ||
		c.Deployment.Provider != "azure" && c.GatewayReceiptDigest != "" {
		return nil, ErrUnsupported
	}
	if !validID(c.Template.ID) || c.Template.BuildID != "" && !validID(c.Template.BuildID) || !digestPattern.MatchString(c.Template.BuildDigest) || c.Template.MemoryMiB < 128 || c.Template.MemoryMiB > 32768 || c.Template.CPUMillis < 1000 || c.Template.CPUMillis > 16000 || c.Template.CPUMillis%1000 != 0 {
		return nil, ErrUnsupported
	}
	l := c.Limits
	if l.MaxConcurrent < 1 || l.MaxConcurrent > maxRecords || l.MaxPerProject < 1 || l.MaxPerProject > l.MaxConcurrent || l.MemoryMiB < c.Template.MemoryMiB || l.MemoryMiB > 1<<40 || l.CPUMillis < c.Template.CPUMillis || l.CPUMillis > 1<<40 || l.EstimatedCostMicros <= 0 || l.EstimatedCostMicros > 1<<50 || l.MaxTaskSeconds < 1 || l.MaxTaskSeconds > 86400 {
		return nil, ErrUnsupported
	}
	client, e := newClient(c)
	if e != nil {
		return nil, e
	}
	lease, e := statelease.Acquire(c.StateDir, "e2b-embed-v1")
	if e != nil {
		return nil, e
	}
	root, e := os.OpenRoot(c.StateDir)
	if e != nil {
		lease.Close()
		return nil, e
	}
	ctx, cancel := context.WithCancel(context.Background())
	out := &Controller{config: c, client: client, root: root, lease: lease, busy: map[string]bool{}, nativeObservers: map[string]context.CancelFunc{}, restoredPending: map[string]bool{}, ctx: ctx, cancel: cancel, stopped: make(chan struct{}), journal: journal{Version: 1, OwnerID: c.OwnerID, Scope: c.Deployment.Scope, Records: map[string]Record{}}}
	fail := func(err error) (*Controller, error) {
		root.Close()
		lease.Close()
		cancel()
		if out.checkpoint != nil {
			_ = out.checkpoint.Close()
		}
		return nil, err
	}
	if restored != nil {
		if _, e = root.Lstat("attempts.json"); !errors.Is(e, os.ErrNotExist) {
			return fail(ErrConflict)
		}
		out.journal, e = validateJournal(restored, c.OwnerID, c.Deployment.Scope)
		if e != nil {
			return fail(e)
		}
	} else if e = out.load(); e != nil {
		return fail(e)
	}
	// Every controller open must repeat exact readback, including a restart
	// after a previous restore was interrupted. The durable journal reconstructs
	// this admission fence; a process-local "done" flag cannot do so.
	for op, row := range out.journal.Records {
		if row.State != Absent {
			out.restoredPending[op] = true
		}
	}
	_, fileErr := root.Lstat("attempts.json")
	body, _ := json.Marshal(out.journal)
	out.checkpoint, e = c.Checkpoint.Claim(restoreContext, CheckpointClaim{OwnerID: c.OwnerID, Scope: c.Deployment.Scope, JournalDigest: bytesDigest(body), HasJournal: restored != nil || fileErr == nil})
	if e != nil {
		return fail(e)
	}
	if out.checkpoint == nil {
		return fail(ErrUnsupported)
	}
	out.checkpointDigest = bytesDigest(body)

	if restored != nil {
		if e = out.persist(restoreContext); e != nil {
			return fail(e)
		}
	}

	return out, nil
}
func (c *Controller) load() error {
	info, e := c.root.Lstat("attempts.json")
	if errors.Is(e, os.ErrNotExist) {
		return nil
	}
	if e != nil {
		return e
	}
	if !info.Mode().IsRegular() || info.Size() > maxResponse || (runtime.GOOS != "windows" && info.Mode().Perm()&0077 != 0) {
		return ErrConflict
	}
	f, e := c.root.Open("attempts.json")
	if e != nil {
		return e
	}
	defer f.Close()
	b, e := io.ReadAll(io.LimitReader(f, maxResponse+1))
	if e != nil || len(b) > maxResponse {
		return ErrConflict
	}
	j, e := validateJournal(b, c.config.OwnerID, c.config.Deployment.Scope)
	if e != nil {
		return e
	}
	c.journal = j
	return nil
}
func validateJournal(b []byte, owner, scope string) (journal, error) {
	var j journal
	decoder := json.NewDecoder(bytes.NewReader(b))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&j) != nil || decoder.Decode(new(any)) != io.EOF || j.Version != 1 || j.OwnerID != owner || j.Scope != scope || j.Records == nil || len(j.Records) > maxRecords || len(j.Settled) > maxSettledAttempts {
		return journal{}, ErrConflict
	}
	seenVMs := map[string]bool{}
	seenAttempts := map[string]bool{}
	for op, settled := range j.Settled {
		if !validID(op) || !digestPattern.MatchString(settled.RequestDigest) || !digestPattern.MatchString(settled.AttemptDigest) || !digestPattern.MatchString(settled.RecordDigest) || !digestPattern.MatchString(settled.JournalDigest) || settled.EstimatedCostMicros < 1 || settled.EstimatedCostMicros > 1<<50 || settled.CreatedAt.IsZero() || settled.BillingUserID != "" && !validID(settled.BillingUserID) {
			return journal{}, ErrConflict
		}
		if seenAttempts[settled.AttemptDigest] {
			return journal{}, ErrConflict
		}
		seenAttempts[settled.AttemptDigest] = true
	}
	for op, r := range j.Records {
		if op != r.Request.Identity.OperationID || j.Settled[op].RequestDigest != "" || !validRequest(r.Request) || r.Request.Identity.OwnerID != j.OwnerID || r.RequestDigest != digest(r.Request) || r.Deployment.Scope != j.Scope || r.Deployment.RuntimeRevision != RuntimeRevision || r.Revision < 1 || r.CreatedAt.IsZero() || r.UpdatedAt.Before(r.CreatedAt) || r.ProviderBillingSettled {
			return journal{}, ErrConflict
		}
		if r.EgressDigest != "" && !digestPattern.MatchString(r.EgressDigest) {
			return journal{}, ErrConflict
		}
		if r.Template.MemoryMiB < 128 || r.Template.MemoryMiB > 32768 || r.Template.CPUMillis < 1000 || r.Template.CPUMillis > 16000 || !validID(r.Template.ID) || r.Template.BuildID != "" && !validID(r.Template.BuildID) || !digestPattern.MatchString(r.Template.BuildDigest) {
			return journal{}, ErrConflict
		}
		if r.State != Creating && r.State != Allocated && r.State != Unknown && r.State != Cancelling && r.State != Absent {
			return journal{}, ErrConflict
		}
		if r.ResourcesHeld != (r.State != Absent) || (r.SandboxID != "" && !validID(r.SandboxID)) || (r.State == Allocated && r.SandboxID == "") {
			return journal{}, ErrConflict
		}
		if r.LaunchRequested != (r.LaunchDigest != "") || r.LaunchRequested && (!digestPattern.MatchString(r.LaunchDigest) || r.SandboxID == "") || r.ProcessPID != 0 && !r.LaunchRequested {
			return journal{}, ErrConflict
		}
		if r.InputUploadRequested && r.SandboxID == "" {
			return journal{}, ErrConflict
		}
		if r.LaunchKernel != (CaptureIdentity{}) && ValidateCaptureIdentity(r.LaunchKernel, r) != nil {
			return journal{}, ErrConflict
		}
		if r.NativeExit != (NativeExitReceipt{}) && (ValidateCaptureIdentity(r.LaunchKernel, r) != nil || ValidateNativeExitReceipt(r.NativeExit, r) != nil) {
			return journal{}, ErrConflict
		}
		if r.CaptureRequested && ValidateNativeExitReceipt(r.NativeExit, r) != nil || r.Capture != (CaptureReceipt{}) && (!r.CaptureRequested || ValidateCaptureReceipt(r.Capture, r.LaunchKernel, r) != nil) {
			return journal{}, ErrConflict
		}
		if r.ProductOutcome != "" && (r.State != Absent || r.ResourcesHeld || !validProductOutcome(r, r.ProductOutcome)) {
			return journal{}, ErrConflict
		}
		key := attemptDigest(r.Request.Identity)
		if seenAttempts[key] || r.SandboxID != "" && seenVMs[r.SandboxID] {
			return journal{}, ErrConflict
		}
		seenAttempts[key] = true
		if r.SandboxID != "" {
			seenVMs[r.SandboxID] = true
		}
	}
	return j, nil
}

// Caller holds mu. The private local journal survives a process crash; the
// mandatory hash-acknowledged external checkpoint is the host-loss boundary.
func (c *Controller) persist(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	b, e := json.Marshal(c.journal)
	if e != nil || len(b) > maxResponse {
		return ErrCapacity
	}
	name := ".attempts-next"
	if existing, e := c.root.Lstat(name); e == nil {
		if !existing.Mode().IsRegular() {
			return ErrConflict
		}
		if e = c.root.Remove(name); e != nil {
			return e
		}
	} else if !errors.Is(e, os.ErrNotExist) {
		return e
	}
	f, e := c.root.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if e != nil {
		return e
	}
	_, e = f.Write(b)
	if e == nil {
		e = f.Sync()
	}
	closeErr := f.Close()
	if e == nil {
		e = closeErr
	}
	if e != nil {
		return e
	}
	if e = c.root.Rename(name, "attempts.json"); e != nil {
		return e
	}
	// Sync the containing directory where supported. Windows directory handles
	// do not support Sync; statelease protects the private single-writer root.
	dir, dirErr := os.Open(filepath.Clean(c.config.StateDir))
	if dirErr == nil {
		dirErr = dir.Sync()
		_ = dir.Close()
	}
	if dirErr != nil && runtime.GOOS != "windows" {
		return ErrUnknown
	}
	ack, e := c.checkpoint.Commit(ctx, bytesDigest(b), append([]byte(nil), b...))
	if e != nil || ack != bytesDigest(b) {
		return ErrUnknown
	}
	c.checkpointDigest = ack
	return nil
}

// compactSettledLocked moves only absent, product-joined records into bounded
// no-replay tombstones. The prior complete journal must already be confirmed
// in the off-host checkpoint; no provider resource or billing proof is lost.
// Caller holds mu and refuses allocation if publication is unconfirmed.
func (c *Controller) compactSettledLocked(ctx context.Context, capacity int) error {
	prior, err := json.Marshal(c.journal)
	if err != nil {
		return ErrCapacity
	}
	if len(c.journal.Records) < capacity && len(prior) < settledCompactionWatermark {
		return nil
	}
	if bytesDigest(prior) != c.checkpointDigest {
		return ErrUnknown
	}
	if len(c.journal.Settled) >= maxSettledAttempts {
		return ErrCapacity
	}
	originalRecords := maps.Clone(c.journal.Records)
	originalSettled := maps.Clone(c.journal.Settled)
	rollback := func() {
		c.journal.Records = originalRecords
		c.journal.Settled = originalSettled
	}
	type candidate struct {
		op     string
		record Record
	}
	rows := make([]candidate, 0)
	for op, r := range c.journal.Records {
		if r.State == Absent && !r.ResourcesHeld && r.ProductOutcome != "" && !c.busy[op] {
			rows = append(rows, candidate{op, r})
		}
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].record.UpdatedAt.Equal(rows[j].record.UpdatedAt) {
			return rows[i].op < rows[j].op
		}
		return rows[i].record.UpdatedAt.Before(rows[j].record.UpdatedAt)
	})
	if c.journal.Settled == nil {
		c.journal.Settled = map[string]settledAttempt{}
	}
	changed := false
	for _, row := range rows {
		if len(c.journal.Settled) >= maxSettledAttempts {
			break
		}
		r := row.record
		c.journal.Settled[row.op] = settledAttempt{RequestDigest: r.RequestDigest, AttemptDigest: attemptDigest(r.Request.Identity), RecordDigest: digest(r), JournalDigest: c.checkpointDigest, EstimatedCostMicros: r.Request.EstimatedCostMicros, BillingUserID: r.Request.BillingUserID, CreatedAt: r.CreatedAt, SandboxCreated: r.SandboxID != ""}
		delete(c.journal.Records, row.op)
		changed = true
		if len(c.journal.Records) <= capacity/2 {
			current, marshalErr := json.Marshal(c.journal)
			if marshalErr != nil {
				rollback()
				return ErrCapacity
			}
			if len(current) <= settledCompactionTarget {
				break
			}
		}
	}
	current, err := json.Marshal(c.journal)
	if err != nil || !changed || len(c.journal.Records) >= capacity || len(current) >= settledCompactionWatermark {
		rollback()
		return ErrCapacity
	}
	return c.persist(ctx)
}
func (c *Controller) usageLocked() (Usage, error) {
	var u Usage
	for _, settled := range c.journal.Settled {
		u.EstimatedCostMicros = addEstimatedCost(u.EstimatedCostMicros, settled.EstimatedCostMicros)
	}
	for _, r := range c.journal.Records {
		// Retain estimated spend across VM spin-down. Absence proves neither
		// refunded model spend nor settled provider/storage billing.
		u.EstimatedCostMicros = addEstimatedCost(u.EstimatedCostMicros, r.Request.EstimatedCostMicros)
		if r.ResourcesHeld {
			u.Active++
			u.MemoryMiB += r.Template.MemoryMiB
			u.CPUMillis += r.Template.CPUMillis

		}
	}
	return u, nil
}

func addEstimatedCost(total, cost int64) int64 {
	if cost > math.MaxInt64-total {
		return math.MaxInt64 // A saturated estimate must refuse further admission.
	}
	return total + cost
}
func (c *Controller) Snapshot() ([]Record, Usage) {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]Record, 0, len(c.journal.Records))
	for _, r := range c.journal.Records {
		out = append(out, r)
	}
	u, _ := c.usageLocked()
	return out, u
}

// RestoredPending is a read of the process-local admission fence. It never
// samples the provider or changes durable ownership.
func (c *Controller) RestoredPending(op string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.restoredPending[op]
}
func (c *Controller) begin(ctx context.Context, op string) (context.Context, func(), error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return nil, nil, ErrClosed
	}
	if c.busy[op] {
		return nil, nil, ErrConflict
	}
	if ctx.Err() != nil {
		return nil, nil, ctx.Err()
	}
	c.busy[op] = true
	c.workers.Add(1)
	child, cancel := context.WithCancel(ctx)
	stop := context.AfterFunc(c.ctx, cancel)
	done := func() {
		stop()
		cancel()
		c.mu.Lock()
		delete(c.busy, op)
		c.mu.Unlock()
		c.workers.Done()
	}
	return child, done, nil
}
func (c *Controller) update(ctx context.Context, r Record) (Record, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	prior, ok := c.journal.Records[r.Request.Identity.OperationID]
	if !ok || prior.Revision != r.Revision || prior.RequestDigest != r.RequestDigest {
		return prior, ErrConflict
	}
	r.Revision++
	r.UpdatedAt = time.Now().UTC()
	c.journal.Records[r.Request.Identity.OperationID] = r
	if e := c.persist(ctx); e != nil {
		return r, e
	}
	if r.State == Absent {
		delete(c.restoredPending, r.Request.Identity.OperationID)
	}
	return r, nil
}
func (c *Controller) Allocate(ctx context.Context, req Request) (Record, error) {
	if !validRequest(req) || req.Identity.OwnerID != c.config.OwnerID {
		return Record{}, ErrConflict
	}
	ctx, done, e := c.begin(ctx, req.Identity.OperationID)
	if e != nil {
		return Record{}, e
	}
	defer done()
	if e = c.config.Authorize(ctx, req, "allocate"); e != nil {
		return Record{}, e
	}
	c.mu.Lock()
	if old, ok := c.journal.Records[req.Identity.OperationID]; ok {
		pending := c.restoredPending[req.Identity.OperationID]
		c.mu.Unlock()
		if old.RequestDigest != digest(req) {
			return old, ErrConflict
		}
		if pending {
			return old, ErrUnknown
		}
		if old.State == Allocated || old.State == Absent {
			return old, nil
		}
		return old, ErrUnknown
	}
	if old, ok := c.journal.Settled[req.Identity.OperationID]; ok {
		c.mu.Unlock()
		if old.RequestDigest != digest(req) {
			return Record{}, ErrConflict
		}
		return Record{}, ErrUnknown // The owned operation can never Create again.
	}
	if len(c.restoredPending) != 0 {
		c.mu.Unlock()
		return Record{}, ErrUnknown // Restore must inspect exact prior ownership first.
	}
	if e = c.compactSettledLocked(ctx, maxRecords); e != nil {
		c.mu.Unlock()
		return Record{}, e
	}
	u, _ := c.usageLocked()
	perProject := 0
	key := attemptDigest(req.Identity)
	for _, settled := range c.journal.Settled {
		if settled.AttemptDigest == key {
			c.mu.Unlock()
			return Record{}, ErrConflict
		}
	}
	for _, r := range c.journal.Records {
		if attemptDigest(r.Request.Identity) == key {
			c.mu.Unlock()
			return Record{}, ErrConflict
		}
		if r.ResourcesHeld && r.Deployment.HostGeneration != c.config.Deployment.HostGeneration {
			c.mu.Unlock()
			return Record{}, ErrUnknown
		}
		if r.ResourcesHeld && r.Request.Identity.ProjectID == req.Identity.ProjectID {
			perProject++
		}
	}
	l, t := c.config.Limits, c.config.Template
	if len(c.journal.Records) >= maxRecords || u.Active >= l.MaxConcurrent || perProject >= l.MaxPerProject || u.MemoryMiB+t.MemoryMiB > l.MemoryMiB || u.CPUMillis+t.CPUMillis > l.CPUMillis || req.EstimatedCostMicros > l.EstimatedCostMicros-u.EstimatedCostMicros || req.MaxSeconds > l.MaxTaskSeconds {
		c.mu.Unlock()
		return Record{}, ErrCapacity
	}
	now := time.Now().UTC()
	r := Record{Request: req, RequestDigest: digest(req), EgressDigest: egressDigest(c.config.EgressAllow), Deployment: c.config.Deployment, Template: t, State: Creating, Revision: 1, ResourcesHeld: true, CreatedAt: now, UpdatedAt: now}
	c.journal.Records[req.Identity.OperationID] = r
	e = c.persist(ctx)
	c.mu.Unlock()
	if e != nil {
		return r, e
	}
	// Fresh authority after possible checkpoint latency and immediately before
	// the sole Create effect. A refusal proves no Create was issued here.
	if e = c.config.Authorize(ctx, req, "allocate"); e != nil {
		r.State = Absent
		r.ResourcesHeld = false
		after, saveErr := c.update(context.WithoutCancel(ctx), r)
		if saveErr != nil {
			return after, saveErr
		}
		return after, e
	}
	if ctx.Err() != nil {
		return r, ErrUnknown
	}
	r, e = c.record(req.Identity.OperationID)
	if e != nil {
		return r, e
	}
	if r.CancelRequested {
		r.State, r.ResourcesHeld = Absent, false
		return c.update(context.WithoutCancel(ctx), r)
	}
	created, createErr := c.client.create(ctx, r)
	// A concurrent cancellation may have persisted intent while Create was
	// in flight. Preserve it and settle this same allocation, never replay it.
	r, e = c.record(req.Identity.OperationID)
	if e != nil {
		return r, e
	}
	if createErr != nil {
		if !r.CancelRequested {
			r.State = Unknown
		}
		after, saveErr := c.update(context.WithoutCancel(ctx), r)
		if saveErr != nil {
			return after, saveErr
		}
		return after, ErrUnknown
	}
	r.SandboxID = created.ID
	if !r.CancelRequested {
		r.State = Unknown
	}
	c.mu.Lock()
	for op, old := range c.journal.Records {
		if op != req.Identity.OperationID && old.SandboxID == created.ID {
			c.mu.Unlock()
			return r, ErrConflict
		}
	}
	c.mu.Unlock()
	r, e = c.update(context.WithoutCancel(ctx), r)
	if e != nil {
		return r, e
	}
	// Persist tokens separately from the checkpoint/public allocation journal.
	// A lost or unsupported grant does not justify another Create/Connect.
	if created.EnvdAccessToken != "" || created.TrafficAccessToken != "" {
		if e = c.saveGrant(r, created); e != nil {
			// Create was acknowledged and the exact VM is charged. A missing
			// private grant cannot turn that effect into a no-op refusal.
			return r, ErrUnknown
		}
	}
	return c.observe(ctx, r, r.CancelRequested)
}
func (c *Controller) record(op string) (Record, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	r, ok := c.journal.Records[op]
	if !ok {
		return r, ErrConflict
	}
	return r, nil
}
func (c *Controller) observe(ctx context.Context, r Record, allowDelete bool) (Record, error) {
	if r.Deployment != c.config.Deployment {
		return r, ErrUnknown
	}
	if r.SandboxID == "" {
		found, e := c.client.find(ctx, r)
		if e != nil {
			return r, e
		}
		c.mu.Lock()
		for op, old := range c.journal.Records {
			if op != r.Request.Identity.OperationID && old.SandboxID == found.ID {
				c.mu.Unlock()
				return r, ErrConflict
			}
		}
		c.mu.Unlock()
		r.SandboxID = found.ID
		var saveErr error
		r, saveErr = c.update(ctx, r)
		if saveErr != nil {
			return r, saveErr
		}
	}
	observed, absent, e := c.client.inspect(ctx, r.SandboxID)
	if e != nil {
		return r, e
	}
	if absent {
		r.State = Absent
		r.ResourcesHeld = false
		return c.update(ctx, r)
	}
	if observed.ID != r.SandboxID || !matches(r, observed) {
		return r, ErrConflict
	}
	if allowDelete {
		if e = c.config.Authorize(ctx, r.Request, "cancel"); e != nil {
			return r, e
		}
		if e = c.client.remove(ctx, r.SandboxID); e != nil {
			return r, e
		}
		_, gone, e := c.client.inspect(ctx, r.SandboxID)
		if e != nil || !gone {
			return r, ErrUnknown
		}
		r.State = Absent
		r.ResourcesHeld = false
		return c.update(ctx, r)
	}
	if !r.CancelRequested {
		r.State = Allocated
	}
	return c.update(ctx, r)
}
func (c *Controller) Reconcile(ctx context.Context, op string) (Record, error) {
	ctx, done, e := c.begin(ctx, op)
	if e != nil {
		return Record{}, e
	}
	defer done()
	r, e := c.record(op)
	if e != nil {
		return r, e
	}
	if e = c.config.Authorize(ctx, r.Request, "observe"); e != nil {
		return r, e
	}
	if r.State != Absent {
		r, e = c.observe(ctx, r, false)
		if e != nil {
			return r, e
		}
	}
	c.mu.Lock()
	delete(c.restoredPending, op)
	c.mu.Unlock()
	return r, nil
}

// ReconcileRestored reads back a bounded page of exact restored attempts. It
// never creates, starts or deletes a VM. A failed identity read leaves that
// attempt pending and charged; healthy peers are still inspected on this pass.
// Callers may repeat it after provider recovery. New Allocate is fenced until
// every retained identity has a successful readback.
func (c *Controller) ReconcileRestored(ctx context.Context, max int) (int, error) {
	if max < 1 {
		return 0, ErrConflict
	}
	c.mu.Lock()
	ops := make([]string, 0, len(c.restoredPending))
	for op := range c.restoredPending {
		ops = append(ops, op)
	}
	c.mu.Unlock()
	slices.Sort(ops)
	start := 0
	c.mu.Lock()
	after := c.restoredAfter
	c.mu.Unlock()
	if after != "" {
		start = sort.Search(len(ops), func(i int) bool { return ops[i] > after })
		if start == len(ops) {
			start = 0
		}
	}
	var firstErr error
	for i := 0; i < min(max, len(ops)); i++ {
		op := ops[(start+i)%len(ops)]
		if ctx.Err() != nil {
			firstErr = ctx.Err()
			break
		}
		c.mu.Lock()
		c.restoredAfter = op
		c.mu.Unlock()
		if _, err := c.Reconcile(ctx, op); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	c.mu.Lock()
	remaining := len(c.restoredPending)
	c.mu.Unlock()
	return remaining, firstErr
}
func (c *Controller) Cancel(ctx context.Context, op string, expectedRevision int) (Record, error) {
	originalContext := ctx
	ctx, done, e := c.begin(ctx, op)
	if e != nil {
		if errors.Is(e, ErrConflict) {
			return c.cancelBusy(originalContext, op, expectedRevision)
		}
		return Record{}, e
	}
	defer done()
	r, e := c.record(op)
	if e != nil {
		return r, e
	}
	if e = c.config.Authorize(ctx, r.Request, "cancel"); e != nil {
		return r, e
	}
	if r.Revision != expectedRevision {
		return r, ErrConflict
	}
	if r.State == Absent {
		return r, nil
	}
	r.CancelRequested = true
	r.State = Cancelling
	r, e = c.update(ctx, r)
	if e != nil {
		return r, e
	}
	return c.observe(ctx, r, true)
}

// An in-flight transport owns settlement; cancellation records reviewed intent
// without launching a second provider operation. Its acknowledgement means
// cancellation requested, not that the VM or charge has disappeared.
func (c *Controller) cancelBusy(ctx context.Context, op string, expectedRevision int) (Record, error) {
	r, e := c.record(op)
	if e != nil {
		return r, e
	}
	if e = c.config.Authorize(ctx, r.Request, "cancel"); e != nil {
		return r, e
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	current, ok := c.journal.Records[op]
	if !ok || c.closed || !c.busy[op] || current.Revision != expectedRevision || current.RequestDigest != r.RequestDigest || ctx.Err() != nil {
		return current, ErrConflict
	}
	if current.State == Absent {
		return current, nil
	}
	current.CancelRequested, current.State = true, Cancelling
	current.Revision++
	current.UpdatedAt = time.Now().UTC()
	c.journal.Records[op] = current
	err := c.persist(ctx)
	if cancel := c.nativeObservers[op]; cancel != nil {
		cancel()
	}
	return current, err
}

// Close cancels only this controller's transports. It never deletes microVMs
// or claims charges settled. The state lock remains held until all operations
// drain, even if the caller's shutdown deadline expires.
func (c *Controller) Close(ctx context.Context) error {
	c.mu.Lock()
	if !c.closed {
		c.closed = true
		c.cancel()
		go func() {
			c.workers.Wait()
			c.mu.Lock()
			c.client.http.CloseIdleConnections()
			_ = c.root.Close()
			_ = c.lease.Close()
			if c.checkpoint != nil {
				_ = c.checkpoint.Close()
			}
			c.mu.Unlock()
			close(c.stopped)
		}()
	}
	c.mu.Unlock()
	select {
	case <-c.stopped:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
