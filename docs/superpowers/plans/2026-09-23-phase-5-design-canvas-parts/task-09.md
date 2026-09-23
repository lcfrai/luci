# Task 09 — JSX (inline and Tailwind), the layer tree, and the token files

**Plan:** `../2026-09-23-phase-5-design-canvas.md` · **Depends on:** 08 · **Hours:** 3

## Why

Paper's design-to-code path is `get_jsx` in two formats, Tailwind or inline styles (spec §2.5, §3.2
row 11), and *Copy as React* for humans (§5.2). Ours serves the same two formats to agents (task 15),
to the code panel (task 26), and into the handoff bundle (task 21), with `tokens.css`, a DTCG
`tokens.json` (spec §7.6, [S28]) and `tree.json` — the "component tree" `PROGRAM.md` lists for the
bundle, which in v1 is the layer hierarchy (true components are v2, spec §9.2). The Tailwind class
table is ours, a Go table: no `tailwindcss` dependency (spec L19 is needed only to render Tailwind,
which nothing here does).

## Files

- `canvas/serialize/jsx.go`, `tailwind.go`, `tree.go` (new); `canvas/tokens/css.go`, `dtcg.go` (new)
- tests and `testdata/*.golden` beside each

## Contracts

```go
// canvas/serialize/jsx.go
package serialize

type JSXFormat string

const (
	JSXInline   JSXFormat = "inline"   // style={{ camelCase: "value" }}
	JSXTailwind JSXFormat = "tailwind" // className="…" from tailwind.go
)

type JSXOptions struct {
	Format    JSXFormat
	AssetPath func(ref string) string // nil: src is the asset ref itself (get_jsx); the bundle maps to "assets/…"
}

// JSX renders the subtree as one exported React function component:
//
//   export function <Name>() {
//     return (
//       <tag …>…</tag>
//     );
//   }
//
// Name is the node's layer name in PascalCase (non-alphanumerics removed; a
// leading digit prefixed "Screen"; "" becomes "Frame"). Two-space indent.
// Rules: style keys camelCase (-webkit-x → WebkitX), values as strings;
// text as a JSX string expression {"…"} whenever it holds { } < > or ", else
// raw; "\n" as <br />; input value → defaultValue, checked → defaultChecked;
// a → href="#"; hidden → hidden; img → <img src alt />; svg attributes renamed
// to their React names (stroke-width → strokeWidth, fill-rule → fillRule,
// clip-path → clipPath, stop-color → stopColor, text-anchor → textAnchor,
// font-size → fontSize, …, the full table in jsx.go). Layer names are not
// emitted. Output is deterministic.
func JSX(d *doc.Doc, nodeID string, o JSXOptions) (string, error)
```

```go
// canvas/serialize/tailwind.go

// TailwindClasses maps one node's styles to Tailwind v4 classes, in the
// order of its sorted properties. The table, exhaustive for the keyword
// properties and the common lengths:
//   display flex|inline-flex|block|none → flex|inline-flex|block|hidden
//   flex-direction row|column|row-reverse|column-reverse → flex-row|flex-col|flex-row-reverse|flex-col-reverse
//   flex-wrap wrap|nowrap|wrap-reverse → flex-wrap|flex-nowrap|flex-wrap-reverse
//   align-items / align-self / justify-content → items-* / self-* / justify-* (start, end, center, stretch, baseline, between, around, evenly)
//   position absolute|relative|static → absolute|relative|static
//   gap, row-gap, column-gap, padding*, top/right/bottom/left, inset:
//       a px value that is a multiple of 4 → the spacing scale (8px → gap-2); 0 → *-0; else *-[Npx]
//   width/height: 100% → w-full, fit-content → w-fit, min-content → w-min, max-content → w-max, auto → w-auto, else w-[v]
//   font-weight 100…900 by hundreds → font-thin…font-black; else font-[v]
//   text-align → text-left|center|right|justify|start|end
//   opacity: a multiple of 0.05 → opacity-N (0.5 → opacity-50); else opacity-[v]
//   color / background-color / border-color with var(--x) → text-(--x) / bg-(--x) / border-(--x); any other colour → text-[v] / bg-[v] / border-[v]
//   font-size, line-height, letter-spacing, border-radius → text-[v], leading-[v], tracking-[v], rounded-[v]
//   anything else → an arbitrary property [prop:value], spaces written as "_" and "_" escaped as "\_"
// A padding or inset shorthand with several values is split into its sides first.
func TailwindClasses(styles map[string]string) []string
```

```go
// canvas/serialize/tree.go — tree.json in the bundle

type Tree struct {
	V         int        `json:"v"` // 1
	CanvasID  string     `json:"canvasId"`
	Revision  int        `json:"revision"`
	Artboards []TreeNode `json:"artboards"` // by page order, then artboard order
}

type TreeNode struct {
	ID       string      `json:"id"`
	Name     string      `json:"name"`
	Kind     doc.Kind    `json:"kind"`
	Tag      string      `json:"tag"`
	Page     string      `json:"page,omitempty"`   // artboards only
	Width    string      `json:"width,omitempty"`  // artboards only
	Height   string      `json:"height,omitempty"` // artboards only
	Layout   *TreeLayout `json:"layout,omitempty"` // frames with display flex|inline-flex, or position absolute
	Text     string      `json:"text,omitempty"`   // text nodes: the first 80 runes
	Tokens   []string    `json:"tokens"`           // token names this node's own styles use; [] not null
	Children []TreeNode  `json:"children"`         // [] not null
}

type TreeLayout struct {
	Display   string `json:"display"`
	Direction string `json:"direction,omitempty"`
	Wrap      string `json:"wrap,omitempty"`
	Gap       string `json:"gap,omitempty"`
	Padding   string `json:"padding,omitempty"`
	Align     string `json:"align,omitempty"`
	Justify   string `json:"justify,omitempty"`
	Position  string `json:"position,omitempty"` // "absolute" when so
}

func BuildTree(d *doc.Doc) Tree
```

```go
// canvas/tokens/css.go
package tokens

// CSS is tokens.css:
//   /* Generated from canvas <canvasId> at design revision <n>. Do not edit. */
//   :root {
//     --<name>: <value>;          ← sorted by name, two-space indent
//   }
// Every value is re-checked with dialect.Checker{}.TokenValue before it is written
// (review I2): a refused one fails with dialect.ErrUnsafeToken (task 07), wrapped with
// the token's name, and nothing is returned. DTCG applies the same re-check. tokens.css ships in the handoff bundle (task 21) and
// get_tokens hands it to an agent to paste into code (task 17), so a "}" or url() in a
// value would reach the repository's stylesheet. Same reason as ExportHTML: a document
// replayed from disk was never re-checked (task 10).
func CSS(canvasID string, revision int, ts []doc.Token) (string, error)
```

```go
// canvas/tokens/dtcg.go

// DTCG is tokens.json in the W3C Design Tokens Community Group format
// (spec [S28], draft 2025.10): one top-level member per token, sorted by
// name, each {"$type": …, "$value": …, "$extensions": {"app.panacea-app": {"css": "<value>", "type": "<our type>"}}}.
// $type by our type: color → "color"; radius, spacing, container, breakpoint,
// font-size, letter-spacing → "dimension"; font-family → "fontFamily";
// font-weight → "fontWeight"; line-height → "number" when unitless, else "dimension".
// $value: the draft's structured form where the CSS value is a hex colour
// or a px/rem length, else the CSS string, and $extensions always carries
// the exact CSS so nothing is lost. Step 1 reads [S28] and pins which forms
// the draft defines; the golden file records the choice.
func DTCG(ts []doc.Token) ([]byte, error)
```

## Tests first

| Test | Asserts | Kills the mutant |
|---|---|---|
| `TestJSX_inline_golden` | task 08's card as `testdata/card.inline.jsx.golden`, byte for byte | nondeterministic key order |
| `TestJSX_tailwind_golden` | the same as `card.tailwind.jsx.golden`: `className="h-fit w-[390px]"` on the artboard, `className="text-(--brand)"` on the text | the var() form written as `text-[var(--brand)]` |
| `TestJSX_textEscapes` | text `a {b} <c>` renders as `{"a {b} <c>"}` | raw braces (a JSX syntax error) |
| `TestJSX_input` | `value` → `defaultValue`, `checked` → `defaultChecked` | a controlled input with no handler (React warns) |
| `TestJSX_svgAttrs` | `stroke-width="2"` → `strokeWidth="2"`; `viewBox` unchanged | kebab-case attributes (React warns, some drop) |
| `TestJSX_componentName` | names `"settings page"`, `"2fa"`, `""` → `SettingsPage`, `Screen2fa`, `Frame` | an invalid identifier |
| `TestTailwind_table` (one row per table line above) | each mapping | any one mapping wrong |
| `TestTailwind_arbitraryEscapes` | `grid-area`-like unknowns never occur (dialect), but `box-shadow: 0 1px 2px rgb(0 0 0 / 0.2)` → `[box-shadow:0_1px_2px_rgb(0_0_0_/_0.2)]` | spaces left in a class |
| `TestTree_golden` | `BuildTree` of a two-artboard fixture equals `tree.golden.json` | `null` children |
| `TestTokensCSS_golden` | sorted, header present, trailing newline | unsorted output (a noisy diff per approval) |
| `TestTokensCSS_hostileToken` | `CSS` of tokens holding each of the review's two values (`red}</style><script>…</script><style>` and `url(https://tracker.example/p.png)`, type `font-family`) → an error that `errors.Is` `dialect.ErrUnsafeToken` and names the token, `""` returned; `DTCG` of the same tokens errs the same way | a `}` closing `:root` early, or a remote fetch, in the bundle's `tokens.css` (review I2) |
| `TestDTCG_golden` | every one of the ten token types maps as the table says; `$extensions` carries the CSS | a lossy DTCG (an `oklch()` colour silently dropped) |

## Steps

1. Read [S28] (`https://www.designtokens.org/tr/drafts/format/`) for the `color` and `dimension`
   value forms; write down in `dtcg.go`'s doc comment which you followed. If the page is blocked,
   emit the CSS string in `$value` and say so in the comment.
2. Write the goldens by hand from the rules above, then the tests, then the code.
3. Acceptance; commit by pathspec.

## Acceptance

```bash
cd /c/Users/au-dev-dmackie1/Projects/PanaceaApp/.claude/worktrees/phase-5 && go test ./canvas/serialize/... ./canvas/tokens/... -race -count=1
```
Expected: two `ok` lines.

## Not claimed

- The JSX is not compiled or rendered here. Task 30's end-to-end run has a Develop agent use it.
- Tailwind output targets v4's syntax (`text-(--x)`); it is not validated by Tailwind itself.
- The DTCG file follows a **draft** format.
