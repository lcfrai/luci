import { useEffect, useLayoutEffect, useRef, useState } from 'react';
import { projectSkillsApi, type ProjectSkillAssignment, type ProjectSkillRegistry, type ProjectSkillsSave, type SkillRole } from '../../api/projectSkills';
import { productApi, errorText, type ImplementationTask } from '../../api/product';
import { notifyProjectContextChanged } from '../../api/projectContextEvents';
import { useProjectDraftScope } from '../../hooks/useProjectDraft';
import { useProjectAccess } from '../../hooks/useProjectAccess';
import { useUnsavedProjectDraft } from '../../hooks/useUnsavedProjectDraft';
import { Button } from '../fluid/components/button';

export function ProjectSkills({ projectId }: { projectId: string }) {
  const scope = useProjectDraftScope(projectId);
  const [open, setOpen] = useState(false);
  return <details className="project-work" onToggle={event => { if (event.currentTarget.open) setOpen(true); }}>
    <summary>Project skills</summary>
    {open && <SkillEditor key={JSON.stringify([scope, projectId])} projectId={projectId} scope={scope} />}
  </details>;
}

function SkillEditor({ projectId, scope }: { projectId: string; scope: string | null }) {
  const access = useProjectAccess(projectId);
  const [saved, setSaved] = useState<ProjectSkillRegistry | null>(null);
  const [tasks, setTasks] = useState<ImplementationTask[]>([]);
  const [assignments, setAssignments] = useState<ProjectSkillAssignment[]>([]);
  const [skill, setSkill] = useState<NonNullable<ProjectSkillsSave['skill']>>({ id: '', title: '', provenance: '', roles: ['coding'], instructions: '' });
  const [editing, setEditing] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  const [retry, setRetry] = useState(0);
  const latest = useRef({ canRead: access.canRead, canEdit: access.canEdit }); latest.current = access;
  const epoch = useRef(0);
  const permissionEpoch = useRef(0);
  useLayoutEffect(() => { permissionEpoch.current++; }, [access.canRead, access.canEdit]);
  const loaded = useRef(-1);
  useEffect(() => {
    const ticket = ++epoch.current;
    if (!access.canRead || access.loading || access.error) return;
    if (loaded.current === retry) { setBusy(false); return; }
    const controller = new AbortController();
    setBusy(true); setError('');
    void Promise.all([projectSkillsApi.get(projectId, controller.signal), productApi.plan(projectId)]).then(([registry, plan]) => {
      if (ticket !== epoch.current || !latest.current.canRead) return;
      if (plan.projectId !== projectId) throw new Error('The implementation plan belongs to another project.');
      loaded.current = retry;
      setSaved(registry); setAssignments(registry.assignments); setTasks(plan.tasks);
    }).catch(cause => { if (ticket === epoch.current && latest.current.canRead) setError(errorText(cause)); })
      .finally(() => { if (ticket === epoch.current) setBusy(false); });
    return () => { epoch.current++; controller.abort(); };
  }, [projectId, access.canRead, access.loading, access.error, retry]);
  const dirty = editing || !!saved && JSON.stringify(assignments) !== JSON.stringify(saved.assignments);
  useUnsavedProjectDraft(projectId, dirty, 'Project skills', 'develop');
  async function save(includeSkill: boolean) {
    if (!saved || !latest.current.canEdit || busy) return;
    const ticket = epoch.current;
    const permissionTicket = permissionEpoch.current;
    setBusy(true); setError('');
    try {
      const value = await projectSkillsApi.save(projectId, { expectedRevision: saved.revision, assignments, ...(includeSkill ? { skill } : {}) });
      if (ticket !== epoch.current || permissionTicket !== permissionEpoch.current || !latest.current.canEdit) return;
      setSaved(value); setAssignments(value.assignments); setEditing(false);
      if (includeSkill) setSkill({ id: '', title: '', provenance: '', roles: ['coding'], instructions: '' });
      if (scope) notifyProjectContextChanged(scope, projectId);
    } catch (cause) { if (ticket === epoch.current && latest.current.canRead) setError(errorText(cause)); }
    finally { if (ticket === epoch.current) setBusy(false); }
  }
  if (!access.canRead) return <p role="status">{access.message}</p>;
  const locked = busy || !access.canEdit;
  return <div className="project-work-body">
    <p>Create reusable instructions for your agents. Assign a saved version to coding, review, or a specific task.</p>
    <p className="journey-muted">Skills inherit the task's existing permissions and limits; saving a skill does not start work.</p>
    <details><summary>How skills are used</summary><p>Reviewed tasks receive the exact selected versions in their captured task contract. Manual sessions do not automatically load these skills. A changed assignment needs a newly reviewed run.</p><p>Skills are project-local and exported with project context. Do not include credentials. Provenance is your declaration, not verified attribution. Each immutable version permits 16 KiB of instructions; a reviewed run permits 64 KiB of selected skills. Instructions cannot grant tools, permissions or execution.</p></details>
    {access.message && <p role="status">{access.message}</p>}
    {error && <p role="alert">{error}</p>}
    {!saved ? <><p role="status">{busy ? 'Reading project skills…' : 'Project skills are unavailable.'}</p><Button disabled={busy} onClick={() => setRetry(value => value + 1)}>Reload skills</Button></> : <>
      <p>Registry revision {saved.revision} · {saved.versions.length} immutable versions</p>
      <fieldset disabled={locked} className="workflow-form"><legend>Author a skill version</legend>
        <label>Skill ID<input value={skill.id} maxLength={80} onChange={event => { setSkill({ ...skill, id: event.target.value }); setEditing(true); }} /></label>
        <label>Title<input value={skill.title} maxLength={200} onChange={event => { setSkill({ ...skill, title: event.target.value }); setEditing(true); }} /></label>
        <label>Declared provenance<textarea value={skill.provenance} maxLength={2000} onChange={event => { setSkill({ ...skill, provenance: event.target.value }); setEditing(true); }} /></label>
        <label>Applicable role<select value={skill.roles.join(',')} onChange={event => { setSkill({ ...skill, roles: event.target.value.split(',') as SkillRole[] }); setEditing(true); }}><option value="coding">Coding</option><option value="reviewer">Independent reviewer</option><option value="coding,reviewer">Both roles</option></select></label>
        <label>Instructions<textarea rows={6} value={skill.instructions} maxLength={16384} onChange={event => { setSkill({ ...skill, instructions: event.target.value }); setEditing(true); }} /></label>
        <Button disabled={!editing || !skill.id || !skill.title.trim() || !skill.provenance.trim() || !skill.instructions.trim()} onClick={() => void save(true)}>Save new immutable version</Button>
      </fieldset>
      <details><summary>Saved skill versions</summary>{saved.versions.map(version => <section key={`${version.id}:${version.revision}`}><h4>{version.title} · {version.id} revision {version.revision}</h4><p>{version.roles.join(', ')} · Declared source: {version.provenance}</p><pre>{version.instructions}</pre><p className="journey-muted">Digest {version.digest}</p><Button disabled={locked} onClick={() => { setSkill({ id: version.id, title: version.title, provenance: version.provenance, roles: version.roles, instructions: version.instructions }); setEditing(true); }}>Revise {version.id} from revision {version.revision}</Button></section>)}</details>
      <fieldset disabled={locked} className="workflow-form"><legend>Exact task assignments</legend>
        {assignments.map((assignment, index) => <fieldset key={index}><legend>Assignment {index + 1}</legend>
          <label>Role<select value={assignment.role} onChange={event => setAssignments(assignments.map((row, i) => i === index ? { ...row, role: event.target.value as SkillRole, taskId: '' } : row))}><option value="coding">Coding</option><option value="reviewer">Independent reviewer</option></select></label>
          <label>Task<select value={assignment.taskId ?? ''} onChange={event => setAssignments(assignments.map((row, i) => i === index ? { ...row, taskId: event.target.value } : row))}><option value="">All tasks in this role</option>{assignment.taskId && !tasks.some(task => task.id === assignment.taskId) && <option value={assignment.taskId}>Missing task {assignment.taskId} — repair this selection</option>}{tasks.filter(task => (task.role || 'coding') === assignment.role).map(task => <option key={task.id} value={task.id}>{task.title} · {task.id}</option>)}</select></label>
          <label>Skill revision<select value={`${assignment.skillId}:${assignment.skillRevision}`} onChange={event => { const version = saved.versions.find(value => `${value.id}:${value.revision}` === event.target.value); if (version) setAssignments(assignments.map((row, i) => i === index ? { ...row, skillId: version.id, skillRevision: version.revision } : row)); }}><option value="">Choose a saved version</option>{saved.versions.filter(version => version.roles.includes(assignment.role)).map(version => <option key={`${version.id}:${version.revision}`} value={`${version.id}:${version.revision}`}>{version.title} · {version.id} revision {version.revision}</option>)}</select></label>
          <Button variant="secondary" onClick={() => setAssignments(assignments.filter((_, i) => i !== index))}>Remove assignment {index + 1}</Button>
        </fieldset>)}
        <Button variant="secondary" disabled={assignments.length >= 64 || !saved.versions.length} onClick={() => { const version = saved.versions[saved.versions.length - 1]; setAssignments([...assignments, { role: version.roles[0], skillId: version.id, skillRevision: version.revision }]); }}>Add assignment</Button>
        <Button disabled={!dirty || editing} onClick={() => void save(false)}>Save assignments</Button>
      </fieldset>
      {dirty && <p role="status">Unsaved skill edits. Keep this editor open until you save or discard.</p>}
      <Button variant="secondary" disabled={busy || !dirty} onClick={() => { setAssignments(saved.assignments); setSkill({ id: '', title: '', provenance: '', roles: ['coding'], instructions: '' }); setEditing(false); }}>Discard unsaved skill edits</Button>
      <Button variant="secondary" disabled={busy || dirty} onClick={() => setRetry(value => value + 1)}>Reload saved skills</Button>
    </>}
  </div>;
}
