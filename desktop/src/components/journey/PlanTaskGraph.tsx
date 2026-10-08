import { Fragment } from "react";
import type { ImplementationPlan, ImplementationTask } from "../../api/product";
import "./plan-task-graph.css";

/** Only structural mistakes block saving. Missing scope remains valid for legacy plans. */
export function taskDependencyIssue(tasks: ImplementationTask[]): string {
  const byId = new Map(tasks.map(task => [task.id, task]));
  for (const task of tasks) {
    if (task.dependsOn?.some(id => !byId.has(id))) return "A task still depends on a removed task. Remove the unavailable dependency before saving.";
  }
  const active = new Set<string>(), complete = new Set<string>();
  function cycles(id: string): boolean {
    if (active.has(id)) return true;
    if (complete.has(id)) return false;
    active.add(id);
    if (byId.get(id)?.dependsOn?.some(cycles)) return true;
    active.delete(id); complete.add(id);
    return false;
  }
  return tasks.some(task => cycles(task.id)) ? "These dependencies form a loop. Remove a dependency so each task has a clear starting point before saving." : "";
}

export function PlanTaskGraph({ plan, dirty }: { plan: ImplementationPlan; dirty: boolean }) {
  const graph = plan.graph;
  if (!graph || !plan.tasks.length) return null;
  const names = new Map(plan.tasks.map(task => [task.id, task.title]));
  const name = (id: string) => names.get(id) || id;
  const noAcceptance = plan.tasks.filter(task => task.status !== "done" && !task.acceptance?.trim());
  const list = (ids: string[]) => ids.map(name).join("; ");
  return <section className="plan-task-graph" aria-label="Saved task coordination">
    <div className="journey-header"><h3>How the work fits together</h3><span className="journey-muted">Saved plan · revision {plan.revision}</span></div>
    <p>Dependencies show the order of work. Tasks in the same wave can be considered together after their prerequisites are integrated; shared files still need coordination.</p>
    {dirty && <p className="journey-warning" role="status">This is the saved plan. Save your edits to recalculate its task order and file overlaps.</p>}
    <ol className="plan-task-waves" aria-label="Dependency waves">{graph.waves.map((wave, index) => <li key={index}>
      <strong>Wave {index + 1}</strong><ul>{wave.map(id => <li key={id}>{name(id)}{graph.recordedDoneTaskIds.includes(id) && <span className="journey-muted"> · Done recorded by you</span>}</li>)}</ul>
    </li>)}</ol>
    <details className="plan-task-advice"><summary>Recorded progress · {graph.readyTaskIds.length} ready by dependencies, {graph.blocked.length} waiting</summary>
      <p className="journey-muted">This advice uses the Done status you record. It does not mean an agent ran, changes were integrated, or checks passed. Swarm execution checks its own integration records.</p>
      {!!graph.readyTaskIds.length && <p><strong>Ready by recorded dependencies:</strong> {list(graph.readyTaskIds)}</p>}
      {!!graph.blocked.length && <ul>{graph.blocked.map(item => <li key={item.taskId}><strong>{name(item.taskId)}</strong> waits for {list(item.dependsOn)}.</li>)}</ul>}
      {!!graph.recordedDoneTaskIds.length && <p><strong>Done recorded by you:</strong> {list(graph.recordedDoneTaskIds)}</p>}
    </details>
    {!!graph.conflicts.length && <details className="plan-task-advice plan-task-caution" open>
      <summary>{graph.conflicts.length + graph.conflictsOmitted} potential file {graph.conflicts.length + graph.conflictsOmitted === 1 ? "overlap" : "overlaps"}</summary>
      <p>These tasks can touch the same files. Refine their file scopes or add a dependency if one needs the other’s changes. Isolated branches still need integration review.</p>
      <ul>{graph.conflicts.map((conflict, index) => <li key={index}><strong>{list(conflict.taskIds)}</strong><span>Shared scope: {conflict.paths.map((path, pathIndex) => <span key={path}>{pathIndex > 0 ? " ↔ " : ""}<code>{path.split("/").map((part, partIndex) => <Fragment key={partIndex}>{partIndex > 0 && <>/<wbr /></>}{part}</Fragment>)}</code></span>)}</span></li>)}</ul>
      {graph.conflictsOmitted > 0 && <p>{graph.conflictsOmitted} additional overlaps are omitted. Narrow the file scopes and save to review the remaining pairs.</p>}
    </details>}
    {(graph.unscopedTaskIds.length > 0 || noAcceptance.length > 0) && <div className="plan-task-preparation">
      <strong>Before delegating</strong>
      {!!graph.unscopedTaskIds.length && <p>Add intended files and folders for these tasks: {list(graph.unscopedTaskIds)}. Without a scope, Luci cannot assess their file overlaps.</p>}
      {!!noAcceptance.length && <p>Add verification instructions for these tasks: {noAcceptance.map(task => task.title).join("; ")}. A concrete expected result helps the agent know when to stop.</p>}
      <p className="journey-muted">Open each task’s Scope and dependencies to amend it.</p>
    </div>}
  </section>;
}
