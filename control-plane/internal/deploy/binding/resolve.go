package binding

import (
	"context"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/deploy"
)

// pending is one id part-way through a cycle.
type pending struct {
	id     string
	b      Binding
	key    string
	hubMsg string // the Hub's sentence, shown only when the state has none of its own
	done   bool   // decided locally: NoOrigin or UnknownHost, which no upstream can change
}

// Resolve computes one binding, for a caller acting on one project. Tasks 09,
// 11 and 12 read Cached between cycles.
//
// A Panacea or Hub failure returns the LAST good binding with State Unavailable
// AND the error - never a silently unbound project.
func (r *Resolver) Resolve(ctx context.Context, projectID string) (Binding, error) {
	m, err := r.ResolveMany(ctx, []string{projectID})
	b, ok := m[projectID]
	if !ok {
		if err != nil {
			return Binding{}, err
		}
		return Binding{}, ErrUnknownProject
	}
	return b, err
}

// ResolveMany computes the bindings of several local projects from ONE
// LocalProjects.Projects read, ONE Deployer.Projects read and one
// HubProjects.ProjectForRepo per distinct RepoKey, whatever len(projectIDs) is
// (review M16: N local projects were N GET /api/v1/projects every 15 s).
//
// Every id that names a local project gets an entry. An id that names none is
// ABSENT rather than present-and-empty: the caller that asked for one turns
// absence into ErrUnknownProject, and the watcher drops a project that has
// gone. A Panacea failure gives every entry its last good binding with State
// Unavailable and the one error; a Hub failure keeps the previous Hub answer
// per key.
func (r *Resolver) ResolveMany(ctx context.Context, projectIDs []string) (map[string]Binding, error) {
	locals, err := r.cfg.Projects.Projects(ctx)
	if err != nil {
		// A local scan failure is not a statement about Panacea, so no binding
		// is rewritten and no sentence is invented: the caller sees the error
		// and the bindings it already had.
		return r.cachedFor(projectIDs), err
	}
	byID := make(map[string]string, len(locals))
	for _, p := range locals {
		byID[p.ID] = p.Path
	}

	pendings := make([]*pending, 0, len(projectIDs))
	for _, id := range projectIDs {
		path, ok := byID[id]
		if !ok {
			continue
		}
		pendings = append(pendings, r.localSide(ctx, id, path))
	}

	projects, perr := r.cfg.Deployer.Projects(ctx)
	if perr != nil {
		return r.upstreamDown(pendings, perr), perr
	}
	r.hubPhase(ctx, pendings)
	out := make(map[string]Binding, len(pendings))
	for _, p := range pendings {
		if !p.done {
			r.match(p, projects)
		}
		if p.b.State == Bound {
			p.b.Message = p.hubMsg
		}
		out[p.id] = r.remember(p.b)
	}
	return out, nil
}

// localSide is everything decided before any network call: the origin, the key,
// and the two states that are a fact about the laptop rather than about any
// upstream.
func (r *Resolver) localSide(ctx context.Context, id, path string) *pending {
	prev, _ := r.Cached(id)
	p := &pending{id: id, b: Binding{ProjectID: id, HubProjectID: prev.HubProjectID, HubMiss: prev.HubMiss}}
	origin, err := r.cfg.Remotes.OriginURL(ctx, path)
	if err != nil || origin == "" {
		if err != nil {
			r.cfg.Log.Warn("binding: origin unreadable", "projectId", id, "err", err)
		}
		p.b.State, p.b.Message, p.done = NoOrigin, MessageNoOrigin, true
		p.b.HubProjectID, p.b.HubMiss = "", ""
		return p
	}
	remote, err := r.cfg.ParseRemote(origin)
	if err != nil {
		p.b.State, p.b.Message, p.done = UnknownHost, MessageUnknownHost, true
		p.b.HubProjectID, p.b.HubMiss = "", ""
		return p
	}
	p.key, p.b.RepoKey = remote.RepoKey, remote.RepoKey
	return p
}

// upstreamDown answers a Panacea failure: not_connected is its own state,
// because "no credential" is not a statement about Panacea; anything else keeps
// the last good binding and says so.
func (r *Resolver) upstreamDown(pendings []*pending, err error) map[string]Binding {
	notConnected := deploy.KindOf(err) == deploy.KindNotConnected
	out := make(map[string]Binding, len(pendings))
	for _, p := range pendings {
		if !p.done {
			prev, _ := r.Cached(p.id)
			if notConnected {
				p.b.State, p.b.Message = NotConnected, MessageNotConnected
			} else {
				p.b.Panacea, p.b.ChosenBy = prev.Panacea, prev.ChosenBy
				p.b.State, p.b.Message = Unavailable, MessageUnavailable
			}
		}
		out[p.id] = r.remember(p.b)
	}
	return out
}

func (r *Resolver) cachedFor(ids []string) map[string]Binding {
	out := make(map[string]Binding, len(ids))
	for _, id := range ids {
		if b, ok := r.Cached(id); ok {
			out[id] = b
		}
	}
	return out
}

func (r *Resolver) remember(b Binding) Binding {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.cache[b.ProjectID] = b
	return b
}
