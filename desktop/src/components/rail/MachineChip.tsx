// Contract §6.2: the machine name, its state dot, and the `unsandboxed` badge when
// isolation !== "microvm". Click opens the machines popover. Limitations are honest
// sentences rendered VERBATIM and never summarised into a badge (§2).
import { useEffect, useRef, useState } from "react";
import type { JSX } from "react";
import { useCockpit } from "../../store/store";
import { StatusDot } from "../common/StatusDot";
import { Popover } from "../common/Popover";
import type { Machine } from "../../store/types";
import "./rail.css";

export function MachineChip(props: { machine: Machine | undefined }): JSX.Element {
  const { machine } = props;
  const machinesOpen = useCockpit((s) => s.machinesOpen);
  const openMachines = useCockpit((s) => s.openMachines);
  const closeMachines = useCockpit((s) => s.closeMachines);
  const ref = useRef<HTMLButtonElement | null>(null);
  const [anchor, setAnchor] = useState<DOMRect | null>(null);

  useEffect(() => {
    setAnchor(machinesOpen && ref.current !== null ? ref.current.getBoundingClientRect() : null);
  }, [machinesOpen]);

  if (machine === undefined) {
    return <div className="rail-empty">No machine yet. The control plane reports none.</div>;
  }

  return (
    <>
      <button
        ref={ref}
        type="button"
        className="rail-machine"
        aria-haspopup="dialog"
        aria-expanded={machinesOpen}
        data-overlay-trigger=""
        onClick={() => (machinesOpen ? closeMachines() : openMachines())}
      >
        <StatusDot
          status={machine.state}
          live={machine.state === "running"}
          label={`${machine.state} · ${machine.name}`}
        />
        <span className="rail-name">{machine.name}</span>
        {machine.isolation === "microvm" ? null : (
          <span className="rail-badge">unsandboxed</span>
        )}
      </button>
      <Popover anchor={anchor} onClose={closeMachines}>
        <div className="rail-machine-pop">
          <p>
            {machine.name} · {machine.state} · {machine.backend}
          </p>
          {machine.message === undefined ? null : <p>{machine.message}</p>}
          <ul>
            {machine.limitations.map((limitation) => (
              <li key={limitation}>{limitation}</li>
            ))}
          </ul>
        </div>
      </Popover>
    </>
  );
}
