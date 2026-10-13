package main

import (
	"testing"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/backend/e2bbe"
)

func TestCloudActorMonthlyBudgetRetainsOwnedSpendWithoutDoubleCounting(t *testing.T) {
	now := time.Date(2026, 10, 13, 12, 0, 0, 0, time.UTC)
	req := e2bbe.Request{Identity: e2bbe.Identity{OperationID: "op-new"}, BillingUserID: "user-a", BillingTeamID: "team-a", EstimatedCostMicros: 40}
	retired := e2bbe.Record{Request: e2bbe.Request{Identity: e2bbe.Identity{OperationID: "op-old"}, BillingUserID: "user-a", EstimatedCostMicros: 60}, SandboxID: "vm-old", State: e2bbe.Absent, CreatedAt: now}
	if !cloudActorMonthlyCostFits([]e2bbe.Record{retired}, nil, req, now, 100) || cloudActorMonthlyCostFits([]e2bbe.Record{retired}, nil, req, now, 99) {
		t.Fatal("retired owned VM was not charged to the user's monthly budget")
	}
	self := e2bbe.Record{Request: req, ResourcesHeld: true, State: e2bbe.Creating, CreatedAt: now}
	if !cloudActorMonthlyCostFits([]e2bbe.Record{retired, self}, nil, req, now, 100) {
		t.Fatal("journaled current request was counted twice")
	}
	if cloudActorMonthlyCostFits([]e2bbe.Record{retired, self}, nil, e2bbe.Request{Identity: req.Identity, BillingUserID: "user-a", EstimatedCostMicros: 40}, now, 100) {
		t.Fatal("changed billing request reused an allocation")
	}
	peer := e2bbe.Record{Request: e2bbe.Request{Identity: e2bbe.Identity{OperationID: "op-peer"}, BillingUserID: "user-b", EstimatedCostMicros: 90}, ResourcesHeld: true, CreatedAt: now}
	if !cloudActorMonthlyCostFits([]e2bbe.Record{peer}, nil, req, now, 40) {
		t.Fatal("another person's reservation was charged to this actor")
	}
	legacy := e2bbe.Record{Request: e2bbe.Request{EstimatedCostMicros: 1}, ResourcesHeld: true, CreatedAt: now}
	if cloudActorMonthlyCostFits([]e2bbe.Record{legacy}, nil, req, now, 100) {
		t.Fatal("unattributed live spend was ignored")
	}
	noCreate := e2bbe.Record{Request: legacy.Request, State: e2bbe.Absent, CreatedAt: now}
	if !cloudActorMonthlyCostFits([]e2bbe.Record{noCreate}, nil, req, now, 40) {
		t.Fatal("proven no-Create refusal consumed or blocked the budget")
	}
	previous := retired
	previous.CreatedAt = now.AddDate(0, -1, 0)
	if !cloudActorMonthlyCostFits([]e2bbe.Record{previous}, nil, req, now, 40) {
		t.Fatal("settled previous-month spend was charged again")
	}
	previous.ResourcesHeld = true
	if cloudActorMonthlyCostFits([]e2bbe.Record{previous}, nil, req, now, 40) {
		t.Fatal("still-live prior-month VM was ignored")
	}
	settled := []e2bbe.SettledSpend{{OperationID: "op-old", BillingUserID: "user-a", CreatedAt: now, EstimatedCostMicros: 60, SandboxCreated: true}}
	if !cloudActorMonthlyCostFits(nil, settled, req, now, 100) || cloudActorMonthlyCostFits(nil, settled, req, now, 99) {
		t.Fatal("compacted VM escaped the person's monthly limit")
	}
	if cloudActorMonthlyCostFits(nil, settled, retired.Request, now, 100) {
		t.Fatal("compacted operation was accepted as a fresh request")
	}
}
