# Task 08 — Canonical HTML, export HTML, canonical JSON, the review digest

**Plan:** `../2026-09-23-phase-5-design-canvas.md` · **Depends on:** 05, 07 · **Hours:** 2.5

## Why

Three consumers need the same subtree as text, and each needs it byte-stable:

- **`get_html`** (task 15) — an agent reads back exactly what it wrote, in the dialect, and can
  send it back through `write_html` (round trip).
- **The review digest** (spec §7.6) — SHA-256 over the canonical serialisation of the approved
  artboards, their assets, the tokens they use, and the brief key reviewed against. *Changed after
  approval* is "the digest at head differs from the approved digest", so the digest must be a pure
  function of **content**: no node ids, no revision, no author, no lock flag.
- **The handoff bundle** (task 21) — a standalone `.html` file per artboard that opens in a browser.

## Files

- `canvas/serialize/html.go`, `export.go`, `canonjson.go`, `digest.go` (new)
- `canvas/serialize/*_test.go`, `testdata/` (new)

## Contracts

```go
// canvas/serialize/html.go
package serialize

type HTMLOptions struct {
	IDs    bool // emit data-node="n-…" on every element (get_html's default); the digest uses false
	Pretty bool // two-space indentation and a newline per element; the digest and export use false
}

// HTML serialises the subtree rooted at nodeID in the dialect. Rules:
//   - one element per node: its Tag; void tags (img, input) have no close tag and no "/";
//   - attributes in this order: data-node (if IDs), name (if non-empty), aria-label, the
//     tag's kept attributes sorted by name (alt, checked, placeholder, type, value), hidden
//     (a boolean attribute, when Node.Hidden), style (last, if any);
//   - style is "prop: value" pairs sorted by property name, joined by "; ", no trailing ";";
//   - text: & < > escaped as &amp; &lt; &gt;, "\n" as <br>; attribute values escape & < > " ;
//   - image: src="<asset ref>" and alt; svg: Node.SVG verbatim;
//   - Locked, Revision and EditedBy are never emitted;
//   - an <a> is emitted with href="#" as its first attribute after data-node.
// The output parses back through dialect.Parse to the same nodes (ids aside) — the round trip.
func HTML(d *doc.Doc, nodeID string, o HTMLOptions) (string, error)
```

```go
// canvas/serialize/export.go

// ExportHTML is one artboard as a standalone document for the handoff bundle:
//
//   <!doctype html>
//   <html lang="en"><head><meta charset="utf-8">
//   <meta name="viewport" content="width=<artboard px width>">
//   <title><artboard name, escaped></title>
//   <style>:root{<every token the artboard uses, "--name: value;" sorted>}html,body{margin:0}</style>
//   </head><body><HTML(artboard, IDs:false) with each asset ref rewritten></body></html>
//
// assetPath maps an asset ref to the relative path the bundle writes it at
// (task 21: "assets/<hex>.<ext>"). No script, no external URL, ever.
func ExportHTML(d *doc.Doc, artboardID string, assetPath func(ref string) string) (string, error)

// TokensUsed is every token referenced by var(--x) in the subtree, plus the
// tokens those tokens' values reference, sorted by name. Unknown names are skipped.
func TokensUsed(d *doc.Doc, nodeID string) []doc.Token

// AssetsUsed is every asset ref in the subtree (img src and url() values), sorted.
func AssetsUsed(d *doc.Doc, nodeID string) []string
```

```go
// canvas/serialize/canonjson.go

// CanonicalJSON is the canvas's own canonical encoding (plan P5-U):
// encoding/json with SetEscapeHTML(false), the trailing newline removed.
// It is canonical only for the envelope types below, whose fields are
// declared in lexicographic order of their JSON names and which hold no
// floats; TestEnvelopeFieldOrder enforces both by reflection.
func CanonicalJSON(v any) ([]byte, error)
```

```go
// canvas/serialize/digest.go

// ReviewEnvelope is what an approval's reviewDigest covers. Fields are in
// lexicographic JSON order (canonjson.go).
type ReviewEnvelope struct {
	Artboards []ReviewArtboard `json:"artboards"` // every artboard (plan P5-H): by page order, then artboard order
	Assets    []string         `json:"assets"`    // AssetsUsed over all artboards, sorted, de-duplicated
	BriefKey  string           `json:"briefKey"`  // the brief key reviewed against, verbatim (never parsed, G9)
	CanvasID  string           `json:"canvasId"`
	Kind      string           `json:"kind"`      // "design-review"
	Tokens    []doc.Token      `json:"tokens"`    // TokensUsed over all artboards, sorted by name, de-duplicated
	V         int              `json:"v"`         // 1
}

type ReviewArtboard struct {
	HTML string `json:"html"` // HTML(artboard, IDs:false, Pretty:false)
	ID   string `json:"id"`   // the artboard's node id: an artboard replaced by an identical one is a change
	Name string `json:"name"`
	Page string `json:"page"` // the page's name
}

// ReviewDigest returns "sha256:" + lowercase hex of SHA-256(CanonicalJSON(env)).
// An asset's bytes are covered through its ref, which is their SHA-256.
func ReviewDigest(d *doc.Doc, briefKey string) (digest string, env ReviewEnvelope, err error)
```

### Golden vectors — checked into `digest_test.go`

The document: canvas `cv-00000000000000c1`; page `Page 1`; artboard `n-0000000000a1` named `Card`
with `width: 390px; height: fit-content`; one child, a `p` text node `Hello` with
`color: var(--brand)`; token `brand` (color) `#2fb3bd`. Computed on 2026-09-24 with `sha256sum` and
again with Python's `hashlib`; both agree.

**Vector 1** — brief key `brief:bf-00000000000000a1@5`, 366 bytes:

```
{"artboards":[{"html":"<div name=\"Card\" style=\"height: fit-content; width: 390px\"><p style=\"color: var(--brand)\">Hello</p></div>","id":"n-0000000000a1","name":"Card","page":"Page 1"}],"assets":[],"briefKey":"brief:bf-00000000000000a1@5","canvasId":"cv-00000000000000c1","kind":"design-review","tokens":[{"name":"brand","type":"color","value":"#2fb3bd"}],"v":1}
sha256:d369645afcdb2b5fef7b1684ac803c0c1d167ff25a9bb6c6ef9234d5bb3ca376
```

**Vector 2** — the same document, brief key `brief:bf-00000000000000a1@6` (the re-review of brief
D3), 366 bytes:

```
sha256:8208d6b88abd5a17f75475bc729f9abb23f7efa9fc2bf0fff66fe0833186cc9b
```

A Go implementation passes when `CanonicalJSON(env)` reproduces vector 1's bytes exactly and both
digests match.

## Tests first

| Test | Asserts | Kills the mutant |
|---|---|---|
| `TestReviewDigest_vectors` | both vectors, byte for byte and digest for digest | `SetEscapeHTML(true)` (`<` in place of `<`); a trailing newline |
| `TestReviewDigest_contentOnly` | a `set_props{locked:true}`, an undo-then-redo, and a comment each leave the digest unchanged; a colour change moves it | ids, revisions or authors in the envelope (would make every comment *"changed after approval"*) |
| `TestReviewDigest_tokenValue` | changing `brand`'s value moves the digest though no node changed | tokens missing from the envelope |
| `TestEnvelopeFieldOrder` | by reflection, every field of `ReviewEnvelope`, `ReviewArtboard` and `doc.Token` is in lexicographic JSON-name order, and none is a float | a field added out of order, silently changing canonical bytes |
| `TestHTML_attributeOrder` | a node with every attribute kind emits them in the stated order | map iteration order leaking into output |
| `TestHTML_roundTrip` (property test over 500 random documents) | `dialect.Parse(HTML(n, IDs:false))` yields nodes whose kinds, tags, names, styles, text, assets and order equal the source's | a serialiser the parser cannot read back |
| `TestHTML_escapes` | text `a < b & "c"` and a name with `"` serialise escaped and round-trip | unescaped text (a script vector on re-parse) |
| `TestHTML_newlines` | text `"a\nb"` emits `a<br>b` | `\n` emitted raw (collapsed by the browser) |
| `TestExportHTML_standalone` | the output has no `http`, no `src=` outside `assets/`, no `<script`, and one `:root` block with exactly the tokens used | an export that references the canvas |
| `TestTokensUsed_transitive` | `--a: var(--b)` used by a node yields `a` and `b` | nested tokens dropped from export |

## Steps

1. Write the tests, with the two vectors as literals. 2. Implement the four files. 3. Acceptance;
commit by pathspec.

## Acceptance

```bash
cd /c/Users/au-dev-dmackie1/Projects/PanaceaApp/.claude/worktrees/phase-5 && go test ./canvas/serialize/... -race -count=1
```
Expected: `ok  	github.com/kpmg-au-emu/panacea-app/canvas/serialize`.

## Not claimed

- The digest does not cover pixels. Two renderers that paint the same HTML differently produce the
  same digest; the PNG in the snapshot (task 21) is what the approver saw, recorded, not digested.
- JSX and token files are task 09.
- No TypeScript computes this digest, or any digest (G9).
