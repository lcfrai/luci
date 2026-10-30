import { expect, it } from 'vitest';
import { context, proposalTurn } from '../components/orchestrator/fixtures';
import type { OrchestratorTurn } from './orchestrator';
import type { ProjectSkillRegistry } from './projectSkills';
import { suggestedTaskSkillsSave, validOrchestratorSkillsContext, validSuggestedTaskSkills } from './orchestratorSkills';

function fixture() {
  const registry: ProjectSkillRegistry = { projectId: 'p-1', revision: 3, versions: [{ id: 'persistence', revision: 1, digest: 'b'.repeat(64), title: 'Persistence', provenance: 'Project instructions', roles: ['coding'], instructions: 'Check save and reload.' }], assignments: [{ role: 'coding', skillId: 'persistence', skillRevision: 1 }, { role: 'coding', taskId: 'unrelated', skillId: 'persistence', skillRevision: 1 }] };
  const base = context(), tasks = [...base.plan.tasks, { id: 'unrelated', title: 'Keep existing work', status: 'todo' as const }];
  const turn: OrchestratorTurn = { ...proposalTurn(), appliedRevision: 4, context: { ...base, skills: { projectId: 'p-1', revision: 3, registryDigest: 'c'.repeat(64), versions: registry.versions.map(v => ({ ...v, instructionExcerpt: v.instructions })), omittedVersions: 0, assignments: registry.assignments } } };
  turn.answer = { ...turn.answer!, proposal: { target: 'plan', summary: 'A narrow implementation', specification: null, designPlan: null, plan: { ...base.plan, tasks }, suggestedSkillAssignments: [{ taskId: 'T-1', role: 'coding', skillId: 'persistence', skillRevision: 1, skillDigest: 'b'.repeat(64), reason: 'Verify save and reload behavior.' }] } };
  const plan = { ...base.plan, revision: 4, tasks };
  return { turn, registry, plan };
}

it('keeps legacy missing catalogues and recommendations valid without synthesizing assignments', () => {
  expect(validOrchestratorSkillsContext(undefined, 'p-1')).toBe(true);
  expect(validSuggestedTaskSkills(context(), proposalTurn().answer!.proposal)).toBe(true);
  const f = fixture(); delete f.turn.appliedRevision;
  expect(() => suggestedTaskSkillsSave(f.turn, f.plan, f.registry)).toThrow(/saved plan or skill registry changed/);
});
it('rejects forged catalogue digests, omitted versions, wrong roles and nonplan suggestions', () => {
  const f = fixture(); expect(validOrchestratorSkillsContext(f.turn.context.skills, 'p-1')).toBe(true);
  expect(validOrchestratorSkillsContext(f.turn.context.skills, 'foreign')).toBe(false);
  for (const patch of [{ skillDigest: '0'.repeat(64) }, { skillRevision: 2 }, { role: 'reviewer' as const }, { taskId: 'missing' }]) {
    const proposal = { ...f.turn.answer!.proposal!, suggestedSkillAssignments: [{ ...f.turn.answer!.proposal!.suggestedSkillAssignments![0], ...patch }] };
    expect(validSuggestedTaskSkills(f.turn.context, proposal)).toBe(false);
  }
  expect(validSuggestedTaskSkills(f.turn.context, { ...f.turn.answer!.proposal!, target: 'gtm' })).toBe(false);
  const missing = { ...f.turn.context, skills: { ...f.turn.context.skills!, versions: [], omittedVersions: 1 } };
  expect(validSuggestedTaskSkills(missing, f.turn.answer!.proposal)).toBe(false);
});
it('requires exact applied plan readback and retains role defaults and unrelated task selections', () => {
  const f = fixture(), before = JSON.stringify(f.registry);
  const request = suggestedTaskSkillsSave(f.turn, f.plan, f.registry);
  expect(request).toEqual({ expectedRevision: 3, expectedPlanRevision: 4, assignments: [...f.registry.assignments, { taskId: 'T-1', role: 'coding', skillId: 'persistence', skillRevision: 1 }] });
  expect(JSON.stringify(f.registry)).toBe(before);
});
it('refuses changed task intent or registry revision rather than rebinding an old recommendation', () => {
  const f = fixture();
  expect(() => suggestedTaskSkillsSave(f.turn, { ...f.plan, revision: 5 }, f.registry)).toThrow();
  expect(() => suggestedTaskSkillsSave(f.turn, { ...f.plan, tasks: f.plan.tasks.map(t => ({ ...t, title: 'Changed intent' })) }, f.registry)).toThrow();
  expect(() => suggestedTaskSkillsSave(f.turn, f.plan, { ...f.registry, revision: 4 })).toThrow();
  expect(() => suggestedTaskSkillsSave(f.turn, f.plan, { ...f.registry, versions: f.registry.versions.map(v => ({ ...v, digest: 'd'.repeat(64) })) })).toThrow(/version changed/);
});
