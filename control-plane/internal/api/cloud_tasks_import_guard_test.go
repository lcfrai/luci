package api

import (
	"strings"
	"testing"

	"github.com/lcfrai/luci/control-plane/internal/backend/e2bbe"
	"github.com/lcfrai/luci/control-plane/internal/product"
)

func TestCloudNoResultGuardUsesActualControllerCaptureAndCharge(t *testing.T) {
	row := e2bbe.Record{Request: e2bbe.Request{Identity: e2bbe.Identity{OwnerID: "owner-a"}},
		RequestDigest: strings.Repeat("a", 64), State: e2bbe.Absent}
	scope := product.ProjectSwarmMicroVMImportScope{OwnerID: "owner-a", RequestDigest: row.RequestDigest,
		CleanupOnly: true, RequireUncaptured: true}
	if !cloudImportRecordAllowed(row, scope) {
		t.Fatal("exact absent uncaptured VM refused")
	}
	for name, changed := range map[string]func(*e2bbe.Record, *product.ProjectSwarmMicroVMImportScope){
		"captured-output": func(r *e2bbe.Record, _ *product.ProjectSwarmMicroVMImportScope) {
			r.Capture.ArchiveSHA256 = strings.Repeat("b", 64)
		},
		"charged":       func(r *e2bbe.Record, _ *product.ProjectSwarmMicroVMImportScope) { r.ResourcesHeld = true },
		"live":          func(r *e2bbe.Record, _ *product.ProjectSwarmMicroVMImportScope) { r.State = e2bbe.Allocated },
		"foreign-owner": func(r *e2bbe.Record, _ *product.ProjectSwarmMicroVMImportScope) { r.Request.Identity.OwnerID = "other" },
		"foreign-request": func(_ *e2bbe.Record, s *product.ProjectSwarmMicroVMImportScope) {
			s.RequestDigest = strings.Repeat("c", 64)
		},
		"source-authority": func(_ *e2bbe.Record, s *product.ProjectSwarmMicroVMImportScope) { s.CleanupOnly = false },
	} {
		t.Run(name, func(t *testing.T) {
			candidate, requested := row, scope
			changed(&candidate, &requested)
			if cloudImportRecordAllowed(candidate, requested) {
				t.Fatal("uncaptured cleanup was authorized without exact settled proof")
			}
		})
	}
}
