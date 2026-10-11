// Contract §6.2: status dot + harness command + title. Click OPENS the existing session; it does not
// scroll anything. §6.8 gives horizontal scroll to task 20's ColumnsLayout, which reacts to
// the focus change — a row that called scrollIntoView itself would be a second owner.
import type { JSX } from "react";
import { useCockpit } from "../../store/store";
import { selectHarness, selectSession } from "../../store/selectors";
import { StatusDot } from "../common/StatusDot";
import { useSessionPending } from "../develop/sessionVerbs";
import type { SessionId } from "../../store/types";
import "./rail.css";

export function SessionRow(props: { sessionId: SessionId; selected: boolean }): JSX.Element {
  const { sessionId, selected } = props;
  const session = useCockpit(selectSession(sessionId));
  // A deferred commit is in flight for this row. During the Undo window the row already shows
  // the optimistic state — a killed session reads `exited` — but nothing else says the request
  // has not been sent yet, so a user who clicks Kill twice in six seconds cannot see why the
  // second click changed nothing.
  const pending = useSessionPending(sessionId);
  const command = useCockpit((s) => {
    const found = session === undefined ? undefined : selectHarness(session.harness)(s);
    return found?.command ?? session?.harness ?? "";
  });
  const openSession = useCockpit((s) => s.openSession);
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
      // `undefined` omits the attribute entirely rather than rendering data-pending="false",
      // which keeps a CSS selector written later honest and keeps the DOM snapshot in task 24's
      // defect suite unchanged for the ordinary case. The opacity is inline because rail.css is
      // task 18's and shared-shape §2 sanctions only menu/ CSS for this task; it carries no
      // transition, so the change is instant — correct here, because the fade is a state
      // readout and not a causal animation.
      data-pending={pending ? "true" : undefined}
      style={pending ? { opacity: 0.55 } : undefined}
      className="rail-row rail-level-4"
      onClick={() => openSession(session.id)}
      onContextMenu={(event) => {
        event.preventDefault();
        openContextMenu({ kind: "session", id: session.id, x: event.clientX, y: event.clientY });
      }}
    >
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
