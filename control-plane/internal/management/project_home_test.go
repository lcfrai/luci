package management

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestTeamHomeFiltersRowsAndPinsAtResponseTimeAfterRevocation(t *testing.T) {
	f := newTeamProxyFixture(t)
	envelope := `{"version":1,"observedAt":"2026-10-15T10:00:00Z","total":2,"nextOffset":null,"revision":2,"pinnedProjectIds":["p-alpha","p-secret"],"projects":[{"id":"p-alpha","name":"Allowed","working":true,"needsInput":true,"queued":false,"unknown":false,"stops":[]},{"id":"p-secret","name":"Secret","working":true,"needsInput":false,"queued":false,"unknown":false,"stops":[]}]}`
	capture := newTeamCapture()
	capture.WriteHeader(200)
	_, _ = capture.Write([]byte(envelope))
	first := httptest.NewRecorder()
	f.g.filterTeamHome(first, capture, f.team.ID, f.member.ID)
	if first.Code != 200 || !strings.Contains(first.Body.String(), "p-alpha") || strings.Contains(first.Body.String(), "p-secret") {
		t.Fatalf("current grant leaked hidden Home row: %d %s", first.Code, first.Body.String())
	}
	row := f.g.state.Teams[f.team.ID]
	delete(row.Projects["p-alpha"], f.member.ID)
	f.g.state.Teams[f.team.ID] = row
	revoked := httptest.NewRecorder()
	f.g.filterTeamHome(revoked, capture, f.team.ID, f.member.ID)
	if revoked.Code != 200 || strings.Contains(revoked.Body.String(), "p-alpha") || strings.Contains(revoked.Body.String(), "p-secret") {
		t.Fatalf("revoked Home row or pin leaked: %d %s", revoked.Code, revoked.Body.String())
	}
}
