import type { JSX } from "react";
import { StatusDot } from "../common/StatusDot";
import { selectHarness } from "../../store/selectors";
import { useCockpit } from "../../store/store";
import type { Session, Worktree } from "../../store/types";

/** The screen-reader and hover sentence for the dot. `exited` carries its reason because
 *  "exited" alone does not tell a user whether they killed it or it fell over. */
export function statusLabel(session: Session): string {
  if (session.status === "exited" && session.exitReason !== "") {
    return `exited — ${session.exitReason}`;
  }
  return session.status;
}

export function SessionPaneHeader({
  session,
  worktree,
  variant,
}: {
  session: Session;
  worktree: Worktree | undefined;
  variant: "column" | "tile";
}): JSX.Element {
  const harness = useCockpit(selectHarness(session.harness));
  const openSessionMenu = useCockpit((s) => s.openSessionMenu);
  const closeSessionMenu = useCockpit((s) => s.closeSessionMenu);
  const sessionMenuFor = useCockpit((s) => s.sessionMenuFor);

  return (
    <header
      className={variant === "tile" ? "pane-head pane-head--tile" : "pane-head"}
      data-pane-drag-handle={variant === "tile" ? "true" : undefined}
    >
      <StatusDot status={session.status} live={session.status === "running"} label={statusLabel(session)} />
      <span className="pane-title">{worktree?.name ?? session.title}</span>
      <span className="pane-sub">{harness?.name ?? session.harness}</span>
      {/* TASK 22 — this button is already the complete overlay trigger task 22's step 12 was
          written to add, so that step is a NO-OP: read it before editing. Its own guard,
          `grep -cE '^[[:space:]]+data-overlay-trigger=""' SessionPaneHeader.tsx`, still reads
          the 1 it measured. task-22.md:1389 says the lines belong here "if task 20 is ever
          re-run", and this run carried them, because useDismissable's contract is TWO
          attributes: `data-overlay-trigger` scoped by an `aria-expanded` of "true". With
          either missing, its capture-phase pointerdown closes the menu and the click that
          follows re-opens it, so the button can never close what it opened. The toggle in
          onClick is the third half of the same requirement — the trigger must own it. */}
      <button
        type="button"
        className="pane-more"
        // Task 22 reads this attribute to anchor SessionMenu; see the plan's step 6.
        data-session-menu-anchor={session.id}
        data-overlay-trigger=""
        aria-haspopup="menu"
        aria-expanded={sessionMenuFor === session.id}
        aria-label={`Session menu for ${worktree?.name ?? session.title}`}
        onClick={() =>
          sessionMenuFor === session.id ? closeSessionMenu() : openSessionMenu(session.id)
        }
      >
        {"⋯"}
      </button>
    </header>
  );
}
