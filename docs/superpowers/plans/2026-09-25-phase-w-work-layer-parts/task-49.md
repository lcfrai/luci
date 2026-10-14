### Task 49: the editor, part 1: one Markdown editor on `prosemirror-*` (MIT), author class kept

Stream **WD3**, about 3 hours (an estimate, not a measurement). Depends on task 46 (`AuthorClass`,
the issue view it mounts in). Uses SW.1's `AuthoredText` (`task-02.md:237-242`) and SW.16's
`mutate`. Task 50 adds the slash menu and tables on top of this part's schema, parser, serializer
and shortcut table; task 51 reuses the editor for comments. **No second editor exists anywhere.**

An issue's description is the text an agent is given when its owner adopts it (05 §3.4), so the
editor has two jobs beyond typing: it must keep **who wrote what** (the author class), and it must
never turn a person's text into something else on the way through (a serializer that rewrites
`*` as `_` changes the digested revision for nothing). Five rules, each one a test:

1. **Round trip, byte for byte**, for the fixture set: parse then serialize gives the same bytes.
2. **No HTML, no unsafe link, no embed.** `markdown-it` runs with `html: false`; only `https:`,
   `http:` and `panacea:` links are live; images are not parsed at all, so `![x](url)` stays text
   and a link (L-128 is a SKIP).
3. **The author class is kept per passage.** An agent passage stays `text·A`; a person's edit
   inside it turns **only the edited range** `text·H`; pasted text is the person's. An agent
   passage carries a gutter mark and a word in its accessible name, never colour alone.
4. **One revision write per commit**: on blur, on `Ctrl+Enter`, or after 3 s idle (a proposed
   cadence, the owner's to judge), never per keystroke; opening and leaving without a change
   writes nothing.
5. **Typing is typing.** `C`, `S`, `N` and `1` inside the editor insert characters and reach no
   list's keys (task 53); `Ctrl+Enter` commits and does **not** reach the cockpit's `Ctrl+Enter`.

**Decisions made here, so nobody re-derives them:**

1. **The wire carries one class per field, so the span detail lives in the editor only.**
   `AuthoredText` is `{text, author}` (`task-02.md:237-242`): one class for the whole description.
   A digested revision cannot hold spans without changing task 02's canonical form, which is the
   Hub-contract owner's call (a gap, named below). So: the editor keeps `AuthorSpan`s in memory,
   exactly, while mounted; on commit the field's class is **`A` if any agent span remains, else
   `H`** (`fieldAuthor`). That is the conservative direction: a mixed text is never labelled a
   person's, because a person's label is what lets text reach another agent as spec-class (05
   §3.4). On load, `A` marks the whole text agent and `H` marks none.
2. **The mark is `agent`, and nothing parses it from the clipboard.** It has no `parseDOM` rule,
   so pasted HTML carrying our own `data-author="A"` cannot recreate an agent passage, and an
   `appendTransaction` plugin removes the mark from every range a transaction inserts (typing,
   paste, drop). Deleting inside an agent passage leaves the rest agent. `code_block` allows the
   mark, so an agent's code keeps its class.
3. **Offsets are found by sentinels.** `AuthorSpan {from, to}` counts in the Markdown string. The
   serializer writes the `agent` mark as two private-use characters (U+E000 open, U+E001 close);
   `serializeWithAuthorship` strips them and records their offsets. Both are removed from any
   input first (`clean`), so a text cannot smuggle a span in. Code blocks are written by our own
   serializer rule, which emits the sentinels around marked text inside the fence too.
4. **Nothing is rewritten unless the person changed it.** The editor commits only when the
   document differs from the last committed one (`doc.eq`), so opening a description written
   with `_em_` and leaving it writes nothing, even though the serializer would write `*em*`.
5. **`Ctrl+Enter` is the cockpit's *New session* chord** (`useKeyboardMap.ts:55`, `:208`). In the
   editor it commits and **stops propagation**, so no session is created; every other chord still
   bubbles to the global map. Plain keys (no Ctrl, no Meta) stop propagation at the editor's root,
   so an editor inside a list never feeds that list's single-key verbs.
6. **A live link opens outside the window.** In read-only mode a click on an `https:`/`http:` link
   goes to `@tauri-apps/plugin-opener`'s `openUrl` (already a dependency, `desktop/package.json`),
   never a webview navigation; a `panacea:` link goes through task 47's `parseIssueLink` to
   `onOpenIssue`. Anything else is not a link and renders as text.
7. **The shortcut table is data**, `SHORTCUTS` in `inputRules.ts`, and the input rules are built
   from it; task 50's slash menu reads its hints from the same table (L-226's rule).

**Files:**
- `desktop/src/components/editor/schema.ts` — the schema: `prosemirror-markdown`'s, with no
  `image`, a `checked` attribute on `list_item`, and the `agent` mark (new)
- `desktop/src/components/editor/markdown.ts` — `parseMarkdown`, `serializeMarkdown`,
  `serializeWithAuthorship`, `clean`; the `markdown-it` instance and its link rule (new)
- `desktop/src/components/editor/authorClass.ts` — `AuthorSpan`, `MarkdownDoc`, `fieldAuthor`,
  `docFromAuthored`, the plugin that keeps typed and pasted text the person's (new)
- `desktop/src/components/editor/inputRules.ts` — `SHORTCUTS` and `buildInputRules` (new; not in
  the scope's list)
- `desktop/src/components/editor/Editor.tsx` — the component: view, commit cadence, keys, links (new)
- `desktop/src/components/editor/editor.css` — owns `.ed-*` (new)
- `desktop/src/components/editor/{markdown,authorClass,Editor}.test.ts(x)`,
  `desktop/src/components/editor/fixtures/*.md` — the tests and the round-trip set (new)
- `desktop/package.json` ⚑, `desktop/package-lock.json` — ten `prosemirror-*` packages and
  `markdown-it`, each MIT, and `@types/markdown-it` (MIT) as a dev dependency (modified)
- `desktop/src/components/issue/IssueView.tsx` ⚑ (tasks 46-48's) — `slots.body` defaults to the
  editor on the description (modified)

#### The dependencies — `desktop/package.json` ⚑

| Package | Licence | Why |
|---|---|---|
| `prosemirror-model` | MIT | the document and schema |
| `prosemirror-state` | MIT | state, transactions, plugins |
| `prosemirror-view` | MIT | the editable view |
| `prosemirror-transform` | MIT | steps and mapping (the author plugin reads step maps) |
| `prosemirror-commands` | MIT | `baseKeymap`, `toggleMark` |
| `prosemirror-keymap` | MIT | `keymap` |
| `prosemirror-history` | MIT | the editor's own undo, separate from the list's `U` |
| `prosemirror-inputrules` | MIT | Markdown shortcuts as input rules |
| `prosemirror-schema-list` | MIT | `Enter` makes a new list item, `Mod-[`/`Mod-]` outdent and indent (not in the scope's list; without it `Enter` in a list splits a paragraph inside the item) |
| `prosemirror-markdown` | MIT | the CommonMark schema, `MarkdownParser`, `MarkdownSerializer` |
| `markdown-it` | MIT | the tokenizer under `prosemirror-markdown` |
| `@types/markdown-it` (dev) | MIT | its types, if the installed `markdown-it` ships none |

Versions are **UNVERIFIED** here: `desktop/node_modules` is not installed in this worktree, so no
version or licence was read from a lock file while writing. Step 2 installs with exact versions
(`npm --prefix desktop install --save-exact …`, the house pins `zustand` and the testing libraries
exactly), then reads each package's `license` field from `package-lock.json`; the licence test
below fails the build on anything but MIT, Apache-2.0, BSD-2/3-Clause or ISC.

#### Interfaces — complete: `desktop/src/components/editor/authorClass.ts`

```ts
// The author class inside the editor (decision 1). Offsets count in the Markdown string.
import { Plugin, PluginKey } from "prosemirror-state";
import type { AuthoredText } from "../../api/work";
import { editorSchema } from "./schema";

export type AuthorCls = "text·H" | "text·A";
export interface AuthorSpan { from: number; to: number; cls: AuthorCls }
export interface MarkdownDoc { markdown: string; authorship: readonly AuthorSpan[] }

/** Only agent spans are listed; everything not in one is the person's (text·H). */
export function fieldAuthor(spans: readonly AuthorSpan[]): "H" | "A" {
  return spans.some((s) => s.cls === "text·A" && s.to > s.from) ? "A" : "H";
}

/** Load from the wire: a whole-field class becomes one span or none. */
export function docFromAuthored(t: AuthoredText): MarkdownDoc {
  return {
    markdown: t.text,
    authorship: t.author === "A" && t.text !== "" ? [{ from: 0, to: t.text.length, cls: "text·A" }] : [],
  };
}

/** Commit to the wire (decision 1): a mixed text is A, never laundered to H. */
export const toAuthored = (d: MarkdownDoc): AuthoredText => ({ text: d.markdown, author: fieldAuthor(d.authorship) });

export const personTyped = new PluginKey("ed-person-typed");

/** Every range a transaction INSERTS loses the agent mark: typed, pasted and dropped text is the
 *  person's. Deleting leaves the rest as it was. Loads carry meta "ed-load" and are skipped. */
export const personTypedPlugin = new Plugin({
  key: personTyped,
  appendTransaction(trs, _old, state) {
    const agent = editorSchema.marks.agent;
    const ranges: [number, number][] = [];
    for (const tr of trs) {
      if (!tr.docChanged || tr.getMeta("ed-load")) continue;
      ranges.forEach((r, i) => { ranges[i] = [tr.mapping.map(r[0], -1), tr.mapping.map(r[1], 1)]; });
      tr.mapping.maps.forEach((map, i) => {
        map.forEach((_oS, _oE, nS, nE) => {
          if (nE <= nS) return;
          const rest = tr.mapping.slice(i + 1);
          ranges.push([rest.map(nS, -1), rest.map(nE, 1)]);
        });
      });
    }
    if (ranges.length === 0) return null;
    const out = state.tr;
    for (const [from, to] of ranges) if (to > from) out.removeMark(from, to, agent);
    return out.docChanged ? out.setMeta("addToHistory", false) : null;
  },
});
```

The plugin's own transaction is kept out of the undo history, so `Ctrl+Z` undoes the typing and
nothing else. `removeMark` adds a step only where the mark is present, so a person typing in their
own text costs nothing.

#### Interfaces — complete: `desktop/src/components/editor/schema.ts`

```ts
// prosemirror-markdown's CommonMark schema, narrowed and extended (decisions 2 and 3).
import { Schema, type MarkSpec, type NodeSpec } from "prosemirror-model";
import { schema as base } from "prosemirror-markdown";

const listItem: NodeSpec = {
  ...base.spec.nodes.get("list_item")!,
  attrs: { checked: { default: null } },             // null: a plain item; true/false: a task
  toDOM: (n) => ["li", n.attrs.checked === null ? {} : { "data-checked": String(n.attrs.checked) }, 0],
};

const codeBlock: NodeSpec = { ...base.spec.nodes.get("code_block")!, marks: "agent" };

/** The bullet the person typed, so "- a" does not come back as "* a" (rule 1). */
const bulletList: NodeSpec = {
  ...base.spec.nodes.get("bullet_list")!,
  attrs: { ...(base.spec.nodes.get("bullet_list")!.attrs ?? {}), bullet: { default: "-" } },
};

/** No parseDOM: nothing pasted can create an agent passage (decision 2). */
const agent: MarkSpec = {
  inclusive: false,
  excludes: "",
  toDOM: () => ["span", { class: "ed-agent", "data-author": "A" }, 0],
};

/** Only these schemes are live links (decision 6); the parser's validateLink uses the same test. */
export const LIVE_LINK = /^(https?:\/\/|panacea:\/\/)/i;

const link: MarkSpec = {
  ...base.spec.marks.get("link")!,
  toDOM: (m) => LIVE_LINK.test(String(m.attrs.href))
    ? ["a", { href: m.attrs.href, rel: "noopener noreferrer", class: "ed-link" }, 0]
    : ["span", { class: "ed-link-dead" }, 0],
};

export const editorSchema = new Schema({
  nodes: base.spec.nodes.remove("image").update("list_item", listItem)
    .update("code_block", codeBlock).update("bullet_list", bulletList),
  marks: base.spec.marks.update("link", link).addToEnd("agent", agent),
});
```

`base.spec.nodes` is an `OrderedMap` (`prosemirror-model`), whose `remove`, `update` and
`addToEnd` return new maps, so the base schema is untouched. The `link` guard in `toDOM` is the
second line of defence: the parser already refuses a dead scheme, and a document built any other
way (a paste, a future import) still cannot render one as an anchor. Task 50 adds `table`,
`table_row`, `table_cell` and `table_header` to this schema. Whether the installed
`prosemirror-markdown` already gives `bullet_list` a `bullet` attribute is **UNVERIFIED**; the
spread keeps its own if it does and adds ours if not.

#### Interfaces — complete: `desktop/src/components/editor/markdown.ts`, part 1: the parser

```ts
import MarkdownIt from "markdown-it";
import type { Node } from "prosemirror-model";
import { MarkdownParser, MarkdownSerializer, defaultMarkdownParser, defaultMarkdownSerializer }
  from "prosemirror-markdown";
import { Transform } from "prosemirror-transform";
import type { AuthorSpan, MarkdownDoc } from "./authorClass";
import { LIVE_LINK, editorSchema } from "./schema";

/** Decision 3's sentinels: private-use code points, removed from every input. */
export const OPEN = "";
export const CLOSE = "";
export const clean = (s: string): string => s.replace(/[]/g, "");

type Token = ReturnType<MarkdownIt["parse"]>[number];

/** `- [ ] a` and `- [x] a`: a task item. Ours, twelve lines, no plugin. */
function taskItems(state: { tokens: Token[] }): void {
  const t = state.tokens;
  for (let i = 0; i + 2 < t.length; i++) {
    if (t[i].type !== "list_item_open" || t[i + 1].type !== "paragraph_open" || t[i + 2].type !== "inline") continue;
    const m = /^\[( |x|X)\] /.exec(t[i + 2].content);
    if (!m) continue;
    t[i].attrSet("data-checked", m[1] === " " ? "false" : "true");
    t[i + 2].content = t[i + 2].content.slice(4);
    const first = t[i + 2].children?.[0];
    if (first && first.type === "text") first.content = first.content.slice(4);
  }
}

export function makeTokenizer(): MarkdownIt {
  const md = new MarkdownIt("commonmark", { html: false, linkify: false });
  md.disable(["image"]);                          // rule 2: nothing embeds; ![x](u) is "!" and a link
  md.validateLink = (url: string) => LIVE_LINK.test(url.trim());
  md.core.ruler.after("inline", "ed_task_items", taskItems);
  return md;
}

const baseTokens = Object.fromEntries(
  Object.entries(defaultMarkdownParser.tokens).filter(([name]) => name !== "image"),
);

const baseBullet = defaultMarkdownParser.tokens.bullet_list;

export const parserTokens = {
  ...baseTokens,
  bullet_list: {
    block: "bullet_list",
    // Keep whatever the installed version computes (tight, at least) and add the bullet.
    getAttrs: (tok: Token, stream: Token[], i: number) =>
      ({ ...(baseBullet.getAttrs?.(tok, stream, i) ?? {}), bullet: tok.markup || "-" }),
  },
  list_item: {
    block: "list_item",
    getAttrs: (tok: Token) => {
      const c = tok.attrGet("data-checked");
      return { checked: c === null ? null : c === "true" };
    },
  },
};

export const parser = new MarkdownParser(editorSchema, makeTokenizer(), parserTokens);

/** Rule 1 keeps the trailing newlines the person wrote; the serializer never writes any. */
export const trailingNewlines = (md: string): string => /\n*$/.exec(md)?.[0] ?? "";

/** `author` is the field's class (decision 1): "A" marks every text node agent. */
export function parseMarkdown(md: string, author: "H" | "A" = "H"): Node {
  const doc = parser.parse(clean(md));
  if (author === "H" || doc.content.size === 0) return doc;
  return new Transform(doc).addMark(0, doc.content.size, editorSchema.marks.agent.create()).doc;
}
```

`bullet_list`'s `getAttrs` calls the installed one first, so its `tight` computation is kept; the
`(token, stream, index)` signature is `prosemirror-markdown`'s `ParseSpec.getAttrs` as this plan
understands it, **UNVERIFIED** against the installed types until step 2.

#### Interfaces — complete: `markdown.ts`, part 2: the serializer

```ts
const baseNodes = defaultMarkdownSerializer.nodes;
type NodeFn = (typeof baseNodes)[string];

const isAgent = (n: Node): boolean => n.marks.some((m) => m.type === editorSchema.marks.agent);

export const serializerNodes: Record<string, NodeFn> = {
  ...baseNodes,
  bullet_list(state, node) {
    state.renderList(node, "  ", () => `${node.attrs.bullet ?? "-"} `);
  },
  list_item(state, node, parent, index) {
    if (node.attrs.checked !== null) state.write(node.attrs.checked ? "[x] " : "[ ] ");
    baseNodes.list_item(state, node, parent, index);
  },
  /** The base rule writes node.textContent and loses marks; this one writes each text child with
   *  its sentinels, so an agent's code keeps its class (decision 3). The fence rule is the base's. */
  code_block(state, node) {
    const runs = node.textContent.match(/`{3,}/gm);
    const fence = runs ? `${runs.sort().slice(-1)[0]}\`` : "```";
    state.write(`${fence}${node.attrs.params || ""}\n`);
    node.forEach((child) => state.text(isAgent(child) ? OPEN + (child.text ?? "") + CLOSE : child.text ?? "", false));
    state.ensureNewLine();
    state.write(fence);
    state.closeBlock(node);
  },
};

export const serializerMarks = {
  ...defaultMarkdownSerializer.marks,
  agent: { open: OPEN, close: CLOSE, mixable: true, expelEnclosingWhitespace: false },
};

export const serializer = new MarkdownSerializer(serializerNodes, serializerMarks);

/** Strip the sentinels and record where they were; adjacent spans merge. */
export function serializeWithAuthorship(doc: Node, trailing = ""): MarkdownDoc {
  const raw = serializer.serialize(doc) + trailing;
  const out: string[] = [];
  const spans: AuthorSpan[] = [];
  let len = 0;
  let open = -1;
  for (const ch of raw) {
    if (ch === OPEN) { if (open < 0) open = len; continue; }
    if (ch === CLOSE) {
      if (open >= 0 && len > open) {
        const last = spans[spans.length - 1];
        if (last && last.to === open) last.to = len; else spans.push({ from: open, to: len, cls: "text·A" });
      }
      open = -1;
      continue;
    }
    out.push(ch);
    len += ch.length;
  }
  return { markdown: out.join(""), authorship: spans };
}

export const serializeMarkdown = (doc: Node, trailing = ""): string =>
  serializeWithAuthorship(doc, trailing).markdown;
```

The fence rule (the longest run of backticks inside, plus one) is the base serializer's
(`prosemirror-markdown`, MIT, so reuse is permitted; `OD-11`'s clean room covers Linear, Plane and
canvas sources), and a fixture with a triple backtick inside a fence holds it. `ensureNewLine` replaces the base's unconditional
`write("\n")` because `state.text` of a line that already ends in a newline would otherwise add a
blank line; the round-trip fixture holds which is right (**UNVERIFIED** until step 3 runs it).
`len += ch.length` counts UTF-16 units, the unit JavaScript string offsets use, so an emoji in a
passage moves the offsets by two, as `String.prototype.slice` expects.

#### Interfaces — complete: `desktop/src/components/editor/inputRules.ts`

```ts
// Decision 7: the Markdown shortcuts as DATA. The input rules are built from this table, and task
// 50's slash menu shows each item's hint from it, so the menu teaches exactly what typing does.
import { InputRule, inputRules, textblockTypeInputRule, wrappingInputRule } from "prosemirror-inputrules";
import type { MarkType, Schema } from "prosemirror-model";
import { findWrapping } from "prosemirror-transform";
import type { Plugin } from "prosemirror-state";

export type ShortcutId = "h1" | "h2" | "h3" | "bullet" | "ordered" | "check" | "quote" | "code"
  | "divider" | "bold" | "code_inline";
export interface Shortcut { id: ShortcutId; trigger: string; label: string }

export const SHORTCUTS: readonly Shortcut[] = Object.freeze([
  { id: "h1", trigger: "# ", label: "Heading 1" },
  { id: "h2", trigger: "## ", label: "Heading 2" },
  { id: "h3", trigger: "### ", label: "Heading 3" },
  { id: "bullet", trigger: "- ", label: "Bulleted list" },
  { id: "ordered", trigger: "1. ", label: "Numbered list" },
  { id: "check", trigger: "[ ] ", label: "Check list" },
  { id: "quote", trigger: "> ", label: "Quote" },
  { id: "code", trigger: "```", label: "Code block" },
  { id: "divider", trigger: "---", label: "Divider" },
  { id: "bold", trigger: "**", label: "Bold" },
  { id: "code_inline", trigger: "`", label: "Inline code" },
]);

const esc = (s: string) => s.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
/** A block trigger typed at the start of a textblock; its trailing space may be any space. */
export const blockPattern = (trigger: string): RegExp => new RegExp(`^${esc(trigger).replace(/ $/, "\\s")}$`);

function markRule(re: RegExp, type: MarkType): InputRule {
  return new InputRule(re, (state, m, start, end) =>
    state.tr.replaceWith(start, end, state.schema.text(m[1], [type.create()])).removeStoredMark(type));
}

export function buildInputRules(schema: Schema): Plugin {
  const n = schema.nodes;
  const t = (id: ShortcutId) => SHORTCUTS.find((s) => s.id === id)!.trigger;
  const rules: InputRule[] = [
    textblockTypeInputRule(blockPattern(t("h1")), n.heading, { level: 1 }),
    textblockTypeInputRule(blockPattern(t("h2")), n.heading, { level: 2 }),
    textblockTypeInputRule(blockPattern(t("h3")), n.heading, { level: 3 }),
    wrappingInputRule(blockPattern(t("bullet")), n.bullet_list, { bullet: "-" }),
    wrappingInputRule(/^(\d+)\.\s$/, n.ordered_list, (m) => ({ order: +m[1] }),
      (m, node) => node.childCount + node.attrs.order === +m[1]),
    new InputRule(blockPattern(t("check")), (state, _m, start, end) => {
      const tr = state.tr.delete(start, end);
      const $s = tr.doc.resolve(start);
      if ($s.depth >= 2 && $s.node(-1).type === n.list_item) {
        return tr.setNodeMarkup($s.before(-1), undefined, { ...$s.node(-1).attrs, checked: false });
      }
      const range = $s.blockRange();
      const wrap = range && findWrapping(range, n.bullet_list, { bullet: "-" });
      if (!range || !wrap) return null;
      wrap[wrap.length - 1] = { type: n.list_item, attrs: { checked: false } };
      return tr.wrap(range, wrap);
    }),
    wrappingInputRule(blockPattern(t("quote")), n.blockquote),
    textblockTypeInputRule(blockPattern(t("code")), n.code_block),
    new InputRule(blockPattern(t("divider")), (state, _m, start) => {
      const $s = state.doc.resolve(start);
      return state.tr.replaceRangeWith($s.before(), $s.after(), n.horizontal_rule.create());
    }),
    markRule(/\*\*([^*]+)\*\*$/, schema.marks.strong),
    markRule(/`([^`]+)`$/, schema.marks.code),
  ];
  return inputRules({ rules });
}
```

`blockPattern("- ")` is `/^-\s$/`, so the rule fires only when the typed trigger is the whole
start of the block: a hyphen and a space in the middle of a sentence stay text. The numbered-list
rule takes any number, as CommonMark does; its **hint** is `1. `. The divider rule fires on the
third hyphen of an otherwise empty paragraph. `prosemirror-inputrules` skips a textblock whose
type is `code`, so nothing inside a code block is rewritten.

#### Interfaces — complete: `desktop/src/components/editor/Editor.tsx`, part 1: the gutter

```tsx
import { useEffect, useRef, type JSX } from "react";
import { baseKeymap, toggleMark } from "prosemirror-commands";
import { history, redo, undo } from "prosemirror-history";
import { keymap } from "prosemirror-keymap";
import type { Node } from "prosemirror-model";
import { liftListItem, sinkListItem, splitListItem } from "prosemirror-schema-list";
import { EditorState, Plugin } from "prosemirror-state";
import { Decoration, DecorationSet, EditorView } from "prosemirror-view";
import { parseIssueLink, type IssueLinkTarget } from "../../work/links";
import { AUTHOR_PATH } from "../issue/AuthorClass";
import { fieldAuthor, personTypedPlugin, type MarkdownDoc } from "./authorClass";
import { buildInputRules } from "./inputRules";
import { parseMarkdown, serializeWithAuthorship, trailingNewlines } from "./markdown";
import { LIVE_LINK, editorSchema } from "./schema";
import "./editor.css";

const SVG = "http://www.w3.org/2000/svg";

/** The gutter mark: task 46's agent shape and the words "text·A · agent", as plain DOM because a
 *  widget is not React. contenteditable=false, so it is never typed into or copied as text. */
function gutterMark(): HTMLElement {
  const box = document.createElement("span");
  box.className = "ed-gutter";
  box.contentEditable = "false";
  box.dataset.author = "A";
  const svg = document.createElementNS(SVG, "svg");
  svg.setAttribute("width", "12"); svg.setAttribute("height", "12"); svg.setAttribute("viewBox", "0 0 12 12");
  svg.setAttribute("aria-hidden", "true"); svg.setAttribute("focusable", "false");
  const path = document.createElementNS(SVG, "path");
  path.setAttribute("d", AUTHOR_PATH.A); path.setAttribute("class", "ed-gutter-ring");
  svg.appendChild(path);
  const word = document.createElement("span");
  word.className = "ed-vh";
  word.textContent = "text·A · agent: ";
  box.append(svg, word);
  return box;
}

const hasAgent = (block: Node): boolean => {
  let found = false;
  block.descendants((n) => { if (n.marks.some((m) => m.type === editorSchema.marks.agent)) found = true; return !found; });
  return found;
};

/** One gutter mark per textblock that holds agent text; recomputed per change (issue texts are
 *  short; a long brief would want an incremental set, phase 4's concern, not this editor's). */
export const agentGutterPlugin = new Plugin({
  props: {
    decorations(state) {
      const out: Decoration[] = [];
      state.doc.descendants((n, pos) => {
        if (!n.isTextblock) return true;
        if (hasAgent(n)) {
          out.push(Decoration.widget(pos + 1, gutterMark, { side: -1, key: "ed-gutter", ignoreSelection: true }));
          out.push(Decoration.node(pos, pos + n.nodeSize, { class: "ed-agent-block" }));
        }
        return false;
      });
      return DecorationSet.create(state.doc, out);
    },
  },
});
```

The visible mark is the shape; the word is in a visually hidden span, so a screen reader reads
*"text·A · agent:"* before the passage and a sighted reader sees the bracket in the gutter. Inside
the passage the `.ed-agent` span carries a dotted underline as well, so the class survives in
`forced-colors`, where every colour is repainted (task 34).

#### Interfaces — complete: `Editor.tsx`, part 2: the component

```tsx
/** A proposed cadence (rule 4), the owner's to judge. A debounce, not a clock: nothing shows it. */
export const IDLE_COMMIT_MS = 3000;

export interface EditorProps {
  value: MarkdownDoc;
  onCommit(next: MarkdownDoc): void;
  readOnly?: boolean;
  ariaLabel: string;
  onOpenIssue?(target: IssueLinkTarget): void;
}

function openLink(href: string, onOpenIssue?: (t: IssueLinkTarget) => void): void {
  const issue = parseIssueLink(href);
  if (issue) { onOpenIssue?.(issue); return; }
  if (/^https?:\/\//i.test(href)) void import("@tauri-apps/plugin-opener").then((m) => m.openUrl(href));
}

export function Editor(props: EditorProps): JSX.Element {
  const { value, readOnly = false, ariaLabel } = props;
  const host = useRef<HTMLDivElement>(null);
  const view = useRef<EditorView | null>(null);
  const committed = useRef<Node | null>(null);
  const trailing = useRef("");
  const idle = useRef<ReturnType<typeof setTimeout> | null>(null);
  const latest = useRef(props);
  latest.current = props;

  const commit = () => {
    if (idle.current) { clearTimeout(idle.current); idle.current = null; }
    const v = view.current;
    if (!v || committed.current === null || v.state.doc.eq(committed.current)) return;   // decision 4
    committed.current = v.state.doc;
    latest.current.onCommit(serializeWithAuthorship(v.state.doc, trailing.current));
  };

  const load = (d: MarkdownDoc): EditorState => {
    trailing.current = trailingNewlines(d.markdown);
    const doc = parseMarkdown(d.markdown, fieldAuthor(d.authorship));
    committed.current = doc;
    return EditorState.create({ doc, plugins: [
      buildInputRules(editorSchema), personTypedPlugin, agentGutterPlugin, history(),
      keymap({ "Mod-Enter": () => { commit(); return true; }, "Mod-z": undo, "Mod-y": redo,
        "Mod-Shift-z": redo, "Mod-b": toggleMark(editorSchema.marks.strong),
        "Mod-i": toggleMark(editorSchema.marks.em),
        Enter: splitListItem(editorSchema.nodes.list_item),
        "Mod-[": liftListItem(editorSchema.nodes.list_item),
        "Mod-]": sinkListItem(editorSchema.nodes.list_item) }),
      keymap(baseKeymap),
    ] });
  };

  useEffect(() => {
    const v = new EditorView(host.current!, {
      state: load(value),
      editable: () => !latest.current.readOnly,
      attributes: { class: "ed-root", role: "textbox", "aria-multiline": "true", "aria-label": ariaLabel },
      dispatchTransaction(tr) {
        v.updateState(v.state.apply(tr));
        if (!tr.docChanged) return;
        if (idle.current) clearTimeout(idle.current);
        idle.current = setTimeout(commit, IDLE_COMMIT_MS);
      },
      handleDOMEvents: {
        keydown: (_v, e) => {
          // Decision 5: plain keys never reach a list's verbs; Ctrl+Enter never reaches New session.
          if (!(e.ctrlKey || e.metaKey) || e.key === "Enter") e.stopPropagation();
          return false;
        },
        blur: () => { commit(); return false; },
        // The raw click, not PM's handleClick (which runs on mouseup): the anchor's navigation is
        // the click's default action, so this is the event that must be prevented.
        click: (_v, e) => {
          const a = (e.target as Element | null)?.closest("a.ed-link");
          if (!a) return false;
          e.preventDefault();                         // never a webview navigation (decision 6)
          if (!latest.current.readOnly) return false; // editing: a click places the caret
          const href = a.getAttribute("href") ?? "";
          if (LIVE_LINK.test(href)) openLink(href, latest.current.onOpenIssue);
          return true;
        },
      },
    });
    view.current = v;
    VIEWS.set(host.current!, v);
    return () => { commit(); v.destroy(); view.current = null; };
  }, []);                                             // one view per mount; props are read via `latest`

  // PM reads `editable` only on an update, so a readOnly change asks for one.
  useEffect(() => { view.current?.setProps({}); }, [readOnly]);

  // A new value from outside (another person's revision) replaces the doc only when this one has
  // nothing uncommitted; otherwise the person's text wins and task 56's conflict rule applies.
  // Keyed on strings, not on the value object, which a caller rebuilds every render; and a value
  // that parses to the doc already shown changes nothing, so the caret and history survive.
  const cls = fieldAuthor(value.authorship);
  useEffect(() => {
    const v = view.current;
    if (!v || committed.current === null || !v.state.doc.eq(committed.current)) return;
    if (parseMarkdown(value.markdown, cls).eq(v.state.doc)) return;
    v.updateState(load(value));
  }, [value.markdown, cls]);

  // data-verbatim: the text is the person's own (task 20's exemption), so "50% of users" in a
  // description is not a prediction; the chrome around the editor is still scanned.
  return <div ref={host} className="ed" data-verbatim data-readonly={readOnly || undefined} />;
}

/** Test seam: the view behind an editor's host element. A WeakMap, so nothing leaks. */
const VIEWS = new WeakMap<Element, EditorView>();
export const editorViewOf = (el: Element): EditorView | undefined => VIEWS.get(el);
```

`VIEWS` is declared at module scope, above `Editor`, in the file; it is listed after the component
here only to keep the component in one block. A production caller has no reason to reach for it.

Four things in that component are deliberate. `latest` holds the newest props so the one view,
built once, never calls a stale `onCommit`. Unmounting commits, so closing the issue view with
uncommitted text is a blur, not a loss. `openUrl` is `@tauri-apps/plugin-opener`'s v2 call as
this plan understands it (the repo already imports the plugin dynamically for `revealItemInDir`,
`components/menu/ContextMenu.tsx:44`); the name is **UNVERIFIED** until step 2 reads the
installed types. The idle timer is a debounce inside one component, not a displayed clock, so the
one-heartbeat rule (`DESIGN-SYSTEM.md`) does not apply to it; it is cleared on every commit and on
unmount.

#### Interfaces — complete: the edit to `IssueView.tsx` (the description)

```tsx
import { Editor } from "../editor/Editor";
import { docFromAuthored, toAuthored } from "../editor/authorClass";
import { mutate } from "../../work/replica/ops";
//   in IssueViewBody, in place of task 47's plain-text fallback for slots.body:
{slots.body ?? (
  <Editor
    ariaLabel={`Description of ${head.alias}`}
    value={docFromAuthored(shown.value.content.description)}
    readOnly={at !== headRevision.revision}
    onOpenIssue={(t) => props.onOpenIssue?.(t)}
    onCommit={(next) => mutate({ kind: "revise", issueId: head.id, baseRev: headRevision.revision,
      patch: { description: toAuthored(next) } })}
  />
)}
```

`IssueView` gains `onOpenIssue?(target: IssueLinkTarget): void`, which its mount wires to open the
issue at `target.rev` (task 40's). An old revision is read-only: its description can be read and
its links followed, and editing starts from the head (task 47's rule 1). The `revise` op carries
the description only; its `baseRev` is the head, so a teammate's newer revision is task 56's 409
and the person's text is kept.

#### Interfaces — complete: `desktop/src/components/editor/editor.css`

```css
/* editor.css owns .ed-* and no other prefix. Text is sans; code is Commit Mono (voice.split). */
.ed { position: relative; padding: var(--s-3) var(--s-4) var(--s-3) calc(var(--s-4) + 16px); }
.ed-root { min-block-size: 4lh; outline: none; font-family: var(--sans); color: var(--ink);
  white-space: pre-wrap; word-wrap: break-word; }
.ed-root:focus-visible { box-shadow: 0 0 0 1px var(--ring); border-radius: var(--r-sm); }
.ed-root code, .ed-root pre { font-family: var(--mono); font-size: var(--t-fact); }
.ed-root pre { padding: var(--s-2); border-radius: var(--r-sm); box-shadow: 0 0 0 1px var(--ring); }
.ed-agent { text-decoration: underline dotted 1px; text-underline-offset: 3px; }
.ed-agent-block { position: relative; }
.ed-gutter { position: absolute; inset-inline-start: -16px; inset-block-start: 0.2em; color: var(--ink-2);
  user-select: none; }
.ed-gutter-ring { fill: none; stroke: currentColor; stroke-width: 1.25; }
.ed-vh { position: absolute; inline-size: 1px; block-size: 1px; overflow: hidden; clip-path: inset(50%); }
.ed-link { color: var(--ink); text-decoration: underline; text-underline-offset: 2px; }
.ed-link-dead { color: var(--ink); }
.ed[data-readonly] .ed-root { cursor: default; }
.ed-root li[data-checked]::marker { font-family: var(--mono); }
.ed-root li[data-checked="false"]::marker { content: "[ ] "; }
.ed-root li[data-checked="true"]::marker { content: "[x] "; }
```

A dead link is ink with no underline: it reads as text, which it is. The task item's marker is the
Markdown it came from, in mono, so the item teaches its own shortcut; a clickable checkbox is not
in W1 (toggling one is an edit like any other, by typing).

#### The round-trip set — `desktop/src/components/editor/fixtures/*.md`

Seven files, each already in the serializer's canonical form, so parse then serialize must give
the same bytes. `⏎` below marks the file's final newline where it has one.

| File | Content (exact) | What it holds |
|---|---|---|
| `headings.md` | `# One\n\n## Two\n\n### Three` | the three heading levels |
| `lists.md` | `- a\n- b\n\n1. one\n2. two\n\n* star` | `-` and `*` bullets kept apart; numbering |
| `tasks.md` | `- [ ] open\n- [x] done` | the task items and their markers |
| `inline.md` | `Some *em*, **strong**, \`code\`, [a](https://example.test/x) and [b](panacea://issue/is-3f2a91c07b1e4d2a@3).` | marks and the two live schemes |
| `quote.md` | `> quoted\n> twice` | a blockquote over two lines |
| `code.md` | ```` ```go\nfmt.Println("```")\n``` ```` written with a four-backtick fence around it | a fence containing a triple backtick |
| `trailing.md` | `Ends with a newline.⏎` | the trailing newline kept |

`lists.md`'s `* star` sits after the ordered list as its own list; whether `prosemirror-markdown`
separates two adjacent bullet lists of different markers with one blank line or two is
**UNVERIFIED** until step 3, and the fixture is corrected to what CommonMark requires, never to
what the serializer happens to emit.

#### Tests to write first — `desktop/src/components/editor/markdown.test.ts`

```ts
import { describe, expect, it } from "vitest";
import { DOMSerializer } from "prosemirror-model";
import lock from "../../../package-lock.json";
import { editorSchema } from "./schema";
import { clean, parseMarkdown, serializeMarkdown, trailingNewlines } from "./markdown";

const FIXTURES = import.meta.glob("./fixtures/*.md", { query: "?raw", import: "default", eager: true }) as Record<string, string>;

function render(md: string): HTMLElement {
  const box = document.createElement("div");
  box.appendChild(DOMSerializer.fromSchema(editorSchema).serializeFragment(parseMarkdown(md).content));
  return box;
}

describe("markdown", () => {
  it("round-trips every fixture byte for byte", () => {
    expect(Object.keys(FIXTURES)).toHaveLength(7);
    for (const [name, src] of Object.entries(FIXTURES)) {
      expect([name, serializeMarkdown(parseMarkdown(src), trailingNewlines(src))]).toEqual([name, src]);
    }
  });
  it("renders raw HTML as text", () => {
    const box = render("<script>alert(1)</script> and <b>bold</b>");
    expect(box.querySelector("script, b")).toBeNull();
    expect(box.textContent).toContain("<script>alert(1)</script>");
  });
  it("makes no unsafe scheme a live link, parsed or built by hand", () => {
    for (const bad of ["[x](javascript:alert(1))", "[x](data:text/html,hi)", "[x](file:///c:/x)", "<vbscript:x>"]) {
      expect(render(bad).querySelector("a")).toBeNull();
    }
    const handMade = editorSchema.text("x", [editorSchema.marks.link.create({ href: "javascript:alert(1)" })]);
    const box = document.createElement("div");
    box.appendChild(DOMSerializer.fromSchema(editorSchema).serializeFragment(editorSchema.nodes.paragraph.create(null, handMade).content));
    expect(box.querySelector("a")).toBeNull();
  });
  it("keeps https, http and panacea links live", () => {
    const box = render("[a](https://e.test) [b](http://e.test) [c](panacea://issue/is-3f2a91c07b1e4d2a)");
    expect(Array.from(box.querySelectorAll("a")).map((a) => a.getAttribute("href")))
      .toEqual(["https://e.test", "http://e.test", "panacea://issue/is-3f2a91c07b1e4d2a"]);
  });
  it("embeds nothing: an image stays a link", () => {
    expect(render("![x](https://e.test/a.png)").querySelector("img")).toBeNull();
  });
  it("strips the authorship sentinels from any input", () => {
    expect(clean("a\uE000b\uE001c")).toBe("abc");
  });
  it("adds only permissive licences", () => {
    const names = ["prosemirror-model", "prosemirror-state", "prosemirror-view", "prosemirror-transform",
      "prosemirror-commands", "prosemirror-keymap", "prosemirror-history", "prosemirror-inputrules",
      "prosemirror-schema-list", "prosemirror-markdown", "markdown-it"];
    const pkgs = (lock as { packages: Record<string, { license?: string }> }).packages;
    for (const n of names) {
      expect([n, pkgs[`node_modules/${n}`]?.license]).toEqual([n, expect.stringMatching(/^(MIT|Apache-2\.0|BSD-[23]-Clause|ISC)$/)]);
    }
  });
});
```

`desktop/package-lock.json` exists and carries `license` fields (139 of them, counted 2026-09-25),
so the licence test reads the file the install writes. Vitest's `css.include` already admits
`?raw` for CSS only (`desktop/vitest.config.ts`); `.md?raw` goes through Vite's raw loader without it.

#### Tests to write first — `desktop/src/components/editor/authorClass.test.ts`

```ts
import { describe, expect, it } from "vitest";
import { EditorState } from "prosemirror-state";
import { docFromAuthored, fieldAuthor, personTypedPlugin, toAuthored } from "./authorClass";
import { parseMarkdown, serializeWithAuthorship } from "./markdown";
import { editorSchema } from "./schema";

const agentState = (md: string) =>
  EditorState.create({ doc: parseMarkdown(md, "A"), plugins: [personTypedPlugin] });
const after = (s: EditorState, tr: ReturnType<EditorState["tr"]["insertText"]>) =>
  serializeWithAuthorship(s.applyTransaction(tr).state.doc);

describe("the author class inside the editor", () => {
  it("marks only the typed range the person's, and keeps the rest agent on both sides", () => {
    const s = agentState("agent words here");               // doc positions: text starts at 1
    const out = after(s, s.tr.insertText("person ", 1 + "agent ".length));
    expect(out.markdown).toBe("agent person words here");
    expect(out.authorship).toEqual([
      { from: 0, to: 6, cls: "text·A" }, { from: 13, to: 23, cls: "text·A" },
    ]);
  });
  it("keeps what survives a deletion inside an agent passage agent", () => {
    const s = agentState("agent words here");
    const out = after(s, s.tr.delete(1 + 6, 1 + 12));
    expect(out).toEqual({ markdown: "agent here", authorship: [{ from: 0, to: 10, cls: "text·A" }] });
  });
  it("makes pasted text the person's even when it arrives marked agent", () => {
    const s = agentState("x");
    const pasted = editorSchema.text("pasted", [editorSchema.marks.agent.create()]);
    const out = after(s, s.tr.insert(2, pasted));
    expect(out.authorship).toEqual([{ from: 0, to: 1, cls: "text·A" }]);
  });
  it("labels a mixed text agent on the wire, never the person's", () => {
    expect(fieldAuthor([])).toBe("H");
    expect(fieldAuthor([{ from: 3, to: 5, cls: "text·A" }])).toBe("A");
    expect(toAuthored({ markdown: "ab", authorship: [{ from: 0, to: 1, cls: "text·A" }] })).toEqual({ text: "ab", author: "A" });
  });
  it("loads a whole-field class as one span or none", () => {
    expect(docFromAuthored({ text: "abc", author: "A" }).authorship).toEqual([{ from: 0, to: 3, cls: "text·A" }]);
    expect(docFromAuthored({ text: "abc", author: "H" }).authorship).toEqual([]);
    expect(docFromAuthored({ text: "", author: "A" }).authorship).toEqual([]);
  });
});
```

| Test | Mutant it kills |
|---|---|
| round-trips every fixture | a serializer that rewrites `-` as `*`, `*` as `_`, or drops a trailing newline |
| raw HTML as text | `html: true` |
| no unsafe live link | an unsafe `href` (parsed, or built by a paste) |
| https, http, panacea live | a guard so strict that no link works |
| embeds nothing | an `<img>` (L-128 is a SKIP) |
| strips sentinels | a text that smuggles an agent span in |
| permissive licences | a copyleft dependency |
| only the typed range | flipping the whole passage to the person's; losing the agent mark on any edit |
| deletion keeps the rest | a delete that clears the class of what is left |
| paste is the person's | a paste that recreates an agent passage |
| mixed is agent on the wire | a mixed text laundered to `text·H` (decision 1) |
| whole-field load | an empty text marked agent; a load that ignores the class |

#### Tests to write first — `desktop/src/components/editor/Editor.test.tsx`

```tsx
import { afterEach, describe, expect, it, vi } from "vitest";
import { act, fireEvent, render } from "@testing-library/react";
import { Editor, IDLE_COMMIT_MS, editorViewOf } from "./Editor";

vi.mock("@tauri-apps/plugin-opener", () => ({ openUrl: vi.fn(async () => {}) }));
afterEach(() => vi.useRealTimers());

function mount(markdown: string, author: "H" | "A" = "H", readOnly = false) {
  const onCommit = vi.fn();
  const parentKeys = vi.fn();
  const authorship = author === "A" ? [{ from: 0, to: markdown.length, cls: "text·A" as const }] : [];
  const r = render(
    <div role="list" onKeyDown={parentKeys}>
      <Editor ariaLabel="Description" value={{ markdown, authorship }} onCommit={onCommit} readOnly={readOnly} />
    </div>,
  );
  const host = r.container.querySelector(".ed")!;
  const view = editorViewOf(host)!;
  const type = (s: string) => act(() => { view.dispatch(view.state.tr.insertText(s)); });
  return { ...r, host, view, onCommit, parentKeys, type };
}

describe("Editor", () => {
  it("marks an agent passage with a shape and a word, not a colour", () => {
    const { host } = mount("an agent wrote this", "A");
    const gutter = host.querySelector(".ed-gutter")!;
    expect(gutter.querySelector("svg path")).not.toBeNull();
    expect(gutter.textContent).toBe("text·A · agent: ");
  });
  it("keeps plain keys from a surrounding list, and inserts what is typed", () => {
    const { view, parentKeys, type } = mount("");
    for (const key of ["C", "S", "N", "1"]) fireEvent.keyDown(view.dom, { key });
    type("CSN1");
    expect(parentKeys).not.toHaveBeenCalled();
    expect(view.state.doc.textContent).toBe("CSN1");
  });
  it("lets a chord reach the cockpit, except Ctrl+Enter, which commits here", () => {
    const { view, parentKeys, onCommit, type } = mount("a");
    fireEvent.keyDown(view.dom, { key: "k", ctrlKey: true });
    expect(parentKeys).toHaveBeenCalledTimes(1);
    type("b");
    fireEvent.keyDown(view.dom, { key: "Enter", ctrlKey: true });
    expect(parentKeys).toHaveBeenCalledTimes(1);               // New session never fires
    expect(onCommit).toHaveBeenCalledTimes(1);
  });
  it("sends one revision write per blur, none per keystroke, none without a change", () => {
    const { view, onCommit, type } = mount("start");
    fireEvent.blur(view.dom);
    expect(onCommit).not.toHaveBeenCalled();
    for (const ch of "abcde") type(ch);
    expect(onCommit).not.toHaveBeenCalled();
    fireEvent.blur(view.dom);
    fireEvent.blur(view.dom);
    expect(onCommit).toHaveBeenCalledTimes(1);
    expect(onCommit.mock.calls[0][0].markdown).toBe("abcdestart");
  });
  it("commits after the idle cadence, once", () => {
    vi.useFakeTimers();
    const { onCommit, type } = mount("x");
    type("y");
    act(() => { vi.advanceTimersByTime(IDLE_COMMIT_MS - 1); });
    expect(onCommit).not.toHaveBeenCalled();
    act(() => { vi.advanceTimersByTime(1); });
    expect(onCommit).toHaveBeenCalledTimes(1);
  });
  it("never rewrites a person's Markdown style unless they edit it", () => {
    const { view, onCommit } = mount("some _em_ here");
    fireEvent.blur(view.dom);
    expect(onCommit).not.toHaveBeenCalled();
  });
  it("opens a live link outside the window when read-only, and navigates nothing", async () => {
    const { host } = mount("[site](https://example.test/a)", "H", true);
    const a = host.querySelector("a.ed-link")!;
    const click = new MouseEvent("click", { bubbles: true, cancelable: true });
    a.dispatchEvent(click);
    expect(click.defaultPrevented).toBe(true);
    const opener = await import("@tauri-apps/plugin-opener");
    await vi.waitFor(() => expect(opener.openUrl).toHaveBeenCalledWith("https://example.test/a"));
  });
});
```

`view.dispatch(tr.insertText(…))` stands in for typing because jsdom's `contenteditable` does not
run the browser's text input path that ProseMirror observes (**UNVERIFIED** whether
`user-event` 14.6.7 drives it well enough; if it does, step 3 may switch to `user.type`). The
key-propagation half is still a real `keydown` on the view's DOM, which is what task 53's list
would receive. `insertText` at the default selection inserts at the document start, so
`"abcdestart"` is the expected text.

| Test | Mutant it kills |
|---|---|
| shape and a word | colour-only authorship on an agent passage |
| plain keys stay in the editor | a `keys.list` leak into text fields (`C` creating an issue while typing) |
| Ctrl+Enter commits here | a *New session* created by committing a description (`useKeyboardMap.ts:208`) |
| one write per blur | per-keystroke writes; a write on a blur with no change |
| the idle cadence | a timer that never fires, or one per keystroke that fires many |
| never rewrites style | a commit on open that rewrites `_em_` as `*em*` and digests a new revision for nothing |
| opens outside the window | a webview navigation away from the app on a link click |

#### Steps

- [ ] **Step 1: read on the sync base** task 46's `AuthorClass.tsx` (`AUTHOR_PATH` exported),
  task 47's `IssueView.tsx` and `links.ts`, and task 56's `WorkOp` `revise` shape.
- [ ] **Step 2: install** the eleven packages with `--save-exact` (one `npm` command, the list in the
  table above), then read each `license` from `desktop/package-lock.json`, the installed
  `prosemirror-markdown`'s `schema.spec.nodes.get("bullet_list").attrs`, its
  `defaultMarkdownParser.tokens.bullet_list`, `@tauri-apps/plugin-opener`'s `openUrl` export, and
  whether `markdown-it` ships its own types (if so, drop `@types/markdown-it`).
- [ ] **Step 3: the three test files and the seven fixtures first**; run them and see them fail.
- [ ] **Step 4: `schema.ts`, `markdown.ts`**; `markdown.test.ts` passes. Correct a fixture only
  toward what CommonMark requires, never toward what the serializer emits.
- [ ] **Step 5: `authorClass.ts`, `inputRules.ts`**; `authorClass.test.ts` passes.
- [ ] **Step 6: `Editor.tsx`, `editor.css`**, then the `IssueView.tsx` edit; the editor tests and
  tasks 46-48's tests pass.
- [ ] **Step 7: commit** by path: `package.json` and `package-lock.json` together first, then one
  commit per step 4-6, each message ending with the `Co-Authored-By` line.

#### Acceptance

```bash
npm --prefix desktop test -- editor issue
# expect: every file passes; this part adds 3 files and 19 tests (markdown 7, authorClass 5, Editor 7)
npm --prefix desktop run typecheck
# expect: exit 0
grep -rn "html: true\|dangerouslySetInnerHTML\|innerHTML" desktop/src/components/editor/ | wc -l
# expect: 0
node -e "const l=require('./desktop/package-lock.json').packages;for(const k of Object.keys(l).filter(k=>/node_modules\/(prosemirror-|markdown-it$)/.test(k)))console.log(k,l[k].license)"
# expect: each line ends in MIT
```

The last command lists every `prosemirror-*` and `markdown-it` entry, transitive ones included
(`orderedmap`, `rope-sequence`, `w3c-keyname`, `linkify-it`, `mdurl`, `uc.micro`, `entities`,
`punycode.js` are expected under them; their licences are read, not assumed, and any that is not
MIT, Apache-2.0, BSD or ISC stops the task). Visual acceptance is the owner's eyes.

#### What this task does NOT claim

- Multiplayer editing (`L-121`, team mode's TM2), drafts (`L-33`, W2), agent edits shown as
  proposals (`L-130`, W4), and the Discover brief editor (phase 4's).
- That per-passage authorship survives a reload or reaches the Hub: the wire has one class per
  field (decision 1; gap 1). Within one editing session it is exact.
- The slash menu and tables (task 50); callouts (task 92).
- That the 3 s cadence is right: it is a proposal for the owner.
- That typing in a real WebView2 behaves as `view.dispatch` does in jsdom; the owner's hands on
  the build are the check.
- That issue text reaching an agent is safe: that is 05 §3.4's adoption rule and the Hub's
  checks (task 19), not the editor's.

**Parity** (`05-design.md` rows). Closes **L-124** (`05:268`: every text keeps its author class;
here per passage in the editor, per field on the wire) and **L-126**'s Markdown half (`05:270`:
shortcuts as input rules; the slash menu is task 50). Serves **L-34** (`05:143`): each commit is
one revision (task 47).

#### Gaps found while writing this part

1. **Per-passage authorship has no wire form.** `AuthoredText` is one class per field
   (`task-02.md:237-242`); carrying spans means changing task 02's canonical `issue-revision` form
   and its golden vectors, the Hub-contract owner's call. Until then a mixed text is `A`.
2. **`Ctrl+Enter` is taken.** The scope's commit key is the cockpit's *New session* chord
   (`useKeyboardMap.ts:55`, `:208`); this part stops its propagation inside the editor, which task
   53's keymap must know (a chord that works everywhere except in a text field).
3. **SW.12 has no author mark**, so the gutter draws task 46's `AUTHOR_PATH` (task 46's gap 5).
4. **`IssueView` gains `onOpenIssue`**, which its mount (task 40) must wire.
5. Task 50's slash menu has to mount inside `Editor.tsx`, which the scope's file list for task 50
   does not name; task 50's part edits it.
6. The File map lists `src/components/editor/*` without the files; `inputRules.ts` and the
   `fixtures/` directory are this part's additions.


