import type { JSX } from "react";
import { useAttachState } from "../../api/attachRegistry";
import type { AttachState } from "../../api/attachRegistry";
import { useHeartbeat } from "../../hooks/heartbeat";
import { formatElapsed } from "../../lib/time";
import { selectHarness } from "../../store/selectors";
import { useCockpit } from "../../store/store";
import type { Session } from "../../store/types";

/** Whole seconds since the current ladder started, "" when none is running.
 *
 *  Seconds and not `formatElapsed`: the retry budget is 60 s (`RETRY_BUDGET_MS`), and
 *  `formatElapsed` floors to the minute, so it prints "0m" for the entire life of a ladder —
 *  a number that never moves, which is the spinner §7.7 is asking us not to ship. Local to
 *  this file rather than added to `lib/time.ts`: it is a display choice for one bar, and a
 *  second exported duration formatter invites two surfaces to disagree about the unit. */
function ladderElapsed(state: AttachState, nowMs: number): string {
  if (state.ladderStartedAt === 0) return "";
  return ` ${String(Math.max(0, Math.floor((nowMs - state.ladderStartedAt) / 1000)))}s`;
}

/** One word per phase, exhaustive over AttachPhase, plus the retry elapsed while a ladder is
 *  running. "failed" reads as "not attached" because the session itself is alive — the socket
 *  is what failed, and telling a user their agent failed when it is still working is the lie
 *  this wording avoids.
 *
 *  `nowMs` is the second parameter because contract §6.6 puts the arithmetic HERE: the registry
 *  publishes the raw `ladderStartedAt` stamp and owns no clock, so the pane subtracts it from
 *  `useHeartbeat()`'s 1 Hz tick at render time. `attempts` is deliberately not rendered —
 *  §7.7 asks for "the elapsed retry time, not a spinner", and a count of handshakes tells a
 *  user nothing about how long they have been waiting. Exported so a test can name a phase
 *  rather than match prose. */
export function attachLabel(state: AttachState, nowMs: number): string {
  switch (state.phase) {
    case "detached":
      return "detached";
    case "preparing":
      return "preparing";
    case "connecting":
      return `connecting${ladderElapsed(state, nowMs)}`;
    case "attached":
      return "attached";
    case "reconnecting":
      return `reconnecting${ladderElapsed(state, nowMs)}`;
    case "ended":
      return "ended";
    case "failed":
      return "not attached";
  }
}

export function SessionPaneFooter({ session }: { session: Session }): JSX.Element {
  const nowMs = useHeartbeat();
  const harness = useCockpit(selectHarness(session.harness));
  const attach = useAttachState(session.id);

  // harness.command is the fact; session.harness (the id) is the honest fallback while the
  // catalogue is still loading. Never a hard-coded "claude".
  const facts: string[] = [harness?.command ?? session.harness];
  if (session.model !== "") facts.push(session.model);
  facts.push(formatElapsed(session.createdAt, nowMs));
  facts.push(attachLabel(attach, nowMs));

  return (
    <footer className="pane-foot" title={attach.reason === "" ? undefined : attach.reason}>
      {facts.map((fact, i) => (
        <span key={fact + String(i)} className="pane-foot-fact">
          {i > 0 ? <span className="pane-foot-sep" aria-hidden="true">{" · "}</span> : null}
          {fact}
        </span>
      ))}
    </footer>
  );
}
