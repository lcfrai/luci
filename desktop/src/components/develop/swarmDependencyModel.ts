import { runOperationNeedsRecovery, runOperationNeedsReview, type ProjectRun, type SwarmTask } from '../../api/projectRuns';
import type { ProjectSwarmQueue } from '../../api/swarmQueue';

export interface DependencyTask {
  task: SwarmTask;
  activity: string;
  active: boolean;
  queued: boolean;
  waiting: boolean;
  review: boolean;
  attention: string[];
  unknown: boolean;
  waitingOn: string[];
  dependents: string[];
  integrated: boolean;
}

export type DependencyFilter = 'all' | 'attention' | 'waiting' | 'active' | 'queued' | 'review' | 'integrated' | 'unknown';
export function matchesDependencyFilter(item: DependencyTask, filter: DependencyFilter): boolean {
  switch (filter) {
    case 'all': return true;
    case 'attention': return item.attention.length > 0;
    case 'waiting': return item.waiting;
    case 'active': return item.active;
    case 'queued': return item.queued;
    case 'review': return item.review;
    case 'integrated': return item.integrated;
    case 'unknown': return item.unknown;
  }
}

/** Count tasks, not attempts or queue entries. Categories deliberately overlap. */
export function dependencyFleetCounts(tasks: DependencyTask[]): Record<DependencyFilter, number> {
  const counts = {all: tasks.length, attention: 0, waiting: 0, active: 0, queued: 0, review: 0, integrated: 0, unknown: 0};
  for (const item of tasks) {
    for (const filter of ['attention', 'waiting', 'active', 'queued', 'review', 'integrated', 'unknown'] as const) {
      if (matchesDependencyFilter(item, filter)) counts[filter]++;
    }
  }
  return counts;
}

/** Saved facts only. An idle/normal exit and a reported check never prove correctness. */
export function dependencyTasks(run: ProjectRun, queue?: ProjectSwarmQueue): DependencyTask[] {
  const tasks = run.swarm?.tasks ?? [];
  const byId = new Map(tasks.map(task => [task.id, task]));
  const operations = new Map(run.operations.map(operation => [operation.id, operation]));
  const dependents = new Map<string, string[]>();
  for (const task of tasks) for (const id of new Set(task.dependsOn)) {
    const values = dependents.get(id) ?? []; values.push(task.id); dependents.set(id, values);
  }
  const questions = new Map<string, number>();
  const handoffUnknown = new Set<string>();
  for (const operation of run.operations) {
    if (!operation.taskId || !operation.handoff) continue;
    const handoff = operation.handoff;
    if (handoff.status === 'captured' && Number.isInteger(handoff.questions) && handoff.questions >= 0 && handoff.questions <= 8 &&
        Number.isInteger(handoff.unanswered) && handoff.unanswered >= 0 && handoff.unanswered <= handoff.questions) {
      questions.set(operation.taskId, (questions.get(operation.taskId) ?? 0) + handoff.unanswered);
    } else handoffUnknown.add(operation.taskId);
  }
  const queues = new Map<string, string[]>();
  for (const item of queue?.items ?? []) {
    if (item.projectId !== run.projectId || item.runId !== run.id) continue;
    // An explicitly retained older attempt cannot describe the selected attempt.
    if (item.operationId && byId.get(item.taskId)?.operationId !== item.operationId) continue;
    const states = queues.get(item.taskId) ?? []; states.push(item.state); queues.set(item.taskId, states);
  }
  return tasks.map(task => {
    const operation = task.operationId ? operations.get(task.operationId) : undefined;
    // A mismatched persisted reference cannot describe this task's activity.
    const owned = operation?.taskId === task.id ? operation : undefined;
    const integrated = task.integration?.state === 'integrated';
    const waitingOn = [...new Set(task.dependsOn)].filter(id => byId.get(id)?.integration?.state !== 'integrated');
    const attention: string[] = [];
    const states = queues.get(task.id) ?? [];
    let unknown = !!task.operationId && !owned || handoffUnknown.has(task.id) ||
      !!task.integration && !['integrating', 'integrated'].includes(task.integration.state) || task.dependsOn.some(id => !byId.has(id));
    let activity = 'No attempt recorded';
    let active = false;
    if (owned) {
      active = owned.status === 'running' && owned.allocation === 'ready' && !owned.startFailure && !runOperationNeedsRecovery(owned);
      const labels: Record<string, string> = {reserved: 'Attempt reserved', dispatching: 'Dispatch unconfirmed', running: 'Recorded running', unknown: 'Activity unconfirmed',
        'review-required': 'Attempt ended', succeeded: 'Attempt ended', failed: 'Attempt failed', cancelled: 'Attempt stopped'};
      activity = labels[owned.status] ?? 'Unrecognized attempt state';
      unknown ||= !labels[owned.status] || ['unknown', 'dispatching'].includes(owned.status) || owned.allocation === 'unknown' || owned.allocation === 'allocating';
      if (runOperationNeedsRecovery(owned)) attention.push('Recovery needs inspection');
      if (owned.startFailure) attention.push('Start needs inspection');
      if (!integrated && runOperationNeedsReview(owned)) attention.push('Review needed');
      if (owned.status === 'failed') attention.push('Failed attempt needs inspection');
    }
    const unanswered = questions.get(task.id) ?? 0;
    if (unanswered) attention.push(`${unanswered} unanswered ${unanswered === 1 ? 'question' : 'questions'} across retained attempts`);
    if (handoffUnknown.has(task.id)) attention.push('Handoff unavailable or invalid');
    if (task.integration?.state === 'integrating') { attention.push('Integration pending'); unknown = true; }
    if (states.some(state => ['blocked', 'unknown', 'failed'].includes(state))) attention.push('Queue needs inspection');
    unknown ||= states.some(state => !['queued', 'paused', 'reserved', 'running', 'blocked', 'completed', 'failed', 'cancelled'].includes(state));
    if (!owned && states.includes('queued')) activity = 'Queued; not started';
    if (!owned && states.includes('running')) { activity = 'Queue reports running; attempt unavailable'; unknown = true; }
    // Legacy success is not an exact pending independent-review obligation.
    if (!integrated && owned?.status === 'succeeded') { attention.push('Review disposition unconfirmed'); unknown = true; }
    if (unknown) attention.push('Unconfirmed state');
    const queued = states.includes('queued');
    const waiting = !integrated && (waitingOn.length > 0 || unanswered > 0 || states.some(state => ['paused', 'blocked'].includes(state)));
    const review = !integrated && owned?.status === 'review-required';
    return {task, activity, active, queued, waiting, review, attention, unknown, waitingOn, dependents: dependents.get(task.id) ?? [], integrated};
  });
}
