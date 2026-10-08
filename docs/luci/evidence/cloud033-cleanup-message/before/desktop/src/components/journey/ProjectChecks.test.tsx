import { act, cleanup, fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { beforeEach, expect, it, vi } from 'vitest';
import { api } from '../../api';
import { ApiError, type ProblemCode } from '../../api/client';
import { aBranch, aProject, aSession, aWorktree } from '../../test/fixtures';
import { productApi, type CheckRun, type RequirementCoverage } from '../../api/product';
import { projectRunsApi, type ProjectRun } from '../../api/projectRuns';
import { resetCockpit, useCockpit } from '../../store/store';
import { ProjectChecks } from './ProjectChecks';
import { notifyRequirementEvidenceChanged } from './RequirementsEvidence';
import { ProjectWork } from './ProjectWork';
import * as authentication from '../auth/AuthProvider';
import { checkSetupDraftValid } from '../../hooks/projectDraftValidation';
import { definitionApi } from '../../api/definition';
import { checkRecoveryFixture, checkRunFixture } from "../../test/checkFixtures";
import { designPlanApi } from '../../api/designPlan';

const identity = vi.hoisted(() => ({ scope: 'owner-workspace' }));
const access = vi.hoisted(() => ({ canRead: true, canEdit: true, canExecute: true, canAssist: true, message: '' }));
vi.mock('../../hooks/useProjectAccess', () => ({ useProjectAccess: () => access }));
vi.mock('../../hooks/useProjectDraft', async original => ({ ...await original<typeof import('../../hooks/useProjectDraft')>(), useProjectDraftScope: () => identity.scope }));
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
  vi.restoreAllMocks(); localStorage.clear(); resetCockpit(); identity.scope = 'owner-workspace'; access.canRead = true; access.canEdit = true; access.canExecute = true;
  vi.spyOn(productApi, 'checks').mockResolvedValue({ runs: [], suggested: commands, coverage });
  vi.spyOn(projectRunsApi, 'get').mockResolvedValue(build);
  vi.spyOn(productApi, 'runChecks').mockResolvedValue(passing);
  vi.spyOn(projectRunsApi, 'joinCheck').mockResolvedValue({ ...build, revision: 7, state: 'awaiting-release', allowedActions: ['join-release'] });
  vi.spyOn(definitionApi, 'get').mockResolvedValue({ projectId: 'p-1', revision: 4, briefRevision: 2, status: 'approved' } as never);
  vi.spyOn(designPlanApi, 'get').mockResolvedValue({ projectId: 'p-1', revision: 5, briefRevision: 2, specificationRevision: 4, sketchRevision: 1, status: 'approved', screens: [] } as never);
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
  expect(view.container.querySelector('details.project-work')).toBeNull();
  expect(screen.queryByRole('button', { name: 'Open coding session' })).toBeNull();
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

const unfinishedArguments = ' [ "run", "acceptance';
async function editCheckSetup() {
  await screen.findByRole('button', { name: 'Remove Acceptance tests' });
  const advanced = screen.getByText('Advanced check commands').closest('details')!;
  if (!advanced.open) await userEvent.click(screen.getByText('Advanced check commands'));
  fireEvent.change(screen.getByRole('textbox', { name: 'Executable' }), { target: { value: 'node' } });
  fireEvent.change(screen.getByRole('textbox', { name: 'Arguments · JSON array' }), { target: { value: '["scripts/check-notes.mjs"]' } });
  await userEvent.click(screen.getByRole('button', { name: 'Add check command' }));
  fireEvent.change(screen.getByRole('textbox', { name: 'Executable' }), { target: { value: '  npm  ' } });
  fireEvent.change(screen.getByRole('textbox', { name: 'Arguments · JSON array' }), { target: { value: unfinishedArguments } });
  const requirements = screen.getByText(/Link checks to approved requirements/).closest('details')!;
  if (!requirements.open) await userEvent.click(screen.getByText(/Link checks to approved requirements/));
  await userEvent.click(screen.getByRole('checkbox', { name: 'Check R-KEEP · Keep notes' }));
}
async function restoreCheckSetup() {
  const recovery = await screen.findByRole('region', { name: 'Verification setup draft recovery' });
  await waitFor(() => expect((within(recovery).getByRole('button', { name: 'Restore draft' }) as HTMLButtonElement).disabled).toBe(false));
  await userEvent.click(within(recovery).getByRole('button', { name: 'Restore draft' }));
}
function storedCheckSetup() {
  const key = Object.keys(localStorage).find(key => key.includes('verification-setup'))!;
  return JSON.parse(localStorage.getItem(key)!);
}

it('recovers custom commands, exact unfinished arguments and same-context coverage after remount, without host mode, trust or automatic execution', async () => {
  vi.spyOn(authentication, 'useAuth').mockReturnValue({ profile: { user: { id: 'owner' }, permissions: { runHostCommands: true }, policy: { allowHostCommands: true } } } as never);
  const view = render(<ProjectChecks projectId="p-1" />);
  await editCheckSetup();
  expect(screen.queryByRole('combobox', { name: 'Check execution' })).toBeNull();
  await userEvent.click(screen.getByRole('checkbox', { name: /I trust this project's code/ }));
  const stored = storedCheckSetup();
  expect(stored.value).toEqual({ commands: [...commands, { label: 'node', executable: 'node', args: ['scripts/check-notes.mjs'] }], executable: '  npm  ', args: unfinishedArguments, requirementIds: ['R-KEEP'], buildId: '' });
  expect(checkSetupDraftValid({ ...stored.value, trusted: true })).toBe(false);
  expect(checkSetupDraftValid({ ...stored.value, execution: 'host' })).toBe(false);
  view.unmount(); render(<ProjectChecks projectId="p-1" />);
  const recovery = await screen.findByRole('region', { name: 'Verification setup draft recovery' });
  await userEvent.click(within(recovery).getByText('Compare draft with current check setup'));
  expect(await within(recovery).findAllByText('Arguments being edited · JSON text')).toHaveLength(2);
  expect(within(recovery).getByText((_text, element) => element?.className === 'draft-value' && element.textContent === unfinishedArguments).textContent).toBe(unfinishedArguments);
  expect(productApi.runChecks).not.toHaveBeenCalled();
  await restoreCheckSetup();
  expect((screen.getByRole('textbox', { name: 'Executable' }) as HTMLInputElement).value).toBe('  npm  ');
  expect((screen.getByRole('textbox', { name: 'Arguments · JSON array' }) as HTMLInputElement).value).toBe(unfinishedArguments);
  expect(screen.getByRole('button', { name: 'Remove node' })).not.toBeNull();
  await userEvent.click(screen.getByText(/Link checks to approved requirements/));
  expect((screen.getByRole('checkbox', { name: 'Check R-KEEP · Keep notes' }) as HTMLInputElement).checked).toBe(true);
  expect(screen.queryByRole('combobox', { name: 'Check execution' })).toBeNull();
  expect((screen.getByRole('checkbox', { name: /I trust this project's code/ }) as HTMLInputElement).checked).toBe(false);
  expect((screen.getByRole('button', { name: 'Run selected requirement checks' }) as HTMLButtonElement).disabled).toBe(true);
  expect(productApi.runChecks).not.toHaveBeenCalled();
  await userEvent.click(screen.getByRole('checkbox', { name: /I trust this project's code/ }));
  await userEvent.click(screen.getByRole('button', { name: 'Run selected requirement checks' }));
  expect(productApi.runChecks).toHaveBeenCalledWith('p-1', stored.value.commands, true, 'docker', { requirementIds: ['R-KEEP'], expectedSpecificationRevision: 4, expectedDesignPlanRevision: 5 });
  expect((screen.getByRole('textbox', { name: 'Arguments · JSON array' }) as HTMLInputElement).value).toBe(unfinishedArguments);
});

it.each([
  ['source', { ...coverage, source: { ...coverage.source, commit: 'b'.repeat(40), sourceDigest: 'changed-source' } }],
  ['approvals', { ...coverage, specification: { ...coverage.specification, revision: 6, digest: 'reviewed-again' } }],
])('preserves text but requires review and clears recovered criteria after %s changes', async (_name, current) => {
  const view = render(<ProjectChecks projectId="p-1" />); await editCheckSetup(); view.unmount();
  vi.mocked(productApi.checks).mockResolvedValue({ runs: [], suggested: commands, coverage: current });
  render(<ProjectChecks projectId="p-1" />);
  const recovery = await screen.findByRole('region', { name: 'Verification setup draft recovery' });
  const review = await within(recovery).findByRole('checkbox', { name: /I reviewed the current check setup/ });
  expect((within(recovery).getByRole('button', { name: 'Restore draft' }) as HTMLButtonElement).disabled).toBe(true);
  await userEvent.click(review); await restoreCheckSetup();
  await screen.findByText(/Commands and unfinished arguments recovered/);
  expect((screen.getByRole('textbox', { name: 'Arguments · JSON array' }) as HTMLInputElement).value).toBe(unfinishedArguments);
  await userEvent.click(screen.getByText(/Link checks to approved requirements/));
  expect((screen.getByRole('checkbox', { name: 'Check R-KEEP · Keep notes' }) as HTMLInputElement).checked).toBe(false);
  expect(productApi.runChecks).not.toHaveBeenCalled();
});

it('re-reads the saved build reference after recovery before enabling its restored criteria, with no allocation', async () => {
  queue(); const view = render(<ProjectChecks projectId="p-1" />); await screen.findByText('Build journal');
  await editCheckSetup(); view.unmount();
  let resolve!: (value: ProjectRun) => void;
  vi.mocked(projectRunsApi.get).mockReturnValue(new Promise(done => { resolve = done; }));
  render(<ProjectChecks projectId="p-1" />); await restoreCheckSetup();
  expect(projectRunsApi.get).toHaveBeenLastCalledWith('p-1', build.id);
  expect(screen.getByText('Reading the reviewed build…')).not.toBeNull();
  expect((screen.getByRole('button', { name: 'Run selected requirement checks' }) as HTMLButtonElement).disabled).toBe(true);
  await act(async () => resolve(build));
  await screen.findByText('Build journal');
  expect((screen.getByRole('checkbox', { name: 'Check R-KEEP · Keep notes' }) as HTMLInputElement).checked).toBe(true);
  expect((screen.getByRole('checkbox', { name: /I trust this project's code/ }) as HTMLInputElement).checked).toBe(false);
  expect(productApi.runChecks).not.toHaveBeenCalled(); expect(projectRunsApi.joinCheck).not.toHaveBeenCalled();
});

it('does not transfer old criteria to a newer explicitly selected build when restoring command text', async () => {
  queue(); const view = render(<ProjectChecks projectId="p-1" />); await screen.findByText('Build journal'); await editCheckSetup(); view.unmount();
  const next = { ...build, id: 'run-new' };
  vi.mocked(projectRunsApi.get).mockResolvedValue(next);
  useCockpit.setState({ projectRunVerification: { scope: identity.scope, projectId: 'p-1', runId: next.id } });
  render(<ProjectChecks projectId="p-1" />); await screen.findByText('Build journal');
  await userEvent.click(await screen.findByRole('checkbox', { name: /I reviewed the current check setup/ }));
  await restoreCheckSetup(); await screen.findByText(/Commands and unfinished arguments recovered/);
  expect((screen.getByRole('checkbox', { name: 'Check R-KEEP · Keep notes' }) as HTMLInputElement).checked).toBe(false);
  expect(projectRunsApi.get).toHaveBeenLastCalledWith('p-1', next.id);
  expect(productApi.runChecks).not.toHaveBeenCalled();
});

it.each(['another-account', 'another-team-workspace', 'another-project'])('isolates retained check setup from %s and recovers it only in the original scope', async scope => {
  const view = render(<ProjectChecks projectId="p-1" />); await editCheckSetup();
  identity.scope = scope;
  const projectId = scope === 'another-project' ? 'p-2' : 'p-1';
  vi.mocked(productApi.checks).mockResolvedValue({ runs: [], suggested: commands, coverage: { ...coverage, projectId } });
  view.rerender(<ProjectChecks projectId={projectId} />);
  await screen.findByRole('button', { name: 'Remove Acceptance tests' });
  expect(screen.queryByRole('region', { name: 'Verification setup draft recovery' })).toBeNull();
  expect((screen.getByRole('textbox', { name: 'Arguments · JSON array' }) as HTMLInputElement).value).toBe('[]');
  identity.scope = 'owner-workspace'; vi.mocked(productApi.checks).mockResolvedValue({ runs: [], suggested: commands, coverage });
  view.rerender(<ProjectChecks projectId="p-1" />); await restoreCheckSetup();
  expect((screen.getByRole('textbox', { name: 'Arguments · JSON array' }) as HTMLInputElement).value).toBe(unfinishedArguments);
  expect(productApi.runChecks).not.toHaveBeenCalled();
});

it('preserves edits made before a delayed suggestions response and an intentionally empty command list on refresh', async () => {
  let resolve!: (value: Awaited<ReturnType<typeof productApi.checks>>) => void;
  vi.mocked(productApi.checks).mockReturnValueOnce(new Promise(done => { resolve = done; }));
  render(<ProjectChecks projectId="p-1" />);
  fireEvent.change(screen.getByRole('textbox', { name: 'Executable' }), { target: { value: 'node' } });
  fireEvent.change(screen.getByRole('textbox', { name: 'Arguments · JSON array' }), { target: { value: '["custom.mjs"]' } });
  await userEvent.click(screen.getByRole('button', { name: 'Add check command' }));
  await act(async () => resolve({ runs: [], suggested: commands, coverage }));
  expect(screen.getByRole('button', { name: 'Remove node' })).not.toBeNull();
  expect(screen.queryByRole('button', { name: 'Remove Acceptance tests' })).toBeNull();
  await userEvent.click(screen.getByRole('button', { name: 'Remove node' }));
  await userEvent.click(screen.getByRole('button', { name: 'Refresh requirement evidence' }));
  expect(screen.queryByRole('button', { name: 'Remove Acceptance tests' })).toBeNull();
  expect(storedCheckSetup().value.commands).toEqual([]);
  expect(productApi.runChecks).not.toHaveBeenCalled();
});

it('discards local setup explicitly without running it and resets to current suggestions without retaining consent', async () => {
  const view = render(<ProjectChecks projectId="p-1" />); await editCheckSetup(); view.unmount(); render(<ProjectChecks projectId="p-1" />);
  const recovery = await screen.findByRole('region', { name: 'Verification setup draft recovery' });
  await waitFor(() => expect((within(recovery).getByRole('button', { name: 'Discard browser draft' }) as HTMLButtonElement).disabled).toBe(false));
  await userEvent.click(within(recovery).getByRole('button', { name: 'Discard browser draft' }));
  expect(screen.getByRole('button', { name: 'Remove Acceptance tests' })).not.toBeNull();
  expect(screen.queryByRole('button', { name: 'Remove node' })).toBeNull();
  expect((screen.getByRole('textbox', { name: 'Arguments · JSON array' }) as HTMLInputElement).value).toBe('[]');
  expect(Object.keys(localStorage).filter(key => key.includes('verification-setup'))).toEqual([]);
  expect(productApi.runChecks).not.toHaveBeenCalled();
});

it('keeps browser recovery read-only after execution access is removed', async () => {
  const view = render(<ProjectChecks projectId="p-1" />); await editCheckSetup(); view.unmount();
  access.canEdit = false; access.canExecute = false;
  render(<ProjectChecks projectId="p-1" />);
  const recovery = await screen.findByRole('region', { name: 'Verification setup draft recovery' });
  await within(recovery).findByRole('button', { name: 'Restore draft' });
  expect((within(recovery).getByRole('button', { name: 'Restore draft' }) as HTMLButtonElement).disabled).toBe(true);
  expect((screen.getByRole('checkbox', { name: /I trust this project's code/ }) as HTMLInputElement).disabled).toBe(true);
  expect(productApi.runChecks).not.toHaveBeenCalled();
});

it('keeps recovered text when a saved build is missing, and ignores a delayed build from the previous workspace', async () => {
  queue(); const view = render(<ProjectChecks projectId="p-1" />); await screen.findByText('Build journal'); await editCheckSetup(); view.unmount();
  let resolve!: (value: ProjectRun) => void;
  vi.mocked(projectRunsApi.get).mockReturnValue(new Promise(done => { resolve = done; }));
  const nextView = render(<ProjectChecks projectId="p-1" />); await restoreCheckSetup();
  identity.scope = 'another-workspace'; nextView.rerender(<ProjectChecks projectId="p-1" />);
  await act(async () => resolve(build));
  expect(screen.queryByText('Build journal')).toBeNull();
  expect(screen.queryByRole('region', { name: 'Build verification' })).toBeNull();
  identity.scope = 'owner-workspace'; vi.mocked(projectRunsApi.get).mockRejectedValue(new Error('The recorded build is no longer available.'));
  nextView.rerender(<ProjectChecks projectId="p-1" />); await restoreCheckSetup();
  await screen.findByText('The recorded build is no longer available.');
  await userEvent.click(screen.getByText('Advanced check commands'));
  expect((screen.getByRole('textbox', { name: 'Arguments · JSON array' }) as HTMLInputElement).value).toBe(unfinishedArguments);
  expect((screen.getByRole('button', { name: 'Run selected requirement checks' }) as HTMLButtonElement).disabled).toBe(true);
  expect(productApi.runChecks).not.toHaveBeenCalled();
});

it.each([false, true])('retains the recorded command template while protecting edits made during a late run response (new text: %s)', async lateEdit => {
  let resolve!: (value: CheckRun) => void;
  vi.mocked(productApi.runChecks).mockReturnValue(new Promise(done => { resolve = done; }));
  render(<ProjectChecks projectId="p-1" />); await screen.findByRole('button', { name: 'Remove Acceptance tests' });
  fireEvent.change(screen.getByRole('textbox', { name: 'Executable' }), { target: { value: 'node' } });
  fireEvent.change(screen.getByRole('textbox', { name: 'Arguments · JSON array' }), { target: { value: '["custom.mjs"]' } });
  await userEvent.click(screen.getByRole('button', { name: 'Add check command' }));
  await userEvent.click(screen.getByRole('checkbox', { name: /I trust this project's code/ }));
  await userEvent.click(screen.getByRole('button', { name: 'Run project checks' }));
  if (lateEdit) fireEvent.change(screen.getByRole('textbox', { name: 'Arguments · JSON array' }), { target: { value: unfinishedArguments } });
  await act(async () => resolve(passing));
  expect(await screen.findAllByText('Commands passed')).toHaveLength(2);
  expect(Object.values(useCockpit.getState().unsavedProjectDrafts).some(draft => draft.label === 'Verification setup')).toBe(lateEdit);
  expect(screen.getByText(/Check setup · kept in this browser/)).not.toBeNull();
  expect(screen.queryByText(/^Unsaved changes/)).toBeNull();
  expect(storedCheckSetup().value.commands).toEqual([...commands, { label: 'node', executable: 'node', args: ['custom.mjs'] }]);
  expect(storedCheckSetup().value.args).toBe(lateEdit ? unfinishedArguments : '[]');
  expect(productApi.runChecks).toHaveBeenCalledTimes(1);
});

it('keeps the setup protected when the check request has an uncertain outcome', async () => {
  vi.mocked(productApi.runChecks).mockRejectedValue(new Error('Connection lost before a receipt arrived.'));
  render(<ProjectChecks projectId="p-1" />); await editCheckSetup();
  await userEvent.click(screen.getByRole('checkbox', { name: /I trust this project's code/ }));
  await userEvent.click(screen.getByRole('button', { name: 'Run selected requirement checks' }));
  await screen.findByText('Connection lost before a receipt arrived.');
  expect(Object.values(useCockpit.getState().unsavedProjectDrafts).some(draft => draft.label === 'Verification setup')).toBe(true);
  expect(storedCheckSetup().value.args).toBe(unfinishedArguments);
  expect(productApi.runChecks).toHaveBeenCalledTimes(1);
});

it('reports oversized unfinished text as unretained without truncating it or promising recovery that validation would reject', async () => {
  render(<ProjectChecks projectId="p-1" />); await screen.findByRole('button', { name: 'Remove Acceptance tests' });
  const raw = ' ["' + 'a'.repeat(70000);
  fireEvent.change(screen.getByRole('textbox', { name: 'Arguments · JSON array' }), { target: { value: raw } });
  await screen.findByText(/This check setup exceeds the browser recovery limits/);
  expect((screen.getByRole('textbox', { name: 'Arguments · JSON array' }) as HTMLInputElement).value).toBe(raw);
  expect(screen.queryByText(/kept in this browser for up to 30 days/)).toBeNull();
  expect(Object.keys(localStorage).filter(key => key.includes('verification-setup'))).toEqual([]);
  expect(Object.values(useCockpit.getState().unsavedProjectDrafts).find(draft => draft.label === 'Verification setup')?.retained).toBe(false);
  expect(productApi.runChecks).not.toHaveBeenCalled();
});


it('retains cancellation with Edit when Execute is revoked and keeps new checks unavailable',async()=>{
  const run=checkRunFixture({status:'running',stage:'observe'});vi.mocked(productApi.checks).mockResolvedValue({runs:[run],suggested:commands,coverage});
  const cancel=vi.spyOn(productApi,'cancelChecks').mockResolvedValue({...run,revision:5,status:'cancelled',resourcesHeld:false});
  access.canExecute=false;render(<ProjectChecks projectId="p-1"/>);
  const stop=await screen.findByRole('button',{name:'Cancel checks'});expect(stop).toHaveProperty('disabled',false);
  await userEvent.click(stop);await waitFor(()=>expect(cancel).toHaveBeenCalledExactlyOnceWith('p-1',run.id));expect(productApi.runChecks).not.toHaveBeenCalled();
});
it('retains earlier output for admission-blocked work without turning it into passing build evidence',async()=>{
  const run=checkRunFixture({status:'blocked',stage:'admission',reasonCode:'capacity_unavailable',resourcesHeld:false,message:'Available memory is below the configured reserve.'});
  vi.mocked(productApi.checks).mockResolvedValue({runs:[run],suggested:commands,coverage});queue();render(<ProjectChecks projectId="p-1"/>);
  await screen.findByText('Checks blocked');expect(screen.getByText('Retained test output')).toBeTruthy();expect(screen.getByText(/No blocked command will retry automatically/)).toBeTruthy();expect(screen.queryByRole('button',{name:'Use these results for this build'})).toBeNull();expect(productApi.runChecks).not.toHaveBeenCalled();
});
it('requires a history refresh and fresh trust after an unconfirmed check start receipt',async()=>{
  vi.mocked(productApi.runChecks).mockRejectedValueOnce(new Error('Connection interrupted'));
  render(<ProjectChecks projectId="p-1"/>);await screen.findByRole('button',{name:'Remove Acceptance tests'});
  await userEvent.click(screen.getByRole('checkbox',{name:/I trust this project's code/}));
  await userEvent.click(screen.getByRole('button',{name:'Run project checks'}));
  await screen.findByText(/check start result is unconfirmed/);expect(screen.getByRole('button',{name:'Run project checks'})).toHaveProperty('disabled',true);
  await userEvent.click(screen.getByRole('button',{name:'Refresh check history'}));await waitFor(()=>expect(screen.queryByRole('button',{name:'Refresh check history'})).toBeNull());
  expect(screen.getByRole('checkbox',{name:/I trust this project's code/})).toHaveProperty('checked',false);expect(productApi.runChecks).toHaveBeenCalledOnce();
});


it('continues bounded visible observations beyond four rounds and stops naturally when checks settle',async()=>{
  vi.useFakeTimers();
  try {
    vi.spyOn(document,'visibilityState','get').mockReturnValue('visible');
    const pending=['a','b','c','d','e','f'].map(id=>checkRunFixture({id:'check-'+id,status:'running',stage:'observe'}));
    vi.mocked(productApi.checks).mockResolvedValue({runs:pending,suggested:commands,coverage});
    const get=vi.spyOn(productApi,'checkRun').mockImplementation(async(_project,id)=>pending.find(run=>run.id===id)!);
    render(<ProjectChecks projectId="p-1"/>);await act(async()=>{});
    await act(async()=>vi.advanceTimersByTimeAsync(5000));expect(get).toHaveBeenCalledTimes(4);
    await act(async()=>vi.advanceTimersByTimeAsync(5000));expect(get).toHaveBeenCalledTimes(8);
    expect(new Set(get.mock.calls.map(call=>call[1])).size).toBe(6);
    await act(async()=>vi.advanceTimersByTimeAsync(20_000));expect(get).toHaveBeenCalledTimes(24);
    const settled=pending.map(run=>({...run,revision:5,status:'passed',resourcesHeld:false,stage:'finished',commands:run.commands.map(command=>({...command,cleanupConfirmed:true}))}));
    get.mockImplementation(async(_project,id)=>settled.find(run=>run.id===id)!);vi.mocked(productApi.checks).mockResolvedValue({runs:settled,suggested:commands,coverage});
    await act(async()=>vi.advanceTimersByTimeAsync(5000));const stopped=get.mock.calls.length;
    await act(async()=>vi.advanceTimersByTimeAsync(30_000));expect(get).toHaveBeenCalledTimes(stopped);
    expect(screen.getAllByText('Commands passed')).toHaveLength(6);expect(productApi.runChecks).not.toHaveBeenCalled();
  } finally {cleanup();vi.useRealTimers();}
});
it('does not overlap a delayed observation and aborts it when the view is hidden',async()=>{
  vi.useFakeTimers();
  try {
    const visibility=vi.spyOn(document,'visibilityState','get').mockReturnValue('visible'),run=checkRunFixture({status:'running',stage:'observe'});
    vi.mocked(productApi.checks).mockResolvedValue({runs:[run],suggested:commands,coverage});
    let finish!:(run:CheckRun)=>void;const get=vi.spyOn(productApi,'checkRun').mockReturnValue(new Promise(resolve=>{finish=resolve;}));
    render(<ProjectChecks projectId="p-1"/>);await act(async()=>{});await act(async()=>vi.advanceTimersByTimeAsync(30_000));expect(get).toHaveBeenCalledOnce();
    visibility.mockReturnValue('hidden');fireEvent(document,new Event('visibilitychange'));expect(get.mock.calls[0][2]!.aborted).toBe(true);
    await act(async()=>finish({...run,revision:5,status:'passed',resourcesHeld:false}));expect(screen.queryByText('Commands passed')).toBeNull();
    await act(async()=>vi.advanceTimersByTimeAsync(30_000));expect(get).toHaveBeenCalledOnce();
  } finally {cleanup();vi.useRealTimers();}
});

it.each([
  ['check',401], ['check',403], ['availability',401], ['availability',403],
] as const)('clears current private check history on %s GET %s, gates controls, and retains unsent setup for an explicit fresh read',async (endpoint,status)=>{
  const run=checkRunFixture({message:'Alpha retained private history'});
  vi.mocked(productApi.checks).mockResolvedValue({runs:[run],suggested:commands,coverage});
  const denial=new ApiError('permission_denied' as ProblemCode,'Reading access changed.',status);
  const read=vi.spyOn(productApi,'checkRun').mockResolvedValue(run);
  if(endpoint==='check') read.mockRejectedValue(denial);
  else vi.spyOn(productApi,'checkRecovery').mockRejectedValue(denial);
  const recover=vi.spyOn(productApi,'recoverCheck');
  render(<ProjectChecks projectId="p-1"/>);await screen.findByText('Alpha retained private history');
  await userEvent.type(screen.getByRole('textbox',{name:'Executable'}),'unfinished-command');
  await userEvent.click(screen.getByRole('button',{name:'Review check recovery'}));
  const refresh=await screen.findByRole('button',{name:'Refresh check access'});
  await waitFor(()=>expect(document.activeElement).toBe(refresh));
  expect(screen.queryByText('Alpha retained private history')).toBeNull();expect(screen.queryByText('Retained test output')).toBeNull();
  expect(screen.queryByRole('dialog')).toBeNull();expect(screen.queryByRole('button',{name:'Run project checks'})).toBeNull();
  expect(screen.queryByText('Keep notes')).toBeNull();expect(productApi.checks).toHaveBeenCalledOnce();
  expect(productApi.runChecks).not.toHaveBeenCalled();expect(recover).not.toHaveBeenCalled();
  await userEvent.click(screen.getByRole('button',{name:'Refresh check access'}));
  await screen.findByText('Alpha retained private history');
  expect(screen.getByRole('textbox',{name:'Executable'})).toHaveProperty('value','unfinished-command');
  expect(productApi.checks).toHaveBeenCalledTimes(2);expect(productApi.runChecks).not.toHaveBeenCalled();
});
it('fences an inflight history read on confirmed denial and requires a successful explicit access refresh',async()=>{
  const run=checkRunFixture({message:'Private history before denial'});
  const history={runs:[run],suggested:commands,coverage};
  vi.mocked(productApi.checks).mockResolvedValue(history);
  let finishHistory!:(value:typeof history)=>void;
  vi.spyOn(productApi,'checkRun').mockRejectedValue(new ApiError('permission_denied' as ProblemCode,'Reading access changed.',403));
  render(<ProjectChecks projectId="p-1"/>);await screen.findByText(run.message);
  vi.mocked(productApi.checks).mockReturnValueOnce(new Promise(resolve=>{finishHistory=resolve;}));
  act(()=>notifyRequirementEvidenceChanged('p-1'));
  await waitFor(()=>expect(productApi.checks).toHaveBeenCalledTimes(2));
  const historySignal=vi.mocked(productApi.checks).mock.calls[1][1];
  await userEvent.click(screen.getByRole('button',{name:'Review check recovery'}));
  await screen.findByRole('button',{name:'Refresh check access'});
  expect(historySignal?.aborted).toBe(true);
  await act(async()=>finishHistory(history));
  act(()=>notifyRequirementEvidenceChanged('p-1'));
  expect(productApi.checks).toHaveBeenCalledTimes(2);expect(screen.queryByText(run.message)).toBeNull();
  vi.mocked(productApi.checks).mockRejectedValueOnce(new Error('Connection interrupted'));
  await userEvent.click(screen.getByRole('button',{name:'Refresh check access'}));
  await waitFor(()=>expect(screen.getByRole('button',{name:'Refresh check access'})).toHaveProperty('disabled',false));
  expect(screen.queryByText(run.message)).toBeNull();expect(screen.queryByRole('button',{name:'Run project checks'})).toBeNull();
  await userEvent.click(screen.getByRole('button',{name:'Refresh check access'}));
  await screen.findByText(run.message);
  expect(productApi.checks).toHaveBeenCalledTimes(4);expect(productApi.runChecks).not.toHaveBeenCalled();
});
it('handles a fresh recovery read denial after Edit changes while a history refresh is already pending',async()=>{
  const run=checkRunFixture({message:'Private history during authority refresh'});
  vi.mocked(productApi.checks).mockResolvedValue({runs:[run],suggested:commands,coverage});
  const read=vi.spyOn(productApi,'checkRun').mockResolvedValue(run);
  vi.spyOn(productApi,'checkRecovery').mockResolvedValue(checkRecoveryFixture());
  const view=render(<ProjectChecks projectId="p-1"/>);
  await userEvent.click(await screen.findByRole('button',{name:'Review check recovery'}));
  await screen.findByRole('checkbox',{name:/I authorize recovery/});
  vi.mocked(productApi.checks).mockReturnValue(new Promise(()=>{}));
  act(()=>notifyRequirementEvidenceChanged('p-1'));
  await waitFor(()=>expect(productApi.checks).toHaveBeenCalledTimes(2));
  read.mockRejectedValueOnce(new ApiError('permission_denied' as ProblemCode,'Fresh reading access denied.',403));
  access.canEdit=false;view.rerender(<ProjectChecks projectId="p-1"/>);
  await screen.findByRole('button',{name:'Refresh check access'});
  expect(read).toHaveBeenCalledTimes(2);expect(screen.queryByRole('dialog')).toBeNull();
  expect(screen.queryByText(run.message)).toBeNull();expect(screen.queryByText('Retained test output')).toBeNull();
  expect(productApi.runChecks).not.toHaveBeenCalled();
});
it('ignores an old project recovery read denial after switching to another project',async()=>{
  const alpha=checkRunFixture({message:'Alpha retained history'}),beta=checkRunFixture({id:'check-beta',projectId:'p-2',message:'Beta retained history'});
  vi.mocked(productApi.checks).mockImplementation(async id=>({runs:[id==='p-1'?alpha:beta],suggested:commands,coverage:{...coverage,projectId:id}}));
  let refuse!:(error:Error)=>void;const read=vi.spyOn(productApi,'checkRun').mockReturnValue(new Promise((_resolve,reject)=>{refuse=reject;}));
  const view=render(<ProjectChecks projectId="p-1"/>);await userEvent.click(await screen.findByRole('button',{name:'Review check recovery'}));
  await waitFor(()=>expect(read).toHaveBeenCalledOnce());view.rerender(<ProjectChecks projectId="p-2"/>);
  await screen.findByText('Beta retained history');await act(async()=>refuse(new ApiError('permission_denied' as ProblemCode,'Old project access changed.',403)));
  expect(screen.getByText('Beta retained history')).toBeTruthy();expect(screen.queryByRole('button',{name:'Refresh check access'})).toBeNull();
  expect(screen.queryByText('Alpha retained history')).toBeNull();expect(productApi.runChecks).not.toHaveBeenCalled();
});


it.each(['uncertain rejection', 'discarded success'] as const)('LOCAL-015 retains the start barrier after Edit churn and %s until later explicit history and fresh consent', async outcome => {
  let resolve!: (run: CheckRun) => void, reject!: (error: Error) => void;
  vi.mocked(productApi.runChecks).mockReturnValueOnce(new Promise((done, fail) => { resolve = done; reject = fail; }));
  const view = render(<ProjectChecks projectId="p-1" />);
  await screen.findByRole('button', { name: 'Remove Acceptance tests' });
  await userEvent.click(screen.getByRole('checkbox', { name: /I trust this project's code/ }));
  await userEvent.click(screen.getByRole('button', { name: 'Run project checks' }));
  access.canEdit = false; view.rerender(<ProjectChecks projectId="p-1" />);
  await waitFor(() => expect(productApi.checks).toHaveBeenCalledTimes(2));
  await waitFor(() => expect(screen.getByRole('button', { name: 'Refresh requirement evidence' })).toHaveProperty('disabled', false));
  access.canEdit = true; view.rerender(<ProjectChecks projectId="p-1" />);
  await waitFor(() => expect(productApi.checks).toHaveBeenCalledTimes(3));
  await waitFor(() => expect(screen.getByRole('button', { name: 'Refresh requirement evidence' })).toHaveProperty('disabled', false));
  expect(screen.getByText('No verification runs recorded yet.')).toBeTruthy();
  expect(productApi.runChecks).toHaveBeenCalledOnce();
  await act(async () => {
    if (outcome === 'uncertain rejection') reject(new ApiError('request_timeout' as ProblemCode, 'Private old-authority failure detail', 0));
    else resolve({ ...passing, message: 'Private old-authority successful receipt' });
  });
  expect(screen.getByRole('button', { name: 'Run project checks' })).toHaveProperty('disabled', true);
  expect(screen.getByText(/check start result is unconfirmed/)).toBeTruthy();
  expect(screen.getByRole('checkbox', { name: /I trust this project's code/ })).toHaveProperty('checked', false);
  expect(screen.queryByText(/Private old-authority/)).toBeNull();
  act(() => notifyRequirementEvidenceChanged('p-1'));
  await waitFor(() => expect(productApi.checks).toHaveBeenCalledTimes(4));
  await waitFor(() => expect(screen.getByRole('button', { name: 'Refresh check history' })).toHaveProperty('disabled', false));
  expect(screen.getByRole('button', { name: 'Run project checks' })).toHaveProperty('disabled', true);
  vi.mocked(productApi.checks).mockRejectedValueOnce(new Error('History still unavailable'));
  await userEvent.click(screen.getByRole('button', { name: 'Refresh check history' }));
  await screen.findByText('History still unavailable');
  expect(screen.getByText(/check start result is unconfirmed/)).toBeTruthy();
  await userEvent.click(screen.getByRole('button', { name: 'Refresh check history' }));
  await waitFor(() => expect(screen.queryByRole('button', { name: 'Refresh check history' })).toBeNull());
  const consent = screen.getByRole('checkbox', { name: /I trust this project's code/ });
  expect(consent).toHaveProperty('checked', false);
  expect(screen.getByRole('button', { name: 'Run project checks' })).toHaveProperty('disabled', true);
  expect(productApi.runChecks).toHaveBeenCalledOnce();
  await userEvent.click(consent);
  expect(screen.getByRole('button', { name: 'Run project checks' })).toHaveProperty('disabled', false);
  await userEvent.click(screen.getByRole('button', { name: 'Run project checks' }));
  await waitFor(() => expect(productApi.runChecks).toHaveBeenCalledTimes(2));
});

it('LOCAL-015 does not use an explicit history read begun before start settlement to clear uncertainty', async () => {
  let reject!: (error: Error) => void;
  vi.mocked(productApi.runChecks).mockReturnValueOnce(new Promise((_done, fail) => { reject = fail; }));
  render(<ProjectChecks projectId="p-1" />);
  await screen.findByRole('button', { name: 'Remove Acceptance tests' });
  await userEvent.click(screen.getByRole('checkbox', { name: /I trust this project's code/ }));
  await userEvent.click(screen.getByRole('button', { name: 'Run project checks' }));
  let history!: (value: Awaited<ReturnType<typeof productApi.checks>>) => void;
  vi.mocked(productApi.checks).mockReturnValueOnce(new Promise(done => { history = done; }));
  await userEvent.click(screen.getByRole('button', { name: 'Refresh requirement evidence' }));
  await act(async () => reject(new ApiError('request_timeout' as ProblemCode, 'Connection interrupted', 0)));
  expect(screen.getByText(/check start result is unconfirmed/)).toBeTruthy();
  await act(async () => history({ runs: [], suggested: commands, coverage }));
  expect(screen.getByText(/check start result is unconfirmed/)).toBeTruthy();
  expect(screen.getByRole('button', { name: 'Run project checks' })).toHaveProperty('disabled', true);
  await userEvent.click(screen.getByRole('button', { name: 'Refresh check history' }));
  await waitFor(() => expect(screen.queryByRole('button', { name: 'Refresh check history' })).toBeNull());
  expect(screen.getByRole('checkbox', { name: /I trust this project's code/ })).toHaveProperty('checked', false);
  expect(productApi.runChecks).toHaveBeenCalledOnce();
});

it('LOCAL-015 keeps read revocation private and retains start uncertainty after access returns', async () => {
  let resolve!: (run: CheckRun) => void;
  vi.mocked(productApi.runChecks).mockReturnValueOnce(new Promise(done => { resolve = done; }));
  const view = render(<ProjectChecks projectId="p-1" />);
  await screen.findByRole('button', { name: 'Remove Acceptance tests' });
  await userEvent.click(screen.getByRole('checkbox', { name: /I trust this project's code/ }));
  await userEvent.click(screen.getByRole('button', { name: 'Run project checks' }));
  access.canRead = false; view.rerender(<ProjectChecks projectId="p-1" />);
  await act(async () => resolve({ ...passing, message: 'Private revoked receipt' }));
  expect(screen.queryByText('Private revoked receipt')).toBeNull();
  expect(screen.queryByRole('button', { name: 'Run project checks' })).toBeNull();
  access.canRead = true; view.rerender(<ProjectChecks projectId="p-1" />);
  await screen.findByRole('button', { name: 'Refresh check history' });
  expect(screen.getByRole('button', { name: 'Run project checks' })).toHaveProperty('disabled', true);
  expect(screen.queryByText('Private revoked receipt')).toBeNull();
  expect(productApi.runChecks).toHaveBeenCalledOnce();
});

it.each(['project', 'account'] as const)('LOCAL-015 does not leak an old start result or barrier into another %s scope', async changedScope => {
  let reject!: (error: Error) => void;
  vi.mocked(productApi.runChecks).mockReturnValueOnce(new Promise((_done, fail) => { reject = fail; }));
  vi.mocked(productApi.checks).mockImplementation(async id => ({ runs: [], suggested: commands, coverage: { ...coverage, projectId: id } }));
  const view = render(<ProjectChecks projectId="p-1" />);
  await screen.findByRole('button', { name: 'Remove Acceptance tests' });
  await userEvent.click(screen.getByRole('checkbox', { name: /I trust this project's code/ }));
  await userEvent.click(screen.getByRole('button', { name: 'Run project checks' }));
  if (changedScope === 'account') identity.scope = 'another-owner';
  view.rerender(<ProjectChecks projectId={changedScope === 'project' ? 'p-2' : 'p-1'} />);
  await screen.findByRole('button', { name: 'Remove Acceptance tests' });
  await act(async () => reject(new ApiError('request_timeout' as ProblemCode, 'Private original scope error', 0)));
  expect(screen.queryByText('Private original scope error')).toBeNull();
  expect(screen.queryByText(/check start result is unconfirmed/)).toBeNull();
  await userEvent.click(screen.getByRole('checkbox', { name: /I trust this project's code/ }));
  expect(screen.getByRole('button', { name: 'Run project checks' })).toHaveProperty('disabled', false);
  expect(productApi.runChecks).toHaveBeenCalledOnce();
});
