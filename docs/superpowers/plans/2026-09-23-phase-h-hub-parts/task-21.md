### Task 21: blobs — content-addressed, per org, in Postgres behind `blobstore` (routes 28–29; graft G2)

**Lane A. Depends on: 17. About 3 hours. Contract task H4.**

Phase 5 captures an approved design's snapshot into the Hub **at approval**
(`paper-design-cleanroom-spec.md:474`, as ADR-H1 G2 cites it): a handful of HTML, CSS, image and
token files per approval, uploaded first, then referenced by the approval (task 22). G2 stores them
in Postgres — transactional with the approval that references them, backed up with everything
else, erased with their records — behind a `blobstore` interface, so an Azure Blob implementation
can replace it without a route changing (ADR-H1 §5.6).

**Files:**
- `control-plane/internal/hub/blobstore/blobstore.go` + `postgres.go` (new)
- `control-plane/internal/hub/server/blobs.go` + `blobs_integration_test.go` (new)
- `control-plane/internal/hub/store/blobs.go` — associations and the quota sum (new)
- `control-plane/internal/hub/server/tx.go` — `WriteStream`, the body-less write (modify)
- `control-plane/cmd/hub/serve.go` — pass `BlobMaxBytes`, `ProjectQuota`, the byte budget (modify)
- `control-plane/internal/hub/server/matrix_integration_test.go` — `rowsTask21` (modify)
- `control-plane/internal/hub/server/server.go` — `s.registerBlobs()` (modify: one line)

#### Contract — complete

```go
// control-plane/internal/hub/blobstore/blobstore.go

// Package blobstore holds blob BYTES. Which project may see a blob — the
// association, the quota — is the store package's, in the same transaction.
// The v1 implementation is Postgres (hub.blobs); a later one may keep bytes in
// Azure Blob Storage and ignore tx, and the routes do not change (ADR-H1 §5.6).
package blobstore

var ErrNotFound = errors.New("blobstore: no such blob in this org")

type Store interface {
	// Put stores data under digest for org, once: created is false when the org
	// already holds these bytes. The caller has verified SHA-256(data) == digest.
	Put(ctx context.Context, tx pgx.Tx, orgID string, digest [32]byte, data []byte) (created bool, err error)
	Get(ctx context.Context, tx pgx.Tx, orgID string, digest [32]byte) ([]byte, error)
	Delete(ctx context.Context, tx pgx.Tx, orgID string, digest [32]byte) error
}

func Postgres() Store
```

```go
// control-plane/internal/hub/server/tx.go — added by this task.

// WriteStream is Write (task 14) for a request whose body is not JSON: it reads
// no body, takes no Idempotency-Key (the route is a PUT, and content addressing
// already makes it idempotent), and otherwise follows contract §10.6 exactly.
func (s *Server) WriteStream(w http.ResponseWriter, r *http.Request, fn func(tx *Tx) (WriteResult, error))
```

**Route 28 — `PUT /v1/projects/{projectId}/blobs/{sha}`**, raw bytes, role M. In order:

1. `{sha}` is 64 lowercase hex (`hubapi.ParseDigestHex`), else 400 `field: sha`.
2. `Content-Type` is one of `hubapi.BlobContentTypes`, exactly, else 400 `field: Content-Type`.
3. `Content-Length` is present, else 400 `field: Content-Length`; above
   `PANACEA_APP_HUB_BLOB_MAX_BYTES` (default 16 MiB) → **413 `blob_too_large`**,
   `details.limitBytes`, **before the body is read**.
4. The user's byte budget (`BlobBudget.Reserve(tid/oid, Content-Length)`, task 13) → 429
   `rate_limited`, `scope: blob_bytes`, `Retry-After`.
5. Read exactly `Content-Length` bytes through a SHA-256 hasher, before any transaction opens (a
   16 MiB read must not hold the org's write lock). A short or long body → 400 `bad_request`. A
   hash that is not `{sha}` → **422 `digest_mismatch`**, `details.digest`, nothing stored.
6. `WriteStream`: authorise M of the project (404 `project_not_found`; 403 `required: member`);
   the project's associated total + this size > `PANACEA_APP_HUB_PROJECT_QUOTA_BYTES` (1 GiB) and
   the blob is not already associated → **409 `quota_exceeded`**, `details.limitBytes`;
   `blobstore.Put`; insert the `project_blobs` association if absent (the first `content_type`
   wins; a later upload of the same bytes answers the stored type).
7. `201 hubapi.Blob` when newly associated with **this project**, `200` when it already was.
   Event `blob.uploaded {"size","contentType"}`, subject `sha256:<hex>` — **audit only** (§8).

**Route 29 — `GET` and `HEAD /v1/projects/{projectId}/blobs/{sha}`**, role V. The bytes, with
`Content-Type` as stored, `Content-Length`, `Content-Disposition: attachment;
filename="<sha>"`, `Content-Security-Policy: sandbox; default-src 'none'`,
`X-Content-Type-Options: nosniff`, `Cache-Control: no-store`. A blob not associated with **this**
project → 404 `blob_not_found`, whichever other project holds it. `HEAD` sends the headers only.

**Per org, not global.** Identical bytes uploaded by two projects of one org are stored once and
associated with each; the uploader has proved it holds the bytes, so the association leaks
nothing (§7.6). Two orgs never share a blob row (the primary key is `(org_id, digest)`).

#### Tests to write first — `blobs_integration_test.go`

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestUploadThenDownload` | PUT 48 KiB of HTML → 201 `{digest,size,contentType}`; GET → the same bytes and every header above | a blob served inline as HTML, runnable in the webview (T8) |
| `TestReuploadIs200` | the same PUT again → 200, one `project_blobs` row | a second association, double-counting the quota |
| `TestAHashMismatchStoresNothing` | bytes that do not hash to `{sha}` → 422 `digest_mismatch`; no `blobs` or `project_blobs` row | a path that lies about its content being stored under it |
| `TestTheContentTypeAllowlist` | `text/html` (no charset), `application/javascript`, none → 400 `field: Content-Type` | a script uploaded and later served |
| `TestTooLargeIsRefusedBeforeReading` | `Content-Length: 16777217` with a body reader that fails the test when read → 413 | a 16 GiB body read into memory before the check (T9) |
| `TestContentLengthIsRequired` | chunked encoding with no length → 400 `field: Content-Length` | — |
| `TestBytesAreSharedWithinAnOrgOnly` | the same bytes to P1 and P2 of one org → one `blobs` row, two associations; the same bytes by Xavier in org 2 → a second `blobs` row | cross-org dedup, which would leak that another tenant holds a file |
| `TestAnotherProjectsBlobIs404` | associated with P2 only → GET through P1 → 404 `blob_not_found` | reading a blob by digest across projects |
| `TestTheProjectQuota` | quota set to 100 KiB; 60 KiB then 60 KiB → the second 409 `quota_exceeded`, `limitBytes: "102400"`; re-uploading the first → 200 | a quota that counts re-uploads, or none |
| `TestTheByteBudget` | budget set to 100 KiB/min; two 60 KiB uploads → the second 429 `scope: blob_bytes` with `Retry-After` | a loop filling the database |
| `TestUploadsAreAuditedNotFed` | one `blob.uploaded` event, `feed = false` | teammates' feeds flooded with asset uploads |
| `TestHeadSendsHeadersOnly` | HEAD → 200, `Content-Length` set, no body | — |

Matrix `rowsTask21` (distinct bytes per cell):

| Route | V | M | A | O | N | X |
|---|---|---|---|---|---|---|
| `PUT /v1/projects/{ProjectID}/blobs/{sha}` | 403 | 201 | 201 | 404 | 404 | 404 |
| `GET /v1/projects/{ProjectID}/blobs/{sha}` | 200 | 200 | 200 | 404 | 404 | 404 |
| `HEAD /v1/projects/{ProjectID}/blobs/{sha}` | 200 | 200 | 200 | 404 | 404 | 404 |

#### Steps

- [ ] **Step 1:** Re-read contract §7.6, §11.3 (blob bytes), §12.5 (the janitor's blob rules,
  which task 24 builds on these tables).
- [ ] **Step 2:** Tests and rows; they fail. Then `blobstore`, `store/blobs.go`, `WriteStream`,
  `blobs.go`.
- [ ] **Step 3:** Commit by pathspec.

#### Acceptance

```bash
go test -tags integration ./control-plane/internal/hub/server/ -race -count=1 -v \
  -run 'TestUploadThen|TestReupload|TestAHashMismatch|TestTheContentType|TestTooLarge|TestContentLength|TestBytesAreShared|TestAnotherProjectsBlob|TestTheProjectQuota|TestTheByteBudget|TestUploadsAreAudited|TestHeadSends|TestAuthorisationMatrix|TestEveryRoute' \
  2>&1 | grep -c "^--- PASS"                                                                  # 14
```

#### What this task does not claim

- The 24-hour collection of unreferenced blobs: task 24's janitor.
- An object store: none is provisioned in v1 (HA-7's answer, contract §12.3).
- That phase 5's snapshot files fit the allowlist; phase 5 owns its manifest and its types.
