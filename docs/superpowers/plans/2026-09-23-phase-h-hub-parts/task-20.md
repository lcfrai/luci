### Task 20: `/v1/refs` — the batch resolver, brief half; `hub verify` re-checks every revision (start line S4, server side)

**Lane A. Depends on: 19. About 2.5 hours. Contract task H3.**

`GET /v1/refs` is the provenance thread's one loader (contract §7.5; ruling R6): every lens that
shows a brief key — Discover's past revision, Design's approval, Develop's `contextRef`, Deploy's
pin — resolves it here, in batches of up to 100. A key names an immutable revision, so its result
never changes, **except** that it can become `erased`; only the `heads` map varies with time. When
this task is merged, **start line S4 is reached on the server side**: phase 4 can integrate its
Hub-backed brief store against the local stack (with tasks 28 and 30–35 on the client side).

**Files:**
- `control-plane/internal/hub/server/refs.go` + `refs_integration_test.go` (new)
- `control-plane/internal/hub/store/refs.go` (new)
- `control-plane/cmd/hub/verify.go` — register the `revisions` check (modify)
- `control-plane/cmd/hub/verify_integration_test.go` (new)
- `control-plane/internal/hub/server/matrix_integration_test.go` — `rowsTask20` (modify)
- `control-plane/internal/hub/server/server.go` — `s.registerRefs()` (modify: one line)

#### Contract — the route, complete

**Route 27 — `GET /v1/refs?key=<k1>&key=<k2>…`**, any caller; the answer is per key.

- 1–100 `key` parameters (else 400 `bad_request`, `field: key`); `brief:` and `design:` mixed; a
  repeated key is answered in **each** slot. `200 hubapi.RefsResponse`, one result per key **in
  request order**.
- Per key:

| The key | `kind` | `resolution` | `revision` | `approval` |
|---|---|---|---|---|
| not in contract §3.2's grammar (`hubapi.ParseKey`) | `null` | `invalid` | `null` | `null` |
| `brief:` — no such revision, **or** its project is not readable by the caller | `"brief"` | `missing` | `null` | `null` |
| `brief:` — readable, stored bytes hash to the stored digest | `"brief"` | `resolved` | the `BriefRevision` | `null` |
| `brief:` — readable, **erased** | `"brief"` | `erased` | the `BriefRevision` with `content: null`, key, digest, `savedAt`, `savedBy`, `erasedAt` | `null` |
| `brief:` — readable, bytes **do not** hash to the digest | `"brief"` | `missing` — and an `internal` error is **logged** with the key, because this is data loss | `null` | `null` |
| `design:` | `"design"` | `missing` until task 22; then as task 22 says | `null` | … |

- `heads`: one entry per brief named by a `resolved` or `erased` brief key (and, from task 22, the
  brief of every resolved approval's `briefKey`): `{"headRevision","latestCompleteRevision",
  "archived"}`. `{}` when none.
- **Readability** is the caller's membership of the brief's project's team (contract §5.3,
  "resolves keys in readable projects; every other key is `missing`"). The lookup is **one
  query** for all brief keys — `WHERE (brief_id, revision) IN (…)` joined to `briefs`, `projects`
  and the caller's `team_members` — never one per key.

**The digest re-check on read.** The database CHECK (`brief_revisions_canonical_hashes`, task 07)
makes a mismatch impossible **unless the owner role has dropped it**. The Hub re-checks anyway,
because that is the tampering the check cannot see (contract T5), and a revision that no longer
hashes to its key must never be rendered as if it did (provenance contract §6.2).

**`hub verify`'s `revisions` check.** For each org (listed by `SELECT * FROM hub.org_ids()`, then `set_config` per org), walk
every non-erased revision: `SHA-256(canonical) = digest`; `hubapi.ParseBriefEnvelope(canonical)`
succeeds and its `briefId`, `revision`, `projectId`, `scope`, `branchRef` equal the row's columns;
its `content.status` equals the `status` column. The first fault prints `FAULT revisions: <key>:
<what>` and exits 1. Not `SELECT id FROM hub.orgs`: `hub verify` runs on `hub_app`'s connection,
where `orgs` is under row-level security (review M10, A18, task 07a) and shows no row before a
`set_config`, so that query would walk no org and pass.

#### Tests to write first

`refs_integration_test.go`:

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestTheFiveResolutions` | one call with a resolved key, a nonexistent revision (`missing`), a key of Out Sider's project (`missing` for Grace — **not** 404, not `forbidden`), `b-42` (`invalid`, `kind: null`), and a key erased by `hub.erase_brief` (`erased`, content `null`, digest kept) | a resolver that leaks existence; one that drops erased keys as missing |
| `TestOrderAndRepeatsAreKept` | `[k2, k1, k2]` → three results in that order | a caller reading slot `i` and getting another key's answer |
| `TestKeyCountLimits` | 0 keys and 101 keys → 400 `field: key` | an unbounded batch (T9) |
| `TestHeadsCoverResolvedAndErasedBriefs` | `heads` has exactly the briefs of the resolved and erased keys, with their current head and archive state | a `provenanceSlice` that cannot invalidate a head |
| `TestOnlyHeadsChangeOverTime` | resolve k1; write r2; resolve k1 again → `results` byte-identical, `heads[b].headRevision` 1 → 2 | a result that drifts (a revision is immutable, P3) |
| `TestOneQueryForAllBriefKeys` | 100 brief keys → the store's query counter (a `pgx` tracer in the test) records **one** revision query | N+1 queries under the one loader |
| `TestATamperedRevisionResolvesMissing` | as the owner: drop `brief_revisions_canonical_hashes`, disable `brief_revisions_guard`, alter one byte of `canonical` → that key resolves `missing` and one `internal` log line names it | a tampered revision rendered as the real one |

`verify_integration_test.go` (in `cmd/hub`):

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestVerifyPassesOnAHealthyDatabase` | `run(["verify"])` against a scratch database with revisions in two orgs → exit 0, `ok revisions`; with one org's revision tampered instead → exit 1 whichever org it is (review M10) | a verify that always fails; a verify that lists orgs by `SELECT id FROM hub.orgs`, sees none, and passes |
| `TestVerifyFailsOnATamperedRevision` | the tamper above → exit 1, `FAULT revisions: brief:<id>@<n>: digest mismatch` | the contract's H3 acceptance: *"a row whose canonical is altered by the owner role fails hub verify"* |
| `TestVerifyFailsWhenTheEnvelopeDisagreesWithTheRow` | as the owner, change a revision's `branch_ref` column only (canonical untouched) → exit 1 | a column edited behind the digest's back |

Matrix `rowsTask20` (the harness gains `wantResolution map[column]hubapi.Resolution`):

| Route | V | M | A | O | N | X |
|---|---|---|---|---|---|---|
| `GET /v1/refs?key={BriefKey}` | 200 `resolved` | 200 `resolved` | 200 `resolved` | 200 `missing` | 200 `missing` | 200 `missing` |

#### Steps

- [ ] **Step 1:** Re-read contract §3.2, §7.5 (the resolver), §15; provenance contract §4.1's
  states (the Hub adds `erased`).
- [ ] **Step 2:** Tests and the row; they fail. Then `store/refs.go`, `refs.go`, the verify check.
- [ ] **Step 3:** Commit by pathspec. **Announce S4 (server side)** in the commit message's first
  line: `— S4 server side: phase 4 may integrate against devstack`.

#### Acceptance

```bash
go test -tags integration ./control-plane/internal/hub/server/ -race -count=1 -v \
  -run 'TestTheFiveResolutions|TestOrderAndRepeats|TestKeyCountLimits|TestHeadsCover|TestOnlyHeadsChange|TestOneQueryFor|TestATamperedRevision|TestAuthorisationMatrix|TestEveryRoute' \
  2>&1 | grep -c "^--- PASS"                                                                  # 9
go test -tags integration ./control-plane/cmd/hub/ -run TestVerify -race -count=1 -v 2>&1 | grep -c "^--- PASS"   # 3
```

#### What this task does not claim

- Design keys: task 22.
- The erasure route: task 27. This task tests the `erased` state through the database function
  task 07 created, which is what the route will call.
- That the control plane caches results correctly: task 34 and phase 4's `provenanceSlice`.
