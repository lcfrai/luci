import { useEffect, useRef, useState } from "react";
import type { CSSProperties, JSX } from "react";
import { SessionPane } from "./SessionPane";
import { selectFocusedSessionId } from "../../store/selectors";
import { useCockpit } from "../../store/store";
import type { SessionId } from "../../store/types";
import { Button } from "../fluid/components/button";

export const MAX_MOUNTED_COLUMN_TERMINALS = 6;
function InactiveColumn({ sessionId }: { sessionId: SessionId }) {
  const session = useCockpit(state => state.sessions[sessionId]);
  return <section className="pane cols-inactive" aria-label={`${session?.title ?? "Session"} — output not open`}>
    <h3>{session?.title ?? "Session"}</h3>
    <p>{session?.status ?? "Not listed"}</p>
    <p>Work continues while this view is closed. Reopening restores recent output only; earlier terminal history may be unavailable.</p>
    <Button variant="secondary" onClick={() => useCockpit.getState().focusSession(sessionId)}>Open session</Button>
  </section>;
}

export function ColumnsLayout({ sessionIds, singleSessionId, paneWidth = 520 }: { sessionIds: SessionId[]; singleSessionId?: SessionId; paneWidth?: number }): JSX.Element {
  const focusedId = useCockpit(selectFocusedSessionId);
  const focusSession = useCockpit((s) => s.focusSession);
  const scrollRef = useRef<HTMLDivElement>(null);
  const [visibleIds, setVisibleIds] = useState<SessionId[]>(() => sessionIds.slice(0, 2));
  // Keep inexpensive column placeholders in server order, but allocate terminal
  // buffers and sockets only for visible output and an explicitly focused pane.
  // The cap also bounds unusually wide windows and observer delivery bursts.
  const retained = useRef<SessionId[]>([]);
  const primary = singleSessionId ?? focusedId;
  const mounted = new Set([...(primary && sessionIds.includes(primary) ? [primary] : []),
    ...(singleSessionId ? retained.current : []), ...visibleIds].filter((id, index, all) => sessionIds.includes(id) && all.indexOf(id) === index).slice(0, MAX_MOUNTED_COLUMN_TERMINALS));
  // Closing sibling views must not tear down their terminals. Retain the same
  // bounded working set in Focus/narrow mode; a seventh opened output still
  // follows the existing six-terminal resource ceiling.
  useEffect(() => { retained.current = [...mounted]; });
  useEffect(() => {
    const host = scrollRef.current;
    if (!host || singleSessionId || typeof IntersectionObserver === "undefined") return;
    let live = true;
    const visible = new Set<string>();
    const observer = new IntersectionObserver(entries => {
      if (!live) return;
      for (const entry of entries) {
        const id = (entry.target as HTMLElement).dataset.columnFor;
        if (!id) continue;
        if (entry.isIntersecting) visible.add(id); else visible.delete(id);
      }
      const next = sessionIds.filter(id => visible.has(id));
      setVisibleIds(previous => previous.length === next.length && previous.every((id, i) => id === next[i]) ? previous : next);
    }, { root: host, threshold: 0.01 });
    Array.from(host.children).forEach(pane => observer.observe(pane));
    return () => { live = false; observer.disconnect(); };
  }, [sessionIds, singleSessionId]);
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
    <div className={`cols-scroll${singleSessionId ? ' cols-scroll--single' : ''}`} ref={scrollRef} data-testid="cols-scroll" style={{ '--pane-w': `${paneWidth}px` } as CSSProperties}>
      {sessionIds.map((id) => (
        <div className="cols-pane" key={id} data-column-for={id} hidden={!!singleSessionId && id !== singleSessionId}>
          {mounted.has(id) ? <SessionPane sessionId={id} variant="column" /> : <InactiveColumn sessionId={id} />}
        </div>
      ))}
    </div>
  );
}
