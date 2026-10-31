import { useMemo } from 'react';
import type { OrchestratorContext, OrchestratorProposal } from '../../api/orchestrator';
import { amendmentImpact } from './amendmentImpact';

export function AmendmentImpact({ context, proposal }: { context: OrchestratorContext; proposal: OrchestratorProposal }) {
  const impact = useMemo(() => amendmentImpact(context, proposal), [context, proposal]);
  return <details className="orchestrator-amendment" aria-label="Recorded cross-stage impact">
    <summary>See linked work across the project</summary>
    <p>Based on the saved snapshot Luci used and this proposed draft. These are explicit links and review prompts, not current execution status or an automatic work selection.</p>
    {impact.notes.map(note => <p key={note}>{note}</p>)}
    {!!impact.requirements.length && <section><h3>Requirements to review</h3><ul>{impact.requirements.map(row => <li key={row.id}><strong>{row.label}</strong> ({row.id}) — {row.reason}</li>)}</ul>{impact.requirementsOmitted > 0 && <p>{impact.requirementsOmitted} additional linked requirements are omitted from this compact view.</p>}</section>}
    {impact.groups.map(group => <details key={group.id}><summary>{group.title} · {group.items.length + group.omitted} linked or directly changed</summary>
      {group.items.length > 0 ? <ul>{group.items.map(row => <li key={row.id}><strong>{row.label}</strong> ({row.id}) — {row.reason}</li>)}</ul> : <p>No affected record can be established from the explicit captured links.</p>}
      {group.omitted > 0 && <p>{group.omitted} additional linked records are omitted from this compact view.</p>}
      {group.notes.map(note => <p key={note}>{note}</p>)}
    </details>)}
    <p>Applying still uses the existing context checks, amendment review and separate approval/execution controls.</p>
  </details>;
}
