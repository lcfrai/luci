import { afterEach, expect, it, vi } from 'vitest';
import { productApi } from './product';
import { portfolioGoalsApi, validGoalApplication, validGoalPlan, validGoalsView, type GoalApplication, type GoalPlan, type PortfolioGoal } from './portfolioGoals';

const goal = (): PortfolioGoal => ({ id: 'goal-one', revision: 2, title: 'Ship booking', outcome: 'Customers choose an available appointment', updatedAt: '2026-10-30T00:00:00Z', projects: [{ projectId: 'p-one', priority: 'high', requestedSlots: 2, dependsOn: [] }] });
const plan = (): GoalPlan => ({ goalId: 'goal-one', goalRevision: 2, goalDigest: 'a'.repeat(64), digest: 'b'.repeat(64), executionStarted: false, capacityPolicyChanged: false, decisions: [],
  allocations: [{ ...goal().projects[0], recordedActive: 0, waiting: 1, explanation: 'Planning intention only.' }], actions: [{ projectId: 'p-one', queueId: 'queue-one', runId: 'run-one', taskId: 'task-one', taskTitle: 'Booking form', expectedRevision: 3, fromPriority: 'normal', toPriority: 'high' }] });
const result = (): GoalApplication => ({ id: 'portfolio-one', goalId: 'goal-one', goalRevision: 2, projectIds: ['p-one'], planDigest: 'b'.repeat(64), keyDigest: 'd'.repeat(64), requestDigest: 'e'.repeat(64), createdAt: '2026-10-30T00:00:00Z', status: 'applied', results: [{ action: plan().actions[0], status: 'applied', appliedRevision: 4, intentDigest: 'f'.repeat(64), message: 'Exact priority saved; no work started.' }] });
afterEach(() => vi.restoreAllMocks());

it('refuses execution claims, orphan actions and fabricated complete application states', () => {
  expect(validGoalsView({ revision: 1, goals: [goal()], applications: [result()], partial: false })).toBe(true);
	const validTask = plan(); validTask.actions[0].taskId = '_book_slot'; expect(validGoalPlan(validTask)).toBe(true);
  expect(validGoalPlan({ ...plan(), executionStarted: true } as unknown as GoalPlan)).toBe(false);
  const orphan = plan(); orphan.actions[0].projectId = 'p-hidden'; expect(validGoalPlan(orphan)).toBe(false);
  const unknown = result(); unknown.results[0].status = 'unknown'; delete unknown.results[0].appliedRevision;
  expect(validGoalApplication(unknown)).toBe(false);
  unknown.status = 'unknown'; expect(validGoalApplication(unknown)).toBe(true);
});

it('applies the exact reviewed goal digest once and rejects a returned different queue binding', async () => {
  const response = result(); response.results[0].action.expectedRevision++; response.results[0].appliedRevision = 5;
  const request = vi.spyOn(productApi, 'request').mockResolvedValue(response);
  await expect(portfolioGoalsApi.apply(plan(), 'explicit-key')).rejects.toThrow(/does not match/);
  expect(request).toHaveBeenCalledTimes(1);
  expect(request).toHaveBeenCalledWith('/v1/product/portfolio-goals/goal-one/apply', { goalId: 'goal-one', expectedGoalRevision: 2, expectedPlanDigest: 'b'.repeat(64), idempotencyKey: 'explicit-key' }, 'POST', 30_000);
});

it('reads unknown retained outcomes without issuing another application', async () => {
  const unknown = result(); unknown.status = 'unknown'; unknown.results[0].status = 'unknown'; delete unknown.results[0].appliedRevision;
  const request = vi.spyOn(productApi, 'request').mockResolvedValue(unknown);
  expect(await portfolioGoalsApi.application('portfolio-one')).toEqual(unknown);
  expect(request).toHaveBeenCalledTimes(1);
  expect(request).toHaveBeenCalledWith('/v1/product/portfolio-goals/applications/portfolio-one', undefined, 'GET');
});
