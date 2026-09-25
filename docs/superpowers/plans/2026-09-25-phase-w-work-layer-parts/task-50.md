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
   content only (`cellContent: "inline*"`), which is what a GFM cell can hold, so nothing a
   person builds in the editor fails to serialize. A `|` inside a cell is written `\|`.
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
- `desktop/src/work/links.ts` ⚑ (task 47's) — `criterionLink` (modified)
- `desktop/src/components/editor/editor.css` ⚑ — `.ed-slash*` and `.ed-root table` rules (modified)
- `desktop/src/components/editor/{slashMenu,tables}.test.ts`, `SlashMenu.test.tsx` — tests (new)
- `desktop/package.json` ⚑, `desktop/package-lock.json` — `prosemirror-tables` (MIT, version
  **UNVERIFIED** until install; its licence read from the lock file by task 49's licence test,
  which this task extends by one name) (modified)

