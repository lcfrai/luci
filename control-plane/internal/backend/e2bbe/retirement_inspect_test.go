package e2bbe

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestAzureRetirementInspectionPreservesHeldAttemptUntilExactAbsence(t *testing.T) {
	p := newProvider(t)
	cfg, _ := fixtureConfig(t, p)
	c := openController(t, cfg)
	record, err := c.Allocate(context.Background(), taskRequest("retirement-project", "one"))
	if err != nil {
		t.Fatal(err)
	}
	read := func() []byte {
		data, err := os.ReadFile(filepath.Join(cfg.StateDir, "attempts.json"))
		if err != nil {
			t.Fatal(err)
		}
		return data
	}
	if InspectSettledJournal(read(), cfg.OwnerID, cfg.Deployment.Scope) == nil {
		t.Fatal("held microVM accepted for infrastructure retirement")
	}
	if _, err := c.Cancel(context.Background(), record.Request.Identity.OperationID, record.Revision); err != nil {
		t.Fatal(err)
	}
	if InspectSettledJournal(read(), cfg.OwnerID, cfg.Deployment.Scope) != nil {
		t.Fatal("exact settled absence refused")
	}
	if InspectSettledJournal(read(), "another-owner", cfg.Deployment.Scope) == nil {
		t.Fatal("foreign ownership accepted")
	}
	var changed map[string]any
	_ = json.Unmarshal(read(), &changed)
	changed["unexpected"] = true
	data, _ := json.Marshal(changed)
	if InspectSettledJournal(data, cfg.OwnerID, cfg.Deployment.Scope) == nil {
		t.Fatal("unverified journal schema accepted")
	}
}
