import { useEffect, useRef, useState } from 'react';
import { approvedContractReference, orchestrationContractsApi, type OrchestrationContractReference, type OrchestrationContractView, type PrepareOrchestrationContract } from '../../api/orchestrationContracts';
import { errorText } from '../../api/product';
import { Button } from '../fluid/components/button';
import type { CoordinatorContinuationRequest } from '../../api/coordinatorContinuations';
import './ExecutionContractReview.css';

const prefix = 'luci.execution-contract-review.v1:';
const ordinaryReviewBytes = 65536;
const continuationReviewBytes = 1 << 20; // exact retained checks may include eight bounded argument lists
export interface ExecutionContractReviewRequest {
  version: 1; projectId: string; actorId: string; request: PrepareOrchestrationContract;
  runKey: string; contractId?: string; continuation?: CoordinatorContinuationRequest;
}
export function contractReviewKey(scope: string): string { return prefix + encodeURIComponent(scope); }
export function readContractReview(scope: string | null, projectId: string, actorId: string): ExecutionContractReviewRequest | null {
  if (!scope) return null;
  const raw = localStorage.getItem(contractReviewKey(scope));
  if (!raw) return null;
  if (raw.length * 2 > continuationReviewBytes) throw new Error('The retained execution contract request exceeds its limit.');
  const value = JSON.parse(raw) as ExecutionContractReviewRequest;
  if (!value.continuation && raw.length * 2 > ordinaryReviewBytes) throw new Error('The retained execution contract request exceeds its limit.');
  const request = value?.request, route = request?.route, context = request?.implementationContext;
  const fields = (v: object, allowed: string[]) => Object.keys(v).every(key => allowed.includes(key));
  const text = (v: unknown, max: number) => typeof v === 'string' && v.length <= max;
  if (value.version !== 1 || value.projectId !== projectId || value.actorId !== actorId || value.request?.implementationContext?.projectId !== projectId ||
      !/^[!-~]{16,200}$/.test(value.request.idempotencyKey) || !/^[!-~]{16,200}$/.test(value.runKey) ||
      !['codex', 'claude-code', 'opencode'].includes(route?.harness) || !['ask', 'bypass'].includes(route?.permissionMode) ||
      !['local-docker', 'cloud-microvm'].includes(route.executionLocation) || !['preview', 'publish'].includes(request.outcome) ||
      !text(request.worktreeId, 256) || !request.worktreeId || !text(route.model, 512) || route.effort !== undefined && !text(route.effort, 64) ||
      route.agentConnectionId !== undefined && !text(route.agentConnectionId, 256) || value.contractId !== undefined && !text(value.contractId, 256) ||
      ![context.briefRevision, context.planRevision, context.specificationRevision, context.designPlanRevision].every(x => Number.isSafeInteger(x) && x > 0) ||
      !Array.isArray(context.taskIds) || context.taskIds.length < 1 || context.taskIds.length > 200 || context.taskIds.some(x => !text(x, 256) || !x) ||
      value.continuation !== undefined && (value.continuation.idempotencyKey !== value.runKey || value.continuation.run?.implementationContext?.projectId !== projectId || value.continuation.run?.orchestrationContract !== undefined ||
        value.continuation.run?.worktreeId !== request.worktreeId || value.continuation.run?.harness !== route.harness || value.continuation.run?.model !== route.model || value.continuation.run?.permissionMode !== route.permissionMode || value.continuation.run?.outcome !== request.outcome ||
        JSON.stringify(value.continuation.run?.implementationContext) !== JSON.stringify(context)) ||
      !fields(value, ['version', 'projectId', 'actorId', 'request', 'runKey', 'contractId', 'continuation']) ||
      !fields(request, ['idempotencyKey', 'worktreeId', 'implementationContext', 'route', 'outcome']) ||
      !fields(route, ['harness', 'model', 'permissionMode', 'effort', 'executionLocation', 'agentConnectionId']) ||
      !fields(context, ['projectId', 'briefRevision', 'planRevision', 'specificationRevision', 'designPlanRevision', 'taskIds'])) {
    throw new Error('The retained execution contract request could not be verified.');
  }
  return value;
}
export function retainContractReview(scope: string | null, value: ExecutionContractReviewRequest): void {
  if (!scope) throw new Error('Account and workspace identity are required to retain this review before saving.');
  const raw = JSON.stringify(value);
  if (raw.length * 2 > (value.continuation ? continuationReviewBytes : ordinaryReviewBytes)) throw new Error('The execution contract request exceeds its recovery limit.');
  const key = contractReviewKey(scope);
  const prior = localStorage.getItem(key);
  if (prior) {
    if (prior.length * 2 > continuationReviewBytes) throw new Error('Existing review recovery exceeds its limit.');
    const previous = JSON.parse(prior) as ExecutionContractReviewRequest;
    if (previous.actorId !== value.actorId || previous.projectId !== value.projectId || previous.runKey !== value.runKey || JSON.stringify(previous.request) !== JSON.stringify(value.request) || JSON.stringify(previous.continuation) !== JSON.stringify(value.continuation)) {
      throw new Error('Another execution contract review is retained. Inspect it before replacing its request.');
    }
  }
  localStorage.setItem(key, raw);
  if (localStorage.getItem(key) !== raw) throw new Error('The execution contract request could not be retained. Enable browser storage before saving.');
}

export function archiveContractReview(scope: string, value: ExecutionContractReviewRequest): void {
  const key = contractReviewKey(scope), raw = localStorage.getItem(key);
  const current = readContractReview(scope, value.projectId, value.actorId);
  if (!raw || current?.request.idempotencyKey !== value.request.idempotencyKey) throw new Error('Another review changed. Refresh before choosing new inputs.');
  let count = 0, bytes = raw.length * 2;
  for (let index = 0; index < localStorage.length; index++) {
    const existing = localStorage.key(index);
    if (existing?.startsWith(prefix) && existing.includes(':archive:')) {
      count++; bytes += (localStorage.getItem(existing)?.length ?? 0) * 2;
    }
  }
  if (count >= 128 || bytes > 2 * 1024 * 1024) throw new Error('Retained contract history is full. Keep this request until its history can be inspected.');
  const archived = key + ':archive:' + value.request.idempotencyKey;
  localStorage.setItem(archived, raw);
  if (localStorage.getItem(archived) !== raw) throw new Error('The original review identity could not be preserved.');
  // Read back the active identity again: a second tab cannot lose its new work.
  if (localStorage.getItem(key) !== raw) throw new Error('The active review changed. Its request remains retained.');
  localStorage.removeItem(key);
}

export function ExecutionContractReview({ pending, initialView, scope, actorId, canRead, canEdit, preparing = false, continuation = false, onPrepared, onCreate, onComplete, onReviewInputs }: {
  pending: ExecutionContractReviewRequest; initialView: OrchestrationContractView | null; scope: string | null;
  actorId: string; canRead: boolean; canEdit: boolean;
  preparing?: boolean;
  continuation?: boolean;
  onPrepared(pending: ExecutionContractReviewRequest, view: OrchestrationContractView): void;
  onCreate(request: PrepareOrchestrationContract, reference: OrchestrationContractReference, idempotencyKey: string): Promise<void>;
  onComplete(): void;
  onReviewInputs(): void;
}) {
  const [view, setView] = useState(initialView);
  const [error, setError] = useState('');
  const [busy, setBusy] = useState('');
  const [consent, setConsent] = useState(false);
  const working = useRef(false), generation = useRef(0);
  const allowed = canRead && canEdit && !!scope && actorId === pending.actorId;
  const mayWrite = useRef(allowed); mayWrite.current = allowed;
  const mayRead = useRef(canRead); mayRead.current = canRead;
  useEffect(() => { generation.current++; return () => { generation.current++; }; }, []);
  useEffect(() => { setView(initialView); setConsent(false); }, [initialView]);

  function own(value: OrchestrationContractView) {
    if (value.contract.actor.id !== actorId) throw new Error('This execution contract belongs to another person. Refresh the workspace.');
    return value;
  }
  async function refresh() {
    if (!canRead || !pending.contractId && !mayWrite.current || working.current || preparing) return;
    const current = generation.current;
    working.current = true; setBusy('Reading exact contract'); setError(''); setConsent(false);
    try {
      // A known identity is read only. An unknown preparation can be resolved
      // only by its same retained key; preparation cannot start execution.
      const value = own(pending.contractId ? await orchestrationContractsApi.get(pending.projectId, pending.contractId)
        : await orchestrationContractsApi.prepare(pending.projectId, pending.request));
      if (generation.current !== current || !mayRead.current) return;
      const next = { ...pending, contractId: value.contract.id };
      retainContractReview(scope, next); setView(value); onPrepared(next, value);
    } catch (cause) { if (generation.current === current) setError(errorText(cause)); }
    finally { working.current = false; if (generation.current === current) setBusy(''); }
  }
  useEffect(() => {
    const current = generation.current;
    queueMicrotask(() => { if (generation.current === current && !initialView && pending.contractId && mayRead.current) void refresh(); });
  }, [pending.contractId, canRead]);

  async function approveAndCreate() {
    if (!allowed || !view || !consent || working.current || preparing || view.freshness.status !== 'current') return;
    const current = generation.current;
    working.current = true; setBusy('Saving reviewed contract and run'); setError('');
    try {
      retainContractReview(scope, pending);
      // Always read current context/policy immediately before the explicit
      // write. Approval is not execution, spending or deployment permission.
      let latest = own(await orchestrationContractsApi.get(pending.projectId, view.contract.id));
      if (generation.current !== current || !mayWrite.current) return;
      if (latest.freshness.status !== 'current' || latest.contract.digest !== view.contract.digest || latest.contract.revision !== view.contract.revision) {
        setView(latest); setConsent(false); throw new Error(latest.freshness.reason || 'The contract changed. Inspect the current selection before continuing.');
      }
      if (latest.contract.state !== 'approved') latest = own(await orchestrationContractsApi.approve(pending.projectId, latest));
      if (generation.current !== current || !mayWrite.current) return;
      setView(latest); onPrepared(pending, latest);
      await onCreate(pending.request, approvedContractReference(latest), pending.runKey);
      if (generation.current !== current || !mayWrite.current) return;
      if (scope) {
        const saved = readContractReview(scope, pending.projectId, pending.actorId);
        if (saved?.request.idempotencyKey === pending.request.idempotencyKey) localStorage.removeItem(contractReviewKey(scope));
      }
      onComplete();
    } catch (cause) {
      if (generation.current === current) {
        setError(`${errorText(cause)} The exact request is retained. ${continuation ? 'Refresh its recorded status before continuing; do not submit it again while its effect is unconfirmed.' : 'Refresh its recorded status before continuing; no coding session was started here.'}`);
        setConsent(false);
      }
    } finally { working.current = false; if (generation.current === current) setBusy(''); }
  }

  function reviewUpdatedInputs() {
    if (!allowed || !scope || working.current || preparing) return;
    try { archiveContractReview(scope, pending); onReviewInputs(); }
    catch (cause) { setError(errorText(cause)); }
  }

  if (!canRead) return null;
  const c = view?.contract;
  return <section className="execution-contract-review" aria-label="Execution contract review">
    <h3>Review how Luci will build</h3>
    <p>{continuation ? 'Review and approve the exact contract before the retained continuation can prepare or queue work. Coding, public deployment and paid resources keep their existing controls.' : 'This saves an exact contract and a project run. Coding, public deployment and paid resources keep their existing separate controls.'}</p>
    {error && <p role="alert">{error}</p>}
    {!view ? <p>The preparation outcome is not yet confirmed. Inspect the retained request before preparing another selection.</p> : <>
      <p role={view.freshness.status === 'current' ? undefined : 'status'}><strong>{view.freshness.status === 'current' ? 'Current context and policy' : 'Context needs reconciliation'}</strong>{view.freshness.reason && ` · ${view.freshness.reason}`}</p>
      <dl><dt>Design and definition</dt><dd>Design {c!.input.implementation.designPlanRevision} · Definition {c!.input.implementation.specificationRevision} · Plan {c!.input.implementation.planRevision}</dd>
        <dt>Upstream route</dt><dd>{c!.input.route.harness} · {c!.input.route.model || 'configured model'} · {c!.input.route.permissionMode} · {pending.request.route.executionLocation === 'local-docker' ? 'isolated Docker on this computer' : 'configured isolated cloud microVM'}</dd>
        <dt>Intended result</dt><dd>{c!.input.outcome === 'preview' ? 'Workspace preview' : 'Public deployment, subject to separate authority'}</dd>
        <dt>Standards and controls</dt><dd>{c!.input.standards.status === 'legacy-unassessed' ? 'Approved design notes retained; standards have not been assessed against a structured control library.' : 'No team standards library is configured; no compliance assessment is claimed.'}</dd>
        <dt>Saved policy</dt><dd>Coordinator revision {c!.input.controls.coordinatorPolicyRevision} · Scheduler revision {c!.input.controls.schedulerPolicyRevision}</dd></dl>
      <p><strong>Routine decisions:</strong> {c!.input.decisionPolicy.routine}</p><p><strong>Human input:</strong> {c!.input.decisionPolicy.humanInput}</p>
      <details><summary>Tasks, ownership and acceptance · {c!.input.tasks.length}</summary>{c!.input.tasks.map(task => <div key={task.id}><h4>{task.title}</h4><p>{task.acceptance || 'No task-specific acceptance is saved; inspect the linked requirement criteria.'}</p><p>Depends on: {task.dependsOn?.join(', ') || 'none'}</p><p>Write scope: {task.writePaths?.join(', ') || 'Unscoped; review before automatic integration'}</p><p>Requirements: {task.requirementIds?.join(', ') || 'none linked'}</p></div>)}</details>
      <details><summary>Reviewed requirement criteria · {c!.input.requirements.length}</summary>{c!.input.requirements.map(requirement => <div key={requirement.id}><h4>{requirement.title}</h4><p>{requirement.acceptance}</p></div>)}</details>
      <details><summary>Exact contract identity</summary><p>{c!.id} · revision {c!.revision}</p><code>{c!.digest}</code><p>Starting commit: {c!.input.implementation.sourceCommit || 'not committed'}</p><p>Standards source: {c!.input.standards.source} · revision {c!.input.standards.revision}</p></details>
      <label className="setup-check"><input type="checkbox" checked={consent} disabled={!allowed || !!busy || preparing || view.freshness.status !== 'current'} onChange={event => setConsent(event.target.checked)} />I reviewed these exact scopes, acceptance criteria and decision boundaries.</label>
    </>}
    <div className="workflow-actions"><Button variant="secondary" disabled={!!busy || preparing || !canRead || !pending.contractId && !allowed} onClick={() => void refresh()}>Refresh contract status</Button>
      {view && <Button disabled={!allowed || !consent || !!busy || preparing || view.freshness.status !== 'current'} loading={!!busy} onClick={() => void approveAndCreate()}>{continuation ? c!.state === 'approved' ? 'Continue reviewed work' : 'Approve contract and continue reviewed work' : c!.state === 'approved' ? 'Save reviewed project run' : 'Approve contract and save project run'}</Button>}</div>
    {(!view || view.freshness.status !== 'current') && <><p>Choosing updated inputs preserves this original request and its exact keys. It does not cancel or settle any previously saved run.</p>
      <Button variant="ghost" disabled={!allowed || !!busy || preparing} onClick={reviewUpdatedInputs}>Keep original request and review updated plan</Button></>}
  </section>;
}
