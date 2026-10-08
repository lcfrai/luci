import { useEffect, useRef, useState } from 'react';
import { projectOverviewApi, type ProjectOverview } from '../../api/projectOverview';
import { errorText } from '../../api/product';
import { ApiError } from '../../api/client';
import { swarmQueueApi, type ProjectSwarmQueue, type SwarmQueueItem, type QueuePriority } from '../../api/swarmQueue';
import { personalPermissions, teamsApi, type TeamPermissions } from '../../api/teams';
import { teamScopeGeneration } from '../../api/teamScope';
import { useProjectDraftScope } from '../../hooks/useProjectDraft';
import { useCockpit } from '../../store/store';
import type { Project, Lens } from '../../store/types';
import { useTeamProjectPermissions, useTeamWorkspace } from '../teams/TeamWorkspaceProvider';
import { Button } from '../fluid/components/button';
import { Badge } from '../ui/badge';
import { NavigationIcon } from '../common/NavigationIcon';
import { WorkflowDialog } from '../common/WorkflowDialog';
import { RecordedRunAttention } from '../common/RecordedRunAttention';
import { HostCapacity } from './HostCapacity';
import './project-overview.css';

type Summary = { value?: ProjectOverview; error?: string; sourceBusy?: boolean };
type Picked = { project: Project; item: SwarmQueueItem; canExecute: boolean };
type QueueAction = 'pause' | 'resume' | 'cancel' | 'priority';
type Outcome = { name: string; state: 'changed' | 'skipped' | 'failed' | 'unconfirmed'; message: string };
const itemKey = (item: SwarmQueueItem) => JSON.stringify([item.projectId, item.id]);
const eligible = (item: SwarmQueueItem, action: QueueAction) => action === 'resume' ? item.state === 'paused'
  : action === 'pause' ? ['queued', 'blocked'].includes(item.state) : ['queued', 'paused', 'blocked'].includes(item.state);

export function ProjectOverviewBoard({ projects }: { projects: Project[] }) {
  const scope = useProjectDraftScope('project-overview');
  const team = useTeamWorkspace();
  // A changed account, workspace or permission refresh removes every private
  // projection and pending selection before the new scope renders.
  const key = JSON.stringify([scope, team?.teamId ?? '', team?.selected?.permissions ?? null]);
  return scope ? <ScopedOverview key={key} projects={projects} /> : <p className="projects-setup-note">Connect your account and workspace to load project progress.</p>;
}

function ScopedOverview({ projects }: { projects: Project[] }) {
  const [limit, setLimit] = useState(20);
  const visible = projects.slice(0, limit);
  const ids = JSON.stringify(visible.map(project => project.id));
  const [summaries, setSummaries] = useState<Record<string, Summary>>({});
  const [refresh, setRefresh] = useState(0);
  const [picked, setPicked] = useState<Record<string, Picked>>({});
  const [pending, setPending] = useState(false);
  const [cancelReview, setCancelReview] = useState(false);
  const [outcomes, setOutcomes] = useState<Outcome[]>([]);
  const [priority, setPriority] = useState<QueuePriority>('normal');
  const alive = useRef(true);
  const team = useTeamWorkspace();
  const scope = teamScopeGeneration();
  useEffect(() => { alive.current = true; return () => { alive.current = false; }; }, []);
  useEffect(() => {
    const controller = new AbortController();
    const projectIds = JSON.parse(ids) as string[];
    let timer: ReturnType<typeof setTimeout> | undefined;
    let active = false;
    async function load() {
      if (active || controller.signal.aborted) return;
      active = true;
      let index = 0;
      // Bound overview reads; four visible projects never create terminal or
      // full-document readers. Refreshes do not overlap or run while hidden.
      await Promise.all(Array.from({ length: Math.min(4, projectIds.length) }, async () => {
        while (!controller.signal.aborted && index < projectIds.length) {
          const id = projectIds[index++];
          try {
            const value = await projectOverviewApi.get(id, controller.signal);
            if (!controller.signal.aborted) setSummaries(previous => ({ ...previous, [id]: { value } }));
          } catch (cause) {
            if (!controller.signal.aborted) setSummaries(previous => ({ ...previous, [id]: cause instanceof ApiError && cause.status === 409 && String(cause.code) === 'branch_busy' && previous[id]?.value
              ? { value: previous[id].value, sourceBusy: true } : { error: errorText(cause) } }));
          }
        }
      }));
      active = false;
      if (!controller.signal.aborted) timer = setTimeout(() => { if (document.visibilityState === 'visible') void load(); }, 15_000);
    }
    const focus = () => { if (document.visibilityState === 'visible') { clearTimeout(timer); void load(); } };
    void load(); document.addEventListener('visibilitychange', focus);
    return () => { controller.abort(); clearTimeout(timer); document.removeEventListener('visibilitychange', focus); };
  }, [ids, refresh, team?.revision]);
  useEffect(() => {
    const shown = new Set(JSON.parse(ids) as string[]);
    setSummaries(previous => Object.fromEntries(Object.entries(previous).filter(([id]) => shown.has(id))));
    setPicked(previous => Object.fromEntries(Object.entries(previous).filter(([, row]) => shown.has(row.project.id))));
  }, [ids]);

  function select(project: Project, item: SwarmQueueItem, selected: boolean, canExecute: boolean) {
    if (pending) return;
    setPicked(previous => { const next = { ...previous }; if (selected) next[itemKey(item)] = { project, item, canExecute }; else delete next[itemKey(item)]; return next; });
  }
  function forgetProject(projectId: string) {
    setPicked(previous => Object.fromEntries(Object.entries(previous).filter(([, value]) => value.project.id !== projectId)));
  }
  async function apply(action: QueueAction) {
    if (pending || !Object.keys(picked).length) return;
    setPending(true); setCancelReview(false); setOutcomes([]);
    const selected = Object.values(picked);
    const results: Outcome[] = [];
    const access = new Map<string, TeamPermissions>();
    for (const { project, item } of selected) {
      if (!alive.current || teamScopeGeneration() !== scope) break;
      const name = `${project.name} · ${item.taskTitle}`;
      if (!eligible(item, action)) { results.push({ name, state: 'skipped', message: `Recorded ${item.state}; this action does not apply.` }); continue; }
      let submitted = false;
      try {
        if (!access.has(project.id)) {
          const allowed = !team?.teamId ? personalPermissions : (await teamsApi.projectAccess(team.teamId, project.id)).permissions;
          access.set(project.id, allowed);
        }
        if (!alive.current || teamScopeGeneration() !== scope) break;
        const permission = access.get(project.id);
        if (!permission?.read || !permission.edit) throw new Error('Editing access is unavailable for this project.');
        if (action === 'resume' && !permission.execute) throw new Error('Execution access is required to resume this project’s queued work.');
        submitted = true;
        await swarmQueueApi.control(project.id, item, action, priority);
        results.push({ name, state: 'changed', message: action === 'priority' ? `Priority set to ${priority}.` : action === 'pause' ? 'Paused before dispatch.' : action === 'resume' ? 'Returned to the queue.' : 'Waiting task cancelled.' });
      } catch (cause) {
        const rejected = !submitted || cause instanceof ApiError && cause.status >= 400 && cause.status < 500 && cause.status !== 408;
        results.push({ name, state: rejected ? 'failed' : 'unconfirmed', message: rejected ? errorText(cause) : `${errorText(cause)} The change may already have applied. Refresh this queue and inspect its state before trying again.` });
      }
      if (alive.current && teamScopeGeneration() === scope) setOutcomes([...results]);
    }
    if (alive.current && teamScopeGeneration() === scope) {
      setOutcomes(results); setPicked({}); setPending(false); setRefresh(value => value + 1);
    }
  }
  const selection = Object.values(picked);
  return <div className="project-overview-board">
    <HostCapacity />
    <div className="overview-heading"><div><h2>Across your projects</h2><p>Goals, activity and the next decision.</p></div><Button variant="secondary" disabled={pending} onClick={() => setRefresh(value => value + 1)}>Refresh overview</Button></div>
    {selection.length > 0 && <section className="overview-bulk" aria-label="Selected queued tasks">
      <strong>{selection.length} waiting {selection.length === 1 ? 'task' : 'tasks'} selected</strong>
      <div className="overview-actions"><Button variant="secondary" disabled={pending || !selection.some(row => eligible(row.item, 'pause'))} onClick={() => void apply('pause')}>Pause waiting</Button><Button variant="secondary" disabled={pending || !selection.some(row => row.canExecute && eligible(row.item, 'resume'))} onClick={() => void apply('resume')}>Resume paused</Button><Button variant="tertiary" disabled={pending} onClick={() => setCancelReview(true)}>Cancel waiting…</Button>
        <label>Priority<select aria-label="Selected task priority" value={priority} disabled={pending} onChange={event => setPriority(event.target.value as QueuePriority)}><option value="high">High</option><option value="normal">Normal</option><option value="low">Low</option></select></label><Button variant="secondary" disabled={pending} onClick={() => void apply('priority')}>Set priority</Button><Button variant="ghost" disabled={pending} onClick={() => setPicked({})}>Clear selection</Button></div>
      <p>Applies only to the selected waiting tasks. Running agents need their project’s stop controls.</p>
      {selection.some(row => !row.canExecute) && <p>Editing access allows pause, cancellation and priority changes. Resuming work also requires execution access in each selected project.</p>}
    </section>}
    {pending && <p role="status">Applying the selected queue changes…</p>}
    {outcomes.length > 0 && <section className="overview-outcomes" aria-label="Queue change results" aria-live="polite"><h3>Queue changes</h3><p>{outcomes.filter(row => row.state === 'changed').length} changed · {outcomes.filter(row => row.state === 'failed').length} failed · {outcomes.filter(row => row.state === 'unconfirmed').length} unconfirmed · {outcomes.filter(row => row.state === 'skipped').length} skipped</p><ul>{outcomes.map((row, index) => <li key={index}><strong>{row.name}</strong><span data-outcome={row.state}>{row.state === 'failed' ? 'Failed: ' : row.state === 'unconfirmed' ? 'Unconfirmed: ' : row.state === 'skipped' ? 'Skipped: ' : ''}{row.message}</span></li>)}</ul></section>}
    <ul className="overview-grid">{visible.map(project => <ProjectCard key={project.id} project={project} summary={summaries[project.id]} refresh={refresh} picked={picked} pending={pending} onSelect={select} onForget={forgetProject} />)}</ul>
    {projects.length > visible.length && <Button variant="secondary" onClick={() => setLimit(value => value + 20)}>Show more projects ({projects.length - visible.length} remaining)</Button>}
    {cancelReview && <WorkflowDialog title="Cancel selected waiting tasks?" description="This removes the selected waiting work from dispatch. Saved context and any existing worker output remain available in each project." onClose={() => setCancelReview(false)}><ul className="overview-cancel-list">{selection.map(row => <li key={itemKey(row.item)}>{row.project.name} · {row.item.taskTitle}</li>)}</ul><div className="workflow-actions"><Button variant="secondary" onClick={() => setCancelReview(false)}>Keep queued</Button><Button onClick={() => void apply('cancel')}>Cancel {selection.length} waiting {selection.length === 1 ? 'task' : 'tasks'}</Button></div></WorkflowDialog>}
  </div>;
}

function ProjectCard({ project, summary, refresh, picked, pending, onSelect, onForget }: {
  project: Project; summary?: Summary; refresh: number; picked: Record<string, Picked>; pending: boolean;
  onSelect(project: Project, item: SwarmQueueItem, selected: boolean, canExecute: boolean): void; onForget(projectId: string): void;
}) {
  const [showQueue, setShowQueue] = useState(false);
  const scope = useProjectDraftScope(project.id);
  const value = summary?.value;
  const counts = value?.execution;
  function openDestination(lens?: Lens, runId?: string) {
    if (!value || summary?.sourceBusy) return;
    useCockpit.getState().selectProject(project.id, lens);
    const state = useCockpit.getState(), destination = state.pendingNavigation;
    const accepted = destination ? destination.kind === 'project' && destination.projectId === project.id && destination.lens === lens
      : state.selection.projectId === project.id && (!lens || state.lens === lens);
    // Retain an exact run request through the normal unsaved-draft guard. Its
    // cancel action clears this transient intent; it never authorizes execution.
    if (!accepted || !state.projects.some(item => item.id === project.id && item.path === project.path)) return;
    useCockpit.setState({ projectRunInspection: scope && lens === 'develop' && typeof runId === 'string' && runId
      ? { scope, projectId: project.id, runId, refresh: true } : null });
  }
  function openNext() {
    if (!value) return;
    const lens = (['discover', 'design', 'develop', 'files', 'merge', 'deploy'].includes(value.nextAction.lens) ? value.nextAction.lens : undefined) as Lens | undefined;
    openDestination(lens, value.nextAction.runId);
  }
  return <li className="overview-card"><header><h3><button className="overview-project-open" onClick={() => useCockpit.getState().selectProject(project.id)} aria-label={`Open project ${project.name}`}>{project.name}<NavigationIcon name="chevron-right" /></button></h3>{value && <Badge variant="outline">{summary?.sourceBusy && 'Last recorded: '}{value.phase.label}</Badge>}</header>
    {!summary ? <p role="status">Loading project context…</p> : summary.error ? <p className="overview-warning" role="status">Summary unavailable. {summary.error}</p> : value && <>
      {summary.sourceBusy && <p className="overview-warning" role="status">Source is being prepared; showing last recorded context. Queue controls remain available. Activity counts are recorded, not live.</p>}
      <p className="overview-goal">{value.goal?.text || (value.goal ? 'Add the outcome you want to build.' : 'Saved context could not be read.')}{value.goal?.truncated && '…'}</p>
      <dl className="overview-counts" aria-label="Recorded swarm queue activity">{(['running', 'queued', 'blocked'] as const).map(state => <div key={state}><dt>{state === 'running' ? 'Running' : state === 'queued' ? 'Queued' : 'Blocked'}</dt><dd>{counts?.[state] ?? '—'}</dd></div>)}</dl>
      {counts?.status === 'unavailable' && <p className="overview-warning">Queue activity unavailable; no zero counts inferred.</p>}
      {!!counts?.paused && <p className="overview-meta">{counts.paused} paused</p>}{!!counts?.reserved && <p className="overview-meta">{counts.reserved} reserved for dispatch</p>}{!!counts?.unknown && <p className="overview-warning">{counts.unknown} {counts.unknown === 1 ? 'outcome needs' : 'outcomes need'} reconciliation in the project.</p>}
      {counts?.dispatchPaused && <p className="overview-warning">Workspace dispatch is paused.</p>}
      {value.attention?.status === 'recorded'
        ? <RecordedRunAttention {...value.attention} compact initialLimit={1} count={value.attention.count ?? 0} omitted={value.attention.omitted ?? 0} stale={summary.sourceBusy} onInspect={runId => openDestination('develop', runId)} onOpenHistory={() => openDestination('develop')} />
        : <p className="overview-meta">Review history is unavailable. Open the project to inspect its saved builds.</p>}
    </>}
    <footer>{value && !summary?.sourceBusy && !(value.nextAction.id === 'inspect-run' && value.nextAction.lens === 'develop' && value.attention?.status === 'recorded' && value.attention.items[0]?.id === value.nextAction.runId) && <Button variant="secondary" onClick={openNext}>{value.nextAction.label}</Button>}<Button variant="ghost" onClick={() => useCockpit.getState().selectProject(project.id)}>Open project</Button><Button variant="ghost" disabled={pending} aria-expanded={showQueue} onClick={() => { if (showQueue) onForget(project.id); setShowQueue(value => !value); }}>{showQueue ? 'Hide queue' : 'Manage queue'}</Button></footer>
    {value && <>

      {value.questions.items.length > 0 && <details className="overview-questions"><summary>{value.questions.count === null ? 'Known questions to resolve' : `${value.questions.count} ${value.questions.count === 1 ? 'question' : 'questions'} to resolve`}</summary><ul>{value.questions.items.map(question => <li key={`${question.source}:${question.id}`}>{question.text}{question.truncated && '…'}</li>)}</ul>{!!value.questions.omitted && <p>{value.questions.omitted} more in the project.</p>}</details>}
      {value.questions.status !== 'recorded' && <p className="overview-meta">Question history is incomplete. Open the project for its available context.</p>}
      {value.latestRun?.status === 'recorded' && <p className="overview-meta">{summary?.sourceBusy ? 'Last recorded run' : 'Latest run'}: {value.latestRun.state?.replace(/_/g, ' ') || 'recorded'}. {value.latestRun.contextStatus === 'changed' ? 'Project context changed; review the run against the current brief and plan.' : value.latestRun.contextStatus === 'unavailable' ? 'Its context comparison is unavailable.' : 'Open the run for its checks and review evidence.'}</p>}
      {value.latestRun?.status === 'unavailable' && <p className="overview-meta">Latest run history unavailable. Open the project to inspect its saved runs.</p>}
      <details className="overview-details"><summary>Context and observation</summary>{value.context && <p>Brief v{value.context.briefRevision} · Definition {value.context.definitionStatus || 'draft'} · Design {value.context.designStatus || 'draft'} · Plan v{value.context.planRevision}. {value.context.tasks.recordedDone} of {value.context.tasks.total} tasks marked done by a person; this is not verification evidence.</p>}<p>Observed {new Date(value.observedAt).toLocaleTimeString()}. Running is a recorded queue state, not a live process check. Direct sessions are shown inside the project.</p>{value.limitations.map((text, index) => <p key={index}>{text}</p>)}</details>
    </>}


    {showQueue && <ProjectQueue project={project} refresh={refresh} picked={picked} pending={pending} onSelect={onSelect} />}
  </li>;
}

function ProjectQueue({ project, refresh, picked, pending, onSelect }: { project: Project; refresh: number; picked: Record<string, Picked>; pending: boolean; onSelect(project: Project, item: SwarmQueueItem, selected: boolean, canExecute: boolean): void }) {
  const access = useTeamProjectPermissions(project.id);
  const [queue, setQueue] = useState<ProjectSwarmQueue | null>(null);
  const [error, setError] = useState('');
  const [retry, setRetry] = useState(0);
  const [limit, setLimit] = useState(30);
  useEffect(() => {
    setQueue(null); setError('');
    if (!access.permissions.read) return;
    const controller = new AbortController();
    void swarmQueueApi.get(project.id, controller.signal).then(value => { if (!controller.signal.aborted) setQueue(value); }).catch(cause => { if (!controller.signal.aborted) setError(errorText(cause)); });
    return () => controller.abort();
  }, [project.id, access.permissions.read, refresh, retry]);
  if (!access.permissions.read) return <p className="overview-warning">{access.loading ? 'Checking project access…' : 'Project access is unavailable.'}</p>;
  return <section className="overview-queue" aria-label={`${project.name} queue`}><h4>Waiting and active work</h4>
    {error ? <p role="alert">{error} <Button variant="secondary" onClick={() => setRetry(value => value + 1)}>Retry queue</Button></p> : !queue ? <p role="status">Loading queue…</p> : <>
      {queue.items.length === 0 ? <p>No queued swarm work. Prepare and review tasks inside the project before adding them.</p> : <ul>{queue.items.slice(0, limit).map(item => <li key={item.id}><label className="overview-queue-title"><input type="checkbox" aria-label={`Select ${item.taskTitle} in ${project.name}`} checked={!!picked[itemKey(item)]} disabled={pending || !access.permissions.edit || !eligible(item, 'priority')} onChange={event => onSelect(project, item, event.target.checked, access.permissions.execute)} /><strong>{item.taskTitle}</strong></label><div className="overview-queue-state"><Badge variant={item.state === 'blocked' || item.state === 'unknown' ? 'destructive' : 'secondary'}>{item.state}</Badge><span>{item.priority} priority</span></div>{item.reason && <p>{item.reason}</p>}{['blocked', 'unknown'].includes(item.state) && <p>Open the project to inspect the run and retained output before retrying. Retrying requires a new reviewed task entry.</p>}</li>)}</ul>}
      {queue.items.length > limit && <Button variant="ghost" onClick={() => setLimit(value => value + 30)}>Show more queued tasks</Button>}{queue.hasMore && <p>Only part of this queue is listed. Open the project for remaining work.</p>}
      {!access.permissions.edit ? <p>Editing permission is required to change queued work.</p> : !access.permissions.execute && <p>You can pause, cancel or reprioritize waiting work. Execution permission is required to resume it.</p>}
    </>}
  </section>;
}
