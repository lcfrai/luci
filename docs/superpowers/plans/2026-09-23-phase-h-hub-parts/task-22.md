### Task 22: design approvals, session links, and `design:` keys in `/v1/refs` — routes 30–34 (start line S5, server side)

**Lane A. Depends on: 20, 21. About 3 hours. Contract task H4.**

One route writes **the approval, the design link and the snapshot reference together**
(`paper-design-cleanroom-spec.md:472-474`, contract §7.7): phase 5 uploads the snapshot's files
(task 21), then approves. The approver is the principal; **there is no approver field**. The row is
immutable; a re-review approves a new design revision, which is a new key. When this task is
merged, **start line S5 is reached on the server side**.

**Files:**
- `control-plane/internal/hub/server/approvals.go`, `sessionlinks.go` + `approvals_integration_test.go` (new)
- `control-plane/internal/hub/store/approvals.go`, `sessionlinks.go` (new)
- `control-plane/internal/hub/server/refs.go`, `store/refs.go` — the design half (modify)
- `control-plane/internal/hub/server/matrix_integration_test.go` — `rowsTask22`; the fixture gains an approval, `DesignKey`, `ApprovalID` (modify)
- `control-plane/internal/hub/server/server.go` — `s.registerApprovals()` (modify: one line)

#### Contract — the routes, complete

**Route 30 — `POST /v1/projects/{projectId}/design-approvals`**, body `hubapi.ApprovalRequest`, role
M, `Idempotency-Key` expected. Checks **in this order**, each with its error (contract §7.7):

| # | Check | Failure |
|---|---|---|
| 0 | M of the project | 404 `project_not_found`; 403 `required: member` |
| 1 | `designKey` and `briefKey` in §3.2's grammar; `reviewDigest`, `briefDigest`, `snapshot.manifest` and each `snapshot.blobs[i]` are `sha256:` + 64 hex (the `hubapi.Digest` decoder); `branchRef` is `""` or valid (§3.3) | 400 `bad_request`, `field` named |
| 2 | `briefKey` names a revision of a brief **in this project** that is **not erased** | 422 `key_unresolvable`, `details.key` |
| 3 | `briefDigest` equals that revision's digest | 422 `digest_mismatch`, `details.key` |
| 4 | that revision's `status` is `complete` (ADR-H1 Q1) | 422 `brief_incomplete`, `details.key` |
| 5 | `snapshot.blobs` has 1–500 **distinct** digests, **includes** `snapshot.manifest`, and every one is associated with this project | 400 `field: snapshot.blobs` (count, duplicates, manifest absent); 422 `blob_missing`, `details.digest` = the first absent one in request order |
| 6 | no approval exists for `designKey` in this project | 409 `design_already_approved`, `details.designKey`, `details.approvalId` |

Then, in the one transaction: insert `design_approvals` (`canvas_id` and `design_revision` parsed
from the key; `approved_by` = the principal), insert one `design_approval_blobs` row per digest,
`Emit design.approved {"designKey","briefKey"}`, answer `201 hubapi.DesignApproval` with
`snapshot.blobs` as `Blob` objects (digest, size, stored content type) in request order.

**What the Hub does not check** (§7.7): `reviewDigest` is phase 5's own SHA-256 over its canonical
artboard serialisation — recorded as given; the manifest's contents are phase 5's; an approval
against a complete revision **older than the head** is allowed (shown as superseded, not refused).

**Route 31 — `GET /v1/projects/{projectId}/design-approvals?canvasId=&branchRef=&after=&limit=`**, V:
`200 Page[DesignApproval]`, newest first. **Route 32 — `GET /v1/design-approvals/{approvalId}`**, V:
`200 DesignApproval`; 404 `approval_not_found` (unknown, unreadable, malformed — the same).

**Route 33 — `POST /v1/projects/{projectId}/session-links`**, body `hubapi.SessionLinkRequest`, M:

| Check | Failure |
|---|---|
| `designKey`, `briefKey` in grammar; `sessionId` matches `^[a-z]{1,4}-[0-9a-f]{8,32}$`; `branchRef` valid | 400, `field` named |
| `designKey` has an approval in this project | 422 `design_not_approved`, `details.designKey` |
| `briefKey` names a revision of a brief in this project (erased or not: the key the control plane **stamped** into `contextRef`, provenance contract P6, exists) | 422 `key_unresolvable`, `details.key` |
| the same user has already linked this `(designKey, sessionId)` | **200** with the existing link — not an error |

`201 hubapi.SessionLink`; `Emit session_link.created {"designKey","briefKey","sessionId"}`. The
`briefKey` may differ from the approval's: that difference is the point of recording both
(`prototype-fact-sheet.md:92`, as the contract cites it).

**Route 34 — `GET /v1/projects/{projectId}/session-links?designKey=&sessionId=&after=&limit=`**, V:
`200 Page[SessionLink]`, newest first.

**`/v1/refs`, design half.** A `design:` key resolves `resolved`, with `approval` set, when an
approval for it exists **in a project the caller can read**; else `missing`. Canvas ids are minted
by phase 5 in the control plane, so two projects could in principle hold one design key; if more
than one readable project does, the **earliest** approval answers and a warning is logged
(phase 5 should mint canvas ids with `ids.Wide("cv")`, contract §3.2, making this unreachable).
`heads` gains the brief of every resolved approval's `briefKey`.

#### Tests to write first — `approvals_integration_test.go`

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestAnApprovalWritesLinkAndSnapshotTogether` | upload manifest + 2 files; approve → 201; `design_approval_blobs` holds 3 rows; one `design.approved` event; the response's blobs carry size and type | an approval whose snapshot can be collected by the janitor |
| `TestApprovalChecksRunInOrder` | six subtests, one per check, each failing only that check → its code; one body failing checks 2 and 5 → `key_unresolvable` (check 2 wins) | checks reordered so a caller learns about blobs of a brief it named wrongly |
| `TestTheApproverIsThePrincipal` | a body with `"approvedBy":"u-…"` → 400; a valid approval's `approvedBy` is the caller | an approval attributed to someone else (contract §5.3) |
| `TestASecondApprovalIs409` | the same `designKey` twice → 409 `design_already_approved` with `approvalId` | two approvals of one design revision |
| `TestAnOlderCompleteRevisionMayBeApproved` | revisions 1 (complete) and 2 (complete); approving against `@1` → 201 | a refusal where provenance contract §3.4 says *"disagreement is shown, not resolved"* |
| `TestAnIncompleteRevisionIsRefused` | against an `open` revision → 422 `brief_incomplete` | Q1's rule missing |
| `TestAnErasedBriefCannotBeApprovedAgainst` | after `hub.erase_brief` → 422 `key_unresolvable` | an approval against content nobody can read |
| `TestListAndGet` | two approvals on two canvases; `canvasId=` filters; newest first; `GET` by id; an unknown id → 404 `approval_not_found` | — |
| `TestSessionLinkRules` | unapproved design → 422 `design_not_approved`; another project's brief key → 422 `key_unresolvable`; `sessionId` `S-1` → 400 | a link to a design that was never approved |
| `TestASessionLinkRepeatReturnsTheSame` | the same user, same pair → 200 with the first link's id; another user, same pair → 201, a second link | duplicate links from a retried session start |
| `TestRefsResolvesDesignKeys` | an approved key → `resolved` with the approval, and `heads` holds its brief; for Out Sider → `missing` | a design thread invisible to Develop and Deploy |

Matrix `rowsTask22`:

| Route | V | M | A | O | N | X |
|---|---|---|---|---|---|---|
| `POST …/{ProjectID}/design-approvals` (a fresh design key per cell) | 403 | 201 | 201 | 404 | 404 | 404 |
| `GET …/{ProjectID}/design-approvals` | 200 | 200 | 200 | 404 | 404 | 404 |
| `GET /v1/design-approvals/{ApprovalID}` | 200 | 200 | 200 | 404 | 404 | 404 |
| `POST …/{ProjectID}/session-links` (a fresh session id per cell) | 403 | 201 | 201 | 404 | 404 | 404 |
| `GET …/{ProjectID}/session-links` | 200 | 200 | 200 | 404 | 404 | 404 |
| `GET /v1/refs?key={DesignKey}` | `resolved` | `resolved` | `resolved` | `missing` | `missing` | `missing` |

#### Steps

- [ ] **Step 1:** Re-read contract §7.7, §7.8, §7.5 (design keys in the resolver), §8.
- [ ] **Step 2:** Tests and rows; they fail. Then the two stores, the two handlers, the refs half.
- [ ] **Step 3:** Commit by pathspec. **Announce S5 (server side)** in the first line.

#### Acceptance

```bash
go test -tags integration ./control-plane/internal/hub/server/ -race -count=1 -v \
  -run 'TestAnApprovalWrites|TestApprovalChecks|TestTheApproverIs|TestASecondApproval|TestAnOlderComplete|TestAnIncomplete|TestAnErasedBriefCannot|TestListAndGet|TestSessionLinkRules|TestASessionLinkRepeat|TestRefsResolvesDesign|TestAuthorisationMatrix|TestEveryRoute' \
  2>&1 | grep -c "^--- PASS"                                                                  # 13
```

#### What this task does not claim

- Separation of duties — v1 lets a designer approve their own design (ADR-H1 Q3).
- That an agent cannot cause an approval: that is phase 5's rule (no MCP tool approves) and the
  control plane's (the approve handler is reachable only from the chrome); ADR-H1 R5.
- Revocation: none in v1.
