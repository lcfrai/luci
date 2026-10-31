import { productApi } from './product';
import type { PortfolioGoal } from './portfolioGoals';

export interface FocusDependency { projectId: string; runId: string; inputDigest: string }
export interface FocusAllocation { projectId: string; maxActive: number; waitUntilAccepted: FocusDependency[] }
export interface FocusProfile { goalId: string; goalRevision: number; goalDigest: string; allocations: FocusAllocation[]; digest: string }
export interface FocusView { revision: number; profile?: FocusProfile }
export interface FocusPlan { profile: FocusProfile; expectedFocusRevision: number; digest: string; executionStarted: false }
export interface FocusRunSelection { projectId: string; runId: string }
const id = (value: unknown): value is string => typeof value === 'string' && /^[A-Za-z0-9_-]{1,128}$/.test(value);
const hash = (value: unknown): value is string => typeof value === 'string' && /^[a-f0-9]{64}$/.test(value);
const integer = (value: unknown): value is number => Number.isSafeInteger(value) && (value as number) >= 0;
const runId = (value: unknown): value is string => typeof value === 'string' && /^run-[A-Za-z0-9_-]{24}$/.test(value);
export function validFocusProfile(value: FocusProfile): boolean {
  if (!value || !id(value.goalId) || !integer(value.goalRevision) || value.goalRevision < 1 || !hash(value.goalDigest) || !hash(value.digest) ||
      !Array.isArray(value.allocations) || value.allocations.length < 1 || value.allocations.length > 16) return false;
  const projects = new Set(value.allocations.map(row => row?.projectId));
  if (projects.size !== value.allocations.length) return false;
  return value.allocations.every(row => !!row && id(row.projectId) && integer(row.maxActive) && row.maxActive <= 200 &&
    Array.isArray(row.waitUntilAccepted) && row.waitUntilAccepted.length <= 15 &&
    new Set(row.waitUntilAccepted.map(dependency => dependency?.projectId)).size === row.waitUntilAccepted.length &&
    row.waitUntilAccepted.every(dependency => !!dependency && projects.has(dependency.projectId) && dependency.projectId !== row.projectId && runId(dependency.runId) && hash(dependency.inputDigest)));
}
export function validFocusView(value: FocusView): boolean {
  return !!value && integer(value.revision) && (value.profile === undefined || value.revision > 0 && validFocusProfile(value.profile));
}
export function validFocusPlan(value: FocusPlan): boolean {
  return !!value && validFocusProfile(value.profile) && integer(value.expectedFocusRevision) && hash(value.digest) && value.executionStarted === false;
}
function checked<T>(value: T, valid: (value: T) => boolean): T {
  if (!valid(value)) throw new Error('The focus policy could not be verified. Read latest saved goals before continuing.');
  return value;
}
const path = '/v1/product/portfolio-goals';
const profileBinding = (value: FocusProfile | undefined) => value === undefined ? '' : JSON.stringify([
  value.goalId, value.goalRevision, value.goalDigest, value.digest,
  value.allocations.map(row => [row.projectId, row.maxActive, row.waitUntilAccepted.map(dependency => [dependency.projectId, dependency.runId, dependency.inputDigest])]),
]);
export const portfolioFocusApi = {
  async review(goal: PortfolioGoal, upstreamRuns: FocusRunSelection[]): Promise<FocusPlan> {
    const result = checked(await productApi.request<FocusPlan>(`${path}/${encodeURIComponent(goal.id)}/review`, {
      mode: 'focus', expectedGoalRevision: goal.revision, focus: { upstreamRuns },
    }, 'POST'), validFocusPlan);
    if (result.profile.goalId !== goal.id || result.profile.goalRevision !== goal.revision || result.profile.allocations.length !== goal.projects.length ||
        result.profile.allocations.some(row => {
          const selected = goal.projects.find(project => project.projectId === row.projectId);
          return !selected || row.maxActive !== selected.requestedSlots || row.waitUntilAccepted.length !== selected.dependsOn.length ||
            row.waitUntilAccepted.some(dependency => !selected.dependsOn.includes(dependency.projectId) || !upstreamRuns.some(run => run.projectId === dependency.projectId && run.runId === dependency.runId));
        })) throw new Error('The returned focus policy does not match your selected goal and exact upstream runs.');
    return result;
  },
  async install(plan: FocusPlan, idempotencyKey: string): Promise<FocusView> {
    const runs = new Map<string, string>();
    for (const allocation of plan.profile.allocations) for (const dependency of allocation.waitUntilAccepted) runs.set(dependency.projectId, dependency.runId);
    const response = await productApi.request<{ focus: FocusView }>(`${path}/${encodeURIComponent(plan.profile.goalId)}/apply`, {
      mode: 'focus-install', goalId: plan.profile.goalId, expectedGoalRevision: plan.profile.goalRevision, expectedPlanDigest: plan.digest, idempotencyKey,
      focus: { expectedRevision: plan.expectedFocusRevision, upstreamRuns: [...runs].map(([projectId, runId]) => ({ projectId, runId })) },
    }, 'POST');
    const result = checked(response?.focus, validFocusView);
    if (result.revision !== plan.expectedFocusRevision + 1 || profileBinding(result.profile) !== profileBinding(plan.profile)) throw new Error('Focus installation was not confirmed exactly. Read latest saved goals; do not replay this request.');
    return result;
  },
  async remove(view: FocusView, idempotencyKey: string): Promise<FocusView> {
    if (!view.profile) throw new Error('Read the active focus policy before removing it.');
    const response = await productApi.request<{ focus: FocusView }>(`${path}/${encodeURIComponent(view.profile.goalId)}/apply`, {
      mode: 'focus-remove', goalId: view.profile.goalId, expectedGoalRevision: view.profile.goalRevision, idempotencyKey,
      focus: { expectedRevision: view.revision, expectedProfileDigest: view.profile.digest },
    }, 'POST');
    const result = checked(response?.focus, validFocusView);
    if (result.revision !== view.revision + 1 || result.profile !== undefined) throw new Error('Focus removal was not confirmed. Read latest saved goals; do not replay it.');
    return result;
  },
};
