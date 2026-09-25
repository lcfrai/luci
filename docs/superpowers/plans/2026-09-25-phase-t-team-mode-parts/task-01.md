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
identity. Its Entra token is app-only (`idtyp=app`, no `scp`), which `ADR-H1:369` refuses today.
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
  event v2; §4.2 eight codes (the scope's six, plus `delegation_revoked` so Fleet can
  tell *renew* from *stop*, and `delegation_not_found` for the owner's own routes); §5.1 the delegated step; §5.3 the team rows; §7.0 rows 46 onward;
  new **§7.13 to §7.20**; §8 the team events; §12.2 two variables (modified)
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
  - the problem codes `delegation_required`, `delegation_expired`, `delegation_revoked`,
    `delegation_not_found`, `agent_not_owner`, `text_refused`, `glob_refused`, `breadth_exceeded`
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

1. **A second caller, app-only, one identity.** A token whose `idtyp` is `app` is accepted only
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

**§3.1**, append to the prefix table (the Hub mints the first three; Fleet mints the rest with
`ids.Wide` and sends them as the body's `id`, because it writes through an outbox that must name a
record before the Hub has answered, Phase T task 11):

```markdown
| `dl-` | delegation (Hub mints) | | `th-` | thread (Fleet mints) |
| `hm-` | human message (Hub mints) | | `ms-` | message (Fleet mints) |
| `nt-` | note (Hub mints) | | `es-` | escalation (Fleet mints) |
| `tk-` | task (Fleet mints) | | `dv-` | directive (Fleet mints) |
| `ag-` | agent (Fleet mints) | | `ot-` | orchestrator turn (Fleet mints) |
| `ls-` | lease (Fleet mints) | | `cf-` | conflict (Fleet mints) |
```

and one sentence under it: *"A Fleet-minted id is validated by `hubapi.ValidID` and must be new in
its org: a reuse with a different body is 409 `conflict`; a retry with the same `Idempotency-Key`
replays (§1)."* All twelve are disjoint from the ten above and from the control plane's five.

**§3.6**, replace the audit event row's field list with: `v` (`1`, or `2` for a delegated write),
`kind`, `orgId`, `seq`, `at`, `actorUserId` (or `""`), `event`, `projectId` (or `""`), `subject`,
`data`, and **only when `v` is 2**, `via` (`"fleet"`) and `delegationId`. Add: *"A v1 event's bytes
are unchanged by this amendment; the §3.5 golden vectors stand. A v2 golden vector is added by
Phase T task 03."*

**§4.2**, append to the codes table:

```markdown
| `delegation_required` | 401 | a Fleet token on a route other than the per-org feed, with no `X-Panacea-Delegation` | — |
| `delegation_expired` | 401 | the named delegation's `expiresAt` has passed | `details.delegationId` |
| `delegation_revoked` | 401 | the named delegation has `revokedAt` set | `details.delegationId`, `details.reason` |
| `agent_not_owner` | 403 | an `ag-…` in the write does not belong to the delegation's owner (§5.10 rule 5) | `details.agentId` |
| `text_refused` | 422 | a free-text field fails a bound or the hygiene check (Phase T task 02) | `details.field`, `details.rule` |
| `glob_refused` | 422 | a path or glob is absolute, contains `..` or `\`, or exceeds 200 characters | `details.field`, `details.index` |
| `breadth_exceeded` | 422 | a glob set exceeds 20 globs, or one glob matches more than 10% of tracked files | `details.field`, `details.index` |
```

Add `delegation_not_found` (404) beside the other `*_not_found` codes. A named delegation that is
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
