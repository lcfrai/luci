package management

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func dataRequestBody(revision int) map[string]any {
	return map[string]any{"expectedRevision": revision, "acknowledgeDeletion": true}
}
func lifecycleFromBody(t *testing.T, data []byte) (Team, TeamDataLifecycle) {
	t.Helper()
	var value struct {
		Team      Team              `json:"team"`
		Lifecycle TeamDataLifecycle `json:"dataLifecycle"`
	}
	if err := json.Unmarshal(data, &value); err != nil {
		t.Fatal(err)
	}
	return value.Team, value.Lifecycle
}
func TestTeamDataDeletionRequestIsOwnerOnlyReversibleAndHasNoResourceEffects(t *testing.T) {
	g, h, token, owner := fixture(t, nil)
	admin := addUser(t, h, token, "delegate", "member")
	adminToken := loginAs(t, h, admin.Username)
	team := createTestTeam(t, h, token, "Retained team")
	base := "/v1/teams/" + team.ID
	team = teamFromResponse(t, request(t, h, "POST", base+"/members", token, teamMemberBody(team.Revision, admin.Username, "editor", TeamGrants{Admin: true}), 200))
	request(t, h, "GET", base+"/data-lifecycle", adminToken, nil, 403)
	request(t, h, "POST", base+"/data-lifecycle/request", adminToken, dataRequestBody(team.Revision), 403)
	request(t, h, "POST", base+"/data-lifecycle/request", token, map[string]any{"expectedRevision": team.Revision}, 400)
	request(t, h, "POST", base+"/data-lifecycle/request", token, map[string]any{"expectedRevision": team.Revision, "acknowledgeDeletion": true, "status": "completed"}, 400)
	retained := filepath.Join(g.opts.StateDir, "retained-project-data")
	os.WriteFile(retained, []byte("owned data"), 0600)
	before := g.state.Teams[team.ID]
	w := request(t, h, "POST", base+"/data-lifecycle/request", token, dataRequestBody(team.Revision), 200)
	team, lifecycle := lifecycleFromBody(t, w.Body.Bytes())
	if lifecycle.Status != "requested" || lifecycle.RequestedBy != owner.ID || lifecycle.Revision != 1 || !deletionRequestID.MatchString(lifecycle.RequestID) {
		t.Fatal("incorrect private owner request")
	}
	if !reflect.DeepEqual(before.Members, g.state.Teams[team.ID].Members) || !reflect.DeepEqual(before.Projects, g.state.Teams[team.ID].Projects) || before.Service != g.state.Teams[team.ID].Service || len(g.runtimes) != 0 {
		t.Fatal("request changed access, service or workloads")
	}
	data, err := os.ReadFile(retained)
	if err != nil || !bytes.Equal(data, []byte("owned data")) {
		t.Fatal("request erased project bytes")
	}
	request(t, h, "POST", base+"/data-lifecycle/request", token, dataRequestBody(team.Revision), 409)
	request(t, h, "POST", base+"/data-lifecycle/cancel", token, map[string]any{"expectedRevision": team.Revision, "requestId": "delete-wrong"}, 409)
	queue := request(t, h, "GET", "/v1/admin/team-data-requests", token, nil, 200).Body.Bytes()
	if !bytes.Contains(queue, []byte(lifecycle.RequestID)) {
		t.Fatal("operator cannot see pending request")
	}
	request(t, h, "GET", "/v1/admin/team-data-requests", adminToken, nil, 403)
	w = request(t, h, "POST", base+"/data-lifecycle/cancel", token, map[string]any{"expectedRevision": team.Revision, "requestId": lifecycle.RequestID}, 200)
	team, cancelled := lifecycleFromBody(t, w.Body.Bytes())
	if cancelled.Status != "cancelled" || cancelled.RequestID != lifecycle.RequestID || cancelled.CancelledBy != owner.ID || cancelled.Revision != 2 {
		t.Fatal("cancellation lost request attribution")
	}
	queue = request(t, h, "GET", "/v1/admin/team-data-requests", token, nil, 200).Body.Bytes()
	if bytes.Contains(queue, []byte(lifecycle.RequestID)) {
		t.Fatal("cancelled request still eligible for processing")
	}
	reopened, err := New(g.opts)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close(context.Background())
	w = request(t, reopened.Handler(), "GET", base+"/data-lifecycle", token, nil, 200)
	_, saved := lifecycleFromBody(t, w.Body.Bytes())
	if saved != cancelled {
		t.Fatal("cancelled receipt was not durable")
	}
	w = request(t, h, "POST", base+"/data-lifecycle/request", token, dataRequestBody(team.Revision), 200)
	_, next := lifecycleFromBody(t, w.Body.Bytes())
	if next.RequestID == lifecycle.RequestID || next.CancelledBy != "" || next.Revision != 3 {
		t.Fatal("new review request reused cancelled authority")
	}
}

func TestTeamDataDeletionCompareAndSwapRollbackAndForeignOwner(t *testing.T) {
	g, h, token, _ := fixture(t, nil)
	person := addUser(t, h, token, "other-owner", "member")
	personToken := loginAs(t, h, person.Username)
	team := createTestTeam(t, h, token, "Private")
	createTestTeam(t, h, personToken, "Other team")
	base := "/v1/teams/" + team.ID + "/data-lifecycle"
	request(t, h, "POST", base+"/request", personToken, dataRequestBody(team.Revision), 403)
	request(t, h, "POST", base+"/request", token, dataRequestBody(team.Revision+1), 409)
	dir := g.dir
	bad := filepath.Join(t.TempDir(), "file")
	os.WriteFile(bad, []byte("file"), 0600)
	before := cloneIdentityState(g.state)
	g.dir = bad
	request(t, h, "POST", base+"/request", token, dataRequestBody(team.Revision), 500)
	g.dir = dir
	if !reflect.DeepEqual(g.state, before) {
		t.Fatal("failed request write changed lifecycle or audit")
	}
	w := request(t, h, "POST", base+"/request", token, dataRequestBody(team.Revision), 200)
	team, receipt := lifecycleFromBody(t, w.Body.Bytes())
	before = cloneIdentityState(g.state)
	g.dir = bad
	request(t, h, "POST", base+"/cancel", token, map[string]any{"expectedRevision": team.Revision, "requestId": receipt.RequestID}, 500)
	g.dir = dir
	if !reflect.DeepEqual(g.state, before) {
		t.Fatal("failed cancellation write changed active request")
	}
	request(t, h, "POST", base+"/cancel", token, map[string]any{"expectedRevision": team.Revision - 1, "requestId": receipt.RequestID}, 409)
}

func TestTeamDataLifecycleRejectsUnprovenCompletionOrMalformedReceipts(t *testing.T) {
	_, h, token, _ := fixture(t, nil)
	team := createTestTeam(t, h, token, "Team")
	w := request(t, h, "POST", "/v1/teams/"+team.ID+"/data-lifecycle/request", token, dataRequestBody(team.Revision), 200)
	_, valid := lifecycleFromBody(t, w.Body.Bytes())
	for _, change := range []func(*TeamDataLifecycle){func(v *TeamDataLifecycle) { v.Status = "completed" }, func(v *TeamDataLifecycle) { v.RequestID = "fake" }, func(v *TeamDataLifecycle) { v.RequestedBy = "" }, func(v *TeamDataLifecycle) { v.CancelledBy = "somebody" }, func(v *TeamDataLifecycle) { v.UpdatedAt = "invalid" }} {
		invalid := valid
		change(&invalid)
		if validateTeamDataLifecycle(invalid) == nil {
			t.Fatalf("invalid deletion receipt accepted: %+v", invalid)
		}
	}
}
