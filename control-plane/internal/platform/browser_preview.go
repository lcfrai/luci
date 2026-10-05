package platform

import "github.com/lcfrai/luci/control-plane/internal/previewgateway"

func (s *Service) browserDeployment(d Deployment) Deployment {
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
	return s.browserRegistry.Publish(previewgateway.Target{Kind: "data", ProjectID: i.ProjectID, ResourceID: i.StackID, Upstream: dataURL(i), Container: s.dataName(i, "api-gw"), Labels: map[string]string{labelInstance: s.instance, labelProject: i.ProjectID, labelResource: "data-" + i.StackID + "-api-gw"}, PublishableKey: secret.Anon})
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
