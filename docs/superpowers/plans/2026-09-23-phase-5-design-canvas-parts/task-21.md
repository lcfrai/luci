# Task 21 — Approval: the snapshot, the blobs, the Hub's design link, *changed after approval*, re-review

**Plan:** `../2026-09-23-phase-5-design-canvas.md` · **Depends on:** 08, 09, 13, 20 · **External:** Hub H0 (`hubapi`, `hubclient.Designs`, `NewFake`), Hub C2 (the relay rule); phase 4 task 07 (`discoverhub`, through task 20's port) · **Hours:** 3

## Why

The spine (`CLAUDE.md`): the brief written in Define is visibly referenced in Design and carried
into Develop. Approval is where Design joins the thread. **Only a human approves, from our chrome**
(plan rule 4; ADR-H1 R5), over a SHA-256 digest of the design's content (task 08), against a
**complete** brief revision (ADR Q1). One Hub call writes the approval, the design link
`design:<canvasId>@<rev>` → `brief:<id>@<rev>` and the snapshot together (Hub contract §7.7), after
the snapshot's files are uploaded as blobs (§7.6). So the approved design survives the canvas —
brief R8 / P-g closed — and a release's design key always resolves.

## Files

- `canvas/handoff/snapshot.go`, `manifest.go`, `slug.go` + tests (new)
- `control-plane/internal/designapi/approve.go`, `approvals.go` (the in-memory `Approvals` port),
  `hub.go` (the `Designs` port and the relay), `approve_test.go` (new); `routes.go` gains three routes

## Contracts

```go
// canvas/handoff/snapshot.go
package handoff

type File struct {
	Path        string // relative to the bundle root, "/"-separated
	ContentType string // one of the Hub's blob types (contract §7.6)
	Data        []byte
	SHA256      string // "sha256:" + lowercase hex
}

type SnapshotInput struct {
	Doc         *doc.Doc          // the document AT the approved revision
	DesignKey   string            // doc.DesignKey(canvasID, revision)
	BriefKey    string
	BriefDigest string
	BranchRef   string            // refs/heads/…
	PNGs        map[string][]byte // artboard id → PNG at 2× (spec §7.6, Paper's default [S5])
	Asset       func(ref string) (doc.Asset, []byte, error)
	GeneratedAt string
}

type Snapshot struct {
	Files        []File // every file but the manifest, sorted by Path
	Manifest     Manifest
	ManifestFile File   // manifest.json, canonical JSON (task 08)
	ReviewDigest string // serialize.ReviewDigest(Doc, BriefKey)
}

// BuildSnapshot writes, per artboard (in page, then artboard, order), with
// slug s from slug.go:
//   <s>.html          serialize.ExportHTML, assets at "assets/<hex>.<ext>"  text/html; charset=utf-8
//   <s>.png           PNGs[id]                                             image/png
//   <s>.jsx           serialize.JSX inline                                 text/plain; charset=utf-8
//   <s>.tailwind.jsx  serialize.JSX tailwind                               text/plain; charset=utf-8
// and once: tokens.css (text/css; charset=utf-8), tokens.json, tree.json
// (application/json), assets/<hex>.<png|jpg|webp|svg> (their media type),
// then manifest.json. A missing PNG for any artboard is an error.
func BuildSnapshot(in SnapshotInput) (Snapshot, error)

const (
	MaxFiles     = 500      // the Hub's snapshot.blobs cap, manifest included (contract §7.7 check 5)
	MaxFileBytes = 16 << 20 // the Hub's blob cap (contract §7.6)
)
var ErrTooLarge = errors.New("handoff: snapshot over the Hub's limits")
```

```go
// canvas/handoff/manifest.go — fields in lexicographic JSON order (task 08's canonical JSON)

type Manifest struct {
	Artboards      []ManifestArtboard `json:"artboards"`
	BranchRef      string             `json:"branchRef"`
	BriefDigest    string             `json:"briefDigest"`
	BriefKey       string             `json:"briefKey"`
	CanvasID       string             `json:"canvasId"`
	DesignKey      string             `json:"designKey"`
	DesignRevision int                `json:"designRevision"`
	Files          []ManifestFile     `json:"files"` // every file but manifest.json, by path
	GeneratedAt    string             `json:"generatedAt"`
	Kind           string             `json:"kind"` // "design-handoff"
	ReviewDigest   string             `json:"reviewDigest"`
	V              int                `json:"v"`    // 1
}

type ManifestArtboard struct {
	Height      string `json:"height"`
	HTML        string `json:"html"`        // path
	ID          string `json:"id"`
	JSX         string `json:"jsx"`
	Name        string `json:"name"`
	Page        string `json:"page"`
	PNG         string `json:"png"`
	Slug        string `json:"slug"`
	TailwindJSX string `json:"tailwindJsx"`
	Width       string `json:"width"`
}

type ManifestFile struct {
	ContentType string `json:"contentType"`
	Path        string `json:"path"`
	SHA256      string `json:"sha256"`
	Size        int64  `json:"size"`
}
```

```go
// canvas/handoff/slug.go

// Slug: lowercase; every run of characters outside [a-z0-9] becomes "-";
// trimmed of "-"; at most 60 bytes; "" becomes "artboard"; a repeat within
// one canvas gets "-2", "-3", … in page-then-artboard order.
func Slugs(artboards []doc.Node) map[string]string // artboard id → slug
```

```go
// control-plane/internal/designapi/hub.go

// Designs is the part of hubclient.Designs phase 5 uses (plan HA5-2): the
// contract's five methods plus the Hub plan's Approval.
type Designs interface {
	PutBlob(ctx context.Context, projectID, digest, contentType string, size int64, body io.Reader) (hubapi.Blob, error)
	GetBlob(ctx context.Context, projectID, digest string) (io.ReadCloser, hubapi.Blob, error)
	Approve(ctx context.Context, projectID string, req hubapi.ApprovalRequest) (hubapi.DesignApproval, error)
	Approval(ctx context.Context, approvalID string) (hubapi.DesignApproval, error)
	Approvals(ctx context.Context, projectID string, q hubapi.ApprovalQuery) (hubapi.Page[hubapi.DesignApproval], error)
	LinkSession(ctx context.Context, projectID string, req hubapi.SessionLinkRequest) (hubapi.SessionLink, error)
}

// relay writes a Hub error to the desktop by the Hub contract's §4.3 table.
// It is hubclient.Relay when that exists (phase 4's ask HA4-5), else this
// one function, marked for deletion when it does.
func relay(w http.ResponseWriter, r *http.Request, err error) (handled bool)
```

### Routes (added to task 20's table)

| # | Method | Path | Request | `2xx` response |
|---|---|---|---|---|
| 21 | POST | `/v1/design/canvases/{canvasId}/approvals` | `{"revision": n}` | `201 hubapi.DesignApproval` |
| 22 | GET | `/v1/design/canvases/{canvasId}/approvals` | — | `200 ApprovalState` |
| 23 | POST | `/v1/design/canvases/{canvasId}/review-marks` | `{}` | `200 {"revision": n, "briefKey": "…"}` |

```go
// control-plane/internal/designapi/approvals.go

// ApprovalState is the lens's approval bar (prototype states 3, 4, 5) and
// get_canvas_info's ApprovalView (task 15).
type ApprovalState struct {
	Latest          *hubapi.DesignApproval `json:"latest"`   // null when none
	State           string                 `json:"state"`    // "none" | "approved" | "changed" | "superseded"
	HeadRevision    int                    `json:"headRevision"`
	HeadDigest      string                 `json:"headDigest"` // ReviewDigest(head, latest.briefKey); "" when none
	BriefSuperseded bool                   `json:"briefSuperseded"`
	ChangedBy       []authority.ActivityRow `json:"changedBy"` // content rows after the approved revision: who changed what (prototype state 5)
	History         []hubapi.DesignApproval `json:"history"`  // newest first, at most 20
}
```

**Approve, in order** (route 21), each failure with its answer:

1. `revision` is 1 … head, else `400 bad_request`.
2. `Briefs.Binding(projectId)` is `bound`, else `409 design_not_shared` `{state}`.
3. `Briefs.Head(projectId, branchId)` is `resolved` with status `complete`, else `409 design_no_brief`.
4. The document at `revision` (`authority.Document`), then **2× screenshots of every artboard at
   that revision** (`authority.Render`, 50 per request), else `409 renderer_unavailable` — the lens is
   open when a human approves, so this means the renderer failed.
5. `handoff.BuildSnapshot`; over `MaxFiles` or `MaxFileBytes` → `413 design_too_large`.
6. `PutBlob` for every file and the manifest, each from a `*bytes.Reader` (HA5-1), retried once on
   `ErrUnavailable` with a fresh reader.
7. `Approve` with `designKey`, `reviewDigest`, `briefKey`, `briefDigest`, `branchRef`
   (`Briefs.BranchRef`), `snapshot.manifest` and every digest in `snapshot.blobs`. **No approver
   field exists** (contract §7.7); the Hub records the principal. Its 4xx pass through unchanged
   (`brief_incomplete`, `digest_mismatch`, `blob_missing`, `design_already_approved`); its 5xx and
   transport failures are `503 upstream_unavailable` `upstream: "hub"`.
8. Hold the approval in memory (the `Approvals` port, ADR-H1 §5.4 — never on disk); close every open
   review request with one human batch labelled `Approved design rev <n>`.

**Re-review** (route 23, brief D3): the branch's current head brief key, as a `review_mark` op in a
human batch — a new design revision whose digest covers the new key (task 08, vector 2) — which the
lens then offers to approve. It is refused with `design_no_brief` when there is no complete head.

The in-memory approvals cache holds the newest approval per canvas; it is filled at approval and on
a miss from `Approvals(projectId, {CanvasID})`, and expires after 60 s so a teammate's approval on the
same Hub project shows within a minute (the Hub's change feed, contract §7.11, is read by phase 4's
loader; this cache does not poll it).

## Tests first

| Test | Asserts | Kills the mutant |
|---|---|---|
| `TestBuildSnapshot_files` | a two-artboard fixture yields exactly the file list in the contract, sorted, with correct content types | a file missing from the bundle |
| `TestBuildSnapshot_manifestCanonical` | two builds of one input give identical manifest bytes; the manifest lists every other file's SHA-256 | a manifest that does not pin the files |
| `TestBuildSnapshot_digestMatches` | `Snapshot.ReviewDigest == serialize.ReviewDigest(doc, briefKey)` | a second digest function |
| `TestBuildSnapshot_limits` | 501 files → `ErrTooLarge` | a 422 from the Hub after 500 uploads |
| `TestSlugs` | `"Settings / Profile"`, `"Settings: Profile"` → `settings-profile`, `settings-profile-2`; `""` → `artboard` | two artboards writing one file |
| `TestApprove_happy` (Hub fake) | blobs uploaded before the approval; `ApprovalRequest` has the right keys and digests; the review request is closed; route 22 then says `approved` | an approval with no snapshot |
| `TestApprove_notShared` | an unbound project → 409 `design_not_shared` and **no** Hub call | blobs uploaded for a project the Hub cannot hold |
| `TestApprove_incompleteBrief` | head brief `open` → 409 `design_no_brief`, no upload | an approval against an unfinished brief (ADR Q1) |
| `TestApprove_noRenderer` | no renderer → 409 `renderer_unavailable`, no upload | an approval whose PNGs are blank |
| `TestApprove_hubErrorsPassThrough` | fake answers `design_already_approved` → the same 409 and code reach the desktop; a fake 500 → 503 `upstream_unavailable` `upstream: hub` | Hub codes rewritten |
| `TestApprove_retryFreshReader` | a `PutBlob` failing once with `ErrUnavailable` then succeeding uploads the full bytes | a retry sending an empty body (HA5-1) |
| `TestApprove_noApproverField` | the marshalled request has no `approvedBy`, `approver` or `actor` | an actor asserted by the caller |
| `TestApprove_noMCPPath` | every route of `designapi` is on the guarded mux; no MCP tool name maps to route 21 (reflection over `mcp.RegisterAll`'s tools) | an agent reaching approval (ADR-H1 R5) |
| `TestApprovalState_changed` | approved at 5; a human colour edit at 6 → `changed`, `changedBy` names the human; a comment at 7 alone would not | *changed after approval* that fires on comments |
| `TestApprovalState_superseded` | fake `Superseded: true` → `superseded` | a stale brief shown as current |
| `TestReReview` | route 23 on a canvas approved at 11 against `@5`, head brief `@6` → revision 12; approving 12 sends `briefKey @6` and a digest ≠ 11's | re-review impossible (the Hub allows one approval per design key) |
| `TestNothingOnDisk` | after an approval, no file under the canvas store dir contains the approval id | ADR-H1 §5.4 broken |

## Steps

1. `canvas/handoff` tests, then code. 2. `designapi` tests against `hubclient.NewFake()` and a
fake `BriefSource`, then code. 3. Acceptance; commit by pathspec.

## Acceptance

```bash
cd /c/Users/au-dev-dmackie1/Projects/PanaceaApp/.claude/worktrees/phase-5 && go test ./canvas/handoff/... ./control-plane/internal/designapi/... -race -count=1
```
Expected: two `ok` lines.

## Not claimed

- **No real Hub is called**: every test runs against `hubclient.NewFake()` (Hub contract §9.2).
  Integration against `compose.yaml` is task 30, when Hub H4 has landed.
- The approver's identity is the Hub's principal; the control plane never asserts it.
- A teammate sees an approval only through the Hub; there is no live co-editing in v1 (OQ-P5-4).
