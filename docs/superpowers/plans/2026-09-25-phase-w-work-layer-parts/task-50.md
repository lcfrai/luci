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
   typing to filter); while the menu is open the editor's root carries `aria-expanded="true"`,
   `aria-controls` naming the list and `aria-activedescendant` naming the active option, which the
   `textbox` role supports. The list is `role="listbox"`, each item `role="option"`.
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

