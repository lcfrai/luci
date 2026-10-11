import type { RuntimeAdmissionObservation } from "../../api/product";

/** Retained decision evidence only. Expanding this starts no probe or retry. */
export function RuntimeAdmission({ observation }: { observation?: RuntimeAdmissionObservation }) {
  if (!observation || !["initial", "before-build"].includes(observation.stage) || !Number.isFinite(Date.parse(observation.decidedAt))) return null;
  const o = observation;
  const reasons: string[] = [];
  if (o.samplerAvailable === false) reasons.push("The capacity monitor had no observation available.");
  else if (o.samplerAvailable === true) {
    if (o.hostFresh === false) reasons.push("The computer capacity observation was missing, expired or ahead of the decision time.");
    if (o.executionFresh === false) reasons.push("The Docker capacity observation was missing, expired or ahead of the decision time.");
    if (o.hostMemoryKnown === false) reasons.push("Available computer memory could not be measured.");
    if (o.hostCPUKnown === false) reasons.push("Computer CPU capacity could not be measured.");
    if (o.executionMemoryKnown === false) reasons.push("Docker memory capacity could not be measured.");
    if (o.executionCPUKnown === false) reasons.push("Docker CPU capacity could not be measured.");
    if (o.inventoryComplete === false) reasons.push("The Docker workload inventory was incomplete.");
    if (o.engineMatched === false) reasons.push("The observed Docker engine did not match the engine preparing this installation.");
  }
  if (o.reasonCode === "memory_pressure") reasons.push("The observed memory could not preserve the configured headroom. Existing workload and pending reservations still count.");
  if (o.reasonCode === "cpu_pressure") reasons.push("The observed CPU load exceeded the configured headroom limit.");
  if (o.reasonCode === "execution_limits_unknown") reasons.push("An active workload had neither verified limits nor fresh usage.");
  if (reasons.length === 0) return null;
  return <details className="journey-muted">
    <summary>Why preparation paused</summary>
    <p>Recorded at <time dateTime={o.decidedAt}>{new Date(o.decidedAt).toLocaleString()}</time>, {o.stage === "initial" ? "before reserving capacity" : "at the final check before building"}.</p>
    <ul>{reasons.map(reason => <li key={reason}>{reason}</li>)}</ul>
    <p>This is the saved decision, not current capacity. Reading it does not retry the installation or change resource limits.</p>
  </details>;
}
