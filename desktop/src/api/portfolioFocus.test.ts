import { afterEach, expect, it, vi } from 'vitest';
import { productApi } from './product';
import { portfolioFocusApi, validFocusPlan, type FocusPlan } from './portfolioFocus';
import type { PortfolioGoal } from './portfolioGoals';

const goal = (): PortfolioGoal => ({ id: 'goal-one', revision: 2, title: 'Useful outcome', outcome: 'Customers can book', updatedAt: '2026-10-31T00:00:00Z', projects: [{ projectId: 'p-one', priority: 'high', requestedSlots: 1, dependsOn: [] }] });
const plan = (): FocusPlan => ({ expectedFocusRevision: 3, digest: 'a'.repeat(64), executionStarted: false,
  profile: { goalId: 'goal-one', goalRevision: 2, goalDigest: 'b'.repeat(64), digest: 'c'.repeat(64), allocations: [{ projectId: 'p-one', maxActive: 1, waitUntilAccepted: [] }] } });
afterEach(() => vi.restoreAllMocks());

it('reviews exact saved slot ceilings and rejects a different returned scope', async () => {
  const response = plan(); response.profile.allocations[0].projectId = 'p-other';
  const request = vi.spyOn(productApi, 'request').mockResolvedValue(response);
  await expect(portfolioFocusApi.review(goal(), [])).rejects.toThrow(/does not match/);
  expect(request).toHaveBeenCalledWith('/v1/product/portfolio-goals/goal-one/review', { mode: 'focus', expectedGoalRevision: 2, focus: { upstreamRuns: [] } }, 'POST');
  const invalid = plan(); invalid.profile.allocations[0].waitUntilAccepted = [{ projectId: 'p-hidden', runId: `run-${'a'.repeat(24)}`, inputDigest: 'd'.repeat(64) }];
  expect(validFocusPlan(invalid)).toBe(false);
});

it('installs only the exact reviewed profile and refuses an unconfirmed result without retry', async () => {
  const selected = plan(), changed = plan().profile; changed.allocations[0].maxActive = 2;
  const request = vi.spyOn(productApi, 'request').mockResolvedValue({ focus: { revision: 4, profile: changed } });
  await expect(portfolioFocusApi.install(selected, 'single-explicit-focus')).rejects.toThrow(/not confirmed/);
  expect(request).toHaveBeenCalledTimes(1);
  expect(request).toHaveBeenCalledWith('/v1/product/portfolio-goals/goal-one/apply', {
    mode: 'focus-install', goalId: 'goal-one', expectedGoalRevision: 2, expectedPlanDigest: selected.digest, idempotencyKey: 'single-explicit-focus', focus: { expectedRevision: 3, upstreamRuns: [] },
  }, 'POST');
});

it('removes using exact active revision and profile digest rather than changing priority settings', async () => {
  const request = vi.spyOn(productApi, 'request').mockResolvedValue({ focus: { revision: 5 } });
  await expect(portfolioFocusApi.remove({ revision: 4, profile: plan().profile }, 'explicit-remove-focus')).resolves.toEqual({ revision: 5 });
  expect(request).toHaveBeenCalledWith('/v1/product/portfolio-goals/goal-one/apply', {
    mode: 'focus-remove', goalId: 'goal-one', expectedGoalRevision: 2, idempotencyKey: 'explicit-remove-focus', focus: { expectedRevision: 4, expectedProfileDigest: 'c'.repeat(64) },
  }, 'POST');
});
