import { useCallback, useEffect, useRef, useState } from 'react';
import { portfolioFocusApi, type FocusPlan, type FocusView } from '../../api/portfolioFocus';
import type { PortfolioGoal } from '../../api/portfolioGoals';
import { errorText } from '../../api/product';
import { teamScopeGeneration } from '../../api/teamScope';
import type { Project } from '../../store/types';
import { Button } from '../fluid/components/button';
import { projectRunsApi, type ProjectRun } from '../../api/projectRuns';
import { useProjectDraftScope } from '../../hooks/useProjectDraft';
import { useProjectAccess } from '../../hooks/useProjectAccess';
import { useTeamWorkspace } from '../teams/TeamWorkspaceProvider';

export function PortfolioFocus({ goal, focus, projects, disabled }: {
  goal: PortfolioGoal; focus?: FocusView; projects: Project[]; disabled: boolean;
}) {
  const scope = useProjectDraftScope('portfolio-goals'), team = useTeamWorkspace();
  const binding = JSON.stringify([scope, teamScopeGeneration(), team?.teamId, team?.revision, goal, projects.map(row => [row.id, row.path])]);
  return <FocusEditor key={binding} goal={goal} focus={focus} projects={projects} disabled={disabled || !scope || !!team?.accessBlocked} binding={binding} />;
}
function FocusEditor({ goal, focus, projects, disabled, binding }: {
  goal: PortfolioGoal; focus?: FocusView; projects: Project[]; disabled: boolean; binding: string;
}) {
  const [runs, setRuns] = useState<Record<string, string>>({});
  const [plan, setPlan] = useState<FocusPlan | null>(null);
  const [result, setResult] = useState<FocusView | undefined>(focus);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  const [uncertain, setUncertain] = useState(false);
  const epoch = useRef(0), generation = teamScopeGeneration();
  useEffect(() => { epoch.current++; setResult(focus); setPlan(null); setUncertain(false); setBusy(false); }, [focus]);
  useEffect(() => { return () => { epoch.current++; }; }, []);
  const current = (ticket: number) => ticket === epoch.current && generation === teamScopeGeneration();
  const upstream = [...new Set(goal.projects.flatMap(project => project.dependsOn))].sort();
  const selectRun = useCallback((projectId: string, runId: string) => {
    setRuns(previous => previous[projectId] === runId ? previous : { ...previous, [projectId]: runId }); setPlan(null);
  }, []);
  async function review() {
    if (disabled || busy || uncertain) return;
    const ticket = ++epoch.current; setBusy(true); setError(''); setPlan(null);
    try {
      const value = await portfolioFocusApi.review(goal, upstream.map(projectId => ({ projectId, runId: runs[projectId]?.trim() ?? '' })));
      if (current(ticket)) setPlan(value);
    } catch (cause) { if (current(ticket)) setError(errorText(cause)); }
    finally { if (current(ticket)) setBusy(false); }
  }
  async function change(mode: 'install' | 'remove') {
    if (disabled || busy || uncertain || mode === 'install' && !plan || mode === 'remove' && !result?.profile) return;
    const ticket = ++epoch.current, selected = plan; setBusy(true); setError(''); setPlan(null);
    try {
      const value = mode === 'install' ? await portfolioFocusApi.install(selected!, `focus-${crypto.randomUUID()}`)
        : await portfolioFocusApi.remove(result!, `focus-${crypto.randomUUID()}`);
      if (current(ticket)) setResult(value);
    } catch (cause) { if (current(ticket)) { setError(errorText(cause)); setUncertain(true); } }
    finally { if (current(ticket)) setBusy(false); }
  }
  const name = (id: string) => projects.find(project => project.id === id)?.name ?? id;
  return <section aria-label="Portfolio focus policy">
    <h3>Control focus for your queued work</h3>
    <p>Install the saved goal’s positive slot counts as ceilings for your own queued agents. Zero stays unspecified. Existing agents keep running; workspace, project, account and provider limits still apply. A ceiling does not reserve capacity or start work.</p>
    <p>For each dependency, choose the exact upstream run to wait until accepted with matching checks and release evidence. This does not mean a whole project is complete.</p>
    {focus === undefined && <p role="status">The current focus policy could not be read. Read latest saved goals before changing it.</p>}
    {error && <p role="alert">{error}</p>}
    {uncertain && <p role="status">The policy change is unconfirmed. Read latest saved goals; this request will not be repeated automatically.</p>}
    {result?.profile && <div>
      <p>Active focus revision {result.revision} · goal revision {result.profile.goalRevision}{result.profile.goalId === goal.id && result.profile.goalRevision !== goal.revision ? ' (earlier saved goal; review and reinstall to update)' : ''}.</p>
      <ul>{result.profile.allocations.map(row => <li key={row.projectId}>{name(row.projectId)}: {row.maxActive || 'Unspecified'} active-agent ceiling{row.waitUntilAccepted.map(dependency => `; wait until ${name(dependency.projectId)} run ${dependency.runId} is accepted`).join('')}</li>)}</ul>
      <Button variant="secondary" disabled={disabled || busy || uncertain} onClick={() => void change('remove')}>Remove active focus policy</Button>
    </div>}
    <fieldset disabled={disabled || busy || uncertain || focus === undefined}>
      <legend>Exact upstream acceptance dependencies</legend>
      {upstream.length === 0 && <p>This goal has no wait-until-accepted dependencies.</p>}
      {upstream.map(projectId => <UpstreamRunPicker key={projectId} projectId={projectId} name={name(projectId)} binding={binding} selected={runs[projectId] ?? ''} disabled={disabled || busy || uncertain || focus === undefined} onSelect={selectRun} />)}
      <Button disabled={upstream.some(projectId => !/^run-[A-Za-z0-9_-]{24}$/.test(runs[projectId]?.trim() ?? ''))} onClick={() => void review()}>Review focus policy</Button>
    </fieldset>
    {plan && <section aria-label="Reviewed focus policy">
      <p>Replace your active focus policy at revision {plan.expectedFocusRevision}. Only future queued starts owned by your account in this team are constrained.</p>
      <ul>{plan.profile.allocations.map(row => <li key={row.projectId}>{name(row.projectId)}: {row.maxActive || 'Unspecified'} ceiling{row.waitUntilAccepted.map(dependency => `; wait until run ${dependency.runId} is accepted (${dependency.inputDigest.slice(0, 12)})`).join('')}</li>)}</ul>
      <Button disabled={disabled || busy || uncertain} onClick={() => void change('install')}>Install this focus policy</Button>
    </section>}
  </section>;
}

type RunChoice = { id: string; label: string };
function runChoices(projectId: string, runs: ProjectRun[]): RunChoice[] {
  if (!Array.isArray(runs) || runs.length > 200) throw new Error('The run list exceeds its recorded history limit.');
  const seen = new Set<string>();
  return runs.map(run => {
    const titles = run.input?.implementation?.taskTitles;
    if (run.projectId !== projectId || run.input?.implementation?.projectId !== projectId || !/^run-[A-Za-z0-9_-]{24}$/.test(run.id) || seen.has(run.id) ||
        typeof run.createdAt !== 'string' || run.createdAt.length > 64 || !Number.isFinite(Date.parse(run.createdAt)) ||
        !['ready', 'coding', 'awaiting-checks', 'awaiting-release', 'blocked', 'cancelled', 'complete'].includes(run.state) ||
        !Array.isArray(titles) || titles.length > 200 || titles.some(title => typeof title !== 'string' || title.length > 2000)) {
      throw new Error('The recorded runs could not be confirmed for this project.');
    }
    seen.add(run.id);
    const title = titles[0]?.trim() || 'Recorded work', shortTitle = title.length > 100 ? `${title.slice(0, 100)}…` : title;
    return { id: run.id, label: `${shortTitle}${titles.length > 1 ? ` (+${titles.length - 1} tasks)` : ''} · ${new Date(run.createdAt).toLocaleString()} · recorded ${run.state.replace(/-/g, ' ')} · ${run.id.slice(0, 12)}…` };
  });
}
function UpstreamRunPicker({ projectId, name, binding, selected, disabled, onSelect }: {
  projectId: string; name: string; binding: string; selected: string; disabled: boolean; onSelect(projectId: string, runId: string): void;
}) {
  const access = useProjectAccess(projectId);
  const canRead = access.canRead && !access.loading && !access.error;
  const identity = JSON.stringify([binding, projectId, canRead]);
  const owner = useRef({ identity, epoch: 0 });
  if (owner.current.identity !== identity) owner.current = { identity, epoch: owner.current.epoch + 1 };
  const request = useRef<{ controller: AbortController; token: number } | null>(null), sequence = useRef(0);
  const [list, setList] = useState<{ identity: string; choices: RunChoice[]; loading: boolean; error: string } | null>(null);
  useEffect(() => {
    onSelect(projectId, ''); setList(null);
    return () => { sequence.current++; request.current?.controller.abort(); request.current = null; };
  }, [identity, projectId, onSelect]);
  async function load() {
    if (disabled || !canRead || request.current) return;
    onSelect(projectId, '');
    const controller = new AbortController(), token = ++sequence.current, epoch = owner.current.epoch;
    request.current = { controller, token }; setList({ identity, choices: [], loading: true, error: '' });
    const current = () => !controller.signal.aborted && sequence.current === token && owner.current.epoch === epoch;
    try {
      const choices = runChoices(projectId, await projectRunsApi.list(projectId, controller.signal));
      if (current()) setList({ identity, choices, loading: false, error: '' });
    } catch (cause) { if (current()) setList({ identity, choices: [], loading: false, error: errorText(cause) }); }
    finally { if (current()) request.current = null; }
  }
  const visible = list?.identity === identity ? list : null;
  return <div>
    <p>Wait for recorded work in <strong>{name}</strong></p>
    {!canRead ? <p role="status">{access.message || 'Current project reading access is required to choose a run.'}</p> : <>
      <Button type="button" variant="secondary" disabled={disabled || !!visible?.loading} onClick={() => void load()}>{visible?.error ? `Retry recorded runs for ${name}` : `Load recorded runs for ${name}`}</Button>
      {visible?.loading && <p role="status">Reading recorded runs for {name}…</p>}
      {visible?.error && <p role="alert">{visible.error}</p>}
      {visible && !visible.loading && !visible.error && !visible.choices.length && <p>No recorded runs are available for {name}.</p>}
      {visible && !visible.loading && !visible.error && visible.choices.length > 0 && <label>Run to wait for in {name}<select value={visible.choices.some(row => row.id === selected) ? selected : ''} disabled={disabled} onChange={event => {
        const id = event.target.value;
        if (id === '' || visible.choices.some(row => row.id === id)) onSelect(projectId, id);
      }}><option value="">Choose a recorded run</option>{visible.choices.map(row => <option key={row.id} value={row.id}>{row.label}</option>)}</select></label>}
    </>}
    <p className="journey-muted">Choose explicitly. Recorded state is not acceptance proof; the backend rechecks the exact run, checks and release evidence.</p>
  </div>;
}
