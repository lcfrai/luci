import { useEffect, useRef, useState } from 'react';
import { personalLuciApi, type HandoffRequest, type PersonalPreferences, type PersonalTurn, type PersonalView } from '../../api/personalLuci';
import { orchestratorApi } from '../../api/orchestrator';
import { portfolioGoalsApi, type PortfolioGoal } from '../../api/portfolioGoals';
import { errorText } from '../../api/product';
import type { Project } from '../../store/types';
import { useProjectDraftScope } from '../../hooks/useProjectDraft';
import { agentConnectionIssue, configuredAgentModel, selectedAgentConnection, useAgentConnections } from '../../hooks/useAgentConnections';
import { useTeamWorkspace } from '../teams/TeamWorkspaceProvider';
import { AgentConnectionSelect } from '../setup/AgentConnectionSelect';
import { Button } from '../fluid/components/button';
import { useProjectHome } from './ProjectHomeProvider';
import { useCockpit } from '../../store/store';
import { readPersonalRecovery, savePersonalRecovery } from './personalRecovery';
import './personal-luci.css';

const empty = (): PersonalPreferences => ({ computeProjectId: '', harness: 'codex', model: '', connectionId: '', projectIds: [], goalIds: [] });
export function PersonalLuci({ projects }: { projects: Project[] }) {
  const scope = useProjectDraftScope('personal-luci'), team = useTeamWorkspace();
  const key = JSON.stringify([scope, team?.teamId, team?.revision, team?.selected?.permissions, projects.map(project => [project.id, project.path])]);
  if (!scope || team?.accessBlocked) return <section className="personal-luci"><h2>Your Luci</h2><p>Connect your account and workspace to continue your private conversation.</p></section>;
  return <ScopedPersonal key={key} scope={scope} projects={projects} />;
}
function ScopedPersonal({ scope, projects }: { scope: string; projects: Project[] }) {
  const [view, setView] = useState<PersonalView | null>(null), [preferences, setPreferences] = useState(empty);
  const [controls, setControls] = useState(false), [goals, setGoals] = useState<PortfolioGoal[]>([]);
  const [error, setError] = useState(''), [notice, setNotice] = useState(''), [busy, setBusy] = useState('');
  const [recovery, setRecovery] = useState(() => { try { return { value: readPersonalRecovery(scope), error: '' }; } catch (cause) { return { value: { message: '', pending: null }, error: errorText(cause) }; } });
  const [message, setMessage] = useState(recovery.value.message);
	const recoveryRef = useRef(recovery.value); recoveryRef.current = recovery.value;
  const [review, setReview] = useState<{ turn: PersonalTurn; projectId: string; message: string; ready?: { revision: number; digest: string }; pending?: HandoffRequest } | null>(null);
  const [archiveId, setArchiveId] = useState('');
  const alive = useRef(true), generation = useRef(0), reviewRef = useRef(review); reviewRef.current = review;
  const home = useProjectHome();
  const { catalogue, error: connectionError } = useAgentConnections();
  const connection = selectedAgentConnection(catalogue, preferences.connectionId);
  const routeIssue = agentConnectionIssue(connection, preferences.harness, preferences.connectionId);
  const activeTurn = view?.turns.find(turn => turn.status === 'reserved' && !turn.withheld);
  const changed = !!view && JSON.stringify(preferences) !== JSON.stringify(view.preferences);
  const locked = !!busy || !!activeTurn || !!recovery.value.pending || !!recovery.error;
  const names = (id: string) => projects.find(project => project.id === id)?.name ?? 'Unavailable project';
  useEffect(() => { alive.current = true; return () => { alive.current = false; ++generation.current; }; }, []);
  async function refresh(before?: number) {
    const ticket = ++generation.current;
    const next = await personalLuciApi.get(undefined, before, archiveId || undefined);
    if (!alive.current || ticket !== generation.current) return;
    setView(next); if (!archiveId && !changed) setPreferences(next.preferences.computeProjectId ? next.preferences : empty());
  }
  useEffect(() => {
    const controller = new AbortController(), ticket = ++generation.current;
    void personalLuciApi.get(controller.signal).then(next => { if (!controller.signal.aborted && ticket === generation.current) { setView(next); setPreferences(next.preferences.computeProjectId ? next.preferences : empty()); } }).catch(cause => { if (!controller.signal.aborted) setError(errorText(cause)); });
    return () => controller.abort();
  }, []);
  useEffect(() => {
    if (!controls) return;
    const controller = new AbortController();
    void portfolioGoalsApi.get(controller.signal).then(next => { if (!controller.signal.aborted) setGoals(next.goals); }).catch(cause => { if (!controller.signal.aborted) setError(errorText(cause)); });
    return () => controller.abort();
  }, [controls]);
  useEffect(() => {
    if (!activeTurn) return;
    const controller = new AbortController(); let count = 0, timer: ReturnType<typeof setTimeout> | undefined;
    const poll = async () => {
      if (controller.signal.aborted) return;
      if (++count > 90) { setNotice('This request is still retained. Check its status when you are ready; nothing has been resent.'); return; }
      if (document.visibilityState === 'hidden') { timer = setTimeout(poll, 2000); return; }
      try {
        const job = await personalLuciApi.status(activeTurn.id, controller.signal);
        if (controller.signal.aborted) return;
        if (['succeeded', 'failed', 'cancelled'].includes(job.status)) { await refresh(); return; }
        setNotice(job.message); timer = setTimeout(poll, 2000);
      } catch (cause) { if (!controller.signal.aborted) setError(errorText(cause)); }
    };
    timer = setTimeout(poll, 1000);
    return () => { controller.abort(); clearTimeout(timer); };
  }, [activeTurn?.id]);
  function remember(nextMessage: string, pending = recovery.value.pending) {
    const next = { message: nextMessage, pending };
    setMessage(nextMessage);
    try { savePersonalRecovery(scope, recoveryRef.current, next); recoveryRef.current = next; setRecovery({ value: next, error: '' }); return true; }
    catch (cause) { setRecovery(current => ({ ...current, error: errorText(cause) })); return false; }
  }
  async function saveChoices() {
    if (!view || locked) return;
    setBusy('preferences'); setError('');
    try { const next = await personalLuciApi.preferences(view.revision, preferences); if (alive.current) { setView(next); setPreferences(next.preferences); setControls(false); setNotice('Your choices are saved. Nothing has been sent.'); } }
    catch (cause) { if (alive.current) setError(errorText(cause)); }
    finally { if (alive.current) setBusy(''); }
  }
  async function chooseConversation(id: string) {
    if (locked) return; const ticket = ++generation.current; setBusy('history'); setError('');
    try { const next = await personalLuciApi.get(undefined, undefined, id || undefined); if (alive.current && ticket === generation.current) { setView(next); setArchiveId(id); setReview(null); } }
    catch (cause) { if (alive.current) setError(errorText(cause)); }
    finally { if (alive.current) setBusy(''); }
  }
  async function continueConversation() {
    if (!view || view.archived || locked || changed || !view.turns.length || review) return;
    setBusy('continue'); setError('');
    try { const next = await personalLuciApi.continue(view.revision, view.conversationId); if (alive.current) { setView(next); setArchiveId(''); setNotice('Earlier messages are saved in conversation history. Your draft and choices stay here; archived messages are not automatically included in a new response.'); } }
    catch (cause) { if (alive.current) setError(errorText(cause)); }
    finally { if (alive.current) setBusy(''); }
  }
  async function send() {
    if (!view || view.archived || locked || changed || !message.trim() || !view.context?.digest || routeIssue || !preferences.computeProjectId) return;
    const request = { expectedRevision: view.revision, expectedContextDigest: view.context.digest, idempotencyKey: crypto.randomUUID(), message };
    if (!remember(message, request)) return;
    setBusy('send'); setError('');
    try { await personalLuciApi.send(request); if (alive.current && remember('', null)) await refresh(); }
    catch (cause) { if (alive.current) setError(errorText(cause) + ' Check the original request before sending another message.'); }
    finally { if (alive.current) setBusy(''); }
  }
  async function resolveOriginal() {
    if (!recovery.value.pending || busy) return;
    setBusy('resolve'); setError('');
    try { const result = await personalLuciApi.resolve(recovery.value.pending); if (!alive.current) return; if (result.status === 'accepted') remember('', null); else remember(message, null); await refresh(); setNotice(result.status === 'accepted' ? 'Your original request is retained. Nothing was resent.' : 'The request was not sent. Your message is still a draft.'); }
    catch (cause) { if (alive.current) setError(errorText(cause)); }
    finally { if (alive.current) setBusy(''); }
  }
  async function actOnTurn(id: string, cancel: boolean) {
    if (busy) return; setBusy(cancel ? 'cancel' : 'check'); setError('');
    try { if (cancel) await personalLuciApi.cancel(id); else await personalLuciApi.check(id); if (alive.current) await refresh(); }
    catch (cause) { if (alive.current) setError(errorText(cause)); }
    finally { if (alive.current) setBusy(''); }
  }
  async function prepareProjectMessage() {
    if (!review || busy) return;
    const original = review; setBusy('review'); setError('');
    try { const project = await orchestratorApi.get(original.projectId); if (alive.current && reviewRef.current?.projectId === original.projectId && reviewRef.current?.message === original.message) setReview({ ...original, ready: { revision: project.revision, digest: project.context.digest } }); }
    catch (cause) { if (alive.current) setError(errorText(cause)); }
    finally { if (alive.current) setBusy(''); }
  }
  async function forward() {
    if (!view || !review?.ready || busy || review.pending) return;
    const original = review;
    const request: HandoffRequest = { expectedRevision: view.revision, projectId: original.projectId, message: original.message, projectRevision: original.ready!.revision, projectContextDigest: original.ready!.digest, idempotencyKey: crypto.randomUUID() };
    setReview({ ...original, pending: request }); setBusy('handoff'); setError('');
    try { await personalLuciApi.handoff(original.turn.id, request); if (alive.current) { setReview(null); await refresh(); } }
    catch (cause) { if (alive.current) setError(errorText(cause) + ' Check this project message; do not resend it.'); }
    finally { if (alive.current) setBusy(''); }
  }
  async function checkHandoff(id: string) {
    if (busy) return; setBusy('handoff-check');
    try { await personalLuciApi.resolveHandoff(id); if (alive.current) { setReview(null); await refresh(); } }
    catch (cause) { if (alive.current) setError(errorText(cause)); }
    finally { if (alive.current) setBusy(''); }
  }
  async function exportHistory() {
    if (busy) return; setBusy('export');
    try { const data = await personalLuciApi.export(archiveId || undefined); if (!alive.current) return; const url = URL.createObjectURL(new Blob([JSON.stringify(data, null, 2)], { type: 'application/json' })); const link = document.createElement('a'); link.href = url; link.download = 'luci-private-conversation.json'; link.click(); URL.revokeObjectURL(url); }
    catch (cause) { if (alive.current) setError(errorText(cause)); }
    finally { if (alive.current) setBusy(''); }
  }
  return <section className="personal-luci" aria-labelledby="personal-luci-title">
    <header><div><h2 id="personal-luci-title">Your Luci</h2><p>A private conversation across the projects you include.</p></div><Button variant="ghost" disabled={!!busy} onClick={() => void exportHistory()}>Export conversation</Button></header>
    {view && <div className="personal-history"><label>Conversation history<select aria-label="Conversation history" value={archiveId} disabled={locked || !!review} onChange={event => void chooseConversation(event.target.value)}><option value="">Current conversation</option>{view.archives.map((archive, index) => <option key={archive.id} value={archive.id}>Saved part {index + 1} · {new Date(archive.lastMessageAt).toLocaleDateString()} · {archive.turns} messages</option>)}</select></label>{!view.archived && <Button variant="ghost" disabled={locked || changed || !view.turns.length || !!review} onClick={() => void continueConversation()}>Continue conversation</Button>}{view.archived && <p>Saved history. Return to Current conversation to write; your draft stays there.</p>}</div>}
    {error && <p role="alert">{error}</p>}{recovery.error && <p role="alert">{recovery.error} Your current text remains here; copy it before leaving.</p>}{notice && <p role="status">{notice}</p>}
    {!view && <p role="status">Reading your conversation… <Button variant="ghost" onClick={() => void refresh().catch(cause => setError(errorText(cause)))}>Refresh</Button></p>}
    <div className="personal-transcript" aria-label="Private conversation" aria-live="polite">
      {view?.hasEarlier && <Button variant="ghost" disabled={locked} onClick={() => void refresh(view.beforeSequence).catch(cause => setError(errorText(cause)))}>Earlier messages</Button>}
      {view?.turns.length === 0 && <p>What are you trying to achieve? Tell Luci what matters, then choose which projects it can use.</p>}
      {view?.turns.map(turn => <article key={turn.id} className="personal-turn">
        {turn.withheld ? <p>Earlier project context is no longer accessible. This message is withheld.</p> : <><p className="personal-human"><strong>You</strong><span>{turn.message}</span></p>
          {turn.answer && <><p><strong>Luci</strong><span>{turn.answer.summary}</span></p>{turn.answer.question && <p>{turn.answer.question}</p>}{turn.answer.options.map(option => <Button key={option} variant="ghost" disabled={locked || view.archived} onClick={() => remember(option, null)}>{option}</Button>)}
            {turn.answer.discussion && !turn.handoff && !view.archived && <Button variant="secondary" disabled={locked} onClick={() => setReview({ turn, ...turn.answer!.discussion! })}>Review message for {names(turn.answer.discussion.projectId)}</Button>}
          </>}
          {turn.status === 'reserved' && <p>Request retained · {busy === 'check' ? 'Checking…' : 'waiting for its recorded outcome'}. <Button variant="ghost" disabled={!!busy} onClick={() => void actOnTurn(turn.id, false)}>Check status</Button><Button variant="ghost" disabled={!!busy} onClick={() => void actOnTurn(turn.id, true)}>Cancel request</Button></p>}
          {turn.status === 'failed' && <p>That request did not complete a valid response. Nothing was applied to a project.</p>}{turn.status === 'cancelled' && <p>Request cancelled.</p>}
          {turn.handoff && <p>{turn.handoff.status === 'forwarded' ? `Your reviewed message is in ${names(turn.handoff.projectId)}.` : 'Project message outcome is not confirmed.'}{turn.handoff.status !== 'forwarded' && <Button variant="ghost" disabled={!!busy} onClick={() => void checkHandoff(turn.id)}>Check project message</Button>}{turn.handoff.status === 'forwarded' && <Button variant="ghost" onClick={() => home ? home.open(turn.handoff!.projectId, 'discover', 'luci') : useCockpit.getState().selectProject(turn.handoff!.projectId, 'discover', 'luci')}>Open project Luci</Button>}</p>}
        </>}
      </article>)}
    </div>
    {review && <section className="personal-review" aria-label="Review project message"><h3>Message for {names(review.projectId)}</h3><p>Only this text enters the project conversation. Your private history and goals stay here. The project reply cannot apply changes or start work.</p><label>Reviewed message<textarea maxLength={16000} value={review.message} disabled={!!busy || !!review.pending} onChange={event => setReview({ ...review, message: event.target.value, ready: undefined })} /></label><p>The project reply uses {review.turn.context.preferences.harness} · {review.turn.context.preferences.model} with current project permissions.</p>{review.pending ? <Button disabled={!!busy} onClick={() => void checkHandoff(review.turn.id)}>Check original project message</Button> : review.ready ? <Button disabled={!!busy || !review.message.trim()} onClick={() => void forward()}>Send reviewed message</Button> : <Button disabled={!!busy || !review.message.trim()} onClick={() => void prepareProjectMessage()}>Review current project context</Button>}<Button variant="ghost" disabled={!!busy} onClick={() => setReview(null)}>Close review</Button></section>}
    <div className="personal-quick-prompts">{['What needs me?', 'What should we do next?', 'Plan a new project'].map(prompt => <Button key={prompt} variant="ghost" disabled={locked || !!view?.archived} onClick={() => remember(prompt, null)}>{prompt}</Button>)}</div>
    <form onSubmit={event => { event.preventDefault(); void send(); }}><label htmlFor="personal-message">Message Luci</label><textarea id="personal-message" rows={3} maxLength={16000} value={message} disabled={!!busy || !!view?.archived || !!recovery.value.pending} onChange={event => remember(event.target.value, null)} placeholder="What would you like to achieve?" /><div className="personal-composer-actions"><Button type="submit" disabled={locked || !!view?.archived || changed || !message.trim() || !view?.context.digest || !!routeIssue}>{busy === 'send' ? 'Sending…' : 'Send'}</Button>{recovery.value.pending && <Button type="button" variant="secondary" disabled={!!busy} onClick={() => void resolveOriginal()}>Check original request</Button>}<Button type="button" variant="ghost" disabled={locked || !!view?.archived} aria-expanded={controls} onClick={() => setControls(value => !value)}>{preferences.computeProjectId ? 'Connection and included context' : 'Choose connection'}</Button></div></form>
    {preferences.computeProjectId && !controls && <p className="personal-route">{preferences.model} · {names(preferences.computeProjectId)} supplies this request's existing permissions and capacity.</p>}
    {controls && !view?.archived && <div className="personal-choices"><p>Choose where Luci runs. This does not start work or change any project.</p><fieldset disabled={locked}><label>Compute project<select value={preferences.computeProjectId} onChange={event => setPreferences({ ...preferences, computeProjectId: event.target.value })}><option value="">Choose a project</option>{projects.map(project => <option key={project.id} value={project.id}>{project.name}</option>)}</select></label><label>Agent<select value={preferences.harness} onChange={event => setPreferences({ ...preferences, harness: event.target.value, model: configuredAgentModel(connection, event.target.value) ?? '' })}><option value="codex">Codex</option><option value="claude-code">Claude Code</option><option value="opencode">OpenCode</option></select></label><AgentConnectionSelect catalogue={catalogue} value={preferences.connectionId} label="Luci connection" onChange={id => { const exact = id === undefined ? catalogue?.activeConnectionId ?? '' : id; setPreferences({ ...preferences, connectionId: exact, model: configuredAgentModel(selectedAgentConnection(catalogue, exact), preferences.harness) ?? '' }); }} /><label>Model<input value={preferences.model} maxLength={512} onChange={event => setPreferences({ ...preferences, model: event.target.value })} /></label><details><summary>Include project context and saved goals</summary><p>Include up to 15 projects plus the compute project. Only this selection enters the next response. Other history stays saved. A new-project request is discussion, not project creation.</p>{projects.map(project => <label key={project.id}><input type="checkbox" checked={preferences.projectIds.includes(project.id)} disabled={!preferences.projectIds.includes(project.id) && preferences.projectIds.length >= 15} onChange={event => setPreferences({ ...preferences, projectIds: event.target.checked ? [...preferences.projectIds, project.id] : preferences.projectIds.filter(id => id !== project.id), goalIds: [] })} />{project.name}</label>)}{goals.map(goal => <label key={goal.id}><input type="checkbox" checked={preferences.goalIds.includes(goal.id)} disabled={!goal.projects.every(project => preferences.projectIds.includes(project.projectId))} onChange={event => setPreferences({ ...preferences, goalIds: event.target.checked ? [...preferences.goalIds, goal.id] : preferences.goalIds.filter(id => id !== goal.id) })} />{goal.title}{!goal.projects.every(project => preferences.projectIds.includes(project.projectId)) && ' · include its projects first'}</label>)}</details><Button disabled={!preferences.computeProjectId || !preferences.connectionId || !preferences.model.trim() || !!routeIssue} onClick={() => void saveChoices()}>Save choices</Button></fieldset>{(routeIssue || connectionError) && <p role="status">{routeIssue || connectionError}</p>}{projects.length === 0 && <p>Create a project using the existing New project action below before selecting compute permissions.</p>}</div>}
  </section>;
}
