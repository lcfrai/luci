package product

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lcfrai/luci/control-plane/internal/management"
)

func TestWorkspaceInvitationBridgeAndRedemptionPreserveLocalAccess(t *testing.T) {
	root := t.TempDir()
	gateway, err := management.New(management.Options{StateDir: root, Version: Version, InitialHandler: http.NotFoundHandler()})
	if err != nil {
		t.Fatal(err)
	}
	defer gateway.Close(context.Background())
	s, h, tunnel := workspaceAdminFixture(t, gateway.Handler())
	bootstrap, err := os.ReadFile(filepath.Join(root, "control-plane", "bootstrap-token"))
	if err != nil {
		t.Fatal(err)
	}
	var owner struct {
		Token string          `json:"token"`
		User  management.User `json:"user"`
	}
	if err := remoteWorkspaceAPI(context.Background(), tunnel.APIURL, "POST", "/v1/auth/bootstrap", "", map[string]string{"bootstrapToken": strings.TrimSpace(string(bootstrap)), "username": "remote-owner", "displayName": "Remote Owner", "password": "remote owner password"}, &owner); err != nil {
		t.Fatal(err)
	}
	s.workspaceAuth[workspaceCredentialKey(*tunnel)] = workspaceCredential{Token: owner.Token, User: owner.User}
	base := "/v1/setup/workspace/admin/invitations"
	made := journeyRequest(t, h, "POST", base, map[string]string{"role": "member"}, 201)
	var invitation struct {
		Invitation management.Invitation `json:"invitation"`
		InviteCode string                `json:"inviteCode"`
	}
	if json.Unmarshal(made.Body.Bytes(), &invitation) != nil || invitation.InviteCode == "" {
		t.Fatal(made.Body.String())
	}
	listed := journeyRequest(t, h, "GET", base, nil, 200)
	if !strings.Contains(listed.Body.String(), invitation.Invitation.ID) || strings.Contains(listed.Body.String(), invitation.InviteCode) || strings.Contains(listed.Body.String(), "codeHash") {
		t.Fatal("bridge leaked invitation secret or omitted metadata")
	}
	// A failed backing redemption must remain 409, leaving the separate local
	// account's authenticated Settings/administration surface reachable.
	payload := map[string]string{"connectionId": tunnel.ID, "inviteCode": "invalid-code", "username": "invited-member", "displayName": "Invited Member", "password": "invited member password"}
	journeyRequest(t, h, "POST", "/v1/setup/workspace/auth", payload, 409)
	journeyRequest(t, h, "GET", base, nil, 200)
	journeyRequest(t, h, "DELETE", "/v1/setup/workspace/auth?connectionId="+tunnel.ID, nil, 200)
	payload["inviteCode"] = invitation.InviteCode
	accepted := journeyRequest(t, h, "POST", "/v1/setup/workspace/auth", payload, 200)
	if !strings.Contains(accepted.Body.String(), `"role":"member"`) || strings.Contains(accepted.Body.String(), invitation.InviteCode) || strings.Contains(accepted.Body.String(), `"token"`) {
		t.Fatal(accepted.Body.String())
	}
	if s.workspaceAuth[workspaceCredentialKey(*tunnel)].User.ID == owner.User.ID {
		t.Fatal("invitation overwrote remote owner identity")
	}
	journeyRequest(t, h, "POST", "/v1/setup/workspace/auth", payload, 409)
	// Reauthenticate the existing remote owner; original credentials/workspace
	// survive. Local role is still trusted independently by workspaceAdminActor.
	journeyRequest(t, h, "POST", "/v1/setup/workspace/auth", map[string]string{"connectionId": tunnel.ID, "username": "remote-owner", "password": "remote owner password"}, 200)
	second := journeyRequest(t, h, "POST", base, map[string]string{"role": "member"}, 201)
	json.Unmarshal(second.Body.Bytes(), &invitation)
	journeyRequest(t, h, "DELETE", base+"/"+invitation.Invitation.ID, nil, 200)
	payload["inviteCode"] = invitation.InviteCode
	journeyRequest(t, h, "POST", "/v1/setup/workspace/auth", payload, 409)
	for _, path := range []string{base + "/bad%2Fid", base + "/anything?redirect=elsewhere"} {
		r := httptest.NewRequest("DELETE", path, nil)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code < 400 {
			t.Fatal("noncanonical invitation ID accepted", path)
		}
	}
	journeyRequest(t, h, "POST", base, map[string]string{"role": "member", "targetUrl": "http://elsewhere"}, 400)
	private, _ := os.ReadFile(filepath.Join(s.stateDir, "workspace-auth.json"))
	if strings.Contains(string(private), "invited member password") || strings.Contains(string(private), invitation.InviteCode) {
		t.Fatal("redemption credential persisted")
	}
}
