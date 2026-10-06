import { act, render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { beforeEach, expect, it, vi } from 'vitest';
import { api } from '../../api';
import { aBranch, aProject, aSession, aWorktree } from '../../test/fixtures';
import { productApi, type CheckRun, type RequirementCoverage } from '../../api/product';
import { projectRunsApi, type ProjectRun } from '../../api/projectRuns';
import { resetCockpit, useCockpit } from '../../store/store';
import { ProjectChecks } from './ProjectChecks';
import { ProjectWork } from './ProjectWork';

const identity = vi.hoisted(() => ({ scope: 'owner-workspace' }));
const access = vi.hoisted(() => ({ canRead: true, canEdit: true, canExecute: true, canAssist: true, message: '' }));
vi.mock('../../hooks/useProjectAccess', () => ({ useProjectAccess: () => access }));
vi.mock('../../hooks/useProjectDraft', () => ({ useProjectDraftScope: () => identity.scope }));
vi.mock('./ProjectPlan', () => ({ ProjectPlan: () => null }));
const commands = [{ label: 'Acceptance tests', executable: 'npm', args: ['test'] }];
const requirement = { id: 'R-KEEP', title: 'Keep notes', acceptance: 'Saved notes survive reload.' };
const build: ProjectRun = {
  id: 'run-reviewed', projectId: 'p-1', worktreeId: 'w-1', revision: 5, createdAt: '2026-10-06', updatedAt: '2026-10-06', state: 'awaiting-checks', reason: 'Coding ended; verify the result.',
  input: {
    implementation: { projectId: 'p-1', worktreeId: 'w-1', briefRevision: 2, planRevision: 3, specificationRevision: 4, designPlanRevision: 5, taskIds: ['t-1'], taskTitles: ['Build journal'], requirementIds: [requirement.id], briefDigest: 'brief', planDigest: 'plan', specificationDigest: 'spec', designPlanDigest: 'design', sourceCommit: 'a'.repeat(40), sourceDigest: 'source', sourceDirty: false },
    approvals: { specificationApprovalId: 'a-spec', specificationRevision: 4, specificationDigest: 'spec', designApprovalId: 'a-design', designRevision: 5, designDigest: 'design', briefDigest: 'brief', sketchDigest: 'sketch' },
    outcome: 'preview', requirements: [requirement], route: { harness: 'codex', model: 'reviewed-model', permissionMode: 'ask', connection: { id: 'route', revision: 1, kind: 'compatible-api' } },
  }, operations: [], checks: [], releases: [], allowedActions: ['join-check', 'reconcile', 'start-coding'],
};
const coverage: RequirementCoverage = { projectId: 'p-1', specification: { revision: 4, status: 'approved', digest: 'spec' }, design: { revision: 5, status: 'approved', digest: 'design' }, source: { commit: 'a'.repeat(40), sourceDigest: 'source', available: true, dirty: false }, rows: [
  { requirementId: requirement.id, title: requirement.title, acceptance: requirement.acceptance, taskIds: ['t-1'], doneTaskIds: [], evidenceRunIds: [], evidenceStatus: 'unverified' },
  { requirementId: 'R-OTHER', title: 'Other work', acceptance: 'Not selected for this build.', taskIds: [], doneTaskIds: [], evidenceRunIds: [], evidenceStatus: 'unverified' },
] };
const passing: CheckRun = { id: 'check-pass', projectId: 'p-1', commit: coverage.source.commit, sourceDigest: coverage.source.sourceDigest, dirty: false, execution: 'docker', requirementIds: [requirement.id], requirementSnapshots: [requirement], specificationRevision: 4, specificationDigest: 'spec', designPlanRevision: 5, designPlanDigest: 'design', evidenceStatus: 'linked-test-evidence', status: 'passed', commands: [{ ...commands[0], status: 'passed', exitCode: 0, output: 'Acceptance assertion passed.', truncated: false }], createdAt: '2026-10-06', updatedAt: '2026-10-06', message: 'Commands passed' };
const queue = () => useCockpit.setState({ projectRunVerification: { scope: identity.scope, projectId: 'p-1', runId: build.id } });
beforeEach(() => {
  vi.restoreAllMocks(); resetCockpit(); identity.scope = 'owner-workspace'; access.canRead = true;
  vi.spyOn(productApi, 'checks').mockResolvedValue({ runs: [], suggested: commands, coverage });
  vi.spyOn(projectRunsApi, 'get').mockResolvedValue(build);
  vi.spyOn(productApi, 'runChecks').mockResolvedValue(passing);
  vi.spyOn(projectRunsApi, 'joinCheck').mockResolvedValue({ ...build, revision: 7, state: 'awaiting-release', allowedActions: ['join-release'] });
});
async function selectAndTrust() {
  await userEvent.click(screen.getByRole('checkbox', { name: 'Check R-KEEP · Keep notes' }));
  await userEvent.click(screen.getByRole('checkbox', { name: /I trust this project's code/ }));
}

it('opens the exact reviewed build with visible, initially unchecked criteria and no execution', async () => {
  queue(); render(<ProjectChecks projectId="p-1" />);
  const region = await screen.findByRole('region', { name: 'Build verification' });
  await within(region).findByText('Build journal');
  expect(projectRunsApi.get).toHaveBeenCalledWith('p-1', build.id);
  expect((within(region).getByRole('checkbox', { name: 'Check R-KEEP · Keep notes' }) as HTMLInputElement).checked).toBe(false);
  expect(screen.queryByRole('checkbox', { name: /Check R-OTHER/ })).toBeNull();
  expect(screen.getByText('Advanced check commands').closest('details')?.open).toBe(false);
  expect(productApi.runChecks).not.toHaveBeenCalled(); expect(projectRunsApi.joinCheck).not.toHaveBeenCalled();
  await userEvent.click(screen.getByRole('checkbox', { name: /I trust this project's code/ }));
  expect((screen.getByRole('button', { name: 'Run selected requirement checks' }) as HTMLButtonElement).disabled).toBe(true);
});

it('runs explicitly selected coverage and attaches eligible results only by a separate action using the fresh exact run revision', async () => {
  queue(); render(<ProjectChecks projectId="p-1" />);
  await screen.findByText('Build journal'); await selectAndTrust();
  expect((screen.getByRole('button', { name: 'Run selected requirement checks' }) as HTMLButtonElement).disabled).toBe(false);
  await userEvent.click(screen.getByRole('button', { name: 'Run selected requirement checks' }));
  const use = await screen.findByRole('button', { name: 'Use these results for this build' });
  expect(productApi.runChecks).toHaveBeenCalledWith('p-1', commands, true, 'docker', { requirementIds: ['R-KEEP'], expectedSpecificationRevision: 4, expectedDesignPlanRevision: 5 });
  expect(projectRunsApi.joinCheck).not.toHaveBeenCalled();
  vi.mocked(projectRunsApi.get).mockResolvedValue({ ...build, revision: 6 });
  await userEvent.dblClick(use);
  await screen.findByText('Results added to this build. Continue from its next step above.');
  expect(projectRunsApi.joinCheck).toHaveBeenCalledTimes(1);
  expect(projectRunsApi.joinCheck).toHaveBeenCalledWith('p-1', build.id, 6, passing.id);
  expect(useCockpit.getState().projectRunInspection).toEqual({ scope: identity.scope, projectId: 'p-1', runId: build.id, refresh: true });
});

it.each([
  ['generic', { evidenceStatus: 'unverified' as const, requirementIds: [] }],
  ['failed', { status: 'failed', commands: [{ ...passing.commands[0], status: 'failed', exitCode: 1 }] }],
  ['dirty', { dirty: true }],
  ['old source', { commit: 'b'.repeat(40) }],
  ['different review', { designPlanDigest: 'changed' }],
  ['different acceptance', { requirementSnapshots: [{ ...requirement, acceptance: 'Different outcome' }] }],
])('does not offer %s check evidence for the selected build', async (_name, patch) => {
  vi.mocked(productApi.checks).mockResolvedValue({ runs: [{ ...passing, ...patch }], suggested: commands, coverage });
  queue(); render(<ProjectChecks projectId="p-1" />); await screen.findByText('Build journal');
  expect(screen.queryByRole('button', { name: 'Use these results for this build' })).toBeNull();
  expect(projectRunsApi.joinCheck).not.toHaveBeenCalled();
});

it('blocks stale reviewed context and dirty source with a recovery action instead of silently checking different inputs', async () => {
  vi.mocked(productApi.checks).mockResolvedValue({ runs: [passing], suggested: commands, coverage: { ...coverage, source: { ...coverage.source, dirty: true } } });
  queue(); render(<ProjectChecks projectId="p-1" />); await screen.findByText('Build journal');
  expect(screen.getByText(/Review and commit your changes before verifying/)).not.toBeNull();
  expect((screen.getByRole('checkbox', { name: 'Check R-KEEP · Keep notes' }) as HTMLInputElement).disabled).toBe(true);
  expect(screen.queryByRole('button', { name: 'Use these results for this build' })).toBeNull();
  vi.mocked(productApi.checks).mockResolvedValue({ runs: [passing], suggested: commands, coverage: { ...coverage, design: { ...coverage.design, digest: 'new-design' } } });
  await userEvent.click(screen.getByRole('button', { name: 'Refresh requirement evidence' }));
  await screen.findByText(/no longer match this build's reviewed inputs/);
  expect(productApi.runChecks).not.toHaveBeenCalled();
});

it('does not attach a late result or expose the old selected build after an account switch', async () => {
  vi.mocked(productApi.checks).mockResolvedValue({ runs: [passing], suggested: commands, coverage });
  queue(); const view = render(<ProjectChecks projectId="p-1" />); await screen.findByText('Build journal');
  let resolve!: (value: ProjectRun) => void;
  vi.mocked(projectRunsApi.get).mockReturnValue(new Promise(done => { resolve = done; }));
  await userEvent.click(screen.getByRole('button', { name: 'Use these results for this build' }));
  identity.scope = 'different-owner'; view.rerender(<ProjectChecks projectId="p-1" />);
  await act(async () => resolve(build));
  expect(projectRunsApi.joinCheck).not.toHaveBeenCalled();
  expect(screen.queryByRole('region', { name: 'Build verification' })).toBeNull();
  expect(useCockpit.getState().projectRunInspection).toBeNull();
});

it('opens collapsed project work when Verify this build is chosen', async () => {
  vi.spyOn(projectRunsApi, 'list').mockResolvedValue([build]);
  vi.spyOn(productApi, 'brief').mockResolvedValue({ projectId: 'p-1', revision: 2, purpose: 'Journal', audience: '', constraints: '', criteria: [], updatedAt: '' });
  vi.spyOn(productApi, 'design').mockResolvedValue({ projectId: 'p-1', revision: 1, briefRevision: 2, notes: '', scene: null, updatedAt: '' } as never);
  vi.spyOn(productApi, 'git').mockResolvedValue({ clean: true, changes: [], log: [] } as never);
  const { container } = render(<ProjectWork projectId="p-1" expanded={false} />);
  await userEvent.click(await screen.findByRole('button', { name: 'Verify this build' }));
  await waitFor(() => expect(container.querySelector<HTMLDetailsElement>('.project-work')?.open).toBe(true));
  await screen.findByRole('region', { name: 'Build verification' });
  expect(productApi.runChecks).not.toHaveBeenCalled();
});


function codingWorkFixture() {
  const session = aSession({ id: 's-current', projectId: 'p-1', branchId: 'b-1', worktreeId: 'w-1', projectRunId: build.id, projectRunOperationId: 'op-current', status: 'running' });
  const operation = { id: 'op-current', kind: 'session', childId: session.id, attempt: 1, status: 'running', requestDigest: 'request', createdAt: '', updatedAt: '' };
  vi.spyOn(projectRunsApi, 'list').mockResolvedValue([{ ...build, operations: [operation] }]);
  vi.spyOn(api, 'getSession').mockResolvedValue(session);
  vi.spyOn(productApi, 'brief').mockResolvedValue({ projectId: 'p-1', revision: 2, purpose: 'Journal', audience: '', constraints: '', criteria: [], updatedAt: '' });
  vi.spyOn(productApi, 'design').mockResolvedValue({ projectId: 'p-1', revision: 1, briefRevision: 2, notes: '', scene: null, updatedAt: '' } as never);
  vi.spyOn(productApi, 'git').mockResolvedValue({ clean: true, changes: [], log: [] } as never);
  useCockpit.setState({ projects: [aProject({ id: 'p-1' })], branchesByProject: { 'p-1': [aBranch({ id: 'b-1', projectId: 'p-1' })] }, worktreesByBranch: { 'b-1': [aWorktree({ id: 'w-1', projectId: 'p-1', branchId: 'b-1', isPrimary: true })] }, selection: { projectId: 'p-1', branchId: 'b-1', worktreeId: 'w-1', sessionId: null } });
  return session;
}

it('gives the owned coding session room by collapsing checks after focus, retaining mounted draft controls and reopening verification explicitly', async () => {
  const session = codingWorkFixture();
  useCockpit.setState({ unsavedProjectDrafts: { draft: { projectId: 'p-1', label: 'Draft note', lens: 'develop' } } });
  const { container } = render(<ProjectWork projectId="p-1"><input aria-label="Draft note" defaultValue="Keep this draft" /></ProjectWork>);
  const draft = await screen.findByRole('textbox', { name: 'Draft note' });
  await userEvent.type(draft, ' while coding');
  await userEvent.click(await screen.findByRole('button', { name: 'Open coding session' }));
  await waitFor(() => expect(container.querySelector<HTMLDetailsElement>('.project-work')?.open).toBe(false));
  expect(useCockpit.getState().selection.sessionId).toBe(session.id);
  expect(draft.isConnected).toBe(true);
  expect((draft as HTMLInputElement).value).toBe('Keep this draft while coding');
  expect(useCockpit.getState().unsavedProjectDrafts.draft).toBeDefined();
  await userEvent.click(screen.getByRole('button', { name: 'Verify this build' }));
  await waitFor(() => expect(container.querySelector<HTMLDetailsElement>('.project-work')?.open).toBe(true));
  expect((draft as HTMLInputElement).value).toBe('Keep this draft while coding');
});

it('keeps plan and checks open when the coding session fails the exact run ownership check', async () => {
  const session = codingWorkFixture(); vi.mocked(api.getSession).mockResolvedValue({ ...session, projectRunId: 'another-run' });
  const { container } = render(<ProjectWork projectId="p-1" />);
  await userEvent.click(await screen.findByRole('button', { name: 'Open coding session' }));
  await screen.findByText(/This coding session does not belong to the selected run/);
  expect(container.querySelector<HTMLDetailsElement>('.project-work')?.open).toBe(true);
  expect(useCockpit.getState().selection.sessionId).toBeNull();
});

it('keeps the panel and drafts open when session navigation is pending and then cancelled', async () => {
  codingWorkFixture();
  useCockpit.setState({ selection: { projectId: 'p-1', branchId: 'b-other', worktreeId: 'w-other', sessionId: null }, unsavedProjectDrafts: { draft: { projectId: 'p-1', label: 'Draft note', lens: 'develop' } } });
  const { container } = render(<ProjectWork projectId="p-1" />);
  await userEvent.click(await screen.findByRole('button', { name: 'Open coding session' }));
  await waitFor(() => expect(useCockpit.getState().pendingNavigation).toEqual({ kind: 'branch', projectId: 'p-1', branchId: 'b-1' }));
  expect(container.querySelector<HTMLDetailsElement>('.project-work')?.open).toBe(true);
  act(() => useCockpit.getState().cancelNavigation());
  expect(container.querySelector<HTMLDetailsElement>('.project-work')?.open).toBe(true);
  expect(useCockpit.getState().selection).toEqual({ projectId: 'p-1', branchId: 'b-other', worktreeId: 'w-other', sessionId: null });
  expect(useCockpit.getState().unsavedProjectDrafts.draft).toBeDefined();
});

it('does not focus or collapse from an old workspace session response', async () => {
  const session = codingWorkFixture(); let resolve!: (value: typeof session) => void;
  vi.mocked(api.getSession).mockReturnValue(new Promise(done => { resolve = done; }));
  const view = render(<ProjectWork projectId="p-1" />);
  await userEvent.click(await screen.findByRole('button', { name: 'Open coding session' }));
  identity.scope = 'different-workspace'; view.rerender(<ProjectWork projectId="p-1" />);
  await act(async () => resolve(session));
  expect(view.container.querySelector<HTMLDetailsElement>('.project-work')?.open).toBe(true);
  expect(useCockpit.getState().selection.sessionId).toBeNull();
});


it('does not focus or collapse after reading permission changes while opening a session', async () => {
  const session = codingWorkFixture(); let resolve!: (value: typeof session) => void;
  vi.mocked(api.getSession).mockReturnValue(new Promise(done => { resolve = done; }));
  const view = render(<ProjectWork projectId="p-1" />);
  await userEvent.click(await screen.findByRole('button', { name: 'Open coding session' }));
  access.canRead = false; view.rerender(<ProjectWork projectId="p-1" />);
  await act(async () => resolve(session));
  expect(view.container.querySelector<HTMLDetailsElement>('.project-work')?.open).toBe(true);
  expect(useCockpit.getState().selection.sessionId).toBeNull();
  expect(useCockpit.getState().sessions[session.id]).toBeUndefined();
});

it('preserves a newer same-project branch choice when an old session lookup returns', async () => {
  const session = codingWorkFixture(); let resolve!: (value: typeof session) => void;
  vi.mocked(api.getSession).mockReturnValue(new Promise(done => { resolve = done; }));
  const view = render(<ProjectWork projectId="p-1" />);
  await userEvent.click(await screen.findByRole('button', { name: 'Open coding session' }));
  act(() => useCockpit.getState().selectBranch('p-1', 'b-chosen-later'));
  await act(async () => resolve(session));
  expect(view.container.querySelector<HTMLDetailsElement>('.project-work')?.open).toBe(true);
  expect(useCockpit.getState().selection.branchId).toBe('b-chosen-later');
  expect(useCockpit.getState().selection.sessionId).toBeNull();
});
