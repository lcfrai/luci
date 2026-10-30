import { useEffect, useLayoutEffect, useRef, useState } from 'react';
import type { OrchestratorTurn } from '../../api/orchestrator';
import { suggestedTaskSkillsSave } from '../../api/orchestratorSkills';
import { projectSkillsApi, type ProjectSkillsSave } from '../../api/projectSkills';
import { productApi, errorText } from '../../api/product';
import { notifyProjectContextChanged } from '../../api/projectContextEvents';
import { useCockpit } from '../../store/store';
import { Button } from '../fluid/components/button';

export function SuggestedTaskSkillList({ turn }: { turn: OrchestratorTurn }) {
  const list = turn.answer?.proposal?.suggestedSkillAssignments;
  if (!list?.length) return null;
  return <section aria-label="Suggested task skills"><h3>Suggested task skills</h3>
    <p>Plan Apply saves only the plan. These selections need a separate review and save. Skills do not grant tools or start work.</p>
    <ul>{list.map(s => <li key={`${s.role}/${s.taskId}/${s.skillId}`}><strong>{s.taskId} · {s.role}</strong>: {s.skillId} revision {s.skillRevision}
      <p>{s.reason}</p><p className="orchestrator-muted">Exact skill digest <code>{s.skillDigest}</code></p>
    </li>)}</ul>
    {!!turn.context.skills?.omittedVersions && <p>{turn.context.skills.omittedVersions} saved skill versions were omitted from this turn's bounded catalogue. They were not available for recommendation.</p>}
  </section>;
}

export function SuggestedTaskSkills({ turn, scope, canRead, canEdit, blocked }: {
  turn: OrchestratorTurn; scope: string | null; canRead: boolean; canEdit: boolean; blocked: boolean;
}) {
  const [prepared, setPrepared] = useState<ProjectSkillsSave | null>(null), [consent, setConsent] = useState(false);
  const [busy, setBusy] = useState(false), [error, setError] = useState(''), [notice, setNotice] = useState('');
  const mounted = useRef(true), generation = useRef(0), working = useRef(false);
  const latest = useRef({ canRead, canEdit, blocked, scope, turn }); latest.current = { canRead, canEdit, blocked, scope, turn };
  const identity = `${turn.id}/${turn.contextDigest}/${turn.appliedRevision ?? 0}`;
  useLayoutEffect(() => { generation.current++; setPrepared(null); setConsent(false); }, [canRead, canEdit, blocked, scope, identity]);
  useEffect(() => { mounted.current = true; return () => { mounted.current = false; generation.current++; }; }, []);
  const dirty = () => Object.values(useCockpit.getState().unsavedProjectDrafts).some(d => d.projectId === turn.projectId);
  const permitted = () => !!latest.current.scope && latest.current.canRead && latest.current.canEdit && !latest.current.blocked && !dirty();
  async function readCurrent() {
    const [plan, registry] = await Promise.all([productApi.plan(turn.projectId), projectSkillsApi.get(turn.projectId)]);
    return suggestedTaskSkillsSave(turn, plan, registry);
  }
  async function review() {
    if (!permitted() || working.current) return;
    const ticket = generation.current; working.current = true; setBusy(true); setPrepared(null); setConsent(false); setError(''); setNotice('');
    try {
      const value = await readCurrent();
      if (mounted.current && ticket === generation.current && permitted()) setPrepared(value);
    } catch (cause) { if (mounted.current && ticket === generation.current && latest.current.canRead) setError(errorText(cause)); }
    finally { working.current = false; if (mounted.current) setBusy(false); }
  }
  async function save() {
    if (!permitted() || working.current || !prepared || !consent) return;
    const ticket = generation.current; working.current = true; setBusy(true); setError(''); setNotice('');
    try {
      // Re-read immediately before POST. The server also checks both plan and
      // registry CAS under the existing journey write lock, closing this wait.
      const current = await readCurrent();
      if (!mounted.current || ticket !== generation.current || !permitted()) return;
      if (JSON.stringify(current) !== JSON.stringify(prepared)) throw new Error('The reviewed skill selection changed. Review current plan and skills again.');
      const saved = await projectSkillsApi.save(turn.projectId, current);
      if (!mounted.current || ticket !== generation.current || !permitted()) return;
      const key = (a: ProjectSkillsSave['assignments'][number]) => `${a.role}/${a.taskId ?? ''}/${a.skillId}/${a.skillRevision}`;
      if (saved.assignments.map(key).sort().join('\n') !== current.assignments.map(key).sort().join('\n')) throw new Error('The saved assignments were not confirmed. Read Project skills before another change.');
      if (turn.answer!.proposal!.suggestedSkillAssignments!.some(s => !saved.versions.some(v => v.id === s.skillId && v.revision === s.skillRevision && v.digest === s.skillDigest && v.roles.includes(s.role)))) throw new Error('The returned skill version did not match this recommendation. Read Project skills before another change.');
      setPrepared(null); setConsent(false); setNotice('Task skill assignments saved. No work was started.');
      if (scope) notifyProjectContextChanged(scope, turn.projectId);
    } catch (cause) {
      if (mounted.current && ticket === generation.current && latest.current.canRead) {
        setPrepared(null); setConsent(false);
        setError(`${errorText(cause)} Nothing will be resent automatically. Review the current saved plan and registry before another action.`);
      }
    } finally { working.current = false; if (mounted.current) setBusy(false); }
  }
  if (!canRead) return null;
  return <section aria-label="Review saved task skill suggestions" className="orchestrator-agent">
    <SuggestedTaskSkillList turn={turn}/>
    <p>Read back this exact saved plan before using its suggestions. Assignments for unrelated tasks and role-wide defaults stay intact.</p>
    {blocked && <p role="status">Save or discard current project edits and refresh context before selecting skills.</p>}
    {error && <p role="alert">{error}</p>}{notice && <p role="status">{notice}</p>}
    <Button variant="secondary" disabled={busy || blocked || !canEdit || !scope} onClick={() => void review()}>Review current plan and skills</Button>
    {prepared && <><p>Saved plan revision {prepared.expectedPlanRevision} · skill registry revision {prepared.expectedRevision}. {prepared.assignments.length} assignments in the complete saved selection.</p>
      <label className="orchestrator-ack"><input type="checkbox" checked={consent} disabled={busy || blocked || !canEdit} onChange={event => setConsent(event.target.checked)}/>Use these exact task skill versions, preserving unrelated selections.</label>
      <Button disabled={busy || blocked || !canEdit || !consent} onClick={() => void save()}>Use suggested task skills</Button>
    </>}
  </section>;
}
