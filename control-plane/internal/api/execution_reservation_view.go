package api

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/hostpath"
)

const executionReservationReadTimeout = 100 * time.Millisecond

// An unavailable observation deliberately has no numeric totals. Reservations
// are shared across API processes; a process-local mutation cache cannot report
// another user's or team's current allocations.
type executionReservationObservation struct {
	Status string `json:"status"`
	*executionReservationView
}

func (g *executionAdmission) reservationObservation(ctx context.Context) executionReservationObservation {
	unavailable := executionReservationObservation{Status: "unavailable"}
	if g == nil || ctx.Err() != nil {
		return unavailable
	}
	ctx, cancel := context.WithTimeout(ctx, executionReservationReadTimeout)
	defer cancel()
	ledger, err := g.readReservationLedger(ctx)
	if err != nil || ctx.Err() != nil {
		return unavailable
	}
	view := &executionReservationView{ObservedAt: time.Now().UTC(), Count: len(ledger.Leases)}
	for _, lease := range ledger.Leases {
		view.MemoryMiB += lease.Resources.MemoryMiB
		view.CPUMillis += lease.Resources.CPUMillis
	}
	for _, check := range ledger.Checks {
		if check.State == "finished" {
			continue
		}
		view.Count++
		view.MemoryMiB += check.Resources.MemoryMiB
		view.CPUMillis += check.Resources.CPUMillis
	}
	addPreviewReservationView(view, ledger.Previews)
	addRuntimeImageReservationView(view, ledger.RuntimeImages)
	return executionReservationObservation{Status: "ready", executionReservationView: view}
}

// Read the existing ledger under the same cross-process lock as admission.
// Unlike mutate, this creates no directories or lock files, changes no modes,
// rewrites no receipts, and never reconciles or calls an execution backend.
// An absent, unsafe, oversized or invalid ledger is unknown, not zero demand.
func (g *executionAdmission) readReservationLedger(ctx context.Context) (executionLeaseLedger, error) {
	empty := executionLeaseLedger{}
	if err := ctx.Err(); err != nil {
		return empty, err
	}
	if !filepath.IsAbs(g.path) {
		return empty, errors.New("execution state path is not absolute")
	}
	lockPath := filepath.Join(filepath.Dir(g.path), "leases.lock")
	if err := codexRegularPath(filepath.VolumeName(lockPath)+string(filepath.Separator), lockPath); err != nil {
		return empty, err
	}
	if _, err := hostpath.ResolveOutside(g.api.projectsDir(), lockPath); err != nil {
		return empty, err
	}
	info, err := os.Lstat(lockPath)
	if err != nil || !info.Mode().IsRegular() {
		return empty, errors.New("execution lock unavailable")
	}
	file, err := os.Open(lockPath)
	if err != nil {
		return empty, err
	}
	defer file.Close()
	opened, err := file.Stat()
	if err != nil || !os.SameFile(info, opened) {
		return empty, errors.New("execution lock identity changed")
	}
	release, err := lockExecutionLedger(ctx, file)
	if err != nil {
		return empty, err
	}
	defer release()
	// Fail closed if a private state owner changed the lock path while we waited.
	if err := codexRegularPath(filepath.VolumeName(lockPath)+string(filepath.Separator), lockPath); err != nil {
		return empty, err
	}
	current, err := os.Lstat(lockPath)
	if err != nil || !current.Mode().IsRegular() || !os.SameFile(current, opened) {
		return empty, errors.New("execution lock identity changed")
	}
	if err := ctx.Err(); err != nil {
		return empty, err
	}
	// The existing reader confines ordinary files, enforces the 2 MiB bound,
	// validates every scope's ownership fields and decodes the entire ledger.
	return g.readFenceLedger()
}

// Retained unknown preview builders and servers stay visible in the same host totals.
func addPreviewReservationView(view *executionReservationView, rows []previewExecutionLease) {
	for _, row := range rows {
		if row.State == "finished" {
			continue
		}
		view.Count++
		view.MemoryMiB += row.Resources.MemoryMiB
		view.CPUMillis += row.Resources.CPUMillis
	}
}
