package product

// Caller holds publicHTTPSMu through the existing durable group-delete marker.
// No remote call or resource deletion occurs here; Pause is not retirement.
func (s *Server) publicHTTPSClusterDeleteGuard(job CloudJob) error {
	if job.Kind != "aks" {
		return nil
	}
	_, index, e := s.readPublicHTTPSActive()
	if e != nil {
		return e
	}
	for _, ref := range index.Rows {
		p, e := s.resolveProject(ref.ProjectID)
		if e != nil {
			return ErrPublicHTTPS
		}
		_, row, e := s.readPublicHTTPS(p, ref.ID)
		if e != nil {
			return ErrPublicHTTPS
		}
		if row.Plan.AzureJobID == job.ID && row.State != "retired" {
			return errPublicHTTPSRetireCluster
		}
	}
	return nil
}
