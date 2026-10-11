package platform

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/previewruntime"
)

func (s *Service) previewRuntime() previewruntime.Runtime {
	if s.opts.PreviewRuntime == nil {
		return previewruntime.Runtime{}
	}
	return s.opts.PreviewRuntime()
}

func (s *Service) storeAdmittedDeployment(before, after Deployment) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, err := s.load(before.ProjectID)
	if err != nil {
		return err
	}
	for i, current := range v.Deployments {
		if current.ID != before.ID {
			continue
		}
		if current.Revision != before.Revision || current.Commit != before.Commit || current.ConfigurationRevision != before.ConfigurationRevision || current.Runtime == nil || before.Runtime == nil || current.Runtime.Identity != before.Runtime.Identity {
			return errors.New("the exact deployment has a newer outcome")
		}
		after.Revision = before.Revision + 1
		v.Deployments[i] = after
		return s.write(before.ProjectID, v)
	}
	return errors.New("the exact deployment receipt is unavailable")
}

func (s *Service) publishDeploymentOutcome(d *Deployment, out previewruntime.Outcome, environment map[string]string) error {
	if d.Runtime == nil || !previewruntime.Continues(d.Runtime.Identity, out.Identity) {
		return errors.New("preview outcome does not match its exact deployment")
	}
	next := *d
	copyOutcome := out
	for _, value := range environment {
		if value != "" {
			copyOutcome.Message = strings.ReplaceAll(copyOutcome.Message, value, "[redacted]")
		}
	}
	next.Runtime = &copyOutcome
	next.Container, next.Image, next.URL = out.ContainerID, out.Image, ""
	next.Message, next.UpdatedAt, next.Status = copyOutcome.Message, now(), "unknown"
	switch out.Phase {
	case "reserved", "building", "built", "creating", "created", "starting", "running":
		next.Status = "building"
	case "finished":
		if out.CleanupConfirmed && !out.ResourcesHeld {
			next.Status = "stopped"
		}
	}
	if err := s.storeAdmittedDeployment(*d, next); err != nil {
		return err
	}
	next.Revision++
	*d = next
	return nil
}

// Preparing an application never creates a network outside admission. A data
// service must already have established its owned network; an application with
// no data binding can use Docker's default network.
func (s *Service) existingDeploymentNetwork(ctx context.Context, projectID string, environment map[string]string) (string, error) {
	needsNetwork := false
	for _, key := range []string{"DATABASE_URL", "SUPABASE_DB_URL", "SUPABASE_URL"} {
		address, err := url.Parse(environment[key])
		if err == nil && (address.Hostname() == "database" || address.Hostname() == "supabase-api" || address.Hostname() == "supabase-db") {
			needsNetwork = true
		}
	}
	if !needsNetwork {
		return "bridge", nil
	}
	name := s.resource(projectID, "net")
	data, err := s.docker(ctx, "network", "inspect", name, "--format", "{{json .}}")
	if err != nil {
		return "", errors.New("the selected database network is unavailable")
	}
	var network struct {
		ID     string            `json:"Id"`
		Driver string            `json:"Driver"`
		Labels map[string]string `json:"Labels"`
	}
	if json.Unmarshal(data, &network) != nil || len(network.ID) != 64 || strings.Trim(network.ID, "0123456789abcdef") != "" || network.Driver != "bridge" || network.Labels[labelInstance] != s.instance || network.Labels[labelProject] != projectID {
		return "", errors.New("database network ownership could not be verified")
	}
	return network.ID, nil
}

func (s *Service) observeAdmittedDeployment(ctx context.Context, d Deployment) (Deployment, error) {
	if err := ctx.Err(); err != nil {
		return d, err
	}
	if d.Runtime == nil || s.previewRuntime().Observe == nil {
		return d, errors.New("shared preview observation is unavailable")
	}
	out, err := s.previewRuntime().Observe(ctx, d.Runtime.Identity)
	if err != nil || out.Identity != d.Runtime.Identity || out.Phase != "running" || !out.ResourcesHeld || out.ContainerID == "" || out.URL == "" {
		return d, errors.New("the exact admitted deployment is not verified running")
	}
	address, parseErr := url.Parse(out.URL)
	if parseErr != nil {
		return d, errors.New("the admitted deployment origin is invalid")
	}
	port, portErr := strconv.Atoi(address.Port())
	if address.Scheme != "http" || address.Hostname() != "127.0.0.1" || address.User != nil || address.Path != "" || address.RawPath != "" || address.RawQuery != "" || address.ForceQuery || address.Fragment != "" || address.Opaque != "" || portErr != nil || port < 1 || port > 65535 {
		return d, errors.New("the admitted deployment origin is not a private loopback origin")
	}
	// The loopback port and historical extra labels are checked below and by
	// every direct browser resolver. Runtime observation proves exact engine,
	// instance, allocation and immutable container ownership first.
	d.Container, d.Image, d.URL = out.ContainerID, out.Image, out.URL
	return d, nil
}

func (s *Service) inspectAdmittedDeployment(ctx context.Context, d Deployment) (containerInfo, error) {
	observed, err := s.observeAdmittedDeployment(ctx, d)
	if err != nil {
		return containerInfo{}, err
	}
	info, err := s.inspect(ctx, d.ProjectID, d.ID, observed.Container)
	if err != nil {
		return containerInfo{}, err
	}
	address, err := loopbackURL(info, d.Port)
	if err != nil || !info.State.Running || info.ID != observed.Container || info.Image != observed.Image || address != observed.URL {
		return containerInfo{}, errors.New("admitted deployment identity or port changed")
	}
	return info, nil
}

func (s *Service) inspectDeploymentForLogs(ctx context.Context, d Deployment) (containerInfo, error) {
	if d.Runtime == nil {
		return s.inspect(ctx, d.ProjectID, d.ID, d.Container)
	}
	runtime := s.previewRuntime()
	if runtime.Observe == nil {
		return containerInfo{}, errors.New("shared preview observation is unavailable")
	}
	out, err := runtime.Observe(ctx, d.Runtime.Identity)
	if err != nil || out.Identity != d.Runtime.Identity || !out.ResourcesHeld || out.ContainerID == "" {
		return containerInfo{}, errors.New("exact retained preview logs cannot be verified")
	}
	info, err := s.inspect(ctx, d.ProjectID, d.ID, out.ContainerID)
	if err != nil || info.ID != out.ContainerID || info.Image != out.Image {
		return containerInfo{}, errors.New("exact retained preview ownership changed")
	}
	return info, nil
}

func (s *Service) waitAdmittedDeploymentHealthy(ctx context.Context, d *Deployment) error {
	bounded, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	for {
		info, err := s.inspectAdmittedDeployment(bounded, *d)
		if err != nil {
			return err
		}
		address, err := loopbackURL(info, d.Port)
		if err != nil {
			return err
		}
		if s.healthy(bounded, address, d.HealthPath) {
			d.URL = address
			return nil
		}
		select {
		case <-bounded.Done():
			return bounded.Err()
		case <-ticker.C:
		}
	}
}

func (s *Service) admittedDeploymentAction(w http.ResponseWriter, r *http.Request, ctx context.Context, d Deployment, action string) {
	runtime := s.previewRuntime()
	var out previewruntime.Outcome
	var err error
	publish := func(value previewruntime.Outcome) error { return s.publishDeploymentOutcome(&d, value, nil) }
	switch action {
	case "stop":
		if d.Runtime.Phase == "finished" && d.Runtime.CleanupConfirmed && !d.Runtime.ResourcesHeld {
			respond(w, 200, s.browserDeployment(d))
			return
		}
		if runtime.Stop == nil {
			problem(w, 503, "preview_runtime_unavailable", "The shared runtime is unavailable; the reservation is retained.")
			return
		}
		out, err = runtime.Stop(ctx, d.Runtime.Identity, publish)
	case "restart":
		if runtime.Restart == nil || runtime.CaptureActor == nil || s.opts.ValidateDeploymentSource == nil || !d.Runtime.ResourcesHeld || d.Runtime.ContainerID == "" {
			problem(w, 409, "redeployment_required", "Create a new admitted deployment from the reviewed commit.")
			return
		}
		actor, captureErr := runtime.CaptureActor(r.Context(), d.ProjectID)
		if captureErr != nil {
			problem(w, 403, "deployment_forbidden", "Current Edit and Deploy access are required.")
			return
		}
		validate := func(verify context.Context) error {
			if err := s.opts.ValidateDeploymentSource(verify, d.ProjectID, d.Commit); err != nil {
				return err
			}
			rev, _, err := s.environment(d.ProjectID)
			if err != nil || rev != d.ConfigurationRevision {
				return errors.New("configuration changed; create a new deployment")
			}
			// Restart can be checked while the exact retained container is
			// stopped. Never require a running observation between Stop/Start.
			info, err := s.inspect(verify, d.ProjectID, d.ID, d.Runtime.ContainerID)
			if err != nil || info.ID != d.Runtime.ContainerID || info.Image != d.Runtime.Image {
				return errors.New("exact retained deployment ownership changed")
			}
			return s.checkDeploymentDatabaseBinding(d.ProjectID, d, info)
		}
		out, err = runtime.Restart(ctx, previewruntime.Request{Identity: d.Runtime.Identity, Actor: actor, ValidateSource: validate}, publish)
	case "refresh":
		if runtime.Observe == nil {
			problem(w, 503, "preview_runtime_unavailable", "Shared preview observation is unavailable.")
			return
		}
		out, err = runtime.Observe(ctx, d.Runtime.Identity)
	}
	if err == nil {
		err = publish(out)
	}
	if err != nil {
		problem(w, 503, "preview_operation_unconfirmed", "The exact operation is unconfirmed. Its receipt and any reserved capacity are retained.")
		return
	}
	if action == "stop" {
		if d.Runtime.ResourcesHeld || !d.Runtime.CleanupConfirmed {
			problem(w, 503, "preview_cleanup_unconfirmed", "Cleanup is unconfirmed; the reservation is retained.")
			return
		}
	} else {
		next := d
		if err := s.waitAdmittedDeploymentHealthy(ctx, &next); err != nil {
			next.Status, next.URL, next.Message = "unknown", "", "The owned deployment did not pass health verification; its reservation is retained."
		} else {
			next.Status, next.Message = "ready", "The exact admitted deployment is healthy."
		}
		if err := s.storeAdmittedDeployment(d, next); err != nil {
			problem(w, 409, "deployment_changed", "A newer outcome owns this deployment receipt.")
			return
		}
		next.Revision++
		d = next
	}
	respond(w, 200, s.browserDeployment(d))
}
