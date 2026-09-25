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
	"strings"

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
	Agents      []hubapi.AgentStart `json:"-"` // route 57: each ag-…'s owner
	Conflicts   []hubapi.Conflict   `json:"-"`
	Escalations []hubapi.Escalation `json:"-"`
	Land        LandCheck           `json:"-"`
	Bump        BumpCheck           `json:"-"`
	Quota       QuotaCheck          `json:"-"`
}

var Files2 = []string{"tm2-run.json", "mirror.json", "mirror-pairs.json", "agents.json",
	"conflicts.json", "escalations.json", "land.json", "bump.json", "quota.json"}

func Load2(dir string) (Export2, error) {
	var e Export2
	dst := []any{&e, &e.Mirror, &e.Pairs, &e.Agents, &e.Conflicts, &e.Escalations, &e.Land, &e.Bump, &e.Quota}
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
		if string(c.Tier) != "M" || !twoOwners(c, e) {
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

func m3(e Export2) Verdict {
	v := Verdict{Row: "M3"}
	l := e.Land
	if !l.Ran {
		return notRun(v)
	}
	for _, ld := range l.Lands {
		if ld.MergeCommit != l.RemoteHead {
			continue
		}
		if pusher, want := l.Pushers[ld.MergeCommit], l.Identities[ld.OwnerUserID]; pusher == "" || pusher != want {
			return fail(v, "the push of "+ld.ID+" was not made by its owner's identity")
		}
		if l.Release == nil || l.Release.Attribution.Status != hubapi.AttributionListed {
			return fail(v, "the release at the land's commit lists no lands")
		}
		for _, a := range l.Release.Attribution.Lands {
			if a.LandID == ld.ID && a.Owner.ID == ld.OwnerUserID && a.TaskID != nil && a.BriefRef != nil {
				v.Result, v.Why = Pass, "landed through the mirror, pushed by its owner, attributed on the release"
				v.Evidence = []string{ld.ID, ld.MergeCommit, l.Release.ID}
				return v
			}
		}
		return fail(v, "the release does not attribute "+ld.ID+" with its task and brief section")
	}
	return fail(v, "no land's merge commit is the remote head")
}

func m4(e Export2) Verdict {
	v := Verdict{Row: "M4"}
	if e.Escalations == nil {
		return notRun(v)
	}
	for _, es := range e.Escalations {
		if es.Kind != hubapi.EscalationDecide || es.FromAgentID != nil || !strings.Contains(es.Question.String(), " builds on ") {
			continue
		}
		for _, p := range es.AgentConfig {
			if p.Path == ".claude/settings.json" {
				if others := sameQuestionElsewhere(e.Escalations, es); len(others) > 0 {
					return fail(v, "the build-on decide reached a second person: "+others[0])
				}
				v.Result, v.Why, v.Evidence = Pass, "build-on is a decide listing .claude/settings.json", []string{es.ID}
				return v
			}
		}
		return fail(v, "a build-on decide without .claude/settings.json on its own line: "+es.ID)
	}
	return fail(v, "no build-on decide")
}

func m5(e Export2) Verdict {
	v := Verdict{Row: "M5"}
	b := e.Bump
	if !b.Ran {
		return notRun(v)
	}
	if len(b.Gates) != len(bumpGates) {
		return fail(v, fmt.Sprintf("%d gates ran, not %d", len(b.Gates), len(bumpGates)))
	}
	for i, g := range b.Gates {
		if g.Name != bumpGates[i] || !g.Passed {
			return fail(v, "gate "+bumpGates[i]+" did not pass in its place")
		}
	}
	switch {
	case !digestRef.MatchString(b.Promote):
		return fail(v, "the reference to promote is not a digest: "+b.Promote)
	case !b.DryRun || b.PoolBefore != b.PoolAfter:
		return fail(v, "the dry run changed the warm pool")
	}
	v.Result, v.Why, v.Evidence = Pass, "four gates passed; a digest, not promoted", []string{b.Promote}
	return v
}

func m6(e Export2) Verdict {
	v := Verdict{Row: "M6"}
	q := e.Quota
	if !q.Ran {
		return notRun(v)
	}
	switch {
	case q.Refusal == nil || q.Refusal.Status != 402 || q.Refusal.Window != "day":
		return fail(v, "no day-window refusal at the gateway")
	case !q.TeammateMint:
		return fail(v, "the teammate was refused too: the quota was not per member")
	case q.Ceiling == nil || q.Ceiling.Status != 422 || q.Ceiling.Reason != "above_org_ceiling":
		return fail(v, "a quota above the ceiling was not refused")
	}
	var cents int64
	for _, r := range q.Rows {
		if r.UserID == q.LoweredUser {
			cents += r.CostCents
		}
	}
	if cents < q.QuotaCents {
		return fail(v, fmt.Sprintf("the Hub rows show %d cents, below the %d-cent quota", cents, q.QuotaCents))
	}
	v.Result, v.Why, v.Evidence = Pass, "refused for today at the gateway; the teammate still minted", []string{q.LoweredUser}
	return v
}
```

The helpers, same file: `notRun(v)` and `fail(v, why)` set `Result` and `Why`; `sortedKeys`
(`sort.Strings`, so the first failing probe is named the same way every run); `twoOwners(c, e)`
is true when `AgentA` and `AgentB` belong, by `e.Agents`' `OwnerUserID`, to the two different
people of `e.People` (M2 calls it with `e`, and its signature is `twoOwners(c hubapi.Conflict, e
Export2) bool`); `basisOf(c)` is `[]string{c.Basis.A, c.Basis.B}`; `sameShas` compares as sets;
`sameQuestionElsewhere` lists the ids of escalations with the same `ThreadID` and another
`OwnerUserID`; `parsePod` and `parsePolicy` read the facts each needs from the raw JSON, with table
tests over canned specs. `a.Owner.ID` is the Hub `User`'s
id field as `reservedBy` spells it, **UNVERIFIED** in its name; `Question.String()` is
`teamtext.Text`'s accessor (task 02), likewise. M6's refusal comes from the collector calling
keysmith's mint as that member through the operator's test harness, and its `QuotaCents` is the
lowered quota; the Hub row may lag by ten minutes (task 66 decision f), so the collector waits for
the next post before writing `quota.json`.

**`cmd/teamproof/main.go`, the one arm** (task 57's file): `teamproof tm2 -dir <export>` calls
`proof.Load2` and prints `proof.TM2`'s six rows in the record's table format, exiting 0 only when
all six are `PASS`, as the `tm1` arm does for its rows.

#### The tests, first (each names the mutant it kills)

`tm2_test.go`, over canned exports in `testdata/tm2/`: `pass/` (every row `PASS`) and one
directory per failure below, each a copy of `pass/` with one file changed.

| Test | Asserts | Mutant killed |
|---|---|---|
| `TestTM2AlwaysSixRows` | an export with every `ran: false` and `null`s: six rows, all `NOT RUN` | a row dropped when its item did not run |
| `TestTM2PassOnTheCannedRun` | `pass/`: six `PASS`, each with at least one evidence id | a verdict that passes without evidence |
| `TestM1ExtraVolumeFails` | the mirror pod mounts an `emptyDir` named `cache` besides its disk: `FAIL`, names `cache` | a check of the Secret only |
| `TestM1AutomountFails` | `automountServiceAccountToken` absent (Kubernetes' default is true): `FAIL` | an absent field read as false |
| `TestM1ConnectedProbeFails` | `1.1.1.1:443` answered `connected`: `FAIL` naming it; `refused` also fails | "refused" counted as isolated (a refusal proves a route exists) |
| `TestM1MachineMountsMirrorDiskFails` | one machine pod lists the mirror's claim: `FAIL` | the "no sandbox mount" half of the item never checked |
| `TestM2UnflaggedPairFails` | one pair with `flaggedBy: ""`: `FAIL` | work growing with the square of the team unnoticed (task 60's rule) |
| `TestM2SameOwnerDoesNotCount` | the only tier-M conflict is between two of one person's lanes: `FAIL (no tier-M conflict)` | a solo overlap passing as the team proof |
| `TestM3PushedByFetchIdentityFails` | the host's pusher for the head is the fetch credential's service identity: `FAIL` | a push with the read-only credential, or with anyone's but the owner's |
| `TestM3ReleaseWithoutSectionFails` | the release lists the land with `briefRef: null`: `FAIL` | the provenance thread stopping short of the release |
| `TestM4DecideToBothFails` | the same build-on question in both people's escalations: `FAIL` | a decide shown to someone who is not the builder's human |
| `TestM4MissingSettingsLineFails` | the decide's `agentConfig` lists `CLAUDE.md` only: `FAIL` | any agent-config path accepted as the one the item names |
| `TestM5TagFails` | `promote` is `…/sandbox:2.1.4`: `FAIL` | a tag promoted (the scope's rule: a digest, never a tag) |
| `TestM5GateOrderAndCount` | three gates; and four in another order: both `FAIL` | a skipped gate; the canary run last |
| `TestM5PoolChangedFails` | `poolAfter` differs from `poolBefore` with `dryRun: true`: `FAIL` | a dry run that promoted |
| `TestM6TeammateRefusedFails` | `teammateMint: false`: `FAIL` | a project-wide stop passing as a per-member quota |
| `TestM6MonthWindowFails` | the refusal's `window` is `month`: `FAIL` | the monthly budget mistaken for the daily quota |
| `TestLoad2RefusesHalfARun` | `pass/` without `bump.json`: an error naming it | a run judged on part of its evidence |

#### `infra/smoke/team/tm2-collect.sh` (complete)

```bash
#!/usr/bin/env bash
# Fetches what a TM2 run left behind into ./export. Reads only; writes nothing
# to the cluster, the Hub or the git host. jq is not assumed (task 57's (a)).
set -euo pipefail
: "${HUB:?}" "${PROJECT:?}" "${AUDITOR_TOKEN:?}" "${OPERATOR_GIT_TOKEN:?}" "${RUN_ID:?}"
out=./export; mkdir -p "$out"
hub() { curl --ssl-no-revoke -fsS -H "Authorization: Bearer $AUDITOR_TOKEN" "$HUB$1"; }

# M1: the mirror's spec and policy; every machine pod's claims; the probe pod's answers.
kubectl -n panacea-mirrors get pod -l panacea-app/component=mirror,panacea-app/project="$PROJECT" -o json > "$out/mirror-pod.raw.json"
kubectl -n panacea-mirrors get networkpolicy -l panacea-app/component=mirror -o json > "$out/mirror-policy.raw.json"
kubectl -n panacea-sandboxes get pods -o jsonpath='{range .items[*]}{range .spec.volumes[*]}{.persistentVolumeClaim.claimName}{"\n"}{end}{end}' > "$out/machine-claims.txt"
bash infra/smoke/team/tm2-mirror-probe.sh > "$out/mirror-probes.txt"   # target<TAB>answer, one a line

# M2, M4: Hub records, read as the org auditor (task 57's (b)).
hub "/v1/projects/$PROJECT/agents"      > "$out/agents.raw.json"
hub "/v1/projects/$PROJECT/conflicts"   > "$out/conflicts.raw.json"
hub "/v1/projects/$PROJECT/escalations" > "$out/escalations.raw.json"
kubectl -n panacea-mirrors logs -l panacea-app/component=mirror --tail=-1 | grep '"msg":"pair"' > "$out/mirror-pairs.ndjson" || true

# M3: lands, the release reserved by the script, and the git host's push record.
hub "/v1/projects/$PROJECT/lands?branchRef=refs/heads/main" > "$out/lands.raw.json"
hub "/v1/releases/$(cat ./release-id)" > "$out/release.raw.json"
bash infra/smoke/team/tm2-git-host.sh > "$out/git-host.json"            # head sha, pusher per sha

# M5, M6: the bump job's result and the quota probe's answers, as the run wrote them.
cp ./bump-result.json "$out/bump.raw.json"
cp ./quota-probe.json "$out/quota.raw.json"

go run ./control-plane/cmd/teamproof tm2-pack -dir "$out" -run "$RUN_ID"   # raw → the nine Files2
```

`tm2-pack` is a second subcommand in `teamproof` that turns the raw files into `Files2`'s nine
(the Hub's page envelopes unwrapped, following `next` with the same token, ndjson to an array, the
probe text to a map, `null` for an item the script marks skipped), so all parsing is Go with tests
and the shell only fetches. The mirror's `panacea-app/project` label is **UNVERIFIED** against
task 58, which fixes only `panacea-app/component: mirror`; the collector uses whatever per-project
label task 58 sets. `tm2-mirror-probe.sh` starts the probe pod
(decision b) and runs one `timeout 3 bash -c '</dev/tcp/host/port'` per target; `tm2-git-host.sh`
reads GitHub's `GET /repos/{o}/{r}/activity?ref=main` or Azure DevOps' `GET …/pushes?searchCriteria.refName=refs/heads/main`
with `OPERATOR_GIT_TOKEN` (**UNVERIFIED**: the endpoints are as remembered, and the run's first
step is to confirm them). Both helper scripts are written in this task, 20 lines each.

#### `infra/smoke/team/tm2-script.md`, its fixed order

1. **Host.** Name the git host (GitHub or Azure DevOps) and confirm `tm2-git-host.sh`'s endpoint
   answers with the operator's own token. No other step starts until it does.
2. **M1.** Start the team project; wait for its mirror pod `Running`; run the collector's M1 lines.
3. **M2.** The sharer's and the joiner's Orchestrators are each told, in their own words, to
   change `web/auth/session.go` (the intent is fixed, the words are private, OD-24). Both commit.
   Wait for a `cf-…` at tier M, or 15 minutes.
4. **M3.** The joiner lands through the queue and clicks **Land**; the sharer reserves a run
   release at the new head with a direct `POST /v1/projects/{projectId}/releases`, writes its id
   to `./release-id`, then abandons it (decision d).
5. **M4.** The sharer's lane changes `.claude/settings.json`; the joiner's Orchestrator is told to
   build on it. The joiner answers **Hold**, so nothing is applied.
6. **M5.** The operator queues task 65's bump job with its dry-run parameter and saves the result
   JSON as `./bump-result.json`.
7. **M6.** The member quota is one number for every member (task 66), so the script makes the
   two members' spend differ: after steps 3–5 the sharer has spent more today. A team admin sets
   the member quota between the two figures task 67's view shows; the sharer's next mint is
   refused, the joiner's next mint succeeds; a `PUT` of the member quota above the org ceiling is
   sent and its answer kept. The probe writes `./quota-probe.json`.
8. Run the collector; run `teamproof tm2 -dir ./export`; paste the output into the record; delete
   `./export`, which holds two people's agents' records.

#### `docs/verification/2026-09-25-phase-t-tm2-results.md` (complete template)

```markdown
# Phase T — TM2 results

Each run is appended, never replaced. A later PASS does not remove an earlier FAIL.

### Run 1 — NOT RUN

| Field | Value |
|---|---|
| Run id | — |
| Date (UTC) | — |
| People | — (two, by role: the sharer and the joiner) |
| Git host | — (GitHub or Azure DevOps) |
| Deployed commit | — |
| Operator | — |

Paste `teamproof tm2` output here, unedited:

| Row | Result | Why | Evidence |
|---|---|---|---|
| M1 | NOT RUN | | |
| M2 | NOT RUN | | |
| M3 | NOT RUN | | |
| M4 | NOT RUN | | |
| M5 | NOT RUN | | |
| M6 | NOT RUN | | |

### Observed, and not observed

Per item, in words, after the run: what was seen, and what the row does **not** show.

- M1: — (not observed: what the mirror binary itself attempts; the probe pod stands in for it)
- M2: —
- M3: — (not observed: phase 7's Ship; the release was reserved by hand)
- M4: —
- M5: — (not observed: a real promotion; this was a dry run)
- M6: — (not observed: the overrun bound, N × the slice, task 66)

## What these results do not show

- That the mirror resists a hostile lane bundle; only that it has no way out.
- That the bump job has promoted anything, or how it learns of a release (`05-design.md` §10.4).
- That quotas hold to the cent: the gateway's cache and sweep allow a bounded overrun (task 66).
```

#### Steps

1. Write `tm2_test.go` and the canned exports; run
   `go test ./internal/fleet/proof/ -run 'TM2|M[1-6]|Load2'`; each fails (no `TM2`).
2. Write `tm2.go`, then the `tm2` and `tm2-pack` arms in `cmd/teamproof/main.go`; green.
3. Write the collector, its two helper scripts, the run script and the record template. Commit.
4. **The run** (not in the estimate): two people follow the script on the real environment once
   tasks 58–68 are deployed; paste the table; commit the record.

#### Acceptance

```bash
cd control-plane
go test ./internal/fleet/proof/ -run 'TM2|M[1-6]|Load2' -count=1 -v 2>&1 | grep -c '^--- PASS'
# expect 18
go run ./cmd/teamproof tm2 -dir internal/fleet/proof/testdata/tm2/pass; echo "exit $?"
# expect six PASS rows, then "exit 0"
go run ./cmd/teamproof tm2 -dir internal/fleet/proof/testdata/tm2/m5-tag; echo "exit $?"
# expect M5 FAIL naming the tag, then "exit 1"
bash -n ../infra/smoke/team/tm2-collect.sh && echo syntax-ok   # expect syntax-ok
```

Until two people run the script, every row in the record says `NOT RUN`, and TM2 is not done.

#### What this task does NOT claim

- **That TM2 works.** It builds the means of proving it; the proof is the run.
- **That M1's probe is the mirror.** It is a pod the same policy selects (decision b).
- **That M3 covers phase 7.** The release is reserved by hand; phase 7 has no plan (task 68).
- **That M5's job can learn of a release.** How it does is **UNVERIFIED** in the skeleton's own
  scope (`:1089-1090`); this dry run is started by hand.

#### Gaps, raised here and not resolved alone

1. **The skeleton's file map** lists only the record for task 69; the collector, its helpers, the
   run script and `proof/tm2.go` are added here.
2. **Task 65 is not on disk.** M5 needs its result as JSON with the four gate names in order, the
   reference it would promote, a dry-run flag and the warm pool's reference before and after
   (`BumpCheck` above). Task 65 should emit exactly that.
3. **Task 59's result log** must carry, per pair, the conflict id, the two owners, which tier
   flagged it and the basis shas, as one `"msg":"pair"` line, for M2 to read.
4. **The git host's push record** is the only evidence of *who* pushed; its endpoints are
   **UNVERIFIED**, and a host that does not expose the pusher cannot pass M3 as written.

