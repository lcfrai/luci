package panacea

import (
	"cmp"
	"context"
	"encoding/json"
	"net/http"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/deploy"
)

// Task 04: the governance calls. Its writes, like the acts, make one request each; its three
// reads (AutoDeploy, Posture, Access) are GETs, retried by do like every read.

const branchP = actsP + "/branches/feat%2Fx"

// TestSetAutoDeployDecodesPut kills a decoder written for a different PUT shape (review M6),
// and enabled read back as "no explicit label": the PUT's data is the GET's five keys, enabled
// the bool just written (handlers/projects.go:4192-4198, :4194). false is sent, never omitted.
func TestSetAutoDeployDecodesPut(t *testing.T) {
	rs := serveRecorded(t, map[string]fixture{"PUT " + branchP + "/auto-deploy": {file: "autodeploy-set.json"}})
	got, err := rs.c.SetAutoDeploy(context.Background(), project, "feat/x", true)
	if err != nil {
		t.Fatalf("SetAutoDeploy: %v", err)
	}
	if got.Explicit == nil || !*got.Explicit || got.ProjectDefault || !got.Effective || got.ProductionBranch || got.Branch != "feat/x" {
		t.Errorf("SetAutoDeploy = %+v (Explicit %v), want Explicit &true, ProjectDefault false, Effective true, not production", got, got.Explicit)
	}
	oneRequest(t, rs, "PUT "+branchP+"/auto-deploy")
	if b := rs.sent(); len(b) != 1 || b[0] != `{"enabled":true}` {
		t.Errorf("body = %q, want exactly {\"enabled\":true}", b)
	}
	rs = serveRecorded(t, map[string]fixture{"PUT " + branchP + "/auto-deploy": {body: `{"data":{"branch":"feat/x","enabled":false,"projectDefault":true,"effective":false,"isProductionBranch":false}}`}})
	got, err = rs.c.SetAutoDeploy(context.Background(), project, "feat/x", false)
	if err != nil || got.Explicit == nil || *got.Explicit || got.Effective {
		t.Errorf("SetAutoDeploy(false) = (%+v, %v), want Explicit &false, Effective false", got, err)
	}
	if b := rs.sent(); len(b) != 1 || b[0] != `{"enabled":false}` {
		t.Errorf("body = %q, want exactly {\"enabled\":false}: Panacea answers 400 when enabled is absent (projects.go:4165-4167)", b)
	}
}

// TestSetAutoDeployAnswerWithoutTheWrite kills trusting a PUT answer that does not hold the
// value written: Panacea writes *body.Enabled (:4194), so null or the other bool is an answer
// this code does not know, KindUnavailable with the 200 received (re-review M-4).
func TestSetAutoDeployAnswerWithoutTheWrite(t *testing.T) {
	for _, enabled := range []string{"null", "false"} {
		rs := serveRecorded(t, map[string]fixture{"PUT " + branchP + "/auto-deploy": {
			body: `{"data":{"branch":"feat/x","enabled":` + enabled + `,"projectDefault":true,"effective":true,"isProductionBranch":false}}`}})
		_, err := rs.c.SetAutoDeploy(context.Background(), project, "feat/x", true)
		wantKind(t, err, deploy.KindUnavailable, 200, "")
	}
}

// TestAutoDeployReadsNullAsFollowsDefault kills flattening the GET's *bool: null is "no explicit
// label" (handlers/projects.go:4139), which is not "off".
func TestAutoDeployReadsNullAsFollowsDefault(t *testing.T) {
	rs := serveRecorded(t, map[string]fixture{"GET " + branchP + "/auto-deploy": {file: "autodeploy.json"}})
	got, err := rs.c.AutoDeploy(context.Background(), project, "feat/x")
	want := deploy.AutoDeploy{Branch: "feat/x", ProjectDefault: true, Effective: true}
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Errorf("AutoDeploy = (%+v, %v), want %+v", got, err, want)
	}
}

// TestAutoDeployNoEnvironmentIsConflict: the 409 "branch has no environment yet — …" of the GET
// (:4133) and the PUT (:4178) is KindConflict with the sentence, so the lens can say it.
func TestAutoDeployNoEnvironmentIsConflict(t *testing.T) {
	const path = actsP + "/branches/feat%2Fnew/auto-deploy"
	rs := serveRecorded(t, map[string]fixture{
		"GET " + path: {file: "autodeploy-409-noenv.json", status: 409},
		"PUT " + path: {file: "autodeploy-409-noenv.json", status: 409},
	})
	want := fixtureSentence(t, "autodeploy-409-noenv.json")
	_, err := rs.c.AutoDeploy(context.Background(), project, "feat/new")
	wantKind(t, err, deploy.KindConflict, 409, want)
	_, err = rs.c.SetAutoDeploy(context.Background(), project, "feat/new", true)
	wantKind(t, err, deploy.KindConflict, 409, want)
}

// TestPostureMapping: GET B/access's posture fields (handlers/project_security.go:668-677), the
// word through deploy.ParsePostureMode, the allowlist and counts verbatim.
func TestPostureMapping(t *testing.T) {
	rs := serveRecorded(t, map[string]fixture{"GET " + branchP + "/access": {file: "posture.json"}})
	got, err := rs.c.Posture(context.Background(), project, "feat/x")
	want := deploy.Posture{Branch: "feat/x", Environment: "feat-x", Mode: deploy.PosturePassword, ModeRaw: "password",
		ModeKnown: true, PasswordSet: true, BypassTokenCount: 2, IPAllowlist: []string{"10.20.0.0/16", "203.0.113.7/32"}}
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Errorf("Posture = (%+v, %v), want %+v", got, err, want)
	}
}

// notEnforcedExample is notEnforcedSentence's example in task 04's DTO block, byte for byte.
const notEnforcedExample = "Access mode saved as bypass-token for feat/fix(ui), but not yet enforced: the route could not " +
	"be updated, so traffic still meets the previous mode. Re-apply, and do not treat the new mode as active until it succeeds."

// TestPostureNotEnforced kills reporting a saved-but-unapplied posture as done; an internal error
// (reconcileErr, project_security.go:818-821, :837-839) shown to the user (review M7); and a cut
// at the first "(", which the branch's own "(" defeats (re-review M-5). A minted token is still
// carried: the mode was saved with it (:786-797, :830-833).
func TestPostureNotEnforced(t *testing.T) {
	const path = actsP + "/branches/feat%2Ffix%28ui%29/access"
	rs := serveRecorded(t, map[string]fixture{"PUT " + path: {file: "posture-set-not-enforced.json"}})
	got, err := rs.c.SetPosture(context.Background(), project, "feat/fix(ui)", deploy.PostureChange{Mode: deploy.PostureBypassToken})
	if err != nil {
		t.Fatalf("SetPosture: %v", err)
	}
	if got.Enforced {
		t.Error("a saved-but-not-enforced posture read as enforced")
	}
	if got.Message != notEnforcedExample {
		t.Errorf("Message = %q\nwant      %q", got.Message, notEnforcedExample)
	}
	for _, leak := range []string{"10.0.0.9", "retry", "`", "SAVED"} {
		if strings.Contains(got.Message, leak) {
			t.Errorf("Message carries %q: %q", leak, got.Message)
		}
	}
	const token = "pnb_5c2e9a0f7b3d4e1a8c6f2b9d0e4a7c13"
	if !slices.Equal(got.BypassTokens, []string{token}) {
		t.Errorf("BypassTokens = %q, want the minted token", got.BypassTokens)
	}
	wantPosture := deploy.Posture{Branch: "feat/fix(ui)", Environment: "feat-fix(ui)", Mode: deploy.PostureBypassToken,
		ModeRaw: "bypass-token", ModeKnown: true}
	if !reflect.DeepEqual(got.Posture, wantPosture) {
		t.Errorf("Posture = %+v, want %+v", got.Posture, wantPosture)
	}
	oneRequest(t, rs, "PUT "+path)
	var envelope struct{ Message string }
	if err := json.Unmarshal(loadFixture(t, "posture-set-not-enforced.json"), &envelope); err != nil || envelope.Message == "" {
		t.Fatalf("fixture message: %v", err)
	}
	log := rs.log.String()
	if !strings.Contains(log, "level=WARN") || !strings.Contains(log, strconv.Quote(envelope.Message)) {
		t.Errorf("no warn line carries Panacea's sentence %q:\n%s", envelope.Message, log)
	}
	if strings.Contains(log, token) {
		t.Errorf("the log carries the minted token:\n%s", log)
	}
	// Enforced: Panacea's own sentence, which holds no internal error (:835).
	rs = serveRecorded(t, map[string]fixture{"PUT " + branchP + "/access": {file: "posture-set-minted.json"}})
	got, err = rs.c.SetPosture(context.Background(), project, "feat/x", deploy.PostureChange{Mode: deploy.PostureBypassToken, RotateBypassToken: true})
	if err != nil || !got.Enforced || got.Message != "access mode set to bypass-token for engagement-portal@feat/x" {
		t.Errorf("SetPosture(enforced) = (%+v, %v), want Enforced and Panacea's sentence", got, err)
	}
}

// TestPostureMintedTokenReturnedNotLogged kills a token, or a password, in the control plane's
// log. The token is returned once (:830-833); the Debug log of a failed call is checked too.
func TestPostureMintedTokenReturnedNotLogged(t *testing.T) {
	const token, password = "pnb_9d41f07c2a6b3e58d1c0a4f7e2b9c615", "correct-horse-battery"
	rs := serveRecorded(t, map[string]fixture{
		"PUT " + branchP + "/access": {file: "posture-set-minted.json"},
		"PUT " + actsP + "/branches/locked/access": {status: 409,
			body: `{"error":"access mode cannot be relaxed: a stand-in echo of ` + password + ` and ` + token + `"}`},
		// Posture's GET passes noBodyLog too: a failed read's body is not logged either.
		"GET " + actsP + "/branches/locked/access": {status: 404,
			body: `{"error":"a stand-in echo of ` + password + ` and ` + token + `"}`},
	})
	ctx := context.Background()
	got, err := rs.c.SetPosture(ctx, project, "feat/x", deploy.PostureChange{Mode: deploy.PostureBypassToken, RotateBypassToken: true})
	if err != nil || !slices.Equal(got.BypassTokens, []string{token}) {
		t.Fatalf("SetPosture = (%+v, %v), want the minted token", got, err)
	}
	if b := rs.sent(); len(b) != 1 || b[0] != `{"accessMode":"bypass-token","rotateBypassTokens":true}` {
		t.Errorf("body = %q", b)
	}
	_, err = rs.c.SetPosture(ctx, project, "locked", deploy.PostureChange{Mode: deploy.PosturePassword, Password: password})
	if deploy.KindOf(err) != deploy.KindConflict {
		t.Fatalf("a 409 read as %v", err)
	}
	if _, err := rs.c.Posture(ctx, project, "locked"); deploy.KindOf(err) != deploy.KindNotFound {
		t.Fatalf("a 404 read as %v", err)
	}
	log := rs.log.String()
	if n := strings.Count(log, "panacea call failed"); n != 2 {
		t.Fatalf("%d failed calls logged, want the PUT and the GET, so this test proves nothing:\n%s", n, log)
	}
	if !strings.Contains(log, "panacea call failed") {
		t.Fatalf("the failed call was not logged, so this test proves nothing:\n%s", log)
	}
	for _, secret := range []string{token, password} {
		if strings.Contains(log, secret) {
			t.Errorf("the log carries %q:\n%s", secret, log)
		}
	}
}

// TestPasswordOnlyForPasswordMode kills sending a stale password field Panacea might persist, or
// a rotation outside bypass-token: each is sent only with its own mode (project_security.go:681-691).
func TestPasswordOnlyForPasswordMode(t *testing.T) {
	ok := func(mode string) fixture {
		return fixture{body: `{"message":"access mode set to ` + mode + ` for engagement-portal@feat/x","data":{"branch":"feat/x","env":"feat-x","accessMode":"` + mode + `","enforced":true}}`}
	}
	for _, c := range []struct {
		change deploy.PostureChange
		want   string
	}{
		{deploy.PostureChange{Mode: deploy.PostureKPMGSSO, Password: "x"}, `{"accessMode":"kpmg-sso"}`},
		{deploy.PostureChange{Mode: deploy.PostureKPMGSSO, RotateBypassToken: true}, `{"accessMode":"kpmg-sso"}`},
		{deploy.PostureChange{Mode: deploy.PostureBypassToken, Password: "x"}, `{"accessMode":"bypass-token"}`},
		{deploy.PostureChange{Mode: deploy.PosturePassword, Password: "s3cret", RotateBypassToken: true}, `{"accessMode":"password","password":"s3cret"}`},
		{deploy.PostureChange{Mode: deploy.PosturePublic}, `{"accessMode":"none"}`},
	} {
		rs := serveRecorded(t, map[string]fixture{"PUT " + branchP + "/access": ok(string(c.change.Mode))})
		got, err := rs.c.SetPosture(context.Background(), project, "feat/x", c.change)
		if err != nil || got.Posture.Mode != c.change.Mode {
			t.Errorf("SetPosture(%s) = (%+v, %v)", c.change.Mode, got, err)
		}
		if b := rs.sent(); len(b) != 1 || b[0] != c.want {
			t.Errorf("SetPosture(%+v) body = %q, want exactly %s", c.change, b, c.want)
		}
	}
}

// TestSetPostureRefusesUnknownMode: a word outside the five is never sent (Panacea would 400 it,
// project_security.go:718-722); no request is made.
func TestSetPostureRefusesUnknownMode(t *testing.T) {
	rs := serveRecorded(t, nil)
	for _, m := range []deploy.PostureMode{"public", "", "KPMG-SSO"} {
		_, err := rs.c.SetPosture(context.Background(), project, "feat/x", deploy.PostureChange{Mode: m})
		if deploy.KindOf(err) != deploy.KindRejected {
			t.Errorf("SetPosture(%q) = %v, want KindRejected", m, err)
		}
	}
	if got := rs.requests(); len(got) != 0 {
		t.Errorf("an unknown mode reached Panacea: %q", got)
	}
}

// TestSetPostureAnswerFailsClosed kills reading the PUT's answered accessMode as known whatever
// it says (named survivor Y2). The answer's word goes through deploy.ParsePostureMode as the GET's
// does: a word outside the five is (PostureKPMGSSO, ModeKnown false), ModeRaw verbatim, so the
// lens never draws an answer this code does not know as the mode just saved.
func TestSetPostureAnswerFailsClosed(t *testing.T) {
	rs := serveRecorded(t, map[string]fixture{"PUT " + branchP + "/access": {body: `{"message":"access mode set to ` +
		`open-to-all for engagement-portal@feat/x","data":{"branch":"feat/x","env":"feat-x","accessMode":"open-to-all","enforced":true}}`}})
	got, err := rs.c.SetPosture(context.Background(), project, "feat/x", deploy.PostureChange{Mode: deploy.PosturePublic})
	want := deploy.Posture{Branch: "feat/x", Environment: "feat-x", Mode: deploy.PostureKPMGSSO, ModeRaw: "open-to-all"}
	if err != nil || !reflect.DeepEqual(got.Posture, want) {
		t.Errorf("SetPosture = (%+v, %v), want Posture %+v", got, err, want)
	}
}

// TestAccessLegacyGroup kills drawing a group binding as a person, and a legacy test keyed on
// Grantee == "", which Panacea's code never sends (review M30): ListProjectAccess fills a
// person's row with group == grantee and a legacy row with the group's name and its attribute
// (clients/keycloak.go:143-159; clients/keycloak_access_test.go:349-351).
func TestAccessLegacyGroup(t *testing.T) {
	rs := serveRecorded(t, map[string]fixture{"GET " + actsP + "/access": {file: "access.json"}})
	got, err := rs.c.Access(context.Background(), project)
	want := []deploy.AccessGrant{
		{Grantee: "a.person@kpmg.com.au", Group: "a.person@kpmg.com.au", Role: deploy.RoleDeveloper, RoleRaw: "editor"},
		{Grantee: "team-old", Group: "project-legacy-viewer", Legacy: true, Role: deploy.RoleViewer, RoleRaw: "viewer"},
		{Grantee: "b.owner", Group: "b.owner", Role: deploy.RoleOwner, RoleRaw: "owner"},
	}
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Errorf("Access = (%+v, %v)\nwant %+v", got, err, want)
	}
}

// TestAccessRoleWords: models/project_groups.go:9-12 spells developer "editor" (also
// "-developer"); ListProjectAccess writes only owner, editor or viewer (clients/keycloak.go:503-512).
// Any other word is RoleUnknown, kept raw, never a guess.
func TestAccessRoleWords(t *testing.T) {
	rows := `{"items":[{"group":"a","role":"viewer","grantee":"a"},{"group":"b","role":"editor","grantee":"b"},` +
		`{"group":"c","role":"developer","grantee":"c"},{"group":"d","role":"owner","grantee":"d"},` +
		`{"group":"e","role":"admin","grantee":"e"},{"group":"f","role":"","grantee":"f"},{"group":"g","role":"Owner","grantee":"g"}],"totalCount":7}`
	rs := serveRecorded(t, map[string]fixture{"GET " + actsP + "/access": {body: rows}})
	got, err := rs.c.Access(context.Background(), project)
	if err != nil || len(got) != 7 {
		t.Fatalf("Access = (%+v, %v), want 7 rows", got, err)
	}
	want := []deploy.AccessRole{deploy.RoleViewer, deploy.RoleDeveloper, deploy.RoleDeveloper, deploy.RoleOwner,
		deploy.RoleUnknown, deploy.RoleUnknown, deploy.RoleUnknown}
	for i, g := range got {
		if g.Role != want[i] || g.Legacy {
			t.Errorf("row %d (%q) = Role %q Legacy %v, want %q, not legacy", i, g.RoleRaw, g.Role, g.Legacy, want[i])
		}
	}
}

// TestGrantEditorSpelling kills the domain word leaking onto the wire: Panacea builds the group
// "project-<name>-"+role unchecked (clients/keycloak.go:192) and reads any suffix but -owner and
// -editor back as viewer (:503-512), so a "developer" grant would list as a viewer.
func TestGrantEditorSpelling(t *testing.T) {
	for role, wire := range map[deploy.AccessRole]string{
		deploy.RoleDeveloper: "editor", deploy.RoleViewer: "viewer", deploy.RoleOwner: "owner"} {
		rs := serveRecorded(t, map[string]fixture{"POST " + actsP + "/access": {status: 201, body: `{"message":"access binding added"}`}})
		if err := rs.c.Grant(context.Background(), project, deploy.GrantRequest{Grantee: "a.person@kpmg.com.au", Role: role}); err != nil {
			t.Errorf("Grant(%s): %v", role, err)
		}
		if b, want := rs.sent(), `{"group":"a.person@kpmg.com.au","role":"`+wire+`"}`; len(b) != 1 || b[0] != want {
			t.Errorf("Grant(%s) body = %q, want exactly %s", role, b, want)
		}
	}
	rs := serveRecorded(t, nil)
	for _, role := range []deploy.AccessRole{deploy.RoleUnknown, "", "editor", "admin"} {
		err := rs.c.Grant(context.Background(), project, deploy.GrantRequest{Grantee: "a.person@kpmg.com.au", Role: role})
		if deploy.KindOf(err) != deploy.KindRejected {
			t.Errorf("Grant(%q) = %v, want KindRejected before sending", role, err)
		}
	}
	if got := rs.requests(); len(got) != 0 {
		t.Errorf("a role the domain cannot grant reached Panacea: %q", got)
	}
}

// TestGrantUnknownUserIsRejected is plan 7P-12's single parsed sentence. AccessHandler.Add
// answers EVERY AddProjectAccess error as writeError(500, "failed to add access binding: " +
// err.Error()) (handlers/access.go:57). Two of those errors are a typo, not an outage:
// fmt.Errorf("a grantee (username or email) is required") (clients/keycloak.go:182) and
// fmt.Errorf("no Keycloak user found for grantee %q", grantee) (:189). Two share the prefix and
// ARE outages: fmt.Errorf("get admin token: %w", err) (:177) and fmt.Errorf("resolve grantee %q:
// %w", grantee, err) (:186, wrapping "user search returned %d: %s", :988-989). The text after the
// prefix must BEGIN with the first phrase or EQUAL the second; never a contains (re-review M-1).
func TestGrantUnknownUserIsRejected(t *testing.T) {
	const noUser = "No Panacea user matches that name or email."
	for _, c := range []struct {
		name, file, body string
		status           int
		kind             deploy.Kind
		message          string
	}{
		{name: "no-user", file: "grant-500-no-user.json", status: 500, kind: deploy.KindRejected, message: noUser},
		{name: "blank", file: "grant-500-blank.json", status: 500, kind: deploy.KindRejected, message: noUser},
		{name: "resolve", file: "grant-500-resolve.json", status: 500, kind: deploy.KindUnavailable},
		{name: "admin-token", file: "grant-500-admin-token.json", status: 500, kind: deploy.KindUnavailable},
		{name: "other", body: `{"error":"failed to add access binding: add grantee \"x\" to \"project-engagement-portal-editor\": 409"}`,
			status: 500, kind: deploy.KindUnavailable},
		// A contains-match would read this outage as a typo.
		{name: "phrase-inside", body: `{"error":"failed to add access binding: resolve grantee \"x\": no Keycloak user found for grantee \"x\""}`,
			status: 500, kind: deploy.KindUnavailable},
		{name: "no-prefix", body: `{"error":"no Keycloak user found for grantee \"x\""}`, status: 500, kind: deploy.KindUnavailable},
		{name: "blank-and-more", body: `{"error":"failed to add access binding: a grantee (username or email) is required twice"}`,
			status: 500, kind: deploy.KindUnavailable},
		// The phrase on another 5xx is not AccessHandler.Add's answer.
		{name: "502", file: "grant-500-no-user.json", status: 502, kind: deploy.KindUnavailable},
	} {
		t.Run(c.name, func(t *testing.T) {
			rs := serveRecorded(t, map[string]fixture{"POST " + actsP + "/access": {file: c.file, body: c.body, status: c.status}})
			err := rs.c.Grant(context.Background(), project, deploy.GrantRequest{Grantee: "x", Role: deploy.RoleViewer})
			wantKind(t, err, c.kind, c.status, c.message)
			oneRequest(t, rs, "POST "+actsP+"/access")
		})
	}
}

// TestRevokeEscapesEmail kills an unescaped path segment. The group is url.PathEscape'd, as every
// segment is (segment, reads.go): "@" is a pchar (RFC 3986 §3.3) and stays, "/" and " " do not.
// Panacea's mux decodes either form to the same {group} (routes.go:1409).
func TestRevokeEscapesEmail(t *testing.T) {
	rs := serveRecorded(t, map[string]fixture{
		"DELETE " + actsP + "/access/a.b@kpmg.com.au": {body: `{"message":"access binding removed"}`},
		"DELETE " + actsP + "/access/x%2Fy%20z%3F":    {body: `{"message":"access binding removed"}`},
	})
	ctx := context.Background()
	if err := rs.c.Revoke(ctx, project, "a.b@kpmg.com.au"); err != nil {
		t.Errorf("Revoke(email): %v", err)
	}
	if err := rs.c.Revoke(ctx, project, "x/y z?"); err != nil {
		t.Errorf("Revoke(x/y z?): %v", err)
	}
	if err := rs.c.Revoke(ctx, project, ""); deploy.KindOf(err) != deploy.KindRejected {
		t.Errorf("Revoke(\"\") = %v, want KindRejected before sending", err)
	}
	if n := len(rs.requests()); n != 2 {
		t.Errorf("%d requests, want 2", n)
	}
}

// TestRevokeLegacyByGroup kills revoking a legacy row by its grantee: Panacea resolves no user
// "team-old", finds no group of that name, and answers 200 having removed nothing
// (clients/keycloak.go:262-268) (review M30).
func TestRevokeLegacyByGroup(t *testing.T) {
	rs := serveRecorded(t, map[string]fixture{
		"GET " + actsP + "/access":                          {file: "access.json"},
		"DELETE " + actsP + "/access/project-legacy-viewer": {body: `{"message":"access binding removed"}`},
	})
	ctx := context.Background()
	grants, err := rs.c.Access(ctx, project)
	if err != nil || len(grants) != 3 || !grants[1].Legacy {
		t.Fatalf("Access = (%+v, %v), want the legacy row second", grants, err)
	}
	if err := rs.c.Revoke(ctx, project, grants[1].Group); err != nil {
		t.Errorf("Revoke(legacy group): %v", err)
	}
	if got := rs.requests(); slices.Contains(got, "DELETE "+actsP+"/access/team-old") ||
		!slices.Contains(got, "DELETE "+actsP+"/access/project-legacy-viewer") {
		t.Errorf("requests = %q, want the legacy row revoked by its group, never its grantee", got)
	}
}

// TestURLPrefix: the PUT's data {branch, prefix, host} (handlers/projects.go:4365-4371), and its
// 409 "url prefix already taken: …" (:4321) verbatim. Clearing reads its data {branch,
// routeReverted} (:4448-4451); no prefix to clear is success (:4407). The not-reverted answer is
// TestClearURLPrefixRouteNotReverted's.
func TestURLPrefix(t *testing.T) {
	rs := serveRecorded(t, map[string]fixture{
		"PUT " + branchP + "/url-prefix":                    {file: "urlprefix.json"},
		"PUT " + actsP + "/branches/feat%2Fy/url-prefix":    {file: "urlprefix-409.json", status: 409},
		"DELETE " + branchP + "/url-prefix":                 {body: `{"message":"url prefix cleared","data":{"branch":"feat/x","routeReverted":true}}`},
		"DELETE " + actsP + "/branches/feat%2Fy/url-prefix": {body: `{"message":"url prefix cleared"}`, status: 202},
		// Reverted with no branch echoed: Branch is the argument, never the echo (named survivor
		// CS1a, the reverted half of CS1).
		"DELETE " + actsP + "/branches/feat%2Fw/url-prefix": {body: `{"message":"url prefix cleared","data":{"routeReverted":true}}`},
		// A project in a team namespace: Panacea's host is <prefix>.<team>.<domain>
		// (handlers/branch_url_prefix.go:73-81, projects.go:4309, :4318, :4370), so Host is the
		// answer's, never built here from the prefix (named survivor Z2).
		"PUT " + actsP + "/branches/feat%2Fz/url-prefix": {body: `{"message":"url prefix set","data":` +
			`{"branch":"feat/z","prefix":"uat-test","host":"uat-test.audit.panacea.kpmg"}}`},
	})
	ctx := context.Background()
	got, err := rs.c.SetURLPrefix(ctx, project, "feat/x", "uat-test")
	if want := (deploy.URLPrefix{Branch: "feat/x", Prefix: "uat-test", Host: "uat-test.panacea.kpmg"}); err != nil || got != want {
		t.Errorf("SetURLPrefix = (%+v, %v), want %+v", got, err, want)
	}
	if b := rs.sent(); len(b) != 1 || b[0] != `{"prefix":"uat-test"}` {
		t.Errorf("body = %q, want exactly {\"prefix\":\"uat-test\"}", b)
	}
	got, err = rs.c.SetURLPrefix(ctx, project, "feat/z", "uat-test")
	if want := (deploy.URLPrefix{Branch: "feat/z", Prefix: "uat-test", Host: "uat-test.audit.panacea.kpmg"}); err != nil || got != want {
		t.Errorf("SetURLPrefix(team) = (%+v, %v), want %+v", got, err, want)
	}
	_, err = rs.c.SetURLPrefix(ctx, project, "feat/y", "uat-test")
	wantKind(t, err, deploy.KindConflict, 409, fixtureSentence(t, "urlprefix-409.json"))
	// A reverted route: Panacea's own sentence, which holds no internal error (:4443).
	cleared, err := rs.c.ClearURLPrefix(ctx, project, "feat/x")
	if want := (deploy.URLPrefixCleared{Branch: "feat/x", RouteReverted: true, Message: "url prefix cleared"}); err != nil || cleared != want {
		t.Errorf("ClearURLPrefix = (%+v, %v), want %+v", cleared, err, want)
	}
	cleared, err = rs.c.ClearURLPrefix(ctx, project, "feat/w")
	if want := (deploy.URLPrefixCleared{Branch: "feat/w", RouteReverted: true, Message: "url prefix cleared"}); err != nil || cleared != want {
		t.Errorf("ClearURLPrefix(no echo) = (%+v, %v), want %+v", cleared, err, want)
	}
	// The handler answers 200 on success; another 2xx is an answer this code does not know.
	_, err = rs.c.ClearURLPrefix(ctx, project, "feat/y")
	wantKind(t, err, deploy.KindUnavailable, 202, "")
	if n := len(rs.requests()); n != 6 {
		t.Errorf("%d requests, want 6", n)
	}
}

// routeNotRevertedExample is routeNotRevertedSentence("feat/x"), spelled out.
const routeNotRevertedExample = "URL prefix released for feat/x, but the live route is not confirmed reverted: " +
	"the branch may keep serving on its old host until the next deploy or reconcile."

// TestClearURLPrefixRouteNotReverted kills reading a released-but-not-reverted clear as done
// (found t04): ClearBranchURLPrefix releases the claim, then answers 200 with routeReverted false
// when removing the host annotation or the reconcile fails (handlers/projects.go:4427-4451), so
// the branch keeps serving at its old host. Panacea's sentence embeds err.Error() (:4431, :4438,
// :4445), so it goes to warn and Message is the connector's own. An answer without routeReverted
// has not confirmed the revert and reads the same way (fail closed): here one with no data at
// all, so Branch is the branch this call named, never the echo (named survivor CS1).
func TestClearURLPrefixRouteNotReverted(t *testing.T) {
	rs := serveRecorded(t, map[string]fixture{
		"DELETE " + branchP + "/url-prefix":                 {file: "urlprefix-clear-stale.json"},
		"DELETE " + actsP + "/branches/feat%2Fy/url-prefix": {body: `{"message":"url prefix cleared"}`},
	})
	ctx := context.Background()
	got, err := rs.c.ClearURLPrefix(ctx, project, "feat/x")
	if want := (deploy.URLPrefixCleared{Branch: "feat/x", Message: routeNotRevertedExample}); err != nil || got != want {
		t.Errorf("ClearURLPrefix(stale) = (%+v, %v)\nwant %+v", got, err, want)
	}
	for _, leak := range []string{"10.0.0.9", "NOT", "reconciling"} {
		if strings.Contains(got.Message, leak) {
			t.Errorf("Message carries %q: %q", leak, got.Message)
		}
	}
	var envelope struct{ Message string }
	if err := json.Unmarshal(loadFixture(t, "urlprefix-clear-stale.json"), &envelope); err != nil || envelope.Message == "" {
		t.Fatalf("fixture message: %v", err)
	}
	if log := rs.log.String(); !strings.Contains(log, "level=WARN") || !strings.Contains(log, strconv.Quote(envelope.Message)) {
		t.Errorf("no warn line carries Panacea's sentence %q:\n%s", envelope.Message, log)
	}
	got, err = rs.c.ClearURLPrefix(ctx, project, "feat/y")
	want := deploy.URLPrefixCleared{Branch: "feat/y",
		Message: strings.Replace(routeNotRevertedExample, "feat/x", "feat/y", 1)}
	if err != nil || got != want {
		t.Errorf("ClearURLPrefix(no routeReverted) = (%+v, %v)\nwant %+v", got, err, want)
	}
	// The unconfirmed answer is logged too, not only the explicit false (named survivor W1).
	warned := false
	for _, line := range strings.Split(rs.log.String(), "\n") {
		warned = warned || strings.Contains(line, "level=WARN") && strings.Contains(line, " branch=feat/y ")
	}
	if !warned {
		t.Errorf("no warn line for feat/y, whose answer has no routeReverted:\n%s", rs.log.String())
	}
	if n := len(rs.requests()); n != 2 {
		t.Errorf("%d requests, want 2", n)
	}
}

// TestNoWriteIsRetried is the whole file's rule: against a server answering 503 once, then its
// success, each write saw ONE request and failed; each of the three reads was retried by do and
// read its answer (task 02; the part's header calls every method a write, and three are GETs).
func TestNoWriteIsRetried(t *testing.T) {
	type answer struct {
		file, body string
		status     int
	}
	routes := map[string]answer{
		"POST " + actsP + "/deploy":                  {file: "ship-202.json", status: 202},
		"POST " + actsP + "/pipelines/r1/cancel":     {file: "cancel-cancelled.json"},
		"POST " + actsP + "/deployments/d1/rollback": {body: `{"data":{"id":"d1","status":"rolling-back","createdAt":"2026-09-23T11:02:09Z","rollbackWarmth":"warm"}}`},
		"PUT " + branchP + "/auto-deploy":            {file: "autodeploy-set.json"},
		"PUT " + branchP + "/access":                 {file: "posture-set-minted.json"},
		"POST " + actsP + "/access":                  {body: `{"message":"access binding added"}`, status: 201},
		"DELETE " + actsP + "/access/a":              {body: `{"message":"access binding removed"}`},
		"PUT " + branchP + "/url-prefix":             {file: "urlprefix.json"},
		"DELETE " + branchP + "/url-prefix":          {body: `{"message":"url prefix cleared"}`},
		"GET " + branchP + "/auto-deploy":            {file: "autodeploy.json"},
		"GET " + branchP + "/access":                 {file: "posture.json"},
		"GET " + actsP + "/access":                   {file: "access.json"},
	}
	var mu sync.Mutex
	hits := map[string]int{}
	hs := newHarness(t, func(w http.ResponseWriter, r *http.Request) {
		key := r.Method + " " + r.URL.RequestURI()
		mu.Lock()
		hits[key]++
		n := hits[key]
		mu.Unlock()
		a, ok := routes[key]
		if !ok {
			t.Errorf("unexpected request %s", key)
		}
		w.Header().Set("Content-Type", "application/json")
		if n == 1 || !ok {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte(`{"error":"service unavailable"}`))
			return
		}
		body := []byte(a.body)
		if a.file != "" {
			body = loadFixture(t, a.file)
		}
		w.WriteHeader(cmp.Or(a.status, http.StatusOK))
		_, _ = w.Write(body)
	})
	c, ctx := hs.c, context.Background()
	writes := map[string]error{}
	_, writes["POST "+actsP+"/deploy"] = c.Ship(ctx, project, deploy.ShipRequest{Branch: "feat/x", Commit: commit40})
	_, writes["POST "+actsP+"/pipelines/r1/cancel"] = c.Cancel(ctx, project, "r1")
	_, writes["POST "+actsP+"/deployments/d1/rollback"] = c.Rollback(ctx, project, "d1")
	_, writes["PUT "+branchP+"/auto-deploy"] = c.SetAutoDeploy(ctx, project, "feat/x", true)
	_, writes["PUT "+branchP+"/access"] = c.SetPosture(ctx, project, "feat/x", deploy.PostureChange{Mode: deploy.PostureBypassToken})
	writes["POST "+actsP+"/access"] = c.Grant(ctx, project, deploy.GrantRequest{Grantee: "a", Role: deploy.RoleViewer})
	writes["DELETE "+actsP+"/access/a"] = c.Revoke(ctx, project, "a")
	_, writes["PUT "+branchP+"/url-prefix"] = c.SetURLPrefix(ctx, project, "feat/x", "uat-test")
	_, writes["DELETE "+branchP+"/url-prefix"] = c.ClearURLPrefix(ctx, project, "feat/x")
	count := func(key string) int {
		mu.Lock()
		defer mu.Unlock()
		return hits[key]
	}
	for key, err := range writes {
		if n := count(key); n != 1 || deploy.KindOf(err) != deploy.KindUnavailable {
			t.Errorf("%s: %d requests, error %v; want exactly one request and KindUnavailable", key, n, err)
		}
	}
	reads := map[string]error{}
	_, reads["GET "+branchP+"/auto-deploy"] = c.AutoDeploy(ctx, project, "feat/x")
	_, reads["GET "+branchP+"/access"] = c.Posture(ctx, project, "feat/x")
	_, reads["GET "+actsP+"/access"] = c.Access(ctx, project)
	for key, err := range reads {
		if n := count(key); n != 2 || err != nil {
			t.Errorf("%s: %d requests, error %v; want the read retried once and answered", key, n, err)
		}
	}
	if len(writes)+len(reads) != len(routes) {
		t.Errorf("%d routes, %d calls: a method is untested", len(routes), len(writes)+len(reads))
	}
}

// TestCallsAcceptOnlyTheirStatus kills a call that takes any 2xx. Each accepts only the status
// its handler writes on success: Ship 202 (handlers/projects.go:2998), Grant 201
// (handlers/access.go:61), every other 200. Another 2xx is an answer this code does not know:
// KindUnavailable with the status received, one request, never retried.
func TestCallsAcceptOnlyTheirStatus(t *testing.T) {
	ctx := context.Background()
	rollback := `{"data":{"id":"d1","status":"rolling-back","createdAt":"2026-09-23T11:02:09Z","rollbackWarmth":"warm"}}`
	for _, c := range []struct {
		name, key string
		answer    fixture
		call      func(*Client) error
	}{
		{"Ship", "POST " + actsP + "/deploy", fixture{file: "ship-202.json", status: 200}, func(c *Client) error {
			_, err := c.Ship(ctx, project, deploy.ShipRequest{Branch: "feat/x", Commit: commit40})
			return err
		}},
		{"Cancel", "POST " + actsP + "/pipelines/r1/cancel", fixture{file: "cancel-cancelled.json", status: 202}, func(c *Client) error {
			_, err := c.Cancel(ctx, project, "r1")
			return err
		}},
		{"Rollback", "POST " + actsP + "/deployments/d1/rollback", fixture{body: rollback, status: 202}, func(c *Client) error {
			_, err := c.Rollback(ctx, project, "d1")
			return err
		}},
		{"SetAutoDeploy", "PUT " + branchP + "/auto-deploy", fixture{file: "autodeploy-set.json", status: 201}, func(c *Client) error {
			_, err := c.SetAutoDeploy(ctx, project, "feat/x", true)
			return err
		}},
		{"SetPosture", "PUT " + branchP + "/access", fixture{file: "posture-set-minted.json", status: 201}, func(c *Client) error {
			_, err := c.SetPosture(ctx, project, "feat/x", deploy.PostureChange{Mode: deploy.PostureBypassToken})
			return err
		}},
		{"Grant", "POST " + actsP + "/access", fixture{body: `{"message":"access binding added"}`, status: 200}, func(c *Client) error {
			return c.Grant(ctx, project, deploy.GrantRequest{Grantee: "a", Role: deploy.RoleViewer})
		}},
		{"Revoke", "DELETE " + actsP + "/access/a", fixture{body: `{"message":"access binding removed"}`, status: 202}, func(c *Client) error {
			return c.Revoke(ctx, project, "a")
		}},
		{"SetURLPrefix", "PUT " + branchP + "/url-prefix", fixture{file: "urlprefix.json", status: 201}, func(c *Client) error {
			_, err := c.SetURLPrefix(ctx, project, "feat/x", "uat-test")
			return err
		}},
		{"AutoDeploy", "GET " + branchP + "/auto-deploy", fixture{file: "autodeploy.json", status: 203}, func(c *Client) error {
			_, err := c.AutoDeploy(ctx, project, "feat/x")
			return err
		}},
		{"Posture", "GET " + branchP + "/access", fixture{file: "posture.json", status: 203}, func(c *Client) error {
			_, err := c.Posture(ctx, project, "feat/x")
			return err
		}},
		{"Access", "GET " + actsP + "/access", fixture{file: "access.json", status: 203}, func(c *Client) error {
			_, err := c.Access(ctx, project)
			return err
		}},
	} {
		t.Run(c.name, func(t *testing.T) {
			rs := serveRecorded(t, map[string]fixture{c.key: c.answer})
			wantKind(t, c.call(rs.c), deploy.KindUnavailable, c.answer.status, "")
			oneRequest(t, rs, c.key)
		})
	}
}
