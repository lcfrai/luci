package platform

import (
	"context"
	"errors"
	"os"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/previewgateway"
)

func (s *Service) browserDeployment(d Deployment) Deployment {
	ctx, cancel := context.WithTimeout(s.ctx, 5*time.Second)
	defer cancel()
	return s.browserDeploymentContext(ctx, d)
}

func (s *Service) browserDeploymentContext(ctx context.Context, d Deployment) Deployment {
	if d.Runtime != nil {
		public := *d.Runtime
		public.URL = ""
		d.Runtime = &public
	}
	if d.Runtime != nil {
		if d.Provider != "docker" || d.Status != "ready" {
			d.URL = ""
			return d
		}
		observed, err := s.observeAdmittedDeployment(ctx, d)
		if err != nil {
			d.URL = ""
			d.Message = "The exact application allocation could not be verified. Its retained resources are unchanged."
			return d
		}
		d = observed
	}
	if s.browserRegistry == nil || d.Provider != "docker" || d.Status != "ready" {
		return d
	}
	address, err := s.browserRegistry.Publish(previewgateway.Target{Kind: "application", ProjectID: d.ProjectID, ResourceID: d.ID, Upstream: d.URL, Container: d.Container, Labels: map[string]string{labelInstance: s.instance, labelProject: d.ProjectID, labelResource: d.ID}})
	if err != nil {
		d.URL = ""
		d.Message = "Application health passed, but its browser address could not be registered."
	} else {
		d.URL = address
	}
	return d
}
func (s *Service) browserDataURL(i dataInstallation, secret dataSecret) (string, error) {
	if s.browserRegistry == nil {
		return dataURL(i), nil
	}
	// Applications retain their client endpoint across a fresh-volume restore.
	// The durable data identity selects the browser origin; the mutable target
	// still verifies the active stack's exact container, labels and loopback port.
	return s.browserRegistry.Publish(previewgateway.Target{Kind: "data", ProjectID: i.ProjectID, ResourceID: i.storageTenant(), Upstream: dataURL(i), Container: s.dataName(i, "api-gw"), Labels: map[string]string{labelInstance: s.instance, labelProject: i.ProjectID, labelResource: "data-" + i.StackID + "-api-gw"}, PublishableKey: secret.Anon})
}

// A restore candidate needs the durable client address in its configuration,
// but it must not replace the gateway target before healthy active promotion.
func (s *Service) browserDataConfigurationURL(candidate dataInstallation, secret dataSecret) (string, error) {
	if s.browserRegistry == nil {
		return dataURL(candidate), nil
	}
	var active dataInstallation
	err := s.read(candidate.ProjectID+"-data-installation", &active)
	if errors.Is(err, os.ErrNotExist) {
		// The first installation can reserve its address before it is started.
		return s.browserDataURL(candidate, secret)
	}
	if err != nil {
		return "", err
	}
	if active.ProjectID != candidate.ProjectID || active.storageTenant() != candidate.storageTenant() {
		return "", errors.New("private data installation binding mismatch")
	}
	activeSecret, err := s.dataCredentials(active)
	if err != nil {
		return "", err
	}
	return s.browserDataURL(active, activeSecret)
}
func (s *Service) browserDataStatus(v DataStatus) DataStatus {
	if s.browserRegistry == nil || v.URL == "" {
		return v
	}
	var i dataInstallation
	if err := s.read(v.ProjectID+"-data-installation", &i); err != nil || i.ProjectID != v.ProjectID {
		v.URL = ""
		return v
	}
	secret, err := s.dataCredentials(i)
	if err != nil {
		v.URL = ""
		return v
	}
	address, err := s.browserDataURL(i, secret)
	if err != nil {
		v.URL = ""
		return v
	}
	v.URL = address
	v.EndpointScope = "browser-gateway"
	return v
}
