import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { coordinatorContinuationsApi, type CoordinatorAcceptedReview, type CoordinatorCloudCapacity, type CoordinatorContinuation, type CoordinatorContinuationRequest, type CoordinatorContinuationState, type CoordinatorSavedRoute } from '../../api/coordinatorContinuations';
import { coordinatorPolicyApi, effectiveAutomaticRepairLimit, validAutomaticRepairLimit, type CoordinatorPolicy } from '../../api/coordinatorPolicy';
import type { OrchestratorView } from '../../api/orchestrator';
import type { CreateSessionRequest } from '../../api/client';
import { ApiError } from '../../api/client';
import { errorText, ProductApiError } from '../../api/product';
import { MAX_SWARM_PARALLEL_TASKS } from '../../api/swarmLimits';
import { useCockpit } from '../../store/store';
import { selectWorktreeById } from '../../store/selectors';
import { Button } from '../fluid/components/button';
import { CoordinatorPreviewSetup, readCoordinatorConfiguration, readCoordinatorSourceConfiguration, type CoordinatorPreviewSelection } from './CoordinatorPreviewSetup';
import { coordinatorDeploymentApi, sameCoordinatorDeploymentBinding, type CoordinatorDeploymentPlan } from '../../api/coordinatorDeployment';
import { CoordinatorDeploymentSetup, type CoordinatorDeploymentSelection } from './CoordinatorDeploymentSetup';
import { CoordinatorCheckSetup, type CoordinatorCheckSelection } from './CoordinatorCheckSetup';
import { SessionPicker } from '../topbar/SessionPicker';
import { orchestrationContractsApi, type OrchestrationContractReference, type OrchestrationContractView, type PrepareOrchestrationContract } from '../../api/orchestrationContracts';
import { ExecutionContractReview, readContractReview, retainContractReview, archiveContractReview, type ExecutionContractReviewRequest } from '../develop/ExecutionContractReview';

type Pending = { actorId: string; projectId: string; rejected?: true } & (
  { kind: 'create'; input: CoordinatorContinuationRequest } |
  { kind: 'advance'; id: string; revision: number } |
  { kind: 'policy'; revision: number; requiresDeploy?: true; automaticRepairLimit?: number }
);
const refused = (error: unknown) => error instanceof ProductApiError && [400, 401, 403, 409].includes(error.status);
const denied = (error: unknown) => error instanceof ApiError && [401, 403].includes(error.status);
const states = { ready: 'Ready to continue', continuing: 'Continuing', queued: 'Tasks queued', checking: 'Independent checks running', 'checks-verified': 'Independent checks verified', 'previewing': 'Preparing checked-source preview', 'preview-ready': 'Local preview ready', 'preview-blocked': 'Preview needs attention', deploying: 'Deploying checked source', deployed: 'Deployment observed healthy', 'deployment-blocked': 'Deployment needs attention', blocked: 'Needs attention', unknown: 'Outcome unconfirmed', refused: 'Needs attention' };
const steps = { 'create-run': 'Reviewed build', 'prepare-tasks': 'Task preparation', 'queue-tasks': 'Task dispatch' };
const MAX_PENDING_BYTES = 512 * 1024;
const keyFor = (scope: string) => `luci.coordinator-execution.v1:${scope}`;
function initialRequest(key: string, projectId: string, actorId: string) {
  try {
    const raw = localStorage.getItem(key);
    if (!raw) return { raw, value: null as Pending | null, error: '' };
    const value = JSON.parse(raw) as Pending;
    if (new TextEncoder().encode(raw).length > MAX_PENDING_BYTES || value.actorId !== actorId || value.projectId !== projectId || (value.rejected !== undefined && value.rejected !== true) || !['create', 'advance', 'policy'].includes(value.kind) ||
        value.kind === 'create' && (!value.input?.idempotencyKey || value.input.run?.implementationContext?.projectId !== projectId) ||
        value.kind === 'advance' && (!value.id || !Number.isSafeInteger(value.revision)) || value.kind === 'policy' && (!Number.isSafeInteger(value.revision) || value.requiresDeploy !== undefined && value.requiresDeploy !== true || value.automaticRepairLimit !== undefined && !validAutomaticRepairLimit(value.automaticRepairLimit))) throw new Error('The retained continuation request cannot be read. Keep it for recovery; new work is paused.');
    return { raw, value, error: '' };
  } catch (cause) { return { raw: null, value: null as Pending | null, error: errorText(cause) }; }
}
/** Mount keyed to account/workspace/project scope; no credentials or approval grants are saved here. */
export function CoordinatorExecution({ projectId, scope, actorId, view, canRead, canEdit, canExecute, canDeploy = false, blocked, runOn, onInspect, onNavigate, policyEpoch = 0, onPolicyUpdated }: {
  projectId: string; scope: string; actorId: string; view: OrchestratorView;
  canRead: boolean; canEdit: boolean; canExecute: boolean; canDeploy?: boolean; blocked: string; runOn: string;
  policyEpoch?: number; onPolicyUpdated?(): void;
  onInspect(runId: string): void; onNavigate(lens: OrchestratorView['nextAction']['lens']): void;
}) {
  const key = keyFor(scope), [initial] = useState(() => initialRequest(key, projectId, actorId));
  const [pending, setPending] = useState(initial.value), pendingRef = useRef(initial.value), rawRef = useRef(initial.raw);
  const reviewScope = `${scope}/continued-work`;
  const [initialReview] = useState(() => { try { return { value: readContractReview(reviewScope, projectId, actorId), error: '' }; } catch (cause) { return { value: null, error: errorText(cause) }; } });
  const [contractReview, setContractReview] = useState<ExecutionContractReviewRequest | null>(initialReview.value);
  const [contractView, setContractView] = useState<OrchestrationContractView | null>(null);
  const [preparingReview, setPreparingReview] = useState(false), preparingReviewKey = useRef<string | null>(null);
  const [storageError, setStorageError] = useState(initial.error || initialReview.error);
  const [rows, setRows] = useState<CoordinatorContinuation[]>([]), [contextCommits, setContextCommits] = useState<NonNullable<CoordinatorContinuationState['contextCommits']>>([]), [selected, setSelected] = useState('');
  const [acceptedReview, setAcceptedReview] = useState<CoordinatorAcceptedReview | null>(null);
  const [policy, setPolicy] = useState<CoordinatorPolicy | null>(null), [fresh, setFresh] = useState(false);
  const [repairLimit, setRepairLimit] = useState(2);
  useEffect(() => { if (policy) setRepairLimit(effectiveAutomaticRepairLimit(policy.automaticRepairLimit)); }, [policy]);
  const [deliveryReadWarning, setDeliveryReadWarning] = useState('');
  const [error, setError] = useState(''), [notice, setNotice] = useState(''), [busy, setBusy] = useState(''), [readDenied, setReadDenied] = useState(false);
  const [consent, setConsent] = useState(false), [integrationConsent, setIntegrationConsent] = useState(false), [picker, setPicker] = useState<CoordinatorContinuationRequest | null>(null);
  const [deploymentConsent, setDeploymentConsent] = useState(false), [deploymentFresh, setDeploymentFresh] = useState(false);
  const [deploymentSelection, setDeploymentSelection] = useState<CoordinatorDeploymentSelection>({ plan: null, dirty: true, problem: 'Reading retained deployment setup…', draftDigest: '' }), [recordedDeployment, setRecordedDeployment] = useState('');
  const deploymentChanged = useCallback((value: CoordinatorDeploymentSelection) => setDeploymentSelection(value), []);
  const [previewConsent, setPreviewConsent] = useState(false), [configurationRevision, setConfigurationRevision] = useState<number | null>(null);
  const [previewSelection, setPreviewSelection] = useState<CoordinatorPreviewSelection>({ plan: null, dirty: true, problem: 'Reading retained preview setup…', draftDigest: '' }), [recordedPreview, setRecordedPreview] = useState('');
  const previewChanged = useCallback((value: CoordinatorPreviewSelection) => setPreviewSelection(value), []);
  const [savedRoute, setSavedRoute] = useState<CoordinatorSavedRoute | null>(null), [rememberRoute, setRememberRoute] = useState(false), [autoDiscussFindings, setAutoDiscussFindings] = useState(false);
  const [findingConsent, setFindingConsent] = useState(false);
  const [cloudCapacity, setCloudCapacity] = useState<CoordinatorCloudCapacity | null>(null);
  const [checkSelection, setCheckSelection] = useState<CoordinatorCheckSelection>({ plan: null, dirty: true, problem: 'Reading retained check setup…', draftDigest: '' }), [recordedDraft, setRecordedDraft] = useState('');
  const checkChanged = useCallback((value: CoordinatorCheckSelection) => setCheckSelection(value), []);
  const [parallel, setParallel] = useState(2), [outcome, setOutcome] = useState<'preview' | 'publish'>('preview');
  const [executionLocation, setExecutionLocation] = useState<'local-docker' | 'cloud-microvm'>('local-docker');
  const worktreeId = useCockpit(state => state.selection.worktreeId), worktree = useCockpit(selectWorktreeById(worktreeId ?? ''));
  const alive = useRef(true), generation = useRef(0), working = useRef(false), readController = useRef<AbortController | null>(null);
  const latest = useRef({ canRead, canEdit, canExecute, canDeploy, blocked, view, worktree, acceptedReview }); latest.current = { canRead, canEdit, canExecute, canDeploy, blocked, view, worktree, acceptedReview };
  function persist(value: Pending | null) {
    try {
      if (storageError || localStorage.getItem(key) !== rawRef.current) throw new Error('Another window changed the retained continuation. Reopen this project before starting more work.');
      const raw = value ? JSON.stringify(value) : null;
      if (raw && new TextEncoder().encode(raw).length > MAX_PENDING_BYTES) throw new Error('The exact continuation request exceeds browser recovery limits. Reduce its commands before starting.');
      if (raw) localStorage.setItem(key, raw); else localStorage.removeItem(key);
      rawRef.current = raw; pendingRef.current = value; setPending(value);
    } catch (cause) { setStorageError(errorText(cause)); throw cause; }
  }
  const current = (ticket: number) => alive.current && latest.current.canRead && ticket === generation.current;
  function remember(value: CoordinatorContinuation) {
    setRows(previous => [value, ...previous.filter(row => row.id !== value.id)]); setSelected(value.id);
  }
  async function load(explicit: boolean) {
    if (!latest.current.canRead || working.current) return;
    readController.current?.abort();
    const controller = new AbortController(); readController.current = controller; const signal = controller.signal;
    const ticket = ++generation.current; setFresh(false); setConsent(false); setPreviewConsent(false); setDeploymentConsent(false); setDeploymentFresh(false); setDeliveryReadWarning(''); setBusy('read');
    try {
      const [nextPolicy, state] = await Promise.all([coordinatorPolicyApi.get(projectId, signal), coordinatorContinuationsApi.listState(projectId, actorId, signal)]);
      const nextRows = state.continuations;
      if (!current(ticket) || signal.aborted) return;
      // Optional delivery freshness is separate from the authoritative history
      // and policy reads. A removed target must not trap reconfiguration or
      // hide history that is still readable under the current project grant.
      let configuration: number | null = null, deployment: CoordinatorDeploymentPlan | null = null;
      const warnings: string[] = [];
      try { if (state.savedRoute?.previewPlan && latest.current.canDeploy) configuration = await (state.savedRoute.previewPlan.kind === 'reviewed-source' ? readCoordinatorSourceConfiguration(projectId, signal) : readCoordinatorConfiguration(projectId, signal)); }
      catch (cause) { warnings.push(`Saved preview configuration could not be refreshed: ${errorText(cause)}`); }
      if (!current(ticket) || signal.aborted) return;
      try { if (state.savedRoute?.deploymentPlan && latest.current.canEdit && latest.current.canDeploy && nextPolicy.deploymentMode === 'existing-target') deployment = await coordinatorDeploymentApi.prepare(projectId, state.savedRoute.deploymentPlan, signal); }
      catch (cause) { warnings.push(`Saved deployment configuration could not be refreshed: ${errorText(cause)}`); }
      if (!current(ticket) || signal.aborted) return;
      setDeliveryReadWarning(warnings.join(' '));
      const retained = pendingRef.current;
      if (explicit && retained) {
        const match = retained.kind === 'create' ? nextRows.find(row => row.idempotencyKey === retained.input.idempotencyKey && row.originTurnId === retained.input.originTurnId && row.contextDigest === retained.input.expectedContextDigest && row.policyRevision === retained.input.expectedPolicyRevision) : retained.kind === 'advance' ? nextRows.find(row => row.id === retained.id && row.revision > retained.revision && !row.actions.some(action => action.status === 'pending' || action.status === 'unknown')) : undefined;
        const policyObserved = retained.kind === 'policy' && nextPolicy.revision > retained.revision &&
          (retained.automaticRepairLimit === undefined || nextPolicy.automaticRepairLimit === retained.automaticRepairLimit);
        if (retained.kind === 'policy' && retained.automaticRepairLimit !== undefined && nextPolicy.revision > retained.revision && !policyObserved && !retained.rejected) {
          setNotice('The current saved repair limit differs from the retained request. Its outcome remains unconfirmed; the request was not submitted again.');
        }
        if (retained.rejected || match || policyObserved) {
          if (retained.kind === 'create' && contractReview?.runKey === retained.input.idempotencyKey) {
            archiveContractReview(reviewScope, contractReview); setContractReview(null); setContractView(null);
          }
          persist(null); setNotice(match ? 'The exact retained request was observed. It was not submitted again.' : 'Current authoritative state was read. The retained request was not replayed.');
        }
      }
      setPolicy(nextPolicy); setDeploymentFresh(!!deployment && !!state.savedRoute?.deploymentPlan && sameCoordinatorDeploymentBinding(deployment, state.savedRoute.deploymentPlan)); setConfigurationRevision(configuration); setSavedRoute(state.savedRoute); setCloudCapacity(state.cloudCapacity ?? null); setRows(nextRows); setContextCommits(state.contextCommits ?? []); setAcceptedReview(state.acceptedReview ?? null); setSelected(previous => nextRows.some(row => row.id === previous) ? previous : nextRows[nextRows.length - 1]?.id ?? '');
      setFresh(true); setReadDenied(false); setError('');
    } catch (cause) { if (current(ticket) && !signal?.aborted) { setError(errorText(cause)); if (denied(cause)) { setRows([]); setPolicy(null); setSavedRoute(null); setReadDenied(true); } } }
    finally { if (readController.current === controller) readController.current = null; if (current(ticket)) setBusy(''); }
  }
  useEffect(() => { alive.current = true; return () => { alive.current = false; generation.current++; readController.current?.abort(); }; }, []);
  useEffect(() => {
    generation.current++; setConsent(false); setIntegrationConsent(false); setPreviewConsent(false); setDeploymentConsent(false); setFindingConsent(false); setDeploymentFresh(false); setPicker(null); setFresh(false); setRememberRoute(false); setAutoDiscussFindings(false);
    if (!canRead) { setRows([]); setContextCommits([]); setAcceptedReview(null); setPolicy(null); setSavedRoute(null); setCloudCapacity(null); return; }
    void load(false);
    return () => readController.current?.abort();
  }, [projectId, actorId, canRead, canEdit, canExecute, canDeploy, view.context.digest, policyEpoch]);
  async function mutate<T>(label: string, record: Pending, operation: () => Promise<T>, accept: (value: T) => void) {
    const needsDeploy = record.kind === 'create' && (!!record.input.previewPlan || !!record.input.deploymentPlan) || record.kind === 'policy' && record.requiresDeploy;
    if (needsDeploy && !latest.current.canDeploy) return false;
    if (!fresh || !latest.current.canRead || !latest.current.canEdit || !latest.current.canExecute || working.current || pendingRef.current || storageError) return false;
    try { persist(record); } catch { return false; }
    working.current = true; const ticket = ++generation.current; setBusy(label); setConsent(false); setError('');
    try {
      const value = await operation();
      if (!current(ticket) || !latest.current.canEdit || !latest.current.canExecute || needsDeploy && !latest.current.canDeploy) return false;
      persist(null); accept(value); return true;
    } catch (cause) {
      if (current(ticket)) {
        if (refused(cause)) { try { persist({ ...record, rejected: true }); } catch { /* Keep uncertainty. */ } }
        setError(`${errorText(cause)} Read the exact retained outcome before another request; this action will not be replayed automatically.`);
      }
      return false;
    } finally { working.current = false; if (alive.current) setBusy(''); }
  }
  async function enableExecution(mode: 'manual' | 'reviewed-tasks' = 'reviewed-tasks') {
    if (!policy || mode === 'reviewed-tasks' && !consent || !latest.current.canExecute) return;
    await mutate('policy', { kind: 'policy', actorId, projectId, revision: policy.revision }, () => coordinatorPolicyApi.save(projectId, { expectedRevision: policy.revision, mode: policy.mode, allowedTargets: policy.allowedTargets, executionMode: mode }), value => { setPolicy(value); onPolicyUpdated?.(); setNotice(mode === 'manual' ? 'New automatic task continuations are paused. Already allocated tasks keep their current controls; they have not been cancelled.' : 'Reviewed-task continuation is enabled. Saved context approvals, current access and resource limits still apply. Deployment remains separately controlled.'); });
  }
  async function saveRepairLimit() {
    if (!policy || !validAutomaticRepairLimit(repairLimit) || repairLimit === effectiveAutomaticRepairLimit(policy.automaticRepairLimit)) return;
    await mutate('policy', { kind: 'policy', actorId, projectId, revision: policy.revision, automaticRepairLimit: repairLimit },
      () => coordinatorPolicyApi.save(projectId, { expectedRevision: policy.revision, mode: policy.mode, allowedTargets: policy.allowedTargets, automaticRepairLimit: repairLimit }),
      value => { setPolicy(value); onPolicyUpdated?.(); setNotice('The automatic repair limit is saved. Prepare and approve a fresh execution contract before new work. Earlier contracts and retained attempts were not changed or cancelled.'); });
  }
  async function saveFindingPreference(enabled: boolean) {
    if (!savedRoute || !fresh || !latest.current.canEdit || !latest.current.canExecute || working.current || pendingRef.current || storageError || enabled && !findingConsent) return;
    working.current = true; const ticket = ++generation.current; setBusy('policy'); setError('');
    try {
      const updated = await coordinatorContinuationsApi.setFindingPreference(projectId, savedRoute.revision, enabled);
      if (!current(ticket) || !latest.current.canEdit || !latest.current.canExecute) return;
      setSavedRoute(updated); setFindingConsent(false);
      setNotice(enabled ? 'Automatic finding discussion is enabled for this saved provider route while current project auto-draft policy permits it. Each turn remains a question or reviewable draft.' : 'New automatic finding discussions are paused. An already-started model turn may still need readback; no retained effect was cancelled.');
    } catch (cause) {
      if (current(ticket)) { setFresh(false); setError(`${errorText(cause)} Refresh the exact saved preference before another change; this request will not be replayed automatically.`); }
    } finally { working.current = false; if (alive.current) setBusy(''); }
  }
  async function saveIntegrationMode(mode: 'manual' | 'reviewed-scopes') {
    if (!policy || !latest.current.canExecute || mode === 'reviewed-scopes' && !integrationConsent) return;
    await mutate('policy', { kind: 'policy', actorId, projectId, revision: policy.revision }, () => coordinatorPolicyApi.save(projectId, { expectedRevision: policy.revision, mode: policy.mode, allowedTargets: policy.allowedTargets, integrationMode: mode }), value => { setPolicy(value); setIntegrationConsent(false); onPolicyUpdated?.(); setNotice(mode === 'manual' ? 'Automatic candidate integration is paused. Retained attempts have not been cancelled.' : 'Reviewed write-scope integration is enabled for future continuations. Luci still requires a stopped candidate, current authority and independent combined checks.'); });
  }
  async function savePreviewMode(mode: 'manual' | 'checked-source' | 'reviewed-source') {
    if (!policy || !latest.current.canDeploy || mode !== 'manual' && !previewConsent) return;
    await mutate('policy', { kind: 'policy', actorId, projectId, revision: policy.revision, requiresDeploy: true }, () => coordinatorPolicyApi.save(projectId, { expectedRevision: policy.revision, mode: policy.mode, allowedTargets: policy.allowedTargets, previewMode: mode }), value => { setPolicy(value); setPreviewConsent(false); onPolicyUpdated?.(); setNotice(mode === 'manual' ? 'Automatic previews are paused. Existing releases have not been stopped.' : mode === 'reviewed-source' ? 'Reviewed-source static preview permission is saved. Empty runtime configuration, current Deploy access and independent check evidence remain required.' : 'Dockerfile preview permission is saved. Exact configuration, current Deploy access and independent check evidence remain required. Public deployment is separate.'); });
  }
  async function saveDeploymentMode(mode: 'manual' | 'existing-target') {
    if (!policy || !latest.current.canDeploy || mode === 'existing-target' && !deploymentConsent) return;
    await mutate('policy', { kind: 'policy', actorId, projectId, revision: policy.revision, requiresDeploy: true }, () => coordinatorPolicyApi.save(projectId, { expectedRevision: policy.revision, mode: policy.mode, allowedTargets: policy.allowedTargets, deploymentMode: mode }), value => { setPolicy(value); setDeploymentConsent(false); setDeploymentFresh(false); onPolicyUpdated?.(); setNotice(mode === 'manual' ? 'New delegated deployments are paused. Existing releases and their costs have not been stopped.' : 'Existing-target deployment permission is saved. Repository publication, public exposure and deployment costs still require exact saved configuration and separate delegation.'); });
  }
  const ownTurn = [...view.turns].reverse().find(turn => turn.actor.id === actorId);
  const tasks = view.context.plan.tasks.filter(task => task.status !== 'done');
  const issue = blocked || !canEdit ? blocked || 'Editing access is required to continue implementation.' : !canExecute ? 'Execution access is required to continue implementation.' : !fresh ? 'Read current continuation and policy status first.' : !ownTurn || ownTurn.status !== 'succeeded' || !ownTurn.answer || ownTurn.answer.blocking || ownTurn.answer.proposal && !ownTurn.appliedRevision ? 'Complete and save this conversation’s amendment before implementation.' : view.blockingQuestions.length ? 'Resolve the material project questions first.' : view.context.specification.status !== 'approved' || view.context.designPlan.status !== 'approved' ? 'Review and approve the current definition and design first.' : view.context.plan.briefRevision !== view.context.brief.revision || view.context.plan.specificationRevision !== view.context.specification.revision ? 'Review the implementation plan against the current approved context.' : !tasks.length ? 'No unfinished saved tasks are selected for this amendment.' : !worktree?.isPrimary || worktree.projectId !== projectId ? 'Choose this project’s main checkout before configuring continued work.' : policy?.executionMode !== 'reviewed-tasks' ? 'Enable reviewed-task continuation before starting work.' : rows.some(row => row.originTurnId === ownTurn.id) ? 'This conversation already has a retained continuation. Inspect its recorded work instead of starting it again.' : '';
  const requirements = useMemo(() => {
    const ids = new Set(view.context.plan.tasks.filter(task => task.status !== 'done').flatMap(task => task.requirementIds ?? []));
    return view.context.specification.requirements.filter(row => ids.has(row.id)).map(({ id, title, acceptance }) => ({ id, title, acceptance })).sort((a, b) => a.id.localeCompare(b.id));
  }, [view.context.plan, view.context.specification]);
  const checkBase = JSON.stringify([view.context.specification.revision, view.context.designPlan.revision, view.context.plan.revision, requirements]);
  const savedIssue = !savedRoute ? '' : Object.keys(savedRoute).some(key => !['revision', 'route', 'maxParallel', 'updatedAt', 'checkPlan', 'previewPlan', 'deploymentPlan', 'autoDiscussFindings'].includes(key)) ? 'The saved route includes a newer delivery delegation. Use a compatible client before continuing it; its configuration has not been changed.' : savedRoute.deploymentPlan && (!canDeploy || policy?.deploymentMode !== 'existing-target') ? 'The saved deployment requires current Deploy access and existing-target permission.' : savedRoute.deploymentPlan && !deploymentFresh ? 'The saved deployment target, configuration or Data binding changed or could not be read. Review its current configuration before continuing.' : !savedRoute.checkPlan ? 'The saved route has no delegated independent checks. Configure exact commands and coverage before automatic continuation.' : savedRoute.previewPlan && (!canDeploy || policy?.previewMode !== (savedRoute.previewPlan.kind === 'reviewed-source' ? 'reviewed-source' : 'checked-source')) ? 'The saved preview plan requires current Deploy access and matching preview-kind permission.' : savedRoute.previewPlan && configurationRevision !== savedRoute.previewPlan.expectedConfigurationRevision ? 'Runtime configuration changed or could not be read. Delegate the current preview configuration before reusing this route.' : savedRoute.route.worktreeId !== worktree?.id ? 'The saved route belongs to a different checkout. Reconfigure it for this project’s main checkout.' : requirements.length !== savedRoute.checkPlan.requirementSnapshots.length || !requirements.every(item => savedRoute.checkPlan!.requirementSnapshots.some(prior => prior.id === item.id && prior.title === item.title && prior.acceptance === item.acceptance)) ? 'The selected requirements or their acceptance text changed. Review the commands and delegate their new coverage before reusing this route.' : '';
  const deliverySetupIssue = deploymentSelection.plan && (!checkSelection.plan || !previewSelection.plan) ? 'Delegate independent checks and a local preview before starting this deployment-enabled continuation.' : '';
  const row = rows.find(value => value.id === selected);
  const cloudReady = (cloudCapacity?.provider === 'e2b-embed' || cloudCapacity?.provider === 'azure-e2b-embed') && !cloudCapacity.setupNeeds?.length;
  const cloudProviderLabel = cloudCapacity?.provider === 'azure-e2b-embed' ? 'Azure microVM' : 'Configured microVM';
  async function reviewAcceptedChange() {
    const accepted = acceptedReview, route = savedRoute?.route;
    if (!accepted?.selection || accepted.status !== 'review-required' || !route || !fresh || !canEdit || !canExecute || pendingRef.current || contractReview || working.current || preparingReviewKey.current || storageError || readDenied) return;
    if (!['codex', 'claude-code', 'opencode'].includes(route.harness) || !['ask', 'bypass'].includes(route.permissionMode) || !worktree?.isPrimary || route.worktreeId !== worktree.id || worktree.projectId !== projectId) {
      setError('The saved route cannot prepare this changed contract. Review the current upstream connection and project checkout before continuing.'); return;
    }
    const key = crypto.randomUUID();
    const request: PrepareOrchestrationContract = { idempotencyKey: key, worktreeId: route.worktreeId, implementationContext: accepted.selection,
      route: { harness: route.harness as PrepareOrchestrationContract['route']['harness'], model: route.model, permissionMode: route.permissionMode as 'ask' | 'bypass', executionLocation: route.executionLocation === 'cloud-microvm' ? 'cloud-microvm' : 'local-docker', ...(route.effort ? { effort: route.effort } : {}), ...(route.agentConnectionId !== undefined ? { agentConnectionId: route.agentConnectionId } : {}) }, outcome: route.outcome };
    const review: ExecutionContractReviewRequest = { version: 1, projectId, actorId, runKey: key, request, acceptedEventId: accepted.eventId };
    try { retainContractReview(reviewScope, review); setContractReview(review); setContractView(null); }
    catch (cause) { setError(errorText(cause)); return; }
    const ticket = generation.current;
    preparingReviewKey.current = key; setPreparingReview(true);
    try {
      const value = await orchestrationContractsApi.prepare(projectId, request);
      const currentAccepted = latest.current.acceptedReview;
      if (!current(ticket) || !latest.current.canEdit || !latest.current.canExecute ||
          currentAccepted?.eventId !== accepted.eventId || currentAccepted.status !== 'review-required' ||
          JSON.stringify(currentAccepted.selection) !== JSON.stringify(accepted.selection)) return;
      // Another window or the review controls may have archived or replaced
      // the exact pending request while the effects-free prepare was in flight.
      // An absent key must never be recreated by a late response.
      const saved = readContractReview(reviewScope, projectId, actorId);
      if (!saved || saved.runKey !== key || saved.acceptedEventId !== accepted.eventId ||
          saved.contractId !== undefined || JSON.stringify(saved.request) !== JSON.stringify(request)) {
        if (!saved) { setContractReview(null); setContractView(null); }
        return;
      }
      if (value.contract.actor.id !== actorId) throw new Error('The prepared contract belongs to another person.');
      const retained = { ...review, contractId: value.contract.id };
      retainContractReview(reviewScope, retained); setContractReview(retained); setContractView(value);
    } catch (cause) { if (current(ticket)) setError(`${errorText(cause)} Inspect the exact retained changed-contract request; no replacement run was started.`); }
    finally { if (preparingReviewKey.current === key) { preparingReviewKey.current = null; if (alive.current) setPreparingReview(false); } }
  }
  const start = (location: 'local-docker' | 'cloud-microvm' = 'local-docker') => {
    if (issue || deliverySetupIssue || checkSelection.problem || previewSelection.problem || deploymentSelection.problem || pendingRef.current || contractReview || working.current || storageError || !policy || !ownTurn || !worktree) return;
    if (location === 'cloud-microvm' && !cloudReady) return;
    setExecutionLocation(location);
    setParallel(Math.min(2, tasks.length)); setOutcome(deploymentSelection.plan ? 'publish' : 'preview'); setRememberRoute(false);
    setPicker({ idempotencyKey: crypto.randomUUID(), originTurnId: ownTurn.id, expectedContextDigest: view.context.digest, expectedPolicyRevision: policy.revision, maxParallel: Math.min(2, tasks.length), run: { worktreeId: worktree.id, harness: '', model: '', permissionMode: 'ask', outcome: 'preview', implementationContext: { projectId, briefRevision: view.context.brief.revision, planRevision: view.context.plan.revision, specificationRevision: view.context.specification.revision, designPlanRevision: view.context.designPlan.revision, taskIds: tasks.map(task => task.id) } } });
  };
  async function create(selection: CreateSessionRequest) {
    if (!picker || issue || latest.current.blocked || picker.expectedContextDigest !== latest.current.view.context.digest || !selection.implementationContext || JSON.stringify(selection.implementationContext) !== JSON.stringify(picker.run.implementationContext)) return { ok: false as const, message: 'The reviewed selection changed. Close this picker and read the current context before continuing.' };
    if (!['codex', 'claude-code', 'opencode'].includes(selection.harness ?? '') || !['ask', 'bypass'].includes(selection.permissionMode ?? 'ask')) return { ok: false as const, message: 'Choose a supported harness and an explicit Ask or approved noninteractive mode.' };
    if (!Number.isInteger(parallel) || parallel < 1 || parallel > Math.min(MAX_SWARM_PARALLEL_TASKS, tasks.length)) return { ok: false as const, message: 'Choose a concurrency limit within the selected task count.' };
    if (executionLocation === 'cloud-microvm' && !cloudReady) return { ok: false as const, message: 'Cloud setup or spending advice is unavailable or blocked. Refresh and review the exact setup need before another request.' };
    if (executionLocation === 'cloud-microvm' && (!['codex', 'claude-code'].includes(selection.harness ?? '') || selection.permissionMode !== 'bypass' || !selection.agentConnectionId)) return { ok: false as const, message: 'Cloud microVM tasks need a reviewed Codex or Claude Code route, noninteractive permission and an explicit API-key model connection. The server will verify current cloud authority and capacity.' };
    if (checkSelection.problem || checkSelection.dirty && !checkSelection.plan) return { ok: false as const, message: checkSelection.problem || 'Review the exact check command delegation first.' };
    if (previewSelection.problem || previewSelection.dirty && !previewSelection.plan || previewSelection.plan && (!checkSelection.plan || !latest.current.canDeploy || policy?.previewMode !== (previewSelection.plan.kind === 'reviewed-source' ? 'reviewed-source' : 'checked-source'))) return { ok: false as const, message: previewSelection.problem || 'Delegate exact independent checks and matching current preview configuration before starting this preview-enabled continuation.' };
    if (deploymentSelection.problem || deploymentSelection.dirty && !deploymentSelection.plan || deploymentSelection.plan && (!checkSelection.plan || !previewSelection.plan || outcome !== 'publish' || !latest.current.canDeploy || policy?.deploymentMode !== 'existing-target')) return { ok: false as const, message: deploymentSelection.problem || 'Delegate exact checks, local preview and a published outcome before continuing to the selected deployment target.' };
    if (contractReview || pendingRef.current || storageError) return { ok: false as const, message: 'Inspect the retained execution review or continuation before preparing another.' };
    const input: CoordinatorContinuationRequest = { ...picker, rememberRoute, autoDiscussFindings: rememberRoute && autoDiscussFindings, ...(deploymentSelection.plan ? { deploymentPlan: deploymentSelection.plan } : {}), ...(previewSelection.plan ? { previewPlan: previewSelection.plan } : {}), ...(checkSelection.plan ? { checkPlan: checkSelection.plan } : {}), maxParallel: parallel, run: { ...picker.run, worktreeId: selection.worktreeId, harness: selection.harness ?? '', model: selection.model ?? '', permissionMode: selection.permissionMode ?? 'ask', ...(executionLocation === 'cloud-microvm' ? { executionLocation } : {}), ...(selection.effort ? { effort: selection.effort } : {}), ...(selection.agentConnectionId !== undefined ? { agentConnectionId: selection.agentConnectionId } : {}), outcome } };
    const request: PrepareOrchestrationContract = { idempotencyKey: crypto.randomUUID(), worktreeId: input.run.worktreeId, implementationContext: input.run.implementationContext,
      route: { harness: input.run.harness as PrepareOrchestrationContract['route']['harness'], model: input.run.model, permissionMode: input.run.permissionMode as 'ask' | 'bypass', executionLocation: input.run.executionLocation === 'cloud-microvm' ? 'cloud-microvm' : 'local-docker', ...(input.run.effort ? { effort: input.run.effort } : {}), ...(input.run.agentConnectionId !== undefined ? { agentConnectionId: input.run.agentConnectionId } : {}) }, outcome: input.run.outcome };
    const review: ExecutionContractReviewRequest = { version: 1, projectId, actorId, runKey: input.idempotencyKey, request, continuation: input };
    try { retainContractReview(reviewScope, review); setContractReview(review); setPicker(null); }
    catch (cause) { return { ok: false as const, message: errorText(cause) }; }
    try {
      const value = await orchestrationContractsApi.prepare(projectId, request);
      if (!alive.current || !latest.current.canRead) return { ok: true as const };
      if (value.contract.actor.id !== actorId) throw new Error('The prepared contract belongs to another person.');
      const retained = { ...review, contractId: value.contract.id };
      retainContractReview(reviewScope, retained); setContractReview(retained); setContractView(value);
    } catch (cause) { if (alive.current) setError(`${errorText(cause)} The exact preparation request is retained; refresh it before continuing.`); }
    return { ok: true as const };
  }
  async function createReviewedContinuation(prepared: PrepareOrchestrationContract, reference: OrchestrationContractReference, key: string) {
    const review = contractReview;
    if (!review?.continuation || review.runKey !== key || JSON.stringify(review.request) !== JSON.stringify(prepared) || pendingRef.current || !fresh || !latest.current.canEdit || !latest.current.canExecute) throw new Error('The reviewed continuation changed. Read its exact retained state.');
    const input: CoordinatorContinuationRequest = { ...review.continuation, run: { ...review.continuation.run, orchestrationContract: reference } };
    const ok = await mutate('create', { kind: 'create', projectId, actorId, input }, () => coordinatorContinuationsApi.create(projectId, actorId, input), value => { remember(value); if (input.checkPlan) setRecordedDraft(checkSelection.draftDigest); if (input.previewPlan) setRecordedPreview(previewSelection.draftDigest); if (input.deploymentPlan) setRecordedDeployment(deploymentSelection.draftDigest); setPicker(null); setNotice('The approved continuation is retained. Queued work is not yet verified or deployed.'); });
    if (!ok) throw new Error('The exact continuation needs readback. Do not submit another request.');
  }
  async function finishAcceptedReview(prepared: PrepareOrchestrationContract, reference: OrchestrationContractReference, key: string) {
    const review = contractReview;
    if (!review?.acceptedEventId || review.runKey !== key || JSON.stringify(review.request) !== JSON.stringify(prepared) || !reference.approvalId || !latest.current.canEdit || !latest.current.canExecute) throw new Error('The accepted change review changed. Read its exact retained approval.');
    const observed = await coordinatorContinuationsApi.listState(projectId, actorId);
    if (observed.acceptedReview?.eventId !== review.acceptedEventId || observed.acceptedReview.status !== 'approved') throw new Error('The accepted change or its current approval could not be confirmed. Keep this review for exact readback.');
    setAcceptedReview(observed.acceptedReview); setRows(observed.continuations); setContextCommits(observed.contextCommits ?? []);
    setNotice('The changed execution contract is approved. Luci will resume the accepted work only after current source, policy, authority and earlier outcomes are checked. Refresh continued work to inspect its retained result.');
  }
  async function advance() {
    if (!row || !canExecute || blocked) return;
    await mutate('advance', { kind: 'advance', actorId, projectId, id: row.id, revision: row.revision }, () => coordinatorContinuationsApi.advance(projectId, actorId, row.id, row.revision), remember);
  }
  if (!canRead) return null;
  return <section aria-label="Continued work" className="orchestrator-agent">
    <h3>Continue with Luci</h3>
    <p>Run on: {runOn}. Luci uses the selected upstream harness and the project’s reviewed tasks. Deploy to is chosen separately.</p>
    {deliveryReadWarning && <p role="status">{deliveryReadWarning} Automatic reuse remains blocked. Review current configuration or pause the delegation.</p>}{error && <p role="alert">{error}</p>}{storageError && <p role="alert">{storageError}</p>}{notice && <p role="status">{notice}</p>}
    {pending && <p role="status">A continuation request is retained. New requests are paused until its exact outcome is read back.</p>}
    {contractReview && !pending && <ExecutionContractReview key={contractReview.request.idempotencyKey} pending={contractReview} initialView={contractView} scope={reviewScope} actorId={actorId} canRead={canRead} canEdit={canEdit && canExecute && fresh && !readDenied} preparing={preparingReview && preparingReviewKey.current === contractReview.runKey} continuation={!contractReview.acceptedEventId} approvalOnly={!!contractReview.acceptedEventId} onPrepared={(next, value) => { setContractReview(next); setContractView(value); }} onCreate={contractReview.acceptedEventId ? finishAcceptedReview : createReviewedContinuation} onComplete={() => { setContractReview(null); setContractView(null); }} onReviewInputs={() => { generation.current++; setContractReview(null); setContractView(null); setPicker(null); }} />}
    {!readDenied && canRead && <section aria-label="Cloud task swarm activation">
      <h4>Build with a cloud agent swarm</h4>
      <p>{tasks.length} unfinished reviewed task{tasks.length === 1 ? '' : 's'} in this plan. Luci schedules saved dependencies and gives each admitted upstream agent its own owned microVM. Run on the selected cloud service; deploy to is a separate reviewed decision.</p>
      {!cloudCapacity ? <p role="status">Current cloud setup advice is unavailable. Refresh continued work before activation; no cloud task starts from this panel.</p> :
        cloudCapacity.setupNeeds?.length ? <ul role="status">{cloudCapacity.setupNeeds.map(need => <li key={need}>{need}</li>)}</ul> :
        <p role="status">{cloudProviderLabel} · {cloudCapacity.region} · {cloudCapacity.configuredAvailableSlots} configured slot{cloudCapacity.configuredAvailableSlots === 1 ? '' : 's'} now · up to {cloudCapacity.configuredProjectSlots} for this project. Estimated reserve per task ${(cloudCapacity.estimatedTaskCostMicros / 1_000_000).toFixed(2)} USD; retained this month ${(cloudCapacity.estimatedMonthMicros / 1_000_000).toFixed(2)} of ${(cloudCapacity.monthlyBudgetMicros / 1_000_000).toFixed(2)} USD. {cloudCapacity.limitation}</p>}
      {cloudCapacity?.retainedProjectVMs ? <details><summary>{cloudCapacity.retainedProjectVMs} retained task microVM{cloudCapacity.retainedProjectVMs === 1 ? '' : 's'} in this project</summary><p>These exact allocations remain charged until retirement is confirmed. Open the task queue to inspect, recover or request an owned stop; a stop is not proof of provider billing settlement.</p><ul>{cloudCapacity.retainedAttempts.map(attempt => <li key={attempt.operationId}>{attempt.taskId} · run {attempt.runId} · attempt {attempt.attemptId} · {attempt.state}{attempt.recoveryPending ? ' · restored identity readback pending; no replacement launch' : ''} · ${(attempt.estimatedCostMicros / 1_000_000).toFixed(2)} USD estimated · operation {attempt.operationId} <Button variant="secondary" disabled={!fresh} onClick={() => onInspect(attempt.runId)}>Open exact task controls</Button></li>)}</ul>{cloudCapacity.moreRetainedAttempts > 0 && <p>{cloudCapacity.moreRetainedAttempts} more retained attempts are not shown in this bounded summary. Open the project task queue for their exact identities.</p>}</details> : null}
      {!!cloudCapacity?.retiredBillingUnconfirmed && <p role="status">{cloudCapacity.retiredBillingUnconfirmed} retired microVM{cloudCapacity.retiredBillingUnconfirmed === 1 ? '' : 's'} this month have confirmed Luci allocation release but no independent provider billing settlement. Their estimated spend remains in the monthly budget; VM absence is not a bill receipt.</p>}
      <Button variant="primary" disabled={!cloudReady || !!issue || !!deliverySetupIssue || !!pending || !!contractReview || !!busy || !!storageError || !!checkSelection.problem || !!previewSelection.problem || !!deploymentSelection.problem} onClick={() => start('cloud-microvm')}>Review cloud swarm activation</Button>
      <p>Choose the existing Codex or Claude Code connection, exact task concurrency and checks in the review. Starting requires current project execution and spending authority. An uncertain request remains saved for exact readback.</p>
    </section>}
    {readDenied ? <p role="alert">Current access was refused. Retained private work is hidden.</p> : <>
      {rows.length > 0 && <label className="luci-field">Retained continuation<select value={selected} onChange={event => setSelected(event.target.value)}>{rows.map(value => <option value={value.id} key={value.id}>{states[value.status]} · {value.originTurnId}</option>)}</select></label>}
      {contextCommits.some(value => value.status === 'pending') && <p role="status">An accepted context Git commit has an uncertain outcome. Luci will inspect its exact parent, paths and bytes without replaying the write. Review this project’s source if it remains pending.</p>}
      {acceptedReview && <section aria-label="Accepted change continuation"><p role="status">{acceptedReview.reason}</p>{acceptedReview.status === 'review-required' && <Button variant="secondary" disabled={!canEdit || !canExecute || !fresh || !!busy || !!pending || !!contractReview || !!storageError || readDenied} onClick={() => void reviewAcceptedChange()}>Review changed execution contract</Button>}</section>}
      {row && <article aria-label="Selected continuation"><p role="status"><strong>{states[row.status]}</strong> · {row.message}</p>
        {!!row.acceptedEvents?.length && <p>Continued from {row.acceptedEvents.length} accepted project change{row.acceptedEvents.length === 1 ? '' : 's'}, through event {row.acceptedEvents[row.acceptedEvents.length - 1].sequence}. The saved input still requires current review and authority.</p>}
        {row.contextDigest !== view.context.digest && <p>The current context observation differs from this work’s captured snapshot. Luci rechecks the exact reviewed selection before continuing; it does not replace captured context.</p>}
        <ol>{row.actions.map(action => <li key={action.id}>{steps[action.kind]} · {action.status}{action.message ? ` — ${action.message}` : ''}</li>)}</ol>
        {row.queueIds.length > 0 && <p>{row.queueIds.length} queued task reservations. Completion, independent checks and release evidence remain separate.</p>}
        {row.delivery && <section aria-label="Independent check evidence"><p>{row.delivery.status} · {row.delivery.message}</p><p>Check {row.delivery.checkId} · source {row.delivery.commit.slice(0, 12)}. Preview, release and deployment remain separate.</p></section>}
        {row.delivery?.preview && <section aria-label="Checked-source preview"><p>{row.delivery.preview.status} · {row.delivery.preview.message}</p><p>Release {row.delivery.preview.releaseId}. This is a local preview, not public deployment or semantic acceptance.</p>{row.delivery.preview.status === 'ready' && <Button variant="secondary" onClick={() => { if (latest.current.canRead && !readDenied) onNavigate('deploy'); }}>Open Deploy</Button>}</section>}
        {row.delivery?.gitPublication && <section aria-label="Source publication"><p>{row.delivery.gitPublication.status} · {row.delivery.gitPublication.message}</p><p>{row.delivery.gitPublication.repository} · {row.delivery.gitPublication.branch} · {row.delivery.gitPublication.commit.slice(0, 12)}. Publishing source does not establish application deployment.</p></section>}
        {row.delivery?.deployment && <section aria-label="Application deployment"><p>{row.delivery.deployment.status} · {row.delivery.deployment.message}</p><p>Release {row.delivery.deployment.releaseId}. The recorded provider identity and HTTPS observation describe this checked source; they do not prove all product behavior.</p>{row.delivery.deployment.status === 'ready' && row.delivery.deployment.evidence && <a href={row.delivery.deployment.evidence.url} target="_blank" rel="noopener noreferrer">Open deployed application</a>}</section>}
        {row.runId && <Button variant="secondary" onClick={() => { if (latest.current.canRead && !readDenied) onInspect(row.runId!); }}>Inspect this build</Button>}
        {!['checks-verified', 'preview-ready', 'deployed'].includes(row.status) && <Button variant="secondary" disabled={!canEdit || !canExecute || !fresh || !!busy || !!pending || !!storageError || !!blocked} onClick={() => void advance()}>Continue retained work</Button>}
      </article>}
      {policy && policy.executionMode !== 'reviewed-tasks' && <details><summary>Implementation permission</summary><p>Allow Luci to prepare and queue reviewed tasks with your chosen upstream route. Current access, approvals, resource and provider limits still apply. This does not authorize deployment or change draft policy.</p><label className="orchestrator-ack"><input type="checkbox" checked={consent} disabled={!canEdit || !canExecute || !fresh || !!busy || !!pending} onChange={event => setConsent(event.target.checked)} />Allow reviewed-task continuation in this project.</label><Button variant="secondary" disabled={!consent || !canEdit || !canExecute || !fresh || !!busy || !!pending || !!storageError} onClick={() => void enableExecution()}>Enable reviewed-task continuation</Button></details>}
      {policy?.executionMode === 'reviewed-tasks' && <Button variant="tertiary" disabled={!canEdit || !canExecute || !fresh || !!busy || !!pending || !!storageError} onClick={() => void enableExecution('manual')}>Pause new continuations</Button>}
      {policy && <details><summary>Automatic repairs</summary>
        <label className="luci-field">Automatic repairs per coding task<select value={repairLimit} disabled={!canEdit || !canExecute || !fresh || !!busy || !!pending || !!storageError} onChange={event => setRepairLimit(Number(event.target.value))}>
          <option value={0}>0 · Stop after the first failed review</option><option value={1}>1 · Allow one repair</option><option value={2}>2 · Allow two repairs (default)</option>
        </select></label>
        <p>Caps repair reservations for each coding task in an approved run. Initial coding and independent review do not consume this budget; a repaired candidate still receives a final independent review. Cancelled or uncertain reservations are not refunded.</p>
        <p>Changing this policy requires a fresh contract review and approval. It does not authorize more work under an earlier contract.</p>
        <Button variant="secondary" disabled={!canEdit || !canExecute || !fresh || !!busy || !!pending || !!storageError || repairLimit === effectiveAutomaticRepairLimit(policy.automaticRepairLimit)} onClick={() => void saveRepairLimit()}>Save automatic repair limit</Button>
      </details>}
      {policy && <details><summary>Reviewed candidate integration</summary><p>Allow Luci to integrate a stopped upstream candidate only inside its saved nonempty task write paths. Failed or uncertain attempts, unanswered questions and changed source remain for explicit review. Independent checks on the combined source still run afterward.</p>{policy.integrationMode === 'reviewed-scopes' ? <Button variant="tertiary" disabled={!canEdit || !canExecute || !fresh || !!busy || !!pending || !!storageError} onClick={() => void saveIntegrationMode('manual')}>Pause automatic integration</Button> : <><label className="setup-check"><input type="checkbox" checked={integrationConsent} disabled={!canEdit || !canExecute || !fresh || !!busy || !!pending} onChange={event => setIntegrationConsent(event.target.checked)} />Allow reviewed-scope candidate integration for future continuations.</label><Button variant="secondary" disabled={!integrationConsent || !canEdit || !canExecute || !fresh || !!busy || !!pending || !!storageError} onClick={() => void saveIntegrationMode('reviewed-scopes')}>Enable reviewed integration</Button></>}</details>}
      {policy && <details><summary>Checked-source preview permission</summary><p>Choose static reviewed-source serving or a Dockerfile build from the exact independently checked commit. The two approvals are distinct. Current Deploy access and resource limits apply; public deployment remains separate.</p>{policy.previewMode === 'checked-source' || policy.previewMode === 'reviewed-source' ? <><p>Approved preview kind: {policy.previewMode === 'reviewed-source' ? 'static reviewed source' : 'Dockerfile build'}.</p><Button variant="tertiary" disabled={!canEdit || !canExecute || !canDeploy || !fresh || !!busy || !!pending || !!storageError} onClick={() => void savePreviewMode('manual')}>Pause automatic previews</Button></> : <><label className="setup-check"><input type="checkbox" checked={previewConsent} disabled={!canEdit || !canExecute || !canDeploy || !fresh || !!busy || !!pending} onChange={event => setPreviewConsent(event.target.checked)} />Allow checked-source local previews for this project.</label><Button variant="secondary" disabled={!previewConsent || !canEdit || !canExecute || !canDeploy || !fresh || !!busy || !!pending || !!storageError} onClick={() => void savePreviewMode('reviewed-source')}>Enable static reviewed-source previews</Button><Button variant="secondary" disabled={!previewConsent || !canEdit || !canExecute || !canDeploy || !fresh || !!busy || !!pending || !!storageError} onClick={() => void savePreviewMode('checked-source')}>Enable checked-source previews</Button></>}</details>}
      {policy && <details><summary>Existing-target deployment permission</summary><p>Allow Luci to continue checked, previewed work to an existing connected deployment server. It can publish the exact reviewed source and create or update the application under your saved configuration. It does not provision new servers or remove current permission and resource limits.</p>{policy.deploymentMode === 'existing-target' ? <Button variant="tertiary" disabled={!canEdit || !canExecute || !canDeploy || !fresh || !!busy || !!pending || !!storageError} onClick={() => void saveDeploymentMode('manual')}>Pause delegated deployments</Button> : <><label className="setup-check"><input type="checkbox" checked={deploymentConsent} disabled={!canEdit || !canExecute || !canDeploy || !fresh || !!busy || !!pending} onChange={event => setDeploymentConsent(event.target.checked)} />Allow existing-target deployment under an exact saved delivery plan.</label><Button variant="secondary" disabled={!deploymentConsent || !canEdit || !canExecute || !canDeploy || !fresh || !!busy || !!pending || !!storageError} onClick={() => void saveDeploymentMode('existing-target')}>Enable existing-target deployment</Button></>}</details>}
      {savedRoute && <p>Saved upstream route: {savedRoute.route.harness} · {savedRoute.route.model || 'configured model'} · up to {savedRoute.maxParallel} tasks. Luci reuses only this exact route and check delegation for your later approved amendments while reviewed-task permission remains enabled.</p>}
      {savedRoute && (savedRoute.autoDiscussFindings ? <Button variant="tertiary" disabled={!canEdit || !canExecute || !fresh || !!busy || !!pending || !!storageError} onClick={() => void saveFindingPreference(false)}>Pause automatic finding discussions</Button> : <><label className="setup-check"><input type="checkbox" checked={findingConsent} disabled={!canEdit || !canExecute || !fresh || !!busy || !!pending} onChange={event => setFindingConsent(event.target.checked)} />Allow one bounded provider turn per new recorded finding using my saved route. Model charges may apply. Current project auto-draft policy and permissions still control dispatch; results remain questions or drafts.</label><Button variant="secondary" disabled={!findingConsent || !canEdit || !canExecute || !fresh || !!busy || !!pending || !!storageError} onClick={() => void saveFindingPreference(true)}>Enable automatic finding discussions</Button></>)}
      {savedIssue && <p role="status">{savedIssue}</p>}
      <CoordinatorCheckSetup projectId={projectId} scope={scope} base={checkBase} requirements={requirements} disabled={!canEdit || !canExecute || !fresh || !!busy || !!pending || !!storageError || !!blocked} recordedDraft={recordedDraft} onChange={checkChanged} />
      <CoordinatorPreviewSetup projectId={projectId} scope={scope} enabled={policy?.previewMode === 'checked-source' || policy?.previewMode === 'reviewed-source'} mode={policy?.previewMode === 'reviewed-source' ? 'reviewed-source' : 'checked-source'} disabled={!canEdit || !canExecute || !canDeploy || !fresh || !!busy || !!pending || !!storageError || !!blocked} recordedDraft={recordedPreview} onChange={previewChanged} />
      <CoordinatorDeploymentSetup projectId={projectId} scope={scope} enabled={policy?.deploymentMode === 'existing-target'} disabled={!canEdit || !canExecute || !canDeploy || !fresh || !!busy || !!pending || !!storageError || !!blocked} recordedDraft={recordedDeployment} onChange={deploymentChanged} onNavigateDeploy={() => onNavigate('deploy')} />
      {issue && <p>{issue}</p>}{deliverySetupIssue && <p role="status">{deliverySetupIssue}</p>}
      <Button variant="primary" disabled={!!issue || !!deliverySetupIssue || !!pending || !!contractReview || !!busy || !!storageError || !!checkSelection.problem || !!previewSelection.problem || !!deploymentSelection.problem} onClick={() => start()}>Configure continued work</Button>
      {(view.context.specification.status !== 'approved' || view.context.designPlan.status !== 'approved') && <Button variant="ghost" onClick={() => onNavigate(view.context.specification.status !== 'approved' ? 'discover' : 'design')}>Review current context</Button>}
    </>}
    <Button variant="ghost" disabled={!!busy} onClick={() => void load(true)}>Refresh continued work</Button>
    {busy && <p role="status">{busy === 'read' ? 'Reading retained continuation and current policy…' : busy === 'policy' ? 'Saving implementation permission…' : 'Continuing exact reviewed work…'}</p>}
    <SessionPicker open={!!picker && canRead && canEdit && canExecute && !readDenied} worktreeId={picker?.run.worktreeId ?? null} implementationContext={picker?.run.implementationContext} onClose={() => setPicker(null)} onSubmit={create} submitLabel="Review execution contract" progressLabel="Preparing the exact contract for review. No coding resource is allocated by this step." failureGuidance="Inspect the retained contract preparation before choosing a new route." onCreated={() => setPicker(null)}>
      <label className="luci-field">Develop execution location<select value={executionLocation} onChange={event => setExecutionLocation(event.target.value as typeof executionLocation)}><option value="local-docker">This computer · isolated Docker</option><option value="cloud-microvm">Configured cloud · one isolated microVM per task</option></select></label>
      <p>{executionLocation === 'cloud-microvm' ? 'Cloud tasks require an operator-configured microVM service, current cloud provisioning and spending permission, and an explicit upstream API-key connection. Each VM remains charged until exact retirement is observed. Luci will refuse an unavailable cloud route rather than fall back to Docker.' : `Run on: ${runOn}.`} This starts only the selected saved tasks; successful deployment is not implied.</p>
      {executionLocation === 'cloud-microvm' && (cloudReady ? <p role="status">Configured {cloudProviderLabel} in {cloudCapacity.region}: {cloudCapacity.configuredAvailableSlots} task VM slots presently available under Luci limits (global {cloudCapacity.configuredGlobalSlots}, project {cloudCapacity.configuredProjectSlots}). Configured cost estimate per task ${(cloudCapacity.estimatedTaskCostMicros / 1_000_000).toFixed(2)} USD for up to {cloudCapacity.maxTaskSeconds} seconds; your current-month retained estimate is ${(cloudCapacity.estimatedMonthMicros / 1_000_000).toFixed(2)} of ${(cloudCapacity.monthlyBudgetMicros / 1_000_000).toFixed(2)} USD. {cloudCapacity.limitation}</p> : <p role="status">Cloud work cannot start while the setup or policy needs above remain. For an unconnected workspace, a workspace administrator can use Setup → Bring your own cloud to prepare and verify an Azure connection; Luci Cloud is not available yet.</p>)}
      <label className="luci-field">Maximum parallel tasks<input type="number" min={1} max={Math.min(MAX_SWARM_PARALLEL_TASKS, picker?.run.implementationContext.taskIds.length ?? 1)} value={parallel} onChange={event => setParallel(Number(event.target.value))} /></label>
      <label className="luci-field">Intended delivery outcome<select value={outcome} onChange={event => setOutcome(event.target.value as typeof outcome)}><option value="preview">Working preview</option><option value="publish">Published application</option></select></label>
      <label className="setup-check"><input type="checkbox" checked={rememberRoute} onChange={event => { setRememberRoute(event.target.checked); if (!event.target.checked) setAutoDiscussFindings(false); }} />Reuse this upstream route and exact check delegation for my later approved amendments under the current policy.</label>
      {rememberRoute && <label className="setup-check"><input type="checkbox" checked={autoDiscussFindings} onChange={event => setAutoDiscussFindings(event.target.checked)} />Allow Luci to use my saved provider and model for one bounded conversation turn per new recorded finding. Provider charges may apply when project auto-draft policy is enabled. A turn can ask a question or propose a draft; it cannot approve, execute or deploy by itself.</label>}
      {!checkSelection.plan && <p>No independent check plan is delegated. Work will need check configuration after task integration.</p>}
      {deploymentSelection.plan && <p>Deploy to {deploymentSelection.plan.targetId} at {deploymentSelection.plan.domain}, after the exact delegated checks and local preview. The separately reviewed repository push, public exposure and ongoing costs apply.</p>}
      <p>Choosing an outcome alone does not authorize deployment or bypass checks. Only the exact configuration and explicit delivery delegations travel with this request.</p>
    </SessionPicker>
  </section>;
}
