package e2bbe

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/statelease"
)

type journal struct {
	Version int               `json:"version"`
	OwnerID string            `json:"ownerId"`
	Scope   string            `json:"scope"`
	Records map[string]Record `json:"records"`
}
type Controller struct {
	mu         sync.Mutex
	config     Config
	client     *providerClient
	journal    journal
	root       *os.Root
	lease      *os.File
	busy       map[string]bool
	closed     bool
	stopped    chan struct{}
	ctx        context.Context
	cancel     context.CancelFunc
	workers    sync.WaitGroup
	checkpoint CheckpointWriter
}

func New(c Config) (*Controller, error) { return newController(c, nil, context.Background()) }

func newController(c Config, restored []byte, restoreContext context.Context) (*Controller, error) {
	if !validID(c.OwnerID) || !validID(c.Deployment.Scope) || !validID(c.Deployment.HostGeneration) || c.Deployment.Provider != "aws" || c.Deployment.InstanceType != "m8i.xlarge" || !validID(c.Deployment.Region) || c.Deployment.RuntimeRevision != RuntimeRevision || !digestPattern.MatchString(c.Deployment.ManagementReceiptDigest) || !digestPattern.MatchString(c.Deployment.RestoreReceiptDigest) || c.Checkpoint == nil || c.Authorize == nil {
		return nil, ErrUnsupported
	}
	if !validID(c.Template.ID) || !digestPattern.MatchString(c.Template.BuildDigest) || c.Template.MemoryMiB < 128 || c.Template.MemoryMiB > 32768 || c.Template.CPUMillis < 1000 || c.Template.CPUMillis > 16000 || c.Template.CPUMillis%1000 != 0 {
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
	out := &Controller{config: c, client: client, root: root, lease: lease, busy: map[string]bool{}, ctx: ctx, cancel: cancel, stopped: make(chan struct{}), journal: journal{Version: 1, OwnerID: c.OwnerID, Scope: c.Deployment.Scope, Records: map[string]Record{}}}
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
	_, fileErr := root.Lstat("attempts.json")
	body, _ := json.Marshal(out.journal)
	out.checkpoint, e = c.Checkpoint.Claim(restoreContext, CheckpointClaim{OwnerID: c.OwnerID, Scope: c.Deployment.Scope, JournalDigest: bytesDigest(body), HasJournal: restored != nil || fileErr == nil})
	if e != nil {
		return fail(e)
	}
	if out.checkpoint == nil {
		return fail(ErrUnsupported)
	}

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
	if decoder.Decode(&j) != nil || decoder.Decode(new(any)) != io.EOF || j.Version != 1 || j.OwnerID != owner || j.Scope != scope || j.Records == nil || len(j.Records) > maxRecords {
		return journal{}, ErrConflict
	}
	seenVMs := map[string]bool{}
	seenAttempts := map[string]bool{}
	for op, r := range j.Records {
		if op != r.Request.Identity.OperationID || !validRequest(r.Request) || r.Request.Identity.OwnerID != j.OwnerID || r.RequestDigest != digest(r.Request) || r.Deployment.Scope != j.Scope || r.Deployment.RuntimeRevision != RuntimeRevision || r.Revision < 1 || r.CreatedAt.IsZero() || r.UpdatedAt.Before(r.CreatedAt) || r.ProviderBillingSettled {
			return journal{}, ErrConflict
		}
		if r.Template.MemoryMiB < 128 || r.Template.MemoryMiB > 32768 || r.Template.CPUMillis < 1000 || r.Template.CPUMillis > 16000 || !validID(r.Template.ID) || !digestPattern.MatchString(r.Template.BuildDigest) {
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
		key := r.Request.Identity.ProjectID + "/" + r.Request.Identity.TaskID + "/" + r.Request.Identity.AttemptID
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
	return nil
}
func (c *Controller) usageLocked() (Usage, error) {
	var u Usage
	for _, r := range c.journal.Records {
		// Retain estimated spend across VM spin-down. Absence proves neither
		// refunded model spend nor settled provider/storage billing.
		u.EstimatedCostMicros += r.Request.EstimatedCostMicros
		if r.ResourcesHeld {
			u.Active++
			u.MemoryMiB += r.Template.MemoryMiB
			u.CPUMillis += r.Template.CPUMillis

		}
	}
	return u, nil
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
		c.mu.Unlock()
		if old.RequestDigest != digest(req) {
			return old, ErrConflict
		}
		if old.State == Allocated || old.State == Absent {
			return old, nil
		}
		return old, ErrUnknown
	}
	u, _ := c.usageLocked()
	perProject := 0
	for _, r := range c.journal.Records {
		if r.Request.Identity.ProjectID == req.Identity.ProjectID && r.Request.Identity.TaskID == req.Identity.TaskID && r.Request.Identity.AttemptID == req.Identity.AttemptID {
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
	r := Record{Request: req, RequestDigest: digest(req), Deployment: c.config.Deployment, Template: t, State: Creating, Revision: 1, ResourcesHeld: true, CreatedAt: now, UpdatedAt: now}
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
	if r.State == Absent {
		return r, nil
	}
	return c.observe(ctx, r, false)
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
	return current, c.persist(ctx)
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
