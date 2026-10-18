package dockerbe

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/backend"
)

// The registry belongs to the SAME SourceGuard as the originals. It records
// slot metadata only; g.handles remains the sole owner of native values.
// Neither this mutex nor a receiver mutex may span native IO or another owner.
type assemblyOriginalRegistry struct {
	mu                            sync.Mutex
	retiring                      atomic.Bool
	slots                         []assemblyOriginalSlot
	paths                         map[string]uint64
	planned                       bool
	started, stopped, closeActive bool
	lease                         *DocumentImageAssemblyOriginalRegistryLease
}

type assemblyOriginalSlot struct {
	path      string
	directory bool
	bytes     int64
	state     string
}

// No public constructor from path/count/HANDLE/PID/JSON. Lifetime identity is
// the live registry pointer, independent of any transferable guard generation.
type DocumentImageAssemblyOriginalRegistryLease struct{ registry *assemblyOriginalRegistry }
type DocumentImageAssemblyOriginalRegistryCharge struct {
	registry *assemblyOriginalRegistry
	ceiling  int
}
type DocumentImageAssemblyOriginalRegistryRetirement struct {
	CollectorStarted, CollectorStopped, Retiring, CloseInFlight                                    bool
	ReservedSlots, Planned, PendingOpen, ValidACK, UnknownOpen, PendingClose, UnknownClose, Closed int
	AllSlotsClosed                                                                                 bool
}

func newAssemblyOriginalRegistry(planned bool, slots []assemblyOriginalSlot) *assemblyOriginalRegistry {
	r := &assemblyOriginalRegistry{planned: planned, paths: map[string]uint64{}}
	r.slots = append([]assemblyOriginalSlot(nil), slots...)
	for i, s := range r.slots {
		r.paths[s.path] = uint64(i + 1)
	}
	r.lease = &DocumentImageAssemblyOriginalRegistryLease{registry: r}
	return r
}

func (r *assemblyOriginalRegistry) start() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.started || r.stopped || r.retiring.Load() {
		return backend.ErrUnavailable
	}
	r.started = true
	return nil
}

// Called ONLY by the actual collector's own returning frame, or by the
// prepared owner's irrevocable no-start transition. Context expiry is not ACK.
func (r *assemblyOriginalRegistry) stop() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.stopped = true
	for i := range r.slots {
		if r.slots[i].state == "planned" {
			r.slots[i].state = "closed"
		}
	}
}

func (r *assemblyOriginalRegistry) stopUnstarted() {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.started {
		return
	} // a refusal is never another collector's return ACK
	r.stopped = true
	for i := range r.slots {
		if r.slots[i].state == "planned" {
			r.slots[i].state = "closed"
		}
	}
}

func (r *assemblyOriginalRegistry) retire() {
	if r != nil {
		r.retiring.Store(true)
	}
}

func (r *assemblyOriginalRegistry) open(path string, directory bool, bytes int64) (uint64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.retiring.Load() || !r.started || r.stopped || r.closeActive {
		return 0, backend.ErrUnavailable
	}
	id := r.paths[path]
	if id == 0 {
		if r.planned || len(r.slots) >= 1024 {
			return 0, backend.ErrUnsupported
		}
		r.slots = append(r.slots, assemblyOriginalSlot{path: path, directory: directory, bytes: bytes, state: "planned"})
		id = uint64(len(r.slots))
		r.paths[path] = id
	}
	s := &r.slots[id-1]
	if s.directory != directory || s.bytes != bytes || s.state != "planned" {
		return 0, backend.ErrUnavailable
	}
	s.state = "pending-open"
	return id, nil
}

func (r *assemblyOriginalRegistry) acknowledgeOpen(id uint64, valid, knownAbsent bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if id == 0 || id > uint64(len(r.slots)) || r.slots[id-1].state != "pending-open" {
		r.retiring.Store(true)
		return
	}
	s := &r.slots[id-1]
	s.state = "unknown-open"
	if valid {
		s.state = "valid-ack"
	} else if knownAbsent {
		s.state = "closed"
	}
}

func (r *assemblyOriginalRegistry) beginClose() error {
	r.retire() // independent of native g.mu or any receiver lock
	r.mu.Lock()
	defer r.mu.Unlock()
	if !r.stopped || r.closeActive {
		return backend.ErrUnavailable
	}
	r.closeActive = true
	return nil
}

func (r *assemblyOriginalRegistry) beginSlotClose(id uint64) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if !r.closeActive || id == 0 || id > uint64(len(r.slots)) {
		return backend.ErrUnavailable
	}
	s := &r.slots[id-1]
	if s.state != "valid-ack" && s.state != "unknown-close" {
		return backend.ErrUnavailable
	}
	s.state = "pending-close"
	return nil
}

func (r *assemblyOriginalRegistry) acknowledgeClose(id uint64, closed bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if id == 0 || id > uint64(len(r.slots)) || r.slots[id-1].state != "pending-close" {
		r.retiring.Store(true)
		return
	}
	r.slots[id-1].state = "unknown-close"
	if closed {
		r.slots[id-1].state = "closed"
	}
}

func (r *assemblyOriginalRegistry) finishClose() { r.mu.Lock(); r.closeActive = false; r.mu.Unlock() }

// Charge has a private registry identity. Its fixed ceiling never refunds after
// Close compaction; the receiver releases its registration only from actual
// collector-stop AND all-slot-close observations, not caller counts/booleans.
func (l *DocumentImageAssemblyOriginalRegistryLease) Charge() (DocumentImageAssemblyOriginalRegistryCharge, error) {
	if l == nil || l.registry == nil {
		return DocumentImageAssemblyOriginalRegistryCharge{}, backend.ErrLaunchFenceOwnership
	}
	r := l.registry
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.slots) == 0 || len(r.slots) > 1024 || !r.planned && !r.stopped {
		return DocumentImageAssemblyOriginalRegistryCharge{}, backend.ErrUnavailable
	}
	return DocumentImageAssemblyOriginalRegistryCharge{registry: r, ceiling: len(r.slots)}, nil
}
func (c DocumentImageAssemblyOriginalRegistryCharge) Slots() (int, error) {
	if c.registry == nil || c.ceiling <= 0 || c.ceiling > 1024 {
		return 0, backend.ErrLaunchFenceOwnership
	}
	return c.ceiling, nil
}
func (c DocumentImageAssemblyOriginalRegistryCharge) SameRegistry(other DocumentImageAssemblyOriginalRegistryCharge) bool {
	return c.registry != nil && c.registry == other.registry
}
func (l *DocumentImageAssemblyOriginalRegistryLease) RetirementState() (DocumentImageAssemblyOriginalRegistryRetirement, error) {
	if l == nil || l.registry == nil {
		return DocumentImageAssemblyOriginalRegistryRetirement{}, backend.ErrLaunchFenceOwnership
	}
	r := l.registry
	r.mu.Lock()
	defer r.mu.Unlock()
	out := DocumentImageAssemblyOriginalRegistryRetirement{CollectorStarted: r.started, CollectorStopped: r.stopped, Retiring: r.retiring.Load(), CloseInFlight: r.closeActive, ReservedSlots: len(r.slots)}
	for _, s := range r.slots {
		switch s.state {
		case "planned":
			out.Planned++
		case "pending-open":
			out.PendingOpen++
		case "valid-ack":
			out.ValidACK++
		case "unknown-open":
			out.UnknownOpen++
		case "pending-close":
			out.PendingClose++
		case "unknown-close":
			out.UnknownClose++
		case "closed":
			out.Closed++
		}
	}
	out.AllSlotsClosed = r.stopped && !r.closeActive && out.Closed == len(r.slots)
	return out, nil
}

type assemblyOriginalAcquisition struct {
	mu       sync.Mutex // one-shot metadata only, never collector/native/owner IO
	retired  atomic.Bool
	started  bool
	guard    *assemblySourceGuard
	evidence *assemblyEvidenceOwner
	packages *assemblyPackageInputOwner
	namespace *assemblyNamespaceOwner
	directory string
	objects  []backend.DocumentImageAssemblyEvidenceObject
}

func (a *assemblyOriginalAcquisition) begin(ctx context.Context) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.started || a.retired.Load() || ctx.Err() != nil {
		return backend.ErrUnavailable
	}
	a.started = true
	return nil
}
func (a *assemblyOriginalAcquisition) requestRetirement() {
	a.retired.Store(true)
	if a.guard == nil {
		return
	}
	r := a.guard.originalRegistry()
	if r == nil {
		return
	}
	r.retire()
	a.mu.Lock()
	notStarted := !a.started
	a.mu.Unlock()
	if notStarted {
		r.stop()
	} // the latch prevents this one-shot ever starting
}

// Prepared metadata owns no native handles. After Acquire the SAME ordinary
// guard owner receives every partial ACK; the prepared value is latch-only.
// Exported shells contain one shared private state pointer: even copying a
// shell cannot duplicate its one-shot latch, mutex or original ownership.
type DocumentImageAssemblyEvidenceAcquisition struct{ state *assemblyOriginalAcquisition }
type DocumentImageAssemblyPackageAcquisition struct{ state *assemblyOriginalAcquisition }
type DocumentImageAssemblyNamespaceAcquisition struct{ state *assemblyOriginalAcquisition }

func (a *DocumentImageAssemblyNamespaceAcquisition) RegistryLease() (*DocumentImageAssemblyOriginalRegistryLease, error) {
	if a == nil || a.state == nil || a.state.namespace == nil {
		return nil, backend.ErrLaunchFenceOwnership
	}
	return assemblyRegistryLease(a.state.guard)
}

// Immediate metadata latch only. The sole current token must remain retained
// until the receiver's actual borrowers settle before explicit native Close.
func (a *DocumentImageAssemblyNamespaceAcquisition) RequestRetirement() {
	if a == nil || a.state == nil || a.state.namespace == nil { return }
	s := a.state
	s.requestRetirement()
	o := s.namespace
	o.mu.Lock()
	o.closing = true
	o.ready = false
	o.mu.Unlock()
}

func (a *DocumentImageAssemblyNamespaceAcquisition) Acquire(ctx context.Context) (backend.DocumentImageAssemblyNamespaceGuard, error) {
	if a == nil || a.state == nil || a.state.namespace == nil || a.state.guard == nil {
		return nil, backend.ErrLaunchFenceOwnership
	}
	s := a.state
	if err := s.begin(ctx); err != nil { return nil, err }
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	err := acquireAssemblyPreparedNamespace(ctx, s.guard, s.directory)
	o := s.namespace
	o.mu.Lock()
	if err == nil && (o.closing || s.retired.Load() || s.guard.originalRegistry().retiring.Load() || ctx.Err() != nil) {
		err = backend.ErrUnavailable
	}
	o.ready = err == nil
	o.mu.Unlock()
	// Return exact custody even on partial/late failure. Neither timeout nor
	// failed eligibility is a collector-stop or native-close witness.
	return &assemblyNamespaceToken{o, 1}, err
}

func (a *DocumentImageAssemblyEvidenceAcquisition) RegistryLease() (*DocumentImageAssemblyOriginalRegistryLease, error) {
	if a == nil || a.state == nil || a.state.evidence == nil {
		return nil, backend.ErrLaunchFenceOwnership
	}
	return assemblyRegistryLease(a.state.guard)
}
func (a *DocumentImageAssemblyPackageAcquisition) RegistryLease() (*DocumentImageAssemblyOriginalRegistryLease, error) {
	if a == nil || a.state == nil || a.state.packages == nil {
		return nil, backend.ErrLaunchFenceOwnership
	}
	return assemblyRegistryLease(a.state.guard)
}
func assemblyRegistryLease(g *assemblySourceGuard) (*DocumentImageAssemblyOriginalRegistryLease, error) {
	if g == nil || g.originalRegistry() == nil {
		return nil, backend.ErrUnsupported
	}
	return g.originalRegistry().lease, nil
}

func (a *DocumentImageAssemblyEvidenceAcquisition) RequestRetirement() {
	if a != nil && a.state != nil && a.state.evidence != nil {
		s := a.state
		s.requestRetirement()
		o := s.evidence
		o.mu.Lock()
		o.closing = true
		o.ready = false
		loan := o.loan
		o.mu.Unlock()
		if loan != nil {
			_ = loan.Retire()
		}
	}
}
func (a *DocumentImageAssemblyPackageAcquisition) RequestRetirement() {
	if a != nil && a.state != nil && a.state.packages != nil {
		s := a.state
		s.requestRetirement()
		o := s.packages
		o.mu.Lock()
		o.retiring = true
		o.ready = false
		loan := o.loan
		o.mu.Unlock()
		if loan != nil {
			_ = loan.Retire()
		}
	}
}

func (a *DocumentImageAssemblyEvidenceAcquisition) Acquire(ctx context.Context) (backend.DocumentImageAssemblyEvidenceGuard, error) {
	if a == nil || a.state == nil || a.state.evidence == nil || a.state.guard == nil {
		return nil, backend.ErrLaunchFenceOwnership
	}
	s := a.state
	if err := s.begin(ctx); err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	err := acquireAssemblyPreparedSources(ctx, s.guard, s.objects)
	o := s.evidence
	o.mu.Lock()
	if err == nil && (o.closing || s.retired.Load() || s.guard.originalRegistry().retiring.Load() || ctx.Err() != nil) {
		err = backend.ErrUnavailable
	}
	o.ready = err == nil
	o.mu.Unlock()
	return &assemblyEvidenceToken{o, 1}, err
}
func (a *DocumentImageAssemblyPackageAcquisition) Acquire(ctx context.Context) (backend.DocumentImageAssemblyPackageInputGuard, error) {
	if a == nil || a.state == nil || a.state.packages == nil || a.state.guard == nil {
		return nil, backend.ErrLaunchFenceOwnership
	}
	s := a.state
	if err := s.begin(ctx); err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	err := acquireAssemblyPreparedSources(ctx, s.guard, s.objects)
	o := s.packages
	o.mu.Lock()
	if err == nil && (o.retiring || s.retired.Load() || s.guard.originalRegistry().retiring.Load() || ctx.Err() != nil) {
		err = backend.ErrUnavailable
	}
	o.ready = err == nil
	o.mu.Unlock()
	return &assemblyPackageInputToken{o, 1}, err
}

func BorrowDocumentImageAssemblyEvidenceRegistry(ctx context.Context, g backend.DocumentImageAssemblyEvidenceGuard) (*DocumentImageAssemblyOriginalRegistryLease, error) {
	t, ok := g.(*assemblyEvidenceToken)
	if !ok || t == nil || t.owner == nil {
		return nil, backend.ErrLaunchFenceOwnership
	}
	o := t.owner
	o.mu.Lock()
	current := t.generation > 0 && t.current() && o.ready && !o.closing && !o.closed && !o.poisoned && ctx.Err() == nil
	guard := o.guard
	o.mu.Unlock()
	if !current {
		return nil, backend.ErrUnavailable
	}
	lease, err := assemblyRegistryLease(guard)
	if err != nil {
		return nil, err
	}
	o.mu.Lock()
	current = t.current() && o.ready && !o.closing && !o.closed && !o.poisoned && ctx.Err() == nil
	o.mu.Unlock()
	if !current || lease.registry.retiring.Load() {
		return nil, backend.ErrUnavailable
	}
	return lease, nil
}
func BorrowDocumentImageAssemblyPackageRegistry(ctx context.Context, g backend.DocumentImageAssemblyPackageInputGuard) (*DocumentImageAssemblyOriginalRegistryLease, error) {
	t, ok := g.(*assemblyPackageInputToken)
	if !ok || t == nil || t.owner == nil {
		return nil, backend.ErrLaunchFenceOwnership
	}
	o := t.owner
	o.mu.Lock()
	current := t.generation > 0 && t.generation == o.generation && o.ready && !o.retiring && !o.closed && ctx.Err() == nil
	guard := o.guard
	o.mu.Unlock()
	if !current {
		return nil, backend.ErrUnavailable
	}
	lease, err := assemblyRegistryLease(guard)
	if err != nil {
		return nil, err
	}
	o.mu.Lock()
	current = t.generation == o.generation && o.ready && !o.retiring && !o.closed && ctx.Err() == nil
	o.mu.Unlock()
	if !current || lease.registry.retiring.Load() {
		return nil, backend.ErrUnavailable
	}
	return lease, nil
}
