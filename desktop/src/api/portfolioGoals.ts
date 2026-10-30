import { productApi } from './product';
import type { PortfolioIntervention, PortfolioProject } from './portfolio';

export type GoalPriority = 'low' | 'normal' | 'high';
export interface GoalProject { projectId: string; priority: GoalPriority; requestedSlots: number; dependsOn: string[] }
export interface PortfolioGoal { id: string; revision: number; title: string; outcome: string; deadline?: string; projects: GoalProject[]; updatedAt: string }
export interface PriorityAction { projectId: string; queueId: string; runId: string; taskId: string; taskTitle: string; expectedRevision: number; fromPriority: GoalPriority; toPriority: GoalPriority }
export interface GoalPlan {
  goalId: string; goalRevision: number; goalDigest: string; digest: string; actions: PriorityAction[];
  allocations: (GoalProject & { recordedActive: number; waiting: number; explanation: string })[];
  decisions: string[]; executionStarted: false; capacityPolicyChanged: false;
}
export interface GoalApplication {
  id: string; goalId: string; goalRevision: number; projectIds: string[]; planDigest: string; keyDigest: string; requestDigest: string;
  status: 'no_changes' | 'applied' | 'partial' | 'not_applied' | 'unknown'; createdAt: string;
  results: { action: PriorityAction; status: 'not_attempted' | 'pending' | 'applied' | 'blocked' | 'unknown'; message: string; appliedRevision?: number; intentDigest: string }[];
}
export interface PortfolioOutcomeRoute {
  id: string; goalId: string; goalRevision: number; projectId: string;
  status: 'unknown' | 'attention' | 'recorded' | 'not-configured'; reason: string;
  coordinator?: PortfolioProject['coordinator']; evidence: PortfolioIntervention['evidence']; nextAction: PortfolioIntervention['nextAction'];
}
export interface PortfolioOutcomeRouting {
  version: 1; status: 'ready' | 'partial'; observedAt: string; routes: PortfolioOutcomeRoute[];
  readProjects: number; omittedProjects: number; omittedRoutes: number; unknownProjects: number;
}
export interface GoalsView { revision: number; goals: PortfolioGoal[]; applications: GoalApplication[]; partial: boolean; outcomeRouting?: PortfolioOutcomeRouting }
const id = (value: unknown): value is string => typeof value === 'string' && /^[A-Za-z0-9_-]{1,128}$/.test(value);
const digest = (value: unknown): value is string => typeof value === 'string' && /^[a-f0-9]{64}$/.test(value);
const integer = (value: unknown, minimum = 0): value is number => Number.isSafeInteger(value) && (value as number) >= minimum;
const text = (value: unknown, maximum: number): value is string => typeof value === 'string' && value.length <= maximum && !/[\u0000-\u0008\u000b\u000c\u000e-\u001f\u007f]/.test(value);
const priority = (value: unknown): value is GoalPriority => value === 'low' || value === 'normal' || value === 'high';
const date = (value: unknown) => typeof value === 'string' && value.length <= 100 && Number.isFinite(Date.parse(value));
function validProject(value: GoalProject): boolean {
  return !!value && id(value.projectId) && priority(value.priority) && integer(value.requestedSlots) && value.requestedSlots <= 200 &&
    Array.isArray(value.dependsOn) && value.dependsOn.length <= 15 && value.dependsOn.every(id) && new Set(value.dependsOn).size === value.dependsOn.length;
}
export function validGoal(value: PortfolioGoal): boolean {
  return !!value && id(value.id) && integer(value.revision, 1) && text(value.title, 200) && !!value.title.trim() && text(value.outcome, 4000) && !!value.outcome.trim() && date(value.updatedAt) &&
    (value.deadline === undefined || /^\d{4}-\d{2}-\d{2}$/.test(value.deadline)) && Array.isArray(value.projects) && value.projects.length >= 1 && value.projects.length <= 16 &&
    value.projects.every(validProject) && new Set(value.projects.map(p => p.projectId)).size === value.projects.length &&
    value.projects.every(p => p.dependsOn.every(other => other !== p.projectId && value.projects.some(row => row.projectId === other)));
}
function validAction(value: PriorityAction): boolean {
  return !!value && id(value.projectId) && id(value.queueId) && id(value.runId) && id(value.taskId) && integer(value.expectedRevision, 1) &&
    priority(value.fromPriority) && priority(value.toPriority) && text(value.taskTitle, 2000);
}
const actionBinding = (value: PriorityAction) => JSON.stringify([value.projectId, value.queueId, value.runId, value.taskId, value.taskTitle, value.expectedRevision, value.fromPriority, value.toPriority]);
export function validGoalPlan(value: GoalPlan): boolean {
  if (!value || !id(value.goalId) || !integer(value.goalRevision, 1) || !digest(value.goalDigest) || !digest(value.digest) || value.executionStarted !== false || value.capacityPolicyChanged !== false ||
      !Array.isArray(value.actions) || value.actions.length > 64 || !value.actions.every(validAction) || !Array.isArray(value.allocations) || value.allocations.length < 1 || value.allocations.length > 16 ||
      !value.allocations.every(row => validProject(row) && integer(row.recordedActive) && integer(row.waiting) && text(row.explanation, 2000)) ||
      !Array.isArray(value.decisions) || value.decisions.length > 16 || !value.decisions.every(row => text(row, 2000))) return false;
  const projects = new Set(value.allocations.map(row => row.projectId));
  return projects.size === value.allocations.length && value.actions.every(row => projects.has(row.projectId)) &&
    new Set(value.actions.map(row => `${row.projectId}:${row.queueId}`)).size === value.actions.length;
}
export function validGoalApplication(value: GoalApplication): boolean {
  const valid = !!value && id(value.id) && id(value.goalId) && integer(value.goalRevision, 1) && digest(value.planDigest) && digest(value.keyDigest) && digest(value.requestDigest) && date(value.createdAt) &&
    Array.isArray(value.projectIds) && value.projectIds.length > 0 && value.projectIds.length <= 16 && value.projectIds.every(id) && new Set(value.projectIds).size === value.projectIds.length &&
    ['no_changes', 'applied', 'partial', 'not_applied', 'unknown'].includes(value.status) && Array.isArray(value.results) && value.results.length <= 64 &&
    value.results.every(row => !!row && validAction(row.action) && value.projectIds.includes(row.action.projectId) && digest(row.intentDigest) && text(row.message, 300) &&
      ['not_attempted', 'pending', 'applied', 'blocked', 'unknown'].includes(row.status) &&
      (row.status === 'applied' ? row.appliedRevision === row.action.expectedRevision + 1 : row.appliedRevision === undefined || row.appliedRevision === 0));
  if (!valid) return false;
  const applied = value.results.filter(row => row.status === 'applied').length;
  const expected = value.results.length === 0 ? 'no_changes' : value.results.some(row => row.status === 'pending' || row.status === 'unknown') ? 'unknown'
    : applied === value.results.length ? 'applied' : applied > 0 ? 'partial' : 'not_applied';
  return value.status === expected && new Set(value.results.map(row => `${row.action.projectId}:${row.action.queueId}`)).size === value.results.length;
}
export function validGoalsView(value: GoalsView): boolean {
  return !!value && integer(value.revision) && typeof value.partial === 'boolean' && Array.isArray(value.goals) && value.goals.length <= 16 && value.goals.every(validGoal) &&
    new Set(value.goals.map(goal => goal.id)).size === value.goals.length && Array.isArray(value.applications) && value.applications.length <= 32 && value.applications.every(validGoalApplication) &&
    new Set(value.applications.map(application => application.id)).size === value.applications.length && value.applications.every(application => value.goals.some(goal => goal.id === application.goalId)) &&
    (value.outcomeRouting === undefined || validOutcomeRouting(value.outcomeRouting, value.goals, value.partial));
}
export function validOutcomeRouting(value: PortfolioOutcomeRouting, goals: PortfolioGoal[], partial: boolean): boolean {
  const pairs = goals.reduce((count, goal) => count + goal.projects.length, 0);
  const projects = new Set(goals.flatMap(goal => goal.projects.map(row => row.projectId)));
  if (!value || value.version !== 1 || !['ready', 'partial'].includes(value.status) || !date(value.observedAt) ||
      !integer(value.readProjects) || value.readProjects > 16 || !integer(value.omittedProjects) || projects.size !== value.readProjects + value.omittedProjects ||
      !integer(value.omittedRoutes) || !integer(value.unknownProjects) || value.unknownProjects > value.readProjects ||
      !Array.isArray(value.routes) || value.routes.length > 32 || pairs !== value.routes.length + value.omittedRoutes ||
      value.status === 'ready' && (partial || value.omittedRoutes > 0 || value.omittedProjects > 0 || value.unknownProjects > 0)) return false;
  const seen = new Set<string>(), ids = new Set<string>(), unknown = new Set<string>();
  for (const route of value.routes) {
    if (!route || !digest(route.id) || ids.has(route.id) || !id(route.goalId) || !id(route.projectId) || !integer(route.goalRevision, 1) ||
        !goals.some(goal => goal.id === route.goalId && goal.revision === route.goalRevision && goal.projects.some(p => p.projectId === route.projectId)) ||
        !['unknown', 'attention', 'recorded', 'not-configured'].includes(route.status) || !text(route.reason, 1200) || !route.reason.trim() ||
        !Array.isArray(route.evidence) || route.evidence.length < 1 || route.evidence.length > 4 || route.evidence.some(row => !row || !id(row.id) ||
          !['overview', 'run', 'questions', 'run-history', 'queue-item', 'queue', 'coordinator', 'finding', 'project'].includes(row.source) ||
          row.revision !== undefined && !integer(row.revision) || row.observedAt !== undefined && !date(row.observedAt))) return false;
    const action = route.nextAction;
    if (!action || !text(action.label, 200) || !action.label.trim() || action.runId !== undefined && !id(action.runId) ||
        !(action.id === 'inspect-run' ? action.lens === 'develop' : ['answer-question', 'review-context', 'open-conversation'].includes(action.id) && action.lens === 'discover' && action.runId === undefined)) return false;
    const c = route.coordinator;
    if (c !== undefined && (!c || c.status !== 'recorded' || !id(c.id) || !integer(c.revision, 1) || !text(c.state, 100) || !c.state?.trim() || !date(c.updatedAt) || !integer(c.unresolved) || c.runId !== undefined && !id(c.runId))) return false;
    if (route.status === 'recorded' && c === undefined || route.status === 'not-configured' && c !== undefined) return false;
    const pair = `${route.goalId}:${route.projectId}`;
    if (seen.has(pair)) return false;
    seen.add(pair); ids.add(route.id);
    if (route.status === 'unknown') unknown.add(route.projectId);
  }
  return new Set(value.routes.map(route => route.projectId)).size <= value.readProjects && unknown.size <= value.unknownProjects;
}
function checked<T>(value: T, valid: (value: T) => boolean): T {
  if (!valid(value)) throw new Error('The saved goal or queue result could not be verified. Read the current portfolio before continuing.');
  return value;
}
const path = '/v1/product/portfolio-goals';
export const portfolioGoalsApi = {
  async get(signal?: AbortSignal): Promise<GoalsView> { return checked(await productApi.request<GoalsView>(path, undefined, 'GET', 30_000, signal), validGoalsView); },
  async save(expectedRevision: number, goal: PortfolioGoal): Promise<PortfolioGoal> {
    return checked(await productApi.request<PortfolioGoal>(path, { expectedRevision, goal }, 'POST'), validGoal);
  },
  async review(goal: PortfolioGoal): Promise<GoalPlan> {
    const plan = checked(await productApi.request<GoalPlan>(`${path}/${encodeURIComponent(goal.id)}/review`, { expectedGoalRevision: goal.revision }, 'POST'), validGoalPlan);
    if (plan.goalId !== goal.id || plan.goalRevision !== goal.revision) throw new Error('The returned plan belongs to another goal revision.');
    return plan;
  },
  async apply(plan: GoalPlan, idempotencyKey: string): Promise<GoalApplication> {
    const result = checked(await productApi.request<GoalApplication>(`${path}/${encodeURIComponent(plan.goalId)}/apply`, { goalId: plan.goalId, expectedGoalRevision: plan.goalRevision, expectedPlanDigest: plan.digest, idempotencyKey }, 'POST', 30_000), validGoalApplication);
    if (result.goalId !== plan.goalId || result.goalRevision !== plan.goalRevision || result.planDigest !== plan.digest || result.results.length !== plan.actions.length ||
        result.results.some((row, index) => actionBinding(row.action) !== actionBinding(plan.actions[index])) ||
        [...result.projectIds].sort().join(',') !== plan.allocations.map(row => row.projectId).sort().join(',')) throw new Error('The returned priority result does not match this review. Read the retained applications; do not replay it.');
    return result;
  },
  async application(applicationId: string): Promise<GoalApplication> {
    const result = checked(await productApi.request<GoalApplication>(`${path}/applications/${encodeURIComponent(applicationId)}`, undefined, 'GET'), validGoalApplication);
    if (result.id !== applicationId) throw new Error('The returned application identity changed.');
    return result;
  },
};
