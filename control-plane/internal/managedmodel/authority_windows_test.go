//go:build windows

package managedmodel

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func authorityFixture(t *testing.T) (string, Observation) {
	t.Helper()
	prior := configured.Load()
	configured.Store(nil)
	t.Cleanup(func() { configured.Store(prior) })
	path := filepath.Join(t.TempDir(), "status.json")
	boot := strings.Repeat("a", 64)
	if err := Configure(path, boot); err != nil {
		t.Fatal(err)
	}
	o := Observation{Version: 1, Boot: boot, Revision: 3, Status: Status{State: "available", Generation: "7", ObservedAt: time.Now().UTC(), Endpoint: "http://127.0.0.1:24567/v1", Model: "installed:latest"}}
	writeAuthority(t, path, o)
	return path, o
}
func writeAuthority(t *testing.T, path string, o Observation) {
	t.Helper()
	b, e := json.Marshal(o)
	if e != nil {
		t.Fatal(e)
	}
	if e = os.WriteFile(path, b, 0600); e != nil {
		t.Fatal(e)
	}
}
func TestManagedReferenceRequiresCurrentParentGeneration(t *testing.T) {
	path, original := authorityFixture(t)
	for _, host := range []string{"127.0.0.1", "localhost", "host.docker.internal"} {
		if _, e := Check(&Reference{3}, "http://"+host+":24567/v1", "installed:latest"); e != nil {
			t.Fatal(e)
		}
	}
	cases := []struct {
		name   string
		change func(*Observation)
	}{
		{"boot", func(o *Observation) { o.Boot = strings.Repeat("b", 64) }}, {"revision", func(o *Observation) { o.Revision++ }},
		{"withdrawn", func(o *Observation) { o.Withdrawing = true }}, {"stopped", func(o *Observation) { o.Status.State = "stopped" }},
		{"unknown", func(o *Observation) { o.Status.State = "unknown" }}, {"old", func(o *Observation) { o.Status.ObservedAt = time.Now().Add(-6 * time.Second) }},
		{"future", func(o *Observation) { o.Status.ObservedAt = time.Now().Add(time.Second) }}, {"noncanonical-generation", func(o *Observation) { o.Status.Generation = "07" }},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			o := original
			c.change(&o)
			writeAuthority(t, path, o)
			if _, e := Check(&Reference{3}, original.Status.Endpoint, original.Status.Model); e == nil {
				t.Fatal("invalid authority admitted")
			}
		})
	}
	writeAuthority(t, path, original)
	for _, target := range []struct{ endpoint, model string }{{"http://127.0.0.1:24568/v1", original.Status.Model}, {original.Status.Endpoint, "other:latest"}, {"http://example.invalid:24567/v1", original.Status.Model}, {original.Status.Endpoint + "?different=1", original.Status.Model}} {
		if _, e := Check(&Reference{3}, target.endpoint, target.model); e == nil {
			t.Fatal("different selection admitted")
		}
	}
}
func TestManagedWithdrawalWaitsForExistingAllocationAndBlocksNewOnes(t *testing.T) {
	path, o := authorityFixture(t)
	release, _, e := Hold(context.Background(), &Reference{3}, o.Status.Endpoint, o.Status.Model)
	if e != nil {
		t.Fatal(e)
	}
	o.Withdrawing = true
	writeAuthority(t, path, o)
	timeout, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	if unlock, e := WithdrawBarrier(timeout, path); e == nil {
		unlock()
		t.Fatal("withdrawal crossed existing allocation")
	}
	release()
	timeout2, cancel2 := context.WithTimeout(context.Background(), time.Second)
	defer cancel2()
	unlock, e := WithdrawBarrier(timeout2, path)
	if e != nil {
		t.Fatal(e)
	}
	unlock()
	if release, _, e := Hold(context.Background(), &Reference{3}, o.Status.Endpoint, o.Status.Model); e == nil {
		release()
		t.Fatal("withdrawn generation accepted a new allocation")
	}
}
func TestUnmanagedReferenceDoesNotReadOrLockProviderState(t *testing.T) {
	prior := configured.Load()
	configured.Store(nil)
	defer configured.Store(prior)
	release, o, e := Hold(context.Background(), nil, "https://provider.invalid/v1", "remote")
	if e != nil || o.Boot != "" || release == nil {
		t.Fatal("BYO route changed")
	}
	release()
	if _, e := Check(&Reference{1}, "http://127.0.0.1:24567/v1", "local"); e == nil {
		t.Fatal("unbound managed route admitted")
	}
}
