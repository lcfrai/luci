package binding

import "context"

// hubAnswer is one key's Hub result, computed once per cycle and applied to
// every local project that shares the key.
type hubAnswer struct {
	id           string
	found        bool
	miss         string
	keepPrevID   bool
	keepPrevMiss bool
	message      string
}

// hubPhase asks the Hub once per distinct key. The key sent is hubKey(remote),
// verbatim, and the lookup is ALWAYS made - never skipped because the Hub's
// rule would refuse the key, or a repository would stop binding the day the
// Hub widened its rule.
func (r *Resolver) hubPhase(ctx context.Context, pendings []*pending) {
	if r.cfg.Hub == nil {
		for _, p := range pendings {
			p.b.HubProjectID, p.b.HubMiss = "", "" // no Hub configured
		}
		return
	}
	answers := map[string]hubAnswer{}
	var blocked *hubAnswer
	for _, p := range pendings {
		if p.done || p.key == "" {
			p.b.HubProjectID, p.b.HubMiss = "", ""
			continue
		}
		a, seen := answers[p.key]
		if !seen {
			if blocked != nil {
				// Carried b09-2 I-1: one Hub that will not answer must not cost
				// the watcher one timeout per repository. The first failure
				// settles the cycle for every key left.
				a = *blocked
			} else {
				a = r.lookup(ctx, hubKey(Remote{RepoKey: p.key}))
				if a.keepPrevID {
					settled := a
					blocked = &settled
				}
			}
			answers[p.key] = a
		}
		applyHub(p, a)
	}
}

func (r *Resolver) lookup(ctx context.Context, key string) hubAnswer {
	c, cancel := context.WithTimeout(ctx, r.cfg.HubReadTimeout)
	defer cancel()
	id, ok, err := r.cfg.Hub.ProjectForRepo(c, key)
	if err == nil {
		if ok {
			return hubAnswer{id: id, found: true}
		}
		return hubAnswer{miss: missCause(r.cfg.HubKeyAccepted, key)}
	}
	switch r.classify(err) {
	case HubRefusedKey:
		// An ANSWER, not an outage: the Hub itself says the key is not one it
		// accepts, so no Hub project can ever hold it (carried b05-2 N-1).
		return hubAnswer{miss: MissKeySpelling}
	case HubSignedOut:
		return hubAnswer{keepPrevID: true, miss: MissSignedOut}
	default:
		r.cfg.Log.Warn("binding: hub lookup failed", "repoKey", key, "err", err)
		return hubAnswer{keepPrevID: true, keepPrevMiss: true, message: MessageHubDown}
	}
}

func (r *Resolver) classify(err error) HubErrorClass {
	if r.cfg.ClassifyHubError == nil {
		return HubBlip
	}
	return r.cfg.ClassifyHubError(err)
}

func applyHub(p *pending, a hubAnswer) {
	switch {
	case a.found:
		p.b.HubProjectID, p.b.HubMiss = a.id, ""
	case a.keepPrevID:
		if !a.keepPrevMiss {
			p.b.HubMiss = a.miss
		}
	default:
		p.b.HubProjectID, p.b.HubMiss = "", a.miss
	}
	// The Hub's sentence is kept apart from the state's own until the end of
	// the cycle: a Panacea sentence always wins, and Bound - which has no
	// sentence of its own - is the only state that shows the Hub's.
	p.hubMsg = a.message
}
