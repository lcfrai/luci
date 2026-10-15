import type { Session } from "../../store/types";
import type { SessionNativeTurn } from "../../api/client";

/** Native status is not a source diff, accepted candidate, or check receipt. */
export function implementationOutcomeMessage(session: Session, nativeTurn?: SessionNativeTurn): string | null {
  const outcome = session.implementationOutcome;
  if (!session.implementationContext || !outcome) return null;
  if ((session.status === "running" || session.status === "idle") && nativeTurn?.state === "finished" && nativeTurn.turnId && session.harness === "opencode") {
    if (outcome.evidence === "session-only" && (nativeTurn.toolClass === "none" || nativeTurn.toolClass === "reads-only")) {
      return "The latest OpenCode turn ended while this Ask session remains open. Its native tools only read files or did not run; no implementation or checks are linked to this task. Open the terminal to review any report or continue. If coding is still needed, stop this session before trying the saved task with a suitable agent connection.";
    }
    return "The latest OpenCode turn ended while this Ask session remains open. Review its retained work and run checks; a finished turn does not verify the task.";
  }
  switch (outcome.state) {
    case "starting":
      return "The coding environment is starting. No implementation result has been confirmed.";
    case "working":
      return "The coding agent is running. Open its terminal to review progress and Ask permissions; code and checks are not yet verified.";
    case "quiet":
      return "No recent terminal output. The agent may still be working or waiting for Ask permission. Open its terminal; if progress has stopped, use its owned stop control before another attempt.";
    case "unknown":
      return "The coding outcome is unconfirmed. Inspect the retained session and resolve its runtime state before starting another attempt.";
    case "stopped":
      return "This coding attempt was stopped. Its reviewed task stays open; inspect retained work before choosing another attempt.";
    case "failed":
      return "The coding environment failed before a reviewable result. Read the recorded reason and check the agent connection or local capacity before another attempt; the reviewed task stays open.";
    case "ended":
      return outcome.evidence === "project-run"
        ? "The coding process ended. Open the reviewed build to check its recorded candidate and independent checks; terminal output alone does not verify the task."
        : "No candidate or passing checks are linked to this coding session. Inspect the retained work and run checks. If no implementation was produced, try the saved task again with a suitable agent connection; read-only reports remain available in the terminal.";
  }
}
