package management

// UNEXECUTED controlled HTTP fixture. This is a server refusal control, not a
// backend defect reproduction, and launches no owned worker or Docker allocation.
import (
    "encoding/json"
    "net/http"
    "strings"
    "testing"
)

func TestCloud056FreshMembershipRemovalReturnsAuthoritativeSessionListDenial(t *testing.T) {
    f := newTeamProxyFixture(t)
    listed := request(t, f.h, http.MethodGet, f.path("/v1/sessions"), f.memberToken, nil, http.StatusOK)
    if !strings.Contains(listed.Body.String(), "s-visible") { t.Fatal("private session precondition absent") }
    starts, writes, attaches := f.starts.Load(), f.writes.Load(), f.attaches.Load()
    f.g.mu.Lock()
    revision := f.g.state.Teams[f.team.ID].Team.Revision
    f.g.mu.Unlock()
    request(t, f.h, http.MethodDelete, "/v1/teams/"+f.team.ID+"/members/"+f.member.ID,
        f.ownerToken, map[string]int{"expectedRevision": revision}, http.StatusOK)
    refused := request(t, f.h, http.MethodGet, f.path("/v1/sessions"), f.memberToken, nil, http.StatusForbidden)
    var problem struct { Error string `json:"error"` }
    if err := json.Unmarshal(refused.Body.Bytes(), &problem); err != nil || problem.Error != "team_access_denied" {
        t.Fatalf("unexpected denial shape: %s", refused.Body.String())
    }
    if strings.Contains(refused.Body.String(), "s-visible") || strings.Contains(refused.Body.String(), "owned context") {
        t.Fatal("denial leaked private session fixture")
    }
    // Account authentication remains valid; this is not a401 logout event.
    request(t, f.h, http.MethodGet, "/v1/auth/me", f.memberToken, nil, http.StatusOK)
    teams := request(t, f.h, http.MethodGet, "/v1/teams", f.memberToken, nil, http.StatusOK)
    if strings.Contains(teams.Body.String(), f.team.ID) { t.Fatal("removed membership still listed") }
    if f.starts.Load()!=starts || f.writes.Load()!=writes || f.attaches.Load()!=attaches {
        t.Fatal("refusal control unexpectedly mutated or attached to worker")
    }
}
