package management

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/resourcebudget"
)

// The backend performs native observation, never persistence or authorization.
// Status must read its cache without starting a Docker/native probe.
type ExecutionStorageConfigurer interface {
	LocalStorageStatus() resourcebudget.LocalStorageStatus
	PrepareLocalStorageConfirmation(context.Context, string) (resourcebudget.LocalStorageConfirmation, error)
}

func (g *Gateway) executionStoragePath() string {
	return filepath.Join(g.opts.ExecutionStateDir, "local-storage-confirmation.json")
}

// Recheck both current role and this login after native I/O. A context actor is
// an initial routing decision, not durable permission to change host settings.
func (g *Gateway) executionStorageActorLocked(r *http.Request, id string) bool {
	row, ok := g.state.Users[id]
	token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	session, exists := g.state.Sessions[hashToken(token)]
	return ok && !g.closed && !row.User.Disabled && row.User.Role == "super-admin" && token != "" && token != r.Header.Get("Authorization") && exists && session.UserID == id && time.Now().Before(session.ExpiresAt)
}

func clearExecutionStorageSamples(status *resourcebudget.LocalStorageStatus) {
	status.HostAvailableBytes, status.GuestAvailableBytes = nil, nil
	status.ObservedAt = time.Time{}
	status.MountedVolumeCount, status.LimitingVolume = 0, ""
}

func (g *Gateway) executionStorageStatus() (resourcebudget.LocalStorageStatus, error) {
	status := resourcebudget.LocalStorageStatus{Platform: runtime.GOOS, Scope: "not-required", State: "not-required", ReasonCode: "not_required", Message: "Local Windows Docker storage confirmation is not required by this backend."}
	if g.opts.ExecutionStorage != nil {
		status = g.opts.ExecutionStorage.LocalStorageStatus()
	}
	saved, err := resourcebudget.ReadLocalStorageConfirmation(g.executionStoragePath())
	if errors.Is(err, os.ErrNotExist) {
		if status.Required && (status.State == "confirmed" || status.State == "pending-observation" || status.State == "invalidated") {
			status.State, status.ReasonCode = "confirmation-required", "confirmation_required"
			status.Message = "A host Super Admin must confirm the active and pending local Docker storage locations before new heavy work."
		}
		clearExecutionStorageSamples(&status)
		status.Revision, status.VHDPath, status.ConfirmedAt = 0, "", time.Time{}
		return status, nil
	}
	if err != nil {
		return resourcebudget.LocalStorageStatus{}, err
	}
	if status.Required && status.Revision != saved.Revision {
		status.State, status.ReasonCode = "pending-observation", "pending_observation"
		status.Message = "The confirmation is saved. Waiting for a fresh background storage observation before new heavy work."
		clearExecutionStorageSamples(&status)
	}
	status.Revision, status.VHDPath, status.ConfirmedAt = saved.Revision, saved.VHDPath, saved.ConfirmedAt
	return status, nil
}

func (g *Gateway) getExecutionStorage(w http.ResponseWriter, r *http.Request) {
	actor, ok := requireAdmin(w, r, true)
	if !ok {
		return
	}
	status, err := g.executionStorageStatus()
	g.mu.Lock()
	defer g.mu.Unlock()
	if !g.executionStorageActorLocked(r, actor.ID) {
		fail(w, 403, "permission_denied", "Your host Super Admin access changed. Sign in again before reviewing host storage.")
		return
	}
	if err != nil {
		fail(w, 409, "storage_confirmation_unavailable", "The private storage confirmation cannot be verified. Existing configuration has been preserved.")
		return
	}
	jsonReply(w, 200, status)
}

func (g *Gateway) confirmExecutionStorage(w http.ResponseWriter, r *http.Request) {
	actor, ok := requireAdmin(w, r, true)
	if !ok {
		return
	}
	var req struct {
		ExpectedRevision             uint64 `json:"expectedRevision"`
		VHDPath                      string `json:"vhdPath"`
		ConfirmActiveAndPendingLocal bool   `json:"confirmActiveAndPendingLocal"`
	}
	if !decode(w, r, &req) {
		return
	}
	if !req.ConfirmActiveAndPendingLocal || req.ExpectedRevision == math.MaxUint64 || !resourcebudget.ValidLocalVHDPath(req.VHDPath) {
		fail(w, 400, "invalid_request", "Review the active and pending Docker Desktop storage locations, explicitly confirm they are local, and provide the existing VHD path and current revision.")
		return
	}
	if g.opts.ExecutionStorage == nil {
		fail(w, 409, "storage_confirmation_unavailable", "This execution backend does not provide local Windows storage confirmation.")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	prepared, probeErr := g.opts.ExecutionStorage.PrepareLocalStorageConfirmation(ctx, req.VHDPath)
	// Acquire the host-wide publication lock before the identity lock; no native
	// probing takes place under either, and workers only read the published file.
	if probeErr != nil {
		g.mu.Lock()
		defer g.mu.Unlock()
		if !g.executionStorageActorLocked(r, actor.ID) {
			fail(w, 403, "permission_denied", "Your host Super Admin access changed. Nothing was saved.")
			return
		}
		fail(w, 409, "storage_confirmation_unavailable", "The current local Docker storage layout could not be verified. Check Docker Desktop and the selected existing VHD; nothing was saved.")
		return
	}
	release, err := lockExecutionStorage(ctx, g.opts.ExecutionStateDir)
	if err != nil {
		fail(w, 500, "save_failed", "The private storage confirmation could not be locked. Nothing was saved.")
		return
	}
	defer release()
	g.mu.Lock()
	defer g.mu.Unlock()
	if !g.executionStorageActorLocked(r, actor.ID) || ctx.Err() != nil {
		fail(w, 403, "permission_denied", "Your host Super Admin access changed or the request expired. Nothing was saved.")
		return
	}
	current, err := resourcebudget.ReadLocalStorageConfirmation(g.executionStoragePath())
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		fail(w, 409, "storage_confirmation_unavailable", "The existing private storage confirmation cannot be verified and has been preserved.")
		return
	}
	if current.Revision != req.ExpectedRevision {
		fail(w, 409, "storage_revision_conflict", "The storage confirmation changed. Reload and review the current settings before confirming again.")
		return
	}
	prepared.Revision, prepared.ConfirmedAt, prepared.ConfirmedBy = current.Revision+1, time.Now().UTC(), actor.ID
	if !prepared.Valid() || !strings.EqualFold(prepared.VHDPath, req.VHDPath) {
		fail(w, 409, "storage_confirmation_unavailable", "The observed storage identity is incomplete. Nothing was saved.")
		return
	}
	// Persist attribution before publication. If the final audit save fails,
	// the requested event and record's ConfirmedBy still explain the saved
	// revision; report the partial outcome and require an explicit fresh read.
	before := g.state
	g.state = cloneIdentityState(before)
	g.auditLocked(actor.ID, "execution-storage.confirm", "host", "requested")
	if g.persistLocked() != nil {
		g.state = before
		fail(w, 500, "save_failed", "Storage confirmation attribution could not be saved. Nothing was changed.")
		return
	}
	data, err := json.Marshal(prepared)
	if err == nil {
		err = writeExecutionStorage(g.executionStoragePath(), data)
	}
	if err != nil {
		fail(w, 500, "save_failed", "The storage confirmation could not be durably saved. Reload settings before retrying.")
		return
	}
	before = g.state
	g.state = cloneIdentityState(before)
	g.auditLocked(actor.ID, "execution-storage.confirm", "host", "succeeded")
	if g.persistLocked() != nil {
		g.state = before
		fail(w, 500, "save_failed", "The confirmation was saved, but its final audit event could not be recorded. Reload settings before any retry.")
		return
	}
	jsonReply(w, 200, resourcebudget.LocalStorageStatus{Platform: "windows", Scope: "local-docker-desktop", Required: true, State: "pending-observation", Revision: prepared.Revision, ReasonCode: "pending_observation", Message: "Confirmation saved. Waiting for a fresh background storage observation before new heavy work.", Basis: resourcebudget.LocalStorageBasis, VHDPath: prepared.VHDPath, ConfirmedAt: prepared.ConfirmedAt, DesktopVersion: prepared.DesktopVersion})
}
