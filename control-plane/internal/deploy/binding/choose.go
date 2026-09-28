package binding

import (
	"context"
	"fmt"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/deploy"
)

// ChooseRejected is the sentence a pick outside the offered choices gets. It is
// a rejection rather than a 404 because the project may well exist: it is not
// one of the projects that deploy THIS repository.
const ChooseRejected = "That project is not one of the choices."

// Choose records the user's pick for an Ambiguous or NoRepoInfo project.
//
// The pick must be one of the CURRENT candidates, recomputed here rather than
// taken from the cache: a free-text binding would let a caller point Ship at
// any project name the credential can reach, and a cached candidate list can be
// minutes old.
func (r *Resolver) Choose(ctx context.Context, projectID string, p deploy.ProjectRef) (Binding, error) {
	b, err := r.Resolve(ctx, projectID)
	if err != nil {
		return b, err
	}
	if !containsRef(b.Candidates, p) {
		return b, &deploy.Error{
			Kind:    deploy.KindRejected,
			Op:      "deploy binding",
			Message: ChooseRejected,
		}
	}
	if b.RepoKey == "" {
		// Unreachable while candidates are only offered for a parsed origin,
		// and refused rather than written: a choice keyed on "" would be
		// applied to every repository that has no key.
		return b, &deploy.Error{Kind: deploy.KindRejected, Op: "deploy binding", Message: ChooseRejected}
	}
	if err := r.choices.set(b.RepoKey, r.cfg.PanaceaOrigin(), p, r.cfg.Now()); err != nil {
		return b, fmt.Errorf("binding: record the choice: %w", err)
	}
	return r.Resolve(ctx, projectID)
}
