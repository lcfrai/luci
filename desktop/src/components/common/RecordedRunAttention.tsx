import { useId, useState } from "react";
import type { ProjectRunAttention } from "../../api/runAttention";
import { Button } from "../fluid/components/button";
import "./recorded-run-attention.css";

export const reviewKindLabel: Record<ProjectRunAttention["kind"], string> = {
  "task-review": "Task changes to review", "task-question": "Review reported task questions", verification: "Verification to complete",
  "release-review": "Release review", blocked: "Blocked work to inspect",
};
const contextLabels = { current: "Matches current context", changed: "Context has changed", unavailable: "Context comparison unavailable" };

export function RecordedRunAttention({ items, count = items.length, omitted = 0, stale = false, initialLimit = 3, compact = false, onInspect, onOpenHistory }: {
  items: ProjectRunAttention[]; count?: number; omitted?: number; stale?: boolean; initialLimit?: number; compact?: boolean;
  onInspect(runId: string): void; onOpenHistory?(): void;
}) {
  const [limit, setLimit] = useState(initialLimit);
  const heading = useId();
  if (count === 0) return null;
  const label = stale ? "Last recorded review work" : "Recorded review work";
  return <section className={`recorded-run-attention${compact ? ' recorded-run-attention-compact' : ''}`} aria-label={compact ? label : undefined} aria-labelledby={compact ? undefined : heading}>
    <h3 id={heading}>{compact ? `${stale ? 'Last recorded: ' : ''}${count} ${count === 1 ? 'build needs' : 'builds need'} review` : label}</h3>
    {!compact && <p>{count} {count === 1 ? "build needs" : "builds need"} review. Preparing another build does not resolve earlier work.</p>}
    <ul>{items.slice(0, limit).map(run => <li key={run.id}>
      <div><strong>{reviewKindLabel[run.kind]}</strong><p>{stale ? "Last recorded: " : ""}{contextLabels[run.contextStatus]}</p>{run.kind === "task-question" && <p>Unanswered agent questions remain for review; this does not authorize or block execution.</p>}{run.state === "cancelled" && <p>Further dispatch cancelled; retained changes remain.</p>}{run.state === "complete" && <p>Accepted build; reported questions still need review.</p>}<code>{run.id}</code></div>
      {!stale && <Button variant="secondary" size="compact" aria-label={`Inspect ${run.id}: ${reviewKindLabel[run.kind]}`} onClick={() => onInspect(run.id)}>Inspect build</Button>}
    </li>)}</ul>
    {stale ? <p>Refresh project context before inspecting this recorded work.</p> : <div className={compact ? 'recorded-run-attention-actions' : undefined}>
      {items.length > limit && <Button variant="ghost" aria-label={`Show more review work (${items.length - limit} remaining)`} onClick={() => setLimit(value => value + 10)}>{compact ? `Show ${items.length - limit} more` : `Show more review work (${items.length - limit} remaining)`}</Button>}
      {omitted > 0 && onOpenHistory && <Button variant="ghost" aria-label="See all saved builds" onClick={onOpenHistory}>{compact ? 'All saved builds' : 'See all saved builds'}</Button>}
      {omitted > 0 && <p>{omitted} more {omitted === 1 ? "build needs" : "builds need"} review beyond this summary.</p>}
    </div>}
  </section>;
}
