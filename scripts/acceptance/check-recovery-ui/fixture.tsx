// CLOUD020 controlled source fixture. Load at the virtual repository path
// scripts/acceptance/cloud020-check-ui/fixture.tsx so these imports resolve to
// the staged source tree selected by the parent witness. This is API simulation,
// never Docker, control-plane, installed desktop, or real execution evidence.
import '../../../desktop/src/styles/app.css';
import React from 'react';
import {createRoot} from 'react-dom/client';
import {AppShell} from '../../../desktop/src/components/AppShell';
import {resetCockpit, useCockpit} from '../../../desktop/src/store/store';
import {aProject, aBranch, aWorktree, aMachine, aHarness} from '../../../desktop/src/test/fixtures';
import {checkRunFixture, checkRecoveryFixture} from '../../../desktop/src/test/checkFixtures';
import {context, view} from '../../../desktop/src/components/orchestrator/fixtures';
import {productApi, type CheckRun, type RequirementCoverage} from '../../../desktop/src/api/product';
import {type CheckRecoveryReview} from '../../../desktop/src/api/projectCheckContracts';
import {definitionApi} from '../../../desktop/src/api/definition';
import {designPlanApi} from '../../../desktop/src/api/designPlan';
import {projectRunsApi} from '../../../desktop/src/api/projectRuns';
import {orchestratorApi} from '../../../desktop/src/api/orchestrator';
import {ApiError, type ProblemCode} from '../../../desktop/src/api/client';
import {api} from '../../../desktop/src/api';

const supportedModes = ['overview', 'recover', 'lost-response', 'read-denied', 'switch-denial', 'pending-read', 'pending-write', 'polling'] as const;
type Mode = typeof supportedModes[number];
const requestedMode = new URLSearchParams(location.search).get('mode') || 'overview';
if (!supportedModes.includes(requestedMode as Mode)) throw Error('Unknown controlled check fixture mode: ' + requestedMode);
const mode = requestedMode as Mode;
const ids = ['alpha', 'beta'] as const;
type ProjectId = typeof ids[number];
const name = (id: string) => id === 'alpha' ? 'Alpha' : 'Beta';
const clone = <T,>(value: T): T => structuredClone(value);
const stamp = '2026-10-08T00:00:00Z';
const metadata = {
  id: 'CLOUD020', version: 1, mode,
  scope: 'Actual AppShell, DevelopLens, ProjectWork, ProjectChecks, CheckRecoveryDialog, WorkflowDialog, navigation and draft hooks with controlled API callbacks.',
  virtualPath: 'scripts/acceptance/cloud020-check-ui/fixture.tsx',
  omittedLeaf: 'ProjectPlan',
  controlledProviders: ['AuthProvider', 'SetupProvider', 'TeamWorkspaceProvider'],
  executionEvidence: false,
  transportEvidence: false,
  visibilityEvidence: 'A synthetic visibilitychange event and controlled document visibilityState; no operating-system background-window claim.',
  delayedReadBehavior: 'Abort signals are recorded. Delayed callbacks can still settle after abort to exercise stale-response guards.',
  recoveryBinding: {projectId: 'alpha', checkId: 'check-alpha', revision: 4, recoveryDigest: 'd'.repeat(64), method: 'remove-exact', commandIndex: 0},
};

const records: Record<ProjectId, CheckRun[]> = Object.fromEntries(ids.map(id => {
  const unknown = checkRunFixture({
    id: 'check-' + id, projectId: id, stage: 'recovery',
    message: name(id) + ' cleanup is unconfirmed; inspect its original allocation.',
    commands: [{...checkRunFixture().commands[0], allocationId: 'allocation-' + id,
      output: name(id) + ' retained check output: 7 tests passed; cleanup remains unconfirmed.'}],
  });
  const failed = checkRunFixture({
    id: 'failed-' + id, projectId: id, revision: 2, status: 'failed', resourcesHeld: false,
    stage: 'complete', reasonCode: 'command_failed', createdAt: '2026-10-07T00:00:00Z',
    message: name(id) + ' retained failed check.',
    commands: [{...checkRunFixture().commands[0], label: 'API tests', allocationId: 'failed-allocation-' + id,
      status: 'failed', exitCode: 1, cleanupConfirmed: true,
      output: name(id) + ' retained failure output: expected 200, received 500.'}],
  });
  if (mode !== 'polling') return [id, [unknown, failed]];
  const observing = Array.from({length: 6}, (_, index) => checkRunFixture({
    ...unknown, id: index === 0 ? unknown.id : unknown.id + '-' + (index + 1),
    stage: 'cleanup', status: index % 3 === 0 ? 'unknown' : index % 3 === 1 ? 'running' : 'queued',
    message: name(id) + ' controlled observation ' + (index + 1) + ' still holds capacity.',
    commands: [{...unknown.commands[0], allocationId: 'poll-allocation-' + id + '-' + index}],
  }));
  return [id, observing];
})) as Record<ProjectId, CheckRun[]>;

const coverage = (id: string): RequirementCoverage => ({
  projectId: id,
  specification: {revision: 2, status: 'approved', digest: 'spec'},
  design: {revision: 1, status: 'approved', digest: 'design'},
  source: {commit: 'a'.repeat(40), sourceDigest: 'b'.repeat(64), dirty: false, available: true},
  rows: [{requirementId: 'REQ-1', title: name(id) + ' reviewed result',
    acceptance: 'The result is independently verified before it is accepted.',
    taskIds: ['T-1'], doneTaskIds: [], evidenceRunIds: [], evidenceStatus: 'unverified'}],
});
const getRecord = (projectId: string, checkId: string) => {
  const saved = records[projectId as ProjectId]?.find(row => row.id === checkId);
  if (!saved) throw new ApiError('project_not_found', 'Controlled check record is unavailable.', 404);
  return saved;
};
const recovery = (projectId: string, checkId: string): CheckRecoveryReview => {
  const saved = getRecord(projectId, checkId);
  return checkRecoveryFixture({projectId, checkId, revision: saved.revision!,
    ...(saved.resourcesHeld === false ? {
      state: 'complete', method: null, commandIndex: null, recoveryDigest: null,
      reasonCode: 'cleanup_confirmed', message: 'The recorded allocation is stopped and cleanup is confirmed.',
    } : {}),
  });
};

type ReadKind = 'history' | 'check' | 'recovery';
interface ReadLog {
  id: number; kind: ReadKind; method: 'GET'; projectId: string; checkId?: string;
  at: number; round: number | null; aborted: boolean; abortedAt?: number;
  status: 'pending' | 'resolved' | 'rejected'; settledAt?: number; responseStatus?: number;
}
interface WriteLog {
  kind: 'recover'; method: 'POST'; projectId: string; checkId: string; at: number;
  review: CheckRecoveryReview;
  body: {expectedRevision: number; recoveryDigest: string | null; acknowledgeStop: true};
  status: 'pending' | 'resolved' | 'rejected'; savedRevision?: number;
}
const reads: ReadLog[] = [], writes: WriteLog[] = [];
const signalRounds = new WeakMap<AbortSignal, number>();
let nextRound = 0, inflight = 0, maxInflight = 0, starts = 0, cancels = 0;
let slowReads = ['pending-read', 'switch-denial', 'polling'].includes(mode);
const deniedProjects = new Set<string>(mode === 'read-denied' ? ['alpha'] : []);
interface PendingRead {log: ReadLog; finish(forceDeny?: boolean): void}
const pendingReads: PendingRead[] = [];
const pendingWrites: (() => void)[] = [];
const denied = () => new ApiError('permission_denied' as ProblemCode, 'Controlled check GET access was denied.', 403);

function controlledRead<T>(kind: ReadKind, projectId: string, checkId: string | undefined, signal: AbortSignal | undefined, value: () => T): Promise<T> {
  if (signal && !signalRounds.has(signal)) signalRounds.set(signal, ++nextRound);
  const log: ReadLog = {id: reads.length + 1, kind, method: 'GET', projectId, checkId,
    at: performance.now(), round: signal ? signalRounds.get(signal)! : null,
    aborted: !!signal?.aborted, status: 'pending'};
  reads.push(log); inflight++; maxInflight = Math.max(maxInflight, inflight);
  const refused = kind !== 'history' && (deniedProjects.has(projectId) || mode === 'switch-denial' && projectId === 'alpha');
  const delayed = kind === 'check' && slowReads && (mode !== 'switch-denial' || projectId === 'alpha');
  return new Promise<T>((resolve, reject) => {
    let settled = false;
    const aborted = () => { log.aborted = true; log.abortedAt = performance.now(); };
    signal?.addEventListener('abort', aborted, {once: true});
    const entry: PendingRead = {log, finish(forceDeny = false) {
      if (settled) return;
      settled = true; inflight--; log.settledAt = performance.now();
      signal?.removeEventListener('abort', aborted);
      const index = pendingReads.indexOf(entry);
      if (index >= 0) pendingReads.splice(index, 1);
      try {
        if (forceDeny || refused) throw denied();
        const response = clone(value()); log.status = 'resolved'; log.responseStatus = 200; resolve(response);
      } catch (cause) {
        log.status = 'rejected'; log.responseStatus = cause instanceof ApiError ? cause.status : 0; reject(cause);
      }
    }};
    if (delayed) pendingReads.push(entry); else entry.finish();
  });
}
function releaseRead(projectId?: string) {
  const selected = pendingReads.filter(entry => !projectId || entry.log.projectId === projectId);
  selected.forEach(entry => entry.finish());
  return selected.length;
}
function saveRecovery(projectId: string, checkId: string): CheckRun {
  const saved = getRecord(projectId, checkId);
  const updated: CheckRun = {...saved, revision: (saved.revision ?? 0) + 1,
    status: 'failed', resourcesHeld: false, stage: 'complete', reasonCode: 'cleanup_confirmed',
    updatedAt: '2026-10-08T00:01:00Z',
    message: name(projectId) + ' recovery recorded; inspect retained output before authorizing new work.',
    commands: saved.commands.map(command => ({...command, cleanupConfirmed: true})),
  };
  records[projectId as ProjectId] = records[projectId as ProjectId].map(row => row.id === checkId ? updated : row);
  return clone(updated);
}

productApi.checks = async (id, signal) => controlledRead('history', id, undefined, signal, () => ({
  runs: records[id as ProjectId], suggested: [{label: 'Unit tests', executable: 'npm', args: ['test']}], coverage: coverage(id),
}));
productApi.checkRun = async (id, checkId, signal) => controlledRead('check', id, checkId, signal, () => getRecord(id, checkId));
productApi.checkRecovery = async (id, checkId, signal) => controlledRead('recovery', id, checkId, signal, () => recovery(id, checkId));
productApi.recoverCheck = async (projectId, checkId, review) => {
  const log: WriteLog = {kind: 'recover', method: 'POST', projectId, checkId, at: performance.now(), review: clone(review),
    body: {expectedRevision: review.revision, recoveryDigest: review.recoveryDigest, acknowledgeStop: true}, status: 'pending'};
  writes.push(log);
  const expected = recovery(projectId, checkId);
  if (review.projectId !== projectId || review.checkId !== checkId || review.revision !== expected.revision ||
      review.recoveryDigest !== expected.recoveryDigest || review.method !== 'remove-exact' || review.commandIndex !== 0 || expected.state !== 'available') {
    log.status = 'rejected'; throw new ApiError('conflict', 'Controlled recovery binding did not match the exact reviewed allocation.', 409);
  }
  if (mode === 'pending-write') await new Promise<void>(resolve => pendingWrites.push(resolve));
  const saved = saveRecovery(projectId, checkId); log.savedRevision = saved.revision;
  if (mode === 'lost-response') {
    log.status = 'rejected'; throw new TypeError('Controlled response lost after recovery was recorded.');
  }
  log.status = 'resolved'; return saved;
};
productApi.runChecks = async () => { starts++; throw Error('Command starts are forbidden in this controlled witness.'); };
productApi.cancelChecks = async () => { cancels++; throw Error('Cancellation is outside this controlled recovery witness.'); };
api.createSession = async () => { starts++; throw Error('Agent starts are forbidden in this controlled witness.'); };

// Saved context supports the actual AppShell/Develop/ProjectWork hierarchy.
// Only the unrelated ProjectPlan leaf is omitted by the Vite fixture plugin.
const facts = (id: string) => {
  const saved = context(id);
  saved.brief.purpose = name(id) + ' controlled check recovery workspace.';
  saved.specification.purpose = saved.brief.purpose; saved.designPlan.status = 'approved';
  saved.sourceCommit = 'a'.repeat(40); saved.checksSummary = 'Controlled retained check records';
  return saved;
};
productApi.brief = async id => facts(id).brief;
productApi.design = async id => ({projectId: id, revision: 0, briefRevision: 1, description: '', elements: [], appState: {}, files: {}, updatedAt: stamp});
definitionApi.get = async id => facts(id).specification;
designPlanApi.get = async id => facts(id).designPlan;
productApi.git = async id => ({projectId: id, branch: 'main', clean: true, changes: [], log: [], commits: [], remotes: []});
productApi.plan = async id => facts(id).plan;
productApi.agentConnections = async () => ({revision: 1, activeConnectionId: 'controlled', connections: [{
  id: 'controlled', revision: 1, name: 'Controlled provider', kind: 'compatible-api', enabled: true,
  configured: true, keyPresent: true, endpoints: {responses: 'https://invalid.example'}, models: {codex: 'controlled-model'},
}]});
projectRunsApi.list = async () => [];
orchestratorApi.get = async id => view({projectId: id, context: facts(id),
  nextAction: {id: 'inspect-checks', label: 'Inspect retained checks', reason: 'Inspect original check output and recovery before authorizing new work.', lens: 'develop'},
});

const fixture = {
  metadata, reads, writes,
  get starts() { return starts; }, get cancels() { return cancels; },
  selected: () => useCockpit.getState().selection.projectId,
  releaseRead,
  releaseWrite() { const pending = pendingWrites.splice(0); pending.forEach(release => release()); return pending.length; },
  denyRead(projectId = 'alpha') {
    deniedProjects.add(projectId);
    const pending = pendingReads.filter(entry => entry.log.projectId === projectId);
    pending.forEach(entry => entry.finish(true)); return pending.length;
  },
  allowReads() { deniedProjects.clear(); slowReads = false; },
  setSlowReads(value: boolean) { slowReads = value; },
  settlePolls() {
    for (const id of ids) records[id] = records[id].map(saved => ({...saved,
      revision: (saved.revision ?? 0) + 1, status: 'passed', resourcesHeld: false,
      stage: 'complete', reasonCode: 'cleanup_confirmed', updatedAt: '2026-10-08T00:02:00Z',
      message: name(id) + ' controlled observation finished; cleanup is confirmed.',
      commands: saved.commands.map(command => ({...command, status: 'passed', exitCode: 0, cleanupConfirmed: true})),
    }));
    return releaseRead();
  },
  switchProject(id: ProjectId) {
    if (!ids.includes(id)) throw Error('Unknown controlled project.');
    useCockpit.getState().selectProject(id, 'develop');
    // Models an external scope change while a modal owns focus. Preserve drafts
    // through the real store navigation implementation, never by deleting them.
    if (useCockpit.getState().pendingNavigation) useCockpit.getState().discardDraftsAndNavigate(true);
  },
  setVisibility(value: 'hidden' | 'visible') {
    if (value !== 'hidden' && value !== 'visible') throw Error('Unknown controlled visibility.');
    Object.defineProperty(document, 'visibilityState', {configurable: true, get: () => value});
    Object.defineProperty(document, 'hidden', {configurable: true, get: () => value === 'hidden'});
    document.dispatchEvent(new Event('visibilitychange'));
  },
  snapshot() {
    return clone({metadata, reads, writes, starts, cancels, inflight, maxInflight,
      selected: useCockpit.getState().selection.projectId,
      pendingReadCount: pendingReads.length, pendingWriteCount: pendingWrites.length,
      slowReads, visibility: document.visibilityState, records,
      retainedDrafts: Object.keys(localStorage).filter(key => key.startsWith('luci.project-draft.v1:')).map(key => ({key, value: JSON.parse(localStorage.getItem(key)!)})),
    });
  },
};
declare global { interface Window { __checkFixture: typeof fixture } }
window.__checkFixture = fixture;

resetCockpit();
useCockpit.setState({
  projects: ids.map(id => aProject({id, name: name(id), path: '/controlled/' + id})),
  machines: [aMachine()], harnesses: [aHarness()],
  branchesByProject: Object.fromEntries(ids.map(id => [id, [aBranch({id: 'branch-' + id, projectId: id, name: 'main'})]])),
  worktreesByBranch: Object.fromEntries(ids.map(id => ['branch-' + id, [aWorktree({id: 'main-' + id, projectId: id, branchId: 'branch-' + id, isPrimary: true})]])),
});
for (const id of ids) useCockpit.getState().selectProject(id, 'develop');
useCockpit.getState().selectProject('alpha', 'develop');
document.documentElement.dataset.theme = 'light';
document.documentElement.dataset.controlledFixture = 'cloud020-check-ui';
document.title = 'CLOUD020 controlled check recovery fixture · ' + mode;
createRoot(document.getElementById('root')!).render(<AppShell />);
