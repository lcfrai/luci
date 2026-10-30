import type { OrchestratorContext, OrchestratorProposal, OrchestratorTurn } from './orchestrator';
import type { ImplementationPlan } from './product';
import { validProjectSkills, type ProjectSkillAssignment, type ProjectSkillRegistry, type ProjectSkillsSave, type SkillRole } from './projectSkills';

export interface OrchestratorSkillSummary {
  id: string; revision: number; digest: string; title: string; provenance: string; roles: SkillRole[]; instructionExcerpt: string;
}
export interface OrchestratorSkillsContext {
  projectId: string; revision: number; registryDigest: string; versions: OrchestratorSkillSummary[];
  omittedVersions: number; assignments: ProjectSkillAssignment[];
}
export interface OrchestratorSkillSuggestion {
  taskId: string; role: SkillRole; skillId: string; skillRevision: number; skillDigest: string; reason: string;
}
const id = (v: unknown): v is string => typeof v === 'string' && /^[A-Za-z0-9_-]{1,80}$/.test(v);
const hash = (v: unknown) => typeof v === 'string' && /^[a-f0-9]{64}$/.test(v);
const role = (v: unknown): v is SkillRole => v === 'coding' || v === 'reviewer';
const bytes = (s: string) => new TextEncoder().encode(s).length;
const text = (s: unknown, max: number): s is string => typeof s === 'string' && !!s.trim() && bytes(s) <= max && !s.includes('\0');
const positive = (n: number) => Number.isSafeInteger(n) && n > 0;
export function validOrchestratorSkillsContext(value: OrchestratorSkillsContext | undefined, projectId: string): boolean {
  if (value === undefined) return true;
  if (!value || value.projectId !== projectId || !Number.isSafeInteger(value.revision) || value.revision < 0 || !hash(value.registryDigest) ||
      !Array.isArray(value.versions) || value.versions.length > 24 || !Number.isSafeInteger(value.omittedVersions) || value.omittedVersions < 0 || value.versions.length + value.omittedVersions > 128 ||
      !Array.isArray(value.assignments) || value.assignments.length > 64 || bytes(JSON.stringify(value)) > 49152) return false;
  const seen = new Set<string>();
  for (const v of value.versions) {
    const key = `${v?.id}:${v?.revision}`;
    if (!v || !id(v.id) || !positive(v.revision) || !hash(v.digest) || !text(v.title, 200) || !text(v.provenance, 256) || !text(v.instructionExcerpt, 512) ||
        !Array.isArray(v.roles) || v.roles.length < 1 || v.roles.length > 2 || v.roles.some(r => !role(r)) || new Set(v.roles).size !== v.roles.length || seen.has(key)) return false;
    seen.add(key);
  }
  const assignments = new Set<string>();
  return value.assignments.every(a => {
    const key = `${a?.role}/${a?.taskId ?? ''}/${a?.skillId}`;
    if (!a || !role(a.role) || a.taskId !== undefined && a.taskId !== '' && !id(a.taskId) || !id(a.skillId) || !positive(a.skillRevision) || assignments.has(key)) return false;
    assignments.add(key); return true;
  });
}
export function validSuggestedTaskSkills(context: OrchestratorContext, proposal: OrchestratorProposal | null | undefined): boolean {
  const suggested = proposal?.suggestedSkillAssignments;
  if (suggested === undefined) return true;
  if (!Array.isArray(suggested) || suggested.length > 32 || suggested.length > 0 && (proposal?.target !== 'plan' || !proposal.plan || !context.skills)) return false;
  const seen = new Set<string>();
  const perTask = new Map<string, number>();
  return suggested.every(a => {
    const task = proposal?.plan?.tasks.find(t => t.id === a?.taskId);
    const version = context.skills?.versions.find(v => v.id === a?.skillId && v.revision === a?.skillRevision);
    const key = `${a?.role}/${a?.taskId}/${a?.skillId}`;
    if (!a || !task || !id(a.taskId) || !role(a.role) || (task.role || 'coding') !== a.role || !version?.roles.includes(a.role) || a.skillDigest !== version.digest || !text(a.reason, 1000) || seen.has(key)) return false;
    seen.add(key); perTask.set(a.taskId, (perTask.get(a.taskId) ?? 0) + 1); return perTask.get(a.taskId)! <= 16;
  });
}
// Stable equality covers full task intent, not inferred task completion.
function stable(value: unknown): string {
  if (Array.isArray(value)) return `[${value.map(stable).join(',')}]`;
  if (value && typeof value === 'object') return `{${Object.entries(value).filter(([, v]) => v !== undefined).sort(([a], [b]) => a.localeCompare(b)).map(([k, v]) => `${JSON.stringify(k)}:${stable(v)}`).join(',')}}`;
  return JSON.stringify(value) ?? 'null';
}
export function suggestedTaskSkillsSave(turn: OrchestratorTurn, plan: ImplementationPlan, registry: ProjectSkillRegistry): ProjectSkillsSave {
  const proposal = turn.answer?.proposal, context = turn.context.skills;
  if (!turn.appliedRevision || proposal?.target !== 'plan' || !proposal.plan || !context || !proposal.suggestedSkillAssignments?.length ||
      !validOrchestratorSkillsContext(context, turn.projectId) || !validSuggestedTaskSkills(turn.context, proposal) || !validProjectSkills(registry, turn.projectId) ||
      registry.revision !== context.revision || stable(registry.assignments) !== stable(context.assignments) || plan.projectId !== turn.projectId ||
      plan.revision !== turn.appliedRevision || stable(plan.tasks) !== stable(proposal.plan.tasks)) throw new Error('The saved plan or skill registry changed. Review a fresh recommendation before saving task skills.');
  const suggested = proposal.suggestedSkillAssignments;
  for (const s of suggested) {
    const version = registry.versions.find(v => v.id === s.skillId && v.revision === s.skillRevision);
    if (!version || version.digest !== s.skillDigest || !version.roles.includes(s.role)) throw new Error('A suggested skill version changed or is unavailable.');
  }
  // Replace only explicit task/role selections named by this recommendation.
  // Role-wide defaults and every unrelated task assignment are retained.
  const targets = new Set(suggested.map(s => `${s.role}/${s.taskId}`));
  const assignments = registry.assignments.filter(a => !targets.has(`${a.role}/${a.taskId ?? ''}`)).map(a => ({ ...a }));
  assignments.push(...suggested.map(s => ({ role: s.role, taskId: s.taskId, skillId: s.skillId, skillRevision: s.skillRevision })));
  const merged = { ...registry, assignments };
  if (!validProjectSkills(merged, turn.projectId)) throw new Error('The complete assignment set is invalid or exceeds its limit. Review Project skills.');
  for (const task of plan.tasks) {
    const selected = assignments.filter(a => a.role === (task.role || 'coding') && (!a.taskId || a.taskId === task.id));
    const revisions = new Map<string, number>();
    for (const a of selected) {
      if (revisions.has(a.skillId) && revisions.get(a.skillId) !== a.skillRevision) throw new Error('A role-wide selection conflicts with this task skill revision. Review Project skills.');
      revisions.set(a.skillId, a.skillRevision);
    }
    if (revisions.size > 16) throw new Error('A task may use at most 16 skills. Review Project skills.');
  }
  return { expectedRevision: registry.revision, expectedPlanRevision: plan.revision, assignments };
}
