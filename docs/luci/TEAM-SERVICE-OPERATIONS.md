# Team service operations

Luci's MIT self-hosted teams work without a subscription or external account.
Every newly created team has effective service state `self-hosted`, `active`,
revision `0`. Existing identity files without service state retain that behavior.

Super Admins can open **Admin → This machine → Team service access on this
machine** to inspect teams and explicitly change service access. Choose the
team, select an access state, record a reason, then save. A stale revision fails
without changing the saved state; reload before reviewing and retrying. The
selected remote backing's Admin view does not expose these local controls.

These controls manage operator admission; changing a service record does not
create a subscription or verify dedicated host isolation. A configured managed
installation also enforces the separate [Stripe subscription](STRIPE-BILLING.md)
receipt. Payment cannot override operator suspension, and an operator
self-hosted label cannot bypass configured billing. Managed paid availability
still requires the separate production roadmap gates.
The existing team workers use the installation's Docker host; worker and private
directory separation alone is not proof of a boundary between paying tenants.

## Commission one installation for one team

The first managed configuration uses one entire Luci installation on a dedicated
team host. Provisioning the host, checking its identity, sufficient capacity and
exclusive resource ownership remain separate operator gates; the binding below
does not create or verify a VM.

1. Prepare the dedicated host with the supported Luci version and its own private
   state directory, Docker engine and persistent storage. Bootstrap its owner,
   create exactly one team, and grant the intended members access through the
   supported management API or UI. Preserve the returned opaque team ID.
2. Stop the service and any preparation workloads. Keep a private state backup;
   do not commission an existing installation containing other teams. The
   service refuses binding when the selected team is missing or another team
   record exists. Personal project files are retained but become inaccessible
   through this installation after binding; export them before commissioning.
3. Start the service with `LUCI_MANAGED_TEAM_ID` set to the actual existing team
   ID and the same absolute `LUCI_STATE_DIR`. Successful startup writes the
   private `control-plane/managed-team.json` record with version, team ID and
   commissioning time. There is no HTTP endpoint to change that assignment.
4. Verify `GET /v1/teams` returns `managedTeamId`, that personal project/session
   routes and creation of another team return `403`, and that authorized members
   can use the selected team's workspace. A person without membership receives
   the bound ID and an empty team list, with no personal fallback.
5. Record actual host identity, ownership, capacity, backup/restore and upgrade
   evidence before offering paid access. Then use the explicit operator service
   controls for active/suspended execution; the binding itself creates no bill.

The persisted binding remains enforced when the environment variable is omitted
on later restarts. A conflicting setting, malformed/partial marker, missing team
or multiple team records stops startup. Reassign another team to a separate
installation; do not edit the private marker to repurpose a live team's host.
The child team worker receives its own fixed `LUCI_TEAM_ID`; it does not inherit
the controller-only `LUCI_MANAGED_TEAM_ID` startup setting.

On a bound installation personal API routes, personal worker startup, personal
terminal tickets and creation of additional teams are unavailable to every
account, including Super Admins. Authentication and explicit operator/account
management remain available. Binding does not grant any membership or project
permissions, and it cannot bypass service suspension.

## Supported service states

| State | Behavior |
| --- | --- |
| Self-hosted active | No commercial restriction on an installation without managed billing. Configured managed billing still applies regardless of this operator label. Existing identity, project permissions and resource policy always apply. |
| Managed active | Operator permits execution subject to configured billing and project capabilities. The label alone proves neither payment nor host isolation. |
| Managed suspended | Blocks new agent sessions, terminal/browser access, inference, checks, deployment, restore/rebuild and credential replacement. |

Suspended teams retain authorized context reads and edits, project/context
exports, data backup downloads, explicit backup creation, owned stop/cancel and
read-only refresh operations. They can remove saved model connections, clear
direct provider API keys, disconnect the Codex account and remove application
configuration values. Removal-only configuration updates cannot add or replace
another value. These exceptions never grant membership, project access,
execution, secret or deployment capabilities the actor did not already have.

Saving suspension first commits private identity state and audit, then cancels
in-flight team requests and closes registered terminal/browser connections and
pending connection tickets. Admission and socket upgrade recheck current service
state. Persistence failure retains previous live access and durable state.

Already running containers, deployments and detached jobs are retained. Use
their explicit owned stop/cancel controls; suspension does not guarantee that
provider charges stop. It never deletes project files, databases, backups,
volumes, membership or project grants. Resume by saving managed active at the
current revision; configured billing must also permit execution. Members can
inspect and refresh service status in the team
workspace; operator reasons are visible to members and must contain no secrets.

## API

All routes require normal authenticated management identity. API reads do not
provision resources or start project work.

* `GET /v1/admin/team-services`: Super Admin only. Returns
  `{teams:[{teamId,name,service}],billingConnected:boolean,executionIsolationVerified:false}`.
  This operational catalogue exposes no project content, member catalogue or
  secret values and grants no team membership. `billingConnected` reports whether
  Stripe billing is configured, not a verified charge or team entitlement.
* `PUT /v1/admin/team-services/{teamId}`: Super Admin only. Body
  `{expectedRevision,mode,status,reason}`. Mode is `self-hosted` or `managed`;
  status is `active` or `suspended`. Self-hosted suspended is invalid. Reason is
  1–240 characters. Successful writes increment service revision and return the
  saved service record. Revision conflict is `409 team_service_changed`.
* `GET /v1/teams/{teamId}/service`: current team members only. Returns
  `{mode,status,revision,reason?,updatedAt?,updatedBy?,billingEnabled?}`. Service revision is
  independent of membership/project permission revisions. `billingEnabled:true`
  signals configured billing; read the separate billing status for its verified
  subscription outcome.

The service record lives in private management identity state, not project
exports. A project import cannot activate managed execution or establish a paid
subscription. Unknown or malformed persisted service states prevent startup.
Suspended execution returns `403 team_service_suspended`; lost membership and
project grants retain their existing separate authorization failures.

## Verification and remaining production gates

`go test -race ./control-plane/internal/management` covers default self-hosted
access, Super Admin enforcement, revision conflicts, restart persistence,
pre-worker denials, cleanup authorization, durable-before-revoke ordering,
admission races and exact bounded action-body forwarding. Frontend acceptance
tests exercise deliberate operator changes, stale revisions, duplicate-submit
prevention, current-team status and late-response isolation.

The [Stripe implementation](STRIPE-BILLING.md) includes explicit price/terms
acknowledgement, reserved checkout, signed provider events, reconciliation,
expiry and cancellation admission. Its official-SDK HTTP fixture tests pass
(`artifacts/onboarding/billing-sdk-http-tests.log`); they make no charge and do
not establish real merchant or webhook commissioning. These operator service
controls themselves do not contact a payment provider or create cloud resources.

[Private host recovery](MANAGED-HOST-RECOVERY.md) and
[reviewed state removal](MANAGED-DATA-LIFECYCLE.md) are implemented. The compiled
Linux service and real Docker fixture passed seven backup/restore/deletion
checks in `artifacts/onboarding/managed-host-recovery-erasure/result.json`, bound
to service SHA-256
`a53f88ae4554435373d5cf691bed86ef14fd81ab5bc102134b7916ac408a7b8d`.
That receipt covers writer refusals, owner/member access and context after
restoration, cancellation defeating a stale deletion backup, and explicit
verified state removal with retained proofs. It is development evidence; it
does not prove customer-host recovery time, an upgraded package, external data
restoration or removal of provider resources.

Real Stripe test/live commissioning, reviewed published commercial terms,
independently verified dedicated host assignment, matching-package upgrade,
measured production capacity and operator support commitments retain their
separate production gates.
