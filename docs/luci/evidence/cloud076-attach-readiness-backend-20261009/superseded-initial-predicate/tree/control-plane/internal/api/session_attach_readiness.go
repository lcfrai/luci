package api

import "github.com/lcfrai/luci/control-plane/internal/registry"

// sessionAttachReady projects the current allocation's recorded endpoint. A
// starting allocation can already accept attach before its health watch finishes;
// a newly recorded allocation whose Create has not returned cannot. This hint
// grants no authority and makes no backend or health observation.
//
// Unknown legacy identity is omitted so old recoverable records keep their
// existing attach behavior. Callers supply this same record's public generation.
func sessionAttachReady(s registry.Session, generation string) *bool {
	if generation == "" {
		return nil
	}
	ready := s.Endpoint != "" && (s.Status == registry.StatusStarting || s.Status == registry.StatusRunning || s.Status == registry.StatusIdle)
	return &ready
}
