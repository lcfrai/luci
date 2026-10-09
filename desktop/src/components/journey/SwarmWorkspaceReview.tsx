import { useEffect, useRef, useState } from 'react';
import { errorText, productApi } from '../../api/product';
import type { RunOperation } from '../../api/projectRuns';
import { swarmWorkspaceApi, type SwarmWorkspace, type SwarmWorkspaceFile } from '../../api/swarmWorkspace';
import { useProjectAccess } from '../../hooks/useProjectAccess';
import { useProjectDraftScope } from '../../hooks/useProjectDraft';
import { WorkflowDialog } from '../common/WorkflowDialog';
import { Button } from '../fluid/components/button';
import './swarm-workspace-review.css';

interface Props { projectId: string; runId: string; operation: RunOperation; canRead: boolean; onClose(): void }
type ReadResult<T> = {key: string; value?: T; error?: string};
const changeLabel = (status: string) => status === 'saved' ? 'Saved file' : status === 'changed' ? 'Changed' : status === '??' ? 'Untracked' : status.includes('D') ? 'Deleted' : status.includes('R') ? 'Renamed' : status.includes('A') ? 'Added' : status.includes('U') ? 'Unmerged' : status.includes('M') ? 'Modified' : 'Change status unavailable';

/** The parent mounts this only for an explicit inspection, including attempts without a session. */
export function SwarmWorkspaceReview(props: Props) {
  return <WorkspaceReview key={JSON.stringify([props.projectId, props.runId, props.operation.id, props.operation.taskId, props.operation.worktreeId])} {...props} />;
}
function WorkspaceReview({projectId, runId, operation, canRead: allowed, onClose}: Props) {
  const scope = useProjectDraftScope(projectId), access = useProjectAccess(projectId);
  const [openedIn] = useState(() => JSON.stringify([productApi.baseUrl, scope]));
  const sameScope = openedIn === JSON.stringify([productApi.baseUrl, scope]);
  const canRead = allowed && access.canRead && sameScope && !!scope;
  const canRequest = canRead && !access.loading && !access.error && !!operation.worktreeId && !!operation.taskId;
  const [refresh, setRefresh] = useState(0), [selected, setSelected] = useState(''), [fileRetry, setFileRetry] = useState(0);
  const [workspace, setWorkspace] = useState<ReadResult<SwarmWorkspace> | null>(null), [file, setFile] = useState<ReadResult<SwarmWorkspaceFile> | null>(null);
  const listKey = JSON.stringify([openedIn, projectId, runId, operation.id, operation.worktreeId, refresh]);
  const fileKey = JSON.stringify([listKey, selected, fileRetry]);
  const permission = useRef(canRead), generation = useRef(0);
  if (permission.current !== canRead) { permission.current = canRead; generation.current++; }
  const latest = useRef({canRead, listKey, fileKey}); latest.current = {canRead, listKey, fileKey};
  const listAttempt = useRef<{key: string; done: boolean} | null>(null), fileAttempt = useRef<{key: string; done: boolean} | null>(null);
  useEffect(() => () => { generation.current++; }, []);
  useEffect(() => {
    if (!canRead) { setWorkspace(null); setFile(null); setSelected(''); listAttempt.current = null; fileAttempt.current = null; return; }
    if (!canRequest || listAttempt.current?.key === listKey) return;
    const attempt = {key: listKey, done: false}; listAttempt.current = attempt;
    const epoch = generation.current, controller = new AbortController();
    const current = () => !controller.signal.aborted && epoch === generation.current && latest.current.canRead && latest.current.listKey === listKey;
    setWorkspace(null); setFile(null);
    void swarmWorkspaceApi.inspect(projectId, runId, operation, controller.signal).then(value => { if (current()) { attempt.done = true; setWorkspace({key: listKey, value}); } }).catch(cause => { if (current()) { attempt.done = true; setWorkspace({key: listKey, error: errorText(cause)}); } });
    return () => { controller.abort(); if (listAttempt.current === attempt && !attempt.done) listAttempt.current = null; };
  }, [canRead, canRequest, listKey]);
  useEffect(() => {
    if (!canRequest || !selected || workspace?.key !== listKey || !workspace.value?.files.some(item => item.path === selected) || fileAttempt.current?.key === fileKey) return;
    const attempt = {key: fileKey, done: false}; fileAttempt.current = attempt;
    const epoch = generation.current, controller = new AbortController();
    const current = () => !controller.signal.aborted && epoch === generation.current && latest.current.canRead && latest.current.fileKey === fileKey;
    setFile(null);
    void swarmWorkspaceApi.file(projectId, runId, operation, selected, controller.signal).then(value => { if (current()) { attempt.done = true; setFile({key: fileKey, value}); } }).catch(cause => { if (current()) { attempt.done = true; setFile({key: fileKey, error: errorText(cause)}); } });
    return () => { controller.abort(); if (fileAttempt.current === attempt && !attempt.done) fileAttempt.current = null; };
  }, [canRequest, selected, workspace, listKey, fileKey]);
  const current = canRead && workspace?.key === listKey ? workspace : null;
  const preview = canRead && file?.key === fileKey ? file : null;
  const refreshList = () => { if (!canRequest) return; setSelected(''); setFile(null); setRefresh(value => value + 1); };
  return <div onPointerDown={event => event.stopPropagation()}><WorkflowDialog unavailableMessage={!sameScope ? 'The account or workspace changed. Close this view and reopen the task in its workspace.' : 'Reading access is unavailable. Close this view or wait for access to return.'} readAccess={{...access, canRead}} title="Retained task workspace" description="Inspect the files left by this task attempt, including a failed or interrupted start. Reading does not run an agent, change files or integrate its work." onClose={onClose}>
    <div className="swarm-workspace-review">
      {!canRead ? <p role="status">{!sameScope ? 'The account or workspace changed. Close this view and reopen the task in its workspace.' : 'Reading access is unavailable. Close this view or wait for access to return.'}</p> : <>
        <p className="journey-muted">Attempt {operation.attempt}. This is a current read of its retained files, not an immutable review or proof of completion. Files may change while an agent is active.</p>
        {!canRequest && <p role="status">{!operation.worktreeId || !operation.taskId ? 'This attempt has no recorded task workspace to inspect.' : access.message}</p>}
        <Button variant="secondary" disabled={!canRequest || !current} onClick={refreshList}>Refresh retained files</Button>
        {canRequest && !current && <p role="status">Reading retained workspace…</p>}
        {current?.error && <p role="alert" className="journey-error">{current.error}</p>}
        {current?.value && <>
          {!current.value.exists ? <p role="status">{current.value.reason || 'The retained workspace is no longer available. Its task record has been kept.'}</p> : <>
            {current.value.reason && <p className="journey-muted">{current.value.reason}</p>}
            <section aria-label="Retained files"><h3>Retained files</h3>
              {current.value.filesTruncated && <p className="journey-warning">The file list is incomplete.{current.value.filesOmitted > 0 ? ` At least ${current.value.filesOmitted} additional files are not listed.` : ''} Review the retained checkout before making integration decisions.</p>}
              {current.value.files.length ? <label className="luci-field"><span>File to inspect</span><select disabled={!canRequest} value={selected} onChange={event => { setSelected(event.target.value); setFileRetry(value => value + 1); setFile(null); }}><option value="">Choose a retained file</option>{current.value.files.map(item => <option value={item.path} key={item.path}>{item.path} · {changeLabel(item.status)}</option>)}</select></label> : <p>No eligible retained files are listed. This does not establish that the task completed.</p>}
            </section>
            {selected && <section aria-label="Retained file preview"><h3>{selected}</h3>
              {!preview && <p role="status">Reading selected file…</p>}
              {preview?.error && <div role="alert"><p className="journey-error">{preview.error}</p><Button variant="secondary" disabled={!canRequest} onClick={() => setFileRetry(value => value + 1)}>Retry file read</Button></div>}
              {preview?.value && <><p className="journey-muted">{preview.value.bytes.toLocaleString()} bytes · current retained file</p>{preview.value.binary ? <p role="status">Binary file. A text preview is unavailable.</p> : <>{preview.value.omitted && <p role="status" className="journey-warning">Partial file preview. Up to the first 128 KiB of text is shown; this is not the complete file.</p>}{preview.value.content ? <pre tabIndex={0} aria-label={preview.value.omitted ? 'Partial retained file text' : 'Retained file text'}>{preview.value.content}</pre> : !preview.value.omitted && <p>This file is empty.</p>}</>}{preview.value.reason && <p className="journey-muted">{preview.value.reason}</p>}</>}
            </section>}
            <details><summary>Tracked changes preview</summary><p className="journey-muted">Only eligible tracked files appear here. Inspect untracked files separately above. This preview cannot authorize integration.</p>{current.value.patchTruncated && <p className="journey-warning">The changes preview is truncated. Review the full retained checkout before making an integration decision.</p>}{current.value.patch ? <pre tabIndex={0} aria-label="Retained tracked changes">{current.value.patch}</pre> : <p>No eligible tracked changes are included in this preview.</p>}</details>
          </>}
        </>}
      </>}
      <div className="workflow-actions"><Button variant="secondary" onClick={onClose}>Close retained workspace</Button></div>
    </div>
  </WorkflowDialog></div>;
}
