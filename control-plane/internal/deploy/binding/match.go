package binding

import "github.com/kpmg-au-emu/panacea-app/control-plane/internal/deploy"

// match is step 4 and 5: compare repository KEYS, never URLs. Both sides went
// through the same ParseRemote, so the SSH and HTTPS forms of one repository,
// and dev.azure.com and the old .visualstudio.com form of one Azure DevOps
// repository, are one key each.
func (r *Resolver) match(p *pending, projects []deploy.Project) {
	var matches []deploy.Project
	usable := 0
	for _, pr := range projects {
		if pr.RepoURL == "" {
			continue
		}
		remote, err := r.cfg.ParseRemote(pr.RepoURL)
		if err != nil {
			// The project is skipped for MATCHING and stays in the pool the
			// user may choose from: Panacea normalises clone URLs, and a form
			// phase 3 cannot parse is Panacea not saying which repository this
			// project deploys - not Panacea saying it deploys none.
			continue
		}
		usable++
		if remote.RepoKey == p.key {
			matches = append(matches, pr)
		}
	}

	switch {
	case len(matches) == 1:
		// Message is left as the Hub phase left it. Bound carries no sentence
		// of its own, but a Hub blip's sentence is the only report the lens
		// gets that the Hub answer beside this binding is stale.
		p.b.Panacea, p.b.ChosenBy, p.b.State = matches[0].Ref, "auto", Bound
		return
	case len(matches) > 1:
		p.b.State, p.b.Message, p.b.Candidates = Ambiguous, MessageAmbiguous, matches
	case usable == 0 && len(projects) > 0:
		p.b.State, p.b.Message, p.b.Candidates = NoRepoInfo, MessageNoRepoInfo, projects
	default:
		p.b.State, p.b.Message, p.b.Candidates = NoMatch, MessageNoMatch, nil
		return
	}
	r.applyChoice(p)
}

// applyChoice turns a question already answered into a binding. The choice is
// kept by (repository key, Panacea address), so a pick made against dev is
// never applied to prod, and a pick whose project has left this key's matches
// is dropped rather than followed.
func (r *Resolver) applyChoice(p *pending) {
	ref, ok := r.choices.get(p.key, r.cfg.PanaceaOrigin())
	if !ok {
		return
	}
	for _, c := range p.b.Candidates {
		if c.Ref == ref {
			p.b.Panacea, p.b.ChosenBy, p.b.State = ref, "local", Bound
			p.b.Message, p.b.Candidates = "", nil
			return
		}
	}
}

func containsRef(candidates []deploy.Project, ref deploy.ProjectRef) bool {
	for _, c := range candidates {
		if c.Ref == ref {
			return true
		}
	}
	return false
}
