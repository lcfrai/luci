package api

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/resourcebudget"
)

func TestExecutionStorageSchedulerHTTPPublishesPathFreeHostBound(t *testing.T) {
	f := newSwarmSessionFixtureTasks(t)
	snapshot := executionFixtureSnapshot()
	available := uint64(0) // A measured full volume must survive JSON projection.
	snapshot.Execution.HostStorage = &resourcebudget.HostStorageBound{Required: true, State: "confirmed", Disk: &resourcebudget.Disk{Role: "docker-host-storage", ObservedAt: time.Now().UTC(), AvailableBytes: &available, Basis: resourcebudget.LocalStorageBasis}, MountedVolumeCount: 2}
	f.env.api.deps.ResourceSnapshot = func() resourcebudget.Snapshot { return snapshot }
	w := f.env.do(t, "GET", "/v1/product/swarm-scheduler", "")
	var response struct {
		Resources struct {
			Execution struct {
				HostStorage *resourcebudget.HostStorageBound `json:"hostStorage"`
			} `json:"execution"`
		} `json:"resources"`
	}
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &response) != nil {
		t.Fatal(w.Code, w.Body.String())
	}
	got := response.Resources.Execution.HostStorage
	if got == nil || !got.Required || got.State != "confirmed" || got.MountedVolumeCount != 2 || got.Disk == nil || got.Disk.AvailableBytes == nil || *got.Disk.AvailableBytes != 0 {
		t.Fatal("public host-storage bound was omitted or changed", w.Body.String())
	}
	for _, private := range []string{"vhdPath", "windowsSid", "settingsSha256", "fileIdentity", "volumeGuid"} {
		if strings.Contains(w.Body.String(), private) {
			t.Fatal("private native identity entered scheduler response", private)
		}
	}
	if controlledAllocationCount(f.env.be) != 0 {
		t.Fatal("cached capacity GET allocated execution")
	}
}
