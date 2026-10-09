package management

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestTeamOpaqueExecutionGenerationRemainsProjectScoped(t *testing.T) {
	f := newTeamProxyFixture(t)
	visible, hidden := strings.Repeat("a", 64), strings.Repeat("b", 64)
	f.g.opts.TeamHandler = func(_ context.Context, teamID string) (http.Handler, error) {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			capture := httptest.NewRecorder()
			f.worker(t, teamID, capture, r)
			var body map[string]any
			if capture.Code == http.StatusOK && json.Unmarshal(capture.Body.Bytes(), &body) == nil {
				add := func(row map[string]any) {
					if _, session := row["attachPath"]; session {
						row["executionGeneration"] = strings.Repeat("c", 64)
						if row["id"] == "s-hidden" {
							row["executionGeneration"] = hidden
						}
						if row["id"] == "s-visible" {
							row["executionGeneration"] = visible
						}
					}
				}
				add(body)
				if rows, ok := body["sessions"].([]any); ok {
					for _, row := range rows {
						add(row.(map[string]any))
					}
				}
				jsonReply(w, capture.Code, body)
				return
			}
			w.WriteHeader(capture.Code)
			_, _ = w.Write(capture.Body.Bytes())
		}), nil
	}
	for _, path := range []string{"/v1/sessions", "/v1/sessions/s-visible"} {
		response := request(t, f.h, http.MethodGet, f.path(path), f.memberToken, nil, http.StatusOK)
		if !strings.Contains(response.Body.String(), visible) || strings.Contains(response.Body.String(), hidden) {
			t.Fatalf("execution fingerprint was dropped or crossed project visibility at %s", path)
		}
	}
	refused := request(t, f.h, http.MethodGet, f.path("/v1/sessions/s-hidden"), f.memberToken, nil, http.StatusForbidden)
	if strings.Contains(refused.Body.String(), hidden) || strings.Contains(refused.Body.String(), visible) {
		t.Fatal("refused reader received an execution fingerprint")
	}
	team := f.g.state.Teams[f.team.ID].Team
	request(t, f.h, http.MethodDelete, "/v1/teams/"+team.ID+"/projects/p-alpha/members/"+f.member.ID,
		f.ownerToken, map[string]int{"expectedRevision": team.Revision}, http.StatusOK)
	refused = request(t, f.h, http.MethodGet, f.path("/v1/sessions/s-visible"), f.memberToken, nil, http.StatusForbidden)
	if strings.Contains(refused.Body.String(), visible) {
		t.Fatal("revoked project reader received the former fingerprint")
	}
	list := request(t, f.h, http.MethodGet, f.path("/v1/sessions"), f.memberToken, nil, http.StatusOK)
	if strings.Contains(list.Body.String(), visible) || strings.Contains(list.Body.String(), hidden) {
		t.Fatal("revoked catalogue exposed an execution fingerprint")
	}
}
