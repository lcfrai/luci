### Task 32: Phase 8 delta — Fleet's chart, the private gateway path, the Orchestrator's egress

**Slice:** TM1-α. **Focused time:** 3 h. **dependsOn:** 12 (`fleetd`'s variables), P8-11 (the
platform chart), P8-29 (the broker and Hub charts, the pattern this chart copies), P6-19 (the
gateway chart and its NetworkPolicies). **Seams:** P8-D12 (sandbox egress), P6-D1 (the gateway is
never exposed).

**Built after the sync-merge of `feat/env` and `feat/phase-6-gateway`.** None of the files this
task extends exists yet: `deploy/` is absent on `feat/env` (read 2026-09-25), and phase 6's chart is
a plan row. Every sibling path below cites the sibling **plan** and is **UNVERIFIED until merge**:
the platform chart is `.claude/worktrees/env/docs/superpowers/plans/2026-09-23-phase-8-own-environment-parts/task-11.md`
(values at `:36-83`, the sandbox policies at `:147-164`), the broker and Hub charts are that
directory's `task-29.md` (the Hub's values at `:121-150`, its environment and workload identity at
`:152-164`, its migrate hooks at `:166-183`, its policy at `:185-191`), the identity module and the
workloads stack are its `task-10.md` (`:91-105`, `:107-158`), and the gateway chart is
`.claude/worktrees/phase-6/docs/superpowers/plans/2026-09-25-phase-6-gateway.md:262`.

Fleet is the only new server team mode adds to the cluster, and it runs the way the Hub runs: two
replicas, a PDB, a user-assigned identity by workload identity, Entra-only Postgres, no secret in
an environment variable, default-deny with a named allow per direction, Australia East only
(§2.5's invariant). The chart also carries the two network facts team mode changes in phase 8:
a sandbox may now reach one service, the gateway's front, and nothing else in the cluster
(P8-3, `2026-09-25-phase-6-gateway.md:308`); and the `orchestrator` class reaches **only** that
service, with no internet at all.

Patterns only from Panacea (OD-13); no Panacea file is copied. This task applies nothing to a
cluster: task 33 installs it by the owner's runbook, through phase 8's `deploy` pipeline (P8-D3).

**Decisions made here.**

**(a) The Orchestrator needs no egress to Fleet.** The skeleton's scope says the `orchestrator`
class egresses "to the gateway and Fleet's control route only". The control stream is **inbound**:
Fleet dials the broker, which relays to `sandboxd`'s `/control` on port 8080 (task 17's `control`
relay kind, `task-17.md:25-27`; task 19's dial to `{BROKER_URL}/v1/sandboxes/{id}/control`,
`task-19.md:494`), and P8-D12 already admits the broker, and only the broker, on 8080. So the
Orchestrator's egress is kube-dns and the gatekeeper's port, and nothing else. **Raised as gap
G32-1** against the scope's wording; the property it wanted (Fleet's control path works, and
nothing else does) holds.

**(b) The Orchestrator has no internet.** Its tools are `Bash(panacea *)` and `Read` only
(`05-design.md:428`), its inference goes to the gateway, and its control comes in from the broker.
A pod with no route out cannot post a teammate's words anywhere, whatever it is persuaded of. The
platform chart's `egress-internet` selects every pod (`task-11.md:155-159`), and NetworkPolicy allows
are a union, so an extra policy cannot subtract from it: `egress-internet`'s `podSelector` becomes
`panacea-app/class NotIn [orchestrator]`. Pods with no class label (workers today, machines,
balloons) still match. Task 58 adds `mirror` to the same list. **Requires of task 15** that every
sandbox pod carry `panacea-app/class: <spec.Class>` (task 15's part does not yet name a label;
gap G32-2).

**(c) The gateway allow names pods, not an address.** `sandbox-egress-gateway` allows egress from
every pod in `panacea-sandboxes` to pods `app.kubernetes.io/name: gatekeeper` in namespace
`panacea-gateway` on TCP 4000 (phase 6's port, `2026-09-25-phase-6-gateway.md:262`). Cilium
matches in-cluster pods by identity (`task-11.md:161-164`, UNVERIFIED there too), so the rule
admits the gatekeeper and nothing else on the service CIDR; keysmith (4100) and LiteLLM (4001) stay
unreachable from a sandbox. The label is phase 6's to confirm at merge (**UNVERIFIED**).

**(d) Fleet has its own namespace.** `panacea-fleet`, created by the platform chart with the same
Pod Security labels and `default-deny` as the other four (`task-11.md:92-99`, `:137-145`), and
`panacea-app/edge-routes: "true"`, because the desktop reaches Fleet through the Gateway exactly as
it reaches the Hub. A namespace of its own gives Fleet's identity a subject nobody else's pod can
claim (`system:serviceaccount:panacea-fleet:fleet`) and gives its policies a selector no Hub pod
matches.

**(e) Its own database, on the Hub's server, and two identities, as the Hub does.** Fleet's
schema `fleet` lives in a database `fleet` on the Hub's Flexible Server (P8-D10: Entra-only, private
endpoint), so no Hub role can read it and no Fleet role can read the Hub's. A `fleet-migrate`
identity (DDL, owner of the schema) runs a pre-install Job; the serving `fleet` identity gets DML
only, and `INSERT, SELECT` alone on `policy_versions` (task 10's append-only grant,
`task-10.md:375-378`). The roles are created once per environment by a human admin, as the Hub's
are (P8-D10, `task-29.md:193`). **Requires of task 12** a `fleetd migrate` subcommand, and that the
serving process only verifies the schema is at head when `PANACEA_APP_FLEET_MIGRATE=job`: task 12's
`main.go` calls `pgstore.Migrate` on start (`task-12.md:819`), which a DML-only role cannot do
(gap G32-3).

**(f) Fleet calls the Hub and the broker at their https URLs.** `PANACEA_APP_FLEET_HUB_URL` and
`_BROKER_URL` must be `https` (`task-12.md:97`, `task-19.md:504`), and in the cluster TLS ends at
Traefik, so Fleet goes through the edge like the desktop, with the same token checks. Its policy
allows egress to Traefik's pods on the edge's HTTPS container port **and** to 443 outside the
private ranges; which one a pod actually takes depends on how `hub.<zone>` resolves inside the
cluster, **UNVERIFIED**, and task 33 records it. Keysmith is plain http inside its namespace
(`:4100`, no route), so that allow is direct.
