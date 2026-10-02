package product

import (
	"context"
	"errors"
	"net/http"
	"time"
)

func (s *Server) ownedCloudRelease(projectID, releaseID string) (CloudRelease, []CloudRelease, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	rows, err := s.readCloudReleases(projectID)
	if err != nil {
		return CloudRelease{}, nil, err
	}
	for _, row := range rows {
		if row.ID == releaseID {
			return row, rows, nil
		}
	}
	return CloudRelease{}, rows, errors.New("this project has no cloud release with that ID")
}

func (s *Server) stopCloudRelease(w http.ResponseWriter, r *http.Request) {
	p, ok := s.requestProject(w, r)
	if !ok {
		return
	}
	if !s.reserveDelivery("cloud-maintenance:" + p.ID) {
		journeyError(w, 409, "release_busy", "Another release operation is in progress.")
		return
	}
	defer s.releaseDelivery("cloud-maintenance:" + p.ID)
	row, rows, err := s.ownedCloudRelease(p.ID, r.PathValue("releaseId"))
	if err != nil {
		journeyError(w, 404, "release_missing", err.Error())
		return
	}
	if row.Status == "stopped" || row.Status == "stopping" {
		journeyJSON(w, 200, row)
		return
	}
	if row.ApplicationID == "" {
		journeyError(w, 409, "application_unconfirmed", "The release has no confirmed application to stop.")
		return
	}
	for _, candidate := range rows {
		if candidate.ApplicationID == row.ApplicationID && candidate.TargetID == row.TargetID {
			if candidate.ID != row.ID {
				journeyError(w, 409, "newer_release_active", "A newer release owns this application. Stop that release instead.")
				return
			}
			break
		}
	}
	if row.Status == "preparing" || row.Status == "queued" || row.Status == "deploying" || row.Status == "verifying" || row.Status == "attention" || row.Status == "unknown" {
		journeyError(w, 409, "release_unsettled", "Refresh or resolve the running deployment before stopping its application.")
		return
	}
	target, err := s.deploymentTarget(row.TargetID)
	if err != nil || targetFingerprint(target) != row.TargetFingerprint {
		journeyError(w, 409, "target_changed", "Reconnect the exact deployment target for this release.")
		return
	}
	client, err := NewCoolifyClient(target.BaseURL, target.Token)
	if err != nil {
		journeyError(w, 400, "target_invalid", err.Error())
		return
	}
	// Persist intent BEFORE a remote mutation. An ambiguous network result is
	// observed through Refresh instead of automatically issuing another stop.
	row.Status, row.URL, row.Message = "stopping", "", "Stop requested. Refresh to verify the application actually stopped."
	if err = s.storeCloudRelease(row); err != nil {
		journeyError(w, 500, "release_record_failed", "Could not save stop intent; no remote stop was attempted.")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 40*time.Second)
	defer cancel()
	if err = client.StopApplication(ctx, row.ApplicationID); err != nil {
		row.Message = "The stop outcome is uncertain. Refresh to observe the application; no automatic retry or cleanup was performed."
		_ = s.storeCloudRelease(row)
		journeyJSON(w, 502, row)
		return
	}
	journeyJSON(w, 202, row)
}

func (s *Server) rollbackCloudRelease(w http.ResponseWriter, r *http.Request) {
	var req struct{}
	if !journeyDecode(w, r, &req) {
		return
	}
	p, ok := s.requestProject(w, r)
	if !ok {
		return
	}
	if !s.reserveDelivery("cloud-maintenance:" + p.ID) {
		journeyError(w, 409, "release_busy", "Another release operation is in progress.")
		return
	}
	defer s.releaseDelivery("cloud-maintenance:" + p.ID)
	original, rows, err := s.ownedCloudRelease(p.ID, r.PathValue("releaseId"))
	if err != nil {
		journeyError(w, 404, "release_missing", err.Error())
		return
	}
	if original.ApplicationID == "" || original.ProviderProjectID == "" {
		journeyError(w, 409, "application_unconfirmed", "The historical release has no confirmed application to rebuild.")
		return
	}
	for _, current := range rows {
		if current.ApplicationID == original.ApplicationID && current.TargetID == original.TargetID {
			if current.RollbackOf == original.ID && current.Status != "failed" && current.Status != "cancelled" && current.Status != "stopped" {
				journeyJSON(w, 202, current)
				return
			}
			switch current.Status {
			case "preparing", "queued", "deploying", "verifying", "unknown", "attention", "stopping":
				journeyError(w, 409, "release_unsettled", "Refresh or resolve the current application operation before rebuilding history.")
				return
			}
			if current.Domain != original.Domain || current.Repository != original.Repository || current.Source != original.Source || current.PrivateKeyUUID != original.PrivateKeyUUID || current.TargetFingerprint != original.TargetFingerprint {
				journeyError(w, 409, "application_changed", "The application's domain, source or target changed; historical settings cannot be applied automatically.")
				return
			}
			break
		}
	}
	target, err := s.deploymentTarget(original.TargetID)
	if err != nil || targetFingerprint(target) != original.TargetFingerprint {
		journeyError(w, 409, "target_changed", "Reconnect the exact deployment target used by the historical release.")
		return
	}
	client, err := NewCoolifyClient(target.BaseURL, target.Token)
	if err != nil {
		journeyError(w, 400, "target_invalid", err.Error())
		return
	}
	row := original
	row.ID = newID("cloud-rel-")
	row.RollbackOf = original.ID
	row.CreatedAt = time.Now().UTC().Format(time.RFC3339Nano)
	row.Status = "preparing"
	row.URL = ""
	row.DeploymentID = ""
	row.Message = "Rebuilding the selected historical commit."
	s.mu.Lock()
	fresh, err := s.readCloudReleases(p.ID)
	if err == nil && len(fresh) < 5000 {
		err = atomicJSON(s.cloudReleaseFile(p.ID), append([]CloudRelease{row}, fresh...))
	} else if err == nil {
		err = errors.New("cloud release history is full")
	}
	s.mu.Unlock()
	if err != nil {
		journeyError(w, 500, "release_record_failed", "Could not reserve the historical rebuild; nothing was deployed.")
		return
	}
	fail := func() {
		row.Status = "attention"
		row.Message = "The historical rebuild outcome is uncertain. Inspect its recorded application before retrying."
		_ = s.storeCloudRelease(row)
		journeyJSON(w, 502, row)
	}
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Minute)
	defer cancel()
	app := CoolifyGitApplication{ProjectUUID: row.ProviderProjectID, ServerUUID: target.ServerUUID, Name: "luci-" + p.ID, Repository: row.Repository, Branch: row.Branch, Commit: row.Release.Commit, Dockerfile: "/" + row.Dockerfile, Port: row.Port, Domain: row.Domain, HealthPath: row.HealthPath, Source: row.Source, PrivateKeyUUID: row.PrivateKeyUUID}
	if err = client.UpdateGitApplication(ctx, row.ApplicationID, app); err != nil {
		fail()
		return
	}
	deployed, err := client.Deploy(ctx, row.ApplicationID)
	if err != nil {
		fail()
		return
	}
	row.DeploymentID, row.Status = deployed.UUID, "queued"
	row.Message = "Historical commit queued. Refresh to verify its commit, DNS, TLS and application health."
	if err = s.storeCloudRelease(row); err != nil {
		fail()
		return
	}
	journeyJSON(w, 202, row)
}
