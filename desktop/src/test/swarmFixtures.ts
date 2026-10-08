import type { ProjectRun, SwarmTask } from '../api/projectRuns';
import type { ProjectSwarmQueue, SwarmQueueItem } from '../api/swarmQueue';
import { aHarness } from './fixtures';

export const swarmHarnessFixture = () => aHarness({ id: 'codex', name: 'Codex', autonomousTask: true, taskPermissionModes: ['bypass'], taskPermissionNotice: 'Fixture descriptor: unattended tasks require explicit Bypass; their outputs await review.', permissionModes: ['ask', 'bypass', 'plan'], supportsOpeningPrompt: true });

const task = (id: string, patch: Partial<SwarmTask> = {}): SwarmTask => ({id, title: `Build ${id}`, dependsOn: [], writePaths: [`src/${id}.ts`], acceptance: `Verify ${id} behavior`, ...patch});
export const swarmRunFixture = (): ProjectRun => ({
  id: 'run-1', projectId: 'p-1', worktreeId: 'main', revision: 1, state: 'ready', reason: 'Reviewed inputs saved', createdAt: '2026-10-08T00:00:00Z', updatedAt: '2026-10-08T00:00:00Z',
  input: {implementation: {projectId: 'p-1', worktreeId: 'main', briefRevision: 1, planRevision: 1, specificationRevision: 1, designPlanRevision: 1, taskIds: ['a', 'b', 'c'], taskTitles: ['Build a', 'Build b', 'Build c'], requirementIds: ['r1'], briefDigest: 'brief', planDigest: 'plan', specificationDigest: 'spec', designPlanDigest: 'design', sourceCommit: 'a'.repeat(40), sourceDigest: 'source', sourceDirty: false}, approvals: {specificationApprovalId: 'as', specificationRevision: 1, specificationDigest: 'spec', designApprovalId: 'ad', designRevision: 1, designDigest: 'design', briefDigest: 'brief', sketchDigest: ''}, outcome: 'preview', requirements: [{id: 'r1', title: 'App', acceptance: 'Works'}], route: {harness: 'codex', model: 'recorded-model', permissionMode: 'bypass', connection: {id: 'saved', kind: 'compatible-api', revision: 3}}},
  operations: [], checks: [], releases: [], allowedActions: [], contextFreshness: {status: 'current', reason: 'Current'},
  swarm: {state: 'ready', maxParallel: 2, tasks: [task('a'), task('b'), task('c', {dependsOn: ['a', 'b']})]},
});

export const swarmQueueItem = (patch: Partial<SwarmQueueItem> = {}): SwarmQueueItem => ({
  id: 'q-a', revision: 2, projectId: 'p-1', runId: 'run-1', taskId: 'a', taskTitle: 'Build a',
  state: 'queued', priority: 'normal', queuedAt: '2026-10-08T00:00:00Z', updatedAt: '2026-10-08T00:00:00Z', ...patch,
});
export const swarmQueueFixture = (items: SwarmQueueItem[] = []): ProjectSwarmQueue => ({
  revision: 70, observedAt: '2026-10-08T00:00:00Z', dispatchPaused: false, hasMore: false, items,
  counts: {queued: items.filter(i => i.state === 'queued').length, paused: 0, reserved: 0, running: 0, blocked: 0, unknown: 0, completed: 0, failed: items.filter(i => i.state === 'failed').length, cancelled: 0},
});
