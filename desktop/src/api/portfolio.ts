import { productApi } from './product';

export interface PortfolioProject {
  id: string; name: string; status: 'ready' | 'partial';
  coordinator: { status: 'recorded' | 'none' | 'unavailable'; id?: string; revision?: number; state?: string; runId?: string; updatedAt?: string; unresolved: number };
}
export interface PortfolioIntervention {
  id: string; projectId: string;
  kind: 'unconfirmed' | 'question' | 'context' | 'review' | 'blocked' | 'paused' | 'capacity';
  title: string; reason: string; blocks: string;
  nextAction: { id: string; label: string; lens: 'discover' | 'design' | 'develop'; runId?: string };
  evidence: { source: string; id: string; revision?: number; observedAt?: string }[];
}
export interface Portfolio {
  version: 1; observedAt: string; status: 'ready' | 'partial';
  totalProjects: number; omittedProjects: number; omittedInterventions: number;
  projects: PortfolioProject[]; interventions: PortfolioIntervention[];
}
const count = (value: unknown): value is number => Number.isSafeInteger(value) && (value as number) >= 0;
const text = (value: unknown, max: number): value is string => typeof value === 'string' && value.length > 0 && value.length <= max;

// Refuse malformed/older responses rather than rendering "nothing needs you".
export function validPortfolio(value: Portfolio): boolean {
  if (!value || value.version !== 1 || !['ready', 'partial'].includes(value.status) || !text(value.observedAt, 100) || !Number.isFinite(Date.parse(value.observedAt)) ||
      !count(value.totalProjects) || !count(value.omittedProjects) || !count(value.omittedInterventions) ||
      !Array.isArray(value.projects) || value.projects.length > 64 || !Array.isArray(value.interventions) || value.interventions.length > 128 ||
      value.totalProjects !== value.projects.length + value.omittedProjects ||
      (value.status === 'ready' && (value.omittedProjects > 0 || value.omittedInterventions > 0))) return false;
  const ids = new Set<string>();
  for (const project of value.projects) {
    if (!project || !text(project.id, 200) || ids.has(project.id) || !text(project.name, 200) || !['ready', 'partial'].includes(project.status)) return false;
    ids.add(project.id);
    const c = project.coordinator;
    if (!c || !['recorded', 'none', 'unavailable'].includes(c.status) || !count(c.unresolved)) return false;
    if (c.status === 'recorded' && (!text(c.id, 200) || !count(c.revision) || c.revision < 1 || !text(c.state, 100) || !text(c.updatedAt, 100) || !Number.isFinite(Date.parse(c.updatedAt)))) return false;
    if (c.status !== 'recorded' && (c.id !== undefined || c.state !== undefined || c.revision !== undefined || c.runId !== undefined || c.updatedAt !== undefined || c.unresolved !== 0)) return false;
    if (value.status === 'ready' && project.status !== 'ready') return false;
  }
  const seen = new Set<string>();
  return value.interventions.every(item => {
    if (!item || !/^[a-f0-9]{64}$/.test(item.id) || seen.has(item.id) || !ids.has(item.projectId) ||
        !['unconfirmed', 'question', 'context', 'review', 'blocked', 'paused', 'capacity'].includes(item.kind) ||
        !text(item.title, 200) || !text(item.reason, 1200) || !text(item.blocks, 1200) ||
        !item.nextAction || !text(item.nextAction.id, 200) || !text(item.nextAction.label, 200) ||
        !['discover', 'design', 'develop'].includes(item.nextAction.lens) ||
        (item.nextAction.runId !== undefined && !text(item.nextAction.runId, 200)) ||
        !Array.isArray(item.evidence) || item.evidence.length < 1 || item.evidence.length > 4 ||
        item.evidence.some(e => !e || !text(e.source, 100) || !text(e.id, 200) ||
          (e.revision !== undefined && (!count(e.revision) || e.revision < 1)) ||
          (e.observedAt !== undefined && (!text(e.observedAt, 100) || !Number.isFinite(Date.parse(e.observedAt)))))) return false;
    seen.add(item.id); return true;
  });
}

export const portfolioApi = {
  async get(signal?: AbortSignal): Promise<Portfolio> {
    const value = await productApi.request<Portfolio>('/v1/product/portfolio', undefined, 'GET', 30_000, signal);
    if (!validPortfolio(value)) throw new Error('Project attention could not be verified. Refresh before continuing.');
    return value;
  },
};
