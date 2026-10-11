package product

import "github.com/lcfrai/luci/control-plane/internal/previewgateway"

// Keep the verified inner Docker URL in the durable receipt. Only client DTOs
// receive the browser origin, so health checks and private worker bridges retain
// their literal loopback ownership boundary.
func (s *Server) browserRelease(row storedRelease) Release {
	out := row.Release
	// A retained handle permits offering cleanup; the explicit stop handler still
	// verifies current labels and ownership before touching a container.
	out.CanStop = row.Target == "local" && row.Status != "stopped" && previewContainerID.MatchString(row.ContainerID)
	if row.Runtime != nil {
		out.CanStop = row.Target == "local" && row.Runtime.Retained() && row.Runtime.Phase != "finished"
	}
	if s.browserRegistry == nil || row.Target != "local" || row.Status != "ready" {
		return out
	}
	address, err := s.browserRegistry.Publish(previewgateway.Target{Kind: "preview", ProjectID: row.ProjectID, ResourceID: row.ID, Upstream: row.URL, Container: row.ContainerID, Labels: map[string]string{"luci.preview": "true", "luci.release-id": row.ID, "luci.project-id": row.ProjectID}})
	if err != nil {
		out.URL = ""
		out.Message = "The owned preview is healthy, but its browser address could not be registered. Refresh after checking private state permissions."
	} else {
		out.URL = address
	}
	return out
}
