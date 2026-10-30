import { afterEach, expect, it, vi } from 'vitest';
import { productApi } from './product';
import { projectSkillsApi, validProjectSkills, type ProjectSkillRegistry } from './projectSkills';

const fixture = (): ProjectSkillRegistry => ({ projectId: 'p-skills', revision: 1, versions: [{ id: 'review', revision: 1, title: 'Review saves', provenance: 'Project team', roles: ['reviewer'], instructions: 'Check failed saves preserve input.', digest: 'a'.repeat(64) }], assignments: [{ role: 'reviewer', skillId: 'review', skillRevision: 1 }] });
afterEach(() => vi.restoreAllMocks());

it('uses the project route and preserves exact task, role and revision in an explicit save', async () => {
  const request = vi.spyOn(productApi, 'request').mockResolvedValue(fixture());
  const input = { expectedRevision: 0, assignments: fixture().assignments };
  expect(await projectSkillsApi.save('p-skills', input)).toEqual(fixture());
  expect(request).toHaveBeenCalledWith('/v1/product/projects/p-skills/skills', input);
});

it('refuses cross-project, unknown-version, role-mismatched and unconfirmed responses', async () => {
  const wrongProject = fixture(); wrongProject.projectId = 'p-other';
  const wrongVersion = fixture(); wrongVersion.assignments[0].skillRevision = 2;
  const wrongRole = fixture(); wrongRole.assignments[0].role = 'coding';
  const duplicate = fixture(); duplicate.assignments.push(duplicate.assignments[0]);
  for (const value of [wrongProject, wrongVersion, wrongRole, duplicate]) expect(validProjectSkills(value, 'p-skills')).toBe(false);
  vi.spyOn(productApi, 'request').mockResolvedValue({ ...fixture(), revision: 4 });
  await expect(projectSkillsApi.save('p-skills', { expectedRevision: 1, assignments: [] })).rejects.toThrow('could not be confirmed');
});
