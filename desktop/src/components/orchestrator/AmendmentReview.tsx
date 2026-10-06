import { useState } from "react";
import type { OrchestratorTurn, OrchestratorView } from "../../api/orchestrator";
import { Button } from "../fluid/components/button";
import { WorkflowDialog } from "../common/WorkflowDialog";

const names = { specification: "definition", "design-plan": "design handoff", plan: "implementation plan" };
const metadata = new Set(["projectId", "version", "revision", "briefRevision", "specificationRevision", "specificationDigest", "sketchRevision", "status", "digest", "updatedAt"]);
const label = (value: string) => value.replace(/([A-Z])/g, " $1").replace(/^./, first => first.toUpperCase());
function readable(value: unknown, depth = 0): string {
  if (value == null || value === "") return "Not set";
  if (Array.isArray(value)) return value.length ? value.map(item => `${"  ".repeat(depth)}• ${readable(item, depth + 1)}`).join("\n") : "None";
  if (typeof value === "object") return Object.entries(value).map(([key, child]) => `${label(key)}: ${readable(child, depth + 1)}`).join("\n");
  return String(value);
}

export function AmendmentReview({ turn, view, busy, blocked, error, onClose, onApply }: {
  turn: OrchestratorTurn; view: OrchestratorView; busy: boolean; blocked: string; error: string;
  onClose(): void; onApply(): void;
}) {
  const [acknowledged, setAcknowledged] = useState(false);
  const proposal = turn.answer?.proposal;
  if (!proposal) return null;
  const property = proposal.target === "design-plan" ? "designPlan" : proposal.target;
  const before = turn.context[property] as unknown as Record<string, unknown>;
  const after = proposal[property] as unknown as Record<string, unknown> | null;
  const changes = after ? Object.keys(after).filter(key => !metadata.has(key) && JSON.stringify(before[key]) !== JSON.stringify(after[key])) : [];
  const stale = turn.contextDigest !== view.context.digest;
  return <WorkflowDialog title={`Review ${names[proposal.target]} amendment`} description="Compare the saved context Luci used with the proposed draft. Applying does not approve the document, start coding or deploy anything." busy={busy} onClose={onClose}>
    <p>{proposal.summary}</p>
    {turn.answer?.rationale && <p><strong>Why this change: </strong>{turn.answer.rationale}</p>}
    {!!turn.answer?.impact?.length && <div><strong>Affected work</strong><ul>{turn.answer.impact.map((impact, index) => <li key={index}>{impact}</li>)}</ul></div>}
    <div className="orchestrator-amendment" aria-label="Proposed changes">
      {changes.length ? changes.map(key => <section key={key}><h3>{label(key)}</h3><div className="orchestrator-comparison">
        <div><h4>Saved when asked</h4><p>{readable(before[key])}</p></div>
        <div><h4>Proposed draft</h4><p>{readable(after![key])}</p></div>
      </div></section>) : <p>No document content changes were proposed.</p>}
    </div>
    {stale && <p role="alert">The saved context changed after this proposal. Ask Luci to reconcile it with the current context before applying.</p>}
    {blocked && <p role="status">{blocked}</p>}
    {error && <p role="alert">{error}</p>}
    <label className="orchestrator-ack"><input type="checkbox" checked={acknowledged} disabled={busy} onChange={event => setAcknowledged(event.target.checked)} />I reviewed these changes. Save them as a draft and clear any affected approvals.</label>
    <div className="workflow-actions"><Button variant="secondary" disabled={busy} onClick={onClose}>Keep current context</Button><Button disabled={busy || !acknowledged || !!blocked || stale || !changes.length} onClick={onApply}>{busy ? "Applying amendment…" : "Apply reviewed amendment"}</Button></div>
  </WorkflowDialog>;
}
