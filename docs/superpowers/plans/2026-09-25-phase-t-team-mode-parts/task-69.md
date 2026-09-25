### Task 69: TM2 proof — the mirror, tier M, a team land, build-on, the bump job, quotas

**Slice:** TM2. **Focused time:** 2.5 h of writing and wiring, plus the run, which is two people's
morning and is **not** in the estimate. **dependsOn:** 58–68, every TM2 task; task 57's `proof`
package and `teamproof` command (TM1's proof), whose shapes this task extends.

TM2 is "one document" (skeleton `:377`): the record that shows the six things TM2 added work on
the real environment, each with what was and was not observed. This task writes that record, the
fixed run script two people follow, a collector that exports what the run left behind, and six
verdicts in task 57's `proof` package, so every row says `PASS`, `FAIL` or `NOT RUN` with its
evidence ids, computed from records, never from a screenshot (`CLAUDE.md`, working agreements).

**The six items, as the verdict program reads them** (the skeleton's scope, `:1117-1121`):

| # | Item | Read from | `PASS` when |
|---|---|---|---|
| M1 | the mirror pod's isolation: no egress, no credential beyond fetch, no sandbox mount | the mirror pod's spec and its NetworkPolicy (`kubectl get -o json`, namespace `panacea-mirrors`, label `panacea-app/component: mirror`, `task-58.md:360-370`); an egress probe pod carrying the same labels | no volume but its own project disk and the fetch-credential Secret; `automountServiceAccountToken: false`; no env or volume naming a gateway token; the policy's only ingress is Fleet's control route and it has no egress rule; every probe (gateway, `github.com`, `dev.azure.com`, `1.1.1.1:443`, `169.254.169.254`) timed out; **and** no machine pod's spec mounts the mirror's disk |
| M2 | a tier-M forecast on a real pair | Hub conflicts (route 59), the mirror's result log (task 59, ST.7) | a `cf-…` with `tier: "M"` whose two sides are lanes of the two different people, whose basis shas equal the mirror's result for that pair; and every pair in the mirror's log was flagged P or H first (task 60's rule) |
| M3 | a team land through the mirror, pushed with the owner's credential | lands (task 68's route 83), the git host's push event for the branch, the release (route 37) | an `ld-…` whose `mergeCommit` is the remote branch head; the host's push event for that sha names the lane owner's own identity; and a release reserved at that commit lists the land with its owner, `tk-…` and `briefRef` (task 68) |
| M4 | a `build-on` decide listing an agent-config path | escalations (route 66, as org *auditor*) | a decide whose resolution row is `build-on` and whose `agentConfig` holds `.claude/settings.json` on its own entry (task 62), raised to the builder's own human only |
| M5 | the bump job's dry run through its four gates | the pipeline run's JSON result (task 65) | all four gates ran and passed, in order; the reference it would promote matches `@sha256:[0-9a-f]{64}`, never a tag; the warm pool was **not** changed (a dry run) |
| M6 | quotas enforced at the gateway | keysmith's refusal and the Hub's usage rows (task 66), route 75's refusal | a mint refused `402` with `"window": "day"` for the member whose quota was lowered, while a teammate on the same project still mints; the Hub's roll-up row for that member exists with a cost at or above the quota; and a route-75 `PUT` above the ceiling answered 422 `above_org_ceiling` |

**Decisions where the scope is silent, made here so nobody re-derives them.**

**(a) Same package, own export, own subcommand.** Task 57's `Load` refuses a directory missing
any of its twelve files (`task-57.md:216-235`), so a TM2 run is not squeezed into it. This task
adds `Load2` over its own file list and `TM2(Export2) []Verdict`, reusing `Verdict`, `Result` and
the record's row format (`task-57.md:256-270`), and one subcommand, `teamproof tm2 -dir`.

**(b) The mirror is probed beside itself, not inside itself.** The mirror runs a PTY-less command
(task 59) and the plan gives it no shell to exec into, on purpose. So M1 reads the real pod's spec
and policy, and runs its live egress probes from a throwaway pod in `panacea-mirrors` with the
mirror's labels and runtime class, which the same NetworkPolicy selects. The record says so: the
probe proves what the policy does to a pod with those labels, not what the mirror binary tries.

**(c) The push is proved by the git host, not by us.** Constraint 8 says the push uses the lane
owner's own token; nothing on our side holds a record of which token pushed. The git host does:
GitHub's push event names the pusher, and Azure DevOps' push record names `pushedBy`. The collector
reads it with the operator's **own** read-only token, never the project's fetch credential (the
broker's alone, task 48). Which host the run uses is the script's first line; the other host's
read is **UNVERIFIED** until a run uses it.

**(d) M3's release is reserved directly.** Phase 7 has no plan (task 68), so there is no Ship
button that reserves a release. The script reserves one with a direct `POST
/v1/projects/{projectId}/releases` as the sharer, at the land's merge commit, then abandons it. The
row says the release was reserved by hand, and proves only the Hub side.

**(e) Every row fails loudly, as task 57's do** (`task-57.md:67-71`): M2 with no tier-M conflict is
`FAIL (no tier-M conflict)`. Each item's file carries `"ran": true|false`; `false` is `NOT RUN`,
and an item that ran with nothing in it is `FAIL`, never `PASS`.

**Files:**
- `docs/verification/2026-09-25-phase-t-tm2-results.md` — the record (new; the skeleton's row)
- `infra/smoke/team/tm2-script.md` — the fixed run script (new)
- `infra/smoke/team/tm2-collect.sh` — the collector: fetches only, writes the export (new)
- `control-plane/internal/fleet/proof/tm2.go` — `Export2`, `Load2`, `TM2` and the six verdicts
  (new, in task 57's package)
- `control-plane/internal/fleet/proof/tm2_test.go`, `.../proof/testdata/tm2/{pass,…}/*.json` (new)
- `control-plane/cmd/teamproof/main.go` — the `tm2` subcommand (modify, task 57's file, one
  `switch` arm)

The skeleton's file map names only the record; the other five are added here, beside task 57's,
and listed as gap 1.

**Interfaces:**
- Produces: `proof.Export2`, `proof.Load2(dir)`, `proof.TM2(Export2) []Verdict`; `teamproof tm2`.
- Consumes, **written against these names; where the owning task spells one differently, it
  wins and this part is renamed mechanically**:
  - task 57: `proof.Verdict`, `Result`, `Pass`, `Fail`, `NotRun`, `Person` (`task-57.md:129-132`,
    `:256-270`), the record's row format and `teamproof`'s exit rule;
  - task 03 and 68: `hubapi.Conflict` (its `Tier`, sides and basis), `hubapi.Escalation`,
    `hubapi.Land`, `hubapi.Release.Attribution`;
  - task 58: the mirror's namespace and label (`task-58.md:360-370`);
  - task 59: the mirror's typed result (ST.7): pair, tier, basis shas, and whether P or H flagged
    it (**UNVERIFIED** field names until task 59's part is final);
  - task 62: `protocol.AgentConfigLine` and the decide's `agentConfig` (`task-62.md:145-156`,
    `:266-273`);
  - task 65 (**not on disk**): the bump job's result JSON; this part fixes the shape it needs
    (below) and asks task 65 for it;
  - task 66: keysmith's `402` body with `window`, route 77's rows, route 75's refusal
    (`task-66.md`).

#### `tm2.go`, the export (complete)

```go
package proof

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/hub/hubapi"
)

// MirrorCheck is M1's evidence: the real mirror pod and policy, the probe
// pod's answers, and every machine pod's volume claims.
type MirrorCheck struct {
	Ran            bool              `json:"ran"`
	Pod            json.RawMessage   `json:"pod"`            // kubectl get pod -o json, the mirror
	Policy         json.RawMessage   `json:"policy"`         // its NetworkPolicy
	ProjectClaim   string            `json:"projectClaim"`   // the mirror's own PVC name
	FetchSecret    string            `json:"fetchSecret"`    // the fetch credential's Secret name
	Probes         map[string]string `json:"probes"`         // target → "timeout" | "connected" | "refused"
	MachineClaims  []string          `json:"machineClaims"`  // every PVC any machine pod mounts
}

// MirrorPair is one pair the mirror was asked about (task 59's result log).
type MirrorPair struct {
	ConflictID string   `json:"conflictId"` // "" when no conflict was raised
	Owners     [2]string `json:"owners"`
	FlaggedBy  string   `json:"flaggedBy"`  // "P" | "H" | "" (never asked unflagged)
	Tier       string   `json:"tier"`       // "M" when merge-tree ran
	BasisShas  []string `json:"basisShas"`
}

type LandCheck struct {
	Ran        bool            `json:"ran"`
	Lands      []hubapi.Land   `json:"lands"`
	RemoteHead string          `json:"remoteHead"` // the branch head the git host reports
	Pushers    map[string]string `json:"pushers"`  // sha → the host's pusher identity
	Identities map[string]string `json:"identities"` // Hub userId → that person's git host identity
	Release    *hubapi.Release `json:"release"`    // reserved by hand at the land's commit
}

type BumpCheck struct {
	Ran      bool     `json:"ran"`
	Gates    []Gate   `json:"gates"`    // in the order they ran
	Promote  string   `json:"promote"`  // the reference it would promote
	DryRun   bool     `json:"dryRun"`
	PoolBefore string `json:"poolBefore"` // the warm pool's image reference before and after
	PoolAfter  string `json:"poolAfter"`
}

type Gate struct {
	Name   string `json:"name"`   // od8-canary | panacea-channel | g9-corpus | gateway-compat
	Passed bool   `json:"passed"`
}

type QuotaCheck struct {
	Ran          bool               `json:"ran"`
	LoweredUser  string             `json:"loweredUser"`
	Refusal      *MintRefusal       `json:"refusal"`      // the lowered member's mint
	TeammateMint bool               `json:"teammateMint"` // the other member minted after it
	Rows         []hubapi.UsageRow  `json:"rows"`         // route 77, that day
	QuotaCents   int64              `json:"quotaCents"`
	Ceiling      *CeilingRefusal    `json:"ceiling"`      // route 75 above the ceiling
}

type MintRefusal struct {
	Status int    `json:"status"`
	Window string `json:"window"`
}

type CeilingRefusal struct {
	Status int    `json:"status"`
	Reason string `json:"reason"`
}

// Export2 is one TM2 run. Conflicts and escalations are the Hub's records.
type Export2 struct {
	RunID       string              `json:"runId"`
	ProjectID   string              `json:"projectId"`
	People      [2]Person           `json:"people"`
	Mirror      MirrorCheck         `json:"-"`
	Pairs       []MirrorPair        `json:"-"`
	Conflicts   []hubapi.Conflict   `json:"-"`
	Escalations []hubapi.Escalation `json:"-"`
	Land        LandCheck           `json:"-"`
	Bump        BumpCheck           `json:"-"`
	Quota       QuotaCheck          `json:"-"`
}

var Files2 = []string{"tm2-run.json", "mirror.json", "mirror-pairs.json", "conflicts.json",
	"escalations.json", "land.json", "bump.json", "quota.json"}

func Load2(dir string) (Export2, error) {
	var e Export2
	dst := []any{&e, &e.Mirror, &e.Pairs, &e.Conflicts, &e.Escalations, &e.Land, &e.Bump, &e.Quota}
	for i, name := range Files2 {
		b, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			return Export2{}, fmt.Errorf("proof: %s: %w (a half-collected run is not judged)", name, err)
		}
		if err := json.Unmarshal(b, dst[i]); err != nil {
			return Export2{}, fmt.Errorf("proof: %s: %w", name, err)
		}
	}
	return e, nil
}
```

The collector writes `conflicts.json` as `null` when the run skipped item 2, which M2 reads as
`NOT RUN`, and `mirror-pairs.json` as `[]` when the mirror was never asked, which M2 reads as
`FAIL`.

#### `tm2.go`, the verdicts (complete)

```go
var digestRef = regexp.MustCompile(`@sha256:[0-9a-f]{64}$`)

var bumpGates = []string{"od8-canary", "panacea-channel", "g9-corpus", "gateway-compat"}

// TM2 returns the six rows, in order. It never returns fewer.
func TM2(e Export2) []Verdict {
	return []Verdict{m1(e), m2(e), m3(e), m4(e), m5(e), m6(e)}
}

func m1(e Export2) Verdict {
	v := Verdict{Row: "M1"}
	m := e.Mirror
	if !m.Ran {
		return notRun(v)
	}
	spec, err := parsePod(m.Pod) // volumes, automount, env and envFrom names, containers' mounts
	if err != nil {
		return fail(v, "the mirror pod's spec could not be read")
	}
	for _, vol := range spec.Volumes {
		if vol.Claim != m.ProjectClaim && vol.Secret != m.FetchSecret {
			return fail(v, "the mirror mounts "+vol.Name+", which is neither its disk nor the fetch credential")
		}
	}
	if spec.AutomountToken || spec.NamesGatewayToken {
		return fail(v, "the mirror holds a service-account or gateway token")
	}
	pol, err := parsePolicy(m.Policy)
	if err != nil || pol.HasEgressRule || !pol.DeniesEgress || !pol.IngressOnlyFleetControl {
		return fail(v, "the mirror's NetworkPolicy allows more than Fleet's control route in")
	}
	if len(m.Probes) == 0 {
		return fail(v, "no egress probe ran")
	}
	for _, target := range sortedKeys(m.Probes) {
		if m.Probes[target] != "timeout" {
			return fail(v, "the probe pod reached "+target+" ("+m.Probes[target]+")")
		}
	}
	for _, c := range m.MachineClaims {
		if c == m.ProjectClaim {
			return fail(v, "a machine pod mounts the mirror's disk")
		}
	}
	v.Result, v.Why, v.Evidence = Pass, "only its disk and the fetch credential; no egress", []string{m.ProjectClaim}
	return v
}

func m2(e Export2) Verdict {
	v := Verdict{Row: "M2"}
	if e.Conflicts == nil {
		return notRun(v)
	}
	for _, p := range e.Pairs {
		if p.FlaggedBy != "P" && p.FlaggedBy != "H" {
			return fail(v, "the mirror was asked about an unflagged pair")
		}
	}
	for _, c := range e.Conflicts {
		if string(c.Tier) != "M" || !twoOwners(c, e.People) {
			continue
		}
		for _, p := range e.Pairs {
			if p.ConflictID == c.ID && p.Tier == "M" && sameShas(p.BasisShas, basisOf(c)) {
				v.Result, v.Why, v.Evidence = Pass, "tier M on two people's lanes", []string{c.ID}
				return v
			}
		}
	}
	return fail(v, "no tier-M conflict")
}
```

