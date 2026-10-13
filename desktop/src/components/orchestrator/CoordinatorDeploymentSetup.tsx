import { useEffect, useMemo, useRef, useState } from 'react';
import { ApiError } from '../../api/client';
import { errorText, type DeploymentTarget } from '../../api/product';
import { coordinatorDeploymentApi, coordinatorDeploymentInputValid, coordinatorDeploymentPlanValid, type CoordinatorDeploymentInput, type CoordinatorDeploymentPlan } from '../../api/coordinatorDeployment';
import { useProjectDraft } from '../../hooks/useProjectDraft';
import { useUnsavedProjectDraft } from '../../hooks/useUnsavedProjectDraft';
import { Button } from '../fluid/components/button';

export interface CoordinatorDeploymentSelection { plan: CoordinatorDeploymentPlan | null; dirty: boolean; problem: string; draftDigest: string }
interface Props { projectId: string; scope: string; disabled: boolean; enabled: boolean; recordedDraft?: string; onChange(value: CoordinatorDeploymentSelection): void; onNavigateDeploy?(): void }
type Draft = Required<CoordinatorDeploymentInput>;
const initial: Draft = { targetId: '', repository: '', branch: 'main', domain: '', port: 3000, healthPath: '/', dockerfile: 'Dockerfile', source: 'private', privateKeyUuid: '', dataClientFramework: 'none' };
const draftKeys = Object.keys(initial).sort().join(',');
// Browser recovery holds ordinary settings only, including an existing key reference.
// No credentials, metadata fingerprints/revisions, server observations or consent are stored.
function validDraft(value: unknown): value is Draft {
  if (!value || typeof value !== 'object' || Array.isArray(value) || Object.keys(value).sort().join(',') !== draftKeys) return false;
  const row = value as Draft;
  return Object.entries(initial).every(([key, example]) => typeof example === 'number' ? Number.isFinite(row[key as keyof Draft]) : typeof row[key as keyof Draft] === 'string' && (row[key as keyof Draft] as string).length <= 4096) &&
    !/[@?#\r\n\0]/.test(row.repository + row.domain) && /^[A-Za-z0-9_-]{0,128}$/.test(row.privateKeyUuid) &&
    ['public', 'private'].includes(row.source) && ['', 'none', 'vite', 'nextjs'].includes(row.dataClientFramework);
}
export function CoordinatorDeploymentSetup(props: Props) { return <DeploymentSetup key={JSON.stringify([props.scope, props.projectId])} {...props} />; }
function DeploymentSetup({ projectId, scope, disabled, enabled, recordedDraft, onChange, onNavigateDeploy }: Props) {
  const [opened, setOpened] = useState(false), [changed, setChanged] = useState(false), [value, setValue] = useState<Draft>(initial);
  const [targets, setTargets] = useState<DeploymentTarget[]>([]), [targetsRead, setTargetsRead] = useState(false);
  const [prepared, setPrepared] = useState<CoordinatorDeploymentPlan | null>(null), [busy, setBusy] = useState(false), [error, setError] = useState('');
  const [repositoryPush, setRepositoryPush] = useState(false), [publicExposure, setPublicExposure] = useState(false), [deploymentCosts, setDeploymentCosts] = useState(false);
  const digest = JSON.stringify(value), original = useMemo(() => initial, []);
  const draft = useProjectDraft({ scope: JSON.stringify([scope, 'coordinator-deployment']), kind: 'preview-setup', base: projectId, baseValue: original, value, dirty: changed, valid: validDraft });
  const alive = useRef(true), generation = useRef(0), controller = useRef<AbortController | null>(null);
  const latest = useRef({ disabled, enabled, digest }); latest.current = { disabled, enabled, digest };
  function invalidate() { generation.current++; controller.current?.abort(); controller.current = null; setPrepared(null); setRepositoryPush(false); setPublicExposure(false); setDeploymentCosts(false); setBusy(false); }
  function clear() { invalidate(); draft.clear(); setValue(initial); setChanged(false); setError(''); }
  useUnsavedProjectDraft(projectId, changed || !!draft.pending, 'Continued-work deployment', 'develop', { retained: draft.retained || !!draft.pending, discard: clear }, 'coordinator-setup');
  useEffect(() => { alive.current = true; return () => { alive.current = false; generation.current++; controller.current?.abort(); }; }, []);
  useEffect(() => { invalidate(); setTargets([]); setTargetsRead(false); }, [disabled, enabled]);
  useEffect(() => { if (recordedDraft && recordedDraft === digest) clear(); }, [recordedDraft]);
  function edit<K extends keyof Draft>(key: K, next: Draft[K]) {
    invalidate(); setError(''); setChanged(true);
    setValue(current => ({ ...current, [key]: next, ...(key === 'repository' || key === 'targetId' || key === 'source' ? { privateKeyUuid: '' } : {}) }));
  }
  async function read(kind: 'targets' | 'plan') {
    if (latest.current.disabled || !latest.current.enabled || draft.pending) return;
    const selected = targets.find(target => target.id === value.targetId);
    if (kind === 'plan' && (!selected || !coordinatorDeploymentInputValid(value))) return;
    invalidate(); setError(''); const next = new AbortController(); controller.current = next; const ticket = ++generation.current, captured = digest; setBusy(true);
    const current = () => alive.current && ticket === generation.current && !next.signal.aborted && !latest.current.disabled && latest.current.enabled && captured === latest.current.digest;
    try {
      if (kind === 'targets') { const observed = await coordinatorDeploymentApi.targets(next.signal); if (current()) { setTargets(observed); setTargetsRead(true); } }
      else { const observed = await coordinatorDeploymentApi.prepare(projectId, value, next.signal); if (current()) { if (observed.serverUuid !== selected!.serverUuid) throw new Error('The connected server changed. Read the current deployment servers and prepare the plan again.'); setPrepared(observed); } }
    } catch (cause) { if (current()) { setPrepared(null); if (cause instanceof ApiError && [401, 403].includes(cause.status)) { setTargets([]); setTargetsRead(false); setError('Current access to deployment settings was refused.'); } else setError(errorText(cause)); } }
    finally { if (current()) setBusy(false); }
  }
  const candidate = prepared ? { ...prepared, acknowledgeRepositoryPush: repositoryPush, acknowledgePublic: publicExposure, acknowledgeDeploymentCosts: deploymentCosts } : null;
  const problem = draft.pending ? 'Restore or discard the retained deployment setup first.' : draft.error || error || (!changed ? '' : !enabled ? 'Enable deployment to an existing target before delegating this setup.' : !prepared ? 'Read and review the current deployment plan.' : !repositoryPush || !publicExposure || !deploymentCosts ? 'Review and acknowledge repository push, public deployment and deployment costs separately.' : '');
  const plan = !disabled && enabled && !busy && !problem && coordinatorDeploymentPlanValid(candidate) ? candidate : null;
  const projection = JSON.stringify({ plan, dirty: changed || !!draft.pending, problem, draftDigest: digest });
  useEffect(() => { onChange(JSON.parse(projection) as CoordinatorDeploymentSelection); }, [projection, onChange]);
  const locked = disabled || !!draft.pending;
  const target = !disabled ? targets.find(item => item.id === value.targetId) : undefined;
  const consentEnabled = !locked && enabled && !busy && !!prepared && !draft.error && !error;
  return <details onToggle={event => { if (event.currentTarget.open) setOpened(true); }}><summary>Deploy after checks and local preview{draft.pending ? ' · saved draft' : ''}</summary>{opened && <>
    <p>Delegate delivery of independently checked source to an existing connected server after its local preview succeeds. Reading this plan does not push code, create infrastructure or deploy the app.</p>
    {!enabled && <p>Enable deployment to an existing target first. Current Deploy access and public-publishing policy remain required.</p>}
    {draft.pending && <section aria-label="Continued-work deployment draft recovery"><p>Saved deployment settings are available in this browser. Restoring them does not restore any consent or current server, configuration or Data bindings.</p><Button variant="secondary" disabled={disabled || busy} onClick={() => { const restored = draft.take(); if (restored) { invalidate(); setValue(restored.value); setChanged(true); setError(''); } }}>Restore deployment draft</Button><Button variant="tertiary" disabled={disabled || busy} onClick={clear}>Discard browser deployment draft</Button></section>}
    {error && <p role="alert">{error}</p>}{draft.error && <p role="alert">{draft.error}</p>}
    <Button variant="secondary" disabled={locked || !enabled || busy} onClick={() => void read('targets')}>Read connected deployment servers</Button>
    {targetsRead && !disabled && !targets.length && <p>No deployment server connected yet. Configure an existing target and any private repository access in Deploy.</p>}
    {onNavigateDeploy && <Button variant="ghost" onClick={onNavigateDeploy}>Open Deploy</Button>}
    <label className="luci-field">Deployment server<select value={value.targetId} disabled={locked} onChange={event => edit('targetId', event.target.value)}><option value="">Choose your server</option>{value.targetId && !target && <option value={value.targetId}>Saved target · read current servers</option>}{!disabled && targets.map(item => <option key={item.id} value={item.id}>{item.name}</option>)}</select></label>
    <div className="setup-field-grid"><label className="luci-field">Repository URL · HTTPS<input value={value.repository} disabled={locked} onChange={event => edit('repository', event.target.value)} placeholder="https://github.com/you/project" /></label><label className="luci-field">Repository branch<input value={value.branch} disabled={locked} onChange={event => edit('branch', event.target.value)} /></label><label className="luci-field">Application domain<input value={value.domain} disabled={locked} onChange={event => edit('domain', event.target.value)} placeholder="https://app.example.com" /></label></div>
    <details className="deployment-details"><summary>Build and health options</summary><div className="setup-field-grid"><label className="luci-field">Cloud Dockerfile<input value={value.dockerfile} disabled={locked} onChange={event => edit('dockerfile', event.target.value)} /></label><label className="luci-field">Cloud app port<input type="number" min={1} max={65535} value={value.port} disabled={locked} onChange={event => edit('port', Number(event.target.value))} /></label><label className="luci-field">Health check path<input value={value.healthPath} disabled={locked} onChange={event => edit('healthPath', event.target.value)} /></label></div>
      <label className="luci-field">Browser data variables<select value={value.dataClientFramework} disabled={locked} onChange={event => edit('dataClientFramework', event.target.value as Draft['dataClientFramework'])}><option value="none">Runtime only</option><option value="vite">Vite · VITE_SUPABASE_URL and VITE_SUPABASE_ANON_KEY</option><option value="nextjs">Next.js · NEXT_PUBLIC_SUPABASE_URL and NEXT_PUBLIC_SUPABASE_ANON_KEY</option></select><small>Vite and Next.js embed the connected public Data API values in the browser build. Your Dockerfile must declare and use the matching ARG names. Administrator keys stay private.</small></label>
    </details>
    <label className="luci-field">Repository access<select value={value.source} disabled={locked} onChange={event => edit('source', event.target.value as Draft['source'])}><option value="private">Private repository · read-only deploy key</option><option value="public">Public repository</option></select></label>
    {value.source === 'private' && <label className="luci-field">Deployment server key ID<input value={value.privateKeyUuid} disabled={locked} onChange={event => edit('privateKeyUuid', event.target.value)} /><small>Enter an existing key reference prepared in Deploy for this repository and server. Never paste a private key or token here.</small></label>}
    <Button variant="secondary" disabled={locked || !enabled || busy || !target || !coordinatorDeploymentInputValid(value)} onClick={() => void read('plan')}>Read current deployment plan</Button>
    {!disabled && prepared && <section aria-label="Review current deployment plan" className="setup-plan"><h4>Current deployment plan</h4><p>{target?.name} · server {prepared.serverUuid}</p><p>Repository push: {prepared.repository} · branch {prepared.branch}</p><p>Public application: {prepared.domain} · port {prepared.port} · health {prepared.healthPath}</p><p>Dockerfile {prepared.dockerfile} · {prepared.source} repository · Data browser variables {prepared.dataClientFramework || 'none'}</p><p>Runtime configuration revision {prepared.expectedConfigurationRevision}. Values remain private.</p><p>{prepared.expectedDataDomainRevision ? `Data domain revision ${prepared.expectedDataDomainRevision}. Current Data binding captured for review.` : 'No Data binding is configured for this deployment.'}</p><p>Changes to the target, runtime configuration or Data binding invalidate this delegation. No new server is authorized.</p></section>}
    <label className="setup-check"><input type="checkbox" checked={repositoryPush} disabled={!consentEnabled} onChange={event => { setRepositoryPush(event.target.checked); setChanged(true); }} />I authorize pushing the reviewed source to this exact repository and branch.</label>
    <label className="setup-check"><input type="checkbox" checked={publicExposure} disabled={!consentEnabled} onChange={event => { setPublicExposure(event.target.checked); setChanged(true); }} />I authorize public deployment of this application at the reviewed HTTPS domain.</label>
    <label className="setup-check"><input type="checkbox" checked={deploymentCosts} disabled={!consentEnabled} onChange={event => { setDeploymentCosts(event.target.checked); setChanged(true); }} />I accept ongoing deployment and provider costs on this existing target.</label>
    {problem && <p role="status">{problem}</p>}{draft.retained && <p role="status">Deployment settings are retained in this browser; approvals and current bindings are not.</p>}
    {(changed || error) && <Button variant="tertiary" disabled={disabled || busy} onClick={clear}>Discard continued-work deployment setup</Button>}
  </>}</details>;
}
