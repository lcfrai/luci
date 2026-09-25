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
   posts it as **one** `mutate({kind: "comment"})`. `body` is task 49's `toAuthored(doc)`
   (`task-49.md:114-127`): `"A"` if any agent span remains, else `"H"`, so a mixed comment is
   never labelled a person's (05 §3.4; the same rule as the description, no second one). After a
   post the composer remounts empty by `key`.
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
- `desktop/src/components/issue/commentModel.ts` — pure: threads, mentions, reactions, anchors, a
  selection's offsets (new)
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
import { toAuthored, type MarkdownDoc } from "../editor/authorClass";
import { Editor } from "../editor/Editor";
import { mentionHref, mentionsIn, MAX_MENTIONS } from "./commentModel";
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
      req: { body: toAuthored(doc), parentId, anchor, mentions: mentionsIn(doc.markdown) },
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
        <span className="iv-fact"> · {t.replies.length === 0 ? "1 comment" : `${1 + t.replies.length} comments`}</span>
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
holds it). The painted comment keeps `localId` as its `data-comment` until the Hub's `ic-` record (task 20's `WORK_IDS.comment`; task 56 writes `cm-`)
replaces it (`task-56.md:375`); the scroll runs on the painted element, in the same frame as the
post, so the id change does not matter to it.

#### Interfaces — complete: `commentModel.ts`, part 3: a selection becomes offsets

```ts
/** The description is Markdown; the editor's selection is its plain text. A selection becomes a
 *  text anchor only when that text occurs exactly ONCE in the revision's Markdown, so the offsets
 *  are the Markdown's own and nothing maps positions between two models. */
export type Located = { start: number; end: number } | { refused: "empty" | "absent" | "ambiguous" };
export function locate(markdown: string, selected: string): Located {
  if (selected.trim() === "") return { refused: "empty" };
  const first = markdown.indexOf(selected);
  if (first < 0) return { refused: "absent" };                 // it crossed **bold** or a link
  if (markdown.indexOf(selected, first + 1) >= 0) return { refused: "ambiguous" };
  return { start: first, end: first + selected.length };
}
export const LOCATE_WORDS: Readonly<Record<"empty" | "absent" | "ambiguous", string>> = {
  empty: "select some words in the description first",
  absent: "the selection crosses formatting; select plain words",
  ambiguous: "those words appear more than once; select a few more",
};
```

#### Interfaces — complete: the edits to task 49's `Editor.tsx`, task 46's `issueData.ts`, `Comments.tsx` part 4

```tsx
// Editor.tsx: EditorProps (task-49.md:522-528) gains three optional members.
  plugins?: readonly Plugin[];                 // placed BEFORE the editor's own in load()
  onSubmit?(doc: MarkdownDoc): void;           // Mod-Enter calls this in place of commit() when set
  onSelectionChange?(selectedText: string): void;
// …and in the view's dispatchTransaction, after the state is applied:
  if (tr.selectionSet) latest.current.onSelectionChange?.(next.doc.textBetween(next.selection.from, next.selection.to, "\n"));

// issueData.ts (task 46's; the only reader of the replica's shape, task-46.md:475-476):
export function useIssueComments(issueId: string): readonly IssueComment[] {
  return useWork((r) => r.comments(issueId));   // oldest first, as the Hub answers (task-12.md:112)
}

// Comments.tsx, part 4. Serves and the description render inside the provider in the view, and
// alone in their own tests; outside a provider there is no Comment button, not a throw.
export function useMaybeStartComment(): ((anchor: CommentAnchor) => void) | null {
  return useContext(StartCtx)?.start ?? null;
}
```

`r.comments(issueId)` is task 56's `ReplicaReadView.comments` (`task-56.md:184`), read through
`useWork`; it is a sixth replica read beside task 46's four and task 47's one, named as a gap for
task 55.

#### Interfaces — complete: the edits to `IssueView.tsx` and `Serves.tsx`

```tsx
// IssueView.tsx (task 46's, as tasks 47 and 49 leave it). IssueViewBody wraps its section:
import { Comments, CommentsProvider, useMaybeStartComment } from "./Comments";
import { locate, LOCATE_WORDS, textAnchor } from "./commentModel";
//   return <CommentsProvider><section …>…</section></CommentsProvider>;
//   the activity slot's default:
{slots.activity ?? <Comments issueId={head.id} me={me} people={people} head={headRevision} />}
//   the description (task 49's Editor) gains onSelectionChange={setSelected}, and below it:
<CommentOnSelection markdown={shown.value.content.description.text} revision={at} selected={selected} />

function CommentOnSelection(props: { markdown: string; revision: number; selected: string }): JSX.Element | null {
  const start = useMaybeStartComment();
  if (!start || props.selected === "") return null;
  const where = locate(props.markdown, props.selected);
  return "refused" in where
    ? <p className="iv-fact" data-fact="anchor-refused">{LOCATE_WORDS[where.refused]}</p>
    : <button type="button" className="iv-cm-on-sel"
        onClick={() => start(textAnchor(props.revision, where.start, where.end))}>Comment on selection</button>;
}

// Serves.tsx (task 47's): ServeRow gains `revision: number`, and after the facts:
const startComment = useMaybeStartComment();
{startComment ? (
  <button type="button" className="iv-serve-comment" aria-label={`Comment on criterion ${serve.criterionId}`}
    onClick={() => startComment(criterionAnchor(revision, serve.criterionId))}>Comment</button>
) : null}
```

`revision` is `at`, the revision **being read**, so a selection in @3 anchors @3 even when the head
is @4 (decision 2). `IssueViewBody` gains `const [selected, setSelected] = useState("")`, reset by
the same effect that resets `reading`.

#### Interfaces — complete: appended to `desktop/src/components/issue/issue.css`

```css
/* Task 51: comments. Author is a mark and a word; a reaction is names; nothing animates in. */
.iv-cm-list { display: grid; gap: var(--s-3); padding: var(--s-3) var(--s-4); overflow-anchor: auto; }
.iv-cm-thread { display: grid; gap: var(--s-2); }
.iv-cm-thread > .iv-cm + .iv-cm { margin-inline-start: var(--s-4); }
.iv-cm { display: grid; gap: var(--s-1); padding: var(--s-2) var(--s-3); border-radius: var(--r-sm);
  box-shadow: 0 0 0 1px var(--ring); }
.iv-cm-by { display: flex; align-items: baseline; gap: var(--s-2); }
.iv-cm-name, .iv-cm-body, .iv-cm-folded { font-family: var(--sans); color: var(--ink); }
.iv-cm-body { white-space: pre-wrap; }
.iv-cm-mention { font-weight: 600; }
.iv-cm-anchor { display: grid; gap: var(--s-1); font-family: var(--sans); color: var(--ink-2); }
.iv-cm-quote { margin: 0; padding-inline-start: var(--s-2); box-shadow: inset 2px 0 0 var(--ring);
  font-family: var(--sans); color: var(--ink); white-space: pre-wrap; }
.iv-cm-reactions, .iv-cm-add, .iv-cm-acts { display: flex; flex-wrap: wrap; gap: var(--s-1); }
.iv-cm-reaction { display: inline-flex; gap: var(--s-1); align-items: baseline; }
.iv-cm-reaction[aria-pressed="true"] .iv-cm-who { font-weight: 600; }
.iv-cm-who { font-family: var(--sans); color: var(--ink-2); }
.iv-cm-pending { display: flex; align-items: start; gap: var(--s-2); }
.iv-cm-people { list-style: none; margin: 0; padding: var(--s-1); display: grid; }
.iv-cm-people [aria-selected="true"] { box-shadow: 0 0 0 1px var(--accent-ring); border-radius: var(--r-sm); }
```

No `transition` and no `@keyframes`: a comment arriving is not caused by the reader, so no motion
explains it (`DESIGN-SYSTEM.md`, motion rule). `overflow-anchor: auto` is the browser default,
written out so a later reset stylesheet that sets `none` is visible in review (decision 8).
Pressed is a weight change on the names, not a fill colour.

#### Tests to write first — `desktop/src/components/issue/commentModel.test.ts`

```ts
import { describe, expect, it } from "vitest";
import { ALICE, DYLAN, makeComment, WORK_IDS } from "../../test/workFixtures";
import type { IssueComment } from "../../api/work";
import { anchorWords, locate, mentionMarkdown, mentionsIn, nextReactions, quoteOf,
  REACTIONS, textAnchor, threads } from "./commentModel";

const c = (over: Partial<IssueComment>): IssueComment => ({ ...makeComment(), anchor: null, ...over } as IssueComment);

describe("commentModel", () => {
  it("a reply whose parent is absent becomes a root, never lost", () => {
    // kills: dropping orphans, which hides a reply to an erased-and-dropped root
    const t = threads([c({ id: "ic-1" }), c({ id: "ic-2", parentId: "ic-1" }), c({ id: "ic-3", parentId: "ic-9" })]);
    expect(t.map((x) => [x.root.id, x.replies.map((r) => r.id)])).toEqual([["ic-1", ["ic-2"]], ["ic-3", []]]);
  });

  it("a mention stores the user id, and a renamed person still resolves", () => {
    // kills: name-text mentions (the id, not "@Alice Example", is what mentions carries)
    const md = `ping ${mentionMarkdown(ALICE)} and ${mentionMarkdown(ALICE)}`;
    expect(mentionsIn(md)).toEqual([WORK_IDS.alice]);
    const renamed = md.replace("Alice Example", "Alice Renamed");
    expect(mentionsIn(renamed)).toEqual([WORK_IDS.alice]);
  });

  it("the same person reacting twice ends with no reaction", () => {
    // kills: a double add, and a counter that goes 1, 2
    const one = nextReactions([], DYLAN, REACTIONS[0]);
    expect(one).toEqual({ on: true, reactions: [{ emoji: REACTIONS[0], users: [DYLAN] }] });
    const two = nextReactions(one.reactions, DYLAN, REACTIONS[0]);
    expect(two).toEqual({ on: false, reactions: [] });
  });

  it("every reaction in the set is one code point", () => {
    // kills: a skin-tone or ZWJ sequence the Hub's base-range rule would refuse (decision 5)
    for (const e of REACTIONS) expect([...e]).toHaveLength(1);
  });

  it("a text anchor keeps its revision; on a later head it says on @3 and quotes @3", () => {
    // kills: re-mapping offsets onto the head (the head's slice would read other words)
    const at3 = "A restarted sandbox must pick up the same conversation.";
    const at4 = "Every restarted sandbox resumes; see the runbook first.";
    const a = textAnchor(3, 2, 19);
    expect(anchorWords(a, 4)).toBe("on the description · on @3");
    expect(anchorWords(a, 3)).toBe("on the description");
    expect(quoteOf(a, at3)).toBe("restarted sandbox");
    expect(quoteOf(a, at4)).not.toBe(quoteOf(a, at3));
  });

  it("offsets outside the revision give null, never a clamped quote", () => {
    // kills: slice's silent clamp showing a shorter quote as if it were the anchored one
    expect(quoteOf(textAnchor(3, 40, 90), "short")).toBeNull();
    expect(quoteOf(textAnchor(3, 5, 5), "short text")).toBeNull();
  });

  it("a selection anchors only where it occurs exactly once", () => {
    // kills: indexOf's first hit anchoring the wrong one of two equal phrases
    expect(locate("pick up the same pick up", "pick up")).toEqual({ refused: "ambiguous" });
    expect(locate("a **bold** word", "bold word")).toEqual({ refused: "absent" });
    expect(locate("the same conversation", "same")).toEqual({ start: 4, end: 8 });
  });

});
```

The comment's author class is task 49's `toAuthored`, tested there (`task-49.md:815`); this part
adds no second rule, and surface test 1 below holds that the composer calls it.

#### Tests to write first — `desktop/src/components/issue/Comments.test.tsx`

```tsx
import { act, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { IssueComment, User } from "../../api/work";
import { expectNoPrediction } from "../../test/expectNoPrediction";
import { ALICE, DYLAN, makeComment, makeIssueRevision, WORK_IDS } from "../../test/workFixtures";
import { mentionMarkdown, textAnchor } from "./commentModel";

let comments: IssueComment[] = [];
vi.mock("../../work/replica/replica", () => ({
  useWork: (f: (r: unknown) => unknown) => f({ comments: () => comments }),
  search: vi.fn(() => []),
}));
const mutate = vi.fn(() => ({ id: "op-1", undoable: true, undo: vi.fn(), settled: Promise.resolve("applied") }));
vi.mock("../../work/replica/ops", () => ({ mutate: (op: unknown) => mutate(op) }));
const issueRevisionAt = vi.fn();
vi.mock("../../api", () => ({ api: { work: { issueRevisionAt: (...a: unknown[]) => issueRevisionAt(...a) } } }));
// The composer's Editor is task 49's; here a stub hands the composer a finished document, so the
// test holds that what is posted is exactly the Editor's output and nothing else.
let submitDoc = { markdown: "", authorship: [] as { from: number; to: number; cls: string }[] };
vi.mock("../editor/Editor", () => ({
  Editor: (p: { onSubmit?(d: unknown): void; ariaLabel: string }) =>
    <button type="button" aria-label={`${p.ariaLabel} (stub)`} onClick={() => p.onSubmit?.(submitDoc)}>submit</button>,
}));
import { Comments, CommentsProvider } from "./Comments";

const HEAD = makeIssueRevision({ issueId: WORK_IDS.ep163, revision: 4,
  content: { ...makeIssueRevision().content, description: { text: "Every restarted sandbox resumes.", author: "H" } } });
const c = (over: Partial<IssueComment>): IssueComment => ({ ...makeComment(), anchor: null, ...over } as IssueComment);
const view = (people: readonly User[] = [DYLAN, ALICE]) => render(
  <CommentsProvider><Comments issueId={WORK_IDS.ep163} me={DYLAN} people={people} head={HEAD} /></CommentsProvider>);

beforeEach(() => {
  comments = []; mutate.mockClear(); issueRevisionAt.mockReset();
  Element.prototype.scrollIntoView = vi.fn();            // jsdom has none; a spy either way
});

describe("Comments", () => {
  it("posting sends ONE append whose Markdown and authorship come from Editor", () => {
    // kills: a second text path (a textarea's value, or the draft state instead of the doc), and
    // a second authorship rule: one agent span makes the whole comment A (task 49's toAuthored)
    const md = `resume works, ${mentionMarkdown(ALICE)}`;
    submitDoc = { markdown: md, authorship: [{ from: 0, to: 6, cls: "text·A" }] };
    view();
    fireEvent.click(screen.getByRole("button", { name: "New comment (stub)" }));
    expect(mutate).toHaveBeenCalledTimes(1);
    expect(mutate).toHaveBeenCalledWith(expect.objectContaining({
      kind: "comment", issueId: WORK_IDS.ep163, localId: expect.any(String),
      req: { body: { text: md, author: "A" }, parentId: null, anchor: null, mentions: [WORK_IDS.alice] },
    }));
  });

  it("a text anchor on @3 quotes @3 and says so while the head is @4", async () => {
    // kills: quoting the head's description at the stored offsets
    issueRevisionAt.mockResolvedValue(makeIssueRevision({ issueId: WORK_IDS.ep163, revision: 3 }));
    comments = [c({ anchor: textAnchor(3, 2, 19) })];
    view();
    expect(screen.getByText("on the description · on @3")).toBeTruthy();
    await waitFor(() => expect(screen.getByText("restarted sandbox")).toBeTruthy());
    expect(issueRevisionAt).toHaveBeenCalledWith(WORK_IDS.ep163, 3);
  });

  it("a mention renders the member's current name, and a former member says so", () => {
    // kills: rendering the link text (the name at posting time) as the identity
    comments = [c({ body: { text: `[@Alice Old](panacea://user/${WORK_IDS.alice}) look`, author: "H" } })];
    const { unmount } = view();
    expect(screen.getByText("@Alice Example")).toBeTruthy();
    unmount();
    view([DYLAN]);
    expect(screen.getByText(/former member/)).toBeTruthy();
  });

  it("reactions render the people's names and no digit", () => {
    // kills: a reaction count
    comments = [c({ reactions: [{ emoji: "\u{1F44D}", users: [DYLAN, ALICE] }] })];
    view();
    const group = screen.getByRole("group", { name: "Reactions" });
    expect(group.textContent).toContain("Dylan Example, Alice Example");
    expect(group.textContent).not.toMatch(/[0-9]/);
  });

  it("my second click on my own reaction sends on: false", () => {
    // kills: a double add from the surface (the toggle reads who reacted, not a local flag)
    comments = [c({ reactions: [{ emoji: "\u{1F44D}", users: [DYLAN] }] })];
    view();
    fireEvent.click(screen.getByRole("button", { pressed: true }));
    expect(mutate).toHaveBeenCalledWith({ kind: "reaction", issueId: WORK_IDS.ep163,
      commentId: WORK_IDS.comment, emoji: "\u{1F44D}", on: false });
  });

  it("resolve folds the thread, and Reopen and Undo both send it back", async () => {
    // kills: a one-way resolve
    comments = [c({})];
    const { rerender } = view();
    fireEvent.click(screen.getByRole("button", { name: "Resolve" }));
    const op = { kind: "resolve", issueId: WORK_IDS.ep163, commentId: WORK_IDS.comment, resolved: true };
    expect(mutate).toHaveBeenLastCalledWith(op);
    comments = [c({ resolvedAt: "2026-09-25T01:02:00Z", resolvedBy: ALICE })];
    rerender(<CommentsProvider><Comments issueId={WORK_IDS.ep163} me={DYLAN} people={[DYLAN, ALICE]} head={HEAD} /></CommentsProvider>);
    expect(screen.getByText(/^Resolved by Alice Example · \d\d:\d\d$/)).toBeTruthy();
    fireEvent.click(screen.getByRole("button", { name: "Reopen" }));
    expect(mutate).toHaveBeenLastCalledWith({ ...op, resolved: false });
    const real = await vi.importActual<typeof import("../../work/replica/ops")>("../../work/replica/ops");
    expect(real.inverseOf(op as never, new Map(), {} as never)).toEqual({ ...op, resolved: false });
  });

  it("an agent's comment carries its mark and the word, not a colour alone", () => {
    // kills: colour-only authorship
    comments = [c({ body: { text: "Fixture added.", author: "A" } })];
    const { container } = view();
    const art = container.querySelector('[data-author="A"]')!;
    expect(art.querySelector(".iv-author svg")).not.toBeNull();
    expect(art.querySelector(".iv-author-word")!.textContent).toBe("text·A · agent");
  });

  it("an arrival while scrolled up leaves scrollTop unchanged and scrolls nothing", () => {
    // kills: a scroll jump to the newest comment
    comments = [c({})];
    const { container, rerender } = view();
    const list = container.querySelector<HTMLElement>(".iv-cm-list")!;
    list.scrollTop = 120;
    comments = [c({}), c({ id: "ic-00000000000000c2", createdBy: ALICE })];
    rerender(<CommentsProvider><Comments issueId={WORK_IDS.ep163} me={DYLAN} people={[DYLAN, ALICE]} head={HEAD} /></CommentsProvider>);
    expect(list.scrollTop).toBe(120);
    expect(Element.prototype.scrollIntoView).not.toHaveBeenCalled();
  });

  it("my own post is the one scroll", () => {
    // kills: a composer that posts and leaves the person's comment off screen (and pins the ONE
    // scroll to the painted element named by the op's localId)
    submitDoc = { markdown: "mine", authorship: [] };
    const { rerender } = view();
    fireEvent.click(screen.getByRole("button", { name: "New comment (stub)" }));
    const localId = (mutate.mock.calls[0][0] as { localId: string }).localId;
    comments = [c({ id: localId, createdBy: DYLAN, body: { text: "mine", author: "H" } })];
    act(() => rerender(<CommentsProvider><Comments issueId={WORK_IDS.ep163} me={DYLAN} people={[DYLAN, ALICE]} head={HEAD} /></CommentsProvider>));
    expect(Element.prototype.scrollIntoView).toHaveBeenCalledTimes(1);
  });

  it("a thread with anchors, reactions and a resolved fold makes no prediction", () => {
    comments = [c({ reactions: [{ emoji: "✅", users: [ALICE] }] }),
      c({ id: "ic-00000000000000c3", resolvedAt: "2026-09-25T01:02:00Z", resolvedBy: DYLAN })];
    const { container } = view();
    expectNoPrediction(container);
  });
});
```

The size in the fold (*· 1 comment*, *· 3 comments*) is a count of records that exist, not a
prediction, and `expectNoPrediction` passes over it (task 20's rule is about forecasts). The
resolved-fold time is written as `\d\d:\d\d` because the local time zone of the test runner
is not fixed; `toLocaleTimeString` with `hour12: false` is the source.

#### Steps

- [ ] **Step 1: read on the sync base** task 49's `Editor.tsx` (`EditorProps`, `load()`, the
  `Mod-Enter` binding) and `authorClass.ts`, task 46's `issueData.ts` and `AuthorClass.tsx`, task
  47's `Serves.tsx` and `IssueView.tsx`, and task 56's `ops.ts` (`WorkOp`'s `comment`,
  `reaction`, `resolve` arms and `inverseOf`'s `resolve` case). Where one differs from what this
  part cites, the built file wins and the rename lands here.
- [ ] **Step 2: the two test files first**; run them and see them fail on the missing modules.
- [ ] **Step 3: `commentModel.ts`**; `commentModel.test.ts` passes (7 tests).
- [ ] **Step 4: the three `Editor` props** in task 49's `Editor.tsx`; task 49's `Editor.test.tsx`
  still passes unchanged (the props are optional).
- [ ] **Step 5: `mentionPlugin.ts`, `CommentComposer.tsx`, `Comments.tsx`**, the `issueData.ts`
  read and the `issue.css` rules; `Comments.test.tsx` passes (10 tests).
- [ ] **Step 6: the `IssueView.tsx` and `Serves.tsx` edits**; tasks 46-49's issue tests pass.
- [ ] **Step 7: commit** by path, one commit per step 3-6, each message ending with the
  `Co-Authored-By` line.

#### Acceptance

```bash
npm --prefix desktop test -- issue editor
# expect: every file passes; this part adds 2 files and 17 tests (commentModel 7, Comments 10)
npm --prefix desktop run typecheck
# expect: exit 0
grep -rn "scrollTo\|scrollTop *=\|dangerouslySetInnerHTML\|innerHTML" desktop/src/components/issue/Comments.tsx desktop/src/components/issue/CommentComposer.tsx | wc -l
# expect: 0
grep -c "scrollIntoView" desktop/src/components/issue/Comments.tsx
# expect: 1 (the person's own post, decision 8)
```

#### What this task does NOT claim

- That the Hub accepts each of the five reactions: `ValidReactionEmoji` runs in task 12's code,
  not here (decision 5, **UNVERIFIED**).
- That a reply landing above the viewport leaves the visible text still in WebView2: that is
  scroll anchoring, and the owner's eyes check it (decision 8). The test holds only that this code
  moves nothing.
- A brief-section anchor (decision 1), an edit record (decision 7), thread summaries (L-43, W4),
  comment drafts (L-44, W2), custom emoji (L-41's later half), W3's comment kinds (`note`,
  `evidence_proposal`, `negotiation`, SW.19), and agent-written comments before W3.
- That comments never enter an agent's managed block: task 19 tests that (05 §3.4).
- Span and hunk anchors being **created**: task 60 creates them through `useStartComment`; this
  part renders them.

**Parity.** Closes **L-42** and L-41's W1 part (a fixed reaction set, names not counts), per the
scope. Serves 05 §4.1 (the issue view) and §3.6 (who writes comments).

#### Gaps found while writing this part

1. **`mutate` has two homes.** Tasks 47, 48, 49 and this part import `mutate` from
   `work/replica/ops`; task 56 declares it as `Replica.mutate` in `replica.ts`
   (`task-56.md:347`). One module-level export is needed; task 56 owns the choice.
2. **Comment ids.** Task 20's fixtures and the scope say `ic-` (`task-20.md:161`); task 56's text
   says `cm-` (`task-56.md:164`, `:375`). Task 03's prefix decides.
3. **A sixth replica read**, `comments(issueId)`, for task 55 (beside task 46's four and task 47's
   `criterion`); task 56's `ReplicaReadView` already names it.
4. **Three `Editor` props** (`plugins`, `onSubmit`, `onSelectionChange`) added to task 49's file.
5. **SW.12 has no author mark**; the mark is task 46's `AuthorClass`, not `MarkGlyph`.
6. **SW.1 has no brief-section anchor kind and no edit record**, both of which the scope names.
7. **The File map** lists `Comments.tsx` only; `commentModel.ts`, `mentionPlugin.ts` and
   `CommentComposer.tsx` are this part's additions.
