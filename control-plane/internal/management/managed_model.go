package management

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/hostpath"
	"github.com/lcfrai/luci/control-plane/internal/managedmodel"
)

type managedModelConfig struct {
	authorize      func() error
	Enabled        bool                 `json:"enabled"`
	StartWithLuci  bool                 `json:"startWithLuci"`
	ExecutablePath string               `json:"executablePath,omitempty"`
	ModelStore     string               `json:"modelStore,omitempty"`
	Port           int                  `json:"port"`
	Model          string               `json:"model"`
	Profile        managedmodel.Profile `json:"profile"`
}
type managedModelSaved struct {
	Revision           int                `json:"revision"`
	AuthorizingActorID string             `json:"authorizingActorId"`
	Config             managedModelConfig `json:"config"`
}
type managedModelSample struct {
	loaded   bool
	failure  string
	exitCode *uint32
	at       time.Time
}
type managedModelStartError struct {
	code     string
	cause    error
	exitCode *uint32
}

func (e *managedModelStartError) Error() string { return e.code }
func (e *managedModelStartError) Unwrap() error { return e.cause }

func managedHostCapacityDiagnostic(err error) *managedmodel.HostCapacityDiagnostic {
	var reported interface {
		HostCapacityDiagnostic() *managedmodel.HostCapacityDiagnostic
	}
	if !errors.As(err, &reported) {
		return nil
	}
	source := reported.HostCapacityDiagnostic()
	if source == nil {
		return nil
	}
	copy := *source
	if source.PhysicalAvailableBytes != nil {
		value := *source.PhysicalAvailableBytes
		copy.PhysicalAvailableBytes = &value
	}
	if source.CommitAvailableBytes != nil {
		value := *source.CommitAvailableBytes
		copy.CommitAvailableBytes = &value
	}
	if source.CPUPercent != nil {
		value := *source.CPUPercent
		copy.CPUPercent = &value
	}
	if source.SampleAgeMillis != nil {
		value := *source.SampleAgeMillis
		copy.SampleAgeMillis = &value
	}
	return &copy
}

func managedObservationDiagnostic(err error) *managedmodel.ObservationDiagnostic {
	var reported interface {
		ManagedModelObservationDiagnostic() *managedmodel.ObservationDiagnostic
	}
	if !errors.As(err, &reported) {
		return nil
	}
	source := reported.ManagedModelObservationDiagnostic()
	if source == nil {
		return nil
	}
	switch source.Stage {
	case "listener", "peer", "tags", "show", "ps":
	default:
		return nil
	}
	switch source.Reason {
	case "ownership_unavailable", "request_failed", "cancelled", "timeout", "connect_refused", "connection_lost", "http_status", "response_too_large", "response_read_failed", "response_invalid", "response_close_failed", "parameters_ambiguous", "model_identity_mismatch", "profile_mismatch", "loaded_count_mismatch", "loaded_profile_mismatch":
	default:
		if !managedListenerReasonAllowed(source.Reason) || (source.Stage != "listener" && source.Stage != "peer") {
			return nil
		}
	}
	copy := *source
	copy.HTTPStatus = nil
	if source.HTTPStatus != nil && *source.HTTPStatus >= 100 && *source.HTTPStatus <= 599 {
		value := *source.HTTPStatus
		copy.HTTPStatus = &value
	}
	return &copy
}

func managedListenerReasonAllowed(reason string) bool {
	switch reason {
	case "listener_missing", "listener_table_size_failed", "listener_table_resize_required", "listener_table_read_failed", "listener_table_invalid", "listener_address_mismatch", "listener_multiple", "listener_pid_unavailable", "owner_stopping", "owner_pid_mismatch", "owner_wait_failed", "owner_exited", "job_query_failed", "owner_not_in_job":
		return true
	}
	return false
}

type managedModelService interface {
	sample(context.Context) (managedModelSample, error)
	stop(context.Context) error
}
type managedModelPlatform interface {
	start(context.Context, managedModelConfig, uint64) (managedModelService, error)
}
type managedModelManager struct {
	mu                             sync.Mutex
	gateway                        *Gateway
	platform                       managedModelPlatform
	saved                          managedModelSaved
	status                         managedmodel.Status
	boot, path                     string
	barrier                        *os.File // Retained no-delete handle keeps the cross-process fence inode stable.
	owner                          managedModelService
	generation                     uint64
	closing, withdrawing, stopping bool
	cancel                         context.CancelFunc
	done                           chan struct{}
}

func newManagedModelManager(g *Gateway) (*managedModelManager, error) {
	m := &managedModelManager{gateway: g, path: filepath.Join(g.dir, "managed-local-model-status.json"), status: managedmodel.Status{State: "disabled", Generation: "0", ObservedAt: time.Now().UTC()}}
	m.saved.Config.Profile = managedmodel.FixedProfile()
	var random [32]byte
	if _, err := rand.Read(random[:]); err != nil {
		return nil, err
	}
	m.boot = hex.EncodeToString(random[:])
	m.platform = newManagedModelPlatform(g.dir)
	if m.platform == nil || g.opts.LocalAccessToken == "" || g.opts.ManagedModelDependents == nil || g.opts.PortableOwnerAccess || g.managedTeamID != "" {
		m.platform = nil
		return m, nil
	}
	p := filepath.Join(g.dir, "managed-local-model.json")
	if _, err := os.Lstat(p); err == nil {
		if err = hostpath.OneName(p); err != nil {
			return nil, err
		}
		f, err := os.Open(p)
		if err != nil {
			return nil, err
		}
		raw, e := io.ReadAll(io.LimitReader(f, 16385))
		closeErr := f.Close()
		if e != nil || closeErr != nil || len(raw) > 16384 || json.Unmarshal(raw, &m.saved) != nil || m.saved.Revision < 1 || validateManagedModelConfig(m.saved.Config) != nil {
			return nil, errors.New("saved managed model configuration cannot be verified")
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	if m.saved.Config.Enabled {
		m.status.State = "stopped"
	}
	var err error
	m.barrier, err = os.OpenFile(m.path+".barrier", os.O_RDWR|os.O_CREATE, 0600)
	if err != nil {
		return nil, err
	}
	if err = hostpath.OneName(m.path + ".barrier"); err != nil {
		m.barrier.Close()
		return nil, err
	}
	if err = managedmodel.Configure(m.path, m.boot); err != nil {
		m.barrier.Close()
		return nil, err
	}
	if err = m.publishLocked(); err != nil {
		m.barrier.Close()
		return nil, err
	}
	return m, nil
}
func validateManagedModelConfig(c managedModelConfig) error {
	if c.Profile != managedmodel.FixedProfile() || c.Port < 1024 || c.Port > 65535 || c.Model == "" || len(c.Model) > 200 || strings.ContainsAny(c.Model, "\x00\r\n\t \\/") || !filepath.IsAbs(c.ExecutablePath) || filepath.Clean(c.ExecutablePath) != c.ExecutablePath || !filepath.IsAbs(c.ModelStore) || filepath.Clean(c.ModelStore) != c.ModelStore || !strings.EqualFold(filepath.Base(c.ExecutablePath), "ollama.exe") {
		return errors.New("Choose an installed Ollama executable, an existing model store, a local port and an installed model.")
	}
	return nil
}
func (m *managedModelManager) publishLocked() error {
	if m.platform == nil {
		return nil
	}
	if m.status.State != "available" && m.status.State != "loaded" {
		m.status.ObservedAt = time.Now().UTC()
	}
	b, e := json.Marshal(managedmodel.Observation{Version: 1, Boot: m.boot, Revision: m.saved.Revision, Withdrawing: m.withdrawing || m.closing, Status: m.status})
	if e != nil {
		return e
	}
	return atomicPrivate(m.path, b)
}
func (g *Gateway) registerManagedModel(mux *http.ServeMux) {
	mux.HandleFunc("GET /v1/setup/local-model", g.managedModelStatus)
	mux.HandleFunc("POST /v1/setup/local-model", g.saveManagedModel)
	mux.HandleFunc("POST /v1/setup/local-model/start", g.startManagedModel)
	mux.HandleFunc("POST /v1/setup/local-model/stop", g.stopManagedModel)
}
func (g *Gateway) managedModelStatus(w http.ResponseWriter, r *http.Request) {
	m := g.localModels
	u, p, ok := ActorFromContext(r.Context())
	if !ok {
		fail(w, 401, "sign_in_required", "Sign in to Luci to continue.")
		return
	}
	available := m != nil && m.platform != nil
	can := available && !u.Disabled && PermissionsFor(u, p).ManageBacking
	if m == nil {
		jsonReply(w, 200, map[string]any{"available": false, "canManage": false, "revision": 0, "config": nil, "status": managedmodel.Status{State: "disabled", Generation: "0", ObservedAt: time.Now().UTC()}})
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	jsonReply(w, 200, m.projectionLocked(can))
}
func (m *managedModelManager) projectionLocked(can bool) map[string]any {
	var config *managedModelConfig
	if m.saved.Revision > 0 {
		c := m.saved.Config
		if !can {
			c.ExecutablePath = ""
			c.ModelStore = ""
		}
		config = &c
	}
	return map[string]any{"available": m.platform != nil, "canManage": can, "revision": m.saved.Revision, "config": config, "status": m.status}
}
func (g *Gateway) managedModelMutation(w http.ResponseWriter, r *http.Request) (*managedModelManager, bool) {
	actor, ok := requireAdmin(w, r, false)
	if !ok {
		return nil, false
	}
	if !g.managedModelAuthorized(actor.ID) {
		fail(w, 403, "managed_model_authority_changed", "Host backing permission changed. Refresh your account.")
		return nil, false
	}
	g.mu.Lock()
	closed := g.closed
	g.mu.Unlock()
	if closed || g.localModels == nil || g.localModels.platform == nil {
		fail(w, 409, "managed_model_unavailable", "Managed local models require this native app's personal host service.")
		return nil, false
	}
	return g.localModels, true
}
func (g *Gateway) saveManagedModel(w http.ResponseWriter, r *http.Request) {
	m, ok := g.managedModelMutation(w, r)
	if !ok {
		return
	}
	var q struct {
		ExpectedRevision int    `json:"expectedRevision"`
		Enabled          bool   `json:"enabled"`
		StartWithLuci    bool   `json:"startWithLuci"`
		ExecutablePath   string `json:"executablePath"`
		ModelStore       string `json:"modelStore"`
		Port             int    `json:"port"`
		Model            string `json:"model"`
	}
	if !decode(w, r, &q) {
		return
	}
	c := managedModelConfig{Enabled: q.Enabled, StartWithLuci: q.StartWithLuci, ExecutablePath: q.ExecutablePath, ModelStore: q.ModelStore, Port: q.Port, Model: q.Model, Profile: managedmodel.FixedProfile()}
	if err := validateManagedModelConfig(c); err != nil {
		fail(w, 400, "invalid_managed_model", err.Error())
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closing || m.stopping || m.withdrawing && m.status.State != "disabled" && m.status.State != "stopped" || m.owner != nil || m.done != nil || q.ExpectedRevision != m.saved.Revision {
		fail(w, 409, "managed_model_changed", "Stop the current managed model and refresh its settings before saving.")
		return
	}
	actor, _, _ := ActorFromContext(r.Context())
	if !g.managedModelAuthorized(actor.ID) {
		fail(w, 403, "managed_model_authority_changed", "Host backing permission changed. Refresh your account before saving.")
		return
	}
	next := managedModelSaved{Revision: m.saved.Revision + 1, AuthorizingActorID: actor.ID, Config: c}
	b, err := json.Marshal(next)
	if err == nil {
		err = atomicPrivate(filepath.Join(g.dir, "managed-local-model.json"), b)
	}
	if err != nil {
		fail(w, 500, "managed_model_save_failed", "The managed model settings could not be saved.")
		return
	}
	m.saved = next
	m.status = managedmodel.Status{State: "disabled", Generation: m.status.Generation}
	if c.Enabled {
		m.status.State = "stopped"
	}
	if m.publishLocked() != nil {
		m.status.State = "unknown"
		fail(w, 500, "managed_model_status_failed", "The saved settings could not be published safely.")
		return
	}
	jsonReply(w, 200, m.projectionLocked(true))
}
func (g *Gateway) startManagedModel(w http.ResponseWriter, r *http.Request) {
	m, ok := g.managedModelMutation(w, r)
	if !ok {
		return
	}
	var q struct {
		ExpectedRevision int `json:"expectedRevision"`
	}
	if !decode(w, r, &q) {
		return
	}
	actor, _, _ := ActorFromContext(r.Context())
	_, err := m.beginStart(q.ExpectedRevision, actor.ID)
	if err != nil {
		m.mu.Lock()
		previous := m.status.Failure != nil && m.status.Failure.Code == "previous_work_unsettled"
		m.mu.Unlock()
		if previous {
			fail(w, 409, "previous_work_unsettled", "End or reconcile the previous managed-model session in Luci before starting a replacement.")
		} else {
			fail(w, 409, "managed_model_changed", "Refresh the managed model settings before starting.")
		}
		return
	}
	g.managedModelStatus(w, r)
}
func (m *managedModelManager) beginStart(revision int, initiators ...string) (string, error) {
	m.mu.Lock()
	if m.closing || m.stopping || m.owner != nil || m.done != nil || revision != m.saved.Revision || !m.saved.Config.Enabled {
		m.mu.Unlock()
		return "", errors.New("managed model changed")
	}
	if !m.gateway.managedModelAuthorized(m.saved.AuthorizingActorID) {
		m.status.State = "blocked"
		m.status.Failure = &managedmodel.Failure{Code: "authority_changed"}
		_ = m.publishLocked()
		m.mu.Unlock()
		return "", errors.New("managed model authorization changed")
	}
	if err := m.checkPreviousParticipants(); err != nil {
		m.status.State = "blocked"
		m.status.Failure = &managedmodel.Failure{Code: "previous_work_unsettled"}
		_ = m.publishLocked()
		m.mu.Unlock()
		return "", err
	}
	m.generation++
	m.status = managedmodel.Status{State: "starting", Generation: strconv.FormatUint(m.generation, 10)}
	m.withdrawing = true
	ctx, cancel := context.WithCancel(context.Background())
	m.cancel = cancel
	m.done = make(chan struct{})
	done := m.done
	c, gen := m.saved.Config, m.generation
	actor := m.saved.AuthorizingActorID
	initiator := actor
	if len(initiators) > 0 {
		initiator = initiators[0]
	}
	c.authorize = func() error {
		if !m.gateway.managedModelAuthorized(actor) || !m.gateway.managedModelAuthorized(initiator) {
			return errors.New("managed model authorization changed")
		}
		return nil
	}
	if m.publishLocked() != nil {
		m.cancel = nil
		m.done = nil
		m.mu.Unlock()
		cancel()
		return "", errors.New("managed model status unavailable")
	}
	m.mu.Unlock()
	go m.run(ctx, c, gen, done)
	return strconv.FormatUint(gen, 10), nil
}
func (m *managedModelManager) run(ctx context.Context, c managedModelConfig, gen uint64, done chan struct{}) {
	defer func() {
		m.mu.Lock()
		// Only this completed generation may release its start bookkeeping,
		// and only after the owner has positively retired. Close under the same
		// lock so no replacement can observe cleared fields before completion.
		if m.done == done && m.generation == gen && m.owner == nil {
			if m.cancel != nil {
				m.cancel()
			}
			m.cancel = nil
			m.done = nil
		}
		close(done)
		m.mu.Unlock()
	}()
	startup := time.Now().Add(30 * time.Second)
	startCtx, cancel := context.WithDeadline(ctx, startup)
	owner, err := m.platform.start(startCtx, c, gen)
	cancel()
	m.mu.Lock()
	m.owner = owner
	if err != nil {
		m.status.State = "blocked"
		code := "startup_exit"
		var typed *managedModelStartError
		if errors.As(err, &typed) {
			code = typed.code
		}
		if errors.Is(err, context.DeadlineExceeded) {
			code = "startup_timeout"
		}
		m.status.Failure = &managedmodel.Failure{Code: code}
		m.status.Failure.HostCapacity = managedHostCapacityDiagnostic(err)
		m.status.Failure.Observation = managedObservationDiagnostic(err)
		if typed != nil {
			m.status.Failure.ExitCode = typed.exitCode
		}
		if owner != nil {
			m.status.State = "unknown"
		}
		_ = m.publishLocked()
		m.mu.Unlock()
		return
	}
	m.mu.Unlock()
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		sample, e := owner.sample(ctx)
		m.mu.Lock()
		if ctx.Err() != nil {
			m.mu.Unlock()
			return
		}
		if !m.closing && c.authorize() != nil {
			sample.failure = "authority_changed"
			e = errors.New("managed model authorization changed")
		}
		if e != nil {
			m.status.State = "blocked"
			code := sample.failure
			if code == "" {
				code = "observation_unavailable"
			}
			m.status.Failure = &managedmodel.Failure{Code: code, ExitCode: sample.exitCode, HostCapacity: managedHostCapacityDiagnostic(e), Observation: managedObservationDiagnostic(e)}
			m.withdrawing = true
			_ = m.publishLocked()
			m.mu.Unlock()
			m.retireFailedOwner(owner, gen, done)
			return
		}
		if sample.loaded {
			m.status.State = "loaded"
		} else if sample.failure == "starting" {
			m.status.State = "starting"
		} else {
			m.status.State = "available"
		}
		if m.status.State == "available" || m.status.State == "loaded" {
			if sample.at.IsZero() || time.Since(sample.at) < 0 || time.Since(sample.at) > 5*time.Second {
				m.status.State = "blocked"
				m.status.Failure = &managedmodel.Failure{Code: "observation_unavailable"}
				m.withdrawing = true
				_ = m.publishLocked()
				m.mu.Unlock()
				m.retireFailedOwner(owner, gen, done)
				return
			}
			m.status.ObservedAt = sample.at.UTC()
		}
		if m.status.State == "starting" && time.Now().After(startup) {
			m.status.State = "blocked"
			m.status.Failure = &managedmodel.Failure{Code: "startup_timeout"}
			m.withdrawing = true
			_ = m.publishLocked()
			m.mu.Unlock()
			m.retireFailedOwner(owner, gen, done)
			return
		}
		m.status.Endpoint = "http://127.0.0.1:" + strconv.Itoa(c.Port) + "/v1"
		m.status.Model = c.Model
		m.withdrawing = m.stopping || m.closing || m.status.State == "starting"
		_ = m.publishLocked()
		m.mu.Unlock()
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
func (g *Gateway) managedModelAuthorized(id string) bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	record, ok := g.state.Users[id]
	return id != "" && ok && !g.closed && !record.User.Disabled && PermissionsFor(record.User, g.state.Policy).ManageBacking
}
func (m *managedModelManager) retireFailedOwner(owner managedModelService, gen uint64, done chan struct{}) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	err := owner.stop(ctx)
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.done != done || m.generation != gen {
		return
	}
	if err != nil {
		m.status.State = "unknown"
		var host *managedmodel.HostCapacityDiagnostic
		var observation *managedmodel.ObservationDiagnostic
		if m.status.Failure != nil {
			host = m.status.Failure.HostCapacity
			observation = m.status.Failure.Observation
		}
		m.status.Failure = &managedmodel.Failure{Code: "cleanup_unknown", HostCapacity: host, Observation: observation}
	} else {
		m.owner = nil
	}
	_ = m.publishLocked()
}
func (g *Gateway) stopManagedModel(w http.ResponseWriter, r *http.Request) {
	m, ok := g.managedModelMutation(w, r)
	if !ok {
		return
	}
	var q struct {
		ExpectedRevision   int    `json:"expectedRevision"`
		ExpectedGeneration string `json:"expectedGeneration"`
	}
	if !decode(w, r, &q) {
		return
	}
	m.mu.Lock()
	current := q.ExpectedRevision == m.saved.Revision && q.ExpectedGeneration == m.status.Generation
	m.mu.Unlock()
	if !current {
		fail(w, 409, "managed_model_changed", "Refresh the current model before stopping.")
		return
	}
	actor, _, _ := ActorFromContext(r.Context())
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	if err := m.stopExpected(ctx, false, q.ExpectedRevision, q.ExpectedGeneration, actor.ID); err != nil {
		fail(w, 409, "managed_model_stop_blocked", "Stop the dependent agent sessions first, or wait for the current ownership operation to finish.")
		return
	}
	g.managedModelStatus(w, r)
}
func (m *managedModelManager) stop(ctx context.Context, closing bool) error {
	return m.stopExpected(ctx, closing, -1, "")
}
func (m *managedModelManager) stopExpected(ctx context.Context, closing bool, revision int, generation string, actors ...string) error {
	m.mu.Lock()
	if m.stopping || revision >= 0 && (revision != m.saved.Revision || generation != m.status.Generation) {
		m.mu.Unlock()
		return errors.New("managed model generation changed or ownership operation in progress")
	}
	m.stopping = true
	m.closing = m.closing || closing
	m.withdrawing = true
	gen := m.status.Generation
	rev := m.saved.Revision
	startupDone := m.done
	var cancelStartup context.CancelFunc
	if m.owner == nil {
		cancelStartup = m.cancel
	}
	if err := m.publishLocked(); err != nil {
		m.stopping = false
		m.mu.Unlock()
		return err
	}
	m.mu.Unlock()
	if cancelStartup != nil {
		cancelStartup()
	}
	defer func() {
		m.mu.Lock()
		m.stopping = false
		if !m.closing && m.owner != nil && m.status.Failure == nil && (m.status.State == "available" || m.status.State == "loaded") {
			m.withdrawing = false
			_ = m.publishLocked()
		}
		m.mu.Unlock()
	}()
	unlock, err := managedmodel.WithdrawBarrier(ctx, m.path)
	if err != nil {
		return err
	}
	defer unlock()
	if err = m.gateway.settleManagedModelDependents(ctx, m.boot, gen, closing); err != nil {
		return err
	}
	if closing {
		if err = m.gateway.settleManagedPrivateAPIs(ctx); err != nil {
			return err
		}
	}
	if !closing && len(actors) > 0 && !m.gateway.managedModelAuthorized(actors[0]) {
		return errors.New("managed model stop authority changed")
	}
	m.mu.Lock()
	if m.saved.Revision != rev || m.status.Generation != gen {
		m.mu.Unlock()
		return errors.New("managed model generation changed during retirement")
	}
	cancel, done, owner := m.cancel, m.done, m.owner
	if startupDone != nil {
		done = startupDone
	}
	if cancel != nil {
		cancel()
	}
	m.status.State = "stopping"
	_ = m.publishLocked()
	m.mu.Unlock()
	if done != nil {
		select {
		case <-done:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	m.mu.Lock()
	owner = m.owner
	m.mu.Unlock()
	if owner != nil {
		if err = owner.stop(ctx); err != nil {
			m.mu.Lock()
			m.status.State = "unknown"
			m.status.Failure = &managedmodel.Failure{Code: "cleanup_unknown"}
			_ = m.publishLocked()
			m.mu.Unlock()
			return err
		}
	}
	m.mu.Lock()
	m.owner = nil
	m.cancel = nil
	m.done = nil
	m.status.State = "stopped"
	m.status.Failure = nil
	err = m.publishLocked()
	m.mu.Unlock()
	return err
}
