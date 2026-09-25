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
