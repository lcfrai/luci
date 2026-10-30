import type { DesignPlan } from '../../api/designPlan';
import type { ImplementationTask } from '../../api/product';

export function TaskDesignFrames({ task, index, design, disabled, loading, error, onChange }: {
  task: ImplementationTask; index: number; design: DesignPlan | null;
  disabled: boolean; loading: boolean; error: string;
  onChange(task: ImplementationTask): void;
}) {
  const frames = design?.scene?.frames ?? [];
  const approved = design?.status === 'approved' && !loading && !error;
  const selected = task.designFrameIds ?? [];
  function change(id: string, checked: boolean) {
    if (disabled || checked && !approved) return;
    onChange({ ...task, designFrameIds: checked ? [...selected, id] : selected.filter(value => value !== id) });
  }
  return <details className="task-requirements">
    <summary>Design frames for task {index + 1} · {selected.length} linked</summary>
    <p className="journey-muted">Optional: choose the screens this task should build or review. Backend and command-line tasks can leave this empty. The complete approved handoff remains available to the agent.</p>
    {loading && <p role="status">Reading the current design frames…</p>}
    {error && <p role="status">{error}</p>}
    {!approved && !loading && <p className="journey-muted">Approve the current Design handoff to add frame links. Saved links can be removed.</p>}
    {frames.map(frame => <label className="requirement-choice" key={frame.id}>
      <input type="checkbox" aria-label={`Link frame ${frame.id} to task ${index + 1}`} checked={selected.includes(frame.id)} disabled={disabled || !approved && !selected.includes(frame.id)} onChange={event => change(frame.id, event.target.checked)} />
      <span>{frame.name} · {frame.width} × {frame.height}<small>{frame.screenId}{frame.state && ` · ${frame.state}`} · {frame.id}</small></span>
    </label>)}
    {selected.filter(id => !frames.some(frame => frame.id === id)).map(id => <label className="requirement-choice" key={id}>
      <input type="checkbox" aria-label={`Remove unavailable frame ${id} from task ${index + 1}`} checked disabled={disabled} onChange={() => change(id, false)} />
      <span>{id} · {design ? 'Unavailable in the current handoff. Remove or replace this link.' : 'Saved link; refresh Design to inspect it.'}</span>
    </label>)}
    {!loading && frames.length === 0 && selected.length === 0 && <p className="journey-muted">No visual frames selected. A task does not need a frame to be valid.</p>}
  </details>;
}
