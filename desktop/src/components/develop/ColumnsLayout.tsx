import { useEffect, useRef } from "react";
import type { JSX } from "react";
import { SessionPane } from "./SessionPane";
import { selectFocusedSessionId } from "../../store/selectors";
import { useCockpit } from "../../store/store";
import type { SessionId } from "../../store/types";

export function ColumnsLayout({ sessionIds }: { sessionIds: SessionId[] }): JSX.Element {
  const focusedId = useCockpit(selectFocusedSessionId);
  const focusSession = useCockpit((s) => s.focusSession);
  const scrollRef = useRef<HTMLDivElement>(null);
  /** null until the first effect pass, so the effect below can tell "mounted with nine
   *  sessions" from "nine sessions just arrived".
   *
   *  MEASURED EQUIVALENCE, and the sentence that says when it expires. Replacing this with
   *  `useRef(new Set(sessionIds))` survives the whole suite, and it survives correctly: React
   *  runs an effect with the same props the render that built the ref committed, so on the
   *  first pass `fresh` is empty either way and neither form steals focus. The plan's step 14
   *  says the pre-seeded form would make "the second pass see no new one" — that is not what
   *  happens; the second pass still sees s-2.
   *
   *  It stops being equivalent the moment the ref's initialiser can read a DIFFERENT id list
   *  from the one the first effect pass sees — if this ref is hoisted out of the component, if
   *  `sessionIds` is ever computed inside the effect rather than taken from props, or if the
   *  effect below is given a dependency array that can run it before its own render commits.
   *  Any of those, and only the null sentinel still distinguishes the seeding pass. */
  const seen = useRef<Set<SessionId> | null>(null);

  // Defect 4, half one: a session that appears while the app is open takes focus.
  useEffect(() => {
    const previous = seen.current;
    seen.current = new Set(sessionIds);
    if (previous === null) return; // the seeding pass never steals focus
    const fresh = sessionIds.filter((id) => !previous.has(id));
    if (fresh.length === 0) return;
    // `fresh[0]` is the first new id in THIS LIST'S order, which is `selectPaneSessionIds`'
    // order: worktree by worktree in server order, and within each worktree its sessions in
    // server order (createdAt descending). So it is the newest only WITHIN one worktree — not
    // across them, because the concatenation never re-sorts. Two sessions arriving on one poll
    // on two different worktrees are ordered by worktree, not by age. Low stakes: it decides
    // only which of several simultaneous arrivals takes focus, and the client must not re-sort
    // (§3.2). Stated exactly because the earlier wording claimed a property the data lacks.
    focusSession(fresh[0]);
  }, [sessionIds, focusSession]);

  // Defect 4, half two: the focused pane is brought into the viewport, whoever focused it.
  // Kept SEPARATE from the effect above on purpose — §6.8 says ⌘1-9, the palette and a rail
  // row "request it via focusSession; they never call scrollIntoView themselves", which is
  // only true if the scroll is driven by the focused id rather than by the create.
  useEffect(() => {
    if (focusedId === null) return;
    const host = scrollRef.current;
    if (host === null) return;
    // Walked rather than queried: session ids are server-generated and a `querySelector` on
    // an interpolated id would need CSS.escape the day one contains a character that needs it.
    const pane = Array.from(host.children).find(
      (el) => (el as HTMLElement).dataset.columnFor === focusedId,
    );
    // `inline: center` on the horizontal axis, `block: nearest` so a pane never drags the
    // page vertically (§6.8: this is the only scrollIntoView call in Group D).
    (pane as HTMLElement | undefined)?.scrollIntoView({ inline: "center", block: "nearest" });
  }, [focusedId, sessionIds]);

  return (
    <div className="cols-scroll" ref={scrollRef} data-testid="cols-scroll">
      {sessionIds.map((id) => (
        <div className="cols-pane" key={id} data-column-for={id}>
          <SessionPane sessionId={id} variant="column" />
        </div>
      ))}
    </div>
  );
}
