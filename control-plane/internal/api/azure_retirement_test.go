package api

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/lcfrai/luci/control-plane/internal/management"
)

func TestAzureRetirementCapacityRequiresSettledAuthoritativeReservations(t *testing.T) {
	g := executionTestGate(t, filepath.Join(t.TempDir(), "capacity"))
	actor := management.ExecutionActorRef{Local: true, ProjectID: "retirement-project"}
	if _, err := g.reserve(context.Background(), "retirement-one", actor.ProjectID, "api", 4, actor, management.ExecutionAuthority{}, executionFixturePolicy()); err != nil {
		t.Fatal(err)
	}
	read := func() []byte {
		data, err := os.ReadFile(g.path)
		if err != nil {
			t.Fatal(err)
		}
		return data
	}
	if AzureRetirementLedgerSettled(read()) == nil {
		t.Fatal("held capacity accepted")
	}
	if err := g.release(context.Background(), "retirement-one"); err != nil {
		t.Fatal(err)
	}
	if err := AzureRetirementLedgerSettled(read()); err != nil {
		t.Fatal(err)
	}
	if AzureRetirementLedgerSettled([]byte(`{"version":1,"leases":[],"unknown":true}`)) == nil {
		t.Fatal("unverified capacity schema accepted")
	}
}
