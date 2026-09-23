# Task 21 — Approval: the snapshot, the blobs, the Hub's design link, *changed after approval*, re-review

**Plan:** `../2026-09-23-phase-5-design-canvas.md` · **Depends on:** 08, 09, 13, 20a, 20b (review I6: 20b's `design.go` gains this task's `Hub` line) · **External:** **Hub C3 — Hub plan tasks 37–38 (`localkey`), a hard dependency** (route 21 is the human-only approval; without the key a sandbox can call it, review C1), Hub H0 (`hubapi`, `hubclient.Designs`, `NewFake`), Hub C2 (the relay rule), **Hub C1 — Hub plan task 32** (`api.Deps.Hub hubclient.Hub`, `hub/.../task-32.md:58`, and `CodeHubNotConfigured`, `:28`; the `design.go` line below passes that value and route 21 answers that code, re-review RR-I6-2); phase 4 task 07 (`discoverhub`, through task 20a's port) · **Hours:** 3.5

## Why

The spine (`CLAUDE.md`): the brief written in Define is visibly referenced in Design and carried
into Develop. Approval is where Design joins the thread. **Only a human approves, from our chrome**
(plan rule 4; ADR-H1 R5), over a SHA-256 digest of the design's content (task 08), against a
**complete** brief revision (ADR Q1). One Hub call writes the approval, the design link
`design:<canvasId>@<rev>` → `brief:<id>@<rev>` and the snapshot together (Hub contract §7.7), after
the snapshot's files are uploaded as blobs (§7.6). So the approved design survives the canvas —
brief R8 / P-g closed — and a release's design key always resolves.

## Files

- `canvas/handoff/snapshot.go`, `manifest.go`, `slug.go`, `paths.go` + tests (new) — `paths.go` is the
  one bundle-path and manifest rule (`SafePath`, `CheckManifest`) that `BuildSnapshot` runs on its
  own output and task 22's `Materialise` runs on the Hub's (review C2, re-review I-1)
- `control-plane/internal/designapi/approve.go`, `approvals.go` (the in-memory `Approvals` port),
  `hub.go` (the `Designs` port and the relay), `approve_test.go` (new); `routes.go` gains three routes
- `control-plane/internal/designapi/mount.go` — `Config` gains `Hub Designs`; `(*API).Approvals()`
  returns the in-memory port; `approvals_none.go` (task 20a's stub) is deleted
- `control-plane/cmd/api/design.go` — one statement (the nil rule below), and `design_test.go` one test (`TestNewDesignAPI_noHubURL`): `Hub:` is Hub task 32's `hubclient` value (nil when
  `PANACEA_APP_HUB_URL` is unset; route 21, and route 22 on a cache miss, then answer the Hub plan's `503 hub_not_configured`,
  `hub/.../task-32.md:28`, never a nil-pointer panic: `TestApprove_noHub`); the MCP
  wiring is unchanged because task 20b passes `designAPI.Approvals()` (review I6).
  **The nil rule (re-review RR-I6-3).** The line is `if hc != nil { cfg.Hub = hc }`, where `hc`
  is the `hubclient.Hub` interface value from `api.Deps.Hub` (`task-32.md:58`), never a
  `*hubclient.Client` (`hubclient.New` returns one, `hub/.../task-04.md:173`): a nil pointer
  stored in an interface is a non-nil interface, so `c.Hub == nil` would be false and route 21
  would call through it. `designapi.New` also refuses a typed nil: `if c.Hub != nil &&
  reflect.ValueOf(c.Hub).Kind() == reflect.Pointer && reflect.ValueOf(c.Hub).IsNil()` →
  `ErrTypedNilHub`. `TestNewDesignAPI_noHubURL` (task 20b's `design_test.go`, extended here) runs
  `newDesignAPI` with `PANACEA_APP_HUB_URL` unset and calls route 21 through the returned mount:
  `503 hub_not_configured`, no panic

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
// A dialect.ErrUnsafeToken from ExportHTML or tokens.CSS, or a
// dialect.ErrUnsafeNode from ExportHTML (reachable only through a document
// replayed from disk, task 08), fails the whole bundle, wrapped with the token
// or node it names; the approval that asked for it is refused as
// `422 design_unsafe` (task 20) with details.token or details.nodeId, never
// blamed on the renderer (re-review RR-M5).
func BuildSnapshot(in SnapshotInput) (Snapshot, error)

// FileCount is len(Files)+1 of the snapshot BuildSnapshot would write for doc,
// computed without any PNG: 4 per artboard + 3 + one per distinct asset + the
// manifest. Route 21 checks it against MaxFiles before capturing anything
// (re-review RR-I5-2); TestFileCount_matchesBuild keeps the two from drifting.
func FileCount(d *doc.Doc) int

const (
	MaxFiles         = 500       // the Hub's snapshot.blobs cap, manifest included (contract §7.7 check 5)
	MaxFileBytes     = 16 << 20  // the Hub's blob cap (contract §7.6)
	MaxSnapshotBytes = 512 << 20 // every file's Size summed, manifest included; half the Hub's 1 GiB
	                             // project quota (contract §7.6). Both BuildSnapshot and task 22's
	                             // Materialise enforce it through CheckManifest (paths.go), so no
	                             // snapshot is built that a handoff would refuse (review C2, re-review I-1)
)
var ErrTooLarge = errors.New("handoff: snapshot over the Hub's limits")

// BuildSnapshot's last step, before it returns: CheckManifest(snap.Manifest,
// int64(len(snap.ManifestFile.Data)), ManifestWant{CanvasID: the canvas id
// in.DesignKey carries, DesignKey: in.DesignKey, BriefKey: in.BriefKey,
// ReviewDigest: snap.ReviewDigest}). It returns that error unchanged. So a
// snapshot the handoff would refuse is never returned, by construction.

// canvas/handoff/paths.go (review C2) — the ONE definition of a bundle path
// and of an acceptable manifest, shared by BuildSnapshot (which runs
// CheckManifest on its own output, so every Files path passes SafePath) and
// task 22's Materialise (which runs the same CheckManifest on the Hub's
// manifest and refuses whatever fails it).
//
// PathPattern is exactly the set of names BuildSnapshot produces, and nothing
// else: "/"-separated, relative, lowercase, no "..", no "\", no ":", no drive.
const PathPattern = `^(assets/[0-9a-f]{64}\.(png|jpg|webp|svg)` +
	`|[a-z0-9](?:[a-z0-9-]{0,58}[a-z0-9])?\.(html|png|jsx|tailwind\.jsx)` +
	`|tokens\.(css|json)|tree\.json)$`

// SafePath reports whether p may be written under a bundle root: it matches
// PathPattern; the part before its first "." is not a Windows device name
// (con, prn, aux, nul, com0–com9, lpt0–lpt9 — reserved with any extension); and
// filepath.Rel(root, filepath.Join(root, filepath.FromSlash(p))) neither is
// absolute nor starts with "..". The last check cannot fail for a name the
// pattern admits; it stays so a widened pattern cannot open an escape alone.
func SafePath(p string) error // nil, or an error wrapping ErrUnsafePath naming p

var ErrUnsafePath = errors.New("handoff: a snapshot path is not one BuildSnapshot writes")

// ContentTypeFor is the one media type BuildSnapshot gives a SafePath name
// (the table above: .html text/html; charset=utf-8, .png image/png, .jsx and
// .tailwind.jsx text/plain; charset=utf-8, tokens.css text/css; charset=utf-8,
// .json application/json, assets by extension). Materialise requires it.
func ContentTypeFor(p string) string

// ManifestWant is what a manifest must name. Task 22 fills it from the Hub's
// approval record and the route's {canvasId}; BuildSnapshot fills it from its
// own input.
type ManifestWant struct {
	CanvasID     string // the LOCAL canvas: the route's {canvasId}
	DesignKey    string
	BriefKey     string
	ReviewDigest string
}

// CheckManifest is the ONE statement of what a handoff accepts (re-review
// I-1). It reads the manifest only — no blob — so it runs before any file is
// fetched or anything is created on disk. In order:
//
//  1. m.Kind == "design-handoff" and m.V == 1.
//  2. doc.ParseDesignKey(want.DesignKey) (task 05) succeeds, and the canvas id
//     it yields == want.CanvasID; m.DesignKey == want.DesignKey; m.CanvasID and
//     m.DesignRevision are the canvas id and revision it yields;
//     m.ReviewDigest == want.ReviewDigest; m.BriefKey == want.BriefKey.
//     Steps 1–2: any failure → ErrTampered.
//  3. Every Files[].Path passes SafePath; no path appears twice; every
//     ManifestArtboard HTML/PNG/JSX/TailwindJSX path is one of Files, and no
//     two artboards share a Slug; ContentType == ContentTypeFor(Path); an
//     "assets/<hex>.<ext>" path's hex is that file's own declared SHA256
//     (assets are content-addressed, task 05). Any failure → ErrUnsafePath.
//  4. len(Files)+1 ≤ MaxFiles; every Size ≤ MaxFileBytes; the sum of every
//     Size plus manifestSize ≤ MaxSnapshotBytes. Any failure → ErrTooLarge.
//
// Each error wraps its sentinel and names the offending field, path or limit.
func CheckManifest(m Manifest, manifestSize int64, want ManifestWant) error

// ErrTampered is a manifest (or, in task 22, a fetched blob) that does not
// match the approval it is handed off under. Moved here from task 22 because
// CheckManifest returns it.
var ErrTampered = errors.New("handoff: a snapshot file does not match its digest")
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
// trimmed of "-"; cut to 60 bytes and trimmed of "-" again; "" becomes
// "artboard"; a Windows device name (con, prn, aux, nul, com0–com9,
// lpt0–lpt9) gets "-artboard" ("con" → "con-artboard"). Uniqueness is
// measured against the slugs ALREADY GIVEN OUT, not counted per base name
// (re-review I-1): walking in page-then-artboard order with one `taken` set,
// a base not yet taken is used as it is; otherwise the artboard gets the
// smallest n >= 2 for which "<base cut to 60-len("-n") bytes, trimmed of
// "-">-<n>" is not taken — whether the taken slug came from a name or from an
// earlier suffix. "Home", "Home", "Home 2" → home, home-2, home-2-2; "Home 2",
// "Home", "Home" → home-2, home, home-3. The result is at most 60 bytes, and
// every slug is distinct. Every slug therefore passes SafePath as
// "<slug>.html" (review C2) — the slug tests assert it for each rule.
func Slugs(artboards []doc.Node) map[string]string // artboard id → slug

// slugsFn is what BuildSnapshot calls (default Slugs). It is unexported and
// only TestBuildSnapshot_selfChecks swaps it, to prove the self-check fires.
var slugsFn = Slugs
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

### Routes (added to task 20a's table)

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
4. The document at `revision` (`authority.Document`). Then **the count half of step 5's limit,
   before any capture** (re-review RR-I5-2): `handoff.FileCount(doc)` — four files per artboard,
   `tokens.css`, `tokens.json`, `tree.json`, one per distinct asset the document references, and
   the manifest, which is exactly the list `BuildSnapshot` writes — over `MaxFiles` →
   `413 design_too_large` and no render request is sent. The byte limits stay in step 5, since the
   PNGs are most of the bytes. Then **2× screenshots of every artboard at
   that revision, one artboard per request** (review I5): for artboard *i* of *n*, in document
   order, `authority.RenderWithin(ctx, canvasId, {Kind: screenshot, NodeIDs: [id], Scale: 2,
   Revision: revision, Progress: {approval, i, n, name}}, ApprovalRenderTimeout)` (task 13), so each
   capture has its own deadline and the agents' 15 s `RenderTimeout` is untouched. The requests run
   one after another (a renderer captures on its main thread; parallel requests would share it).
   The log records each artboard's capture time (`artboard`, `ms`, `scale`), the figure task 30
   compares with S8.
   - **A detach is re-sent once** (re-review RR-I5-1). A capture lasts long enough for the
     desktop to replace its stream (its topic set changed: another branch gained a session, a
     canvas subscription was added, task 20) or for the person to leave the lens, and either
     detaches the renderer holding the request. So a request failing `ErrRendererDetached`
     (task 13) is re-sent **once**, with a fresh `ApprovalRenderTimeout`, to whichever renderer is
     preferred now: the same window's renderer on its new stream, or the hidden renderer the
     desktop keeps for a canvas under approval (task 27's `approving`). Only a detach is
     re-sent. A timeout or a `*RenderError` is the renderer's answer about that artboard, and
     re-sending it would double the wait for the same answer.
   - **Any other failure stops the loop**: `409 renderer_unavailable` with `details.artboardId`,
     `details.artboardName`, `details.index`, `details.total`, `details.cause`, and for a
     `*RenderError` also `details.renderError` (its code). The sentence names the cause the route
     knows, not a guess:

     | `cause` | when | sentence |
     |---|---|---|
     | `no_renderer` | `ErrNoRenderer` on artboard 1, before any detach | *"No canvas is open to capture this design. Open it in the Design lens and approve again. Nothing was approved."* |
     | `renderer_gone` | a second `ErrRendererDetached` on one artboard, or `ErrNoRenderer` after a detach or on artboard 2 onwards | *"The canvas closed while artboard "<name>" (<i> of <n>) was being captured. Keep the window open until capture ends. Nothing was approved."* |
     | `timeout` | `ErrRenderTimeout` | *"Artboard "<name>" (<i> of <n>) was not captured within <s> seconds. Nothing was approved."* (`<s>` from `ApprovalRenderTimeout`) |
     | `render_error` | a `*RenderError` | *"The renderer could not capture artboard "<name>" (<i> of <n>): <message>. Nothing was approved."* |

     Nothing is uploaded before every capture has succeeded (step 6 comes after), so a failure
     leaves the Hub untouched.
5. `handoff.BuildSnapshot` (called through the API's unexported `buildSnapshot` field, which only
   `TestApprove_snapshotBug` replaces); over `MaxFiles` (already refused in step 4, kept as the
   backstop), `MaxFileBytes` or `MaxSnapshotBytes` → `413 design_too_large`.
   `dialect.ErrUnsafeToken` or `dialect.ErrUnsafeNode` (the export re-check, task 08) →
   `422 design_unsafe` with `details.token` or `details.nodeId`, nothing uploaded, no approval
   asked of the Hub (re-review RR-M5).
   `ErrTampered` or `ErrUnsafePath` from `BuildSnapshot`'s own `CheckManifest` (a path failing
   `SafePath`, a path repeated, a content type or asset hex that disagrees) is a bug in `slug.go` or
   `snapshot.go` — `500 internal`, nothing uploaded, no approval asked of the Hub (re-review I-1).
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
| `TestBuildSnapshot_limits` | 501 files → `ErrTooLarge`; files summing past `MaxSnapshotBytes` → `ErrTooLarge` | a 422 from the Hub after 500 uploads; a snapshot task 22 would refuse to write |
| `TestBuildSnapshot_pathsSafe` | a fixture whose artboards are named `"../../x"`, `"C:\\x"`, `"CON"`, `"a:b"`, `"Ünïcode"` and a 200-character name: every `File.Path` and every `ManifestArtboard` path passes `SafePath`, and `ContentTypeFor(path) == File.ContentType` | BuildSnapshot writing a name Materialise refuses (review C2) |
| `TestSafePath` | accepts each name the contract table lists; refuses `../x`, `/x`, `C:\x`, `a\..\..\x`, `assets/../../x`, `CON.html`, `con.html`, `lpt1.png`, `x.html:s`, `x.HTML`, `-x.html`, `assets/abc.png` (short hex), `manifest.json`, `approval.json`, `""` → each wraps `ErrUnsafePath` | a hostile manifest path accepted (review C2) |
| `TestSlugs` | `"Settings / Profile"`, `"Settings: Profile"` → `settings-profile`, `settings-profile-2`; `""` → `artboard`; `"Con"` → `con-artboard`; a 70-character name repeated → two slugs of at most 60 bytes, the second ending `-2`; `"Home"`, `"Home"`, `"Home 2"` → `home`, `home-2`, `home-2-2`, and `"Home 2"`, `"Home"`, `"Home"` → `home-2`, `home`, `home-3` (re-review I-1); every case asserts the slugs are pairwise distinct | two artboards writing one file; a per-base-name counter (it gives `home-2` twice); a device name or an over-long slug written to disk |
| `TestCheckManifest` (table over one valid manifest, one field changed per row) | the valid manifest → nil; `kind` ≠ `design-handoff`, `v: 2`, a `designKey` that fails `doc.ParseDesignKey`, a `want.CanvasID` other than the key's canvas id, and a changed `canvasId`, `designRevision`, `designKey`, `reviewDigest` or `briefKey` → `ErrTampered`; a duplicate `Files` path, an artboard path not in `Files`, two artboards sharing a slug, `tokens.css` labelled `text/html`, an asset hex ≠ its `sha256` → `ErrUnsafePath`; 500 files + the manifest, one `Size` of `MaxFileBytes`+1, sizes plus `manifestSize` summing to `MaxSnapshotBytes`+1 → `ErrTooLarge` | the two manifest rules — BuildSnapshot's and Materialise's — drifting apart (re-review I-1) |
| `TestBuildSnapshot_selfChecks` | a fixture with artboards named `"Home"`, `"Home"`, `"Home 2"` builds, and `CheckManifest` on its output is nil; with `Slugs` replaced by a per-base-name counter (a test seam, `slugsFn`) `BuildSnapshot` returns an error wrapping `ErrUnsafePath` and no `Snapshot` | BuildSnapshot skipping its own `CheckManifest`: an approval the handoff can never materialise (re-review I-1) |
| `TestApprove_snapshotBug` | the route with its `buildSnapshot` field (default `handoff.BuildSnapshot`) replaced by one returning an error wrapping `handoff.ErrUnsafePath`, then one wrapping `handoff.ErrTampered` → `500 internal` each time; the Hub fake received no `PutBlob` and no `Approve` | an immutable Hub approval that no handoff can use |
| `TestMount_wrapsEveryRoute` (task 20a's test, extended) | the count of mounted design routes is now **23** (routes 21–23 added); each answers 401 `local_key_required` with no key | a new route mounted around the guard (re-review M-3) |
| `TestApprove_happy` (Hub fake) | blobs uploaded before the approval; `ApprovalRequest` has the right keys and digests; the review request is closed; route 22 then says `approved` | an approval with no snapshot |
| `TestApprove_notShared` | an unbound project → 409 `design_not_shared` and **no** Hub call | blobs uploaded for a project the Hub cannot hold |
| `TestApprove_incompleteBrief` | head brief `open` → 409 `design_no_brief`, no upload | an approval against an unfinished brief (ADR Q1) |
| `TestApprove_noRenderer` | no renderer → 409 `renderer_unavailable`, `details.cause: "no_renderer"` and its sentence, no upload | an approval whose PNGs are blank |
| `TestApprove_rendererSwapMidApproval` (re-review RR-I5-1; a recording renderer fake, 12 artboards, a visible and a hidden renderer attached) | the visible renderer detaches while holding artboard 4 → artboard 4 is re-sent once, with a fresh `ApprovalRenderTimeout`, to the hidden renderer, which answers; artboards 5–12 go to the hidden one → 201, twelve PNGs; the same with the visible renderer re-attached under its own id (a stream replacement) instead of a hidden one → 201, artboard 4 captured by the re-attached renderer | a stream swap or a lens switch failing the spine's one action (re-review RR-I5-1) |
| `TestApprove_rendererGoneMidApproval` (re-review RR-I5-1) | the only renderer detaches holding artboard 4 of 12 → `409 renderer_unavailable`, `details.cause: "renderer_gone"`, index 4, the *"The canvas closed while …"* sentence; artboard 4 re-sent to a renderer that detaches again → the same; a `*RenderError` or a timeout on artboard 4 → exactly one request for artboard 4 (no re-send) and cause `render_error` / `timeout`; no `PutBlob`, no `Approve` in any row | an error blaming the renderer for a closed window; a re-send loop; a timeout re-sent, doubling the wait |
| `TestApprove_unsafeDocument` (re-review RR-M5) | a store fixture replayed with a permissive `Checker` whose artboard uses a `font-family` token holding the review I2 script value → `422 design_unsafe` with `details.token: "brand"`; the same with a frame declaration `margin: 0` → `details.nodeId`; in both, no `PutBlob` and no Hub approval call, and the code is not `renderer_unavailable` | a dialect failure at export blamed on the renderer, or an unsafe bundle uploaded |
| `TestApprove_tooManyFilesBeforeCapture` (re-review RR-I5-2) | a document whose `FileCount` is `MaxFiles` + 1 → `413 design_too_large`, and the renderer fake saw **no** request, no `PutBlob`; at exactly `MaxFiles` it proceeds to capture | a canvas over the cap captured artboard by artboard before it is refused |
| `TestFileCount_matchesBuild` | for the two-artboard fixture, a fixture sharing one asset between artboards, and the `"Home"`, `"Home"`, `"Home 2"` fixture, `FileCount(doc) == len(BuildSnapshot(…).Files) + 1` | the pre-check and the build counting different files |
| `TestApprove_onePerArtboard` (a recording renderer fake, 12 artboards) | twelve requests, one after another (the next is sent only after the last `CompleteRender`), each with one node id, `Scale: 2`, `Revision` the approved one, `Progress{approval, i, 12, name}` in document order, and a `Deadline` of now + `ApprovalRenderTimeout` (injected clock) | the old one-request-for-every-artboard capture held to 15 s; parallel captures fighting over one main thread (review I5) |
| `TestApprove_slowArtboardWithinItsDeadline` (injected clock) | each artboard answered at `RenderTimeout + 5 s` → 201; the same with `Render`'s `RenderTimeout` budget would have failed (asserted by a row that calls `Render` directly) | the approval routed through `Render`, not `RenderWithin` |
| `TestApprove_failureNamesArtboard` | artboard 3 of 5 (*"Billing"*) unanswered → at `ApprovalRenderTimeout` (the constant, never a literal, so S8 resetting it leaves the test true: re-review RR-I5-3), `409 renderer_unavailable` with `details.artboardId`, `artboardName: "Billing"`, `index: 3`, `total: 5`, `cause: "timeout"`, the timeout sentence naming it with `ApprovalRenderTimeout`'s seconds; artboards 4–5 never requested; **no** `PutBlob`, **no** `Approve`; a `*RenderError{too-large}` on artboard 2 → the same shape with `cause: "render_error"` and `details.renderError: "too-large"` | a failure the human cannot act on; blobs uploaded for an approval that failed |
| `TestApprove_hubErrorsPassThrough` | fake answers `design_already_approved` → the same 409 and code reach the desktop; a fake 500 → 503 `upstream_unavailable` `upstream: hub` | Hub codes rewritten |
| `TestApprove_noHub` (review I6) | `Config.Hub` nil (the process with `PANACEA_APP_HUB_URL` unset): route 21, and route 22 on a cache miss, → `503 hub_not_configured`, no panic, no blob or approval, authority unchanged; `Approvals().Latest` → not found | a nil `Hub` dereferenced on the one action the spine depends on |
| `TestNew_refusesTypedNilHub` (re-review RR-I6-3) | `New(Config{…, Hub: (*fakeDesigns)(nil)})` → `ErrTypedNilHub`, no `*API`; an untyped nil `Hub` → builds | a nil pointer inside a non-nil interface reaching route 21 |
| `TestNewDesignAPI_noHubURL` (`cmd/api/design_test.go`, re-review RR-I6-3) | the real `newDesignAPI` with `PANACEA_APP_HUB_URL` unset (so `api.Deps.Hub` is nil, Hub task 32): route 21 through the returned mount, right local key → `503 hub_not_configured`, no panic | `design.go` assigning a typed nil `*hubclient.Client`, which `TestApprove_noHub`'s untyped nil cannot see |
| `TestApprove_retryFreshReader` | a `PutBlob` failing once with `ErrUnavailable` then succeeding uploads the full bytes | a retry sending an empty body (HA5-1) |
| `TestApprove_noApproverField` | the marshalled request has no `approvedBy`, `approver` or `actor` | an actor asserted by the caller |
| `TestApprove_noMCPPath` | every route of `designapi` is on the guarded mux; no MCP tool name maps to route 21 (reflection over `mcp.RegisterAll`'s tools) | an agent reaching approval over MCP (ADR-H1 R5) |
| `TestApprove_localKeyRequired` | through `Mount` with a real `localkey.Middleware` (task 20a's harness): route 21 with no `Authorization`, no `Origin` and `Host: 127.0.0.1:8730` → 401 `local_key_required`, and the Hub fake received **no** `PutBlob` and **no** `Approve`; a wrong key → the same; the right key → 201 | an agent approving its own design over REST from inside its sandbox (review C1, ADR-H1 R5) |
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
