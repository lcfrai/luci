import { useEffect, useRef, useState } from 'react';
import { ApiError } from '../../api/client';
import { errorText, productApi } from '../../api/product';
import { projectRunsApi, runOperationNeedsRecovery, swarmIntegrationDiagnostic, type ProjectRun, type RunOperation } from '../../api/projectRuns';
import { swarmHandoffApi, type TaskHandoffMutation, type TaskHandoffView } from '../../api/swarmHandoff';
import { useProjectAccess } from '../../hooks/useProjectAccess';
import { useProjectDraftScope } from '../../hooks/useProjectDraft';
import { useCockpit } from '../../store/store';
import { WorkflowDialog } from '../common/WorkflowDialog';
import { Button } from '../fluid/components/button';
import './swarm-task-handoff.css';

interface Props {
  projectId: string; projectName: string; run: ProjectRun; operation: RunOperation; taskTitle: string;
  canRead: boolean; canEdit: boolean; onRun(run: ProjectRun): void; onClose(): void;
}
interface Draft {owner: number; questionId: string; question: string; digest: string; answer: string; original: string}
interface Read {key: string; value?: TaskHandoffView; error?: string}
const stateLabels = { 'not-recorded': 'No captured task report', captured: 'Agent report captured', missing: 'No report file found', invalid: 'Report could not be accepted', unavailable: 'Report capture unavailable' };
const reportedOutcome = {passed: 'Reported passed', failed: 'Reported failed', 'not-run': 'Reported not run', unknown: 'Reported outcome unknown'};
const denied = (cause: unknown) => cause instanceof ApiError && (cause.status === 401 || cause.status === 403);
const sameAttempt = (run: ProjectRun, operation: RunOperation) => run.operations.some(saved => saved.id === operation.id && saved.taskId === operation.taskId && saved.childId === operation.childId && saved.worktreeId === operation.worktreeId && saved.requestDigest === operation.requestDigest);

/** An explicit report view. It never collects, executes or answers on mount. */
export function SwarmTaskHandoff(props: Props) {
  const scope = useProjectDraftScope(props.projectId);
  return <TaskHandoff key={JSON.stringify([productApi.baseUrl, scope, props.projectId, props.run.id, props.operation.id, props.operation.childId])} {...props} scope={scope} />;
}
function TaskHandoff({projectId, projectName, run, operation, taskTitle, canRead: parentRead, canEdit: parentEdit, onRun, onClose, scope}: Props & {scope: string | null}) {
  const access = useProjectAccess(projectId);
  const canRead = !!scope && parentRead && access.canRead && run.projectId === projectId && sameAttempt(run, operation);
  const canEdit = canRead && parentEdit && access.canEdit;
  const canRequest = canRead && !access.loading && !access.error;
  const authority = useRef({canRead, canEdit, readEpoch: 0, writeEpoch: 0});
  if (authority.current.canRead !== canRead) { authority.current.readEpoch++; authority.current.writeEpoch++; }
  else if (authority.current.canEdit !== canEdit) authority.current.writeEpoch++;
  authority.current.canRead = canRead; authority.current.canEdit = canEdit;
  const latest = useRef({run, onRun, canRead, canEdit}); latest.current = {run, onRun, canRead, canEdit};
  const alive = useRef(true), inflight = useRef<object | null>(null), readAttempt = useRef<string | null>(null), mutationEpoch = useRef(0);
  const [refresh, setRefresh] = useState(0), [read, setRead] = useState<Read | null>(null);
  const [draft, setDraft] = useState<Draft | null>(null), [closeReview, setCloseReview] = useState(false);
  const [pending, setPending] = useState(''), [error, setError] = useState(''), [notice, setNotice] = useState('');
  const [messageOwner, setMessageOwner] = useState(authority.current.readEpoch);
  const [needsRefresh, setNeedsRefresh] = useState(false);
  const readKey = JSON.stringify([authority.current.readEpoch, run.revision, refresh]);
  const current = canRead && read?.key === readKey ? read : null;
  const value = current?.value;
  const integrationDiagnostic = swarmIntegrationDiagnostic(value?.integration);
  const visibleDraft = canRead && draft?.owner === authority.current.readEpoch ? draft : null;
  const dirty = !!visibleDraft && visibleDraft.answer !== visibleDraft.original;
  const busy = !!pending && canRead;
  const draftKey = `task-handoff-answer:${scope}:${run.id}:${operation.id}`;
  useEffect(() => { alive.current = true; return () => { alive.current = false; authority.current.readEpoch++; authority.current.writeEpoch++; }; }, []);
  useEffect(() => {
    useCockpit.getState().setUnsavedProjectDraft(draftKey, dirty ? {projectId, label: 'Task answer', lens: 'develop', discard: () => setDraft(null)} : null);
    return () => useCockpit.getState().setUnsavedProjectDraft(draftKey, null);
  }, [draftKey, projectId, dirty]);
  useEffect(() => {
    if (!canRead) { setRead(null); setDraft(null); setCloseReview(false); return; }
    if (!canRequest || readAttempt.current === readKey) return;
    const epoch = authority.current.readEpoch, mutation = mutationEpoch.current, startedDuringMutation = !!inflight.current, controller = new AbortController();
    let done = false; readAttempt.current = readKey;
    const owned = () => alive.current && !controller.signal.aborted && authority.current.readEpoch === epoch && mutationEpoch.current === mutation && latest.current.canRead && latest.current.run.revision === run.revision;
    // An explicit refresh also obtains the current run revision for the next CAS.
    // Neither read authorizes execution or implicitly retries an answer.
    void Promise.all([swarmHandoffApi.get(projectId, run.id, operation, controller.signal), refresh ? projectRunsApi.get(projectId, run.id) : Promise.resolve(run)])
      .then(([handoff, latestRun]) => {
        if (!owned()) return;
        if (latestRun.revision < run.revision || !sameAttempt(latestRun, operation)) throw new Error('This attempt changed. Close the result and refresh team status.');
        done = true;
        const key = JSON.stringify([epoch, latestRun.revision, refresh]); readAttempt.current = key;
        setRead({key, value: handoff});
        if (!startedDuringMutation && !inflight.current) setNeedsRefresh(false);
        if (latestRun.revision !== run.revision) latest.current.onRun(latestRun);
      }).catch(cause => {
        if (!owned()) return; done = true;
        setRead({key: readKey, error: errorText(cause)});
        if (denied(cause)) { setDraft(null); setCloseReview(false); setError(''); setNotice(''); }
      });
    return () => { controller.abort(); if (!done && readAttempt.current === readKey) readAttempt.current = null; };
  }, [canRead, canRequest, readKey]);

  const refreshResult = () => {
    if (!canRequest || inflight.current) return;
    setError(''); setNotice(''); setMessageOwner(authority.current.readEpoch); setRefresh(value => value + 1);
  };
  const close = () => { if (busy) return; if (dirty) setCloseReview(true); else onClose(); };
  const draftQuestionMatches = !!visibleDraft && !!value?.digest && visibleDraft.digest === value.digest && value.questionIds.includes(visibleDraft.questionId);
  const recordedAnswer = value?.answers.find(answer => answer.questionId === visibleDraft?.questionId)?.answer ?? '';
  const answerBaseChanged = !!visibleDraft && draftQuestionMatches && recordedAnswer !== visibleDraft.original;
  const draftMatches = draftQuestionMatches && !answerBaseChanged;
  const answerBytes = visibleDraft ? new TextEncoder().encode(visibleDraft.answer).length : 0;
  const integrated = value?.integration?.state === 'integrated';
  const settled = operation.allocation === 'ready' && ['review-required', 'succeeded', 'failed', 'cancelled'].includes(operation.status) && !runOperationNeedsRecovery(operation);

  async function change(action: 'collect' | 'answer') {
    if (!canEdit || !canRequest || inflight.current || needsRefresh || !value || (action === 'collect' && (!settled || integrated || dirty)) ||
      (action === 'answer' && (!visibleDraft || !draftMatches || !visibleDraft.answer.trim() || answerBytes > 4000 || !dirty))) return;
    const epoch = authority.current.writeEpoch, readEpoch = authority.current.readEpoch, token = {}, revision = run.revision;
    mutationEpoch.current++;
    inflight.current = token; setPending(action); setError(''); setNotice(''); setMessageOwner(readEpoch);
    const owned = () => alive.current && authority.current.writeEpoch === epoch && latest.current.canEdit && latest.current.run.revision === revision;
    const requireFreshReview = () => {
      if (!alive.current || authority.current.readEpoch !== readEpoch || !latest.current.canRead) return;
      setNeedsRefresh(true);
      setError('Project access or the build changed while saving. The outcome is unconfirmed; refresh the task result before deciding whether to retry.');
    };
    try {
      const result: TaskHandoffMutation = action === 'collect' ? await swarmHandoffApi.collect(run, operation)
        : await swarmHandoffApi.answer(run, operation, visibleDraft!.digest, visibleDraft!.questionId, visibleDraft!.answer);
      if (!owned()) { requireFreshReview(); return; }
      const key = JSON.stringify([authority.current.readEpoch, result.run.revision, refresh]); readAttempt.current = key;
      setRead({key, value: result.handoff}); setNeedsRefresh(false);
      if (action === 'answer') setDraft(null);
      setNotice(action === 'answer' ? 'Answer recorded for future task context. No running agent was changed or restarted.' : 'Task report capture recorded. Its claims still need review; no checks or integration were performed.');
      latest.current.onRun(result.run);
    } catch (cause) {
      if (!owned()) { requireFreshReview(); return; }
      if (denied(cause)) { setRead({key: readKey, error: 'Project access was refused. Refresh after access is restored.'}); setDraft(null); setCloseReview(false); }
      const rejected = cause instanceof ApiError && cause.status >= 400 && cause.status < 500 && cause.status !== 408;
      setError(`${errorText(cause)} ${rejected ? 'Refresh the task result and review it before trying again.' : 'The save outcome is unconfirmed; it may already have applied. Refresh the task result before deciding whether to retry.'}`);
      setNeedsRefresh(true);
    } finally {
      if (inflight.current === token) { inflight.current = null; if (alive.current) setPending(''); }
    }
  }

  const closePrompt = closeReview && <section className="handoff-draft" role="alert"><h3>Discard this unsaved answer?</h3><p>The recorded report and previous answers will remain unchanged.</p><div className="workflow-actions"><Button variant="secondary" onClick={() => setCloseReview(false)}>Keep editing</Button><Button variant="tertiary" onClick={() => { setDraft(null); onClose(); }}>Discard draft and close</Button></div></section>;

  return <WorkflowDialog readAccess={{...access, canRead}} unavailableContent={closePrompt} title="Task result and questions" description="Review what this agent reported, the evidence Luci actually recorded, and decisions needed by later tasks. This view never starts an agent or accepts its work." busy={busy} onClose={close}>
    <div className="swarm-task-handoff">
      {!canRead ? <p role="status">Reading access is unavailable for this task attempt. Close this view and reopen it after access is restored.</p> : <>
        <p><strong>{projectName} · {taskTitle}</strong></p>
        <p className="handoff-muted">Attempt {operation.attempt} · build revision {run.revision}. Answers are shared project context, not execution permission.</p>
        <div className="workflow-actions"><Button variant="secondary" disabled={!canRequest || !!pending} onClick={refreshResult}>Refresh task result</Button><Button variant="secondary" disabled={!canEdit || !canRequest || !!pending || needsRefresh || !value || !settled || integrated || dirty} onClick={() => void change('collect')}>{pending === 'collect' ? 'Capturing report…' : value?.receipt ? 'Capture report again' : 'Capture task report'}</Button></div>
        {!settled && <p className="handoff-muted">Capture becomes available after this attempt settles and any required recovery is finished.</p>}
        {integrated && <p className="handoff-muted">This task is integrated. Its captured report cannot be replaced; answers can still be amended for future task context.</p>}
        {!canEdit && <p className="handoff-muted">Editing access is required to capture a report or save an answer. Reading does not require execution access.</p>}
        {canRequest && !current && <p role="status">Reading recorded task result…</p>}
        {!canRequest && <p role="status">{access.message || 'Checking project permissions…'}</p>}
        {current?.error && <p role="alert">{current.error}</p>}
        {messageOwner === authority.current.readEpoch && error && <p role="alert">{error}</p>}{messageOwner === authority.current.readEpoch && notice && <p role="status">{notice}</p>}
        {value && <>
          <section aria-label="Agent-reported result"><h3>{stateLabels[value.status]}</h3>
            <p className="handoff-muted">Agent statements below are claims. They do not establish passing checks, integration or task completion.</p>
            {!value.receipt?.reported ? <p>{value.status === 'not-recorded' ? 'This attempt has no saved report yet. Older agents may not produce a structured report; inspect their retained output separately.' : value.status === 'missing' ? 'No structured report was found when capture was attempted. No summary, checks or questions have been invented.' : value.status === 'invalid' ? 'The report did not match the supported bounded format. Inspect retained output and correct the report before capturing again.' : 'The report could not be safely read. Inspect the retained attempt and its recovery state before capturing again.'}</p> : <>
              <p className="handoff-text">{value.receipt.reported.summary || 'The agent supplied no summary.'}</p>
              {!!value.receipt.reported.changedFiles.length && <details><summary>Files the agent says it changed ({value.receipt.reported.changedFiles.length})</summary><ul>{value.receipt.reported.changedFiles.map((path, index) => <li key={`${index}:${path}`}><code>{path}</code></li>)}</ul><p>Inspect the retained files and integration review to verify these claims.</p></details>}
              {!!value.receipt.reported.decisions.length && <details><summary>Decisions reported by the agent</summary><ul>{value.receipt.reported.decisions.map((decision, index) => <li key={index}><strong>{decision.decision}</strong><p>{decision.rationale}</p></li>)}</ul></details>}
              <section aria-label="Agent-reported checks"><h4>Agent-reported checks</h4>{value.receipt.reported.reportedChecks.length ? <ul>{value.receipt.reported.reportedChecks.map((check, index) => <li key={index}><strong>{reportedOutcome[check.outcome]}</strong><pre>{check.command}</pre><p>{check.detail}</p></li>)}</ul> : <p>The agent reported no checks.</p>}</section>
              {!!value.receipt.reported.limitations.length && <section aria-label="Reported limitations"><h4>Reported limitations</h4><ul>{value.receipt.reported.limitations.map((limitation, index) => <li key={index}>{limitation}</li>)}</ul></section>}
            </>}
          </section>
          <section aria-label="Recorded verification evidence"><h3>Recorded verification evidence</h3><p className="handoff-muted">Checks recorded for this task’s captured source. Agent-reported checks appear separately above.</p>
            {value.recordedChecks.length ? <ul>{value.recordedChecks.map(check => <li key={check.checkId}><strong>Recorded check {check.checkId}</strong><p>{check.execution} · source <code>{check.commit}</code></p>{check.commands.map((command, index) => <p key={index}>{command.label}: {command.status} · exit {command.exitCode}</p>)}{!check.originalAvailable && <p className="handoff-muted">The original check record is no longer available; this is retained evidence.</p>}</li>)}</ul> : <p>No recorded checks match this task’s captured source.</p>}
            <p>Integration: {value.integration?.state === 'integrated' ? 'Recorded as integrated; this does not establish correctness.' : value.integration?.state === 'integrating' ? 'Integration is in progress; its outcome is not confirmed.' : value.integration?.state === 'unknown' ? 'Outcome unconfirmed; inspect integration before continuing.' : 'No recorded integration.'}</p>
            {integrationDiagnostic && <p className="handoff-warning">{integrationDiagnostic}</p>}
          </section>
          {!!value.receipt?.reported?.questions.length && <section aria-label="Agent questions"><h3>Questions for the project</h3><p className="handoff-muted">Answers inform future tasks. Saving an answer does not start or restart an agent; existing task inputs stay unchanged.</p><ol>{value.receipt.reported.questions.map((question, index) => {
            const id = value.questionIds[index], answer = value.answers.find(item => item.questionId === id);
            return <li key={id}><h4>{question.question}</h4>{question.blocking && <p className={answer ? "handoff-muted" : "handoff-warning"}>{answer ? "Originally flagged as blocking by the agent." : "Agent flagged this as blocking."}</p>}<p>{question.impact}</p>{answer && <div className="handoff-answer"><strong>Recorded answer</strong><p className="handoff-text">{answer.answer}</p><p className="handoff-muted">{answer.attribution || 'Project editor'} · {new Date(answer.answeredAt).toLocaleString()}</p></div>}<Button variant="secondary" disabled={!canEdit || !!pending || needsRefresh || !!visibleDraft} onClick={() => setDraft({owner: authority.current.readEpoch, questionId: id, question: question.question, digest: value.digest!, answer: answer?.answer ?? '', original: answer?.answer ?? ''})}>{answer ? 'Amend recorded answer' : 'Answer question'}</Button></li>;
          })}</ol></section>}
          {value.receipt && <details><summary>Captured source and context</summary><dl><div><dt>Captured</dt><dd>{new Date(value.receipt.capturedAt).toLocaleString()}</dd></div><div><dt>Source commit</dt><dd><code>{value.receipt.sourceCommit || 'Unavailable'}</code>{value.receipt.sourceDirty && ' · uncommitted changes were present'}</dd></div><div><dt>Source fingerprint</dt><dd><code>{value.receipt.sourceDigest || 'Unavailable'}</code></dd></div><div><dt>Captured context</dt><dd><code>{value.receipt.contextDigest || 'Unavailable'}</code></dd></div><div><dt>Report receipt</dt><dd><code>{value.digest}</code></dd></div></dl>{value.receipt.reasonCode && <p className="handoff-muted">Capture detail: {value.receipt.reasonCode}</p>}<p>This records the source at capture time. It is not a current checkout comparison or permission to integrate.</p></details>}
        </>}
        {visibleDraft && <section className="handoff-draft" aria-label="Task answer draft"><h3>Answer this recorded question</h3><p>{visibleDraft.question}</p><label className="luci-field"><span>Your answer</span><textarea autoFocus rows={4} maxLength={4000} value={visibleDraft.answer} disabled={!canEdit || !!pending} onChange={event => setDraft({...visibleDraft, answer: event.target.value})} /></label><p className={answerBytes > 4000 ? 'handoff-warning' : 'handoff-muted'}>{answerBytes.toLocaleString()} / 4,000 UTF-8 bytes. Changes are not saved until you choose Save answer.</p>{!draftQuestionMatches && <p className="handoff-warning">The report changed or is being refreshed. Your draft is kept. Review the current question before saving; copy your text before discarding an outdated draft.</p>}
          {answerBaseChanged && <div role="alert"><p>{recordedAnswer === visibleDraft.answer ? 'Your draft matches the answer now recorded. No further save is needed.' : 'The recorded answer changed while you were editing. Your draft is kept; review the new answer above before choosing to amend it.'}</p><Button variant="secondary" disabled={!!pending || (recordedAnswer !== visibleDraft.answer && !canEdit)} onClick={() => recordedAnswer === visibleDraft.answer ? setDraft(null) : setDraft({...visibleDraft, original: recordedAnswer})}>{recordedAnswer === visibleDraft.answer ? 'Dismiss already recorded draft' : 'Use draft to amend current answer'}</Button></div>}
          <div className="workflow-actions"><Button disabled={!canEdit || !!pending || needsRefresh || !draftMatches || !dirty || !visibleDraft.answer.trim() || answerBytes > 4000} onClick={() => void change('answer')}>{pending === 'answer' ? 'Saving answer…' : 'Save answer'}</Button><Button variant="ghost" disabled={!!pending} onClick={() => setDraft(null)}>Discard answer draft</Button></div></section>}
        {closePrompt}
      </>}
      <div className="workflow-actions"><Button variant="secondary" disabled={busy} onClick={close}>Close task result</Button></div>
    </div>
  </WorkflowDialog>;
}
