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
   people picker of the issue's team; choosing Priya inserts the link `[@Priya](panacea://user/u-…)`
   (task 49 keeps `panacea://` links live, `task-49.md:190`). On post, `mentions` is the distinct `u-` ids of those
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

const MENTION_HREF = /^panacea:\/\/user\/(u-[0-9a-f]{16})$/;
/** The link the picker inserts (decision 3). The name is a fallback, never the identity. */
export const mentionHref = (userId: string): string => `panacea://user/${userId}`;
export const mentionMarkdown = (u: User): string => `[@${u.displayName.replace(/[[\]]/g, "")}](${mentionHref(u.id)})`;
export const userIdOfHref = (href: string): string | null => MENTION_HREF.exec(href)?.[1] ?? null;

/** Distinct u- ids in the order they first appear, capped at MAX_MENTIONS. */
export function mentionsIn(markdown: string): string[] {
  const ids: string[] = [];
  for (const m of markdown.matchAll(/\]\((panacea:\/\/user\/u-[0-9a-f]{16})\)/g)) {
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

/** `onQuery(null)` whenever the cursor leaves a trigger, so the picker closes on its own.
 *  `onKey` sees keydowns first while the editor has focus; it returns true only for the picker's
 *  own keys (arrows, Enter, Tab) while a query is open, so every other key stays the editor's. */
export function mentionPlugin(
  onQuery: (q: MentionQuery | null) => void,
  viewRef: { current: EditorView | null },
  onKey: (e: KeyboardEvent) => boolean,
): Plugin {
  return new Plugin({
    key: mentionKey,
    props: { handleKeyDown: (_view, e) => onKey(e) },
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

#### Interfaces — complete: `desktop/src/components/issue/CommentComposer.tsx`

```tsx
import { useMemo, useRef, useState, type JSX } from "react";
import type { EditorView } from "prosemirror-view";
import type { CommentAnchor, User } from "../../api/work";
import { makeId } from "../../lib/ids";
import { mutate } from "../../work/replica/ops";
import { Popover } from "../common/Popover";
import { Editor, type MarkdownDoc } from "../editor/Editor";
import { authorOf, mentionHref, mentionsIn, MAX_MENTIONS } from "./commentModel";
import { insertMention, mentionPlugin, type MentionQuery } from "./mentionPlugin";

const EMPTY: MarkdownDoc = { markdown: "", authorship: [] };

/** One composer: a new thread (parentId null) or a reply. `anchor` is the pending anchor, shown
 *  above the editor by the caller; it rides with the post and is cleared by `onPosted`. */
export function CommentComposer(props: {
  issueId: string; parentId: string | null; anchor: CommentAnchor | null;
  people: readonly User[]; onPosted(localId: string): void; label: string;
}): JSX.Element {
  const { issueId, parentId, anchor, people, onPosted, label } = props;
  const [draft, setDraft] = useState<MarkdownDoc>(EMPTY);
  const [round, setRound] = useState(0);                 // remounts the Editor empty after a post
  const [q, setQ] = useState<MentionQuery | null>(null);
  const [active, setActive] = useState(0);
  const view = useRef<EditorView | null>(null);

  const matches = useMemo(() => {
    if (!q) return [];
    const needle = q.query.toLocaleLowerCase("en-AU");
    return people.filter((p) => p.displayName.toLocaleLowerCase("en-AU").startsWith(needle)).slice(0, 8);
  }, [q, people]);

  const choose = (u: User) => {
    if (view.current && q && mentionsIn(draft.markdown).length < MAX_MENTIONS) {
      insertMention(view.current, q, u.displayName, mentionHref(u.id));
    }
    setQ(null);
  };
  const qRef = useRef<{ q: MentionQuery | null; matches: User[]; active: number }>({ q: null, matches: [], active: 0 });
  qRef.current = { q, matches, active };
  const onKey = (e: KeyboardEvent): boolean => {
    const s = qRef.current;
    if (!s.q || s.matches.length === 0) return false;
    if (e.key === "ArrowDown" || e.key === "ArrowUp") {
      setActive((i) => (i + (e.key === "ArrowDown" ? 1 : s.matches.length - 1)) % s.matches.length);
      return true;
    }
    if (e.key === "Enter" || e.key === "Tab") { choose(s.matches[s.active]); return true; }
    return false;
  };
  const plugins = useMemo(() => [mentionPlugin((next) => { setQ(next); setActive(0); }, view, onKey)], [round]);

  const post = (doc: MarkdownDoc) => {
    if (doc.markdown.trim() === "") return;
    const localId = makeId("cml");                       // names the painted comment (task 56's op)
    mutate({
      kind: "comment", issueId, localId,
      req: { body: { text: doc.markdown, author: authorOf(doc) }, parentId, anchor, mentions: mentionsIn(doc.markdown) },
    });
    setDraft(EMPTY);
    setRound((n) => n + 1);
    onPosted(localId);
  };

  return (
    <div className="iv-cm-composer">
      <Editor key={round} value={EMPTY} ariaLabel={label} plugins={plugins}
        onCommit={setDraft} onSubmit={post} />
      <button type="button" className="iv-cm-post" disabled={draft.markdown.trim() === ""}
        onClick={() => post(draft)}>
        Comment <kbd className="iv-kbd">Ctrl+Enter</kbd>
      </button>
      <Popover anchor={q && matches.length > 0 ? q.at : null} onClose={() => setQ(null)}>
        <ul className="iv-cm-people" role="listbox" aria-label="Mention a teammate">
          {matches.map((u, i) => (
            <li key={u.id} role="option" aria-selected={i === active} data-user={u.id}
              onMouseDown={(e) => { e.preventDefault(); choose(u); }}>{u.displayName}</li>
          ))}
        </ul>
      </Popover>
    </div>
  );
}
```

Three props on `Editor` are this part's, not in task 49's `EditorProps` (`task-49.md:522-528`):
**`plugins`** (extra ProseMirror plugins, placed **before** the editor's own in `load()`,
`task-49.md:558-566`, because ProseMirror asks plugins in order and the editor's `Enter` keymap
would otherwise split the paragraph before the picker saw the key), **`onSubmit(doc)`** (called by
the `Mod-Enter` binding, `task-49.md:560`, in place of `commit()` when set, so `Ctrl+Enter` posts a
comment and still commits a description), and, for the description, **`onSelectionChange`**
(below). Each is optional and additive, so task 49's description use is unchanged; they are named
as gaps against task 49, and this task's implementer adds them there (a three-line edit to
`Editor.tsx`, task 49's file, listed below). The *Comment* button posts `draft`, which is
current because a click blurs the editor first and task 49 commits on blur (mousedown, blur, click
is the DOM's order). `onMouseDown` + `preventDefault` on a person row keeps the editor focused, so
choosing a person does not blur and commit mid-mention. The picker lists only the issue's team
(`people`, task 46's `issueData`): the Hub checks each mention against the team through task 08's
membership read (`task-12.md:83`; the refusal code is task 12's and is not printed here), and the
picker never offers someone that check would refuse.

#### Interfaces — complete: `desktop/src/components/issue/Comments.tsx`, part 1: the pending anchor

```tsx
import { createContext, useContext, useLayoutEffect, useMemo, useRef, useState, type JSX, type ReactNode } from "react";
import type { CommentAnchor, IssueComment, IssueRevision, User } from "../../api/work";
import { mutate } from "../../work/replica/ops";
import { AuthorClass } from "./AuthorClass";
import { CommentComposer } from "./CommentComposer";
import { anchorWords, nextReactions, quoteOf, REACTIONS, REACTION_WORD, threads, userIdOfHref,
  type CommentThread } from "./commentModel";
import { useIssueComments } from "./issueData";
import { useRevision } from "./useRevision";

/** The one way anything outside the thread starts a comment: the description's selection, a
 *  Serves row, and (task 60) peek's span and hunk. The anchor waits above the composer. */
interface StartComment { start(anchor: CommentAnchor): void }
const StartCtx = createContext<StartComment | null>(null);
const PendingCtx = createContext<{ anchor: CommentAnchor | null; clear(): void } | null>(null);

export function CommentsProvider(props: { children: ReactNode }): JSX.Element {
  const [anchor, setAnchor] = useState<CommentAnchor | null>(null);
  const start = useMemo<StartComment>(() => ({
    start(a) {
      setAnchor(a);
      document.querySelector<HTMLElement>(".iv-cm-new .ProseMirror")?.focus();   // the person asked
    },
  }), []);
  const pending = useMemo(() => ({ anchor, clear: () => setAnchor(null) }), [anchor]);
  return <StartCtx.Provider value={start}><PendingCtx.Provider value={pending}>{props.children}</PendingCtx.Provider></StartCtx.Provider>;
}

/** Outside a provider this throws: a caller that starts a comment nowhere is a bug, not a no-op. */
export function useStartComment(): (anchor: CommentAnchor) => void {
  const s = useContext(StartCtx);
  if (!s) throw new Error("useStartComment outside CommentsProvider");
  return s.start;
}

/** The anchor line: words, then the quote of THAT revision (decision 2), never the head's text. */
function AnchorLine(props: { issueId: string; anchor: CommentAnchor; head: IssueRevision }): JSX.Element {
  const { issueId, anchor, head } = props;
  const text = useRevision(issueId, anchor.kind === "text" ? anchor.revision : null, head);
  const quote = anchor.kind !== "text" ? null
    : text.status === "ready" ? quoteOf(anchor, text.value.content.description.text) : null;
  return (
    <div className="iv-cm-anchor" data-anchor={anchor.kind}>
      <span className="iv-cm-anchor-on">{anchorWords(anchor, head.revision)}</span>
      {anchor.kind === "text" && text.status === "fetching" ? <span className="iv-fact"> @{anchor.revision} …</span> : null}
      {anchor.kind === "text" && text.status === "refused" ? <span className="iv-fact"> {text.message}</span> : null}
      {anchor.kind === "text" && text.status === "ready" && quote === null
        ? <span className="iv-fact"> offsets {anchor.start}-{anchor.end} are outside @{anchor.revision}</span> : null}
      {quote !== null ? <blockquote className="iv-cm-quote">{quote}</blockquote> : null}
      {anchor.kind === "span" || anchor.kind === "hunk" ? <code className="iv-fact">{anchor.ref}</code> : null}
    </div>
  );
}
```

`start` focuses the new-thread composer because the person clicked *Comment*: that moves the
caret, which they caused, and scrolls nothing. `fetching` shows the revision number and an
ellipsis in text, not a spinner (`DESIGN-SYSTEM.md`'s one-heartbeat rule); a refusal shows the
relay's message verbatim; an anchor whose offsets fall outside its own revision says so with the
numbers rather than quoting a clamped guess.

#### Interfaces — complete: `Comments.tsx`, part 2: one comment

```tsx
const time = (iso: string): string =>
  new Date(iso).toLocaleTimeString("en-AU", { hour: "2-digit", minute: "2-digit", hour12: false });

/** A mention renders the member's CURRENT name, found by id (decision 3). */
function Body(props: { text: string; people: readonly User[] }): JSX.Element {
  const parts: ReactNode[] = [];
  let at = 0;
  for (const m of props.text.matchAll(/\[@([^\]]*)\]\(([^)\s]+)\)/g)) {
    const id = userIdOfHref(m[2]);
    if (id === null) continue;                       // not a mention; stays text
    parts.push(props.text.slice(at, m.index));
    const who = props.people.find((p) => p.id === id);
    parts.push(<span key={m.index} className="iv-cm-mention" data-user={id}>
      @{who ? who.displayName : m[1]}{who ? null : <span className="iv-fact"> · former member</span>}</span>);
    at = (m.index ?? 0) + m[0].length;
  }
  parts.push(props.text.slice(at));
  return <div className="iv-cm-body">{parts}</div>;
}

function Reactions(props: { c: IssueComment; me: User }): JSX.Element {
  const { c, me } = props;
  const toggle = (emoji: string) => {
    const { on } = nextReactions(c.reactions, me, emoji);
    mutate({ kind: "reaction", issueId: c.issueId, commentId: c.id, emoji, on });
  };
  return (
    <div className="iv-cm-reactions" role="group" aria-label="Reactions">
      {c.reactions.map((r) => (
        <button key={r.emoji} type="button" className="iv-cm-reaction" aria-pressed={r.users.some((u) => u.id === me.id)}
          onClick={() => toggle(r.emoji)}>
          <span aria-hidden="true">{r.emoji}</span>
          <span className="iv-cm-who">{r.users.map((u) => u.displayName).join(", ")}</span>
        </button>
      ))}
      <span className="iv-cm-add" role="group" aria-label="Add a reaction">
        {REACTIONS.filter((e) => !c.reactions.some((r) => r.emoji === e)).map((e) => (
          <button key={e} type="button" aria-label={REACTION_WORD[e]} onClick={() => toggle(e)}>{e}</button>
        ))}
      </span>
    </div>
  );
}

function OneComment(props: { c: IssueComment; me: User; people: readonly User[]; head: IssueRevision;
  onReply?(): void; onResolve?(): void }): JSX.Element {
  const { c, me, people, head, onReply, onResolve } = props;
  if (c.erasedAt !== null) {
    return <article className="iv-cm" data-comment={c.id} data-erased=""><span className="iv-fact">erased · {time(c.erasedAt)}</span></article>;
  }
  return (
    <article className="iv-cm" data-comment={c.id} data-author={c.body.author}>
      <header className="iv-cm-by">
        <AuthorClass author={c.body.author} />
        <span className="iv-cm-name">{c.createdBy.displayName}</span>
        <time className="iv-fact" dateTime={c.createdAt}>{time(c.createdAt)}</time>
      </header>
      {c.anchor ? <AnchorLine issueId={c.issueId} anchor={c.anchor} head={head} /> : null}
      <Body text={c.body.text} people={people} />
      <Reactions c={c} me={me} />
      <footer className="iv-cm-acts">
        {onReply ? <button type="button" onClick={onReply}>Reply</button> : null}
        {onResolve ? <button type="button" onClick={onResolve}>Resolve</button> : null}
      </footer>
    </article>
  );
}
```

The body is rendered as text with mentions lifted out, **not** through `innerHTML`: a comment is
the person's Markdown, and task 49's read-only rendering of it is the richer path the implementer
may swap in later (`Editor` with `readOnly`, `task-49.md:525`); one ProseMirror view per comment
is the cost this part declines in W1, and it does not depend on that swap. No digit appears in a reaction: the names are the count (decision 5). The add row
offers only the emoji nobody has used yet, so one emoji never shows twice.

#### Interfaces — complete: `Comments.tsx`, part 3: the thread and the list

```tsx
function Thread(props: { t: CommentThread; me: User; people: readonly User[]; head: IssueRevision;
  onOwn(localId: string): void }): JSX.Element {
  const { t, me, people, head, onOwn } = props;
  const [replying, setReplying] = useState(false);
  const r = t.root;
  const resolve = (resolved: boolean) => mutate({ kind: "resolve", issueId: r.issueId, commentId: r.id, resolved });
  if (r.resolvedAt !== null && r.erasedAt === null) {
    return (
      <div className="iv-cm-thread" data-thread={r.id} data-resolved="">
        <span className="iv-cm-folded">Resolved by {r.resolvedBy?.displayName ?? "someone"} · {time(r.resolvedAt)}</span>
        <span className="iv-fact"> · {1 + t.replies.length} comments</span>
        <button type="button" onClick={() => resolve(false)}>Reopen</button>
      </div>
    );
  }
  return (
    <div className="iv-cm-thread" data-thread={r.id}>
      <OneComment c={r} me={me} people={people} head={head}
        onReply={() => setReplying(true)} onResolve={r.erasedAt === null ? () => resolve(true) : undefined} />
      {t.replies.map((c) => <OneComment key={c.id} c={c} me={me} people={people} head={head} />)}
      {replying ? (
        <CommentComposer issueId={r.issueId} parentId={r.id} anchor={null} people={people}
          label={`Reply to ${r.createdBy.displayName}`} onPosted={(id) => { setReplying(false); onOwn(id); }} />
      ) : null}
    </div>
  );
}

/** The default `activity` slot (task 46). `head` is the head revision; anchors fetch their own. */
export function Comments(props: { issueId: string; me: User; people: readonly User[]; head: IssueRevision }): JSX.Element {
  const { issueId, me, people, head } = props;
  const comments = useIssueComments(issueId);
  const pending = useContext(PendingCtx);
  const [own, setOwn] = useState<string | null>(null);
  const list = useRef<HTMLDivElement>(null);

  // The ONE scroll: to a comment this person just posted (decision 8). Nothing else scrolls.
  useLayoutEffect(() => {
    if (own === null) return;
    const el = list.current?.querySelector<HTMLElement>(`[data-comment="${CSS.escape(own)}"]`);
    if (el) { el.scrollIntoView({ block: "nearest" }); setOwn(null); }
  }, [own, comments]);

  return (
    <section className="iv-cm-list" aria-label="Comments" ref={list}>
      {threads(comments).map((t) => (
        <Thread key={t.root.id} t={t} me={me} people={people} head={head} onOwn={setOwn} />
      ))}
      <div className="iv-cm-new">
        {pending?.anchor ? (
          <div className="iv-cm-pending">
            <AnchorLine issueId={issueId} anchor={pending.anchor} head={head} />
            <button type="button" onClick={pending.clear}>Drop the anchor</button>
          </div>
        ) : null}
        <CommentComposer issueId={issueId} parentId={null} anchor={pending?.anchor ?? null} people={people}
          label="New comment" onPosted={(id) => { pending?.clear(); setOwn(id); }} />
      </div>
    </section>
  );
}
```

The resolved fold keeps the thread's **size** in words (*· 3 comments*), not a badge. *Reopen* is a
new write by the person, the same op with `resolved: false`; task 56's `U` and chip undo a
`resolve` the same way, because `inverseOf` of a resolve is the resolve flipped (`task-56.md:234`,
**UNVERIFIED** that its switch has the `resolve` arm until task 56's code runs; the test below
holds it). The painted comment keeps `localId` as its `data-comment` until the Hub's `cm-` record
replaces it (`task-56.md:375`); the scroll runs on the painted element, in the same frame as the
post, so the id change does not matter to it.
