package panacea

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/deploy"
)

// Task 04: deploy.Governor. Its six writes are sent once, never retried (task 02); its three
// reads (AutoDeploy, Posture, Access) are GETs and are retried like every read.
//
// Never logged, anywhere in this file: PostureChange.Password, any bypassTokens value, the
// credential. do never logs a request body or a 2xx body; Posture and SetPosture also pass
// noBodyLog, so not even a failed call's error body reaches the Debug line.

var _ deploy.Governor = (*Client)(nil)

// branchPath is B/<leaf>: P + "/branches/" + url.PathEscape(branch) + "/" + leaf.
func branchPath(op string, p deploy.ProjectRef, branch, leaf string) (string, error) {
	P, err := projectPath(op, p)
	if err != nil {
		return "", err
	}
	b, err := segment(op, "branch", branch)
	if err != nil {
		return "", err
	}
	return P + "/branches/" + b + "/" + leaf, nil
}

// autoDeployOf maps the five keys. Branch is the branch this call named: Panacea echoes the path
// value (handlers/projects.go:4138, :4193), so its echo is not a second source.
func autoDeployOf(branch string, d autoDeployDTO) deploy.AutoDeploy {
	return deploy.AutoDeploy{Branch: branch, Explicit: d.Enabled, ProjectDefault: d.ProjectDefault,
		Effective: d.Effective, ProductionBranch: d.IsProductionBranch}
}

// AutoDeploy reads GET B/auto-deploy (routes.go:1308), a dataDTO[autoDeployDTO]
// (handlers/projects.go:4136-4143). enabled null is Explicit nil: the branch follows the project
// default. Its 409 "branch has no environment yet — …" (:4133) is KindConflict with the sentence.
func (c *Client) AutoDeploy(ctx context.Context, p deploy.ProjectRef, branch string) (deploy.AutoDeploy, error) {
	const op = "autoDeploy"
	path, err := branchPath(op, p, branch, "auto-deploy")
	if err != nil {
		return deploy.AutoDeploy{}, err
	}
	var dto dataDTO[autoDeployDTO]
	if err := c.get(ctx, op, path, &dto); err != nil {
		return deploy.AutoDeploy{}, err
	}
	return autoDeployOf(branch, dto.Data), nil
}

// SetAutoDeploy puts {"enabled": <bool>} to B/auto-deploy (routes.go:1309); false is sent, never
// omitted (Panacea answers 400 without it, handlers/projects.go:4165-4167). The answer is the
// GET's five keys with enabled the bool just written (:4192-4198, :4194), so one whose enabled is
// null or the other bool is an answer this code does not know: KindUnavailable with the 200
// received. Its 409 (:4178) is KindConflict with the sentence.
func (c *Client) SetAutoDeploy(ctx context.Context, p deploy.ProjectRef, branch string, enabled bool) (deploy.AutoDeploy, error) {
	const op = "setAutoDeploy"
	path, err := branchPath(op, p, branch, "auto-deploy")
	if err != nil {
		return deploy.AutoDeploy{}, err
	}
	var dto dataDTO[autoDeployDTO]
	if err := c.do(ctx, op, http.MethodPut, path, setAutoDeployDTO{Enabled: &enabled}, &dto, http.StatusOK); err != nil {
		return deploy.AutoDeploy{}, err
	}
	if got := dto.Data.Enabled; got == nil || *got != enabled {
		c.log.Warn("panacea answered an auto-deploy write without the value written", "op", op, "project", string(p),
			"branch", branch, "sent", enabled, "status", http.StatusOK)
		return deploy.AutoDeploy{}, &deploy.Error{Kind: deploy.KindUnavailable, Op: op, Status: http.StatusOK,
			Cause: fmt.Errorf("panacea: PUT %s answered 200 without enabled %v", path, enabled)}
	}
	return autoDeployOf(branch, dto.Data), nil
}

// Posture reads GET B/access (routes.go:1430), the route EnvironmentFor reads, as a
// dataDTO[postureDTO] (handlers/project_security.go:668-677): Mode, ModeKnown =
// deploy.ParsePostureMode(accessMode), which fails closed to kpmg-sso on a word it does not know.
func (c *Client) Posture(ctx context.Context, p deploy.ProjectRef, branch string) (deploy.Posture, error) {
	const op = "posture"
	path, err := branchPath(op, p, branch, "access")
	if err != nil {
		return deploy.Posture{}, err
	}
	var dto dataDTO[postureDTO]
	if err := c.doWith(ctx, noBodyLog, op, http.MethodGet, path, nil, &dto, http.StatusOK); err != nil {
		return deploy.Posture{}, err
	}
	d := dto.Data
	mode, known := deploy.ParsePostureMode(d.AccessMode)
	return deploy.Posture{Branch: branch, Environment: d.Env, Mode: mode, ModeRaw: d.AccessMode, ModeKnown: known,
		PasswordSet: d.PasswordSet, BypassTokenCount: d.BypassTokenCount, IPAllowlist: d.IPWhitelist}, nil
}

// notEnforcedSentence is the connector's own not-enforced message (review M7, re-review M-5).
// mode is the wire word this call sent (string(mode): "kpmg-sso", "panacea-sso", "bypass-token",
// "password", "none"), the same word the cockpit's radio names; branch is the name the caller
// passed, verbatim. Plain text: no backticks, no quotes. For ("bypass-token", "feat/fix(ui)"):
//
//	Access mode saved as bypass-token for feat/fix(ui), but not yet enforced: the route could not
//	be updated, so traffic still meets the previous mode. Re-apply, and do not treat the new mode
//	as active until it succeeds.
func notEnforcedSentence(mode deploy.PostureMode, branch string) string {
	return "Access mode saved as " + string(mode) + " for " + branch + ", but not yet enforced: the route could not " +
		"be updated, so traffic still meets the previous mode. Re-apply, and do not treat the new mode as active until it succeeds."
}

// SetPosture puts a setPostureDTO to B/access (routes.go:1431). A mode outside the five is refused
// before any request (Panacea would 400 it, project_security.go:718-722). The password is sent
// only with PosturePassword and a rotation only with PostureBypassToken, whatever else the change
// holds. Enforced is carried through. Message is Panacea's envelope message when enforced is true
// (:835); when it is false Panacea's sentence embeds reconcileErr, an err.Error() (:818-821,
// :836-839), so it is logged at warn and Message is notEnforcedSentence, built from the
// structured enforced and the mode and branch this call sent. BypassTokens is whatever Panacea
// minted, enforced or not (:786-797, :830-833); it is returned once and never logged. The answer
// holds no ipWhitelist, passwordSet or bypassTokenCount (:824-833), so Posture carries the branch,
// its environment and the mode only; a caller that shows the counts re-reads Posture.
func (c *Client) SetPosture(ctx context.Context, p deploy.ProjectRef, branch string, ch deploy.PostureChange) (deploy.PostureResult, error) {
	const op = "setPosture"
	if _, known := deploy.ParsePostureMode(string(ch.Mode)); !known {
		return deploy.PostureResult{}, &deploy.Error{Kind: deploy.KindRejected, Op: op,
			Cause: fmt.Errorf("panacea: access mode %q is not one of the five", ch.Mode)}
	}
	path, err := branchPath(op, p, branch, "access")
	if err != nil {
		return deploy.PostureResult{}, err
	}
	body := setPostureDTO{AccessMode: string(ch.Mode)}
	if ch.Mode == deploy.PosturePassword {
		body.Password = ch.Password
	}
	if ch.Mode == deploy.PostureBypassToken {
		body.RotateBypassTokens = ch.RotateBypassToken
	}
	var dto dataDTO[setPostureResponseDTO]
	if err := c.doWith(ctx, noBodyLog, op, http.MethodPut, path, body, &dto, http.StatusOK); err != nil {
		return deploy.PostureResult{}, err
	}
	d := dto.Data
	mode, known := deploy.ParsePostureMode(d.AccessMode)
	res := deploy.PostureResult{
		Posture:  deploy.Posture{Branch: branch, Environment: d.Env, Mode: mode, ModeRaw: d.AccessMode, ModeKnown: known},
		Enforced: d.Enforced, Message: dto.Message, BypassTokens: d.BypassTokens,
	}
	if !d.Enforced {
		c.log.Warn("panacea saved the access mode but did not enforce it", "op", op, "project", string(p),
			"branch", branch, "mode", string(ch.Mode), "panacea_message", dto.Message)
		res.Message = notEnforcedSentence(ch.Mode, branch)
	}
	return res, nil
}

// roleOf is the wire word to the domain: models/project_groups.go:9-12 spells developer "editor"
// (also "-developer"); ListProjectAccess writes only owner, editor or viewer
// (clients/keycloak.go:503-512). Any other word is RoleUnknown, kept in RoleRaw.
func roleOf(word string) deploy.AccessRole {
	switch word {
	case "viewer":
		return deploy.RoleViewer
	case "editor", "developer":
		return deploy.RoleDeveloper
	case "owner":
		return deploy.RoleOwner
	}
	return deploy.RoleUnknown
}

// wireRole is the domain to the wire. RoleDeveloper is "editor": AddProjectAccess builds the
// group "project-<name>-"+role unchecked (clients/keycloak.go:192) and ListProjectAccess reads any
// suffix but -owner and -editor back as viewer (:503-512), so "developer" would list as a viewer.
func wireRole(r deploy.AccessRole) (string, bool) {
	switch r {
	case deploy.RoleViewer:
		return "viewer", true
	case deploy.RoleDeveloper:
		return "editor", true
	case deploy.RoleOwner:
		return "owner", true
	}
	return "", false
}

// Access reads GET P/access (routes.go:1407), a listDTO[accessBindingDTO] (handlers/access.go:31-34).
// Group and Grantee are copied verbatim; Legacy is group != grantee (review M30).
func (c *Client) Access(ctx context.Context, p deploy.ProjectRef) ([]deploy.AccessGrant, error) {
	const op = "access"
	P, err := projectPath(op, p)
	if err != nil {
		return nil, err
	}
	var dto listDTO[accessBindingDTO]
	if err := c.get(ctx, op, P+"/access", &dto); err != nil {
		return nil, err
	}
	return mapSlice(dto.Items, func(b accessBindingDTO) deploy.AccessGrant {
		return deploy.AccessGrant{Grantee: b.Grantee, Group: b.Group, Legacy: b.Group != b.Grantee,
			Role: roleOf(b.Role), RoleRaw: b.Role}
	}), nil
}

// noSuchUser is Grant's sentence for plan 7P-12's one parsed 500.
const noSuchUser = "No Panacea user matches that name or email."

// unknownGrantee reports whether a 500's sentence is AccessHandler.Add's
// "failed to add access binding: "+err.Error() (handlers/access.go:57) over one of the two typo
// errors: "a grantee (username or email) is required" (clients/keycloak.go:182), EQUAL, or "no
// Keycloak user found for grantee %q" (:189), a PREFIX. Never a contains: "resolve grantee %q: …"
// (:186) and "get admin token: …" (:177) are outages behind the same prefix (re-review M-1).
func unknownGrantee(sentence string) bool {
	rest, ok := strings.CutPrefix(sentence, "failed to add access binding: ")
	return ok && (strings.HasPrefix(rest, "no Keycloak user found for grantee ") ||
		rest == "a grantee (username or email) is required")
}

// Grant posts a grantDTO to P/access (routes.go:1408), answered 201 (handlers/access.go:61).
// RoleUnknown, or any word wireRole does not know, is refused before any request. It is
// single-role on Panacea: AddProjectAccess drops the grantee from the project's other role groups
// (clients/keycloak.go:198-214), best-effort, before the add (:216-218), so a grant to a member IS
// the role change, one request, and a failed add can leave no role: the caller re-reads (tasks
// 11, 18; re-review I-1). A 500 is KindUnavailable, except unknownGrantee's two, which are
// KindRejected with noSuchUser and the 500 received (plan 7P-12's single exception).
func (c *Client) Grant(ctx context.Context, p deploy.ProjectRef, g deploy.GrantRequest) error {
	const op = "grant"
	role, ok := wireRole(g.Role)
	if !ok {
		return &deploy.Error{Kind: deploy.KindRejected, Op: op, Cause: fmt.Errorf("panacea: role %q cannot be granted", g.Role)}
	}
	P, err := projectPath(op, p)
	if err != nil {
		return err
	}
	err = c.do(ctx, op, http.MethodPost, P+"/access", grantDTO{Group: g.Grantee, Role: role}, nil, http.StatusCreated)
	var e *deploy.Error
	if errors.As(err, &e) && e.Status == http.StatusInternalServerError && unknownGrantee(answeredSentence(err)) {
		cp := *e
		cp.Kind, cp.Message = deploy.KindRejected, noSuchUser
		return &cp
	}
	return err
}

// Revoke deletes P/access/{group} (routes.go:1409), the row's Group url.PathEscape'd ("@" is a
// pchar and stays). For a person's row it is their identity, removed from EVERY role group
// (clients/keycloak.go:245-260); for a legacy row it is the group's name, and the group is
// deleted (:262-267). A legacy row's Grantee names no group, so Panacea would answer 200 having
// removed nothing (:268), or remove a user who happens to have that name: it is never sent.
func (c *Client) Revoke(ctx context.Context, p deploy.ProjectRef, group string) error {
	const op = "revoke"
	P, err := projectPath(op, p)
	if err != nil {
		return err
	}
	g, err := segment(op, "group", group)
	if err != nil {
		return err
	}
	return c.do(ctx, op, http.MethodDelete, P+"/access/"+g, nil, nil, http.StatusOK)
}

// SetURLPrefix puts {"prefix"} to B/url-prefix (routes.go:1313) and reads SetBranchURLPrefix's
// dataDTO[urlPrefixResponseDTO] (handlers/projects.go:4365-4371); Branch is the branch this call
// named. Its 409 "url prefix already taken: …" (:4321) is KindConflict with the sentence verbatim.
func (c *Client) SetURLPrefix(ctx context.Context, p deploy.ProjectRef, branch, prefix string) (deploy.URLPrefix, error) {
	const op = "setURLPrefix"
	path, err := branchPath(op, p, branch, "url-prefix")
	if err != nil {
		return deploy.URLPrefix{}, err
	}
	var dto dataDTO[urlPrefixResponseDTO]
	if err := c.do(ctx, op, http.MethodPut, path, urlPrefixDTO{Prefix: prefix}, &dto, http.StatusOK); err != nil {
		return deploy.URLPrefix{}, err
	}
	return deploy.URLPrefix{Branch: branch, Prefix: dto.Data.Prefix, Host: dto.Data.Host}, nil
}

// ClearURLPrefix deletes B/url-prefix (routes.go:1314). Clearing a branch with no prefix is
// success (handlers/projects.go:4378-4379, :4407); the body, data {branch, routeReverted} plus a
// message (:4448-4450), is not read: only the 200 is used.
func (c *Client) ClearURLPrefix(ctx context.Context, p deploy.ProjectRef, branch string) error {
	const op = "clearURLPrefix"
	path, err := branchPath(op, p, branch, "url-prefix")
	if err != nil {
		return err
	}
	return c.do(ctx, op, http.MethodDelete, path, nil, nil, http.StatusOK)
}
