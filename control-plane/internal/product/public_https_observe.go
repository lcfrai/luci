package product

import (
	"context"
	"errors"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/deploymentkube"
	"github.com/lcfrai/luci/control-plane/internal/project"
)

func (s *Server) observePublicHTTPS(ctx context.Context, p project.Project, row storedPublicHTTPS) (storedPublicHTTPS, error) {
	if !row.ApplyAttempted {
		return row, nil
	}
	if e := s.publicHTTPSGuard(ctx, row, false, false); e != nil {
		return row, e
	}
	var e error
	row, allReady, e := s.publicHTTPSObservedObjects(ctx, p, row)
	if e != nil {
		return row, e
	}
	if row.RetireAttempted {
		gone := true
		for _, item := range row.Resources {
			if item.Kind != "PersistentVolumeClaim" && item.Attempted && !item.AbsentObserved {
				gone = false
			}
		}
		if row.DNSAttempted && !row.DNSAcknowledged {
			var record publicHTTPSDNSRecord
			if !errors.Is(s.publicHTTPSARMRead(ctx, row, "A", &record), errPublicHTTPSAbsent) {
				return row, ErrPublicHTTPS
			}
			row, e = s.publicHTTPSMutate(p, row.ID, row.Actor, func(r *storedPublicHTTPS) error { r.DNSAbsentObserved = true; return nil })
			if e != nil {
				return row, e
			}
		}
		if row.DNSAcknowledged {
			var record publicHTTPSDNSRecord
			readErr := s.publicHTTPSARMRead(ctx, row, "A", &record)
			if row.DNSDeleteAttempted && errors.Is(readErr, errPublicHTTPSAbsent) {
				row, e = s.publicHTTPSMutate(p, row.ID, row.Actor, func(r *storedPublicHTTPS) error { r.DNSAbsentObserved = true; r.DNSObserved = false; return nil })
				if e != nil {
					return row, e
				}
			} else {
				gone = false
			}
		}
		if !gone {
			return row, nil
		}
		row, e = s.publicHTTPSMutate(p, row.ID, row.Actor, func(r *storedPublicHTTPS) error {
			publicHTTPSUnready(r, "retired", "Original network/controller/configuration/credential resources are retired. Certificate/account state remains in the original PVC for explicit operator recovery; its storage costs may continue. No unrelated resource was removed.")
			r.DNSCredential = PublicHTTPSDNSCredential{}
			r.PVCDataRetained = publicHTTPSResource(r, "PersistentVolumeClaim").Attempted && !publicHTTPSResource(r, "PersistentVolumeClaim").AbsentObserved
			publicHTTPSDelay(r, true)
			return nil
		})
		if e != nil {
			return row, e
		}
		s.publicHTTPSMu.Lock()
		e = s.publicHTTPSDeactivateLocked(p, row)
		s.publicHTTPSMu.Unlock()
		return row, e
	}
	if row.DesiredReplicas == 0 {
		if !allReady {
			return s.publicHTTPSMutate(p, row.ID, row.Actor, func(r *storedPublicHTTPS) error {
				publicHTTPSUnready(r, "provisioning", "Waiting for the original controller's observed replica count to reach zero. Load balancer, DNS and storage costs can continue.")
				publicHTTPSDelay(r, false)
				return nil
			})
		}
		row, e = s.publicHTTPSMutate(p, row.ID, row.Actor, func(r *storedPublicHTTPS) error {
			publicHTTPSUnready(r, "paused", "Original controller scale-to-zero was observed; terminating Pods may finish an in-flight operation. The original load balancer, DNS record and certificate PVC are retained and may continue costing money. Resume or retire explicitly.")
			publicHTTPSDelay(r, true)
			return nil
		})
		return row, e
	}
	if e = s.publicHTTPSDestinationCurrent(ctx, p, row); e != nil {
		return row, e
	}
	if !allReady || row.Address == "" {
		row, e = s.publicHTTPSMutate(p, row.ID, row.Actor, func(r *storedPublicHTTPS) error {
			state := "provisioning"
			if r.Address == "" {
				state = "pending-address"
			}
			publicHTTPSUnready(r, state, "Owned HTTPS resources are pending storage, controller availability or public load-balancer IPv4. A running Pod alone is not HTTPS readiness.")
			publicHTTPSDelay(r, false)
			return nil
		})
		return row, e
	}
	if !row.DNSAttempted {
		row, e = s.publicHTTPSMutate(p, row.ID, row.Actor, func(r *storedPublicHTTPS) error {
			publicHTTPSUnready(r, "pending-dns", "Original public IPv4 is retained. The existing coordinator will create only the reviewed absent Azure A record under current authority; no second consent or blind retry is required.")
			publicHTTPSDelay(r, false)
			return nil
		})
		return row, e
	}
	if !row.DNSAcknowledged {
		return row, ErrPublicHTTPS
	}
	var record publicHTTPSDNSRecord
	if s.publicHTTPSARMRead(ctx, row, "A", &record) != nil || !publicHTTPSDNSMatches(row, record) {
		return row, ErrPublicHTTPS
	}
	probe := probePublicHTTPS
	if s.publicHTTPSProbe != nil {
		probe = s.publicHTTPSProbe
	}
	outcome, probeErr := probe(ctx, row.Plan.Hostname, row.Address, row.Plan.Path)
	if s.publicHTTPSGuard(ctx, row, false, false) != nil {
		return row, ErrPublicHTTPS
	}
	// Repeat the original controlled destination after outbound network I/O.
	if s.publicHTTPSDestinationCurrent(ctx, p, row) != nil {
		return row, ErrPublicHTTPS
	}
	row, e = s.publicHTTPSMutate(p, row.ID, row.Actor, func(r *storedPublicHTTPS) error {
		if r.RetireAttempted || r.DesiredReplicas != 1 || r.Address != row.Address {
			return ErrPublicHTTPS
		}
		r.DNSObserved = outcome.DNSMatched
		r.ObservedAt = time.Now().UTC().Format(time.RFC3339Nano)
		if probeErr != nil || !outcome.HTTPSVerified {
			publicHTTPSUnready(r, "pending-certificate", "Waiting for delegated DNS propagation or native Traefik certificate issuance/renewal. Verify the reviewed Azure DNS credential and public zone delegation; provider bodies/private ACME state are never returned.")
			publicHTTPSDelay(r, false)
			return nil
		}
		expiry, e := time.Parse(time.RFC3339Nano, outcome.CertificateNotAfter)
		if e != nil || !time.Now().Before(expiry) || !hexDigest(outcome.CertificateSHA256) {
			return ErrPublicHTTPS
		}
		r.State = "ready"
		r.HTTPSVerified = true
		r.CertificateNotAfter = outcome.CertificateNotAfter
		r.CertificateSHA256 = outcome.CertificateSHA256
		r.HTTPStatus = outcome.HTTPStatus
		r.URL = "https://" + r.Plan.Hostname + r.Plan.Path
		r.ApplicationHealthVerified = false
		r.Message = "DNS and a trusted SNI certificate were observed at the original owned public IPv4. HTTP response status is recorded; authentication, redirects, 4xx/5xx and application business health remain separate. Native Traefik keeps renewing while this owned controller and credential run, until you pause or retire it."
		publicHTTPSDelay(r, true)
		return nil
	})
	return row, e
}

// One original reviewed DNS intent, never a new controller/order scheduler.
// A false result means no due HTTPS operation; ordinary coordinator work runs.
func (s *Server) continuePublicHTTPS(ctx context.Context) (bool, error) {
	if !s.publicHTTPSWorkMu.TryLock() {
		return false, nil
	}
	defer s.publicHTTPSWorkMu.Unlock()
	s.publicHTTPSMu.Lock()
	_, index, e := s.readPublicHTTPSActive()
	if e != nil {
		s.publicHTTPSMu.Unlock()
		return false, e
	}
	if len(index.Rows) == 0 {
		s.publicHTTPSMu.Unlock()
		return false, nil
	}
	start := s.publicHTTPSCursor % len(index.Rows)
	var selected *storedPublicHTTPS
	var selectedProject project.Project
	for offset := 0; offset < len(index.Rows); offset++ {
		i := (start + offset) % len(index.Rows)
		ref := index.Rows[i]
		p, e := s.resolveProject(ref.ProjectID)
		if e != nil {
			continue
		}
		_, r, e := s.readPublicHTTPS(p, ref.ID)
		if e != nil || r.Plan.Digest != ref.PlanDigest || !r.ApplyAttempted {
			continue
		}
		due, _ := time.Parse(time.RFC3339Nano, r.NextObserveAt)
		if time.Now().Before(due) {
			continue
		}
		s.publicHTTPSCursor = i + 1
		copy := r
		selected = &copy
		selectedProject = p
		break
	}
	s.publicHTTPSMu.Unlock()
	if selected == nil {
		return false, nil
	}
	row := *selected
	fresh, e := s.foundationAuthorityProfile(ctx, row.Actor, false, 0, false, true)
	if e != nil {
		return true, e
	}
	fresh = publicHTTPSRequestContext(fresh)
	// Mark the next sample before slow I/O so one unavailable endpoint cannot
	// starve other original operations or ordinary coding/findings indefinitely.
	row, e = s.publicHTTPSMutate(selectedProject, row.ID, row.Actor, func(r *storedPublicHTTPS) error {
		publicHTTPSDelay(r, r.State == "ready" || r.State == "paused" || r.State == "retired")
		return nil
	})
	if e != nil {
		return true, e
	}
	// Continue only untouched phases of this SAME durable original intent.
	// Each phase repeats current authority/source and has its own before-effect
	// marker. An attempted Create without UID, or attempted DELETE, never retries.
	if row.RetireAttempted {
		e = s.retirePublicHTTPS(fresh, selectedProject, row)
	} else if row.DesiredReplicas == 1 && publicHTTPSHasUnattempted(row) {
		e = s.createPublicHTTPSObjects(fresh, selectedProject, row)
	}
	if e == nil {
		_, row, e = s.readPublicHTTPS(selectedProject, row.ID)
	}
	if e == nil {
		row, e = s.observePublicHTTPS(fresh, selectedProject, row)
	}
	if e == nil && row.State == "pending-dns" && !row.DNSAttempted && row.DesiredReplicas == 1 && !row.RetireAttempted {
		if e = s.publicHTTPSGuard(fresh, row, true, false); e != nil {
			return true, e
		}
		if e = s.publicHTTPSDNSAbsent(fresh, row); e != nil {
			return true, e
		}
		row, e = s.publicHTTPSMutate(selectedProject, row.ID, row.Actor, func(r *storedPublicHTTPS) error {
			if r.DNSAttempted || r.DesiredReplicas != 1 || r.RetireAttempted {
				return ErrPublicHTTPS
			}
			r.DNSAttempted = true
			publicHTTPSUnready(r, "pending-dns", "Original Azure A-record creation was admitted. An unconfirmed response is observed without replay.")
			return nil
		})
		if e == nil {
			e = s.publicHTTPSDNSMutation(fresh, row, false)
		}
		if e == nil {
			_, row, e = s.readPublicHTTPS(selectedProject, row.ID)
			if e == nil {
				row, e = s.observePublicHTTPS(fresh, selectedProject, row)
			}
		}
	}
	if e != nil {
		_, _ = s.publicHTTPSMutate(selectedProject, row.ID, row.Actor, func(r *storedPublicHTTPS) error {
			publicHTTPSUnready(r, "unknown", "An original HTTPS step or current ownership/access could not be confirmed. Existing infrastructure and certificate state may remain and continue costing money. Observe the original; an uncertain creation is never repeated.")
			publicHTTPSDelay(r, true)
			return nil
		})
	}
	if e == nil {
		_, e = s.reconcilePublicHTTPSReceipts(fresh, selectedProject, row)
	}
	return true, e
}
