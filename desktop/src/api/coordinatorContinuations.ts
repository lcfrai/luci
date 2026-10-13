import { productApi, type CheckCommand } from './product';
import { coordinatorDeploymentPlanValid, type CoordinatorDeploymentPlan } from './coordinatorDeployment';
import { MAX_SWARM_PARALLEL_TASKS } from './swarmLimits';
import type { CreateProjectRun, ProjectRun } from './projectRuns';
import type { OrchestratorView } from './orchestrator';

export type CoordinatorActionKind = 'create-run' | 'prepare-tasks' | 'queue-tasks';
export interface CoordinatorContinuation {
  id: string; projectId: string; actor: { id: string; displayName: string }; idempotencyKey: string;
  originTurnId: string; acceptedEvents?: { eventId: string; sequence: number; originIntentId: string; archiveDigest?: string }[]; contextDigest: string; policyRevision: number; revision: number;
  status: 'ready' | 'continuing' | 'queued' | 'checking' | 'checks-verified' | 'unknown' | 'refused' | 'blocked' | 'previewing' | 'preview-ready' | 'preview-blocked' | 'deploying' | 'deployed' | 'deployment-blocked'; message: string;
  runId?: string; queueIds: string[];
  actions: { id: string; kind: CoordinatorActionKind; status: 'pending' | 'confirmed' | 'unknown' | 'refused'; referenceId?: string; message?: string; startedAt: string; finishedAt?: string }[];
  delivery?: { actionId: string; checkId: string; status: 'pending' | 'running' | 'verified' | 'unknown' | 'blocked'; commit: string; sourceDigest: string; integrationDigest: string; message: string; startedAt: string; updatedAt: string; preview?: { actionId: string; releaseId: string; status: 'pending' | 'running' | 'ready' | 'unknown' | 'blocked'; message: string; startedAt: string; updatedAt: string; evidence?: ProjectRun['releases'][number] & { projectId: string } }; gitPublication?: { actionId: string; status: 'pending' | 'unknown' | 'verified' | 'blocked'; repository: string; branch: string; commit: string; startedAt: string; updatedAt: string; message: string }; deployment?: { actionId: string; releaseId: string; status: 'pending' | 'running' | 'ready' | 'unknown' | 'blocked'; message: string; startedAt: string; updatedAt: string; evidence?: ProjectRun['releases'][number] & { projectId: string } } };
  createdAt: string; updatedAt: string; nextAction?: OrchestratorView['nextAction'];
}
export interface CoordinatorCheckPlan {
  commands: CheckCommand[];
  requirementIds: string[];
  requirementSnapshots: { id: string; title: string; acceptance: string }[];
  acknowledgeProjectCommands: true;
}
export interface CoordinatorPreviewPlan { dockerfile: string; port: number; expectedConfigurationRevision: number; acknowledgePreviewBuild: true }
export interface CoordinatorSavedRoute {
  revision: number;
  route: Omit<CreateProjectRun, 'idempotencyKey' | 'implementationContext'>;
  maxParallel: number; updatedAt: string; checkPlan?: CoordinatorCheckPlan; previewPlan?: CoordinatorPreviewPlan; deploymentPlan?: CoordinatorDeploymentPlan;
}
export interface CoordinatorContextCommitView { eventId: string; status: 'pending' | 'confirmed'; commit?: string; createdAt: string }
export interface CoordinatorCloudCapacity {
  provider: 'e2b-embed'; region: string; configuredGlobalSlots: number; configuredProjectSlots: number;
  configuredAvailableSlots: number; estimatedTaskCostMicros: number; estimatedMonthMicros: number;
  monthlyBudgetMicros: number; maxTaskSeconds: number; observedAt: string; limitation: string;
}
export interface CoordinatorContinuationState { continuations: CoordinatorContinuation[]; savedRoute: CoordinatorSavedRoute | null; contextCommits?: CoordinatorContextCommitView[]; cloudCapacity?: CoordinatorCloudCapacity | null }
export interface CoordinatorContinuationRequest {
  idempotencyKey: string; originTurnId: string; expectedContextDigest: string; expectedPolicyRevision: number;
  run: Omit<CreateProjectRun, 'idempotencyKey'>; maxParallel: number;
  rememberRoute?: boolean; expectedRouteRevision?: number; checkPlan?: CoordinatorCheckPlan; previewPlan?: CoordinatorPreviewPlan; deploymentPlan?: CoordinatorDeploymentPlan;
}
const base = (project: string) => `/v1/product/projects/${encodeURIComponent(project)}/orchestrator/continuations`;
const kinds: CoordinatorActionKind[] = ['create-run', 'prepare-tasks', 'queue-tasks'];
const text = (value: unknown): value is string => typeof value === 'string' && value.length > 0;
function verified(value: CoordinatorContinuation, project: string, actor: string, id?: string) {
  if (!value || value.projectId !== project || value.actor?.id !== actor || !text(value.id) || (id && value.id !== id) ||
      !text(value.idempotencyKey) || !text(value.originTurnId) || !text(value.contextDigest) ||
      value.acceptedEvents !== undefined && (!Array.isArray(value.acceptedEvents) || value.acceptedEvents.length > 16 || value.acceptedEvents.some((item, index) => !text(item.eventId) || !text(item.originIntentId) || !Number.isSafeInteger(item.sequence) || item.sequence < 1 || index > 0 && value.acceptedEvents![index - 1].sequence >= item.sequence)) ||
      !Number.isSafeInteger(value.revision) || value.revision < 1 || !Number.isSafeInteger(value.policyRevision) || value.policyRevision < 1 ||
      !['ready', 'continuing', 'queued', 'checking', 'checks-verified', 'unknown', 'refused', 'blocked', 'previewing', 'preview-ready', 'preview-blocked', 'deploying', 'deployed', 'deployment-blocked'].includes(value.status) || typeof value.message !== 'string' ||
      !Array.isArray(value.queueIds) || value.queueIds.some(item => !text(item)) || new Set(value.queueIds).size !== value.queueIds.length ||
      !Array.isArray(value.actions) || value.actions.length > kinds.length || value.actions.some((action, index) =>
        action.kind !== kinds[index] || action.id !== `${value.id}-${action.kind}` || !['pending', 'confirmed', 'unknown', 'refused'].includes(action.status) || !text(action.startedAt)) ||
      ['checks-verified', 'previewing', 'preview-ready', 'preview-blocked', 'deploying', 'deployed', 'deployment-blocked'].includes(value.status) && value.delivery?.status !== 'verified' ||
      value.status === 'preview-ready' && value.delivery?.preview?.status !== 'ready' ||
      value.status === 'deployed' && value.delivery?.deployment?.status !== 'ready' ||
      ['deploying', 'deployed', 'deployment-blocked'].includes(value.status) && !value.delivery?.deployment ||
      value.delivery?.gitPublication && !gitVerified(value.delivery.gitPublication, value) ||
      value.delivery?.deployment && !deploymentVerified(value.delivery.deployment, value, project) ||
      value.delivery?.preview && !previewVerified(value.delivery.preview, value, project) ||
      value.delivery && (value.delivery.actionId !== `${value.id}-checks` || !text(value.delivery.checkId) || !['pending', 'running', 'verified', 'unknown', 'blocked'].includes(value.delivery.status) || !/^[0-9a-f]{40,64}$/.test(value.delivery.commit) || !/^[0-9a-f]{64}$/.test(value.delivery.sourceDigest) || !/^[0-9a-f]{64}$/.test(value.delivery.integrationDigest) || typeof value.delivery.message !== 'string') ||
      !Number.isFinite(Date.parse(value.createdAt)) || !Number.isFinite(Date.parse(value.updatedAt)) ||
      value.nextAction && (!['discover', 'design', 'develop', 'deploy'].includes(value.nextAction.lens) || !text(value.nextAction.label) || typeof value.nextAction.reason !== 'string')) {
    throw new Error('The continuation response could not be verified for this person and project. Refresh its exact outcome before continuing.');
  }
  return value;
}
function previewVerified(preview: NonNullable<NonNullable<CoordinatorContinuation['delivery']>['preview']>, row: CoordinatorContinuation, project: string) {
  if (preview.actionId !== `${row.id}-preview` || !/^rel-[a-zA-Z0-9_-]+$/.test(preview.releaseId) || !['pending', 'running', 'ready', 'unknown', 'blocked'].includes(preview.status) || typeof preview.message !== 'string' || !Number.isFinite(Date.parse(preview.startedAt)) || !Number.isFinite(Date.parse(preview.updatedAt))) return false;
  const evidence = preview.evidence, delivery = row.delivery;
  if (!delivery || delivery.status !== 'verified') return false;
  if (preview.status === 'ready' && !evidence) return false;
  return !evidence || (evidence.projectId === project && evidence.releaseId === preview.releaseId && evidence.checkId === delivery.checkId && evidence.commit === delivery.commit && evidence.sourceDigest === delivery.sourceDigest && evidence.target === 'local' && evidence.outcome === 'preview' && Number.isFinite(Date.parse(evidence.observedAt)));
}
function gitVerified(action: NonNullable<NonNullable<CoordinatorContinuation['delivery']>['gitPublication']>, row: CoordinatorContinuation) {
  return row.delivery?.status === 'verified' && action.actionId === `${row.id}-git` && ['pending', 'unknown', 'verified', 'blocked'].includes(action.status) &&
    action.commit === row.delivery.commit && httpsAddress(action.repository) && text(action.branch) && typeof action.message === 'string' && Number.isFinite(Date.parse(action.startedAt)) && Number.isFinite(Date.parse(action.updatedAt));
}
function httpsAddress(value: unknown): value is string {
  if (typeof value !== 'string') return false;
  try { const url = new URL(value); return url.protocol === 'https:' && !!url.hostname && !url.username && !url.password && !url.hash; } catch { return false; }
}
function deploymentVerified(action: NonNullable<NonNullable<CoordinatorContinuation['delivery']>['deployment']>, row: CoordinatorContinuation, project: string) {
  const delivery = row.delivery, evidence = action.evidence;
  if (!delivery || delivery.status !== 'verified' || delivery.preview?.status !== 'ready' || delivery.gitPublication?.status !== 'verified' ||
      action.actionId !== `${row.id}-deploy` || !/^cloud-rel-[a-zA-Z0-9_-]+$/.test(action.releaseId) || !['pending', 'running', 'ready', 'unknown', 'blocked'].includes(action.status) || typeof action.message !== 'string' || !Number.isFinite(Date.parse(action.startedAt)) || !Number.isFinite(Date.parse(action.updatedAt))) return false;
  if (action.status === 'ready' && !evidence) return false;
  return !evidence || (evidence.projectId === project && evidence.releaseId === action.releaseId && evidence.checkId === delivery.checkId && evidence.commit === delivery.commit && evidence.sourceDigest === delivery.sourceDigest && text(evidence.target) && evidence.target !== 'local' && evidence.outcome === 'publish' && httpsAddress(evidence.url) && Number.isFinite(Date.parse(evidence.observedAt)));
}
export function coordinatorPreviewPlanValid(value: unknown): value is CoordinatorPreviewPlan {
  const plan = value as CoordinatorPreviewPlan;
  return !!plan && plan.acknowledgePreviewBuild === true && typeof plan.dockerfile === 'string' && !!plan.dockerfile.trim() && new TextEncoder().encode(plan.dockerfile).length <= 1024 && !/[\\:\0]/.test(plan.dockerfile) && !plan.dockerfile.startsWith('/') && !plan.dockerfile.split('/').some(part => part === '..' || part === '.') && Number.isSafeInteger(plan.port) && plan.port > 0 && plan.port <= 65535 && Number.isSafeInteger(plan.expectedConfigurationRevision) && plan.expectedConfigurationRevision >= 0;
}
const bytes = (value: string) => new TextEncoder().encode(value).length;
/** Same command bounds as the production check API; this does not confer execution authority. */
export function coordinatorCheckCommandsValid(commands: unknown): commands is CheckCommand[] {
  return Array.isArray(commands) && commands.length > 0 && commands.length <= 8 && commands.every(command => command &&
    typeof command.label === 'string' && !!command.label.trim() && bytes(command.label) <= 200 &&
    typeof command.executable === 'string' && /^[a-zA-Z0-9][a-zA-Z0-9_.+-]{0,79}$/.test(command.executable) &&
    Array.isArray(command.args) && command.args.length <= 100 && command.args.every((arg: unknown) => typeof arg === 'string' && !arg.includes('\0')) &&
    command.args.reduce((total: number, arg: string) => total + bytes(arg), 0) <= 32768);
}
function checkPlanValid(value: CoordinatorCheckPlan) {
  return value && value.acknowledgeProjectCommands === true && coordinatorCheckCommandsValid(value.commands) &&
    Array.isArray(value.requirementIds) && value.requirementIds.length > 0 && value.requirementIds.length <= 1000 &&
    new Set(value.requirementIds).size === value.requirementIds.length && value.requirementIds.every(id => text(id)) &&
    Array.isArray(value.requirementSnapshots) && value.requirementSnapshots.length === value.requirementIds.length &&
    value.requirementSnapshots.every((row, index) => row?.id === value.requirementIds[index] && typeof row.title === 'string' && bytes(row.title) <= 2000 && typeof row.acceptance === 'string' && bytes(row.acceptance) <= 8000);
}
function savedRoute(value: CoordinatorSavedRoute | null | undefined) {
  if (value == null) return null;
  const route = value.route;
  if (!Number.isSafeInteger(value.revision) || value.revision < 1 || !Number.isSafeInteger(value.maxParallel) || value.maxParallel < 1 || value.maxParallel > MAX_SWARM_PARALLEL_TASKS ||
      !Number.isFinite(Date.parse(value.updatedAt)) || !route || Object.keys(route).some(key => !['worktreeId', 'harness', 'model', 'permissionMode', 'effort', 'executionLocation', 'agentConnectionId', 'outcome'].includes(key)) || !text(route.worktreeId) || !text(route.harness) || typeof route.model !== 'string' ||
      !['ask', 'bypass', 'plan'].includes(route.permissionMode) || !['preview', 'publish'].includes(route.outcome) ||
      route.executionLocation !== undefined && (route.executionLocation !== 'cloud-microvm' || route.permissionMode !== 'bypass' || !['codex', 'claude-code'].includes(route.harness)) ||
      route.effort !== undefined && typeof route.effort !== 'string' || route.agentConnectionId !== undefined && typeof route.agentConnectionId !== 'string' ||
      value.checkPlan !== undefined && !checkPlanValid(value.checkPlan) || value.previewPlan !== undefined && (!value.checkPlan || !coordinatorPreviewPlanValid(value.previewPlan)) ||
      value.deploymentPlan !== undefined && (!value.checkPlan || !value.previewPlan || route.outcome !== 'publish' || !coordinatorDeploymentPlanValid(value.deploymentPlan))) throw new Error('The saved continuation route or check delegation could not be verified. Reconfigure it before continuing.');
  return value;
}
async function listState(project: string, actor: string, signal?: AbortSignal): Promise<CoordinatorContinuationState> {
  const result = await productApi.request<CoordinatorContinuationState>(base(project), undefined, 'GET', 30_000, signal);
  if (!Array.isArray(result?.continuations) || result.continuations.length > 40 || new Set(result.continuations.map(row => row.id)).size !== result.continuations.length || new Set(result.continuations.map(row => row.idempotencyKey)).size !== result.continuations.length) throw new Error('The continuation history could not be verified. Refresh before continuing.');
  if (result.contextCommits !== undefined && (!Array.isArray(result.contextCommits) || result.contextCommits.length > 40 || result.contextCommits.some(item => !text(item.eventId) || !['pending', 'confirmed'].includes(item.status) || item.status === 'confirmed' && !/^[0-9a-f]{40,64}$/.test(item.commit ?? '') || !Number.isFinite(Date.parse(item.createdAt))))) throw new Error('The accepted context source history could not be verified. Refresh before continuing.');
  const capacity = result.cloudCapacity;
  if (capacity != null && (capacity.provider !== 'e2b-embed' || !text(capacity.region) || !text(capacity.limitation) || !Number.isFinite(Date.parse(capacity.observedAt)) ||
    ![capacity.configuredGlobalSlots, capacity.configuredProjectSlots, capacity.configuredAvailableSlots, capacity.maxTaskSeconds].every(value => Number.isSafeInteger(value) && value >= 0) ||
    ![capacity.estimatedTaskCostMicros, capacity.estimatedMonthMicros, capacity.monthlyBudgetMicros].every(value => Number.isSafeInteger(value) && value >= 0))) throw new Error('The cloud task capacity observation could not be verified. Refresh before choosing cloud execution.');
  return { continuations: result.continuations.map(row => verified(row, project, actor)), savedRoute: savedRoute(result.savedRoute), contextCommits: result.contextCommits ?? [], cloudCapacity: capacity ?? null };
}
export const coordinatorContinuationsApi = {
  listState,
  async list(project: string, actor: string, signal?: AbortSignal) { return (await listState(project, actor, signal)).continuations; },
  async get(project: string, actor: string, id: string, signal?: AbortSignal) {
    return verified(await productApi.request<CoordinatorContinuation>(`${base(project)}/${encodeURIComponent(id)}`, undefined, 'GET', 30_000, signal), project, actor, id);
  },
  async create(project: string, actor: string, input: CoordinatorContinuationRequest) {
    const value = verified(await productApi.request<CoordinatorContinuation>(base(project), input), project, actor);
    if (value.idempotencyKey !== input.idempotencyKey || value.originTurnId !== input.originTurnId || value.contextDigest !== input.expectedContextDigest || value.policyRevision !== input.expectedPolicyRevision) throw new Error('The returned continuation did not match this exact request. Read its retained outcome before another start.');
    return value;
  },
  async advance(project: string, actor: string, id: string, expectedRevision: number) {
    const value = verified(await productApi.request<CoordinatorContinuation>(`${base(project)}/${encodeURIComponent(id)}/advance`, { expectedRevision }), project, actor, id);
    if (value.revision < expectedRevision) throw new Error('The continuation returned an older revision. Refresh before continuing.');
    return value;
  },
};
