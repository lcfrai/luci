// Package binding decides which deploy-target project deploys a local project,
// by repository.
//
// A local project is identified three ways and each has one owner: the local
// "p-…" id (phase 2, a hash of a path on one laptop), the repository key (phase
// 3's githost, the same for every teammate, both hosts - OD-4), and the Hub
// project (found by repository key). The deploy target's project is found by
// matching repository keys too, so the common case - one Panacea project
// deploys this repository - binds with no question asked.
//
// What this package deliberately does NOT own: the spelling of a repository key
// and the rule for which spellings the Hub accepts. Both arrive through Config
// (ParseRemote, HubKeyAccepted). A copy of either here would be a canonicaliser
// beside phase 3's, the Hub's and Panacea's, and it would drift the day one of
// them changed.
package binding

import (
	"context"
	"errors"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/project"
)

// ErrUnknownProject is returned for a local project id no configured root
// holds. The route maps it to phase 2's 404 project_not_found.
var ErrUnknownProject = errors.New("binding: unknown local project id")

// ErrUnknownHost is what Config.ParseRemote returns for an origin that is
// neither GitHub nor Azure DevOps. main.go's adapter maps phase 3's
// githost.ErrUnknownHost onto it.
var ErrUnknownHost = errors.New("binding: origin is neither GitHub nor Azure DevOps")

// Remote is the part of phase 3's githost.Remote this package reads. It is one
// field because one field is all that is used: the key both sides compare.
type Remote struct {
	RepoKey string
}

// ParseRemote turns a clone URL into its repository key. main.go satisfies it
// with phase 3 task 13's githost:
//
//	func(u string) (binding.Remote, error) {
//		r, err := githost.ParseRemote(u)
//		if errors.Is(err, githost.ErrUnknownHost) { return binding.Remote{}, binding.ErrUnknownHost }
//		if err != nil { return binding.Remote{}, err }
//		return binding.Remote{RepoKey: r.RepoKey()}, nil
//	}
type ParseRemote func(cloneURL string) (Remote, error)

// Remotes reads a local project's origin URL. main.go satisfies it with phase
// 3's gitx: `git -C <path> remote get-url origin`. "" with a nil error when
// there is no origin.
type Remotes interface {
	OriginURL(ctx context.Context, projectPath string) (string, error)
}

// HubProjects is the one hubclient.Projects method this package needs (Hub
// contract §9.2), narrowed to the project id because the id is all the binding
// keeps. main.go's adapter is one line over hubclient.Projects.ProjectForRepo.
type HubProjects interface {
	ProjectForRepo(ctx context.Context, repoKey string) (hubProjectID string, ok bool, err error)
}

// LocalProjects is the one phase 2 method this package needs. *project.Scanner
// satisfies it, as it satisfies api.Enumerator; declaring it here keeps binding
// from importing api (api imports binding for the route).
type LocalProjects interface {
	Projects(ctx context.Context) ([]project.Project, error)
}
