import { useCallback, useEffect, useRef, useState } from 'react';
import { portfolioGoalsApi, type GoalApplication, type GoalPlan, type GoalProject, type GoalsView, type PortfolioGoal } from '../../api/portfolioGoals';
import { errorText } from '../../api/product';
import { teamScopeGeneration } from '../../api/teamScope';
import { useProjectDraftScope } from '../../hooks/useProjectDraft';
import { useProjectAccess } from '../../hooks/useProjectAccess';
import { useUnsavedProjectDraft } from '../../hooks/useUnsavedProjectDraft';
import type { Project } from '../../store/types';
import { useTeamWorkspace } from '../teams/TeamWorkspaceProvider';
import { Button } from '../fluid/components/button';

const freshGoal = (): PortfolioGoal => ({ id: '', revision: 0, title: '', outcome: '', projects: [], updatedAt: '' });
const cloneGoal = (goal: PortfolioGoal) => ({ ...goal, projects: goal.projects.map(row => ({ ...row, dependsOn: [...row.dependsOn] })) });
const goalContent = (goal: PortfolioGoal) => JSON.stringify([goal.title, goal.outcome, goal.deadline ?? '', goal.projects.map(row => ({ ...row, dependsOn: [...row.dependsOn].sort() })).sort((a, b) => a.projectId.localeCompare(b.projectId))]);

export function PortfolioGoals({ projects }: { projects: Project[] }) {
  const scope = useProjectDraftScope('portfolio-goals');
  const team = useTeamWorkspace();
  const [opened, setOpened] = useState(false);
  const key = JSON.stringify([scope, teamScopeGeneration(), team?.teamId, team?.revision, team?.selected?.permissions, projects.map(p => [p.id, p.path])]);
  if (!scope || team?.accessBlocked || projects.length === 0) return null;
  return <details className="project-overview-board" onToggle={event => { if (event.currentTarget.open) setOpened(true); }}>
    <summary>Goals across projects</summary>
    {opened && <GoalEditor key={key} projects={projects} />}
  </details>;
}

function GoalEditor({ projects }: { projects: Project[] }) {
  const [view, setView] = useState<GoalsView | null>(null);
  const [draft, setDraft] = useState<PortfolioGoal>(freshGoal);
  const [original, setOriginal] = useState<PortfolioGoal>(freshGoal);
  const [plan, setPlan] = useState<GoalPlan | null>(null);
  const [application, setApplication] = useState<GoalApplication | null>(null);
  const [permissions, setPermissions] = useState<Record<string, boolean>>({});
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  const [mustReload, setMustReload] = useState(false);
  const [retry, setRetry] = useState(0);
  const epoch = useRef(0);
  const generation = teamScopeGeneration();
  const dirty = JSON.stringify(draft) !== JSON.stringify(original);
  const editor = useRef({ draft, dirty }); editor.current = { draft, dirty };
  useUnsavedProjectDraft(draft.projects[0]?.projectId ?? projects[0].id, dirty, 'Portfolio goal', 'discover');
  const projectPermission = useCallback((id: string, allowed: boolean) => setPermissions(current => current[id] === allowed ? current : { ...current, [id]: allowed }), []);
  const current = (ticket: number) => ticket === epoch.current && generation === teamScopeGeneration();
  useEffect(() => {
    const ticket = ++epoch.current, controller = new AbortController();
    setBusy(true); setError('');
    void portfolioGoalsApi.get(controller.signal).then(value => { if (current(ticket)) {
      setView(value); setMustReload(false);
      if (!editor.current.dirty) {
        const latest = value.goals.find(goal => goal.id === editor.current.draft.id) ?? freshGoal();
        setDraft(cloneGoal(latest)); setOriginal(cloneGoal(latest));
      }
    } })
      .catch(cause => { if (current(ticket)) setError(errorText(cause)); })
      .finally(() => { if (current(ticket)) setBusy(false); });
    return () => { epoch.current++; controller.abort(); };
  }, [retry, generation]);
  function edit(value: PortfolioGoal) { setDraft(value); setPlan(null); setApplication(null); }
  function choose(goal: PortfolioGoal) {
    if (busy || dirty) return;
    const copy = cloneGoal(goal); setDraft(copy); setOriginal(cloneGoal(copy)); setPlan(null); setApplication(null); setError('');
  }
  async function save() {
    if (!view || busy || !dirty || mustReload) return;
    const ticket = epoch.current, requested = cloneGoal(draft);
    setBusy(true); setError(''); setPlan(null);
    try {
      const saved = await portfolioGoalsApi.save(view.revision, requested);
      if (!current(ticket)) return;
      // Never overwrite an unconfirmed identity/revision with an unrelated row.
      if (requested.id && saved.id !== requested.id || saved.revision !== requested.revision + 1 || goalContent(requested) !== goalContent(saved)) throw new Error('The saved goal revision did not match your edit. Read current goals before continuing.');
      const latest = await portfolioGoalsApi.get();
      if (!current(ticket)) return;
      setView(latest); setDraft(cloneGoal(saved)); setOriginal(cloneGoal(saved));
    } catch (cause) { if (current(ticket)) { setError(errorText(cause)); setMustReload(true); } }
    finally { if (current(ticket)) setBusy(false); }
  }
  async function review() {
    if (!draft.id || dirty || busy || mustReload) return;
    const ticket = epoch.current; setBusy(true); setError(''); setPlan(null); setApplication(null);
    try { const result = await portfolioGoalsApi.review(draft); if (current(ticket)) setPlan(result); }
    catch (cause) { if (current(ticket)) setError(errorText(cause)); }
    finally { if (current(ticket)) setBusy(false); }
  }
  async function apply() {
    if (!plan || busy || dirty || application || plan.decisions.length || !draft.projects.every(p => permissions[p.projectId])) return;
    const ticket = epoch.current; setBusy(true); setError('');
    // One explicit click issues one application. A failed response destroys
    // this review: refresh retained results, never automatically replay it.
    const selected = plan; setPlan(null);
    try { const result = await portfolioGoalsApi.apply(selected, `portfolio-${crypto.randomUUID()}`); if (current(ticket)) setApplication(result); }
    catch (cause) { if (current(ticket)) { setError(errorText(cause)); setMustReload(true); } }
    finally { if (current(ticket)) setBusy(false); }
  }
  async function refreshApplication(id: string) {
    if (busy) return;
    const ticket = epoch.current; setBusy(true); setError('');
    try { const result = await portfolioGoalsApi.application(id); if (current(ticket)) setApplication(result); }
    catch (cause) { if (current(ticket)) setError(errorText(cause)); }
    finally { if (current(ticket)) setBusy(false); }
  }
  function updateProject(row: GoalProject) { edit({ ...draft, projects: draft.projects.map(old => old.projectId === row.projectId ? row : old) }); }
  const selectedProjects = new Set(draft.projects.map(row => row.projectId));
  return <div className="project-work-body">
    <p>Tell Luci what matters across projects. Review exactly how waiting work would change before applying priorities.</p>
    <p className="journey-muted">Your goals are private to your account. Focus slots and cross-project dependencies express intentions; they do not change machine limits or task dependencies.</p>
    {error && <p role="alert">{error}</p>}
    {view?.partial && <p role="status">Some saved goals reference projects you cannot currently read. This is a partial view.</p>}
    {!view && <p role="status">{busy ? 'Reading saved portfolio goals…' : 'Saved goals are unavailable.'}</p>}
    <Button variant="secondary" disabled={busy} onClick={() => { setPlan(null); setApplication(null); setRetry(value => value + 1); }}>Read latest saved goals</Button>
    {view && <>
      <label className="luci-field">Saved goal<select value={draft.id} disabled={busy || dirty} onChange={event => choose(view.goals.find(goal => goal.id === event.target.value) ?? freshGoal())}>
        <option value="">New goal</option>{view.goals.map(goal => <option key={goal.id} value={goal.id}>{goal.title} · revision {goal.revision}</option>)}
      </select></label>
      <fieldset disabled={busy} className="workflow-form"><legend>What should move forward?</legend>
        <label>Goal title<input value={draft.title} maxLength={200} onChange={event => edit({ ...draft, title: event.target.value })} /></label>
        <label>Outcome<textarea rows={3} value={draft.outcome} maxLength={4000} onChange={event => edit({ ...draft, outcome: event.target.value })} /></label>
        <label>Target date (optional)<input type="date" value={draft.deadline ?? ''} onChange={event => edit({ ...draft, deadline: event.target.value || undefined })} /></label>
        <fieldset><legend>Selected projects (up to 16)</legend>{projects.map(project => <label key={project.id}>
          <input type="checkbox" checked={selectedProjects.has(project.id)} disabled={!selectedProjects.has(project.id) && draft.projects.length >= 16} onChange={event => edit({ ...draft, projects: event.target.checked
            ? [...draft.projects, { projectId: project.id, priority: 'normal', requestedSlots: 0, dependsOn: [] }]
            : draft.projects.filter(row => row.projectId !== project.id).map(row => ({ ...row, dependsOn: row.dependsOn.filter(id => id !== project.id) })) })} />{project.name}
        </label>)}</fieldset>
        {draft.projects.map(row => <ProjectIntention key={row.projectId} row={row} projects={projects} selected={draft.projects} update={updateProject} permission={projectPermission} />)}
        <Button disabled={!dirty || mustReload || !draft.title.trim() || !draft.outcome.trim() || !draft.projects.length} onClick={() => void save()}>Save private goal</Button>
      </fieldset>
      {dirty && <p role="status">Unsaved portfolio goal. Keep this editor open until you save or discard it.</p>}
      {dirty && <Button variant="secondary" disabled={busy} onClick={() => { const saved = view.goals.find(goal => goal.id === draft.id) ?? freshGoal(); setDraft(cloneGoal(saved)); setOriginal(cloneGoal(saved)); setPlan(null); setApplication(null); }}>Discard edits and load saved goal</Button>}
      <Button disabled={busy || dirty || mustReload || !draft.id} onClick={() => void review()}>Review waiting-work priorities</Button>
      {plan && <section aria-label="Reviewed priority changes"><h3>Review before applying</h3>
        {plan.allocations.map(row => <p key={row.projectId}><strong>{projects.find(project => project.id === row.projectId)?.name ?? row.projectId}</strong>: {row.priority} priority · {row.requestedSlots || 'No specified'} focus slots · {row.recordedActive} recorded active · {row.waiting} waiting. {row.dependsOn.length > 0 && `Planning dependencies: ${row.dependsOn.map(id => projects.find(p => p.id === id)?.name ?? id).join(', ')}.`}</p>)}
        <p>These priorities are scheduling preferences for the exact reviewed queue items. Apply does not enqueue, resume, start or stop work. Future task admission separately rechecks source, approvals and capacity; existing project fairness and resource policies remain authoritative.</p>
        <ul>{plan.actions.map(action => <li key={`${action.projectId}:${action.queueId}`}>{projects.find(p => p.id === action.projectId)?.name} · {action.taskTitle}: {action.fromPriority} → {action.toPriority} (queue revision {action.expectedRevision})</li>)}</ul>
        {plan.actions.length === 0 && <p>No waiting queue priorities need changing.</p>}
        {plan.decisions.map(message => <p key={message} role="status">{message}</p>)}
        <Button disabled={busy || !plan.actions.length || !!plan.decisions.length || !draft.projects.every(p => permissions[p.projectId])} onClick={() => void apply()}>Apply these priority changes</Button>
      </section>}
      {application && <section aria-label="Priority application result"><h3>Priority result: {application.status.replace(/_/g, ' ')}</h3>
        <p>This records queue changes, not execution or completion. Partial and unconfirmed outcomes must be inspected before a new review.</p>
        <ul>{application.results.map(result => <li key={`${result.action.projectId}:${result.action.queueId}`}>{result.action.taskTitle} · {result.status}: {result.message}</li>)}</ul>
        <Button variant="secondary" disabled={busy} onClick={() => void refreshApplication(application.id)}>Refresh this retained application</Button>
      </section>}
      <details><summary>Retained priority applications</summary>{view.applications.map(row => <p key={row.id}>{view.goals.find(goal => goal.id === row.goalId)?.title} · goal revision {row.goalRevision} · {row.status.replace(/_/g, ' ')} <Button variant="secondary" disabled={busy} onClick={() => void refreshApplication(row.id)}>Read application {row.id}</Button></p>)}</details>
    </>}
  </div>;
}

function ProjectIntention({ row, projects, selected, update, permission }: {
  row: GoalProject; projects: Project[]; selected: GoalProject[]; update(value: GoalProject): void; permission(id: string, allowed: boolean): void;
}) {
  const access = useProjectAccess(row.projectId);
  useEffect(() => { permission(row.projectId, access.canEdit && access.canRead); return () => permission(row.projectId, false); }, [row.projectId, access.canEdit, access.canRead, permission]);
  return <fieldset><legend>{projects.find(project => project.id === row.projectId)?.name ?? row.projectId}</legend>
    {!access.canEdit && <p className="journey-muted">You can save this intention. Applying queue priorities requires current project edit/control permission.</p>}
    <label>Priority<select value={row.priority} onChange={event => update({ ...row, priority: event.target.value as GoalProject['priority'] })}><option value="low">Low</option><option value="normal">Normal</option><option value="high">High</option></select></label>
    <label>Desired focus slots (0 means unspecified)<input type="number" min={0} max={200} step={1} value={row.requestedSlots} onChange={event => update({ ...row, requestedSlots: Number(event.target.value) })} /></label>
    <fieldset><legend>Planning dependencies</legend>{selected.filter(other => other.projectId !== row.projectId).map(other => <label key={other.projectId}><input type="checkbox" checked={row.dependsOn.includes(other.projectId)} onChange={event => update({ ...row, dependsOn: event.target.checked ? [...row.dependsOn, other.projectId] : row.dependsOn.filter(id => id !== other.projectId) })} />{projects.find(project => project.id === other.projectId)?.name ?? other.projectId}</label>)}</fieldset>
  </fieldset>;
}
