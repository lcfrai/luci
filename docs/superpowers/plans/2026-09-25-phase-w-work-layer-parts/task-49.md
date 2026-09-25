### Task 49: the editor, part 1: one Markdown editor on `prosemirror-*` (MIT), author class kept

Stream **WD3**, about 3 hours (an estimate, not a measurement). Depends on task 46 (`AuthorClass`,
the issue view it mounts in). Uses SW.1's `AuthoredText` (`task-02.md:224-229`) and SW.16's
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
   `AuthoredText` is `{text, author}` (`task-02.md:224-229`): one class for the whole description.
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
- `desktop/package.json` ⚑, `desktop/package-lock.json` — nine `prosemirror-*` packages and
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
  nodes: base.spec.nodes.remove("image").update("list_item", listItem).update("code_block", codeBlock),
  marks: base.spec.marks.update("link", link).addToEnd("agent", agent),
});
```

`base.spec.nodes` is an `OrderedMap` (`prosemirror-model`), whose `remove`, `update` and
`addToEnd` return new maps, so the base schema is untouched. The `link` guard in `toDOM` is the
second line of defence: the parser already refuses a dead scheme, and a document built any other
way (a paste, a future import) still cannot render one as an anchor. Task 50 adds `table`,
`table_row`, `table_cell` and `table_header` to this schema.


