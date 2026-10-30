import { afterEach, expect, it, vi } from 'vitest';
import { portfolioGoalsApi, validGoalsView, type GoalsView } from './portfolioGoals';
import { productApi } from './product';

function view(): GoalsView {
  return { revision: 2, partial: false, applications: [], goals: [{ id: 'goal-one', revision: 3, title: 'Book appointments', outcome: 'Customers can select an available slot', updatedAt: '2026-10-30T00:00:00Z', projects: [{ projectId: 'p-one', priority: 'high', requestedSlots: 2, dependsOn: [] }] }],
    outcomeRouting: { version: 1, status: 'ready', observedAt: '2026-10-30T00:00:00Z', readProjects: 1, omittedProjects: 0, omittedRoutes: 0, unknownProjects: 0,
      routes: [{ id: 'a'.repeat(64), goalId: 'goal-one', goalRevision: 3, projectId: 'p-one', status: 'recorded', reason: 'Inspect retained work.', coordinator: { status: 'recorded', id: 'continue-alice', revision: 7, state: 'working', runId: 'run-one', updatedAt: '2026-10-30T00:00:00Z', unresolved: 0 }, evidence: [{ source: 'coordinator', id: 'continue-alice', revision: 7 }], nextAction: { id: 'inspect-run', label: 'Inspect recorded work', lens: 'develop', runId: 'run-one' } }] } };
}
afterEach(() => vi.restoreAllMocks());

it('reads outcome links through the existing private goals GET and accepts legacy missing projection', async () => {
  const current = view(), request = vi.spyOn(productApi, 'request').mockResolvedValue(current);
  expect(await portfolioGoalsApi.get()).toEqual(current);
  expect(request).toHaveBeenCalledTimes(1);
  expect(request).toHaveBeenCalledWith('/v1/product/portfolio-goals', undefined, 'GET', 30_000, undefined);
  delete current.outcomeRouting;
  expect(validGoalsView(current)).toBe(true);
});

it('refuses orphan projects and old goal revision routes instead of borrowing another goal', () => {
  const orphan = view(); orphan.outcomeRouting!.routes[0].projectId = 'p-hidden'; expect(validGoalsView(orphan)).toBe(false);
  const stale = view(); stale.outcomeRouting!.routes[0].goalRevision = 2; expect(validGoalsView(stale)).toBe(false);
  const duplicate = view(); duplicate.outcomeRouting!.routes.push(duplicate.outcomeRouting!.routes[0]); expect(validGoalsView(duplicate)).toBe(false);
});

it('keeps incomplete observations partial and verifies read and route omission totals', () => {
  const unknown = view(); unknown.outcomeRouting!.routes[0].status = 'unknown';
  expect(validGoalsView(unknown)).toBe(false);
  unknown.outcomeRouting!.unknownProjects = 1; unknown.outcomeRouting!.status = 'partial'; expect(validGoalsView(unknown)).toBe(true);
  unknown.outcomeRouting!.omittedProjects = 1; expect(validGoalsView(unknown)).toBe(false);
});

it('allows only existing navigation actions and rejects embedded apply or worker-start actions', () => {
  const invalid = view(); invalid.outcomeRouting!.routes[0].nextAction.id = 'apply-priorities';
  expect(validGoalsView(invalid)).toBe(false);
  invalid.outcomeRouting!.routes[0].nextAction = { id: 'open-conversation', label: 'Open project Luci', lens: 'discover', runId: 'run-one' };
  expect(validGoalsView(invalid)).toBe(false);
});
