import {useEffect, useRef, useState} from 'react';
import {ApiError} from '../../api/client';
import {productApi, errorText, type CheckRun} from '../../api/product';
import {checkCanRecover, type CheckRecoveryReview} from '../../api/projectCheckContracts';
import {Button} from '../fluid/components/button';
import {WorkflowDialog, type WorkflowDialogReadAccess} from '../common/WorkflowDialog';
import './check-recovery.css';

interface Review {run: CheckRun; recovery: CheckRecoveryReview; owner: number}
export function CheckRecoveryDialog({run, projectName, readAccess, canRead, canEdit, onUpdate, onClose, onReadDenied, getCloseFocusTarget}: {
  readAccess: WorkflowDialogReadAccess; run: CheckRun; projectName: string; canRead: boolean; canEdit: boolean;
  onUpdate(value: CheckRun): void; onClose(): void; onReadDenied?(): void;
  getCloseFocusTarget(openingControl: HTMLElement | null): HTMLElement | null;
}) {
  const [review, setReview] = useState<Review | null>(null), [approved, setApproved] = useState(false);
  const [pending, setPending] = useState(''), [error, setError] = useState(''), [notice, setNotice] = useState('');
  const live = useRef(true), working = useRef(false), reading = useRef<AbortController | null>(null);
  const epoch = useRef(0), boundary = `${run.projectId}:${run.id}:${canRead}:${canEdit}`, previous = useRef(boundary);
  if (previous.current !== boundary) { previous.current = boundary; epoch.current++; }
  const current = useRef({run, canRead, canEdit, onUpdate, onReadDenied}); current.current = {run, canRead, canEdit, onUpdate, onReadDenied};
  const usable = (owner: number) => live.current && epoch.current === owner && current.current.canRead;
  const value = review?.owner === epoch.current ? review : null;
  const fresh = !!value && value.run.revision === run.revision && value.recovery.revision === run.revision;
  const supported = !!value && checkCanRecover(value.recovery);
  const commandIndex = value?.recovery.commandIndex;
  const command = value && commandIndex !== null && commandIndex !== undefined ? value.run.commands[commandIndex] : undefined;
  async function read(owner = epoch.current) {
    reading.current?.abort(); const controller = new AbortController(); reading.current = controller;
    setApproved(false); setReview(null);
    try {
      const saved = await productApi.checkRun(run.projectId, run.id, controller.signal);
      if (!usable(owner) || controller.signal.aborted) return;
      const availability = await productApi.checkRecovery(run.projectId, run.id, controller.signal);
      if (!usable(owner) || controller.signal.aborted) return;
      if (availability.revision !== saved.revision || (saved.revision ?? 0) < (current.current.run.revision ?? 0) ||
        availability.commandIndex !== null && !saved.commands[availability.commandIndex]) throw new Error('This check changed during review. Refresh its recovery before continuing.');
      current.current.onUpdate(saved); setReview({run: saved, recovery: availability, owner});
    } catch (cause) {
      if (usable(owner) && !controller.signal.aborted) {
        setReview(null); setError(errorText(cause));
        if (cause instanceof ApiError && [401, 403].includes(cause.status)) current.current.onReadDenied?.();
      }
    } finally { if (reading.current === controller) reading.current = null; }
  }
  async function refresh() {
    if (!canRead || working.current) return;
    const owner = epoch.current; setError(''); setNotice(''); setPending('Reading recovery');
    try { await read(owner); } finally { if (usable(owner)) setPending(''); }
  }
  useEffect(() => {
    live.current = true; if (canRead) void refresh();
    return () => { live.current = false; epoch.current++; reading.current?.abort(); };
  }, [run.projectId, run.id, canRead, canEdit]);
  useEffect(() => { setApproved(false); }, [run.revision, canEdit]);
  async function recover() {
    if (!value || !fresh || !supported || !approved || !canEdit || working.current) return;
    const owner = epoch.current, selected = value.recovery;
    working.current = true; reading.current?.abort(); setApproved(false); setError(''); setNotice(''); setPending('Recovering this check');
    try {
      const saved = await productApi.recoverCheck(run.projectId, run.id, selected);
      if (!usable(owner) || !current.current.canEdit) return;
      current.current.onUpdate(saved);
      setNotice(saved.resourcesHeld === false ? 'Recovery is recorded and this check no longer holds capacity. Output is retained; no command was restarted.' : 'Recovery was recorded. Capacity release is not yet confirmed; inspect the current recovery state before continuing.');
      await read(owner);
    } catch (cause) {
      if (usable(owner)) {
        setError(errorText(cause));
        if (cause instanceof ApiError && (cause.status === 401 || cause.status === 403)) setReview(null);
        else {
          setNotice('The recovery result could not be confirmed. The current record is shown below when available. No recovery request or command was repeated.');
          await read(owner);
        }
      }
    } finally { working.current = false; if (live.current) setPending(''); }
  }
  const continuing = value?.recovery.state === 'pending';
  const mutating = pending === 'Recovering this check';
  return <WorkflowDialog readAccess={readAccess} title="Review check recovery" busy={mutating} onClose={onClose} getCloseFocusTarget={getCloseFocusTarget}
    description="Inspect this exact check allocation. Recovery may stop its original command and retains recorded output. It never reruns a command or starts a replacement.">
    <div className="check-recovery" role={mutating ? 'region' : undefined} aria-label={mutating ? 'Check recovery details' : undefined} tabIndex={mutating ? 0 : undefined}>
    {!canRead ? <p role="status">Reading access is unavailable. Close this view and refresh project access.</p> : <>
      <p><strong>{projectName}</strong> · check <code>{run.id}</code></p>
      {pending && <p role="status">{pending}…</p>}
      {value && <>
        <p>Revision {value.recovery.revision} · source <code>{value.run.commit || 'No commit recorded'}</code>{value.run.dirty && ' · includes uncommitted changes'}</p>
        {command && <p>Command {value.recovery.commandIndex! + 1}: <strong>{command.label}</strong><br /><code>{command.executable} {command.args.map(arg => JSON.stringify(arg)).join(' ')}</code></p>}
        <p className={supported ? 'journey-muted' : 'journey-warning'} role="status">{value.recovery.message}</p>
        <p>{value.run.resourcesHeld === false ? 'This check is recorded as holding no capacity.' : value.run.resourcesHeld === true ? 'Capacity remains held until recovery and cleanup are confirmed.' : 'Capacity ownership is unavailable. No release is assumed.'}</p>
        {value.recovery.method === 'fence-launch' && <p>A small permanent Docker recovery marker prevents the delayed original launch. Removing it would remove that protection. Luci does not automatically delete these markers.</p>}
        {value.recovery.method === 'remove-exact' && <p>Luci will inspect and, if necessary, stop and remove only the exact recorded container on its original Docker engine.</p>}
        {value.recovery.state === 'complete' && <p>Recovery is complete. Review the retained output separately; this does not establish that the checks passed.</p>}
        {!fresh && <p className="journey-warning">The recorded check changed. Refresh this recovery and review its current command before consenting.</p>}
        {supported && <label className="setup-check"><input type="checkbox" checked={approved && fresh} disabled={!canEdit || !fresh || !!pending} onChange={event => setApproved(event.target.checked)} />I authorize recovery of this exact check command, including stopping its original allocation{value.recovery.method === 'fence-launch' ? ' and retaining a permanent Docker recovery marker' : ''}.</label>}
      </>}
      {!canEdit && <p className="journey-muted">Editing access is required to stop or recover checks. Execution access is not required for these safe-stop actions.</p>}
      {error && <p role="alert">{error}</p>}{notice && <p role="status">{notice}</p>}
    </>}
    <div className="workflow-actions"><Button variant="secondary" disabled={mutating} onClick={onClose}>Keep inspecting</Button><Button variant="tertiary" disabled={!!pending || !canRead} onClick={() => void refresh()}>Refresh check recovery</Button><Button disabled={!canEdit || !!pending || !fresh || !supported || !approved} onClick={() => void recover()}>{continuing ? 'Finish reviewed recovery' : 'Recover this exact check'}</Button></div>
    </div>
  </WorkflowDialog>;
}
