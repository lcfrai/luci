package management

import (
	"encoding/json"
	"net/http"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// Seed a saved billing observation, not a provider payment. Tests below drive
// actual authenticated routes and require admission to perform zero Stripe I/O.
func guidanceBillingObservation(t *testing.T, g *Gateway, teamID, status string, until int64) *securityBillingProvider {
	t.Helper()
	p := &securityBillingProvider{}
	g.mu.Lock()
	defer g.mu.Unlock()
	g.managedTeamID = teamID
	g.billing = &teamBillingRuntime{configID: strings.Repeat("a", 64), provider: p, operations: make(chan struct{}, 1)}
	row := g.state.Teams[teamID]
	row.Billing = &TeamBilling{ConfigID: g.billing.configID, Revision: 1, Status: status, PeriodEnd: until}
	if status != "unsubscribed" {
		row.Billing.CustomerID, row.Billing.SubscriptionID = "cus_fixture", "sub_fixture"
	}
	g.state.Teams[teamID] = row
	if err := g.persistLocked(); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestTeamServiceGuidanceDistinguishesSubscriptionRecoveryWithoutDispatch(t *testing.T) {
	for _, tc := range []struct {
		name, status, want string
		member, expired    bool
	}{
		{"owner before subscription", "unsubscribed", "Open the team's Service and data settings", false, false},
		{"owner payment failure", "past_due", "Open the team's Service and data settings", false, false},
		{"owner elapsed receipt", "active", "last verified subscription period has ended", false, true},
		{"member payment failure", "past_due", "Ask a team owner to open Service and data", true, false},
		{"member elapsed trial receipt", "trialing", "Ask a team owner to open Service and data", true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newTeamProxyFixture(t)
			until := time.Now().Add(time.Hour).Unix()
			if tc.expired {
				until = time.Now().Add(-time.Minute).Unix()
			}
			p := guidanceBillingObservation(t, f.g, f.team.ID, tc.status, until)
			before := cloneTeamBilling(f.g.state.Teams[f.team.ID].Billing)
			token := f.ownerToken
			if tc.member {
				token = f.memberToken
			}
			for _, path := range []string{"/v1/product/projects/p-alpha/checks", "/v1/product/projects/p-alpha/releases"} {
				w := request(t, f.h, "POST", f.path(path), token, map[string]any{}, 403)
				var problem map[string]string
				if json.Unmarshal(w.Body.Bytes(), &problem) != nil || problem["error"] != "team_service_suspended" || !strings.Contains(problem["message"], tc.want) || strings.Contains(problem["message"], "Ask the service operator") || strings.Contains(problem["message"], "execution is suspended") {
					t.Fatalf("subscription recovery misidentified the blocker: %s", w.Body.String())
				}
				if tc.expired && !strings.Contains(problem["message"], "last verified subscription period has ended") {
					t.Fatal("elapsed observation was treated as a verified new provider outcome")
				}
				if !strings.Contains(problem["message"], "Existing workloads are not stopped automatically") {
					t.Fatal("guidance did not distinguish denied admission from stopping existing work")
				}
			}
			if f.starts.Load() != 0 || f.writes.Load() != 0 || p.calls != 0 {
				t.Fatal("denial dispatched worker or billing activity")
			}
			service := request(t, f.h, "GET", "/v1/teams/"+f.team.ID+"/service", token, nil, 200)
			if !strings.Contains(service.Body.String(), `"status":"active"`) {
				t.Fatal("subscription guidance silently changed operator admission")
			}
			// Subscription denial retains authorized context and explicit cleanup.
			request(t, f.h, "GET", f.path("/v1/product/projects/p-alpha/brief"), token, nil, 200)
			request(t, f.h, "POST", f.path("/v1/product/projects/p-alpha/releases/release-one/stop"), token, map[string]any{}, 200)
			if !reflect.DeepEqual(before, f.g.state.Teams[f.team.ID].Billing) || p.calls != 0 {
				t.Fatal("guidance or permitted cleanup altered billing authority")
			}
		})
	}
}

func TestTeamServiceGuidanceKeepsOperatorSuspensionAndProjectDenialAuthoritative(t *testing.T) {
	f := newTeamProxyFixture(t)
	p := guidanceBillingObservation(t, f.g, f.team.ID, "past_due", 0)
	request(t, f.h, "PUT", "/v1/admin/team-services/"+f.team.ID, f.ownerToken, serviceBody(0, "managed", "suspended"), 200)
	for _, token := range []string{f.ownerToken, f.memberToken} {
		w := request(t, f.h, "POST", f.path("/v1/product/projects/p-alpha/checks"), token, map[string]any{}, 403)
		if !strings.Contains(w.Body.String(), "Ask the service operator") || strings.Contains(w.Body.String(), "verify it with Stripe") {
			t.Fatal("payment guidance implied that paying could restore operator suspension", w.Body.String())
		}
	}
	w := request(t, f.h, "POST", f.path("/v1/product/projects/p-secret/checks"), f.memberToken, map[string]any{}, 403)
	if !strings.Contains(w.Body.String(), "team_access_denied") || strings.Contains(w.Body.String(), "subscription") {
		t.Fatal("billing guidance replaced the caller's actual missing project authority")
	}
	if f.starts.Load() != 0 || f.writes.Load() != 0 || p.calls != 0 {
		t.Fatal("denial reached worker or payment provider")
	}
}

func TestTeamServiceGuidanceForPreviewDoesNotIssueBrowserAuthority(t *testing.T) {
	var appRequests atomic.Int32
	g, h, token, team, _, origin := teamBrowserFixture(t, http.HandlerFunc(func(http.ResponseWriter, *http.Request) { appRequests.Add(1) }))
	p := guidanceBillingObservation(t, g, team.ID, "active", time.Now().Add(-time.Minute).Unix())
	path := "/v1/teams/" + team.ID + "/projects/p-browser/browser-access"
	w := request(t, h, "POST", path, token, map[string]string{"url": origin}, 403)
	if !strings.Contains(w.Body.String(), "last verified subscription period has ended") || !strings.Contains(w.Body.String(), "Open the team's Service and data settings") {
		t.Fatal("preview handoff did not offer the owner's actual billing recovery path", w.Body.String())
	}
	request(t, h, "PUT", "/v1/admin/team-services/"+team.ID, token, serviceBody(0, "managed", "suspended"), 200)
	w = request(t, h, "POST", path, token, map[string]string{"url": origin}, 403)
	if !strings.Contains(w.Body.String(), "Ask the service operator") || strings.Contains(w.Body.String(), "verify it with Stripe") {
		t.Fatal("preview payment guidance overrode operator suspension")
	}
	if len(g.teamBrowser.tickets) != 0 || len(g.teamBrowser.sessions) != 0 || appRequests.Load() != 0 || p.calls != 0 {
		t.Fatal("denial allocated browser authority or contacted app/Stripe")
	}
}
