package management

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// ExecutionActorRef is a durable identity reference, never an authorization
// snapshot. A queued operation must resolve current policy and project grants
// immediately before admission and again before allocation.
type ExecutionActorRef struct {
	UserID    string `json:"userId,omitempty"`
	TeamID    string `json:"teamId,omitempty"`
	ProjectID string `json:"projectId"`
	Local     bool   `json:"local,omitempty"`
}

type ExecutionAuthority struct {
	Actor  User
	Policy Policy
	Team   *TeamAccess
}

type ExecutionAuthorizer func(context.Context, ExecutionActorRef, string, bool) (ExecutionAuthority, error)

var ErrExecutionAuthority = errors.New("Current account, project or execution permission does not authorize this task. Review its access before continuing.")

// CaptureExecutionActor retains only immutable identity. Request context,
// bearer tokens, roles, grants and policy never become queue authority.
func CaptureExecutionActor(ctx context.Context, projectID string) (ExecutionActorRef, error) {
	if projectID == "" {
		return ExecutionActorRef{}, ErrExecutionAuthority
	}
	actor, _, authenticated := ActorFromContext(ctx)
	team, scoped := TeamAccessFromContext(ctx)
	if scoped {
		if !authenticated || actor.ID == "" || team.Team.ID == "" || team.ProjectID != projectID || !team.Permissions.Edit || !team.Permissions.Execute {
			return ExecutionActorRef{}, ErrExecutionAuthority
		}
		return ExecutionActorRef{UserID: actor.ID, TeamID: team.Team.ID, ProjectID: projectID}, nil
	}
	if authenticated {
		return ExecutionActorRef{UserID: actor.ID, ProjectID: projectID}, nil
	}
	if TeamRuntimeIDFromContext(ctx) != "" {
		return ExecutionActorRef{}, ErrExecutionAuthority
	}
	return ExecutionActorRef{ProjectID: projectID, Local: true}, nil
}

// AuthorizeExecution resolves durable identity under the same lock used for
// revocation. Request contexts, stored queue roles and old grants are ignored.
func (g *Gateway) AuthorizeExecution(ctx context.Context, ref ExecutionActorRef, harness string, needsSecrets bool) (ExecutionAuthority, error) {
	if ctx.Err() != nil {
		return ExecutionAuthority{}, ctx.Err()
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.authorizeExecutionLocked(ref, harness, needsSecrets)
}

func (g *Gateway) authorizeExecutionLocked(ref ExecutionActorRef, harness string, needsSecrets bool) (ExecutionAuthority, error) {
	if g.closed || ref.Local || ref.UserID == "" || !teamProjectIDPattern.MatchString(ref.ProjectID) || validatePolicy(g.state.Policy) != nil {
		return ExecutionAuthority{}, ErrExecutionAuthority
	}
	user, ok := g.state.Users[ref.UserID]
	if !ok || user.User.Disabled || user.User.ID != ref.UserID || (user.User.Role != "member" && user.User.Role != "admin" && user.User.Role != "super-admin") {
		return ExecutionAuthority{}, ErrExecutionAuthority
	}
	allowed := false
	for _, candidate := range g.state.Policy.AllowedHarnesses {
		if candidate == harness {
			allowed = true
			break
		}
	}
	if !allowed {
		return ExecutionAuthority{}, ErrExecutionAuthority
	}
	policy := g.state.Policy
	policy.AllowedHarnesses = append([]string(nil), policy.AllowedHarnesses...)
	out := ExecutionAuthority{Actor: user.User, Policy: policy}
	if ref.TeamID == "" {
		if g.managedTeamID != "" {
			return ExecutionAuthority{}, ErrExecutionAuthority
		}
		return out, nil
	}
	access, e := g.resolveTeamAccessLocked(ref.TeamID, ref.UserID, ref.ProjectID)
	if e != nil || !access.Permissions.Edit || !access.Permissions.Execute || (needsSecrets && !access.Permissions.Secrets) || !g.teamServiceAllowsExecutionLocked(ref.TeamID) {
		return ExecutionAuthority{}, ErrExecutionAuthority
	}
	out.Actor = scopedTeamActor(user.User, access)
	out.Policy = teamPolicy(policy)
	out.Team = &access
	return out, nil
}

// WithExecutionAuthority installs freshly checked values and masks any stale
// team/actor authority carried by the incoming request context.
func WithExecutionAuthority(ctx context.Context, authority ExecutionAuthority) context.Context {
	authority.Policy.AllowedHarnesses = append([]string(nil), authority.Policy.AllowedHarnesses...)
	if authority.Team != nil {
		copy := *authority.Team
		authority.Team = &copy
	}
	return executionAuthorityContext{Context: ctx, authority: authority}
}

type executionAuthorityContext struct {
	context.Context
	authority ExecutionAuthority
}

func (c executionAuthorityContext) Value(key any) any {
	switch key.(type) {
	case actorKey:
		return actorContext{c.authority.Actor, c.authority.Policy}
	case teamAccessKey:
		if c.authority.Team != nil {
			return *c.authority.Team
		}
		return nil
	case teamRuntimeKey:
		if c.authority.Team != nil {
			return c.authority.Team.Team.ID
		}
		return ""
	default:
		return c.Context.Value(key)
	}
}

// FileExecutionAuthorityScope is injected by the private management parent.
// UserID selects a personal worker; TeamID selects a team worker. Neither is
// accepted from queued data or the worker's public request headers.
type FileExecutionAuthorityScope struct {
	IdentityPath    string
	UserID          string
	TeamID          string
	ManagedTeamID   string
	BillingConfigID string
}

func NewFileExecutionAuthorizer(scope FileExecutionAuthorityScope) (ExecutionAuthorizer, error) {
	if !filepath.IsAbs(scope.IdentityPath) || filepath.Clean(scope.IdentityPath) != scope.IdentityPath || filepath.Base(scope.IdentityPath) != "identity.json" || (scope.UserID == "") == (scope.TeamID == "") || (scope.TeamID != "" && !teamIDPattern.MatchString(scope.TeamID)) || (scope.ManagedTeamID != "" && scope.ManagedTeamID != scope.TeamID) || (scope.BillingConfigID != "" && (scope.ManagedTeamID == "" || len(scope.BillingConfigID) != 64 || strings.Trim(scope.BillingConfigID, "0123456789abcdef") != "")) {
		return nil, ErrExecutionAuthority
	}
	return func(ctx context.Context, ref ExecutionActorRef, harness string, needsSecrets bool) (ExecutionAuthority, error) {
		if ctx.Err() != nil {
			return ExecutionAuthority{}, ctx.Err()
		}
		if ref.Local || ref.UserID == "" || (scope.UserID != "" && (ref.UserID != scope.UserID || ref.TeamID != "")) || (scope.TeamID != "" && ref.TeamID != scope.TeamID) {
			return ExecutionAuthority{}, ErrExecutionAuthority
		}
		state, e := readExecutionIdentity(scope.IdentityPath)
		if e != nil {
			return ExecutionAuthority{}, ErrExecutionAuthority
		}
		g := Gateway{state: state, managedTeamID: scope.ManagedTeamID}
		if scope.BillingConfigID != "" {
			g.billing = &teamBillingRuntime{configID: scope.BillingConfigID}
		}
		// A durable billing binding cannot be silently reinterpreted as a
		// self-hosted team merely because its worker lost billing configuration.
		if row, ok := state.Teams[ref.TeamID]; ok && row.Billing != nil && scope.BillingConfigID == "" {
			return ExecutionAuthority{}, ErrExecutionAuthority
		}
		if ctx.Err() != nil {
			return ExecutionAuthority{}, ctx.Err()
		}
		return g.authorizeExecutionLocked(ref, harness, needsSecrets)
	}, nil
}

func readExecutionIdentity(filename string) (storedState, error) {
	var state storedState
	root, e := openExecutionIdentityDirectory(filepath.Dir(filename))
	if e != nil {
		return state, e
	}
	defer root.Close()
	before, e := root.Lstat("identity.json")
	if e != nil || !before.Mode().IsRegular() || before.Size() > 8<<20 {
		return state, ErrExecutionAuthority
	}
	f, e := root.Open("identity.json")
	if e != nil {
		return state, e
	}
	defer f.Close()
	opened, e := f.Stat()
	if e != nil || !os.SameFile(before, opened) || !executionIdentityFilePrivate(f, opened) {
		return state, ErrExecutionAuthority
	}
	data, e := io.ReadAll(io.LimitReader(f, (8<<20)+1))
	if e != nil || len(data) > 8<<20 {
		return state, ErrExecutionAuthority
	}
	after, e := root.Lstat("identity.json")
	if e != nil || !after.Mode().IsRegular() || !os.SameFile(opened, after) || opened.Size() != after.Size() || !opened.ModTime().Equal(after.ModTime()) {
		return state, ErrExecutionAuthority
	}
	if json.Unmarshal(data, &state) != nil || state.Version != 1 || state.Users == nil || state.Sessions == nil || validatePolicy(state.Policy) != nil || validateTeams(state) != nil {
		return state, ErrExecutionAuthority
	}
	owner, ok := state.Users[state.OwnerID]
	if !ok || owner.User.ID != state.OwnerID {
		return state, ErrExecutionAuthority
	}
	return state, nil
}

// Pin every directory independently. A substituted symlink/junction cannot
// turn the fixed private authority path into project-controlled state.
func openExecutionIdentityDirectory(directory string) (*os.Root, error) {
	volume := filepath.VolumeName(directory)
	base := volume + string(filepath.Separator)
	root, e := os.OpenRoot(base)
	if e != nil {
		return nil, e
	}
	relative := strings.TrimPrefix(directory, base)
	for _, component := range strings.Split(relative, string(filepath.Separator)) {
		if component == "" {
			continue
		}
		if component == "." || component == ".." {
			root.Close()
			return nil, ErrExecutionAuthority
		}
		before, e := root.Lstat(component)
		if e != nil || !before.IsDir() || before.Mode()&(os.ModeSymlink|os.ModeIrregular) != 0 {
			root.Close()
			return nil, ErrExecutionAuthority
		}
		next, e := root.OpenRoot(component)
		if e != nil {
			root.Close()
			return nil, e
		}
		after, e := next.Stat(".")
		root.Close()
		if e != nil || !os.SameFile(before, after) {
			next.Close()
			return nil, ErrExecutionAuthority
		}
		root = next
	}
	return root, nil
}
