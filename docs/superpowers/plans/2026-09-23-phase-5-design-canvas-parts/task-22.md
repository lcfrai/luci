# Task 22 — Handoff: the bundle into the worktree, and the session link

**Plan:** `../2026-09-23-phase-5-design-canvas.md` · **Depends on:** 21 · **External:** **Hub C3 — Hub plan tasks 37–38 (`localkey`), a hard dependency** (route 24 writes into a worktree and route 25 writes to the Hub as the signed-in user; without the key a sandbox can call both, review C1); Hub H4 (fake until then); phase 3's 3-D6 exclude rule · **Hours:** 2

## Why

The thread carried into Develop (spec §7.6, §9.1 item 11). The approved snapshot is read **from the
Hub**, not from the canvas — so a handoff works while the canvas has moved on, and what the Develop
agent reads is byte-for-byte what was approved (each file checked against the manifest's SHA-256).
It lands in `<worktree>/.panacea/design/<canvasId>@<rev>/` (P5-G), excluded from git (phase 3's
3-D6: the handoff bundle is never committed). A session started from the design is then linked
`session → design:` in the Hub (contract §7.8), while its brief is inherited like any other
session's (phase 4's P4-A, seam S4-P5.2).

**Why writing Hub data into the worktree is allowed here.** ADR-H1 §5.4 forbids *persisting Hub
data on the machine* as a cache. The bundle is not a cache: it is an explicit render into the
worktree, at the person's request, for an agent to read — the same category as phase 4's managed
`CLAUDE.md` block (ADR-H1 §1.3). The control plane **never reads it back** and never renders from it
(P4), and it holds no brief text (there is no brief mirror), so a brief erasure is unaffected.

## Files

- `canvas/handoff/bundle.go` + test (new) — the write, pure over an `FS`
- `control-plane/internal/designapi/handoff.go`, `links.go`, `gitexclude.go` + tests (new); `routes.go` gains three routes

## Contracts

```go
// canvas/handoff/bundle.go

// BundleDir is the bundle's directory relative to the worktree root (P5-G).
func BundleDir(canvasID string, revision int) string // ".panacea/design/<canvasID>@<revision>"

// ApprovalRecord is approval.json in the bundle: the Hub's record, as the
// Develop agent needs it. Written by Materialise, never read back.
type ApprovalRecord struct {
	ApprovalID   string `json:"approvalId"`
	ApprovedAt   string `json:"approvedAt"`
	ApprovedBy   string `json:"approvedBy"` // display name
	BriefKey     string `json:"briefKey"`
	DesignKey    string `json:"designKey"`
	ReviewDigest string `json:"reviewDigest"`
}

// Fetch reads one blob; the control plane passes hubclient.Designs.GetBlob.
type Fetch func(digest string) ([]byte, error)

// Materialise writes the bundle. The Hub is shared across a team (OD-3) and
// checks nothing inside a manifest (Hub contract §7.7, hub-contract.md:903), so a manifest
// is hostile input until every check below has passed (review C2). In order:
//
//  1. Fetch the manifest by its digest; ≤ MaxFileBytes; its SHA-256 is the
//     digest; it decodes strictly (unknown fields refused) as task 21's
//     Manifest with Kind "design-handoff" and V 1.
//  2. It names THIS approval: manifest.DesignKey == rec.DesignKey, and
//     manifest.CanvasID and manifest.DesignRevision are the canvas id and
//     revision rec.DesignKey carries (doc.DesignKey's form, task 21);
//     manifest.ReviewDigest == rec.ReviewDigest; manifest.BriefKey ==
//     rec.BriefKey. BundleDir is built from rec (the Hub's record), never from
//     the manifest. Any mismatch → ErrTampered.
//  3. Every Files[].Path passes handoff.SafePath (task 21's paths.go); no path
//     appears twice; every ManifestArtboard HTML/PNG/JSX/TailwindJSX path is one
//     of Files; ContentType == ContentTypeFor(Path); an "assets/<hex>.<ext>"
//     path's hex is that file's own SHA-256 (assets are content-addressed,
//     task 05). Any failure → ErrUnsafePath.
//  4. len(Files)+1 ≤ MaxFiles (500); every Size ≤ MaxFileBytes; the sum of
//     Size, manifest included, ≤ MaxSnapshotBytes (512 MiB) → else ErrTooLarge.
//     Steps 1–4 run BEFORE any file is fetched or anything is created on disk.
//  5. Fetch every file; its length is its declared Size and its SHA-256 its
//     declared digest, else ErrTampered.
//  6. Write each into a temporary sibling directory tmp at
//     filepath.Join(tmp, filepath.FromSlash(p)), after re-checking that
//     filepath.Rel(tmp, that) neither is absolute nor starts with ".." (the
//     same check SafePath makes, repeated at the write site); write
//     manifest.json and approval.json; then rename tmp into BundleDir —
//     replacing an existing bundle only if its manifest differs.
//
// Every error writes nothing: tmp is removed and the worktree is as before.
func Materialise(fs FS, worktree string, manifestDigest string, fetch Fetch, rec ApprovalRecord) (Result, error)

type Result struct {
	Dir     string   `json:"dir"`     // BundleDir, "/"-separated
	Files   []string `json:"files"`   // relative to Dir, sorted, approval.json and manifest.json included
	Created bool     `json:"created"` // false when an identical bundle was already there
}

type FS interface { // os in production; a temp dir in tests; phase 8 supplies one for a pod (seam S4-P8.1's shape)
	MkdirAll(path string, perm fs.FileMode) error
	WriteFile(path string, data []byte, perm fs.FileMode) error
	ReadFile(path string) ([]byte, error)
	Rename(from, to string) error
	RemoveAll(path string) error
}

var ErrTampered = errors.New("handoff: a snapshot file does not match its digest")
// ErrUnsafePath and ErrTooLarge are task 21's (paths.go, snapshot.go); Materialise
// returns them wrapped, naming the offending path or limit. The route answers all
// three as 422 handoff_snapshot_invalid with details.reason
// ("tampered" | "unsafe_path" | "too_large") and writes nothing.
```

### Routes (added to task 20's table)

| # | Method | Path | Request | `2xx` response |
|---|---|---|---|---|
| 24 | POST | `/v1/design/canvases/{canvasId}/handoffs` | `{"approvalId":"ap-…","worktreeId":"w-…"}` | `201 handoff.Result`, or `200` when unchanged |
| 25 | POST | `/v1/design/canvases/{canvasId}/session-links` | `{"approvalId":"ap-…","sessionId":"s-…"}` | `201 hubapi.SessionLink`, or `200` for a repeat (the Hub's rule, contract §7.8) |
| 26 | GET | `/v1/design/canvases/{canvasId}/session-links` | — | `200 {"items":[hubapi.SessionLink]}` (route 26 is read-only: `catalogReadOnlyRoutes`) |

**Handoff, in order:** the approval by id from the Hub, and its `canvasId` must be this canvas
(else `404 approval_not_found` — the Hub's code); the worktree resolves (`worktree_not_found`) and is
**on the canvas's branch** (else `409 conflict`: *"that worktree is on another branch"*);
`git -C <worktree> ls-files -- .panacea` is empty (else `409 handoff_path_tracked`); then
`Materialise`, whose `Fetch` reads each blob through `io.LimitReader(body, MaxFileBytes+1)` so a Hub
answer longer than any legal blob is refused unread (`ErrTooLarge`), and whose `rec` is the Hub's
approval record just fetched — never a field of the request body; any of its three errors is
`422 handoff_snapshot_invalid` and nothing is written; then, if `/.panacea/` is not already a line of the clone's exclude file (found with
`git -C <worktree> rev-parse --git-path info/exclude`, which works for a clone and a linked
worktree), it is appended. A Hub failure mid-way writes nothing (the temporary directory is removed).

**Session link, in order:** the approval; the session from the registry, on the canvas's branch;
its `ContextRef` — the key phase 4 stamped (P6) — must be a brief key, not `""` or `none:declined`
(else `409 design_no_brief`: *"this session started without the brief, so it cannot be linked to
an approved design"*); then `LinkSession{designKey, briefKey: ContextRef, sessionId, branchRef}`.
The link records **both** keys; they may differ, and that difference is the point
(`prototype-fact-sheet.md:92`).

### The desktop's three calls (P5-N), for task 28

1. `POST …/handoffs {approvalId, worktreeId}`
2. `POST /v1/sessions {worktreeId, harness: "claude-code", openingPrompt}` — the existing route; no
   `contextRef` is sent (phase 4's P4-A ignores one)
3. `POST …/session-links {approvalId, sessionId}` — retried from the lens's *Link* action if it
   fails after 2 succeeded

## Tests first

| Test | Asserts | Kills the mutant |
|---|---|---|
| `TestMaterialise_verifies` | a fake fetch returning one altered file → `ErrTampered`, nothing under the worktree | a tampered snapshot handed to an agent |
| `TestMaterialise_hostilePaths` (table; each row a manifest with **correct** digests for every file, one `Files[].Path` replaced) | `../x`, `/x`, `C:\x`, `a\..\..\x`, `assets/../../x`, `CON.html`, `con.html`, `x.html:s`, `../../../../AppData/Roaming/Microsoft/Windows/Start Menu/Programs/Startup/x.cmd` → `errors.Is(err, ErrUnsafePath)`; **nothing written anywhere**: the worktree's parent directory (a `t.TempDir()` holding the worktree) lists exactly what it did before, no tmp sibling survives, and the fake `Fetch` was called for the manifest only | a teammate's manifest writing outside the worktree — code run at the victim's next sign-in (review C2) |
| `TestMaterialise_manifestLies` | a duplicate path; an artboard `HTML` path not in `Files`; `tokens.css` labelled `text/html`; an `assets/<hex>.png` whose bytes hash to another hex → `ErrUnsafePath`, nothing written | a second file overwriting the first; a path smuggled through the artboard table |
| `TestMaterialise_otherDesign` | a valid manifest whose `canvasId`, `designRevision`, `designKey`, `reviewDigest` or `briefKey` differs from the approval record → `ErrTampered`, nothing written | one approval materialising another design's files |
| `TestMaterialise_limits` | 500 files + the manifest → `ErrTooLarge`; sizes summing to `MaxSnapshotBytes` + 1 → `ErrTooLarge`, **before** any file fetch; a fetched blob longer than its declared `Size` → `ErrTampered` | a manifest that fills the disk |
| `TestHandoff_invalidSnapshot` | the route over a Hub fake holding the `../x` manifest → 422 `handoff_snapshot_invalid`, `details.reason: "unsafe_path"`; `info/exclude` untouched | the error flattened to 500, or a half-done handoff |
| `TestHandoff_localKeyRequired` | routes 24 and 25 with no `Authorization`, no `Origin` and `Host: 127.0.0.1:8730` → 401 `local_key_required`; no `Fetch`, no file, no `LinkSession` call (task 20's harness) | a sandbox materialising files into a worktree or linking a session as the human (review C1) |
| `TestMaterialise_idempotent` | twice with one manifest → the second `Created: false`, files untouched (mtime) | a rewrite on every click |
| `TestMaterialise_replacesDifferent` | a directory at `BundleDir` whose `manifest.json` differs (a hand edit, a crash leftover) is replaced whole by the rename; no stray file from it survives | half-old, half-new bundles |
| `TestMaterialise_approvalJSON` | `approval.json` holds the record's six fields | the Develop agent unable to name what it implements |
| `TestHandoff_wrongBranch` | a worktree of another branch → 409 | a design handed to the wrong branch |
| `TestHandoff_tracked` | a fixture repo with `.panacea/x` committed → 409 `handoff_path_tracked` | the bundle overwriting tracked files |
| `TestHandoff_excludeOnce` | two handoffs → `/.panacea/` appears once in `info/exclude`; `git status --porcelain` shows nothing under `.panacea` | the bundle committed (3-D6) |
| `TestHandoff_hubDown` | fake `GetBlob` → `ErrUnavailable` mid-way → 503 `upstream_unavailable`, no directory left | a partial bundle |
| `TestLink_usesStampedKey` | the Hub fake receives the session's `ContextRef`, not the approval's `briefKey` | the link recording the wrong brief |
| `TestLink_declined` | `ContextRef: "none:declined"` → 409 `design_no_brief`, no Hub call | a link with an unresolvable key (a 422 from the Hub) |
| `TestLink_repeat` | the same link twice → 201 then 200 | duplicate links |

## Steps

1. `bundle.go` tests with an in-memory `FS`, then code. 2. `designapi` tests with a real temp git
repository (the `project` package's tests show the pattern) and the Hub fake. 3. Acceptance; commit.

## Acceptance

```bash
cd /c/Users/au-dev-dmackie1/Projects/PanaceaApp/.claude/worktrees/phase-5 && go test ./canvas/handoff/... ./control-plane/internal/designapi/... -race -count=1
```
Expected: two `ok` lines.

## Not claimed

- Worktree creation from a design is phase 3's (S3-P5.1, seam S5-P3.1); this task writes into a
  worktree that exists.
- Whether a Develop agent builds the screen well from the bundle is task 30's observation.
