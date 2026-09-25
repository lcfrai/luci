### Task 51: comments: the thread, mentions, reactions, anchors, resolve

Stream **WD3** (the issue view), about 3 hours (an estimate, not a measurement). Depends on task 49
(the one `Editor`). Uses SW.1 (task 03's `Comment`, `CommentAnchor`, `CreateCommentRequest`; task
12's rules, `task-12.md:16-40`), SW.15 (`IssueComment`, `CommentAnchor`, `Reaction`,
`CreateCommentRequest` in `desktop/src/api/work.ts`, `task-37.md:147-159`, `:205-207`), SW.16
(`useWork`, `mutate`), task 46's `AuthorClass` (`task-46.md:437-467`), task 47's `useRevision`
(`task-47.md:191-215`) and SW.8 (`expectNoPrediction`). Task 52 fills the same `activity` slot
beside this one; task 60 creates span and hunk anchors through this part's `useStartComment`.

A comment is the one place on an issue where people talk rather than record facts, so it is where
a tool is most tempted to be convenient at the reader's expense: to fix a typo in a posted comment,
to move an anchor to "where that text is now", to show *3* instead of who reacted, to jump the page
to the newest reply. Each of those is a small lie about what was said, when, and by whom. This part
refuses all four, and each refusal is a test.

**Decisions made here, so nobody re-derives them:**

1. **Four anchor kinds, not five.** SW.1 carries `text`, `criterion`, `span` and `hunk`
   (`task-12.md:106-110`, `task-37.md:148-151`). The scope's fifth, *a brief section*, is not in
   SW.1, and this part does not invent a wire field for it; a comment on a brief section is a
   comment on the brief, which is phase 4's surface. Named as a gap.
2. **The quote is derived, never stored.** A `text` anchor stores `revision`, `start` and `end` in
   UTF-16 code units of that revision's description (`task-12.md:27-31`). The quote is
   `description.text.slice(start, end)` **of the anchored revision**, read through task 47's
   `useRevision`, so an anchor on @3 reads @3 when the head is @4 and says *on @3*. It is never
   re-mapped onto the head: offsets carried across a rewrite point at different words.
3. **A mention is a link in the text; `mentions` is derived from it.** `@` in the composer opens a
   people picker of the issue's team; choosing Priya inserts the link `[@Priya](panacea:user/u-…)`
   (task 49 keeps `panacea:` links live). On post, `mentions` is the distinct `u-` ids of those
   links, at most `MaxMentions` (20, `task-12.md:141`). A rendered mention shows the member's
   **current** `displayName`, looked up by id, so a renamed person still resolves; the name in the
   link text is a fallback for a person no longer on the team, shown as *former member*.
4. **Posting is one write, and the text comes only from `Editor`.** The composer is task 49's
   `Editor`; its `onCommit` keeps the draft and `onSubmit` (`Ctrl+Enter`) or the *Comment* button
   posts it as **one** `mutate({kind: "comment"})`. `body.text` is `serializeMarkdown`'s output, and
   `body.author` is `authorOf(doc)`: `"A"` only when every character sits in a `text·A` span, else
   `"H"` (a person who typed or pasted anything made the comment theirs; task 49's rule that pasted
   text is the person's). After a post the composer remounts empty by `key`.
5. **Reactions are a fixed set of five single code points** (👍 👀 ✅ ❓ 🎉), our choice; custom
   emoji are *later* (L-41). Each is one person's toggle: `on` is "I have not reacted with this".
   A reaction renders its emoji and the **names** of the people, in reaction order, and no digit
   (05 §4.4 work rule 3). Whether each passes the Hub's `ValidReactionEmoji` base ranges
   (`task-12.md:167-179`) is **UNVERIFIED** until task 12's code runs; a test here holds that each
   is one code point, the simplest shape that rule accepts.
6. **Resolve folds, and is never one-way.** A resolved top-level comment folds its thread to
   *Resolved by Priya · 11:02* with a *Reopen* button on the same line; task 56's `mutate` handle
   gives the undo window as well. A reply cannot be resolved on its own (the Hub allows it; the
   desktop resolves threads).
7. **Nothing rewrites a posted comment.** There is no *Edit*; a correction is a reply (the Hub has
   no edit route, `task-12.md:68-69`; SW.1 carries no edit record, so the scope's alternative does
   not arise). An erased comment keeps its place and id and reads *erased · 11:02*.
8. **An arrival never moves the page.** Comments never calls `scrollIntoView`, `scrollTo` or a
   `scrollTop` setter for a comment someone else wrote. The one scroll it makes is to the person's
   **own** just-posted comment, which they caused. Visible content staying put when a reply lands
   above the viewport is Chromium's scroll anchoring (`overflow-anchor: auto`, the default), which
   is **UNVERIFIED** in WebView2 for this layout: the owner's eyes, not a test, accept that.
9. **An agent's comment carries its mark and a word.** `body.author === "A"` renders
   `<AuthorClass author="A"/>`, the bracket mark and *text·A · agent* (never colour alone). In W1 a
   person composes every comment the desktop sends, so it sends `"H"`; agent comments arrive in W3.
   SW.12's `MarkKind` has no author kind (`task-29.md:145`), so the mark is task 46's `AuthorClass`,
   not `MarkGlyph`; named as a gap against the scope's wording.

**Files** (the scope names `Comments.tsx` and `IssueView.tsx`; the four split-out siblings and the
`Serves.tsx` edit are not yet in the File map, a gap named in the return):
- `desktop/src/components/issue/commentModel.ts` — pure: threads, mentions, reactions, anchors, the
  author class of a draft (new)
- `desktop/src/components/issue/commentModel.test.ts` — the pure rules (new)
- `desktop/src/components/issue/mentionPlugin.ts` — the `@` query as a ProseMirror plugin (new)
- `desktop/src/components/issue/CommentComposer.tsx` — `Editor`, the pending anchor, the people
  picker, *Comment* (new)
- `desktop/src/components/issue/Comments.tsx` — the thread, `useStartComment`, the anchor line (new)
- `desktop/src/components/issue/Comments.test.tsx` — the surface tests (new)
- `desktop/src/components/issue/IssueView.tsx` ⚑ (tasks 46-48's) — the provider, the default
  `activity`, *Comment on selection* over the description (modified)
- `desktop/src/components/issue/Serves.tsx` ⚑ (task 47's) — one *Comment* button per row, the
  criterion anchor (modified)
- `desktop/src/components/issue/issue.css` ⚑ — the `.iv-cm-*` rules appended (modified)

#### Interfaces — complete: `desktop/src/components/issue/commentModel.ts`

```ts
// Pure. No React, no DOM, no replica: what a thread IS, so every rule has a unit test.
import type { CommentAnchor, IssueComment, Reaction, User } from "../../api/work";
import type { MarkdownDoc } from "../editor/Editor";

/** Our fixed set (decision 5). One code point each; custom emoji are later (L-41). */
export const REACTIONS = ["\u{1F44D}", "\u{1F440}", "✅", "❓", "\u{1F389}"] as const;
export type ReactionEmoji = (typeof REACTIONS)[number];
export const REACTION_WORD: Readonly<Record<ReactionEmoji, string>> = {
  "\u{1F44D}": "agree", "\u{1F440}": "looking", "✅": "done", "❓": "question", "\u{1F389}": "thanks",
};

/** task-12.md:141. The Hub refuses more; the composer stops the picker at this many. */
export const MAX_MENTIONS = 20;

export interface CommentThread { root: IssueComment; replies: IssueComment[] }

/** The Hub answers oldest first, roots and replies in one list (task-12.md:112-113). A reply whose
 *  parent is not in the list (not loaded, or erased and dropped) becomes a root, never lost. */
export function threads(comments: readonly IssueComment[]): CommentThread[] {
  const roots = new Map<string, CommentThread>();
  const out: CommentThread[] = [];
  for (const c of comments) {
    const parent = c.parentId === null ? undefined : roots.get(c.parentId);
    if (parent) { parent.replies.push(c); continue; }
    const t: CommentThread = { root: c, replies: [] };
    roots.set(c.id, t);
    out.push(t);
  }
  return out;
}

const MENTION_HREF = /^panacea:user\/(u-[0-9a-f]{16})$/;
/** The link the picker inserts (decision 3). The name is a fallback, never the identity. */
export const mentionHref = (userId: string): string => `panacea:user/${userId}`;
export const mentionMarkdown = (u: User): string => `[@${u.displayName.replace(/[[\]]/g, "")}](${mentionHref(u.id)})`;
export const userIdOfHref = (href: string): string | null => MENTION_HREF.exec(href)?.[1] ?? null;

/** Distinct u- ids in the order they first appear, capped at MAX_MENTIONS. */
export function mentionsIn(markdown: string): string[] {
  const ids: string[] = [];
  for (const m of markdown.matchAll(/\]\((panacea:user\/u-[0-9a-f]{16})\)/g)) {
    const id = userIdOfHref(m[1]);
    if (id && !ids.includes(id)) ids.push(id);
    if (ids.length === MAX_MENTIONS) break;
  }
  return ids;
}

/** "A" only when every character of a non-empty draft is agent text (decision 4). */
export function authorOf(doc: MarkdownDoc): "H" | "A" {
  const n = doc.markdown.length;
  if (n === 0 || doc.authorship.some((s) => s.cls === "text·H")) return "H";
  let covered = 0;
  for (const s of doc.authorship) covered += Math.max(0, s.to - s.from);
  return covered >= n ? "A" : "H";
}

/** One person's toggle: `on` is true exactly when `me` has not reacted with `emoji`. The next list
 *  is what the replica shows optimistically; a set, never a counter (task-12.md:32-33). */
export function nextReactions(rs: readonly Reaction[], me: User, emoji: string): { on: boolean; reactions: Reaction[] } {
  const hit = rs.find((r) => r.emoji === emoji);
  const on = !(hit?.users.some((u) => u.id === me.id) ?? false);
  const reactions = rs
    .map((r) => r.emoji !== emoji ? r
      : { emoji, users: on ? [...r.users, me] : r.users.filter((u) => u.id !== me.id) })
    .filter((r) => r.users.length > 0);
  if (on && !hit) reactions.push({ emoji, users: [me] });
  return { on, reactions };
}
```

User ids match `u-` and 16 lowercase hex, the `ids.Wide` shape task 20's fixtures use
(`task-20.md:155-163`); if the Hub's user ids differ, the two regular expressions above are the one
place to change.

#### Interfaces — complete: `commentModel.ts`, part 2: anchors

```ts
/** A text anchor the person just made: the revision READ, not the head (decision 2). */
export function textAnchor(revision: number, start: number, end: number): CommentAnchor {
  return { kind: "text", revision, start, end, criterionId: "", ref: "" };
}
export function criterionAnchor(revision: number, criterionId: string): CommentAnchor {
  return { kind: "criterion", revision, start: 0, end: 0, criterionId, ref: "" };
}
/** Task 60 makes these from peek; the ref is stored verbatim (task-12.md:109-110). */
export function refAnchor(kind: "span" | "hunk", ref: string): CommentAnchor {
  return { kind, revision: 0, start: 0, end: 0, criterionId: "", ref };
}

/** What the anchor line says before its quote. `head` is the issue's head revision number. */
export function anchorWords(a: CommentAnchor, head: number): string {
  const on = a.revision !== 0 && a.revision !== head ? ` · on @${a.revision}` : "";
  switch (a.kind) {
    case "text": return `on the description${on}`;
    case "criterion": return `on criterion ${a.criterionId}${on}`;
    case "span": return `on transcript span ${a.ref}`;
    case "hunk": return `on ${a.ref}`;
  }
}

/** The quote of a text anchor, from THAT revision's description. Offsets are UTF-16 code units,
 *  which is what String.prototype.slice counts, so no conversion happens here. Out-of-range
 *  offsets (a Hub that did not check, or a stale fixture) give null, never a clamped guess. */
export function quoteOf(a: CommentAnchor, descriptionAtRevision: string): string | null {
  if (a.kind !== "text") return null;
  if (a.start < 0 || a.end <= a.start || a.end > descriptionAtRevision.length) return null;
  return descriptionAtRevision.slice(a.start, a.end);
}
```

#### Interfaces — complete: `desktop/src/components/issue/mentionPlugin.ts`

```ts
// Our own plugin on the public ProseMirror API (MIT); written from the API docs, from no editor's
// mention extension (OD-11). It reports "@query" before the cursor and never edits by itself.
import { Plugin, PluginKey, type EditorState } from "prosemirror-state";
import type { EditorView } from "prosemirror-view";

export interface MentionQuery { query: string; from: number; to: number; at: DOMRect }
export const mentionKey = new PluginKey<null>("iv-mention");

/** "@" at the start of a text block or after a space, then up to 32 characters with no space. */
const TRIGGER = /(?:^|\s)@([^\s@]{0,32})$/;

export function queryAt(state: EditorState): { query: string; from: number; to: number } | null {
  const { $from, empty } = state.selection;
  if (!empty || !$from.parent.isTextblock) return null;
  const before = $from.parent.textBetween(0, $from.parentOffset, undefined, "￼");
  const m = TRIGGER.exec(before);
  if (!m) return null;
  const to = $from.pos;
  return { query: m[1], from: to - m[1].length - 1, to };
}

/** `onQuery(null)` whenever the cursor leaves a trigger, so the picker closes on its own. */
export function mentionPlugin(onQuery: (q: MentionQuery | null) => void, viewRef: { current: EditorView | null }): Plugin {
  return new Plugin({
    key: mentionKey,
    view(view) {
      viewRef.current = view;
      return {
        update(v) {
          const q = queryAt(v.state);
          if (!q) return onQuery(null);
          const c = v.coordsAtPos(q.from);
          onQuery({ ...q, at: new DOMRect(c.left, c.top, 1, c.bottom - c.top) });
        },
        destroy() { viewRef.current = null; onQuery(null); },
      };
    },
  });
}

/** Replaces "@query" with the mention link, as the person's text. The schema's `link` mark
 *  (task 49's schema.ts, prosemirror-markdown's default, UNVERIFIED until 49 lands) carries href. */
export function insertMention(view: EditorView, q: { from: number; to: number }, label: string, href: string): void {
  const link = view.state.schema.marks.link;
  if (!link) return;
  const text = view.state.schema.text(`@${label}`, [link.create({ href, title: null })]);
  view.dispatch(view.state.tr.replaceWith(q.from, q.to, [text, view.state.schema.text(" ")]).scrollIntoView());
  view.focus();
}
```

`tr.scrollIntoView()` keeps the **caret** of the person typing in view; it is the editor's own
caret, which the person moved, not a comment arriving (decision 8).
