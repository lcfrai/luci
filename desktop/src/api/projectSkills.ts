import { productApi } from './product';

export type SkillRole = 'coding' | 'reviewer';
export interface ProjectSkillVersion {
  id: string; revision: number; title: string; provenance: string;
  roles: SkillRole[]; instructions: string; digest: string;
}
export interface ProjectSkillAssignment { role: SkillRole; taskId?: string; skillId: string; skillRevision: number }
export interface ProjectSkillRegistry { projectId: string; revision: number; versions: ProjectSkillVersion[]; assignments: ProjectSkillAssignment[] }
export interface ProjectSkillsSave {
  expectedRevision: number;
  expectedPlanRevision?: number;
  skill?: Omit<ProjectSkillVersion, 'revision' | 'digest'>;
  assignments: ProjectSkillAssignment[];
}
const role = (value: unknown): value is SkillRole => value === 'coding' || value === 'reviewer';
const id = (value: unknown): value is string => typeof value === 'string' && /^[A-Za-z0-9_-]{1,80}$/.test(value);
export function validProjectSkills(value: ProjectSkillRegistry, projectId: string): boolean {
  if (!value || value.projectId !== projectId || !Number.isSafeInteger(value.revision) || value.revision < 0 ||
      !Array.isArray(value.versions) || value.versions.length > 128 || !Array.isArray(value.assignments) || value.assignments.length > 64) return false;
  const versions = new Map<string, ProjectSkillVersion>();
  const latest = new Map<string, number>();
  for (const skill of value.versions) {
    if (!skill || !id(skill.id) || skill.revision !== (latest.get(skill.id) ?? 0) + 1 || typeof skill.title !== 'string' || !skill.title.trim() || skill.title.length > 200 ||
        typeof skill.provenance !== 'string' || !skill.provenance.trim() || skill.provenance.length > 2000 || typeof skill.instructions !== 'string' || !skill.instructions.trim() || skill.instructions.length > 16384 ||
        !Array.isArray(skill.roles) || skill.roles.length < 1 || skill.roles.length > 2 || skill.roles.some(value => !role(value)) || new Set(skill.roles).size !== skill.roles.length ||
        !/^[a-f0-9]{64}$/.test(skill.digest)) return false;
    latest.set(skill.id, skill.revision); versions.set(`${skill.id}:${skill.revision}`, skill);
  }
  const seen = new Set<string>();
  return value.assignments.every(assignment => {
    if (!assignment || !role(assignment.role) || (assignment.taskId !== undefined && assignment.taskId !== '' && !id(assignment.taskId))) return false;
    const version = versions.get(`${assignment.skillId}:${assignment.skillRevision}`);
    const key = `${assignment.role}/${assignment.taskId ?? ''}/${assignment.skillId}`;
    if (!version?.roles.includes(assignment.role) || seen.has(key)) return false;
    seen.add(key); return true;
  });
}
function checked(value: ProjectSkillRegistry, projectId: string): ProjectSkillRegistry {
  if (!validProjectSkills(value, projectId)) throw new Error('The saved skill registry could not be verified. Reload before editing.');
  return value;
}
const path = (projectId: string) => `/v1/product/projects/${encodeURIComponent(projectId)}/skills`;
export const projectSkillsApi = {
  async get(projectId: string, signal?: AbortSignal) {
    return checked(await productApi.request<ProjectSkillRegistry>(path(projectId), undefined, 'GET', 30_000, signal), projectId);
  },
  async save(projectId: string, input: ProjectSkillsSave) {
    const value = checked(await productApi.request<ProjectSkillRegistry>(path(projectId), input), projectId);
    if (value.revision !== input.expectedRevision + 1) throw new Error('The saved skill revision could not be confirmed. Reload the registry before continuing.');
    return value;
  },
};
