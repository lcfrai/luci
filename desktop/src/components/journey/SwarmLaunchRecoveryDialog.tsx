import { useEffect, useState } from 'react';
import { errorText } from '../../api/product';
import type { ProjectRun, RunOperation } from '../../api/projectRuns';
import { projectSwarmApi, type SwarmRecoveryAvailability } from '../../api/projectSwarm';
import { Button } from '../fluid/components/button';
import { WorkflowDialog } from '../common/WorkflowDialog';

export function launchRecoveryStatus(operation: RunOperation) {
  switch (operation.recovery?.state) {
    case 'pending': return 'Recovery requested. The old launch is not yet proved blocked; capacity remains reserved.';
    case 'fenced': return 'The old launch is proved blocked. Finishing the saved recovery must still release its capacity.';
    case 'finalized': return 'Recovery finished and capacity released. Output is retained; no replacement agent was started.';
    default: return '';
  }
}

/** Mounted for one exact run revision/attempt and authority epoch by ProjectSwarm. */
export function SwarmLaunchRecoveryDialog({run, operation, taskTitle, projectName, canEdit, busy, error, onClose, onConfirm}: {
  run: ProjectRun; operation: RunOperation; taskTitle: string; projectName: string;
  canEdit: boolean; busy: boolean; error: string; onClose(): void;
  onConfirm(availability: SwarmRecoveryAvailability): void;
}) {
  const [availability, setAvailability] = useState<SwarmRecoveryAvailability | null>(null);
  const [loading, setLoading] = useState(true), [readError, setReadError] = useState('');
  const [refresh, setRefresh] = useState(0), [approved, setApproved] = useState(false);
  useEffect(() => {
    const controller = new AbortController(); let current = true;
    setAvailability(null); setReadError(''); setLoading(true); setApproved(false);
    void projectSwarmApi.recovery(run.projectId, run, operation, controller.signal).then(value => {
      if (current) setAvailability(value);
    }).catch(cause => { if (current) setReadError(errorText(cause)); })
      .finally(() => { if (current) setLoading(false); });
    return () => { current = false; controller.abort(); };
  }, [run.projectId, run.id, run.revision, operation.id, operation.childId, refresh]);
  const continuing = !!operation.recovery;
  const supported = availability?.availability.supported === true;
  return <WorkflowDialog title={continuing ? 'Continue launch recovery' : 'Stop uncertain launch and retain output'} onClose={onClose} busy={busy}
    description="Recover this exact uncertain task launch. Luci may stop its original worker and will keep the workspace, context and output. This action never starts a replacement agent.">
    <dl className="swarm-recovery-scope">
      <div><dt>Project</dt><dd>{projectName}</dd></div>
      <div><dt>Task</dt><dd>{taskTitle}</dd></div>
      <div><dt>Build</dt><dd><code>{run.id}</code> · revision {run.revision}</dd></div>
      <div><dt>Attempt</dt><dd>{operation.attempt} · <code>{operation.id}</code></dd></div>
    </dl>
    {operation.recovery && <p role="status">{launchRecoveryStatus(operation)}</p>}
    <p>A small permanent recovery marker remains in Docker to prevent a delayed launch. It is not a running agent. Removing it would remove that protection; Luci does not automatically delete recovery markers.</p>
    <p>Capacity stays reserved until recovery is proved and finalized. Existing output still needs review; stopping a launch does not establish that the task succeeded. A fresh attempt requires a separate start.</p>
    {loading ? <p role="status">Checking whether this exact attempt supports recovery…</p> : availability && <p role="status" className={supported ? 'journey-muted' : 'journey-warning'}>{availability.availability.message || (supported ? 'This recorded attempt supports explicit launch recovery.' : 'This recorded attempt does not support launch recovery. Keep its output and reservation for inspection.')}</p>}
    {readError && <p role="alert">{readError}</p>}
    {!loading && !supported && <p>Nothing is stopped by this check. An older launch or unsupported backend cannot be made recoverable by retrying or removing its reservation. A host operator must inspect the reported blocker.</p>}
    {supported && <label className="setup-check"><input type="checkbox" checked={approved} disabled={busy || !canEdit} onChange={event => setApproved(event.target.checked)} />I authorize recovery of this exact attempt, including stopping its original worker if needed and keeping a permanent Docker recovery marker.</label>}
    {error && <p role="alert">{error}</p>}
    <div className="workflow-actions">
      <Button variant="secondary" disabled={busy} onClick={onClose}>Keep inspecting</Button>
      <Button variant="tertiary" disabled={busy || loading} onClick={() => { setApproved(false); setAvailability(null); setLoading(true); setRefresh(value => value + 1); }}>Refresh recovery availability</Button>
      <Button disabled={busy || loading || !canEdit || !supported || !approved} onClick={() => {
        if (!availability || !approved || !canEdit) return;
        setApproved(false); onConfirm(availability);
      }}>{continuing ? 'Continue reviewed recovery' : 'Stop uncertain launch and retain output'}</Button>
    </div>
  </WorkflowDialog>;
}
