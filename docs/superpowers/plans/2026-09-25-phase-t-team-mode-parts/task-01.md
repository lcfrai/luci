### Task 01: ADR-H1 "delegated callers" amendment and the team-records contract

**Slice:** TM1-α. **Size:** 3 focused hours. **dependsOn:** the sync-merge of `feat/hub` into
`feat/team-mode`. **Seams defined:** ST.2 (the Hub's team records), ST.3 (delegation).

This task is documents only, and it is the gate for every line of team-mode code
(`05-design.md` §11.1 row 4, critique `06` I4). The Hub was designed with one caller, the user's
own laptop control plane, and four of its rules say so in plain words: `azp` must be the desktop's
client id (`ADR-H1:370`), a token with no `scp` is rejected (`:369`), the principal comes from the
token (`:372`) and no body carries an actor (`:434-435`), and tenancy is the token's org under RLS
(G4, `:253`). Team mode adds a second caller, Fleet, which writes in a member's name while that
member is away (`05-design.md` §2.4). If that caller arrives as code before it arrives as a rule,
each route will invent its own answer to *"whose write is this?"*, and at least one will take the
answer from the body. So the rule is written first, in the two documents the Hub is built from,
and tasks 03 to 08 build what it says.

Four decisions the design left open are made here, so no later task re-derives them.

**(a) Fleet is an app-only caller, and the amendment says so.** Fleet runs as an AKS workload
identity. Its Entra token is app-only (no `scp`), which `ADR-H1:369` refuses today. The Hub reads
the absence of `scp`, never `idtyp`: Entra emits `idtyp` only as a configured optional claim
(**UNVERIFIED** for our registration; task 05, decision (a)).
The amendment therefore changes **five** lines, not the four the design lists: `:369` gains the
app-only case, gated on an app role (`Hub.Fleet`) **and** on `azp` being Fleet's client id **and**
on the request naming a delegation. The design's §2.4 list misses `:369`; this is recorded as a
skeleton gap in the return.

**(b) Exactly one Fleet route names no delegation.** The per-org change feed (amendment (d)) is
read by Fleet for every team project in an org at once, so no single member's delegation can
cover it. It is the only exception, it serves metadata only (the feed never held text, contract
§8), and it is listed by name so a second exception needs a second amendment.

**(c) Records Fleet itself authors still name a delegation.** A conflict, a thread, a Fleet
escalation and a templated `answer` are written by Fleet, not by an agent. Each names the
delegation of the owner the record is **first about** (the conflict's `agentA`, the thread's
opener, the escalation's owner, the Orchestrator the answer speaks for) and records
`author = fleet`. The Hub's owner check is then *"at least that owner's `ag-…` is on the record"*,
so Fleet cannot write a conflict between two agents neither of which belongs to the named owner.

**(d) The audit record gains `via` and `delegationId` without moving a single existing digest.**
Contract §3.6's audit event is hashed into the G5 chain. A delegated write is canonicalised as
audit-event **`"v": 2`**, with two added fields; every non-delegated event stays `"v": 1`, byte for
byte, so the golden vectors in `hubapi/canonical_test.go` (contract §3.5) do not change.

**Files** (all on `feat/team-mode`, after the Hub's sync-merge; paths relative to the repo root):
- `docs/superpowers/adr/ADR-H1-hub-architecture.md` — a new **§5.10 Delegated callers**; the five
  replaced lines marked in place with a pointer to §5.10; risk **R12** in §8 (modified)
- `docs/superpowers/contracts/2026-09-23-hub-contract.md` — §3.1 prefix rows; §3.6 the audit
  event v2; §4.2 fifteen codes (the scope's six; `delegation_revoked`, so Fleet can tell
  *renew* from *stop*; seven `*_not_found`; `team_state`); §5.1 the delegated step; §5.2 two org
  capabilities; §5.3 the team rows; §7.0 a team routes table, rows 46 to 79;
  new **§7.13 to §7.26**; §8 the team events; §12.2 two variables (modified)
- `docs/superpowers/contracts/team-records-fieldcheck.txt` — one line per `05-design.md` §3.3
  field, the acceptance script's input (new)

The ADR and contract live at `.claude/worktrees/hub/docs/superpowers/…` on 2026-09-25
(`ADR-H1-hub-architecture.md`, 776 lines; `2026-09-23-hub-contract.md`, 2,298 lines). Every line
number below was read there on that date; step 1 re-reads them after the merge.

**Interfaces:**
- Produces (as documents; task 03 turns them into Go, task 04 into SQL, task 05 into auth):
  - **ST.3** — the `X-Panacea-Delegation: dl-…` header; the delegable route names of §7.13; the
    actor rule (`actor = owner, via = fleet, delegationId = dl-…`); the `ag-…` owner check
  - **ST.2** — every Hub-owned record of `05-design.md` §3.3: id prefix, fields, bounds, author
    class, route, audit event kind, feed kind
  - the fifteen problem codes of Step 4, in their table order: `delegation_required`,
    `delegation_expired`, `delegation_revoked`, `agent_not_owner`, `delegation_not_found`,
    `task_not_found`, `agent_not_found`, `lease_not_found`, `conflict_not_found`,
    `thread_not_found`, `escalation_not_found`, `team_state`, `text_refused`, `glob_refused`,
    `breadth_exceeded`
  - the app roles `Hub.Fleet`, `Hub.Auditor`, `Hub.Compliance`
  - **Id minting:** the Hub mints `dl`, `hm`, `nt`; Fleet mints the other nine with `ids.Wide`
    and sends each as the body's `id` (Step 4)
  - `PANACEA_APP_HUB_FLEET_CLIENT_ID`, `PANACEA_APP_HUB_DELEGATION_TTL` (contract §12.2)
- Consumes: the Hub's ADR and contract as merged; `05-design.md` §2.4, §3.3, §3.4, §11.1.
- Amends: `ADR-H1:253`, `:369`, `:370`, `:372`, `:434-435`, and contract §3.6's audit-event row.
  Both are locked documents; the owner accepts this amendment before task 03 starts
  (`CLAUDE.md`: the owner reviews plans before code).

- [ ] **Step 1: Re-read the lines this amendment replaces, after the merge**

The line numbers above were read on the Hub worktree. A sync-merge can move them. Run, from the
repo root:

```bash
~/.claude/tools/claudezsh/claudezsh 'sed -n "252,253p;369,372p;434,435p;467,474p;698,700p" docs/superpowers/adr/ADR-H1-hub-architecture.md | cut -c1-90'
```

Expected, in order: the G3 row (`| G3 | **An ordered change feed.**`), the G4 row (`| G4 |
**Postgres row-level security on`), the `scp` row, the `azp` row (`in
PANACEA_APP_HUB_ALLOWED_CLIENTS`), the `exp`/`nbf` row, the `Principal` row (`**(tid, oid)**`),
the two lines ending `no request body carries an actor, an approver or a recorder field**.`, the
A5 paragraph (`**What the desktop may cache`), and `## 8. Open risks`. If any line moved, use its
new number everywhere below and in the commit message; if any line's **text** changed, stop and
raise it: the amendment was written against the text, not the number.

- [ ] **Step 2: Write ADR-H1 §5.10, "Delegated callers"**

Insert it after §5.9 and before `## 6. Rejected alternatives`. The complete text:

```markdown
### 5.10 Delegated callers — Fleet writes in a member's name (team projects only)

*Amendment of 2026-09-25, for Phase T (OD-14, OD-16). It changes five lines of §4.2 and §5.2–§5.4,
each marked in place with "→ §5.10".*

A team project's agents run centrally, as managed sessions that Fleet coordinates (OD-14), and they
keep working while their owner's laptop is shut. Their records (tasks, agent starts and ends, lease
state changes, messages, directives) are still the owner's records. They reach the Hub through a
**delegation**: a record the owner creates with an ordinary call of their own, at launch, that
names the project, the session class, the routes Fleet may call on the owner's behalf, and an
expiry of 24 hours. Nothing about a solo project changes.

**The rule, in the order the Hub applies it.**

1. **A second caller, app-only, one identity.** A token with no `scp` is accepted only
   when its `azp` equals `PANACEA_APP_HUB_FLEET_CLIENT_ID` **and** its `roles` contain
   `Hub.Fleet`. Any other app-only token is refused, as before. *(Replaces `:369`, "A token with
   no `scp` … is rejected: the Hub is delegated-only", for this one identity.)*
2. **Fleet must name a delegation.** A request authenticated as Fleet carries
   `X-Panacea-Delegation: dl-…`, or is refused 401 `delegation_required`. The delegation must
   exist in the token's org, have `revokedAt` null (else 401 `delegation_revoked`), have
   `expiresAt` in the future (else 401 `delegation_expired`), and list the route being called
   (else 403 `forbidden`). **One route is exempt, by name**: `GET /v1/orgs/{orgId}/changes`, the
   per-org feed of rule 7. A desktop token that carries the header is refused 403 `forbidden`
   with `details.required = "fleet"`: the header is Fleet's, so no client can borrow a delegation.
   *(Replaces `:370`, "`azp` in `PANACEA_APP_HUB_ALLOWED_CLIENTS`", which stays true for every
   non-Fleet token.)*
3. **The principal is the delegation's owner.** Fleet's own `(tid, oid)` is never a principal and
   is never provisioned as a user. The handler runs as the owner, with the owner's memberships, and
   §5.3's matrix is applied to the owner. *(Replaces `:372` for delegated requests.)*
4. **The actor is recorded, never supplied.** Every audit event of a delegated write records
   `actorUserId = owner`, `via = "fleet"`, `delegationId = dl-…`, as audit-event `v: 2`
   (contract §3.6). A non-delegated event stays `v: 1`, byte for byte. **No request body carries an
   actor, an owner, a `via` or a delegation field**; strict decoding (contract §1, `httpx.DecodeJSON`)
   refuses one that does with 400 `bad_request`. *(Extends `:434-435`; it does not relax it.)*
5. **An agent id must be the owner's.** Every team write that names an `ag-…` (a task's
   Orchestrator, an agent start's parent, a message's sender, a directive's two ends, a conflict's
   pair, a thread's opener) is checked **in the write's transaction** against the agent-start rows:
   at least the record's first-named agent belongs to the delegation's owner, and every agent the
   record says *acts* (sender, directive `from` and `to`) belongs to them. A mismatch is 403
   `agent_not_owner`, and nothing is written. A Fleet mis-stamp therefore fails closed at the Hub.
6. **Tenancy comes from the delegation.** For a delegated request the Hub sets
   `hub.org_id` (contract §10.3) from the delegation's org, looked up by a `SECURITY DEFINER`
   function, and refuses one whose org is not the token's `tid`'s enrolled org. *(Replaces `:253`'s
   "tenancy from the token" for delegated requests; RLS itself is unchanged.)*
7. **Fleet's feed.** `GET /v1/orgs/{orgId}/changes?after=&kinds=` serves Fleet, and only Fleet, the
   G3 feed of every **team** project in the org, metadata only, as contract §7.11 serves a member.
   It is how Fleet sees a revoked delegation or a removed member within one poll. *(Extends `:252`.)*
8. **`devissuer` issues client-credentials tokens.** For the offline stack (§5.7), `devissuer` gains
   `grant_type=client_credentials` for one configured client, minting the app-only token of rule 1,
   so the Hub's offline proof exercises this path without Entra.

**What does not change.** `ADR-H1:467-474` holds: Fleet is a client, persists no Hub record, keeps
Hub records in memory only and rebuilds from the Hub after a restart; its own Postgres holds Fleet
state and the outbox of Hub-bound writes. The Hub still never sees a terminal byte (`:88`).
```

- [ ] **Step 3: Mark the five replaced lines in place, and add risk R12**

Append ` → §5.10 for Fleet` to the last cell of the `scp` row (`:369`), the `azp` row (`:370`) and
the `Principal` row (`:372`); append ` A delegated write records its actor per §5.10 rule 4.` to
the sentence ending at `:435`; append ` Delegated requests take \`org_id\` from the delegation
(§5.10 rule 6).` to the last cell of the G4 row (`:253`). No other word of those lines changes.

Then add one row to §8's table, after R11:

```markdown
| R12 | **Fleet is a trusted deputy.** Within a live delegation Fleet writes in a member's name; a compromised Fleet can write, for up to a day, any record the member's own agents could, on the routes the delegation lists (`05-design.md` §2.4 item 5) | low · a day of false records in one member's name, per compromised delegation | workload identity only, and no user token ever held by Fleet; 24-hour delegations naming routes; the Hub's `ag-…` owner check in the write's transaction (§5.10 rule 5); every delegated write in the audit chain as `v: 2` with `via` and `delegationId`; removing a member revokes their delegations (Phase T task 44). **What remains:** within those bounds the Hub cannot tell a true Fleet write from a forged one. Anchoring the chain outside the database (Phase T task 45) makes the forgery visible afterwards, not impossible |
```

- [ ] **Step 4: Contract — ids, the audit event v2, codes, the principal step, variables**

**§3.1**, append six rows to the prefix table, in its own two-pairs-per-row shape (the Hub's
`TestPrefixesAndReasonsAreTheContracts`, `hubapi/keys_test.go:737-775`, reads exactly five cells a
row and takes the entity cell as the entity's name, so no annotation goes in it):

```markdown
| `dl-` | delegation | | `th-` | thread |
| `hm-` | human message | | `ms-` | message |
| `nt-` | note | | `es-` | escalation |
| `tk-` | task | | `dv-` | directive |
| `ag-` | agent | | `ot-` | orchestrator turn |
| `ls-` | lease | | `cf-` | conflict |
```

and two sentences under it: *"The Hub mints `dl-`, `hm-` and `nt-`. Fleet mints the other nine
with `ids.Wide` and sends each as the body's `id`, because it writes through an outbox that must
name a record before the Hub has answered (Phase T task 11); such an id is validated by
`hubapi.ValidID` and must be new in its org: a reuse with a different body is 409 `conflict`, a
retry with the same `Idempotency-Key` replays (§1)."* All twelve are disjoint from the ten above
and from the control plane's five.

**§3.6**, replace the audit event row's field list with: `v` (`1`, or `2` for a delegated write),
`kind`, `orgId`, `seq`, `at`, `actorUserId` (or `""`), `event`, `teamId` (or `""`; the key
`hubapi.AuditEnvelope` already writes, `hubapi/envelope.go:264-276`, plan correction A10),
`projectId` (or `""`), `subject`, `data`, and **only when `v` is 2**, `via` (`"fleet"`) and `delegationId`. Add: *"A v1 event's bytes
are unchanged by this amendment; the §3.5 golden vectors stand. A v2 golden vector is added by
Phase T task 03."*

**§4.2**, append fifteen rows at the **end** of the codes table, after `unavailable`, in exactly
this order. The Hub's `TestHubCodesAreContractSection42s` (`hubapi/keys_test.go:530-625`) requires
`codes.go` to declare its own codes in the table's order and each `details` cell to hold the keys
in backticks, so task 03 appends the constants in this order and names these keys:

```markdown
| `delegation_required` | 401 | a Fleet token, on a route other than the per-org feed, with no `X-Panacea-Delegation` | — |
| `delegation_expired` | 401 | the named delegation's `expiresAt` has passed | `delegationId` |
| `delegation_revoked` | 401 | the named delegation has `revokedAt` set, or is unknown to this org | `delegationId`, `reason` |
| `agent_not_owner` | 403 | an `ag-…` in the write does not belong to the delegation's owner (§5.10 rule 5) | `agentId` |
| `delegation_not_found` | 404 | the owner's own delegation route names none of theirs | — |
| `task_not_found` | 404 | no such task in this project | — |
| `agent_not_found` | 404 | no such agent in this project | — |
| `lease_not_found` | 404 | no lease events for this lease in this project | — |
| `conflict_not_found` | 404 | no such conflict in this project | — |
| `thread_not_found` | 404 | no such thread in this project, or the caller is not a party | — |
| `escalation_not_found` | 404 | no such escalation of the caller's | — |
| `team_state` | 409 | a team record's state does not allow this move | `from`, `to` |
| `text_refused` | 422 | a free-text field fails a bound or the hygiene check (Phase T task 02) | `field`, `rule` |
| `glob_refused` | 422 | a path or glob is empty, absolute, has `..` or `\`, a bad segment, is whole-repo, or exceeds 200 characters | `field`, `index`, `rule` |
| `breadth_exceeded` | 422 | a glob set exceeds 20 globs, or one glob matches more than 10% of tracked files | `field`, `index` |
```

The not-found and state rows are the ones tasks 06 and 07 need (their parts' "Gaps"); listing them
here keeps §4.2 and `codes.go` one list. A transition refused by a team record's state is 409
`team_state`, not the generic `conflict`, which stays for id reuse. A named delegation that is
absent, or in another org, is **401 `delegation_revoked` with `details.reason = "unknown"`**, not
404, so a Fleet caller cannot probe for another org's delegation ids; `delegation_not_found` is
served only on the owner's own `GET`/`renew`/`DELETE` routes (§7.13).

**§5.1**, insert between steps 3 and 4: *"3a. If the token is Fleet's (§5.10 rule 1), apply §5.10
rules 2, 3 and 6: resolve the named delegation, refuse it or take its owner as the principal and its
org as `hub.org_id`. Step 5 then resolves the **owner**, never Fleet."*

**§12.2**, append two rows:

```markdown
| `PANACEA_APP_HUB_FLEET_CLIENT_ID` | `""` (Fleet disabled) | Fleet's workload-identity client id. Empty refuses every app-only token, as before this amendment |
| `PANACEA_APP_HUB_DELEGATION_TTL` | `24h` | a delegation's life from creation or renewal; a Go duration, at most `24h` |
```

- [ ] **Step 5: Contract §7.0 — rows 46 to 78, and the delegable route names**

Add a second table to §7.0, directly under the first, headed **Team routes (Phase T)**; it has one
more column than the first, so it is a table of its own, not appended rows. Its numbering
continues from row 45. **Caller** `Fleet·O` is Fleet under an `orchestrator`-class delegation,
`Fleet·W` under a `worker`-class one, `Fleet` under either; `desk` is the member's own desktop
token. The **route name** is what a delegation lists; a route with no name cannot be delegated.

```markdown
| # | Method | Path | Caller and role | Route name | Task | Section |
|---|---|---|---|---|---|---|
| 46 | POST | `/v1/delegations` | desk, M of the body's project | — | T06 | §7.13 |
| 47 | GET | `/v1/delegations/{delegationId}` | its owner (desk), or Fleet under it | `delegations.read` | T06 | §7.13 |
| 48 | POST | `/v1/delegations/{delegationId}/renew` | its owner (desk) | — | T06 | §7.13 |
| 49 | DELETE | `/v1/delegations/{delegationId}` | its owner, or A of its project (desk) | — | T06 | §7.13 |
| 50 | POST | `/v1/projects/{projectId}/tasks` | Fleet·O | `tasks.create` | T06 | §7.14 |
| 51 | PATCH | `/v1/projects/{projectId}/tasks/{taskId}` | Fleet·O, the task's owner | `tasks.update` | T06 | §7.14 |
| 52 | GET | `/v1/projects/{projectId}/tasks` | V (desk), or Fleet | `tasks.read` | T06 | §7.14 |
| 53 | POST | `/v1/projects/{projectId}/agents` | Fleet | `agents.start` | T06 | §7.15 |
| 54 | POST | `/v1/projects/{projectId}/agents/{agentId}/end` | Fleet, the agent's owner | `agents.end` | T06 | §7.15 |
| 55 | GET | `/v1/projects/{projectId}/agents` | V (desk), or Fleet | `agents.read` | T06 | §7.15 |
| 56 | POST | `/v1/projects/{projectId}/leases/{leaseId}/events` | Fleet, the holder's owner | `leases.events` | T06 | §7.16 |
| 57 | POST | `/v1/projects/{projectId}/conflicts` | Fleet, owner of `agentA` | `conflicts.create` | T06 | §7.17 |
| 58 | PATCH | `/v1/projects/{projectId}/conflicts/{conflictId}` | Fleet, owner of either agent | `conflicts.update` | T06 | §7.17 |
| 59 | GET | `/v1/projects/{projectId}/conflicts` | V (desk), or Fleet | `conflicts.read` | T06 | §7.17 |
| 60 | POST | `/v1/projects/{projectId}/threads` | Fleet·O, owner of the opener | `threads.create` | T07 | §7.18 |
| 61 | PATCH | `/v1/projects/{projectId}/threads/{threadId}` | Fleet·O, owner of a participant | `threads.update` | T07 | §7.18 |
| 62 | POST | `/v1/projects/{projectId}/threads/{threadId}/messages` | Fleet·O, owner of the sender | `messages.append` | T07 | §7.18 |
| 63 | GET | `/v1/projects/{projectId}/threads/{threadId}/messages` | Fleet·O for a participant's owner; org *auditor* (desk) | `messages.read` | T07 | §7.18 |
| 64 | POST | `/v1/projects/{projectId}/escalations` | Fleet·O, the escalation's owner | `escalations.create` | T07 | §7.19 |
| 65 | POST | `/v1/escalations/{escalationId}/answer` | its owner (desk) only | — | T07 | §7.19 |
| 66 | GET | `/v1/projects/{projectId}/escalations` | own only (desk, or Fleet·O); org *auditor* | `escalations.read` | T07 | §7.19 |
| 67 | POST | `/v1/projects/{projectId}/directives` | Fleet·O, owner of both ends | `directives.append` | T07 | §7.20 |
| 68 | GET | `/v1/projects/{projectId}/directives` | own only (desk, or Fleet·O); org *auditor* | `directives.read` | T07 | §7.20 |
| 69 | POST | `/v1/projects/{projectId}/turns` | Fleet·O, the Orchestrator's owner | `turns.record` | T07 | §7.21 |
| 70 | POST | `/v1/projects/{projectId}/human-messages` | desk, M, to their own Orchestrator | — | T07 | §7.22 |
| 71 | GET | `/v1/projects/{projectId}/human-messages` | own only (desk, or Fleet·O); org *auditor* | `human_messages.read` | T07 | §7.22 |
| 72 | POST | `/v1/projects/{projectId}/notes` | desk, M, as themselves | — | T46 | §7.23 |
| 73 | GET | `/v1/projects/{projectId}/notes` | V (desk), or Fleet·O | `notes.read` | T46 | §7.23 |
| 74 | GET | `/v1/projects/{projectId}/team-policy` | V (desk), or Fleet | `policy.read` | T43 | §7.24 |
| 75 | PUT | `/v1/projects/{projectId}/team-policy` | desk, A within floors; the route lowered only by org *compliance* | — | T43 | §7.24 |
| 76 | POST | `/v1/orgs/{orgId}/usage-rollups` | the gateway's identity — **caller rule open**, see §7.25 | — | T66 | §7.25 |
| 77 | GET | `/v1/projects/{projectId}/usage` | A (every member), M (own) | — | T66 | §7.25 |
| 78 | GET | `/v1/orgs/{orgId}/changes?after=&kinds=` | Fleet, **no delegation** (§5.10 rule 7) | — | T08 | §7.26 |
| 79 | POST | `/v1/projects/{projectId}/team-policy/route` | desk: M raises to `au`; only org *compliance* lowers to `global` | — | T43 | §7.24 |
```

`T06` means Phase T task 06. §7.0's closing sentence becomes *"Nothing else is routed than these two
tables."*

**The class route sets.** A delegation's `routes` must be a subset of its class's set, or the create
is 422 `bad_request` with `details.route`:

- `orchestrator`: every route name above.
- `worker` (a worker the member launched with no Orchestrator): `delegations.read`,
  `tasks.read`, `agents.start`, `agents.end`, `agents.read`, `leases.events`, `conflicts.create`,
  `conflicts.update`, `conflicts.read`, `policy.read`.

A worker-class delegation can therefore never carry a message, a directive or an escalation: those
are the cross-user channel, and OD-16 gives it to the Orchestrator alone.

- [ ] **Step 6: Contract §7.13 to §7.17 — delegation, task, agent, lease event, conflict**

Conventions for every team section. **Requests** carry free text as plain strings; the Hub bounds
and cleans them (task 02), stamps the author class from the route (a `Fleet` route writes `agent`,
a `desk` route writes `human`) and returns them as a `Text` object:
`{"body", "author": "agent"|"human", "authorAgentId" (agent text only), "ownerUserId", "at"}`.
**Globs** are repo-relative, validated by `teamtext.ValidGlob`, ≤ 20 per set, each matching ≤ 10%
of the project's tracked files when Fleet supplies the count (`teamtext.Breadth`; the Hub holds no
file list, so it checks the count Fleet sends in `trackedFiles` and the glob grammar; Fleet checks
the match). **No request body carries an owner, an actor, a `via`, a delegation or a session id**;
strict decoding refuses them (§5.10 rule 4, `hub-contract.md:941-942`). Every write is one
metadata-only audit event and one feed row, in the §10.6 transaction. **Status for a refused
move:** wherever a section below says 409 `conflict` for a *state* (a terminal task or conflict, a
second agent end, renewing a revoked delegation, a second answer, an event after a terminal lease
state), the code is 409 **`team_state`** with `details.from` and `details.to`; 409 `conflict` stays
only for an id reused with a different body and a lease `seq` out of order.

**§7.13 Delegations.** `POST /v1/delegations`, body `{"projectId","class","routes"}`, 201:

```json
{ "id": "dl-3f2a91c07b1e4d2a", "projectId": "pj-…", "ownerUserId": "u-…",
  "class": "orchestrator", "routes": ["tasks.create", "agents.start"],
  "createdAt": "2026-09-25T01:00:00Z", "expiresAt": "2026-09-26T01:00:00Z",
  "renewedAt": null, "revokedAt": null, "revokedReason": null }
```

`class` is `orchestrator`·`worker`. `routes` is 1 to 25 distinct names from the class's set.
`renew` sets `expiresAt = now + PANACEA_APP_HUB_DELEGATION_TTL` and `renewedAt = now`; a lapsed but
unrevoked delegation may be renewed (the event carries `"lapsed": true`), a revoked one may not (409
`conflict`). `DELETE` sets `revokedAt` and `revokedReason` = `owner` or `admin`; removing a member
revokes theirs with `member_removed` (task 44), and a failed account check with `account_disabled`.

**§7.14 Tasks.** `POST …/tasks`, body:

```json
{ "id": "tk-…", "orchestratorAgentId": "ag-…", "title": "Add the export route",
  "titleSource": "hm-…", "reason": { "briefRef": "brief:bf-…@3#export", "why": "…" },
  "scope": ["src/export/**"], "trackedFiles": 812, "state": "planned" }
```

`title` ≤ 140; `why` ≤ 200; `titleSource` optional: the owner's own `hm-…` whose trimmed body
**equals** the title, which makes the title `human`, else it is `agent`. `briefRef` is a brief key
(§3.2) plus `#` and a section id `^[a-z0-9][a-z0-9-]{0,63}$` (**UNVERIFIED** against phase 4's
section ids until P4's merge). `state` is `planned`·`active`·`blocked`·`done`·`abandoned`, default
`planned`; `done` and `abandoned` are terminal (409 `conflict`). `PATCH` body `{"revision", "state"?,
"scope"?, "trackedFiles"?, "why"?}`, 409 `revision_conflict` on a stale revision. The record adds
`projectId`, `ownerUserId`, `revision`, `createdAt`, `updatedAt`. **`title` and `reason.why` are
returned only to Fleet·O**; every other reader gets the record without them (`05-design.md` §5.1).

**§7.15 Agents.** `POST …/agents` (start), body:

```json
{ "id": "ag-…", "role": "worker", "parentAgentId": "ag-…", "taskId": "tk-…",
  "harness": { "id": "claude-code", "version": "2.1.4" },
  "laneRef": "refs/panacea/lanes/ag-…/main", "policy": { "version": 3, "digest": "sha256:…" } }
```

`role` is `orchestrator`·`worker`; an Orchestrator has no parent; a worker's parent, when given,
is an Orchestrator of the **same owner**. `harness.id` `^[a-z][a-z0-9-]{0,31}$`, `version`
`^[0-9A-Za-z.+-]{1,32}$`. `laneRef` starts `refs/panacea/lanes/` (OD-2) and passes
`hubapi.ValidBranchRef`'s component rules. `policy` is the OD-23 version and digest the sandbox was
started with (Phase T task 13). `POST …/agents/{agentId}/end`, body `{"exitClass"}`, one of
`done`·`stopped`·`failed`·`access_removed`·`lapsed`; a second end is 409 `conflict`. The record
is the start body plus `projectId`, `ownerUserId`, `startedAt`, `endedAt` and `exitClass`, the
last two `null` while the agent runs. Status
(`working`, `waiting`, …) is Fleet state and never reaches the Hub. Solo projects keep §7.8's
`session_link`; a team project's route 33 is 409 `conflict` with `details.use = "agents"`.

**§7.16 Lease events.** `POST …/leases/{leaseId}/events`, body
`{"seq", "holderAgentId", "mode", "globs", "trackedFiles", "state", "expiresAt", "at"}`. `mode`
`intent`·`hold`·`bulk`; `state` `requested`·`granted`·`contested`·`yielded`·`released`·`expired`·
`admin-released`. The 201 is `{"leaseId", "seq", "state", "at"}`. Fleet stays authoritative: the Hub checks shape, owner and that `seq` is the
lease's last `seq` + 1 (409 `conflict` otherwise), and that nothing follows a terminal state
(`yielded`, `released`, `expired`, `admin-released`). **One exception to "the holder's owner"**:
an `admin-released` event is written under the delegation of an A of the project, and records it.

**§7.17 Conflicts.** `POST …/conflicts`, body:

```json
{ "id": "cf-…", "agentA": "ag-…", "agentB": "ag-…", "tier": "H",
  "paths": [ { "path": "src/export/route.ts", "lines": [[10, 24]] } ],
  "basis": { "a": "<40 or 64 hex>", "b": "<40 or 64 hex>" } }
```

`tier` `P`·`H`·`M`·`S`; ≤ 50 paths, ≤ 20 line ranges each, `1 ≤ start ≤ end`; the two agents
differ. The record adds `state` (`raised` at create; then `negotiating`·`accepted`·`resolved`·
`escalated`·`cleared`), `resolution` (`split`·`sequence`·`yield`·`build-on`·`proceed-both`, or
`null`), `threadId` (or `null`), `projectId`, `revision`, `createdAt`, `updatedAt`. `PATCH` body `{"revision", "state",
"resolution"?, "threadId"?}`; `resolution` is required with `resolved` and `accepted`.

- [ ] **Step 7: Contract §7.18 to §7.22 — thread, message, escalation, directive, turn, human message**

**§7.18 Threads and messages.** `POST …/threads`, body `{"id", "openerAgentId", "participants",
"subject": {"kind": "conflict"|"lease"|"free", "id": "cf-…"|"ls-…"|""}, "deadline"}`:
`participants` is exactly the 2 distinct Orchestrator `ag-…` of **different** owners, the opener
among them; `deadline` is Fleet-set, 30 minutes to 24 hours after now. The record adds `state`
(`open`, then `resolved`·`escalated`·`expired`), `rounds` (0), `revision`, `createdAt`. `PATCH`
body `{"revision", "state"?, "rounds"?}`; `rounds` only grows. `POST …/threads/{threadId}/messages`
is **append-only**, body `{"id", "senderAgentId", "kind", "fields", "text"?, "templated"}`:

| `kind` | `fields` (typed) | `text` |
|---|---|---|
| `ask` | `{"subject": {"kind": "task"\|"lease"\|"path"\|"conflict", "ref"}}` | question ≤ 600, required |
| `answer` | `{"inReplyTo": "ms-…"}` | ≤ 600; only with `"templated": false`, which Fleet sets only under the owner's standing permission (§6.5) |
| `inform` | `{"subject": …}` as `ask` | ≤ 600, required |
| `propose` · `counter` | `{"resolution": Resolution}` | rationale ≤ 600, optional |
| `accept` · `reject` | `{"proposal": "ms-…"}` | reason ≤ 300, optional |
| `withdraw` | `{"proposal": "ms-…"}`, the sender's own | none |

`Resolution` is one of `{"kind": "split", "sides": {"<ag>": [globs], "<ag>": [globs]}}`,
`{"kind": "sequence", "first": "ag-…"}`, `{"kind": "yield", "globs": [globs]}`,
`{"kind": "build-on", "base": "ag-…"}`, `{"kind": "proceed-both", "paths": [globs]}`. A message
whose `senderAgentId` is not a participant is 403 `forbidden`; not the owner's, 403
`agent_not_owner`. `GET` returns messages to Fleet·O **for a participant's owner** and to an org
*auditor*, to nobody else; text is always a `Text` with `author = "agent"`. A message record is
the body plus `threadId`, `projectId`, `senderUserId` (the sender's owner), `createdAt` and
`tombstonedAt` (erasure, task 46); a thread record adds `projectId` too.

**§7.19 Escalations.** `POST …/escalations`, body `{"id", "fromAgentId" ("" when Fleet raises it
for itself), "kind": "inform"|"decide", "threadId"?, "question", "options", "default",
"deadline"}`. `question` ≤ 300. `options`: 0 for `inform`, 2 to 4 for `decide`, each
`{"label" ≤ 80, "consequence" ≤ 200}`. `default` is an option index, required for `decide`, and is
**Fleet's** choice, always the safe one (§6.5); `deadline` ≥ 30 minutes after now. The owner is the
delegation's owner; `fromAgentId`, when set, is the owner's Orchestrator. The record adds
`ownerUserId`, `answer` (`null` or `{"choice", "text": Text·human, "answeredAt"}`), `createdAt`.
`POST /v1/escalations/{escalationId}/answer`, body `{"choice", "text"?}`, `text` ≤ 300, **by the
owner's own desktop token only**: anyone else, Fleet included, is 403 `forbidden`; a second answer
is 409 `conflict`. Past its deadline with no answer, an escalation *reads* as defaulted; Fleet
acts on the default and cites the `es-…` in the directive it issues. Readable by the owner and an
org *auditor* only: another member's `GET` is filtered to their own, so a teammate's `es-…` never
appears, and their `answer` on it is 403 `forbidden`.

**§7.20 Directives.** `POST …/directives`, **append-only**, body `{"id", "fromAgentId",
"toAgentId", "kind", "params", "text"?, "rung", "origin", "read", "approvedBy"?}`. `kind`
`pause`·`resume`·`narrow`·`wait-for`·`rebase`·`free`; `params` is `{"globs": [globs]}` for
`narrow`, `{"leaseId": "ls-…"}` for `wait-for`, `{}` otherwise; `text` ≤ 600, required for `free`
and refused otherwise;
`rung` `piggyback`·`inbox-socket`·`pty-nudge`; `origin` `human`·`fleet`; `read` ≤ 100 ids
(`ms-`, `tk-`, `nt-`, `es-`) the Orchestrator session read since its human last spoke;
`approvedBy` an `es-…`. `from` is the owner's Orchestrator, `to` the owner's worker. **The Hub
re-checks the kind half of the taint rule** (`05-design.md` §7.2 item 2) as a second layer to
Fleet's verb handler: when `origin` is `fleet` and `kind` is `free`, `approvedBy` must name a
`decide` escalation of the same owner **answered by that owner**, else 403 `forbidden` with
`details.required = "decide"`. The typed five (`pause`, `resume`, `narrow`, `wait-for`, `rebase`)
pass; whether a `narrow` stays inside the task's scope is Fleet's check alone, since the Hub does
not evaluate glob containment. Readable by the owner and an org *auditor* only.

**§7.21 Orchestrator turns.** `POST …/turns`, body `{"id", "orchestratorAgentId", "origin",
"consumed" (≤ 200 event ids), "issued" (≤ 100 `dv-`/`ms-`/`es-` ids), "tokens": {"input",
"output"}}`, 201 with `{"id"}`. It writes **one audit event and no table row**
(`05-design.md` §3.3: "audit event only").

**§7.22 Human messages.** `POST …/human-messages`, body `{"toAgentId", "text"}`, `text` ≤ 2,000,
**append-only**, by the owner's desktop; `toAgentId` must be the owner's own Orchestrator (403
`agent_not_owner` otherwise). The Hub mints `hm-…`; the record is `{"id", "projectId",
"ownerUserId", "toAgentId", "text": Text·human, "createdAt", "tombstonedAt": null}`. Readable by the
owner (desk, or Fleet·O under the owner's delegation) and an org *auditor* only. Erasure tombstones
it (task 46): `text` becomes `null`, the audit event stays.

- [ ] **Step 8: Contract §7.23 to §7.26 — note, team policy, usage roll-up, Fleet's feed**

**§7.23 Notes.** `POST …/notes`, body `{"target": {"kind": "project"|"task"|"user", "id"}, "text"}`,
`text` ≤ 1,000, by any M as themselves; the Hub mints `nt-…`; the author is the token's user. The
record is `{"id", "projectId", "authorUserId", "target", "text": Text·human, "createdAt",
"tombstonedAt": null}`, **immutable**. `GET` serves V and Fleet·O; Fleet passes a note to an
Orchestrator only through `panacea inbox`, quoted with its author's name, and to no worker
(`07` M2, task 46).

**§7.24 Team policy.** One record per team project:

```json
{ "projectId": "pj-…", "dataClass": "…", "route": "global",
  "quotas": { "memberDailyCents": 2500, "projectDailyCents": 20000 },
  "orchestratorTurnsPerHour": 30,
  "escalationFloors": ["yield-unedited", "accept-split", "proceed-both-ps", "sequence-short"],
  "messageBudgets": { "perPairPerHour": 20, "roundsPerSide": 3, "threadDeadlineMinutes": 30 },
  "revision": 4, "updatedAt": "…" }
```

`PUT` (an A, body without `projectId`, `dataClass`, `route`, `updatedAt`) enforces, each as 422
`bad_request` with `details.field`: every quota > 0 (OD-24, budgets on by default);
`orchestratorTurnsPerHour` 1 to 30; `perPairPerHour` 1 to 20, `roundsPerSide` 1 to 3 (tighten only,
§6.6); `threadDeadlineMinutes` 30 to 1,440 (lengthen only, §6.4); `escalationFloors` a subset of
the four §6.5 rows whose default is *act*, since a floor only adds asking. `dataClass` is the
project's own, set at creation (OD-24), read-only here; its values and its home on the project
record are **UNVERIFIED until merge** (task 43 reuses whatever the merged project record carries).
Route 79 moves `route`: any M may raise it to `au`; only a token whose `roles` contain
`Hub.Compliance` may lower it to `global`, and that write's audit event says so.

**§7.25 Usage roll-ups.** `POST /v1/orgs/{orgId}/usage-rollups`, body `{"day": "2026-09-25",
"rows": [ {"userId", "teamId", "projectId", "agentId", "route", "counterpartUserId" (or ""),
"inputTokens", "outputTokens", "costCents"} ]}`, ≤ 5,000 rows, one post per `(org, day)`,
replacing any earlier post for that day. **Its caller is open:** it is the gateway's own
workload identity, a third caller that §5.10 does not admit. This contract fixes the record only;
the caller rule is Phase T task 66's to write, as a §5.10 rule 9, before any code calls the route.
`GET …/usage?from=&to=` serves an A every row of the project, an M their own.

**§7.26 Fleet's feed.** `GET /v1/orgs/{orgId}/changes?after=&kinds=`, Fleet only, no delegation.
The same response and cursor as §7.11, over **team projects only** (a project whose mode is
`team`, set by Share, `05-design.md` §2.3, §10.1), plus `team.member_removed` and every
`delegation.*` event of the org. `kinds` is an optional comma list of §8 kinds. `orgId` must be
the org of Fleet's token `tid`, else 404 `route_not_found`. Metadata only, as every feed row.

- [ ] **Step 9: Contract §8 — the team events**

Append to §8's table. Every kind matches the audit table's `CHECK (kind ~
'^[a-z]+(_[a-z]+)*\.[a-z]+(_[a-z]+)*$')` (`hub-contract.md:1766`). **In the feed** is now three
values: `yes` (the member feed of §7.11 **and** Fleet's feed), `fleet` (Fleet's feed only; the
audit row's new `fleet_feed` flag, task 04), `no` (audit only). `data` never holds text.

```markdown
| `delegation.created` | ✓ | ✓ | delegation id | `{"class","expiresAt"}` | fleet |
| `delegation.renewed` | ✓ | ✓ | delegation id | `{"expiresAt","lapsed"}` | fleet |
| `delegation.revoked` | ✓ | ✓ | delegation id | `{"reason"}` | fleet |
| `task.created` | ✓ | ✓ | task id | `{"state","orchestratorAgentId"}` | yes |
| `task.updated` | ✓ | ✓ | task id | `{"state","revision"}` | yes |
| `agent.started` | ✓ | ✓ | agent id | `{"role","parentAgentId","harnessId","harnessVersion","policyVersion"}` | yes |
| `agent.ended` | ✓ | ✓ | agent id | `{"exitClass"}` | yes |
| `lease.state_changed` | ✓ | ✓ | lease id | `{"seq","state","mode","holderAgentId"}` | yes |
| `conflict.raised` | ✓ | ✓ | conflict id | `{"tier","agentA","agentB"}` | yes |
| `conflict.updated` | ✓ | ✓ | conflict id | `{"state","resolution"}` | yes |
| `thread.opened` | ✓ | ✓ | thread id | `{"subjectKind","participantA","participantB"}` | fleet |
| `thread.updated` | ✓ | ✓ | thread id | `{"state","rounds"}` | fleet |
| `message.appended` | ✓ | ✓ | message id | `{"threadId","kind","senderAgentId"}` | fleet |
| `escalation.created` | ✓ | ✓ | escalation id | `{"kind","deadline","fromAgentId"}` | fleet |
| `escalation.answered` | ✓ | ✓ | escalation id | `{"choice"}` | fleet |
| `directive.appended` | ✓ | ✓ | directive id | `{"kind","rung","origin","fromAgentId","toAgentId","approvedBy"}` | fleet |
| `orchestrator_turn.recorded` | ✓ | ✓ | turn id | `{"orchestratorAgentId","origin","consumed","issued","inputTokens","outputTokens"}` | no |
| `human_message.appended` | ✓ | ✓ | human message id | `{"toAgentId"}` | fleet |
| `note.created` | ✓ | ✓ | note id | `{"targetKind","targetId"}` | yes |
| `team_policy.updated` | ✓ | ✓ | project id | `{"revision"}` | yes |
| `team_policy.route_changed` | ✓ | ✓ | project id | `{"route","by"}`; `by` is `member` or `compliance` | yes |
| `usage.rolled_up` | — | — | org id | `{"day","rows"}` | no |
```

**`data` values are strings, integers and booleans only** (`hubapi.AuditEnvelope.check`,
`hubapi/envelope.go:287-311`), so no event carries a list: a thread's two participants are two
keys, and a turn's `consumed` and `issued` are its ids joined by `,` in the order Fleet consumed or
issued them (at most 200 and 100 ids, so at most about 4 KB).

Escalations, directives and human messages are `fleet`, never `yes`, because §3.4 makes them
readable by their owner and an auditor only, and the member feed is read by every member. Every
delegated write's event is the audit-event `v: 2` of §3.6.

- [ ] **Step 10: Contract §5.2 and §5.3 — the two org capabilities and the team rows**

**§5.2**, append: *"**Org auditor** (`Hub.Auditor`) reads escalations, directives, human messages
and thread messages across owners, in every project of the org, and nothing else of project
content. **Org compliance** (`Hub.Compliance`) may lower a team project's route to `global` (route
79), audited. Both are Entra app roles read from each token, like `Hub.OrgAdmin`, and grant no
write of any team record."*

**§5.3**, add a second table, **Team rows**. Columns: `V`, `M` (a member, on their **own** record),
`M′` (a member, on **another** member's record), `A` (an admin, on another's), `Au` (org auditor,
not a member), `N`, `X`, as §5.3; `F` Fleet under the record owner's live delegation listing the
route; `F∅` Fleet with no delegation; `Fx` Fleet under **another** owner's delegation; `D` a desktop
token carrying `X-Panacea-Delegation`. **401d** is `delegation_required`; **403a** is
`agent_not_owner`; **—** means the caller cannot reach the route at all (a Fleet-only or desk-only
route).

```markdown
| Operation | V | M | M′ | A | Au | N | X | F | F∅ | Fx | D |
|---|---|---|---|---|---|---|---|---|---|---|---|
| create a delegation (46) | 403 | ✓ | — | ✓ own | 404 | 404 | 404 | — | 401d | — | 403 |
| revoke a delegation (49) | 403 | ✓ | 403 | ✓ | 404 | 404 | 404 | — | 401d | — | 403 |
| create or update a task (50, 51) | — | — | — | — | — | — | — | ✓ | 401d | 403a | 403 |
| start or end an agent (53, 54) | — | — | — | — | — | — | — | ✓ | 401d | 403a | 403 |
| append a lease event (56) | — | — | — | — | — | — | — | ✓ | 401d | 403a | 403 |
| raise or update a conflict (57, 58) | — | — | — | — | — | — | — | ✓ | 401d | 403a | 403 |
| open or update a thread; append a message (60–62) | — | — | — | — | — | — | — | ✓ | 401d | 403a | 403 |
| read a thread's messages (63) | — | — | — | — | ✓ | 404 | 404 | ✓ party | 401d | 403 | 403 |
| create an escalation (64) | — | — | — | — | — | — | — | ✓ | 401d | 403a | 403 |
| answer an escalation (65) | 403 | ✓ | 403 | 403 | 403 | 404 | 404 | 403 | 403 | 403 | 403 |
| read escalations, directives, human messages (66, 68, 71) | ∅ | ✓ own | ∅ | ∅ | ✓ | 404 | 404 | ✓ own | 401d | ∅ | 403 |
| append a directive; record a turn (67, 69) | — | — | — | — | — | — | — | ✓ | 401d | 403a | 403 |
| write a human message (70) | 403 | ✓ | — | ✓ own | 404 | 404 | 404 | 403 | 401d | 403 | 403 |
| write a note (72) | 403 | ✓ | ✓ | ✓ | 404 | 404 | 404 | 403 | 401d | 403 | 403 |
| set team policy (75) | 403 | 403 | 403 | ✓ | 404 | 404 | 404 | 403 | 401d | 403 | 403 |
| read tasks, agents, conflicts, notes, policy (52, 55, 59, 73, 74) | ✓ | ✓ | ✓ | ✓ | 404 | 404 | 404 | ✓ | 401d | ✓ | 403 |
```

`∅` means 200 with an empty list: the route answers, the other person's rows are absent. A `—` in
a desk column is 403 `forbidden` with `details.required = "fleet"` when a desktop calls a Fleet-only
route. **Nobody writes another person's task, lease, message, directive or escalation, whatever
their role**: no `✓` stands in an `M′` or `A` cell of a write row, and `Fx` is refused on every
owner-bound write.

- [ ] **Step 11: The field check — written before the check is run, so it is the test**

For a documents task the test is a list the documents must satisfy. Write
`docs/superpowers/contracts/team-records-fieldcheck.txt`, one line per `05-design.md` §3.3 field
of a Hub-owned record, `record|field as the design names it|the token the contract must contain`:

```text
delegation|owner|"ownerUserId"
delegation|project|"projectId"
delegation|class|"class": "orchestrator"
delegation|allowed routes|"routes"
delegation|expiresAt (24 h)|"expiresAt"
delegation|revokedAt|"revokedAt"
task|orchestrator ag|"orchestratorAgentId"
task|title <= 140, text·H or text·A|"titleSource"
task|reason briefRef|"briefRef"
task|why-line <= 200|"why"
task|intended scope <= 20 globs|"scope"
task|state planned..abandoned|`abandoned`
agent|role orchestrator·worker|"role"
agent|parent orchestrator|"parentAgentId"
agent|harness id and pinned version|"harness"
agent|task|"taskId"
agent|lane ref|"laneRef"
agent|exitClass|"exitClass"
lease|holder ag|"holderAgentId"
lease|globs|"globs"
lease|mode intent·hold·bulk|`bulk`
lease|expiresAt|"expiresAt"
lease|state incl. admin-released|`admin-released`
conflict|agent pair|"agentB"
conflict|tier P·H·M·S|"tier"
conflict|paths and line ranges|"lines"
conflict|basis shas|"basis"
conflict|state raised..cleared|`cleared`
conflict|resolution kind|`proceed-both`
thread|participants|"participants"
thread|subject conflict·lease·free|"subject"
thread|state open..expired|`expired`
thread|round count|"rounds"
thread|deadline (Fleet-set, 30 min)|"deadline"
message|thread|"threadId"
message|sender ag and its owner|"senderAgentId"
message|kind|`withdraw`
message|typed fields per kind|"fields"
message|free-text answer only under standing permission|"templated"
escalation|owner (a human)|"ownerUserId"
escalation|from orchestrator or Fleet|"fromAgentId"
escalation|kind inform·decide|"kind": "inform"
escalation|question <= 300|"question"
escalation|up to 4 options with consequence|"consequence"
escalation|default set by Fleet|"default"
escalation|answer text·H|"answer"
escalation|answeredAt|"answeredAt"
directive|from Orchestrator ag|"fromAgentId"
directive|to worker ag|"toAgentId"
directive|typed kind pause..free|`wait-for`
directive|delivery rung|"rung"
directive|turn origin human·fleet|"origin"
directive|peer messages and cards read|"read"
directive|approving es|"approvedBy"
orchestrator turn|event ids consumed|"consumed"
orchestrator turn|directives and messages issued|"issued"
orchestrator turn|tokens used|"tokens"
human message|to own Orchestrator|"toAgentId"
human message|tombstoned on erasure|"tombstonedAt"
note|author from token|"authorUserId"
note|target project·task·person|"target"
team policy|data classification|"dataClass"
team policy|route|"route"
team policy|non-zero default quotas|"quotas"
team policy|Orchestrator turn budget|"orchestratorTurnsPerHour"
team policy|escalation floors|"escalationFloors"
team policy|message budgets|"messageBudgets"
usage|counterpart for peer-triggered turns|"counterpartUserId"
usage|tokens and cost|"costCents"
```

The mutant this list kills: a record section that drops a design field (for example, a directive
with no `"read"`, which loses §7.2's taint evidence) fails the check by name.

Then write the check itself to `.superpowers/sdd/team/t01-check.sh` (git-ignored; a script, not a
heredoc, per the shell rules):

```bash
#!/usr/bin/env bash
# Phase T task 01 acceptance. Run from the repo root.
set -u
C=docs/superpowers/contracts/2026-09-23-hub-contract.md
A=docs/superpowers/adr/ADR-H1-hub-architecture.md
F=docs/superpowers/contracts/team-records-fieldcheck.txt
S=$(awk '/^### 7\.13 /,/^## 8\. /' "$C")
E=$(awk '/^## 8\. /,/^## 9\. /' "$C")
fail=0
while IFS='|' read -r rec field tok; do
  [ -z "$rec" ] && continue
  printf '%s\n' "$S" | grep -qF -- "$tok" || { echo "MISSING $rec: $field ($tok)"; fail=1; }
done < "$F"
echo "fields: $(grep -c . "$F")"
if printf '%s\n' "$S" | grep -nE '"(actor|actorUserId|via|delegationId|sessionId)"'; then
  echo "A TEAM SECTION NAMES AN ACTOR FIELD"; fail=1
fi
bad=$(printf '%s\n' "$E" | grep -oE '^\| `[^`]+`' | sed 's/^| `//; s/`$//' \
  | grep -vcE '^[a-z]+(_[a-z]+)*\.[a-z]+(_[a-z]+)*$')
echo "bad event kinds: $bad"; [ "$bad" = 0 ] || fail=1
[ "$(grep -c '^### 5\.10 Delegated callers' "$A")" = 1 ] || { echo "NO 5.10"; fail=1; }
[ "$(grep -c '^| R12 |' "$A")" = 1 ] || { echo "NO R12"; fail=1; }
[ "$(grep -c '→ §5.10 for Fleet' "$A")" = 3 ] || { echo "REPLACED ROWS NOT MARKED"; fail=1; }
[ "$(grep -cE '^### 7\.(1[3-9]|2[0-6]) ' "$C")" = 14 ] || { echo "NOT 14 TEAM SECTIONS"; fail=1; }
echo "result: $([ "$fail" = 0 ] && echo PASS || echo FAIL)"
exit "$fail"
```

Run it **now, before Steps 2 to 10 are written into the two documents**:

```bash
~/.claude/tools/claudezsh/claudezsh 'bash .superpowers/sdd/team/t01-check.sh | tail -4'
```

Expected: `MISSING` lines for every field, `NO 5.10`, `NO R12`, `REPLACED ROWS NOT MARKED`,
`NOT 14 TEAM SECTIONS`, and `result: FAIL`. That is the failing test.

- [ ] **Step 12: Write Steps 2 to 10 into the documents, then run the check to green**

Write each section from Steps 2 to 10. **The fenced blocks are normative and copied verbatim**; the
Conventions paragraph of Step 6 is the first paragraph of §7.13. Then:

```bash
~/.claude/tools/claudezsh/claudezsh 'bash .superpowers/sdd/team/t01-check.sh'
```

Expected, exactly:

```text
fields: 69
bad event kinds: 0
result: PASS
```

Then the one check the script cannot make: read §5.3's team rows against §3.4 of `05-design.md`
row by row, and confirm no `✓` is in an `M′` or `A` cell of a write row. Record *"matrix read
against §3.4: yes"* in the commit message.

- [ ] **Step 13: Commit**

```bash
~/.claude/tools/claudezsh/claudezsh 'git add docs/superpowers/contracts/team-records-fieldcheck.txt && git commit -F .superpowers/sdd/team/msg/t01.txt -- docs/superpowers/adr/ADR-H1-hub-architecture.md docs/superpowers/contracts/2026-09-23-hub-contract.md docs/superpowers/contracts/team-records-fieldcheck.txt'
```

The message file's subject: `docs(hub): ADR-H1 5.10 delegated callers; contract 7.13-7.26 team
records (Phase T task 01)`, then the matrix line, then the `Co-Authored-By` line.

**What this task does not claim.**
- That the Hub accepts any of it. Tasks 03 (types), 04 (tables), 05 (auth), 06 and 07 (routes),
  08 (feed and client), 43 (policy and matrix), 46 (notes and erasure) build it.
- That the caller of route 76 is settled. §7.25 leaves it open for task 66, on purpose.
- That the data class values, or the project's `mode` flag, exist on the merged project record:
  both are **UNVERIFIED until merge**; §7.24 and §7.26 name them, and task 43 reuses or adds them.
- That the section ids in `briefRef` match phase 4's: **UNVERIFIED** until P4's merge.
- That a delegation makes Fleet safe. R12 says what it bounds and what remains.
