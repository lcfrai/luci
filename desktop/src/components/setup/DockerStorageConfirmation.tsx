import { useEffect, useId, useLayoutEffect, useRef, useState } from 'react';
import { type ExecutionStorage, type ExecutionStorageClient } from '../../api/executionStorage';
import { ApiError } from '../../api/client';
import { errorText } from '../../api/product';
import { useAuth } from '../auth/AuthProvider';
import { useTeamWorkspace } from '../teams/TeamWorkspaceProvider';
import { Button } from '../fluid/components/button';
import { Input } from '../ui/input';

const observedTime = (value?: string) => value && Number.isFinite(Date.parse(value)) && Date.parse(value) > 0 ? new Date(value).toLocaleString() : '';
const bytes = (value?: number) => value === undefined ? 'Unavailable' : `${(value / 1024 ** 3).toFixed(1)} GiB`;
const labels: Record<ExecutionStorage['state'], string> = {
  'not-required': 'Not required', 'confirmation-required': 'Confirmation needed',
  'pending-observation': 'Measuring storage', confirmed: 'Location confirmed',
  invalidated: 'Confirm the changed location', unavailable: 'Storage unavailable',
};

/** A changed storage receipt invalidates advice; it must not start a readiness probe. */
type StorageProps = { client: ExecutionStorageClient; enabled: boolean; onChanged?(): void };

/** A fresh child owns every authenticated scope, so old paths never paint between identities. */
export function DockerStorageConfirmation(props: StorageProps) {
  const auth = useAuth(), team = useTeamWorkspace();
  const clientScope = useRef({ client: props.client, revision: 0 });
  if (clientScope.current.client !== props.client) clientScope.current = { client: props.client, revision: clientScope.current.revision + 1 };
  if (!props.enabled) return null;
  if (team?.teamId || auth?.profile.user.role !== 'super-admin' || !auth.profile.user.id) return <section className="setup-plan"><h2>Docker storage capacity</h2><p>A Super Admin on this computer confirms Docker Desktop’s storage location. You can keep planning while new agent and build starts wait for the required storage checks.</p></section>;
  return <StorageReview key={JSON.stringify([auth.profile.user.id, clientScope.current.revision])} {...props} />;
}

/** Privileged host paths never enter project views or a selected team workspace. */
function StorageReview({ client, enabled, onChanged }: {
  client: ExecutionStorageClient; enabled: boolean; onChanged?(): void;
}) {
  const auth = useAuth(), team = useTeamWorkspace();
  const allowed = enabled && !team?.teamId && auth?.profile.user.role === 'super-admin';
  const identity = `${allowed}:${auth?.profile.user.id ?? ''}:${team?.teamId ?? ''}`;
  const authority = useRef({ identity, client, epoch: 0 });
  if (authority.current.identity !== identity || authority.current.client !== client) authority.current = { identity, client, epoch: authority.current.epoch + 1 };
  const live = useRef(false), working = useRef(false), recorded = useRef<ExecutionStorage | null>(null);
  const [snapshot, setSnapshot] = useState<ExecutionStorage | null>(null);
  const [path, setPath] = useState(''), [acknowledged, setAcknowledged] = useState(false);
  const [review, setReview] = useState<{ snapshot: ExecutionStorage; path: string } | null>(null);
  const [editing, setEditing] = useState(false);
  const [busy, setBusy] = useState(''), [error, setError] = useState('');
  const [unconfirmed, setUnconfirmed] = useState(false), [serverDenied, setServerDenied] = useState(false);
  const titleId = useId(), helpId = useId();
  const title = useRef<HTMLHeadingElement>(null), reviewTitle = useRef<HTMLHeadingElement>(null), pathInput = useRef<HTMLInputElement>(null), previousReview = useRef(false);
  useEffect(() => {
    if (review) reviewTitle.current?.focus();
    else if (previousReview.current) (pathInput.current ?? title.current)?.focus();
    previousReview.current = !!review;
  }, [review]);
  useEffect(() => { if (editing) pathInput.current?.focus(); }, [editing]);
  const changed = useRef(onChanged); changed.current = onChanged;
  const usable = (epoch: number) => live.current && authority.current.epoch === epoch && authority.current.identity.startsWith('true:');
  function accept(value: ExecutionStorage, confirmed = false) {
    const previous = recorded.current;
    const changedReadiness = confirmed || !!previous && (previous.revision !== value.revision || previous.state !== value.state);
    if (changedReadiness) {
      setReview(null); setAcknowledged(false);
    }
    recorded.current = value; setSnapshot(value); setServerDenied(false);
    if (changedReadiness) changed.current?.();
  }
  function reject(cause: unknown) {
    if (cause instanceof ApiError && [401, 403].includes(cause.status)) {
      recorded.current = null; setSnapshot(null); setPath(''); setReview(null); setAcknowledged(false); setServerDenied(true);
    }
    setError(errorText(cause));
  }
  useLayoutEffect(() => {
    live.current = true; working.current = false; recorded.current = null;
    setSnapshot(null); setPath(''); setAcknowledged(false); setReview(null); setEditing(false); setError(''); setUnconfirmed(false); setServerDenied(false); setBusy('');
    if (!allowed) return () => { live.current = false; authority.current.epoch++; };
    const epoch = authority.current.epoch, controller = new AbortController();
    setBusy('Reading Docker storage');
    void client.get(controller.signal).then(value => { if (!controller.signal.aborted && usable(epoch)) accept(value); })
      .catch(cause => { if (!controller.signal.aborted && usable(epoch)) reject(cause); })
      .finally(() => { if (usable(epoch)) setBusy(''); });
    return () => { live.current = false; authority.current.epoch++; controller.abort(); };
  }, [allowed, identity, client]);
  useEffect(() => {
    if (!allowed || !snapshot || !['pending-observation', 'confirmed'].includes(snapshot.state) || busy || unconfirmed || serverDenied) return;
    const epoch = authority.current.epoch, controller = new AbortController();
    let timer: ReturnType<typeof setTimeout>;
    let active = false;
    const load = async () => {
      if (active || document.visibilityState !== 'visible' || working.current || controller.signal.aborted) return;
      active = true;
      try {
        const value = await client.get(controller.signal);
        if (controller.signal.aborted || !usable(epoch)) return;
        accept(value);
      } catch (cause) { if (!controller.signal.aborted && usable(epoch)) reject(cause); }
      finally { active = false; }
    };
    const visible = () => { if (document.visibilityState === 'visible') { clearTimeout(timer); void load(); } };
    timer = setTimeout(() => void load(), snapshot.state === 'pending-observation' ? 3000 : 15_000);
    document.addEventListener('visibilitychange', visible);
    return () => { clearTimeout(timer); controller.abort(); document.removeEventListener('visibilitychange', visible); };
  }, [allowed, client, snapshot, busy, unconfirmed, serverDenied]);
  async function refresh() {
    if (working.current || !allowed) return;
    const epoch = authority.current.epoch;
    working.current = true; setBusy('Refreshing Docker storage'); setReview(null); setAcknowledged(false); setError('');
    try {
      const value = await client.get();
      if (usable(epoch)) { accept(value); setUnconfirmed(false); }
    } catch (cause) { if (usable(epoch)) { recorded.current = null; setSnapshot(null); reject(cause); } }
    finally { if (usable(epoch)) { working.current = false; setBusy(''); } }
  }
  async function confirm() {
    if (working.current || !allowed || !review || unconfirmed || !acknowledged) return;
    const epoch = authority.current.epoch;
    working.current = true; setBusy('Confirming Docker storage'); setError('');
    try {
      const value = await client.confirm(review.snapshot, review.path);
      if (usable(epoch)) { accept(value, true); setReview(null); setAcknowledged(false); setEditing(false); }
    } catch (cause) {
      if (usable(epoch)) {
        setUnconfirmed(true); setAcknowledged(false); reject(cause);
        if (!(cause instanceof ApiError && [401, 403].includes(cause.status))) setError(`${errorText(cause)} This confirmation could not be established. Refresh the saved status before trying again; the service may already have accepted it.`);
      }
    } finally { if (usable(epoch)) { working.current = false; setBusy(''); } }
  }
  if (!enabled) return null;
  if (!allowed) return <section className="setup-plan"><h2>Docker storage capacity</h2><p>A Super Admin on this computer confirms Docker Desktop’s storage location. You can keep planning while new agent and build starts wait for the required storage checks.</p></section>;
  const canReview = !serverDenied && snapshot?.required && snapshot.scope === 'local-docker-desktop' && snapshot.state !== 'pending-observation' && (snapshot.state !== 'confirmed' || editing);
  return <section className="setup-plan docker-storage" aria-labelledby={titleId}>
    <h2 id={titleId} ref={title} tabIndex={-1}>Docker storage capacity</h2>
    <p>Docker’s Linux disk and your physical drives have separate space limits. Luci needs both before starting new heavy work. Existing workers keep running.</p>
    {busy && <p role="status">{busy}…</p>}
    {error && <p className="journey-error" role="alert">{error}</p>}
    {snapshot && <>
      <p role="status"><strong>{labels[snapshot.state]}</strong> · {snapshot.message}</p>
      <dl className="docker-storage-capacity"><div><dt>Free inside Docker’s Linux disk</dt><dd>{bytes(snapshot.guestAvailableBytes)}</dd></div><div><dt>Lowest free space across mounted local drives</dt><dd>{bytes(snapshot.hostAvailableBytes)}</dd></div></dl>
      <p className="journey-muted">The physical-drive figure is a conservative minimum across all measured mounted local drives; it does not identify free space on only the VHD’s drive.{snapshot.mountedVolumeCount !== undefined && ` ${snapshot.mountedVolumeCount} mounted local volumes measured.`}</p>
      {observedTime(snapshot.observedAt) && <p className="journey-muted">Observed {observedTime(snapshot.observedAt)}.</p>}
      {snapshot.limitingVolume && <p>Lowest-space mounted volume: <span className="docker-storage-path">{snapshot.limitingVolume}</span></p>}
      {snapshot.desktopVersion && <p>Docker Desktop version: {snapshot.desktopVersion}</p>}
      {snapshot.vhdPath && <details><summary>Saved operator-confirmed location</summary><p className="docker-storage-path">{snapshot.vhdPath}</p>{observedTime(snapshot.confirmedAt) && <p>Confirmed {observedTime(snapshot.confirmedAt)}.</p>}</details>}
      {snapshot.state === 'pending-observation' && <p>Waiting for a fresh background measurement. Confirmation alone does not establish enough free space.</p>}
    </>}
    {snapshot?.state === 'confirmed' && !editing && <Button type="button" variant="secondary" disabled={!!busy} onClick={() => setEditing(true)}>Review or change storage location</Button>}
    {canReview && <form onSubmit={event => { event.preventDefault(); if (!busy && !unconfirmed && acknowledged && path.trim()) setReview({ snapshot, path: path.trim() }); }}>
      <p id={helpId}>In Docker Desktop, check Settings → Resources → Advanced for the disk image location. Enter the full path to the active VHD file. Luci will validate the file and current Docker settings; it will not move data or change Docker.</p>
      <label className="luci-field">Active Docker VHD file<Input ref={pathInput} value={path} required disabled={!!busy || !!review} aria-describedby={helpId} placeholder="Full Windows path to the active .vhdx file" autoComplete="off" spellCheck={false} onChange={event => { setPath(event.target.value); setAcknowledged(false); setReview(null); }} /></label>
      <label className="luci-check"><input type="checkbox" checked={acknowledged} disabled={!!busy || !!review || unconfirmed} onChange={event => setAcknowledged(event.target.checked)} />I checked that this is Docker Desktop’s active VHD file and that its active and any pending disk locations are on mounted local drives.</label>
      {review ? <div className="setup-check" role="group" aria-label="Review Docker storage confirmation">
        <h3 ref={reviewTitle} tabIndex={-1}>Confirm this storage layout</h3><p className="docker-storage-path">{review.path}</p>
        <p>Luci records your confirmation and rechecks the current Docker engine, version, file identity and settings. A change requires confirmation again. Previously authorized queued work may start when every capacity check passes.</p>
        <div className="setup-actions"><Button type="button" variant="secondary" disabled={!!busy} onClick={() => { setReview(null); setAcknowledged(false); }}>Change location</Button><Button type="button" loading={busy === 'Confirming Docker storage'} disabled={!!busy || unconfirmed || !acknowledged} onClick={() => void confirm()}>Confirm Docker storage location</Button></div>
      </div> : <Button type="submit" variant="secondary" disabled={!!busy || unconfirmed || !acknowledged || !path.trim()}>Review storage location</Button>}
    </form>}
    <Button type="button" variant="tertiary" disabled={!!busy} onClick={() => void refresh()}>Refresh Docker storage status</Button>
  </section>;
}
