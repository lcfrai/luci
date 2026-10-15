import { useId, useState } from "react";
import type { ImplementationTask } from "../../api/product";

export function TaskScope({ task, index, tasks, disabled, onChange }: {
  task: ImplementationTask; index: number; tasks: ImplementationTask[]; disabled: boolean;
  onChange(task: ImplementationTask): void;
}) {
  const [open, setOpen] = useState(false);
  const id = useId();
  const waiting = task.dependsOn ?? [];
  const candidates = tasks.filter(item => item.id !== task.id);
	const codingCandidates = candidates.filter(item => item.role !== 'reviewer');
	const reviewer = task.role === 'reviewer';
  const missing = waiting.filter(dependency => !candidates.some(item => item.id === dependency));
  const toggle = (dependency: string, checked: boolean) => onChange({ ...task, dependsOn: checked ? [...waiting, dependency] : waiting.filter(value => value !== dependency) });
  return <details className="task-scope" onToggle={event => setOpen(event.currentTarget.open)}>
    <summary>Scope and dependencies for task {index + 1} · {task.acceptance?.trim() ? "Verification described" : "Verification needed"} · {waiting.length} {waiting.length === 1 ? "dependency" : "dependencies"}</summary>
    {open && <div className="task-scope-fields">
	  <label className="luci-field"><span>Task role</span><select disabled={disabled} value={reviewer ? 'reviewer' : 'coding'} onChange={event => {
	    if (event.target.value === 'reviewer') onChange({...task, role: 'reviewer', reviewOfTaskId: codingCandidates[0]?.id ?? '', writePaths: [], dependsOn: []});
	    else onChange({...task, role: 'coding', reviewOfTaskId: ''});
	  }}><option value="coding">Implement code</option><option value="reviewer">Independently review a saved coding task</option></select></label>
	  {reviewer && <label className="luci-field"><span>Candidate task to review</span><select disabled={disabled} value={task.reviewOfTaskId ?? ''} onChange={event => onChange({...task, reviewOfTaskId: event.target.value})}><option value="">Choose a saved coding task</option>{codingCandidates.map(candidate => <option key={candidate.id} value={candidate.id}>{candidate.title || candidate.id}</option>)}</select></label>}
      <label className="luci-field"><span>How to verify task {index + 1}</span>
        <textarea rows={3} disabled={disabled} value={task.acceptance ?? ""} aria-describedby={`${id}-acceptance`} onChange={event => onChange({ ...task, acceptance: event.target.value })} placeholder="Describe the observable result and the checks that prove it." />
      </label>
      <p id={`${id}-acceptance`} className="journey-muted">Include the expected behavior and important failure cases. This is an instruction to verify, not evidence that verification passed.</p>
	  {!reviewer && <><label className="luci-field"><span>Intended files and folders for task {index + 1}</span>
        <textarea rows={3} disabled={disabled} value={(task.writePaths ?? []).join("\n")} aria-describedby={`${id}-paths`} onChange={event => onChange({ ...task, writePaths: event.target.value.split("\n") })} placeholder={"src/features/editor\ntests/editor.test.ts"} spellCheck={false} />
      </label>
      <p id={`${id}-paths`} className="journey-muted">One project-relative file or folder per line, using / separators. A folder includes its contents. Use concrete paths without wildcards or a trailing slash. This declares intended changes; it is not a filesystem permission boundary.</p>
      <fieldset className="task-dependencies" disabled={disabled}>
        <legend>Task {index + 1} waits for</legend>
        {candidates.map(candidate => <label key={candidate.id}><input type="checkbox" aria-label={`${candidate.title || "Untitled task"} · Task ${tasks.indexOf(candidate) + 1}`} checked={waiting.includes(candidate.id)} onChange={event => toggle(candidate.id, event.target.checked)} /><span>{candidate.title || "Untitled task"}<small className="journey-muted"> · Task {tasks.indexOf(candidate) + 1}</small></span></label>)}
        {missing.map(dependency => <label key={dependency}><input type="checkbox" checked onChange={() => toggle(dependency, false)} /><span>{dependency} · Unavailable task. Remove this dependency before saving.</span></label>)}
        {!candidates.length && !missing.length && <p className="journey-muted">Add another task to define a dependency.</p>}
      </fieldset></>}
    </div>}
  </details>;
}
