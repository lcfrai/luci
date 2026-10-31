import { productApi } from './product';
import { validRunAttention, type ProjectRunAttention } from './runAttention';
import type { OrchestratorActivity } from './orchestrator';

export interface ProjectOutcomeMeasurements {
  version:1; scope:'retained-run-history'; measuredTaskIntervals:number; unknownTaskIntervals:number;
  recordedNoWorkerAttempts:number; taskRuntimeMilliseconds:number|null; runToAcceptanceMilliseconds:number|null;
  acceptedIntervalsMeasured:number; acceptedIntervalsUnknown:number; recordedHumanActions:number|null;
  omittedHumanActions:number; runsWithEarlierUninstrumentedActions:number; humanActionScope:'recorded-run-controls';
  inputTokens:null; outputTokens:null;
}
export interface ProjectOutcomes {
  measurements?:ProjectOutcomeMeasurements;
  status: 'recorded' | 'unavailable'; scope: 'retained-run-history'; observedAt: string;
  retainedRuns: number | null; recordedAcceptances: number | null; unconfirmedAcceptedObservations: number | null;
  reviewRequiredRuns: number | null; unresolvedCheckRepairTasks: number | null; omittedRuns: number | null;
  latestAccepted: { runId: string; checkId: string; releaseId: string; acceptedAt: string; commit: string; target: string } | null;
}

export interface ProjectOverview {
  projectId: string; observedAt: string; status: 'ready' | 'partial' | 'unavailable';
  goal: { text: string; revision: number; updatedAt: string; truncated: boolean } | null;
  phase: { id: string; label: string };
  context: { briefRevision: number; specificationRevision: number; definitionStatus: string; designRevision: number; designStatus: string; planRevision: number; tasks: { total: number; recordedDone: number } } | null;
  questions: { status: 'recorded' | 'partial' | 'unavailable'; count: number | null; items: { id: string; source: 'definition' | 'orchestrator'; text: string; truncated: boolean }[]; omitted: number | null };
  execution: { scope: 'swarm-queue'; status: 'recorded' | 'unavailable'; observedAt: string; queued: number | null; paused: number | null; reserved: number | null; running: number | null; blocked: number | null; unknown: number | null; failed: number | null; dispatchPaused: boolean | null };
  nextAction: { id: string; label: string; lens: string; runId?: string; reason?: string };
  latestRun?: { status: 'recorded' | 'none' | 'unavailable'; id?: string; state?: string; updatedAt?: string; contextStatus?: 'current' | 'changed' | 'unavailable'; contextReason?: string };
  attention?: { status: 'recorded' | 'unavailable'; count: number | null; items: ProjectRunAttention[]; omitted: number | null };
  sharedChange?: { status: 'recorded' | 'none' | 'unavailable'; event?: OrchestratorActivity['accepted'][number] };
  privateFinding?: { status: 'recorded' | 'none' | 'unavailable'; id?: string; sourceStatus?: string; state?: string; updatedAt?: string };
  limitations: string[];
  outcomes?: ProjectOutcomes;
}
const count = (value: unknown) => value === null || (Number.isSafeInteger(value) && (value as number) >= 0);
export function validProjectOutcomes(value: ProjectOutcomes | undefined): boolean {
  if (value === undefined) return true; // Older services do not supply this optional projection.
  if (!value || value.scope !== 'retained-run-history' || typeof value.observedAt !== 'string' || !Number.isFinite(Date.parse(value.observedAt))) return false;
  const counts = [value.retainedRuns, value.recordedAcceptances, value.unconfirmedAcceptedObservations, value.reviewRequiredRuns, value.unresolvedCheckRepairTasks, value.omittedRuns];
  if (value.status === 'unavailable') return counts.every(item => item === null) && value.latestAccepted === null && value.measurements === undefined;
  if (value.status !== 'recorded' || counts.some(item => !count(item) || item === null) ||
      value.retainedRuns! > 200 || value.recordedAcceptances! > value.retainedRuns! || value.reviewRequiredRuns! > value.retainedRuns! ||
      value.unconfirmedAcceptedObservations! > value.recordedAcceptances! || value.omittedRuns !== 0) return false;
  if (value.measurements !== undefined && !validOutcomeMeasurements(value.measurements, value.retainedRuns!, value.recordedAcceptances!)) return false;
  const latest = value.latestAccepted;
  return value.recordedAcceptances === 0 ? latest === null : !!latest &&
    typeof latest.runId === 'string' && latest.runId.length <= 200 && /^run-[a-zA-Z0-9-]+$/.test(latest.runId) && typeof latest.checkId === 'string' && !!latest.checkId && latest.checkId.length <= 200 &&
    typeof latest.releaseId === 'string' && !!latest.releaseId && latest.releaseId.length <= 200 &&
    typeof latest.commit === 'string' && /^[a-f0-9]{40}([a-f0-9]{24})?$/.test(latest.commit) && typeof latest.target === 'string' && !!latest.target && latest.target.length <= 200 && typeof latest.acceptedAt === 'string' && Number.isFinite(Date.parse(latest.acceptedAt));
}
function validOutcomeMeasurements(value:ProjectOutcomeMeasurements,runs:number,acceptances:number):boolean {
  if (!value || value.version!==1 || value.scope!=='retained-run-history' || value.humanActionScope!=='recorded-run-controls' || value.inputTokens!==null || value.outputTokens!==null) return false;
  const n=(v:unknown,max:number):v is number=>Number.isSafeInteger(v)&&(v as number)>=0&&(v as number)<=max;
  if (!n(value.measuredTaskIntervals,20000)||!n(value.unknownTaskIntervals,20000)||!n(value.recordedNoWorkerAttempts,20000)||value.measuredTaskIntervals+value.unknownTaskIntervals+value.recordedNoWorkerAttempts>20000||!n(value.acceptedIntervalsMeasured,acceptances)||!n(value.acceptedIntervalsUnknown,acceptances)||value.acceptedIntervalsMeasured+value.acceptedIntervalsUnknown!==acceptances||!n(value.omittedHumanActions,200000000000)||!n(value.runsWithEarlierUninstrumentedActions,runs)||value.recordedHumanActions!==null&&!n(value.recordedHumanActions,51200)) return false;
  const duration=(v:unknown,intervals:number)=>intervals===0?v===null:n(v,intervals*365*24*60*60*1000);
  return duration(value.taskRuntimeMilliseconds,value.measuredTaskIntervals)&&duration(value.runToAcceptanceMilliseconds,value.acceptedIntervalsMeasured);
}
function validAttention(value: ProjectOverview['attention']): boolean {
  if (value === undefined) return true; // Older services have no observation, not zero obligations.
  if (!value || !validRunAttention(value.items, 3)) return false;
  if (value.status === 'unavailable') return value.count === null && value.omitted === null && value.items.length === 0;
  return value.status === 'recorded' && count(value.count) && value.count !== null && count(value.omitted) && value.omitted !== null &&
    value.count <= 200 && value.count === value.items.length + value.omitted;
}
function validSharedChange(value: ProjectOverview['sharedChange']): boolean {
  if (value === undefined) return true; // An older service has no shared-event projection.
  if (!value || !['recorded', 'none', 'unavailable'].includes(value.status)) return false;
  if (value.status !== 'recorded') return value.event === undefined;
  const event = value.event;
  return !!event && typeof event.eventId === 'string' && !!event.eventId &&
    Number.isSafeInteger(event.sequence) && event.sequence > 0 && !!event.intentId && !!event.originTurnId &&
    typeof event.actor?.id === 'string' && !!event.actor.id && typeof event.actor.displayName === 'string' &&
    !!event.target && /^[a-f0-9]{64}$/.test(event.appliedTargetDigest) &&
    (event.acceptedContextDigest === undefined || /^[a-f0-9]{64}$/.test(event.acceptedContextDigest)) &&
    Array.isArray(event.taskIds) && event.taskIds.every(id => typeof id === 'string' && !!id) &&
    typeof event.affectsAllTasks === 'boolean';
}
function validPrivateFinding(value: ProjectOverview['privateFinding']): boolean {
  if (value === undefined) return true;
  if (!value || !['recorded', 'none', 'unavailable'].includes(value.status)) return false;
  if (value.status !== 'recorded') return value.id === undefined && value.sourceStatus === undefined && value.state === undefined && value.updatedAt === undefined;
  return !!value.id?.startsWith('finding-') && !!value.sourceStatus &&
    ['pending', 'reserved', 'inflight', 'question', 'review', 'observed', 'resumed', 'unknown'].includes(value.state ?? '') &&
    Number.isFinite(Date.parse(value.updatedAt ?? ''));
}
export const projectOverviewApi = {
  async get(projectId: string, signal?: AbortSignal, options?: { includeOutcomes?: boolean }): Promise<ProjectOverview> {
    const query = options?.includeOutcomes ? '?includeOutcomes=true' : '';
    const value = await productApi.request<ProjectOverview>(`/v1/product/projects/${encodeURIComponent(projectId)}/overview${query}`, undefined, 'GET', 30_000, signal);
    if (!value || value.projectId !== projectId || !['ready', 'partial', 'unavailable'].includes(value.status) ||
      typeof value.observedAt !== 'string' || typeof value.phase?.label !== 'string' || typeof value.nextAction?.label !== 'string' ||
      (value.goal !== null && (typeof value.goal?.text !== 'string' || typeof value.goal.truncated !== 'boolean')) ||
      (value.context !== null && (!Number.isSafeInteger(value.context?.tasks?.total) || !Number.isSafeInteger(value.context?.tasks?.recordedDone))) ||
      !value.questions || !count(value.questions.count) || !count(value.questions.omitted) || !Array.isArray(value.questions.items) ||
      value.questions.items.some(item => typeof item.text !== 'string' || typeof item.id !== 'string') ||
      !value.execution || value.execution.scope !== 'swarm-queue' || !['recorded', 'unavailable'].includes(value.execution.status) ||
      ['queued', 'paused', 'reserved', 'running', 'blocked', 'unknown'].some(key => !count(value.execution[key as 'queued'])) ||
      (value.execution.failed !== undefined && (!count(value.execution.failed) ||
        (value.execution.status === 'recorded' ? value.execution.failed === null : value.execution.failed !== null))) ||
      !validAttention(value.attention) ||
      !validSharedChange(value.sharedChange) ||
      !validPrivateFinding(value.privateFinding) ||
      !validProjectOutcomes(value.outcomes) ||
      !Array.isArray(value.limitations) || value.limitations.some(item => typeof item !== 'string')) {
      throw new Error('This project summary could not be read. Refresh it before continuing.');
    }
    // Older services omitted failure counts; retain unknown rather than inventing zero.
    return value.execution.failed === undefined ? { ...value, execution: { ...value.execution, failed: null } } : value;
  },
};
