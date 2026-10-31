import { act, cleanup, render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, beforeEach, expect, it, vi } from 'vitest';
import { PortfolioFocus } from './PortfolioFocus';
import { portfolioFocusApi, type FocusPlan } from '../../api/portfolioFocus';
import type { PortfolioGoal } from '../../api/portfolioGoals';
import { projectRunsApi, type ProjectRun } from '../../api/projectRuns';

vi.mock('../../hooks/useProjectDraft', () => ({ useProjectDraftScope: () => 'actor:workspace' }));
vi.mock('../../hooks/useProjectAccess', () => ({ useProjectAccess: () => ({ canRead: true, canEdit: true, loading: false, error: '', message: '' }) }));
vi.mock('../teams/TeamWorkspaceProvider', () => ({ useTeamWorkspace: () => null }));

const recorded = (projectId = 'p-two', suffix = 'a'): ProjectRun => {
  const hash = 'd'.repeat(64);
  return { id: `run-${suffix.repeat(24)}`, projectId, worktreeId: 'worktree-one', revision: 1,
    createdAt: '2026-10-31T00:00:00Z', updatedAt: '2026-10-31T00:00:00Z', state: 'complete', reason: '',
    input: { implementation: { projectId, worktreeId: 'worktree-one', briefRevision: 1, planRevision: 1, specificationRevision: 1, designPlanRevision: 1, taskIds: ['task-one'], taskTitles: ['Payment service contract'], requirementIds: [],
      briefDigest: hash, planDigest: hash, specificationDigest: hash, designPlanDigest: hash, sourceCommit: 'a'.repeat(40), sourceDigest: hash, sourceDirty: false },
      approvals: { specificationApprovalId: 'spec-one', specificationRevision: 1, specificationDigest: hash, designApprovalId: 'design-one', designRevision: 1, designDigest: hash, briefDigest: hash, sketchDigest: hash },
      outcome: 'preview', requirements: [], route: { harness: 'codex', model: 'test', permissionMode: 'bypass', connection: { kind: 'legacy', revision: 0 } } },
    operations: [], checks: [], releases: [], allowedActions: [],
  };
};

const goal: PortfolioGoal = { id: 'goal-one', revision: 2, title: 'Ship', outcome: 'Booking works', updatedAt: '2026-10-31T00:00:00Z', projects: [{ projectId: 'p-one', priority: 'normal', requestedSlots: 1, dependsOn: [] }] };
const projects = [{ id: 'p-one', name: 'Appointments', path: '/appointments' }];
const reviewed = (): FocusPlan => ({ expectedFocusRevision: 0, digest: 'a'.repeat(64), executionStarted: false, profile: { goalId: goal.id, goalRevision: goal.revision, goalDigest: 'b'.repeat(64), digest: 'c'.repeat(64), allocations: [{ projectId: 'p-one', maxActive: 1, waitUntilAccepted: [] }] } });
beforeEach(() => {
  vi.spyOn(projectRunsApi, 'list').mockResolvedValue([recorded()]);
  vi.spyOn(portfolioFocusApi, 'review').mockResolvedValue(reviewed());
  vi.spyOn(portfolioFocusApi, 'install').mockResolvedValue({ revision: 1, profile: reviewed().profile });
  vi.spyOn(portfolioFocusApi, 'remove').mockResolvedValue({ revision: 2 });
});
afterEach(() => { cleanup(); vi.restoreAllMocks(); });

it('requires a separate review and explicit installation before controlling queued work', async () => {
  render(<PortfolioFocus goal={goal} projects={projects} focus={{ revision: 0 }} disabled={false} />);
  const user = userEvent.setup();
  expect(portfolioFocusApi.install).not.toHaveBeenCalled();
  await user.click(screen.getByRole('button', { name: 'Review focus policy' }));
  await user.click(await screen.findByRole('button', { name: 'Install this focus policy' }));
  await waitFor(() => expect(portfolioFocusApi.install).toHaveBeenCalledTimes(1));
  expect(await screen.findByText(/Active focus revision 1/)).toBeTruthy();
  await user.click(screen.getByRole('button', { name: 'Remove active focus policy' }));
  await waitFor(() => expect(portfolioFocusApi.remove).toHaveBeenCalledTimes(1));
  expect(portfolioFocusApi.install).toHaveBeenCalledTimes(1);
});

it('retains unknown installation as unknown and never automatically repeats the request', async () => {
  vi.mocked(portfolioFocusApi.install).mockRejectedValue(new Error('Connection lost after save'));
  render(<PortfolioFocus goal={goal} projects={projects} focus={{ revision: 0 }} disabled={false} />);
  const user = userEvent.setup();
  await user.click(screen.getByRole('button', { name: 'Review focus policy' }));
  await user.click(await screen.findByRole('button', { name: 'Install this focus policy' }));
  expect(await screen.findByText(/policy change is unconfirmed/)).toBeTruthy();
  expect(portfolioFocusApi.install).toHaveBeenCalledTimes(1);
  expect(screen.getByRole('button', { name: 'Review focus policy' }).matches(':disabled')).toBe(true);
});

it('keeps dirty or unauthorized goal edits outside the control flow and requires an explicitly selected recorded run', async () => {
  const dependent = { ...goal, projects: [{ ...goal.projects[0], dependsOn: ['p-two'] }, { projectId: 'p-two', priority: 'normal' as const, requestedSlots: 0, dependsOn: [] }] };
  const all = [...projects, { id: 'p-two', name: 'Payments', path: '/payments' }];
  const { rerender } = render(<PortfolioFocus goal={dependent} projects={all} focus={{ revision: 0 }} disabled />);
  const user = userEvent.setup();
  await user.click(screen.getByRole('button', { name: 'Review focus policy' }));
  expect(portfolioFocusApi.review).not.toHaveBeenCalled();
  expect(projectRunsApi.list).not.toHaveBeenCalled();
  rerender(<PortfolioFocus goal={dependent} projects={all} focus={{ revision: 0 }} disabled={false} />);
  await user.click(screen.getByRole('button', { name: 'Load recorded runs for Payments' }));
  const select = await screen.findByRole('combobox', { name: 'Run to wait for in Payments' });
  expect(select).toHaveProperty('value', '');
  expect(screen.getByRole('button', { name: 'Review focus policy' }).matches(':disabled')).toBe(true);
  expect(screen.getByRole('option', { name: /Payment service contract.*recorded complete/ })).not.toBeNull();
  await user.selectOptions(select, recorded().id);
  await user.click(screen.getByRole('button', { name: 'Review focus policy' }));
  await waitFor(() => expect(portfolioFocusApi.review).toHaveBeenCalledWith(dependent, [{ projectId: 'p-two', runId: `run-${'a'.repeat(24)}` }]));
});

it('aborts a changed-goal list, ignores its late result and clears selection when the user refreshes runs', async () => {
  const dependent = { ...goal, projects: [{ ...goal.projects[0], dependsOn: ['p-two'] }, { projectId: 'p-two', priority: 'normal' as const, requestedSlots: 0, dependsOn: [] }] };
  const all = [...projects, { id: 'p-two', name: 'Payments', path: '/payments' }];
  let finish!: (value: ProjectRun[]) => void; let signal: AbortSignal | undefined;
  vi.mocked(projectRunsApi.list).mockImplementationOnce((_id, abort) => { signal = abort; return new Promise(resolve => { finish = resolve; }); });
  const page = render(<PortfolioFocus goal={dependent} projects={all} focus={{ revision: 0 }} disabled={false} />);
  const user = userEvent.setup(); await user.click(screen.getByRole('button', { name: 'Load recorded runs for Payments' }));
  page.rerender(<PortfolioFocus goal={{ ...dependent, revision: 3 }} projects={all} focus={{ revision: 0 }} disabled={false} />);
  expect(signal?.aborted).toBe(true);
  await act(async () => finish([recorded()]));
  expect(screen.queryByRole('combobox', { name: 'Run to wait for in Payments' })).toBeNull();
  expect(projectRunsApi.list).toHaveBeenCalledTimes(1);
  await user.click(screen.getByRole('button', { name: 'Load recorded runs for Payments' }));
  const select = await screen.findByRole('combobox', { name: 'Run to wait for in Payments' });
  await user.selectOptions(select, recorded().id);
  expect(screen.getByRole('button', { name: 'Review focus policy' }).matches(':disabled')).toBe(false);
  await user.click(screen.getByRole('button', { name: 'Load recorded runs for Payments' }));
  await waitFor(() => expect(screen.getByRole('combobox', { name: 'Run to wait for in Payments' })).toHaveProperty('value', ''));
  expect(screen.getByRole('button', { name: 'Review focus policy' }).matches(':disabled')).toBe(true);
});

it('keeps empty, failed and foreign lists out of dependency review', async () => {
  const dependent = { ...goal, projects: [{ ...goal.projects[0], dependsOn: ['p-two'] }, { projectId: 'p-two', priority: 'normal' as const, requestedSlots: 0, dependsOn: [] }] };
  const all = [...projects, { id: 'p-two', name: 'Payments', path: '/payments' }];
  vi.mocked(projectRunsApi.list).mockResolvedValueOnce([]).mockRejectedValueOnce(new Error('Reading access changed')).mockResolvedValueOnce([recorded('other-project')]);
  render(<PortfolioFocus goal={dependent} projects={all} focus={{ revision: 0 }} disabled={false} />);
  const user = userEvent.setup(); await user.click(screen.getByRole('button', { name: 'Load recorded runs for Payments' }));
  await screen.findByText('No recorded runs are available for Payments.');
  await user.click(screen.getByRole('button', { name: 'Load recorded runs for Payments' }));
  await screen.findByText('Reading access changed');
  await user.click(screen.getByRole('button', { name: 'Retry recorded runs for Payments' }));
  await screen.findByText('The recorded runs could not be confirmed for this project.');
  expect(screen.queryByRole('combobox', { name: 'Run to wait for in Payments' })).toBeNull();
  expect(portfolioFocusApi.review).not.toHaveBeenCalled();
});
