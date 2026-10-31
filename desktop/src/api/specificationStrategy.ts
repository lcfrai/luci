export interface StrategyEvidence { id: string; source: string; url: string; observation: string; observedAt: string }
export interface StrategyAlternative { id: string; name: string; difference: string; evidenceIds: string[] }
export interface StrategyAssumption { id: string; claim: string; status: 'untested' | 'supported' | 'contradicted'; experiment: string; evidenceIds: string[]; requirementIds: string[] }
export interface StrategyMetric { id: string; name: string; baseline: string; target: string; window: string; measurement: string; requirementIds: string[] }
export interface SpecificationStrategy {
  customer: string; problem: string; valueProposition: string;
  alternatives: StrategyAlternative[]; assumptions: StrategyAssumption[]; evidence: StrategyEvidence[]; metrics: StrategyMetric[];
}

export const emptySpecificationStrategy = (): SpecificationStrategy => ({ customer: '', problem: '', valueProposition: '', alternatives: [], assumptions: [], evidence: [], metrics: [] });

// Draft recovery admits unfinished text, but never an unknown shape that can
// crash an editor or smuggle additional authority into a retained draft.
export function specificationStrategyDraftValid(value: unknown): value is SpecificationStrategy {
  const record = (v: unknown): v is Record<string, unknown> => !!v && typeof v === 'object' && !Array.isArray(v);
  const text = (v: unknown, max = 12000) => typeof v === 'string' && v.length <= max;
  const keys = (v: Record<string, unknown>, names: string[]) => Object.keys(v).sort().join(',') === names.sort().join(',');
  const links = (v: unknown) => Array.isArray(v) && v.length <= 100 && v.every(id => text(id, 256));
  const rows = (v: unknown, max: number, fields: string[], check: (v: Record<string, unknown>) => boolean) => Array.isArray(v) && v.length <= max && v.every(row => record(row) && keys(row, ['id', ...fields]) && text(row.id, 256) && check(row));
  return record(value) && keys(value, ['customer', 'problem', 'valueProposition', 'alternatives', 'assumptions', 'evidence', 'metrics']) &&
    text(value.customer) && text(value.problem) && text(value.valueProposition) &&
    rows(value.evidence, 80, ['source', 'url', 'observation', 'observedAt'], row => text(row.source, 500) && text(row.url, 2048) && text(row.observation) && text(row.observedAt, 40)) &&
    rows(value.alternatives, 40, ['name', 'difference', 'evidenceIds'], row => text(row.name, 500) && text(row.difference) && links(row.evidenceIds)) &&
    rows(value.assumptions, 60, ['claim', 'status', 'experiment', 'evidenceIds', 'requirementIds'], row => text(row.claim) && typeof row.status === 'string' && ['untested', 'supported', 'contradicted'].includes(row.status) && text(row.experiment) && links(row.evidenceIds) && links(row.requirementIds)) &&
    rows(value.metrics, 40, ['name', 'baseline', 'target', 'window', 'measurement', 'requirementIds'], row => text(row.name, 500) && text(row.baseline, 2000) && text(row.target, 2000) && text(row.window, 2000) && text(row.measurement) && links(row.requirementIds));
}

export function strategyProblems(strategy: SpecificationStrategy | undefined, requirementIds: string[]): string[] {
  if (!strategy) return [];
  const errors: string[] = [], sources = new Set(strategy.evidence.map(row => row.id)), requirements = new Set(requirementIds);
  const sourceLinks = [...strategy.alternatives, ...strategy.assumptions];
  if (sourceLinks.some(row => row.evidenceIds.some(id => !sources.has(id)))) errors.push('Remove unavailable source links or restore the source before saving.');
  if ([...strategy.assumptions, ...strategy.metrics].some(row => row.requirementIds.some(id => !requirements.has(id)))) errors.push('Review strategy links to removed requirements before saving.');
  if (strategy.assumptions.some(row => row.status !== 'untested' && !row.evidenceIds.length)) errors.push('Link an observation before marking an assumption supported or contradicted.');
  if (strategy.evidence.some(row => !row.source.trim() || !row.observation.trim())) errors.push('Each source needs its name and the observation it supports.');
  if (strategy.alternatives.some(row => !row.name.trim()) || strategy.assumptions.some(row => !row.claim.trim()) || strategy.metrics.some(row => !row.name.trim())) errors.push('Name each alternative and success measure, and state each assumption before saving.');
  return errors;
}
