package product

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestResponsiveDefaultAndSavedLegacyPolicies(t *testing.T) {
	p := defaultSwarmSchedulerPolicy()
	if p.MinHeadroomMiB != 512 || p.MaxHeadroomMiB != 1024 || p.HeadroomPercent != 25 || !validateSwarmSchedulerPolicy(p) {
		t.Fatal(p)
	}
	legacy := SwarmSchedulerPolicy{MaxActive: 6, MaxPerProject: 3, HeadroomPercent: 30, MinHeadroomMiB: 2048, DiskReserveMiB: 4096}
	if !validateSwarmSchedulerPolicy(legacy) || legacy.MemoryHeadroomBytes(32<<30) != uint64(32<<30)*30/100 {
		t.Fatal("legacy policy changed", legacy)
	}
	root := t.TempDir()
	s := sharedPolicyServer(t, root, "owner", filepath.Join(root, "shared"))
	if err := s.updateSharedSwarmPolicy(context.Background(), 1, legacy, false); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "shared", "policy.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	got, err := s.readSharedSwarmPolicy()
	if err != nil || !reflect.DeepEqual(got.Policy, legacy) {
		t.Fatal("saved policy silently migrated", got, err)
	}
	after, _ := os.ReadFile(path)
	if string(after) != string(data) {
		t.Fatal("read rewrote saved override")
	}
}

func TestResponsiveReserveFloorAndInvalidCap(t *testing.T) {
	p := defaultSwarmSchedulerPolicy()
	if p.MemoryHeadroomBytes(1<<30) != 512<<20 {
		t.Fatal("floor lost")
	}
	p.MaxHeadroomMiB = 511
	if validateSwarmSchedulerPolicy(p) {
		t.Fatal("cap below floor accepted")
	}
	p.MaxHeadroomMiB = -1
	if validateSwarmSchedulerPolicy(p) {
		t.Fatal("negative cap accepted")
	}
}
