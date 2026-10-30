import { useEffect, useId, useRef, useState } from 'react';
import { deploymentBlueprintApi, type BlueprintProjectContext, type BlueprintService, type DeploymentBlueprint, type DeploymentBlueprintPlan } from '../../api/deploymentBlueprint';
import { errorText } from '../../api/product';
import { teamScopeGeneration } from '../../api/teamScope';
import { PROJECT_CONTEXT_CHANGED } from '../../api/projectContextEvents';
import { useProjectDraftScope } from '../../hooks/useProjectDraft';
import { useProjectAccess } from '../../hooks/useProjectAccess';
import { useUnsavedProjectDraft } from '../../hooks/useUnsavedProjectDraft';
import { useTeamWorkspace } from '../teams/TeamWorkspaceProvider';
import { Button } from '../fluid/components/button';
import './deployment-blueprint.css';
import { AzureFoundationHandoff } from './AzureFoundationHandoff';

type ServiceDraft = BlueprintService & { useTelemetry: boolean; commandText: string; argsText: string };
type Editor = { reference: string; namespace: string; services: ServiceDraft[]; telemetry: boolean; telemetryReference: string; endpoint: string };
const service = (kind: 'app' | 'inference', id: string): ServiceDraft => ({ id, kind, image: '', port: 0, replicas: 0,
  resources: { requestMilliCpu: 0, limitMilliCpu: 0, requestMemoryMiB: 0, limitMemoryMiB: 0, gpuCount: 0 },
  command: [], args: [], dependsOn: [], useTelemetry: false, commandText: '', argsText: '',
  ...(kind === 'inference' ? { model: { name: '', claimName: '', mountPath: '', relativePath: '' } } : {}),
});
const initial = (): Editor => ({ reference: '', namespace: '', services: [service('app', 'app-1')], telemetry: false, telemetryReference: '', endpoint: '' });
const contextKey = (value: BlueprintProjectContext) => JSON.stringify(value);
const lines = (value: string) => value.split('\n').filter(line => line.trim() !== '');
function requestFor(projectId: string, draft: Editor, context: BlueprintProjectContext): DeploymentBlueprint {
  const telemetry = draft.telemetry && draft.services.some(row => row.useTelemetry);
  return { version: 1, projectId, source: context.source, infrastructure: { id: 'cluster', kind: 'kubernetes', reference: draft.reference, namespace: draft.namespace },
    services: draft.services.map(({ useTelemetry, commandText, argsText, ...row }) => ({ ...row, command: lines(commandText), args: lines(argsText), ...(telemetry && useTelemetry ? { telemetry: { resourceId: 'analytics', endpoint: draft.endpoint } } : {}) })),
    ...(telemetry ? { externalResources: [{ id: 'analytics', kind: 'analytics' as const, reference: draft.telemetryReference }] } : {}),
  };
}

export function DeploymentBlueprint({ projectId, active, disabled = false, onPublishing }: { projectId: string; active: boolean; disabled?: boolean; onPublishing(): void }) {
  const scope = useProjectDraftScope(projectId), team = useTeamWorkspace();
  return <BlueprintEditor key={JSON.stringify([scope, teamScopeGeneration(), team?.teamId])} projectId={projectId} scope={scope} active={active} disabled={disabled} onPublishing={onPublishing} />;
}

function BlueprintEditor({ projectId, scope, active, disabled, onPublishing }: { projectId: string; scope: string | null; active: boolean; disabled: boolean; onPublishing(): void }) {
  const access = useProjectAccess(projectId), team = useTeamWorkspace(), heading = useId();
  const canPlan = !!scope && access.canRead && !access.loading && !access.error && !team?.accessBlocked;
  const authority = JSON.stringify([scope, projectId, canPlan, active, disabled, teamScopeGeneration()]);
  const owner = useRef({ key: authority, epoch: 0 });
  if (owner.current.key !== authority) owner.current = { key: authority, epoch: owner.current.epoch + 1 };
  const sequence = useRef(0), pending = useRef<AbortController | null>(null), mounted = useRef(true);
  const [draft, setDraft] = useState<Editor>(initial), [dirty, setDirty] = useState(false);
  const [context, setContext] = useState<BlueprintProjectContext | null>(null), [plan, setPlan] = useState<DeploymentBlueprintPlan | null>(null);
  const [busy, setBusy] = useState(false), [error, setError] = useState(''), [notice, setNotice] = useState('');
  const downloads = useRef(new Map<string, ReturnType<typeof setTimeout>>());
  const nextService = useRef({ app: 1, inference: 0 });
  function retire() { sequence.current++; pending.current?.abort(); pending.current = null; }
  function discard() { retire(); nextService.current = { app: 1, inference: 0 }; setDraft(initial()); setDirty(false); setPlan(null); setError(''); setNotice('Planning draft cleared.'); setBusy(false); }
  useUnsavedProjectDraft(projectId, dirty, 'Deployment blueprint', 'deploy', { retained: false, discard });
  useEffect(() => {
    mounted.current = true;
    return () => { mounted.current = false; retire(); for (const [url, timer] of downloads.current) { clearTimeout(timer); URL.revokeObjectURL(url); } downloads.current.clear(); };
  }, []);
  useEffect(() => { if (!active || !canPlan || disabled) { retire(); setBusy(false); setPlan(null); } }, [active, canPlan, disabled]);
  useEffect(() => {
    const changed = (event: Event) => {
      const detail = (event as CustomEvent<{ scope: string; projectId: string }>).detail;
      if (detail?.scope !== scope || detail.projectId !== projectId) return;
      retire(); setPlan(null); setContext(null); setBusy(false); setNotice('Project context changed. Generate a fresh plan before using any output.');
    };
    window.addEventListener(PROJECT_CONTEXT_CHANGED, changed);
    return () => window.removeEventListener(PROJECT_CONTEXT_CHANGED, changed);
  }, [scope, projectId]);
  function edit(next: Editor) { retire(); setDraft(next); setDirty(true); setPlan(null); setError(''); setNotice(''); setBusy(false); }
  function change(id: string, patch: Partial<ServiceDraft>) { edit({ ...draft, services: draft.services.map(row => row.id === id ? { ...row, ...patch } : row) }); }
  function add(kind: 'app' | 'inference') {
    const index = ++nextService.current[kind];
    edit({ ...draft, services: [...draft.services, service(kind, `${kind}-${index}`)] });
  }
  async function generate() {
    if (!canPlan || !active || disabled || pending.current) return;
    const controller = new AbortController(), token = ++sequence.current, epoch = owner.current.epoch;
    pending.current = controller; setBusy(true); setPlan(null); setError(''); setNotice('');
    const current = () => mounted.current && !controller.signal.aborted && sequence.current === token && owner.current.epoch === epoch;
    try {
      const before = await deploymentBlueprintApi.context(projectId);
      if (!current()) return;
      setContext(before);
      const result = await deploymentBlueprintApi.compile(requestFor(projectId, draft, before), controller.signal);
      if (!current()) return;
      const after = await deploymentBlueprintApi.context(projectId);
      if (!current()) return;
      setContext(after);
      if (contextKey(before) !== contextKey(after)) throw new Error('Project source or context changed during planning. Review the current revisions and generate again.');
      setPlan(result);
    } catch (cause) { if (current()) setError(errorText(cause)); }
    finally { if (current()) { pending.current = null; setBusy(false); } }
  }
  function download(name: string, content: string) {
    if (!canPlan || !active || !plan) return;
    try {
      const url = URL.createObjectURL(new Blob([content], { type: 'application/json' }));
      const timer = setTimeout(() => { URL.revokeObjectURL(url); downloads.current.delete(url); }, 1000);
      downloads.current.set(url, timer);
      const link = document.createElement('a'); link.href = url; link.download = name; document.body.appendChild(link);
      try { link.click(); } finally { link.remove(); }
      setNotice('Download requested. Keep the plan with its source revisions; this does not deploy it.');
    } catch { setError('This browser could not export the file. The generated text remains available below.'); }
  }
  if (!canPlan) return <section className="journey-page"><p role="status">{access.message || 'Current project read access is required to prepare a deployment plan.'}</p></section>;
  const locked = disabled || busy;
  return <section className="journey-page blueprint-editor" aria-labelledby={heading}>
    <header><h2 id={heading}>Plan your application services</h2><p>Plan app and inference services, then review and export manifests for your existing cluster.</p><p className="journey-muted">Your draft stays here while you switch tabs. Export a copy before closing the workspace. Use resource references, not credentials.</p></header>
    <form onSubmit={event => { event.preventDefault(); void generate(); }}>
      <fieldset disabled={locked} className="definition-fields"><legend>Existing Kubernetes infrastructure</legend><div className="blueprint-grid">
        <label className="luci-field">Cluster reference<input value={draft.reference} maxLength={512} placeholder="Your existing cluster identity" onChange={e => edit({ ...draft, reference: e.target.value })}/></label>
        <label className="luci-field">Namespace<input value={draft.namespace} maxLength={63} placeholder="An existing namespace" onChange={e => edit({ ...draft, namespace: e.target.value })}/></label>
      </div></fieldset>
      {draft.services.map(row => <fieldset className="definition-record" key={row.id} disabled={locked}><legend>{row.kind === 'app' ? 'Application' : 'Inference'} · {row.id}</legend><div className="blueprint-grid">
        <label className="luci-field">Image digest<input value={row.image} maxLength={512} placeholder="registry/image@sha256:…" onChange={e => change(row.id, { image: e.target.value })}/></label>
        <NumberField label="Internal port" value={row.port} max={65535} onChange={port => change(row.id, { port })}/>
        <NumberField label="Replicas" value={row.replicas} max={32} onChange={replicas => change(row.id, { replicas })}/>
        {(['requestMilliCpu', 'limitMilliCpu', 'requestMemoryMiB', 'limitMemoryMiB', 'gpuCount'] as const).map((key, index) => <NumberField key={key} label={['CPU request (millicores)', 'CPU limit (millicores)', 'Memory request (MiB)', 'Memory limit (MiB)', 'NVIDIA GPUs per replica'][index]} value={row.resources[key] ?? 0} max={index === 4 ? 8 : index < 2 ? 64000 : 1048576} onChange={value => change(row.id, { resources: { ...row.resources, [key]: value } })}/>)}
      </div>
      <details><summary>Container entrypoint and arguments</summary><p className="journey-muted">One argument per line. These describe the image’s existing command; nothing runs here. Inference needs its explicit server configuration.</p><div className="blueprint-grid">
        <label className="luci-field">Command<textarea rows={3} value={row.commandText} maxLength={16000} onChange={e => change(row.id, { commandText: e.target.value })}/></label>
        <label className="luci-field">Arguments<textarea rows={3} value={row.argsText} maxLength={60000} onChange={e => change(row.id, { argsText: e.target.value })}/></label>
      </div></details>
      {row.model && <div className="blueprint-grid">
        {(['name', 'claimName', 'mountPath', 'relativePath'] as const).map((key, index) => <label className="luci-field" key={key}>{['Model name', 'Existing model PVC', 'Read-only mount path', 'Path within the PVC (optional)'][index]}<input value={row.model![key] ?? ''} maxLength={index === 0 ? 128 : index === 1 ? 63 : 256} onChange={e => change(row.id, { model: { ...row.model!, [key]: e.target.value } })}/></label>)}
      </div>}
      <fieldset className="definition-links"><legend>Depends on services</legend>
        {draft.services.filter(other => other.id !== row.id).map(other => <label key={other.id}><input type="checkbox" checked={row.dependsOn?.includes(other.id) ?? false} onChange={e => change(row.id, { dependsOn: e.target.checked ? [...(row.dependsOn ?? []), other.id] : row.dependsOn?.filter(id => id !== other.id) })}/>{other.id}</label>)}
        {(row.dependsOn ?? []).filter(id => !draft.services.some(other => other.id === id)).map(id => <label key={id}><input type="checkbox" checked onChange={() => change(row.id, { dependsOn: row.dependsOn?.filter(value => value !== id) })}/>{id} · removed service. Uncheck to repair this dependency.</label>)}
      </fieldset>
      <label><input type="checkbox" checked={row.useTelemetry} onChange={e => edit({ ...draft, telemetry: e.target.checked || draft.telemetry, services: draft.services.map(item => item.id === row.id ? { ...item, useTelemetry: e.target.checked } : item) })}/>Bind this service to the telemetry receiver below</label>
      <Button type="button" variant="tertiary" disabled={draft.services.length === 1} onClick={() => edit({ ...draft, services: draft.services.filter(other => other.id !== row.id) })}>Remove {row.id}</Button>
      </fieldset>)}
      <div className="journey-actions"><Button type="button" variant="secondary" disabled={locked || draft.services.length >= 8} onClick={() => add('app')}>Add application</Button><Button type="button" variant="secondary" disabled={locked || draft.services.length >= 8} onClick={() => add('inference')}>Add inference service</Button></div>
      <fieldset disabled={locked} className="definition-fields"><legend>Optional telemetry</legend><label><input type="checkbox" checked={draft.telemetry} onChange={e => edit({ ...draft, telemetry: e.target.checked, services: e.target.checked ? draft.services : draft.services.map(row => ({ ...row, useTelemetry: false })) })}/>Use an existing OTLP/HTTP receiver</label>
        {draft.telemetry && <div className="blueprint-grid"><label className="luci-field">Receiver reference<input value={draft.telemetryReference} maxLength={512} onChange={e => edit({ ...draft, telemetryReference: e.target.value })}/></label><label className="luci-field">HTTPS receiver endpoint<input value={draft.endpoint} maxLength={2048} onChange={e => edit({ ...draft, endpoint: e.target.value })}/><small>No credentials, query tokens or request headers. This binds configuration; it does not instrument the app or provision analytics.</small></label></div>}
      </fieldset>
      <div className="journey-actions"><Button type="submit" variant="primary" disabled={locked}>Generate deployment plan</Button><Button type="button" variant="tertiary" disabled={locked || !dirty} onClick={discard}>Clear planning draft</Button></div><p className="journey-muted">Planning does not create services.</p>
    </form>
    {busy && <p role="status">Reading the project’s current source and context, then compiling the plan…</p>}{error && <p role="alert" className="journey-error">{error}</p>}{notice && <p role="status">{notice}</p>}
    {context && <p className="journey-muted">Observed HEAD: {context.head || 'No commit'} · configuration {context.source.configurationRevision} · definition {context.source.context?.specificationRevision} ({context.definitionStatus}) · design {context.source.context?.designRevision} ({context.designStatus}). {!context.clean && 'Uncommitted changes are not represented by HEAD. Commit them to resolve the source input.'}</p>}
    {plan && <section className="blueprint-result" aria-label="Generated deployment plan"><h3>{plan.state === 'compiled-prototype' ? 'Manifests ready for review' : 'Inputs still needed'}</h3><p>Review the generated services and unresolved inputs before exporting.</p>
      <ul>{plan.decisions.map(row => <li key={`${row.path}:${row.code}`}><strong>{row.path}</strong>: {row.message}</li>)}</ul>
      <h4>Service order</h4><ol>{plan.steps.map(step => <li key={step.componentId}><strong>{step.componentId}</strong> · {step.action.replace(/-/g, ' ')}{step.dependsOn.length > 0 && ` · after ${step.dependsOn.join(', ')}`}</li>)}</ol>
      <p>Requested totals: {plan.resourceSummary.replicas} replicas · {plan.resourceSummary.requestMilliCpu} millicores · {plan.resourceSummary.requestMemoryMiB} MiB memory · {plan.resourceSummary.gpuCount} NVIDIA GPUs. These are planned requests, not available capacity or a price quote.</p>
      {plan.artifacts.map(artifact => <details key={artifact.name}><summary>{artifact.name}</summary><p className="journey-muted">SHA-256: {artifact.sha256}</p><pre tabIndex={0}>{artifact.content}</pre><Button type="button" variant="secondary" onClick={() => download(artifact.name, artifact.content)}>Download {artifact.name}</Button></details>)}
      <div className="journey-actions"><Button type="button" variant="secondary" onClick={() => download('luci-deployment-plan.json', JSON.stringify(plan, null, 2)+'\n')}>Export plan JSON</Button><Button type="button" variant="secondary" disabled={disabled} onClick={onPublishing}>Open existing release paths</Button></div>
      <AzureFoundationHandoff key={plan.digest} plan={plan} active={active && canPlan && !disabled} />
      <details><summary>Capabilities and review notes</summary><p>Plan <code>{plan.digest}</code>. Exporting does not deploy the manifests. Local, your cloud and GitHub Pages keep their existing release forms and permissions; they do not apply Kubernetes manifests. Cluster application and observation are not connected yet.</p><ul>{plan.capabilities.map(row => <li key={row.id}><strong>{row.id} · {row.state}</strong>: {row.description}</li>)}{plan.warnings.map((warning, index) => <li key={index}>{warning}</li>)}</ul></details>
    </section>}
  </section>;
}

function NumberField({ label, value, max, onChange }: { label: string; value: number; max: number; onChange(value: number): void }) {
  return <label className="luci-field">{label}<input type="number" min={0} max={max} step={1} value={value || ''} onChange={event => onChange(event.target.value === '' ? 0 : Number(event.target.value))}/></label>;
}
