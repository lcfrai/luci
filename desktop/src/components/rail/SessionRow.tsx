// Contract §6.2: status dot + harness command + title. Click FOCUSES the session; it does not
// scroll anything. §6.8 gives horizontal scroll to task 20's ColumnsLayout, which reacts to
// the focus change — a row that called scrollIntoView itself would be a second owner.
import type { JSX } from "react";
import { useCockpit } from "../../store/store";
import { selectHarness, selectSession } from "../../store/selectors";
import { StatusDot } from "../common/StatusDot";
import type { SessionId } from "../../store/types";
import "./rail.css";

export function SessionRow(props: { sessionId: SessionId; selected: boolean }): JSX.Element {
  const { sessionId, selected } = props;
  const session = useCockpit(selectSession(sessionId));
  const command = useCockpit((s) => {
    const found = session === undefined ? undefined : selectHarness(session.harness)(s);
    return found?.command ?? session?.harness ?? "";
  });
  const focusSession = useCockpit((s) => s.focusSession);
  const openContextMenu = useCockpit((s) => s.openContextMenu);

  if (session === undefined) return <></>;

  return (
    <div
      role="treeitem"
      tabIndex={-1}
      aria-level={4}
      aria-selected={selected}
      data-kind="session"
      data-id={session.id}
      className="rail-row rail-level-4"
      onClick={() => focusSession(session.id)}
      onContextMenu={(event) => {
        event.preventDefault();
        openContextMenu({ kind: "session", id: session.id, x: event.clientX, y: event.clientY });
      }}
    >
      {/* TASK 22 NOTE — the rail's session verbs arrive through ContextMenu.tsx's
          sessionItems(); this row's dispatch is already complete and needs no change. If task
          22 wants an optimistic "kill in flight" affordance on the row, it belongs here and
          nowhere else. Anchored guard if it adds one:
            grep -cE '^[[:space:]]+data-pending=' src/components/rail/SessionRow.tsx */}
      <StatusDot
        status={session.status}
        live={session.status === "running"}
        label={`${session.status} · ${session.title}`}
      />
      <span className="rail-name">{command}</span>
      <span className="rail-name">{session.title}</span>
    </div>
  );
}
