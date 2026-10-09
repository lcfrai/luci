import { useEffect, useRef, useState } from 'react';
import { errorText } from '../../api/product';
import type { ProjectRun, RunOperation } from '../../api/projectRuns';
import { swarmCheckpointApi, type SwarmCheckpointReview } from '../../api/swarmCheckpoint';
import { Button } from '../fluid/components/button';
import { WorkflowDialog, type WorkflowDialogReadAccess } from '../common/WorkflowDialog';

/** The parent binds this view to one authority epoch and exact run revision. */
export function SwarmCheckpointDialog({run, operation, taskTitle, projectName, readAccess, canContinue, busy, error, onClose, onConfirm, getCloseFocusTarget}: {
  run: ProjectRun; operation: RunOperation; taskTitle: string; projectName: string;
  readAccess: WorkflowDialogReadAccess; canContinue: boolean; busy: boolean; error: string; onClose(): void;
  onConfirm(review: SwarmCheckpointReview, idempotencyKey: string): void;
  getCloseFocusTarget?(openingControl: HTMLElement | null): HTMLElement | null;
}) {
  const [review, setReview] = useState<SwarmCheckpointReview | null>(null);
  const [loading, setLoading] = useState(true), [readError, setReadError] = useState('');
  const [refresh, setRefresh] = useState(0), [approved, setApproved] = useState(false);
  const keys = useRef(new Map<string, string>());
  useEffect(() => {
    const controller = new AbortController(); let live = true;
    setReview(null); setReadError(''); setLoading(true); setApproved(false);
    void swarmCheckpointApi.review(run, operation, controller.signal).then(value => { if (live) setReview(value); })
      .catch(cause => { if (live) setReadError(errorText(cause)); }).finally(() => { if (live) setLoading(false); });
    return () => { live = false; controller.abort(); };
  }, [run.projectId, run.id, run.revision, operation.id, operation.childId, refresh]);
  return <WorkflowDialog readAccess={readAccess} title="Continue from retained changes" onClose={onClose} busy={busy} getCloseFocusTarget={getCloseFocusTarget}
    description="Review the retained changes to copy into a new isolated attempt. The previous attempt stays intact. This prepares a workspace; starting its agent remains a separate action.">
    <dl className="swarm-recovery-scope">
      <div><dt>Project</dt><dd>{projectName}</dd></div>
      <div><dt>Task</dt><dd>{taskTitle}</dd></div>
      <div><dt>Build</dt><dd><code>{run.id}</code> · revision {run.revision}</dd></div>
      <div><dt>Retained attempt</dt><dd>{operation.attempt} · <code>{operation.id}</code></dd></div>
    </dl>
    <p>These changes may be unfinished. The task stays incomplete, and dependent tasks still wait for reviewed integration. The saved task scope and acceptance criteria apply to the new attempt.</p>
    <p className="journey-muted">Retry in a new workspace starts from the current project without copying this output. Continuing here carries only the complete changes reviewed below.</p>
    {loading && <p role="status">Reading complete retained changes and checking the current source…</p>}
    {readError && <p role="alert">{readError}</p>}
    {review && <>
      <p role="status" className={review.canContinue ? 'journey-muted' : 'journey-warning'}>{review.reason}</p>
      <p className="project-swarm-commits">Retained commit <code>{review.sourceCommit || 'unavailable'}</code><br />Current project commit <code>{review.targetCommit || 'unavailable'}</code></p>
      <p>{review.files.length} changed {review.files.length === 1 ? 'file' : 'files'} · {review.totalBytes.toLocaleString()} reviewed text bytes</p>
      <section aria-label="Complete retained changes">{review.files.map(file => <details key={file.path}>
        <summary><code>{file.path}</code> · {file.status}</summary>
        <p className="journey-muted">File mode: {file.beforeMode || 'absent'} → {file.afterMode || 'absent'}</p>
        <h4>Before</h4>{file.beforeMode ? <pre className="project-swarm-patch" tabIndex={0} aria-label={`Before ${file.path}`}>{file.before || '(empty file)'}</pre> : <p>File did not exist.</p>}
        <h4>After</h4>{file.afterMode ? <pre className="project-swarm-patch" tabIndex={0} aria-label={`After ${file.path}`}>{file.after || '(empty file)'}</pre> : <p>File will be removed from the new attempt.</p>}
      </details>)}</section>
      {review.canContinue && <label className="setup-check"><input type="checkbox" checked={approved} disabled={busy || !canContinue} onChange={event => setApproved(event.target.checked)} />I reviewed these retained changes to continue in a new attempt. This is not approval to integrate them.</label>}
    </>}
    {error && <p role="alert">{error}</p>}
    <div className="workflow-actions">
      <Button variant="secondary" disabled={busy} onClick={onClose}>Keep inspecting</Button>
      <Button variant="tertiary" disabled={busy || loading} onClick={() => { setApproved(false); setReview(null); setLoading(true); setRefresh(value => value + 1); }}>Refresh retained-change review</Button>
      <Button disabled={busy || loading || !canContinue || !review?.canContinue || !approved} onClick={() => {
        if (!review?.canContinue || !approved || !canContinue || busy) return;
        let key = keys.current.get(review.reviewDigest);
        if (!key) { key = crypto.randomUUID(); keys.current.set(review.reviewDigest, key); }
        setApproved(false); onConfirm(review, key);
      }}>Continue from these reviewed changes</Button>
    </div>
  </WorkflowDialog>;
}
