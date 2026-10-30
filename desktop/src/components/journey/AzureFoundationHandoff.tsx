import { useEffect, useRef, useState, type FormEvent } from 'react';
import { productApi, errorText, type CloudPlan, type CloudPlanRequest } from '../../api/product';
import type { DeploymentBlueprintPlan } from '../../api/deploymentBlueprint';
import { useProjectAccess } from '../../hooks/useProjectAccess';
import { useUnsavedProjectDraft } from '../../hooks/useUnsavedProjectDraft';
import { CloudManager } from '../setup/CloudManager';
import { Button } from '../fluid/components/button';

export function AzureFoundationHandoff({ plan, active }: { plan: DeploymentBlueprintPlan; active: boolean }) {
 const access = useProjectAccess(plan.blueprint.projectId);
 const allowed = active && access.canRead && access.canEdit && !access.loading && !access.error;
 const [request, setRequest] = useState<Omit<CloudPlanRequest, 'foundation'>>({ provider: 'azure', name: '', region: '', domain: '', size: 'small', monthlyBudgetUsd: 0, sshPublicKey: '' });
 const [prepared, setPrepared] = useState<CloudPlan | null>(null), [busy, setBusy] = useState(false), [error, setError] = useState(''), [dirty, setDirty] = useState(false);
 const owner = useRef({ allowed, digest: plan.digest, generation: 0 });
 if (owner.current.allowed !== allowed || owner.current.digest !== plan.digest) owner.current = { allowed, digest: plan.digest, generation: owner.current.generation + 1 };
 const pending = useRef<AbortController | null>(null), mounted = useRef(true);
 useEffect(() => { mounted.current = true; return () => { mounted.current = false; pending.current?.abort(); owner.current.generation++; }; }, []);
 useEffect(() => { if (!allowed) { pending.current?.abort(); setPrepared(null); setBusy(false); } }, [allowed]);
 useUnsavedProjectDraft(plan.blueprint.projectId, dirty, 'Azure foundation request', 'deploy');
 function change(patch: Partial<typeof request>) { pending.current?.abort(); owner.current.generation++; setRequest(value => ({ ...value, ...patch })); setPrepared(null); setError(''); setBusy(false); setDirty(true); }
 async function prepare(event: FormEvent) {
  event.preventDefault(); if (!allowed || busy) return;
  const controller = new AbortController(), generation = owner.current.generation;
  pending.current?.abort(); pending.current = controller; setBusy(true); setError(''); setPrepared(null);
  const current = () => mounted.current && !controller.signal.aborted && owner.current.allowed && owner.current.generation === generation && owner.current.digest === plan.digest;
  try {
   const value = await productApi.cloudPlan({ ...request, provider: 'azure', foundation: { blueprint: plan.blueprint, expectedBlueprintDigest: plan.digest } }, controller.signal);
   if (!current()) return;
   const binding = value.foundation;
   if (value.provider !== 'azure' || !binding || binding.version !== 1 || binding.request.blueprint.projectId !== plan.blueprint.projectId || binding.actor.projectId !== plan.blueprint.projectId || binding.request.expectedBlueprintDigest !== plan.digest || binding.fit.blueprintDigest !== plan.digest || binding.fit.tier !== request.size || !binding.fit.fits || binding.fit.capacityVerified !== false || !/^[a-f0-9]{64}$/.test(binding.contextDigest) || !/^[a-f0-9]{64}$/.test(binding.planDigest)) throw new Error('The returned foundation does not match this project and blueprint. Review it again.');
   setPrepared(value); setDirty(false);
  } catch (cause) { if (current()) setError(errorText(cause)); }
  finally { if (current()) { pending.current = null; setBusy(false); } }
 }
 if (!active) return null;
 return <details className="blueprint-result"><summary>Use this blueprint to size an Azure VM foundation</summary>
  <p>Create the existing Docker/Coolify foundation in your own Azure account. This step does not apply Kubernetes manifests, start application or inference containers, populate model storage, or connect telemetry.</p>
  <p>Source plan <code>{plan.digest}</code>. CPU and memory are requested totals; quota, price and available capacity remain unverified. Current project Deploy permission and provisioning/public-exposure policy are checked by the server.</p>
  {!allowed && <p role="status">Current project read/edit access is required. Creating resources also requires a current Deploy grant.</p>}
  <form onSubmit={event => void prepare(event)}><fieldset disabled={!allowed || busy}><legend>Choose the foundation explicitly</legend>
   <label className="luci-field">Azure resource-group name<input required value={request.name} pattern="[a-z][a-z0-9-]{1,39}" maxLength={40} onChange={event => change({ name: event.target.value })}/></label>
   <label className="luci-field">Azure region<input required value={request.region} pattern="[a-z][a-z0-9-]{1,39}" maxLength={40} onChange={event => change({ region: event.target.value })}/></label>
   <label className="luci-field">Domain you control<input required value={request.domain} maxLength={253} onChange={event => change({ domain: event.target.value })}/></label>
   <label className="luci-field">Foundation tier<select value={request.size} onChange={event => change({ size: event.target.value as CloudPlanRequest['size'] })}><option value="small">Small · 2 vCPU / 4 GiB</option><option value="standard">Standard · 2 vCPU / 8 GiB</option><option value="full">Full · 4 vCPU / 8 GiB</option></select></label>
   <label className="luci-field">Monthly planning budget (USD)<input type="number" min={1} max={100000} step="0.01" required value={request.monthlyBudgetUsd || ''} onChange={event => change({ monthlyBudgetUsd: Number(event.target.value) })}/></label>
   <label className="luci-field">Your SSH public key<textarea required value={request.sshPublicKey} rows={2} maxLength={24000} onChange={event => change({ sshPublicKey: event.target.value })}/><small>A public key only. No private key or provider credential is placed in this blueprint.</small></label>
   <Button type="submit" disabled={!allowed || busy}>{busy ? 'Reviewing foundation…' : 'Review Azure foundation'}</Button>
  </fieldset></form>
  {error && <p role="alert">{error}</p>}
  {prepared?.foundation && <section aria-label="Blueprint-bound Azure foundation"><h4>Foundation reviewed; resources not created yet</h4>
   <p>{prepared.resources.join(' · ')}. An explicit {prepared.foundation.fit.systemReserveMiB} MiB system reserve is excluded from workload sizing. This is not a provider availability or workload-performance guarantee.</p>
   <ul>{prepared.foundation.fit.decisions.map(row => <li key={`${row.path}:${row.code}`}>{row.message}</li>)}</ul>
   <CloudManager key={prepared.id} client={productApi} plan={prepared} mode="managed" foundationOnly onlyPlanId={prepared.id}/>
  </section>}
 </details>;
}
