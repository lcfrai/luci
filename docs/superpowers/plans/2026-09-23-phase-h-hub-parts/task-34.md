### Task 34: `hubclient` methods, the digest re-check, and the conformance suite against the real Hub

**Lane C. Depends on: 33, 20 (more groups run as 22–27 land), 11 (`devissuer.New`, `MintAccessToken`; review M15). About 3 hours. Contract task C2.**

Every `hubclient.Hub` method over HTTP, each mapped to its contract §7 route, and the one
guarantee the client adds on read: it **re-computes the digest of every brief revision it
receives** with `hubapi.Canonical`, and a mismatch is never rendered (contract §9.2; provenance
contract §6.2). Then task 05's conformance suite runs against a **real** Hub — in-process, on a
scratch database, with tokens from an in-process `devissuer` — which is the proof that what phases
3, 4, 5 and 7 built against the fake holds against the server.

**Files:**
- `control-plane/internal/hub/hubclient/{methods_projects.go,methods_briefs.go,methods_designs.go,methods_releases.go,methods_feed.go}` (new)
- `control-plane/internal/hub/hubclient/verify.go` — the digest re-check (new)
- `control-plane/internal/hub/hubclient/methods_test.go` (new)
- `control-plane/internal/hub/hubclient/conformance/hub_integration_test.go` (new)

#### Contract — the method → route map, complete

| Method | Route (contract §7) | Notes |
|---|---|---|
| `Me` | `GET /v1/me` | |
| `ProjectForRepo(repoKey)` | `GET /v1/projects?repo=<repoKey>` | `ok` = one item; two items is a Hub bug → `ErrUnavailable` |
| `Project`, `CreateProject`, `AddRemote` | routes 15, 14, 17 | `CreateProject`, `AddRemote` send an `Idempotency-Key` |
| `Teams`, `CreateTeam`, `Team` | routes 4, 5, 6 | `Teams` returns `items` |
| `SetMember`, `RemoveMember` | routes 8, 9 | |
| `Invites`, `CreateInvite`, `RevokeInvite` | routes 10, 11, 12 | `Invites` follows `next` to the end (a team has few) |
| `List(projectID, q)` | route 19, `?scope=&branchRef=&includeArchived=&after=&limit=` from `hubapi.BriefQuery` (HA4-2, task 00) | one page, as asked |
| `CreateBrief`, `ForBranch`, `Brief`, `Rebind`, `Archive` | routes 20, 21, 22, 23, 24 | every revision in a response is re-checked — `latestComplete` included (HA4-4) |
| `Revisions(briefID, page)` | route 25 | one page, as asked |
| `WriteRevisions(projectID, writes)` | route 26 | more than `hubapi.MaxRevisionBatch` writes → 400-shaped `*Error` `field: revisions` before any request; each `content` is replaced by `hubapi.Canonical(content)` before sending — a failure is a 400-shaped `*Error` `field: revisions[<i>].content` before any request — and task 33's transport encodes the body without HTML escaping, so each item's wire bytes are fewer than its canonical envelope's and 64 maximum-size revisions fit `hubapi.MaxRevisionBatchBytes` whatever their characters (HA4-3; task 00, the wire rule). `json.Compact` is not enough: it keeps a writer's `é`-style escapes. Re-checks each returned revision |
| `Resolve(keys)` | route 27, in batches of **100**, results concatenated in request order, `heads` merged | a revision failing the re-check becomes `{resolution: missing, revision: null}` and an `error` log line naming the key |
| `PutBlob(projectID, digest, contentType, size, body)` | route 28, `PUT …/blobs/<64 hex>` with `Content-Type` and `Content-Length: size` | `body` is an `io.ReadSeeker` (HA5-1, task 00), passed to task 33's transport, which seeks it to 0 before every attempt; retried on a transient failure like any call |
| `GetBlob(projectID, digest)` | route 29 | the `Blob` from `Content-Type` and `Content-Length`; the caller closes the body; a body past 16 MiB is cut off with an error |
| `Approve`, `Approval`, `Approvals`, `LinkSession`, `SessionLinks` | routes 30, 32, 31, 33, 34 | |
| `Reserve`, `Confirm`, `Abandon`, `JoinDeployment`, `Release`, `Releases` | routes 35, 38, 39, 40, 37, 36 | `JoinDeployment` sends its `DeploymentJoinRequest` as the body (HA7-1); `Releases` sends `kind=` when `q.Kind` is set (HA7-5) |
| `RecordEvidence` | route 41 | |
| `EvidenceByDigest(repo, digests)` | route 42, in batches of 100, concatenated, then **sorted oldest first** again across batches | |
| `Changes(after, limit)` | route 43 | |

```go
// control-plane/internal/hub/hubclient/verify.go

// ErrDigestMismatch: a revision's content does not hash to its digest. Outside
// Resolve (where it becomes "missing") a method returns it wrapped in
// ErrUnavailable: a Hub whose data cannot be trusted is, to the caller, down.
var ErrDigestMismatch = errors.New("hubclient: a brief revision does not match its digest")

// VerifyRevision rebuilds hubapi.BriefEnvelope from r's fields and content and
// compares its digest with r.Digest. An erased revision (content null) is not
// checked: there is nothing left to check, and its resolution says so.
func VerifyRevision(r hubapi.BriefRevision) error

// FormatRecorder is phase 3's Record.RecordedBy string for a Hub user (contract
// §14, H-4): "<displayName> <<email>>".
func FormatRecorder(u hubapi.User) string
```

#### Contract — the real-Hub conformance harness

`hub_integration_test.go` (`//go:build integration`) builds **one Hub per test binary**:
`hubtest.Migrated`, `devissuer.New` on an `httptest.Server`, `entra.New` against it
(`AllowInsecureAuthority`), and `server.New` with an injected clock — then per row a fresh team, so
rows do not see each other. Each principal's `hubclient.Client` uses a static `TokenSource` whose
token is `issuer.MintAccessToken(tenant, oid, desktop client, hub API, ["Hub.Access"])`. `Env`:
`Admin` = Grace, `Member` = Linus, `Viewer` = Vera, `Outsider` = Out Sider, and `Advance` moves the
injected clock — so the `⏱` rows run here too.

**Which groups run** is read from the server itself, not from a list someone must remember to edit:
the test asks `server.Patterns()` (task 08's chassis records them) and runs a group only when every
route it needs is registered — `designs` needs routes 28–34, `releases` 35–40, `evidence` 41–42,
`feed` 43. A skipped group is reported by name (`t.Log`), and task 43 requires **all eight**.

#### Tests to write first

`methods_test.go` (unit, against an `httptest.Server` that records requests and answers canned
JSON):

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestEveryMethodCallsItsRoute` | one row per line of the map above: method, path, query, and the JSON body's key set | a client and a server that disagree about a path the compiler cannot check |
| `TestWriteRevisionsSendsCanonicalContent` | 64 writes whose `content` is written non-canonically (keys out of order, `"é"` and `"<"` escapes, whitespace) and holds ~60,000 `<`, `>` and `&` characters, each envelope just under 64 KiB canonical → one request, its body ≤ `hubapi.MaxRevisionBatchBytes`, and each item's `content` byte-equal to `hubapi.Canonical` of the write; a `content` with a duplicate key → `*Error` `field: revisions[3].content` and no request | `json.Compact` in place of `Canonical`, or `json.Marshal` in the transport: either one puts the batch HA4-3 promised over 4 MiB on the wire (task 00, the wire rule) |
| `TestResolveBatchesByHundred` | 250 keys → 3 requests of 100, 100, 50; the results in the original order; `heads` from all three | a 400 for a batch the caller did not know was too big |
| `TestAMismatchedRevisionResolvesMissing` | the fake answers a revision whose content was altered after digesting → that slot `missing`, the others intact, one error logged | a tampered revision rendered (provenance contract §6.2) |
| `TestAMismatchedHeadIsUnavailable` | `Brief()` receiving the same → `errors.Is(err, ErrUnavailable)` and `ErrDigestMismatch` | a Discover pane rendering text its key does not describe |
| `TestErasedRevisionsAreNotRechecked` | an erased revision passes through as `erased` | every erased key turned into `missing` |
| `TestPutBlobStreams` | `Content-Length` and `Content-Type` as given; the body arrives whole; a 503 then 201 with a `bytes.Reader` → two attempts, the second body whole; a `bytes.Reader` already read to its end → the whole blob arrives (rewound first) | a retry that sends an empty body (HA5-1) |
| `TestGetBlobReadsItsMetadata` | `Blob.ContentType` and `Size` from the headers; the body is the bytes | — |
| `TestProjectForRepo` | zero items → `ok == false`; one → the project | a missing link treated as an error |
| `TestEvidenceBatchesStayOldestFirst` | 150 digests over two batches with interleaved times → one list, oldest first | phase 3 taking a later judgement as the first |
| `TestFormatRecorder` | `{DisplayName:"Linus Example", Email:"linus@example.test"}` → `Linus Example <linus@example.test>` | H-4's string drifting from phase 3's field |

`hub_integration_test.go`:

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestTheRealHubConforms` | `conformance.Run` over every landed group passes | the fake and the Hub disagreeing — the thing that would make every phase's S0 work wrong |
| `TestAllGroupsAreDetected` | with every route registered (after task 27), the harness finds all eight groups | a group silently skipped for ever |

#### Steps

- [ ] **Step 1:** Re-read contract §7 (all routes), §9.2.
- [ ] **Step 2:** `methods_test.go`; it fails. Then the five method files and `verify.go`.
- [ ] **Step 3:** `hub_integration_test.go`. It runs the groups the server has so far; record in the
  commit message which ran.
- [ ] **Step 4:** Commit by pathspec. When task 27 has landed, re-run it; task 43 checks all eight.

#### Acceptance

```bash
go test ./control-plane/internal/hub/hubclient/ -race -count=1                                                        # ok
go test -tags integration ./control-plane/internal/hub/hubclient/conformance/ -race -count=1 -v 2>&1 | grep -E "^--- (PASS|SKIP)|group"
# after task 27: TestFakeConforms PASS, TestTheRealHubConforms PASS, TestAllGroupsAreDetected PASS; no group skipped
```

#### What this task does not claim

- That the control plane caches results well: phase 4's `provenanceSlice` and task 36's feed own
  invalidation.
- A real sign-in: tokens are minted; task 43 drives the PKCE flow end to end.
