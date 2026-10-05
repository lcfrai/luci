package platform

import (
	"context"
	"errors"
	"os"
	"strings"
	"time"
)

// This is an explicit, nondestructive recovery of a failed first installation.
// A legacy installation without a receipt is unknown, never presumed empty.
func (s *Service) dataInitializationStatus(v *DataStatus) {
	v.InitializationState = "unknown"
	v.CanRecoverInstall = false
	var install dataInstallation
	if s.read(v.ProjectID+"-data-installation", &install) != nil || install.ProjectID != v.ProjectID || install.StackID == "" {
		return
	}
	if install.InitializationState == "pending" || install.InitializationState == "complete" {
		v.InitializationState = install.InitializationState
	}
	v.CanRecoverInstall = install.InitializationState != "complete" && len(install.Previous) == 0 && len(v.Backups) == 0 && v.Status != "ready" && v.Status != "not-installed"
}

func (s *Service) markDataInitialized(install dataInstallation) error {
	// Status reads open the active receipt while holding this mutex. Keep its
	// validation and atomic replacement in the same critical section: Windows
	// cannot rename over a receipt still open by a concurrent status reader.
	s.mu.Lock()
	defer s.mu.Unlock()
	var active dataInstallation
	if e := s.read(install.ProjectID+"-data-installation", &active); e != nil {
		return e
	}
	if active.ProjectID != install.ProjectID || active.StackID != install.StackID {
		return errors.New("initialization receipt does not match active installation")
	}
	status, e := s.loadData(install.ProjectID)
	if e != nil {
		return e
	}
	if status.Status != "ready" || len(status.Services) != len(dataServiceNames) {
		return errors.New("initialization has no healthy service receipt")
	}
	for _, service := range status.Services {
		if service.Status != "running" || service.Health == "starting" || service.Health == "unhealthy" {
			return errors.New("initialization has an unhealthy service receipt")
		}
	}
	active.InitializationState = "complete"
	return s.write(install.ProjectID+"-data-installation", active)
}

func incompleteLegacyDataBootstrap(logs []byte) bool {
	if len(logs) == 0 || len(logs) > 1<<20 {
		return false
	}
	value := string(logs)
	if !strings.Contains(value, "Success. You can now start the database server") || strings.Contains(value, "PostgreSQL init process complete") {
		return false
	}
	// The old masked assets fail before custom passwords/migrations run.
	// Restrict adoption to this evidenced failure, rather than guessing from
	// a stopped service, missing table or absence of an old state marker.
	for _, line := range strings.Split(value, "\n") {
		if strings.Contains(line, "/docker-entrypoint-initdb.d/init-scripts/98-webhooks.sql") && strings.Contains(strings.ToLower(line), "permission denied") {
			return true
		}
	}
	return false
}

func (s *Service) verifyIncompleteDataInstall(ctx context.Context, previous dataInstallation) error {
	status, e := s.loadData(previous.ProjectID)
	if e != nil {
		return e
	}
	if previous.StackID == "" || previous.InitializationState == "complete" || len(previous.Previous) > 0 || len(status.Backups) > 0 || status.Status == "ready" {
		return &dataFailure{"Fresh-install recovery is only for an incomplete first installation. Successful installations and backups must use their normal start or restore actions."}
	}
	if e = s.verifyDataOwnership(ctx, previous, false); e != nil {
		return e
	}
	for _, name := range dataServiceNames {
		info, e := s.inspect(ctx, previous.ProjectID, "data-"+previous.StackID+"-"+name, s.dataName(previous, name))
		if e != nil {
			return e
		}
		if info.State.Running {
			return &dataFailure{"Stop the incomplete data services before recovering their installation. The original database and files will be retained."}
		}
	}
	if previous.InitializationState != "" && previous.InitializationState != "pending" {
		return errors.New("unknown initialization receipt")
	}
	// A pending marker may survive a crash after a successful bootstrap. It
	// is not proof that the database is empty or incomplete. Both modern and
	// legacy receipts require the narrowly evidenced failed first bootstrap.
	manifest, e := reviewedDataAssets()
	if e != nil {
		return e
	}
	db, e := s.inspect(ctx, previous.ProjectID, "data-"+previous.StackID+"-db", s.dataName(previous, "db"))
	if e != nil || db.Config.Image != manifest.Images["supabase/postgres:17.6.1.136"] {
		return &dataFailure{"This older installation has no verified first-install failure receipt. Its data is retained; use an existing backup or inspect the original installation before recovery."}
	}
	logs, e := s.docker(ctx, "logs", "--tail", "1000", s.dataName(previous, "db"))
	if e != nil || !incompleteLegacyDataBootstrap(logs) {
		return &dataFailure{"This older installation has no verified incomplete bootstrap receipt. Luci will not replace a potentially successful database. Its volumes are retained."}
	}
	return nil
}

// A candidate health probe does not write the active installation or publish
// its address. The old route remains selected until promotion is complete.
func (s *Service) checkDataInstallCandidate(ctx context.Context, candidate dataInstallation, secret dataSecret) error {
	if e := s.verifyDataOwnership(ctx, candidate, false); e != nil {
		return e
	}
	for _, name := range dataServiceNames {
		info, e := s.inspect(ctx, candidate.ProjectID, "data-"+candidate.StackID+"-"+name, s.dataName(candidate, name))
		if e != nil {
			return e
		}
		if !info.State.Running || info.State.Health.Status == "starting" || info.State.Health.Status == "unhealthy" {
			return &dataFailure{"The fresh installation did not pass every service health check. The original stack remains selected and retained."}
		}
	}
	if output, e := s.dataSQL(ctx, candidate, "SELECT 1;"); e != nil || strings.TrimSpace(string(output)) != "1" {
		return &dataFailure{"The fresh database did not pass its connection check. The original stack remains selected and retained."}
	}
	for _, endpoint := range []string{"/auth/v1/health", "/rest/v1/", "/storage/v1/status"} {
		if _, e := s.dataHTTP(ctx, candidate, secret, "GET", endpoint, nil, ""); e != nil {
			return &dataFailure{"The fresh Auth, REST or Storage service did not pass its API check. The original stack remains selected and retained."}
		}
	}
	return nil
}

func (s *Service) recoverDataInstallation(ctx context.Context, id string, previous dataInstallation) (err error) {
	if previous.ProjectID != id {
		return errors.New("incomplete installation binding mismatch")
	}
	if e := s.verifyIncompleteDataInstall(ctx, previous); e != nil {
		return e
	}
	if e := s.dataCapacity(ctx, id, true); e != nil {
		return e
	}
	secret, e := s.dataCredentials(previous)
	if e != nil {
		return e
	}
	next := dataInstallation{ProjectID: id, StackID: randomID("stack-"), StorageTenantID: previous.storageTenant(), InitializationState: "pending", Previous: []string{previous.StackID}}
	next.APIPort, e = freeDataPort()
	if e != nil {
		return e
	}
	if e = s.write(next.StackID+"-secret", secret); e != nil {
		return e
	}
	if e = s.prepareDataStack(next, secret); e != nil {
		return e
	}
	started, promoted, previousDetached, candidateAttached := false, false, false, false
	defer func() {
		if err == nil {
			return
		}
		cleanup, cancel := context.WithTimeout(s.ctx, 5*time.Minute)
		defer cancel()
		candidateStopped := true
		if started {
			for _, name := range dataServiceNames {
				container := s.dataName(next, name)
				info, e := s.inspect(cleanup, id, "data-"+next.StackID+"-"+name, container)
				if e != nil {
					// An unavailable or ownership-mismatched container cannot be
					// claimed stopped and must not be stopped by name alone.
					candidateStopped = false
					continue
				}
				if info.State.Running {
					if _, e = s.docker(cleanup, "stop", "--time", "15", container); e != nil {
						candidateStopped = false
						continue
					}
					info, e = s.inspect(cleanup, id, "data-"+next.StackID+"-"+name, container)
					if e != nil || info.State.Running {
						candidateStopped = false
					}
				}
			}
			var retained []string
			if e := s.read(id+"-data-retained", &retained); e == nil || errors.Is(e, os.ErrNotExist) {
				retained = append(retained, next.StackID)
				_ = s.write(id+"-data-retained", retained)
			}
		}
		selectionRestored := true
		if candidateAttached {
			if e := s.detachDataNetwork(cleanup, next); e != nil {
				selectionRestored = false
			}
		}
		if previousDetached {
			if e := s.attachDataNetwork(cleanup, previous); e != nil {
				selectionRestored = false
			}
		}
		if promoted {
			// Do not start, rewrite or replay SQL against the original partial
			// database. Restore selection and its stable browser address only.
			if e := s.write(id+"-data-installation", previous); e != nil {
				selectionRestored = false
				err = &dataFailure{"Fresh-install recovery failed during activation. Both stacks are retained, but the original selection could not be restored. Inspect owned services before retrying."}
			} else if e := s.refreshData(cleanup, previous, false); e != nil {
				selectionRestored = false
				err = &dataFailure{"Fresh-install recovery failed during activation. Both stacks are retained and the original is selected, but its stopped service state could not be checked."}
			} else if _, e := s.browserDataURL(previous, secret); e != nil {
				selectionRestored = false
				err = &dataFailure{"Fresh-install recovery failed during activation. Both stacks are retained and the original is selected, but its browser connection needs to be checked."}
			}
		}
		if !selectionRestored {
			err = &dataFailure{"Fresh-install recovery failed during activation. Both stacks are retained, but the original application selection or network connection could not be fully reinstated. Inspect owned services before retrying."}
		}
		if !candidateStopped {
			err = &dataFailure{"Fresh-install recovery failed. The original installation and the candidate are retained, but Luci could not confirm that every candidate service stopped. Inspect owned services before retrying."}
		}
		_ = s.changeData(id, func(v *DataStatus) {
			v.Status = "stopped"
			if !selectionRestored || !candidateStopped {
				v.Status = "degraded"
			}
			v.URL = ""
			v.PublishableKey = ""
			v.PriorStackRetained = started
			for key := range v.Capabilities {
				v.Capabilities[key] = false
			}
		})
	}()
	s.dataStage(id, "recovering-install", "Preparing a fresh installation. The original incomplete database and files remain stopped and retained.")
	if e = s.verifyDataOwnership(ctx, next, true); e != nil {
		return e
	}
	if _, e = s.compose(ctx, next, "pull", "--quiet"); e != nil {
		return e
	}
	started = true
	if _, e = s.compose(ctx, next, "up", "-d", "--wait", "--wait-timeout", "240"); e != nil {
		return e
	}
	if e = s.checkDataInstallCandidate(ctx, next, secret); e != nil {
		return e
	}
	s.dataStage(id, "switching", "Fresh services passed their database and API checks. Selecting the healthy installation and retaining the original volumes.")
	// Retire the stopped original aliases before adding the new stack. Keeping
	// both supabase-api/supabase-db aliases can route applications to a stopped
	// container; the original attachments are restored if activation fails.
	previousDetached = true
	if e = s.detachDataNetwork(ctx, previous); e != nil {
		return e
	}
	candidateAttached = true
	if e = s.attachDataNetwork(ctx, next); e != nil {
		return e
	}
	if e = s.write(id+"-data-installation", next); e != nil {
		return e
	}
	promoted = true
	if e = s.refreshData(ctx, next, true); e != nil {
		return e
	}
	if _, e = s.browserDataURL(next, secret); e != nil {
		return e
	}
	if e = s.markDataInitialized(next); e != nil {
		return e
	}
	return s.changeData(id, func(v *DataStatus) {
		v.PriorStackRetained = true
		v.Message = "Fresh data services passed all checks. The original incomplete database and files are stopped and retained; account keys and the application address are preserved."
	})
}
