### Task 50: the editor, part 2: the slash menu and tables

Stream **WD3**, about 3 hours (an estimate, not a measurement). Depends on task 49 (the schema,
`markdown.ts`, `SHORTCUTS`, `Editor.tsx`). Uses task 47's `issueLink` and SW.14's criterion
words. The menu is how a person finds what the editor can do; the shortcuts are how they stop
needing it. So the menu **teaches the shortcuts**: each item shows the Markdown that does the same
thing, read from the one table the input rules are built from (task 49's `SHORTCUTS`), and the two
cannot drift apart (L-226's rule, inside the editor).

Six rules, each one a test:

1. **`/` opens the menu only at the start of a block or after whitespace.** A slash inside a word
   (`and/or`, a path) is a slash.
2. **Typing filters; `Enter` inserts; `Esc` closes and leaves the literal `/`.** The slash is
   typed into the document like any character, so closing takes nothing away.
3. **Every hint is its input rule's trigger**, from `SHORTCUTS`. An item with no Markdown form
   (table, the two links) shows no hint rather than an invented one.
4. **Links are by id.** *Link an issue* inserts `panacea://issue/is-…` shown as the alias;
   *Link a criterion* inserts the criterion's key shown as its words. Never the alias as a target.
5. **Tables are GFM pipe tables**, parsed by `markdown-it` and written back by our serializer, so a
   table re-parses to an equal document. `Tab` and `Shift+Tab` move between cells.
6. **Opening the menu does not scroll the document.** It is placed at the caret with fixed
   positioning; nothing calls `scrollIntoView` or moves focus.

**Decisions made here, so nobody re-derives them:**

1. **The menu is a listbox the editor points at.** Focus stays in the editor (the person keeps
   typing to filter); the editor's root carries `aria-haspopup="listbox"` always and, while the
   menu is open, `aria-controls` naming the list and `aria-activedescendant` naming the active
   option. WAI-ARIA 1.2 lists `textbox` among the roles for both; it does **not** list `textbox`
   for `aria-expanded`, so that attribute is not used. The list is `role="listbox"`, each item
   `role="option"`.
2. **The link items take their candidates from the caller.** The editor knows no project and
   reads no replica; `EditorProps` gains `linkSources?: {issues(q); criteria(q)}`, which the issue
   view supplies (task 58's `search` and the replica's criteria read, task 47's gap 2). With no
   `linkSources`, the two link items are not listed, so comments (task 51) get them only when
   task 51 passes the same sources.
3. **A criterion link is `panacea://criterion/<briefKey>/<criterionId>`**, added to task 47's
   `links.ts` as `criterionLink`, because task 49's live-link rule admits `panacea:` and not a
   hash; the thread node's hash `href` (task 47 decision 5) is a different thing, an in-app anchor.
   What opening a criterion link does is Discover's (phase 4) and not claimed.
4. **`prosemirror-tables` (MIT) supplies the table nodes and cell navigation**; cells hold inline
   text only (`cellContent: "text*"`, marks allowed; no `hard_break`, which a GFM cell cannot
   hold), so nothing a person builds in the editor fails to serialize. A `|` inside a cell is
   written `\|`. Every table is kept **GFM-shaped** by a plugin: the first row is header cells, no
   other row is, and no cell spans (a pasted HTML table is flattened on the way in).
5. **Task 49's `Editor.tsx` is edited** to mount the menu (the scope's file list omits it; task
   49's gap 5). The edit is additive: a plugin, a state hook and one child element.

**Files:**
- `desktop/src/components/editor/slashMenu.ts` — `SlashItem`, `SLASH_ITEMS`, the plugin that opens,
  filters and runs them (new)
- `desktop/src/components/editor/SlashMenu.tsx` — the listbox at the caret (new)
- `desktop/src/components/editor/tables.ts` — the table nodes, the parser tokens, the serializer,
  the row and column commands (new)
- `desktop/src/components/editor/schema.ts` ⚑, `markdown.ts` ⚑ (task 49's) — the table nodes and
  the `table` rule, the table tokens and serializer merged in (modified)
- `desktop/src/components/editor/Editor.tsx` ⚑ (task 49's) — `linkSources`, the menu mounted,
  `tableEditing` and the cell keys (modified)
- `desktop/src/work/links.ts` ⚑ (task 47's) — `criterionLink`, `parseCriterionLink` (modified)
- `desktop/src/components/issue/IssueView.tsx` ⚑ (tasks 46-49's) — passes `linkSources` to the
  description's editor (modified; not in the scope's list)
- `desktop/src/components/editor/editor.css` ⚑ — `.ed-slash*` and `.ed-root table` rules (modified)
- `desktop/src/components/editor/{slashMenu,tables}.test.ts`, `SlashMenu.test.tsx` — tests (new)
- `desktop/package.json` ⚑, `desktop/package-lock.json` — `prosemirror-tables` (MIT, version
  **UNVERIFIED** until install; its licence read from the lock file by task 49's licence test,
  which this task extends by one name) (modified)

#### The dependency — `desktop/package.json` ⚑

| Package | Licence | Why |
|---|---|---|
| `prosemirror-tables` | MIT | `tableNodes`, `tableEditing` (cell selection and `fixTables`), `goToNextCell`, the row and column commands |

Its version is **UNVERIFIED** (not installed in this worktree while writing). Step 2 installs it
with `--save-exact` and reads its `license` from `desktop/package-lock.json`; task 49's licence
test gains the name. Its own stylesheet is not imported; `editor.css` styles the two classes it
sets (`selectedCell`, `column-resize-handle`, the second unused since resizing is off).

#### Interfaces — complete: `desktop/src/components/editor/tables.ts`, part 1: nodes and shape

```ts
// GFM pipe tables (decision 4 and rule 5): nodes, parser tokens, serializer, commands.
import type MarkdownIt from "markdown-it";
import type { MarkdownSerializerState } from "prosemirror-markdown";
import type { Node, NodeSpec, Schema } from "prosemirror-model";
import { Plugin, type EditorState } from "prosemirror-state";
import { goToNextCell, isInTable, selectedRect, tableNodes } from "prosemirror-tables";

type Token = ReturnType<MarkdownIt["parse"]>[number];
export type Align = "left" | "center" | "right" | null;

export const alignOf = (style: string | null | undefined): Align =>
  (/text-align:\s*(left|center|right)/.exec(style ?? "")?.[1] as Align | undefined) ?? null;

/** A cell never spans: a pasted colspan/rowspan is read as 1, and fixTables fills the gap. */
function flat(spec: NodeSpec): NodeSpec {
  return {
    ...spec,
    parseDOM: spec.parseDOM?.map((rule) => ({
      ...rule,
      getAttrs: (dom: HTMLElement | string) => {
        const a = typeof rule.getAttrs === "function" ? rule.getAttrs(dom as HTMLElement) : null;
        return a === false ? false : { ...(a ?? {}), colspan: 1, rowspan: 1, colwidth: null };
      },
    })),
  };
}

const specs = tableNodes({
  tableGroup: "block",
  cellContent: "text*",                           // decision 4: no hard_break in a GFM cell
  cellAttributes: {
    align: {
      default: null,
      getFromDOM: (dom) => alignOf((dom as HTMLElement).getAttribute("style")),
      setDOMAttr: (value, attrs) => { if (value) attrs.style = `text-align: ${String(value)}`; },
    },
  },
});

/** The four nodes task 49's schema adds with `addToEnd` (see the schema edit below). */
export const tableNodeSpecs: Record<"table" | "table_row" | "table_cell" | "table_header", NodeSpec> = {
  table: specs.table,
  table_row: specs.table_row,
  table_cell: flat(specs.table_cell),
  table_header: flat(specs.table_header),
};

/** Decision 4: after every change, row 0 is header cells, no other row is, nothing spans, and
 *  every cell carries its column's alignment (GFM keeps alignment per column, not per cell). */
export const gfmShapePlugin = new Plugin({
  appendTransaction(trs, _old, state) {
    if (!trs.some((t) => t.docChanged)) return null;
    const { table, table_header: th, table_cell: td } = state.schema.nodes;
    const tr = state.tr;
    state.doc.descendants((node, pos) => {
      if (node.type !== table) return true;
      const aligns: Align[] = [];
      node.firstChild?.forEach((c) => { aligns.push(c.attrs.align as Align); });
      node.forEach((row, rowOff, r) => {
        row.forEach((cell, cellOff, c) => {
          const type = r === 0 ? th : td;
          const align = r === 0 ? (cell.attrs.align as Align) : aligns[c] ?? null;
          const a = cell.attrs;
          if (cell.type === type && a.colspan === 1 && a.rowspan === 1 && a.align === align) return;
          tr.setNodeMarkup(pos + 1 + rowOff + 1 + cellOff, type, { ...a, colspan: 1, rowspan: 1, align });
        });
      });
      return false;
    });
    return tr.docChanged ? tr : null;
  },
});
```

`setNodeMarkup` changes no sizes, so every position computed in the walk stays valid. A row left
short by flattening is filled by `tableEditing`'s own `fixTables`, which runs in the same
`appendTransaction` round. The `getFromDOM`/`setDOMAttr` names and `tableNodes`' option shape
are `prosemirror-tables`' as this plan understands them, **UNVERIFIED** until step 2 reads the
installed types.

#### Interfaces — complete: `tables.ts`, part 2: parse, serialize, commands

```ts
/** markdown-it's table tokens (its `table` rule, which makeTokenizer now enables) to our nodes.
 *  `ignore` drops thead/tbody open and close tokens and keeps what is between them. */
const cellAttrs = (tok: Token) => ({ align: alignOf(tok.attrGet("style")) });
export const tableParserTokens = {
  table: { block: "table" },
  thead: { ignore: true },
  tbody: { ignore: true },
  tr: { block: "table_row" },
  th: { block: "table_header", getAttrs: cellAttrs },
  td: { block: "table_cell", getAttrs: cellAttrs },
};

const DELIM: Record<string, string> = { left: ":---", center: ":---:", right: "---:", none: "---" };
/** GFM: a pipe in a cell is escaped, inside code spans too (markdown-it removes the backslash
 *  before the cell's inline parse). */
export const escapeCell = (s: string): string => s.replace(/\|/g, "\\|");

/** `inline` renders one cell's content to Markdown (markdown.ts passes its own serializer, so a
 *  cell's marks, links and authorship sentinels are written exactly as in a paragraph). */
export function tableSerializer(inline: (cell: Node) => string) {
  return (state: MarkdownSerializerState, node: Node): void => {
    const rows: string[][] = [];
    node.forEach((row) => {
      const cells: string[] = [];
      row.forEach((cell) => { cells.push(escapeCell(inline(cell))); });
      rows.push(cells);
    });
    const aligns: string[] = [];
    node.firstChild?.forEach((c) => { aligns.push(DELIM[(c.attrs.align as string | null) ?? "none"]); });
    const lines = [rows[0], aligns, ...rows.slice(1)].map((cells) => `| ${cells.join(" | ")} |`);
    lines.forEach((l, i) => { if (i > 0) state.ensureNewLine(); state.write(l); });
    state.closeBlock(node);
  };
}

/** A fresh table: a header row and `bodyRows` body rows of `cols` empty cells. */
export function createTable(schema: Schema, cols = 3, bodyRows = 1): Node {
  const { table, table_row: tr, table_header: th, table_cell: td } = schema.nodes;
  const row = (type: typeof th) => tr.create(null, Array.from({ length: cols }, () => type.create()));
  return table.create(null, [row(th), ...Array.from({ length: bodyRows }, () => row(td))]);
}

/** Where the row and column items may run (the menu lists only what can run). */
export const inTable = (s: EditorState): boolean => isInTable(s);
export const inBodyRow = (s: EditorState): boolean => isInTable(s) && selectedRect(s).top > 0;
export const hasColumns = (s: EditorState): boolean => isInTable(s) && selectedRect(s).map.width > 1;

/** Tab and Shift+Tab between cells. Outside a table both return false, so Tab moves focus out of
 *  the editor as everywhere else (no focus trap); at the last cell, too. */
export const cellKeys = { Tab: goToNextCell(1), "Shift-Tab": goToNextCell(-1) };
```

`state.write` is called once per line, after `ensureNewLine`, so a table inside a blockquote or a
list item gets the container's prefix on every line (`write` adds the delimiter at a blank line
start). The delimiter row is written from the **header** cells' alignment only, which is why
`gfmShapePlugin` copies it down the column: a body cell with its own alignment would re-parse as
its column's and fail rule 5. Escaping `|` after rendering touches text and code alike, which is
what GFM requires; a person's `\|` is written `\\\|`, which markdown-it reads back as `\|`
(**UNVERIFIED** against the installed markdown-it until the fixture `tables.md` runs).

#### Interfaces — complete: the edits to task 49's `schema.ts` and `markdown.ts`

```ts
// schema.ts — the nodes, added last so every existing node keeps its position in the map.
import { tableNodeSpecs } from "./tables";
export const editorSchema = new Schema({
  nodes: base.spec.nodes.remove("image").update("list_item", listItem)
    .update("code_block", codeBlock).update("bullet_list", bulletList)
    .append(tableNodeSpecs),
  marks: base.spec.marks.update("link", link).addToEnd("agent", agent),
});

// markdown.ts — the tokenizer enables GFM tables; the parser and serializer learn them.
import { tableParserTokens, tableSerializer } from "./tables";
//   in makeTokenizer(), after md.disable(["image"]):
md.enable(["table"]);
//   parserTokens gains the six table entries:
export const parserTokens = { ...baseTokens, ...tableParserTokens, bullet_list: { /* as task 49 */ },
  list_item: { /* as task 49 */ } };
//   serializerNodes gains the table (the three row and cell nodes are written by it, never alone):
table: tableSerializer((cell) =>
  serializer.serialize(editorSchema.nodes.doc.create(null, editorSchema.nodes.paragraph.create(null, cell.content)))),
```

`OrderedMap.append` takes a plain object as well as a map (`prosemirror-model`'s `orderedmap`,
MIT). The cell renders through the real serializer inside a one-paragraph document, so a cell's
`em`, `code`, `link` and `agent` marks are written by the same rules as anywhere else, and the
sentinels inside a table are stripped by `serializeWithAuthorship` like any others. Referencing
`serializer` inside `serializerNodes` is safe because the function runs only at serialize time,
after the constant exists. Escaping at a paragraph's line start may write `\-` for a cell that
starts with `-`; that re-parses to the same text and is the serializer's, not this task's.

#### Interfaces — complete: `desktop/src/components/editor/slashMenu.ts`, part 1: the items

```ts
// The slash menu (rules 1-4, decision 1). Items are data; block items take their label AND their
// hint from task 49's SHORTCUTS, so the menu cannot teach a shortcut the editor does not have.
import { setBlockType, wrapIn } from "prosemirror-commands";
import { wrapInList } from "prosemirror-schema-list";
import { Plugin, PluginKey, TextSelection, type Command, type EditorState } from "prosemirror-state";
import { addColumnAfter, addRowAfter, deleteColumn, deleteRow, deleteTable } from "prosemirror-tables";
import { findWrapping } from "prosemirror-transform";
import type { EditorView } from "prosemirror-view";
import { criterionLink, issueLink } from "../../work/links";
import { SHORTCUTS, type ShortcutId } from "./inputRules";
import { editorSchema } from "./schema";
import { createTable, hasColumns, inBodyRow, inTable } from "./tables";

export interface IssueCandidate { id: string; alias: string; title: string }
export interface CriterionCandidate { briefKey: string; criterionId: string; text: string }
/** Decision 2: the caller's reads. The editor knows no project and reads no replica. */
export interface LinkSources {
  issues(q: string): readonly IssueCandidate[];
  criteria(q: string): readonly CriterionCandidate[];
}

export interface SlashItem {
  id: string;
  label: string;
  /** The Markdown that does the same, from SHORTCUTS; "" when there is none (rule 3). */
  shortcut: string;
  /** More words the filter matches ("ul", "todo"); never shown. */
  keywords?: readonly string[];
  /** Listed only while this holds (the row and column items: inside a table). */
  when?(state: EditorState): boolean;
  /** A link item opens a picker over this source instead of running (rule 4). */
  pick?: "issue" | "criterion";
  run(view: EditorView): void;
}

const n = editorSchema.nodes;
const cmd = (c: Command) => (v: EditorView): void => { c(v.state, v.dispatch, v); };

/** A block item whose label and hint are its SHORTCUTS row, by construction. */
function block(id: ShortcutId, c: Command, keywords: readonly string[] = []): SlashItem {
  const s = SHORTCUTS.find((x) => x.id === id)!;
  return { id, label: s.label, shortcut: s.trigger, keywords, run: cmd(c) };
}

/** A task list in one transaction, so one undo step (task 49's check rule, from a command). */
const toTaskList: Command = (state, dispatch) => {
  const range = state.selection.$from.blockRange();
  const wrap = range && findWrapping(range, n.bullet_list, { bullet: "-" });
  if (!range || !wrap) return false;
  wrap[wrap.length - 1] = { type: n.list_item, attrs: { checked: false } };
  dispatch?.(state.tr.wrap(range, wrap));
  return true;
};

/** An empty line becomes the rule; otherwise the rule goes after the line. A paragraph follows a
 *  rule that would end the document, so the caret has somewhere to go. */
const divider: Command = (state, dispatch) => {
  const $f = state.selection.$from;
  const empty = $f.parent.content.size === 0;
  const tr = empty ? state.tr.replaceRangeWith($f.before(), $f.after(), n.horizontal_rule.create())
    : state.tr.insert($f.after(), n.horizontal_rule.create());
  const end = tr.mapping.map($f.after());
  if (end >= tr.doc.content.size) tr.insert(tr.doc.content.size, n.paragraph.create());
  dispatch?.(tr.setSelection(TextSelection.near(tr.doc.resolve(Math.min(end + 1, tr.doc.content.size)))));
  return true;
};

/** Rule 5's table: a header row and one body row of three cells, the caret in the first header. */
const insertTable: Command = (state, dispatch) => {
  const $f = state.selection.$from;
  if ($f.parent.type.spec.code || inTable(state)) return false;
  const at = $f.parent.content.size === 0 ? $f.before() : $f.after();
  const tr = at === $f.before() ? state.tr.replaceWith(at, $f.after(), createTable(editorSchema))
    : state.tr.insert(at, createTable(editorSchema));
  dispatch?.(tr.setSelection(TextSelection.create(tr.doc, at + 3)));   // table, row, cell: +1 each
  return true;
};
```

#### Interfaces — complete: `slashMenu.ts`, part 2: the list and the filter

```ts
const none = (): void => {};                      // a pick item never runs; choose() opens its picker

export const SLASH_ITEMS: readonly SlashItem[] = Object.freeze([
  block("h1", setBlockType(n.heading, { level: 1 }), ["title"]),
  block("h2", setBlockType(n.heading, { level: 2 })),
  block("h3", setBlockType(n.heading, { level: 3 })),
  block("bullet", wrapInList(n.bullet_list, { bullet: "-" }), ["ul"]),
  block("ordered", wrapInList(n.ordered_list, { order: 1 }), ["ol"]),
  block("check", toTaskList, ["todo", "task"]),
  block("quote", wrapIn(n.blockquote), ["blockquote"]),
  block("code", setBlockType(n.code_block), ["pre"]),
  block("divider", divider, ["hr", "rule"]),
  { id: "table", label: "Table", shortcut: "", keywords: ["grid"], when: (s) => !inTable(s), run: cmd(insertTable) },
  { id: "row_add", label: "Add row below", shortcut: "", when: inTable, run: cmd(addRowAfter) },
  { id: "column_add", label: "Add column right", shortcut: "", when: inTable, run: cmd(addColumnAfter) },
  { id: "row_remove", label: "Remove row", shortcut: "", when: inBodyRow, run: cmd(deleteRow) },
  { id: "column_remove", label: "Remove column", shortcut: "", when: hasColumns, run: cmd(deleteColumn) },
  { id: "table_remove", label: "Remove table", shortcut: "", when: inTable, run: cmd(deleteTable) },
  { id: "link_issue", label: "Link an issue", shortcut: "", keywords: ["mention"], pick: "issue", run: none },
  { id: "link_criterion", label: "Link a criterion", shortcut: "", keywords: ["brief", "spec"], pick: "criterion", run: none },
]);

/** One row of the open list: an item, or a link candidate once a link item was chosen. */
export type SlashOption =
  | { kind: "item"; key: string; label: string; shortcut: string; item: SlashItem }
  | { kind: "issue"; key: string; label: string; fact: string; issue: IssueCandidate }
  | { kind: "criterion"; key: string; label: string; fact: string; criterion: CriterionCandidate };

export type SlashMode = "items" | "issue" | "criterion";
export type SlashState =
  | { open: false }
  | { open: true; from: number; query: string; mode: SlashMode; active: number };
export const CLOSED: SlashState = Object.freeze({ open: false });
export const slashKey = new PluginKey<SlashState>("ed-slash");

/** Candidates shown at once; more is a longer query, not a longer list. */
export const MAX_CANDIDATES = 8;

/** Every query word is a prefix of a label word or a keyword, case-insensitive. */
export function matches(item: SlashItem, query: string): boolean {
  const words = [...item.label.toLowerCase().split(/\s+/), ...(item.keywords ?? [])];
  return query.toLowerCase().split(/\s+/).filter(Boolean).every((q) => words.some((w) => w.startsWith(q)));
}

export function optionsFor(state: EditorState, s: SlashState, sources?: LinkSources): SlashOption[] {
  if (!s.open) return [];
  if (s.mode === "issue") {
    return (sources?.issues(s.query) ?? []).slice(0, MAX_CANDIDATES).map((i) =>
      ({ kind: "issue", key: `issue-${i.id}`, label: i.title, fact: i.alias, issue: i }));
  }
  if (s.mode === "criterion") {
    return (sources?.criteria(s.query) ?? []).slice(0, MAX_CANDIDATES).map((c) =>
      ({ kind: "criterion", key: `criterion-${c.criterionId}`, label: c.text, fact: c.criterionId, criterion: c }));
  }
  return SLASH_ITEMS
    .filter((it) => (!it.when || it.when(state)) && (!it.pick || sources !== undefined) && matches(it, s.query))
    .map((it) => ({ kind: "item", key: it.id, label: it.label, shortcut: it.shortcut, item: it }));
}
```

`block()` makes the label and the hint one lookup, so a new row in `SHORTCUTS` with no menu item,
or a menu item with no row, is caught by the parity test below rather than by a reader. The two
Markdown shortcuts that are marks (`**`, `` ` ``) have no menu item: the menu inserts blocks and
links, and a mark is typed around text that already exists. `tab` matches *Table* only while the
caret is outside a table (inside one, *Remove table* is listed and matches too, which is right).

#### Interfaces — complete: `slashMenu.ts`, part 3: the plugin

```ts
/** Rule 1: at a textblock's start or after whitespace; never in a code block or inline code. */
export function opensAt(state: EditorState, pos: number): boolean {
  const $p = state.doc.resolve(pos);
  if (!$p.parent.isTextblock || $p.parent.type.spec.code) return false;
  if ($p.marks().some((m) => m.type.spec.code)) return false;
  const before = $p.parent.textBetween(0, $p.parentOffset, undefined, "￼");
  return before === "" || /\s$/.test(before);
}

/** Follow the open menu through a change: the "/" must survive, the caret must stay after it in
 *  the same block, the selection must be empty; the query is the text between. */
function follow(state: EditorState, prev: Extract<SlashState, { open: true }>, from: number): SlashState {
  const sel = state.selection;
  if (!sel.empty || state.doc.textBetween(from, Math.min(from + 1, state.doc.content.size)) !== "/") return CLOSED;
  if (sel.head <= from || state.doc.resolve(from).parent !== sel.$head.parent) return CLOSED;
  const query = state.doc.textBetween(from + 1, sel.head);
  if (prev.mode === "items" && /\s/.test(query)) return CLOSED;   // "/ " is a slash and a space
  return { ...prev, from, query, active: query === prev.query ? prev.active : 0 };
}

export function choose(view: EditorView, s: SlashState, opt: SlashOption | undefined): void {
  if (!s.open || !opt) return;
  const { state } = view;
  const head = state.selection.head;
  if (opt.kind === "item" && opt.item.pick) {     // keep the "/", drop the query, open the picker
    view.dispatch(state.tr.delete(s.from + 1, head)
      .setMeta(slashKey, { ...s, query: "", mode: opt.item.pick, active: 0 }));
    return;
  }
  if (opt.kind === "item") {
    view.dispatch(state.tr.delete(s.from, head).setMeta(slashKey, CLOSED));
    opt.item.run(view);
    return;
  }
  // Rule 4: the target is the id; the alias or the criterion's words are only the shown text.
  const href = opt.kind === "issue" ? issueLink(opt.issue.id)
    : criterionLink(opt.criterion.briefKey, opt.criterion.criterionId);
  const text = opt.kind === "issue" ? opt.issue.alias : opt.criterion.text;
  const link = editorSchema.marks.link.create({ href, title: null });
  view.dispatch(state.tr.replaceWith(s.from, head, editorSchema.text(text, [link]))
    .removeStoredMark(editorSchema.marks.link).setMeta(slashKey, CLOSED));
}

export interface SlashConfig { listId: string; sources(): LinkSources | undefined }
export const optionId = (listId: string, o: SlashOption): string => `${listId}-${o.key}`;

export function slashMenuPlugin(cfg: SlashConfig): Plugin<SlashState> {
  const open = (st: EditorState) => { const s = slashKey.getState(st); return s?.open ? s : null; };
  return new Plugin<SlashState>({
    key: slashKey,
    state: {
      init: () => CLOSED,
      apply(tr, prev, _old, state) {
        const meta = tr.getMeta(slashKey) as SlashState | undefined;
        if (meta) return meta;
        if (!prev.open || (!tr.docChanged && !tr.selectionSet)) return prev;
        const mapped = tr.mapping.mapResult(prev.from, 1);
        return mapped.deleted ? CLOSED : follow(state, prev, mapped.pos);
      },
    },
    props: {
      handleTextInput(view, from, to, text) {
        if (text !== "/" || open(view.state) || !opensAt(view.state, from)) return false;
        // Rule 6: no scrollIntoView on this transaction; the caret is where the person typed.
        view.dispatch(view.state.tr.insertText("/", from, to)
          .setMeta(slashKey, { open: true, from, query: "", mode: "items", active: 0 }));
        return true;
      },
      handleKeyDown(view, e) {
        const s = open(view.state);
        if (!s) return false;
        const opts = optionsFor(view.state, s, cfg.sources());
        const step = e.key === "ArrowDown" ? 1 : e.key === "ArrowUp" ? -1 : 0;
        if (step !== 0 && opts.length > 0) {
          const active = (s.active + step + opts.length) % opts.length;
          view.dispatch(view.state.tr.setMeta(slashKey, { ...s, active }));
          return true;
        }
        if (e.key === "Escape") { view.dispatch(view.state.tr.setMeta(slashKey, CLOSED)); return true; }
        if (e.key !== "Enter") return false;
        if (opts.length === 0) { view.dispatch(view.state.tr.setMeta(slashKey, CLOSED)); return false; }
        choose(view, s, opts[s.active]);
        return true;
      },
      attributes(state) {
        const s = open(state);
        if (!s) return { "aria-haspopup": "listbox" };
        const active = optionsFor(state, s, cfg.sources())[s.active];
        return { "aria-haspopup": "listbox", "aria-controls": cfg.listId,
          ...(active ? { "aria-activedescendant": optionId(cfg.listId, active) } : {}) };
      },
    },
  });
}
```

`follow` maps the slash's position with `assoc 1`, so text typed before the slash moves it and
text deleted over it closes the menu. Closing is always a state change, never a document change:
`Esc`, a space in the query, a click elsewhere and an arrow out of the block all leave `/` and
the query in the text exactly as typed (rule 2). `Enter` with nothing listed closes the menu and
falls through, so it still splits the line.

#### Interfaces — complete: `desktop/src/components/editor/SlashMenu.tsx`

```tsx
import { useLayoutEffect, type JSX } from "react";
import { createPortal } from "react-dom";
import type { EditorView } from "prosemirror-view";
import { choose, optionId, optionsFor, slashKey, type LinkSources, type SlashState } from "./slashMenu";

/** The caret on screen; null where there is no layout (jsdom has no Range rects). */
export function caretBox(view: EditorView, pos: number): { left: number; top: number; bottom: number } | null {
  try { const c = view.coordsAtPos(pos); return { left: c.left, top: c.top, bottom: c.bottom }; }
  catch { return null; }
}

const HEADINGS = { items: "Insert", issue: "Link an issue", criterion: "Link a criterion" } as const;

export interface SlashMenuProps { view: EditorView; state: SlashState; listId: string; sources?: LinkSources }

/** Rule 6: fixed at the caret, in a portal on <body> (a transformed panel would otherwise become
 *  its containing block), and nothing here scrolls or takes focus. */
export function SlashMenu({ view, state, listId, sources }: SlashMenuProps): JSX.Element | null {
  // The active option is kept in view by scrolling the LIST's own box, never scrollIntoView,
  // which would also scroll every scrollable ancestor, the document included (rule 6).
  useLayoutEffect(() => {
    const li = view.dom.getAttribute("aria-activedescendant");
    const el = li ? document.getElementById(li) : null;
    const ul = el?.parentElement;
    if (!el || !ul) return;
    if (el.offsetTop < ul.scrollTop) ul.scrollTop = el.offsetTop;
    else if (el.offsetTop + el.offsetHeight > ul.scrollTop + ul.clientHeight) {
      ul.scrollTop = el.offsetTop + el.offsetHeight - ul.clientHeight;
    }
  });
  if (!state.open) return null;
  const opts = optionsFor(view.state, state, sources);
  const box = caretBox(view, state.from);
  const above = box !== null && box.bottom > window.innerHeight * 0.6;
  const style = box ? { left: box.left, top: above ? box.top - 4 : box.bottom + 4 } : undefined;
  const none = state.mode === "items" ? `Nothing matches /${state.query}. Esc keeps the text.`
    : `No ${state.mode} matches “${state.query}”.`;
  return createPortal(
    <div className="ed-slash" data-above={above || undefined} style={style}>
      <ul id={listId} role="listbox" aria-label={HEADINGS[state.mode]} className="ed-slash-list">
        {opts.map((o, i) => (
          <li key={o.key} id={optionId(listId, o)} role="option" aria-selected={i === state.active}
            className="ed-slash-opt"
            // mousedown, prevented: a click must not blur the editor (a blur commits, task 49)
            onMouseDown={(e) => { e.preventDefault(); choose(view, slashKey.getState(view.state) ?? state, o); }}>
            {o.kind !== "item" && <span className="ed-slash-fact">{o.fact}</span>}
            <span className="ed-slash-label">{o.label}</span>
            {o.kind === "item" && o.shortcut !== "" && <kbd className="ed-slash-kbd">{o.shortcut}</kbd>}
          </li>
        ))}
        {opts.length === 0 && <li className="ed-slash-none" role="presentation">{none}</li>}
      </ul>
    </div>,
    document.body,
  );
}
```

The empty line is words, not a blank list: a person who typed `/xyz` learns the menu is open and
how to leave it. A candidate row puts the fact first in mono (`EP-142`, the criterion's id) and
the words after in sans, the house split (`voice.split`, 02 C3).

#### Interfaces — complete: the edit to task 47's `desktop/src/work/links.ts`

```ts
// Decision 3: a criterion link. The brief key is the Hub's grammar; a criterionId is any 1-128
// characters (task-02.md:212, :270), so it is percent-encoded and decoded back exactly.
const CRITERION_PREFIX = "panacea://criterion/";
const BRIEF_KEY = /^brief:bf-[0-9a-f]{16}@[1-9][0-9]{0,8}$/;

export interface CriterionLinkTarget { briefKey: string; criterionId: string }

export function criterionLink(briefKey: string, criterionId: string): string {
  if (!BRIEF_KEY.test(briefKey)) throw new Error(`criterionLink: not a brief key: ${JSON.stringify(briefKey)}`);
  if (criterionId.length < 1 || [...criterionId].length > 128) throw new Error("criterionLink: criterionId is 1 to 128 characters");
  return `${CRITERION_PREFIX}${briefKey}/${encodeURIComponent(criterionId)}`;
}

export function parseCriterionLink(s: string): CriterionLinkTarget | null {
  if (!s.startsWith(CRITERION_PREFIX)) return null;
  const rest = s.slice(CRITERION_PREFIX.length);
  const slash = rest.indexOf("/");
  if (slash < 0 || !BRIEF_KEY.test(rest.slice(0, slash))) return null;
  try {
    const criterionId = decodeURIComponent(rest.slice(slash + 1));
    return criterionId.length > 0 ? { briefKey: rest.slice(0, slash), criterionId } : null;
  } catch { return null; }                        // a malformed %-escape is not a link
}
```

`BRIEF_KEY` is the Hub's `briefKeyRe` character for character (read in the Hub worktree,
`control-plane/internal/hub/hubapi/keys.go:57`). Task 49's `openLink` does nothing with a
criterion link (it is neither an issue link nor `https:`), so a click on one in read-only mode is
prevented and goes nowhere until Discover (phase 4) says what it opens; that is stated, not hidden.

#### Interfaces — complete: the edit to task 49's `Editor.tsx`

```tsx
import { useId, useState } from "react";
import { tableEditing } from "prosemirror-tables";
import { SlashMenu } from "./SlashMenu";
import { CLOSED, slashKey, slashMenuPlugin, type LinkSources, type SlashState } from "./slashMenu";
import { cellKeys, gfmShapePlugin } from "./tables";

export interface EditorProps {
  /* task 49's five props, unchanged */
  /** Decision 2: the link items are listed only when the caller passes these. */
  linkSources?: LinkSources;
}

//   in Editor(), beside the other refs:
const listId = `${useId()}-slash`;
const [slash, setSlash] = useState<SlashState>(CLOSED);
const menuOpen = (): boolean => (view.current ? slashKey.getState(view.current.state)?.open === true : false);
/** The idle commit waits while the menu is open: "/ta" is not a revision anyone meant. */
const idleCommit = (): void => {
  if (menuOpen()) { idle.current = setTimeout(idleCommit, IDLE_COMMIT_MS); return; }
  commit();
};

//   in load(), the plugin list becomes (the slash plugin first: while open, its keys win):
plugins: [
  slashMenuPlugin({ listId, sources: () => latest.current.linkSources }),
  tableEditing(), gfmShapePlugin, keymap(cellKeys),
  buildInputRules(editorSchema), personTypedPlugin, agentGutterPlugin, history(),
  keymap({ /* task 49's keys, unchanged */ }),
  keymap(baseKeymap),
],

//   in dispatchTransaction, after v.updateState(...), and the timer now arms idleCommit:
const next = slashKey.getState(v.state) ?? CLOSED;
setSlash((prev) => (prev === next ? prev : next));
if (!tr.docChanged) return;
if (idle.current) clearTimeout(idle.current);
idle.current = setTimeout(idleCommit, IDLE_COMMIT_MS);

//   handleDOMEvents.blur closes the menu before committing (a blur is leaving):
blur: () => {
  if (menuOpen()) view.current!.dispatch(view.current!.state.tr.setMeta(slashKey, CLOSED));
  commit();
  return false;
},

//   the render: the host PM owns, and the menu beside it in a portal
return (
  <>
    <div ref={host} className="ed" data-verbatim data-readonly={readOnly || undefined} />
    {view.current && <SlashMenu view={view.current} state={slash} listId={listId} sources={props.linkSources} />}
  </>
);
```

The plugin state is the one source of truth; `slash` is React's copy so the menu re-renders, set
only when the plugin's state object changes (it is replaced, never mutated). The host `div` keeps
no React children because ProseMirror owns its DOM. `useId` is stable across renders, so the id
built into the plugin at `load` is the list's id every time. A `readOnly` editor never opens the
menu: `handleTextInput` is not called when the view is not editable.

#### Interfaces — complete: the edit to `IssueView.tsx` (the link sources)

```tsx
import { readWork, search } from "../../work/replica/replica";
import type { LinkSources } from "../editor/slashMenu";
//   in IssueViewBody, beside task 49's Editor, and passed as linkSources={sources}:
const sources = useMemo<LinkSources>(() => ({
  issues: (q) => readWork((r) => search(r, q)).filter((h) => h.id !== head.id),
  criteria: (q) => readWork((r) => r.criteria(head.projectId, q)),
}), [head.id, head.projectId]);
```

The sources read the replica **when the menu asks**, not on every render: `readWork` is a
non-subscribing read of SW.16's store (the `useWork` hook's snapshot without the subscription).
Neither `readWork` nor `r.criteria(projectId, q)` exists in any part yet; both are gaps for tasks
55-58, and `search(r, q)` is used with task 46's assumed signature (`task-46.md:309-311`). An
issue never offers itself as a link target. `head.projectId` is the record's (`task-46.md:491`).

#### Interfaces — complete: the rules appended to `editor.css`

```css
/* Task 50: the slash menu and tables. Still .ed-* only (plus prosemirror-tables' selectedCell). */
.ed-slash { position: fixed; z-index: 40; min-inline-size: 240px; max-inline-size: 360px;
  padding: var(--s-1); background: var(--surface); border-radius: var(--r-card);
  box-shadow: var(--e-raised); }
.ed-slash[data-above] { transform: translateY(-100%); }
.ed-slash-list { position: relative; /* the options' offsetParent, for the list's own scroll */
  margin: 0; padding: 0; list-style: none; max-block-size: 320px; overflow-y: auto; }
.ed-slash-opt { display: flex; align-items: baseline; gap: var(--s-2); padding: var(--s-1) var(--s-2);
  border-radius: var(--r-ctl); color: var(--ink); font-family: var(--sans); cursor: default; }
.ed-slash-opt[aria-selected="true"] { background: var(--hover); box-shadow: inset 2px 0 0 var(--ink); }
.ed-slash-label { flex: 1; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.ed-slash-fact { font-family: var(--mono); font-size: var(--t-fact); color: var(--ink-2); }
.ed-slash-kbd { font-family: var(--mono); font-size: var(--t-micro); color: var(--ink-2);
  white-space: pre; padding-inline: var(--s-1); border-radius: var(--r-chip); box-shadow: var(--e-hair); }
.ed-slash-none { padding: var(--s-1) var(--s-2); color: var(--ink-2); }
.ed-root table { border-collapse: collapse; margin-block: var(--s-2); }
.ed-root th, .ed-root td { border: 1px solid var(--line); padding: var(--s-1) var(--s-2);
  min-inline-size: 4ch; vertical-align: top; }
.ed-root th { font-weight: 560; text-align: start; }
.ed-root .selectedCell { background: var(--hover); }
@media (forced-colors: active) {
  .ed-slash-opt[aria-selected="true"], .ed-root .selectedCell { outline: 1px solid Highlight; }
}
```

The tokens are the house's own: `--r-card` for a popover, `--r-ctl` for a row, `--r-chip` for a
`kbd` hint and `--e-raised` for the menu (`desktop/src/styles/tokens.css:110-122`); `--t-micro`
and `--t-fact` are task 26's (`task-26.md:19`, `:102`), `--t-micro` used on the `kbd` hint only, as
its rule requires. The active option carries an inset bar as well as a fill, and an outline under
`forced-colors`, so it is never colour alone. A table's rules are 1px lines between cells, not
elevation, so the ring-and-shadow rule does not apply. No transition anywhere: the menu appears
where the caret is, which is the whole of its causality.

#### Tests to write first — `desktop/src/components/editor/tables.test.ts`

```ts
import { describe, expect, it } from "vitest";
import { EditorState } from "prosemirror-state";
import { tableEditing } from "prosemirror-tables";
import type { Node, NodeType } from "prosemirror-model";
import { parseMarkdown, serializeMarkdown, serializeWithAuthorship } from "./markdown";
import { editorSchema } from "./schema";
import { createTable, gfmShapePlugin } from "./tables";

const TABLE = "| Step | Owner | Done |\n| :--- | :---: | ---: |\n| upload | EP-142 | `no` |\n| resume | **dylan** | yes |";
const N = editorSchema.nodes;
const cell = (t: NodeType, s: string, attrs: Record<string, unknown> = {}): Node =>
  t.create(attrs, s ? editorSchema.text(s) : null);
const shaped = (table: Node): Node => {
  const s0 = EditorState.create({ schema: editorSchema, plugins: [tableEditing(), gfmShapePlugin] });
  return s0.applyTransaction(s0.tr.replaceWith(0, s0.doc.content.size, table)).state.doc;
};

describe("tables", () => {
  it("parses a pipe table into one header row, body rows and column alignment", () => {
    const t = parseMarkdown(TABLE).firstChild!;
    expect([t.type.name, t.childCount]).toEqual(["table", 3]);
    expect(t.child(0).firstChild!.type).toBe(N.table_header);
    expect(t.child(1).firstChild!.type).toBe(N.table_cell);
    expect([0, 1, 2].map((c) => t.child(2).child(c).attrs.align)).toEqual(["left", "center", "right"]);
  });
  it("writes a table back to the same pipe table, which re-parses to an equal document", () => {
    const doc = parseMarkdown(TABLE);
    const md = serializeMarkdown(doc);
    expect(md).toBe(TABLE);
    expect(parseMarkdown(md).eq(doc)).toBe(true);
  });
  it("escapes a pipe inside a cell, in text and in code, and reads it back", () => {
    const src = "| a \\| b | `x \\| y` |\n| --- | --- |\n| c | d |";
    const doc = parseMarkdown(src);
    expect(doc.firstChild!.child(0).child(0).textContent).toBe("a | b");
    expect(serializeMarkdown(doc)).toBe(src);
  });
  it("keeps an agent's cell text agent through the round trip", () => {
    const out = serializeWithAuthorship(parseMarkdown(TABLE, "A"));
    expect(out.markdown).toBe(TABLE);
    expect(out.markdown.slice(out.authorship[0].from, out.authorship[0].to)).toBe("Step");
  });
  it("reshapes any table to GFM: header first row only, no spans, alignment per column", () => {
    const bad = N.table.create(null, [
      N.table_row.create(null, [cell(N.table_cell, "a"), cell(N.table_header, "b", { colspan: 2 })]),
      N.table_row.create(null, [cell(N.table_header, "c", { align: "right" }), cell(N.table_cell, "d"), cell(N.table_cell, "e")]),
    ]);
    const t = shaped(bad).firstChild!;
    const types = [0, 1].map((r) => Array.from({ length: t.child(r).childCount }, (_, c) => t.child(r).child(c).type.name));
    expect(types).toEqual([["table_header", "table_header", "table_header"], ["table_cell", "table_cell", "table_cell"]]);
    expect(t.child(1).child(0).attrs.align).toBeNull();          // the column's (null), not the cell's own
    const doc = shaped(bad);
    expect(parseMarkdown(serializeMarkdown(doc)).eq(doc)).toBe(true);
  });
  it("builds a fresh table with a header row of three and one body row", () => {
    const t = createTable(editorSchema);
    expect([t.childCount, t.child(0).childCount, t.child(0).child(0).type.name]).toEqual([2, 3, "table_header"]);
  });
});
```

The colspan case leaves the header row one cell short after flattening; the third header cell is
`fixTables`' (`tableEditing`), which is why that test mounts both plugins. That `fixTables` adds
the missing cell to the **end** of the short row is **UNVERIFIED** until step 3 runs it.

| Test | Mutant it kills |
|---|---|
| parses a pipe table | the `table` rule left disabled (commonmark preset), so a table stays a paragraph of pipes |
| writes it back, re-parses equal | lossy tables: a dropped delimiter row, lost alignment, a missing header |
| escapes a pipe | an unescaped `\|` that splits one cell into two on the next load |
| agent cell text | a cell serialized by `textContent`, which drops the sentinels and the class |
| reshapes to GFM | a pasted or edited table the serializer cannot write back (a header-less first row, a span) |
| a fresh table | *Table* inserting a grid with no header row, which GFM cannot hold |

#### Tests to write first — `desktop/src/components/editor/slashMenu.test.ts`

```ts
import { describe, expect, it } from "vitest";
import { EditorState, TextSelection } from "prosemirror-state";
import { criterionLink, issueLink, parseCriterionLink } from "../../work/links";
import { SHORTCUTS } from "./inputRules";
import { parseMarkdown } from "./markdown";
import { SLASH_ITEMS, matches, opensAt, optionsFor, type LinkSources, type SlashState } from "./slashMenu";

const stateAt = (md: string, pos: number): EditorState => {
  const s = EditorState.create({ doc: parseMarkdown(md) });
  return s.apply(s.tr.setSelection(TextSelection.create(s.doc, pos)));
};
const open = (query: string): SlashState => ({ open: true, from: 1, query, mode: "items", active: 0 });
const SOURCES: LinkSources = { issues: () => [], criteria: () => [] };
const labels = (st: EditorState, q: string, src?: LinkSources) => optionsFor(st, open(q), src).map((o) => o.label);
const BLOCK_IDS = ["h1", "h2", "h3", "bullet", "ordered", "check", "quote", "code", "divider"];

describe("slashMenu", () => {
  it("shows each block item's hint and label from the input rules' own table, both ways", () => {
    for (const item of SLASH_ITEMS.filter((i) => i.shortcut !== "")) {
      const row = SHORTCUTS.find((s) => s.id === item.id);
      expect([item.id, row?.trigger, row?.label]).toEqual([item.id, item.shortcut, item.label]);
    }
    const withHint = SLASH_ITEMS.filter((i) => i.shortcut !== "").map((i) => i.id);
    expect(withHint).toEqual(BLOCK_IDS);                 // every block shortcut has an item
    expect(SLASH_ITEMS.filter((i) => !BLOCK_IDS.includes(i.id)).every((i) => i.shortcut === "")).toBe(true);
  });
  it("opens at a block's start and after a space, never inside a word or in code", () => {
    expect(opensAt(stateAt("", 1), 1)).toBe(true);
    expect(opensAt(stateAt("and more", 5), 5)).toBe(true);    // after "and "
    expect(opensAt(stateAt("andor", 4), 4)).toBe(false);      // "and|or"
    expect(opensAt(stateAt("```\nx\n```", 2), 2)).toBe(false);
    expect(opensAt(stateAt("`code here`", 6), 6)).toBe(false);
  });
  it("filters by word prefix: tab is Table alone outside a table", () => {
    expect(labels(stateAt("", 1), "tab")).toEqual(["Table"]);
    expect(labels(stateAt("", 1), "HEAD 2")).toEqual(["Heading 2"]);
    expect(labels(stateAt("", 1), "todo")).toEqual(["Check list"]);
  });
  it("lists the row and column items only inside a table, and Remove row never on the header", () => {
    const md = "| a | b |\n| --- | --- |\n| c | d |";
    expect(labels(stateAt("", 1), "row")).toEqual([]);
    const inHeader = stateAt(md, 3);
    expect(labels(inHeader, "row")).toEqual(["Add row below"]);
    const inBody = stateAt(md, parseMarkdown(md).firstChild!.child(0).nodeSize + 4);
    expect(labels(inBody, "row")).toEqual(["Add row below", "Remove row"]);
    expect(labels(inBody, "tab")).toEqual(["Remove table"]);
  });
  it("lists the two link items only when the caller gives sources", () => {
    expect(labels(stateAt("", 1), "link")).toEqual([]);
    expect(labels(stateAt("", 1), "link", SOURCES)).toEqual(["Link an issue", "Link a criterion"]);
  });
  it("matches words, not substrings", () => {
    expect(matches(SLASH_ITEMS.find((i) => i.id === "table")!, "able")).toBe(false);
  });
  it("links a criterion by its key and id, and reads the link back exactly", () => {
    const href = criterionLink("brief:bf-00000000000000a1@5", "c-91e2a0b3c4d5e6f7");
    expect(href).toBe("panacea://criterion/brief:bf-00000000000000a1@5/c-91e2a0b3c4d5e6f7");
    expect(parseCriterionLink(href)).toEqual({ briefKey: "brief:bf-00000000000000a1@5", criterionId: "c-91e2a0b3c4d5e6f7" });
    expect(parseCriterionLink(criterionLink("brief:bf-00000000000000a1@5", "a/b c"))?.criterionId).toBe("a/b c");
    expect(() => criterionLink("EP-142", "c-1")).toThrow();
    expect(parseCriterionLink(issueLink("is-3f2a91c07b1e4d2a"))).toBeNull();
  });
});
```

#### Tests to write first — `desktop/src/components/editor/SlashMenu.test.tsx`

```tsx
import { afterEach, describe, expect, it, vi } from "vitest";
import { act, fireEvent, render, screen } from "@testing-library/react";
import type { Transaction } from "prosemirror-state";
import { parseIssueLink } from "../../work/links";
import { Editor, IDLE_COMMIT_MS, editorViewOf } from "./Editor";
import { serializeMarkdown } from "./markdown";
import type { LinkSources } from "./slashMenu";

vi.mock("@tauri-apps/plugin-opener", () => ({ openUrl: vi.fn(async () => {}) }));
afterEach(() => vi.useRealTimers());

const ISSUE = { id: "is-3f2a91c07b1e4d2a", alias: "EP-142", title: "Resume an upload" };
const SOURCES: LinkSources = {
  issues: (q) => [ISSUE].filter((i) => `${i.alias} ${i.title}`.toLowerCase().includes(q.toLowerCase())),
  criteria: () => [{ briefKey: "brief:bf-00000000000000a1@5", criterionId: "c-91e2a0b3c4d5e6f7", text: "Uploads resume" }],
};

function mount(markdown = "") {
  const onCommit = vi.fn();
  const r = render(<Editor ariaLabel="Description" value={{ markdown, authorship: [] }} onCommit={onCommit} linkSources={SOURCES} />);
  const view = editorViewOf(r.container.querySelector(".ed")!)!;
  const seen: Transaction[] = [];
  const dispatch = view.dispatch.bind(view);
  view.dispatch = (tr: Transaction) => { seen.push(tr); dispatch(tr); };
  /** ProseMirror's own input order: handleTextInput props first, a plain insert if none takes it. */
  const type = (s: string) => act(() => {
    for (const ch of s) {
      const { from, to } = view.state.selection;
      if (!view.someProp("handleTextInput", (f) => f(view, from, to, ch))) view.dispatch(view.state.tr.insertText(ch, from, to));
    }
  });
  const key = (k: string, init: KeyboardEventInit = {}) => act(() => { fireEvent.keyDown(view.dom, { key: k, ...init }); });
  const md = () => serializeMarkdown(view.state.doc);
  const options = () => screen.queryAllByRole("option").map((o) => o.querySelector(".ed-slash-label")?.textContent);
  return { ...r, view, onCommit, seen, type, key, md, options };
}

describe("SlashMenu", () => {
  it("opens on / at a block's start, and types a plain slash inside a word", () => {
    const a = mount();
    a.type("/");
    expect(screen.getByRole("listbox")).toBeTruthy();
    a.unmount();
    const b = mount();
    b.type("and/or");
    expect(screen.queryByRole("listbox")).toBeNull();
    expect(b.md()).toBe("and/or");
  });
  it("filters to Table on 'tab', and Enter inserts a table with a header row", () => {
    const { type, key, view, md, options } = mount();
    type("/tab");
    expect(options()).toEqual(["Table"]);
    key("Enter");
    expect(screen.queryByRole("listbox")).toBeNull();
    const t = view.state.doc.firstChild!;
    expect([t.type.name, t.child(0).child(0).type.name, t.child(1).child(0).type.name])
      .toEqual(["table", "table_header", "table_cell"]);
    expect(md()).not.toContain("/tab");
  });
  it("closes on Esc and leaves the slash and the query in the text", () => {
    const { type, key, md } = mount();
    type("/tab");
    key("Escape");
    expect(screen.queryByRole("listbox")).toBeNull();
    expect(md()).toBe("/tab");
  });
  it("shows each hint as a kbd, from the shortcut table, and none on Table", () => {
    const { type } = mount();
    type("/");
    const row = (label: string) => screen.getAllByRole("option").find((o) => o.textContent?.startsWith(label))!;
    expect(row("Heading 2").querySelector("kbd")?.textContent).toBe("## ");
    expect(row("Divider").querySelector("kbd")?.textContent).toBe("---");
    expect(row("Table").querySelector("kbd")).toBeNull();
  });
```

