import { act, cleanup, render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, beforeEach, expect, it, vi } from 'vitest';
import { PortfolioGoals } from './PortfolioGoals';
import { portfolioGoalsApi, type GoalApplication, type GoalPlan, type GoalsView, type PortfolioGoal } from '../../api/portfolioGoals';
import { resetCockpit, useCockpit } from '../../store/store';

const environment = vi.hoisted(() => ({ scope: 'alice', canEdit: true }));
vi.mock('../../hooks/useProjectDraft', () => ({ useProjectDraftScope: () => environment.scope }));
vi.mock('../../hooks/useProjectAccess', () => ({ useProjectAccess: () => ({ canRead: true, canEdit: environment.canEdit }) }));
vi.mock('../teams/TeamWorkspaceProvider', () => ({ useTeamWorkspace: () => null }));
const projects = [{ id: 'p-one', name: 'Appointments', path: '/appointments' }];
const goal = (): PortfolioGoal => ({ id: 'goal-one', revision: 1, title: 'Ship appointments', outcome: 'Customers book without re-entry', updatedAt: '2026-10-30T00:00:00Z', projects: [{ projectId: 'p-one', priority: 'high', requestedSlots: 2, dependsOn: [] }] });
const view = (): GoalsView => ({ revision: 1, goals: [goal()], applications: [], partial: false });
const plan = (): GoalPlan => ({ goalId: 'goal-one', goalRevision: 1, goalDigest: 'a'.repeat(64), digest: 'b'.repeat(64), executionStarted: false, capacityPolicyChanged: false, decisions: [], allocations: [{ ...goal().projects[0], recordedActive: 0, waiting: 1, explanation: 'Intention only.' }],
  actions: [{ projectId: 'p-one', queueId: 'queue-one', runId: 'run-one', taskId: 'task-one', taskTitle: 'Booking form', expectedRevision: 2, fromPriority: 'normal', toPriority: 'high' }] });
const application = (): GoalApplication => ({ id: 'portfolio-one', goalId: 'goal-one', goalRevision: 1, projectIds: ['p-one'], planDigest: 'b'.repeat(64), keyDigest: 'd'.repeat(64), requestDigest: 'e'.repeat(64), status: 'unknown', createdAt: '2026-10-30T00:00:00Z', results: [{ action: plan().actions[0], status: 'unknown', intentDigest: 'f'.repeat(64), message: 'Result not confirmed. Read this application before another review.' }] });
beforeEach(() => {
  vi.restoreAllMocks(); resetCockpit(); environment.scope = 'alice'; environment.canEdit = true;
  vi.spyOn(portfolioGoalsApi, 'get').mockResolvedValue(view());
  vi.spyOn(portfolioGoalsApi, 'save').mockRejectedValue(new Error('Saved goal changed. Your draft is retained.'));
  vi.spyOn(portfolioGoalsApi, 'review').mockResolvedValue(plan());
  vi.spyOn(portfolioGoalsApi, 'apply').mockResolvedValue(application());
  vi.spyOn(portfolioGoalsApi, 'application').mockResolvedValue(application());
});
afterEach(cleanup);
async function open() {
  const user = userEvent.setup();
  await user.click(screen.getByText('Goals across projects'));
  await screen.findByLabelText('Saved goal');
  return user;
}

it('requires a saved review and explicit Apply, then refreshes unknown evidence without replay', async () => {
  render(<PortfolioGoals projects={projects} />); const user = await open();
  await user.selectOptions(screen.getByLabelText('Saved goal'), 'goal-one');
  expect(portfolioGoalsApi.review).not.toHaveBeenCalled(); expect(portfolioGoalsApi.apply).not.toHaveBeenCalled();
  await user.click(screen.getByRole('button', { name: 'Review waiting-work priorities' }));
  const apply = await screen.findByRole('button', { name: 'Apply these priority changes' });
  await waitFor(() => expect(apply.matches(':disabled')).toBe(false));
  expect(screen.getByText(/Booking form: normal/)).not.toBeNull();
  await user.click(apply);
  await screen.findByRole('heading', { name: 'Priority result: unknown' });
  expect(portfolioGoalsApi.apply).toHaveBeenCalledTimes(1);
  await user.click(screen.getByRole('button', { name: 'Refresh this retained application' }));
  await waitFor(() => expect(portfolioGoalsApi.application).toHaveBeenCalledWith('portfolio-one'));
  expect(portfolioGoalsApi.apply).toHaveBeenCalledTimes(1);
});

it('preserves a rejected goal edit and the normal navigation guard without applying anything', async () => {
  render(<PortfolioGoals projects={projects} />); const user = await open();
  await user.selectOptions(screen.getByLabelText('Saved goal'), 'goal-one');
  const title = screen.getByLabelText('Goal title'); await user.clear(title); await user.type(title, 'Keep this amended goal');
  await user.click(screen.getByRole('button', { name: 'Save private goal' }));
  await screen.findByRole('alert');
  expect((title as HTMLInputElement).value).toBe('Keep this amended goal');
  expect(screen.getByRole('button', { name: 'Review waiting-work priorities' }).matches(':disabled')).toBe(true);
  expect(Object.values(useCockpit.getState().unsavedProjectDrafts).some(row => row.label === 'Portfolio goal')).toBe(true);
  const latest = view(); latest.revision = 2; latest.goals[0].revision = 2; latest.goals[0].title = 'Saved elsewhere';
  vi.mocked(portfolioGoalsApi.get).mockResolvedValue(latest);
  await user.click(screen.getByRole('button', { name: 'Read latest saved goals' }));
  await waitFor(() => expect(screen.getByRole('button', { name: 'Read latest saved goals' }).matches(':disabled')).toBe(false));
  expect((screen.getByLabelText('Goal title') as HTMLInputElement).value).toBe('Keep this amended goal');
  expect(portfolioGoalsApi.apply).not.toHaveBeenCalled();
});

it('refreshes a clean selected goal before reviewing a newer saved revision', async () => {
  render(<PortfolioGoals projects={projects} />); const user = await open();
  await user.selectOptions(screen.getByLabelText('Saved goal'), 'goal-one');
  const latest = view(); latest.revision = 2; latest.goals[0].revision = 2; latest.goals[0].title = 'Saved elsewhere';
  vi.mocked(portfolioGoalsApi.get).mockResolvedValue(latest);
  vi.mocked(portfolioGoalsApi.review).mockResolvedValue({ ...plan(), goalRevision: 2 });
  await user.click(screen.getByRole('button', { name: 'Read latest saved goals' }));
  await waitFor(() => expect((screen.getByLabelText('Goal title') as HTMLInputElement).value).toBe('Saved elsewhere'));
  await user.click(screen.getByRole('button', { name: 'Review waiting-work priorities' }));
  await waitFor(() => expect(portfolioGoalsApi.review).toHaveBeenCalledWith(latest.goals[0]));
  expect(portfolioGoalsApi.apply).not.toHaveBeenCalled();
});

it('keeps read-only intentions separate from permission to change project queue priorities', async () => {
  environment.canEdit = false;
  render(<PortfolioGoals projects={projects} />); const user = await open();
  await user.selectOptions(screen.getByLabelText('Saved goal'), 'goal-one');
  await user.click(screen.getByRole('button', { name: 'Review waiting-work priorities' }));
  const apply = await screen.findByRole('button', { name: 'Apply these priority changes' });
  expect(apply.matches(':disabled')).toBe(true);
  expect(screen.getByText(/You can save this intention/)).not.toBeNull();
  expect(portfolioGoalsApi.apply).not.toHaveBeenCalled();
});

it('discards old actor responses before exposing a new private portfolio', async () => {
  let finish!: (value: GoalsView) => void;
  vi.mocked(portfolioGoalsApi.get).mockImplementationOnce(() => new Promise(resolve => { finish = resolve; }));
  const rendered = render(<PortfolioGoals projects={projects} />);
  const user = userEvent.setup(); await user.click(screen.getByText('Goals across projects'));
  environment.scope = 'bob'; vi.mocked(portfolioGoalsApi.get).mockResolvedValue({ revision: 0, goals: [], applications: [], partial: false });
  rendered.rerender(<PortfolioGoals projects={projects} />);
  await screen.findByLabelText('Saved goal');
  await act(async () => { finish(view()); });
  expect(screen.queryByRole('option', { name: /Ship appointments/ })).toBeNull();
});
