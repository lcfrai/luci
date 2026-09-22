// The one status dot. Contract §6.7 and §7.5: colour PLUS shape, and the shared heartbeat
// phase when live. This is the ONLY component that may call beatDelayStyle().
import { useEffect, useRef, useState } from "react";
import type { JSX } from "react";
import { beatDelayStyle } from "../../hooks/heartbeat";
import type { MachineState, SessionStatus } from "../../store/types";
import "./common.css";

/** Four session statuses and six machine states. `running` and `idle` are in both, so the
 *  union has eight members. docs/AGENTIC-UX.md §4 rule 8 lists seven session statuses; only
 *  EXECUTING and DONE have mechanical analogues in this phase, and the OTHER FIVE need the
 *  agent-runtime signal Phase 2 forbids, so they are not in SessionStatus and must not be
 *  rendered here. Contract §0 is the count that is right; §2's comment above SessionStatus
 *  says three, and types.ts keeps it because task 15 diffs that file against the contract
 *  byte for byte. */
export type DotStatus = SessionStatus | MachineState;

const SHAPE: Record<DotStatus, "dot-ring" | "dot-disc" | "dot-square"> = {
  starting: "dot-ring",
  provisioning: "dot-ring",
  sleeping: "dot-ring",
  running: "dot-disc",
  idle: "dot-disc",
  exited: "dot-square",
  stopped: "dot-square",
  error: "dot-square",
};

const FLASH_MS = 600;

export function StatusDot(props: {
  status: DotStatus;
  live: boolean;
  label: string;
}): JSX.Element {
  const { status, live, label } = props;
  // Initialised to the FIRST status, so a fresh mount never flashes. Forty rail rows all
  // flashing when the window opens is a light show, not a signal.
  const previous = useRef<DotStatus>(status);
  const [flashing, setFlashing] = useState(false);

  useEffect(() => {
    if (previous.current === status) return undefined;
    previous.current = status;
    setFlashing(true);
    const timer = setTimeout(() => setFlashing(false), FLASH_MS);
    return () => clearTimeout(timer);
  }, [status]);

  const className = ["dot", SHAPE[status], `dot-tone-${status}`]
    .concat(live ? ["dot-live"] : [])
    .concat(flashing ? ["dot-flash"] : [])
    .join(" ");

  return (
    <span
      role="img"
      aria-label={label}
      title={label}
      className={className}
      style={live ? beatDelayStyle() : undefined}
    />
  );
}
