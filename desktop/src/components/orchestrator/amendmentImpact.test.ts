import { expect, it } from 'vitest';
import { amendmentImpact } from './amendmentImpact';
import { context } from './fixtures';
import { emptyGTM } from '../../api/gtmWorkspace';
import type { OrchestratorContext, OrchestratorProposal } from '../../api/orchestrator';

function linkedContext(): OrchestratorContext {
  const base = context();
  base.specification.roadmap = [{ id: 'phase-1', title: 'Useful first release', outcome: 'One useful flow', requirementIds: ['REQ-1'], dependsOn: [], completionCriteria: ['Save and reload'] }];
  base.designPlan.screens = [{ id: 'screen-1', name: 'Project editor', route: '/editor', purpose: 'Edit a record', states: ['empty'], widths: [390], requirementIds: ['REQ-1'], accessibility: 'Keyboard accessible' }];
  base.designPlan.scene = { version: 1, frames: [{ id: 'frame-1', name: 'Editor empty', screenId: 'screen-1', state: 'empty', width: 390, height: 844, background: '#ffffff', nodes: [] }] };
  base.plan.tasks = [{ id: 'task-1', title: 'Implement editor', status: 'todo', requirementIds: ['REQ-1'], designFrameIds: ['frame-1'] }, { id: 'task-2', title: 'Verify downstream flow', status: 'todo', dependsOn: ['task-1'] }, { id: 'review-1', title: 'Independent review', status: 'todo', role: 'reviewer', reviewOfTaskId: 'task-1' }];
  base.gtm = { revision: 1, digest: 'a'.repeat(64), sourceDigest: 'b'.repeat(64), content: { ...emptyGTM(), segments: [{ id: 'segment-1', name: 'Small teams', problem: 'Slow updates', value: 'A clear editor', requirementIds: ['REQ-1'] }] }, artifactLinks: [], artifactsOmitted: 0 };
  return base;
}
function specificationProposal(base: OrchestratorContext): OrchestratorProposal {
  return { target: 'specification', summary: 'Clarify the core flow', specification: { ...base.specification, requirements: base.specification.requirements.map(row => ({ ...row, acceptance: 'A meaningful revised observable result' })) }, designPlan: null, plan: null };
}
const idsFor = (impact: ReturnType<typeof amendmentImpact>, stage: string) => impact.groups.find(row => row.id === stage)!.items.map(row => row.id);

it('maps a changed requirement through explicit roadmap, screen, frame, task and customer links', () => {
  const base = linkedContext(), result = amendmentImpact(base, specificationProposal(base));
  expect(result.requirements[0]).toMatchObject({ id: 'REQ-1', reason: 'Requirement changed.' });
  expect(idsFor(result, 'roadmap')).toEqual(['phase-1']); expect(idsFor(result, 'design')).toEqual(['screen-1']);
  expect(idsFor(result, 'frames')).toEqual(['frame-1']); expect(idsFor(result, 'tasks')).toEqual(['task-1', 'task-2', 'review-1']);
  expect(idsFor(result, 'gtm')).toEqual(['segment-1']);
});
it('retains removed-requirement links for review without altering either captured or proposed content', () => {
  const base = linkedContext(), proposal = specificationProposal(base); proposal.specification!.requirements = [];
  const before = JSON.stringify([base, proposal]), result = amendmentImpact(base, proposal);
  expect(result.requirements[0].reason).toBe('Requirement removed.'); expect(idsFor(result, 'tasks')).toContain('task-1');
  expect(JSON.stringify([base, proposal])).toBe(before);
});
it('keeps both sides of a directly edited task link rather than hiding its removed requirement', () => {
  const base = linkedContext();
  const proposal: OrchestratorProposal = { target: 'plan', summary: 'Repair a task', specification: null, designPlan: null, plan: { ...base.plan, tasks: [{ ...base.plan.tasks[0], requirementIds: [] }, ...base.plan.tasks.slice(1)] } };
  const result = amendmentImpact(base, proposal);
  expect(result.requirements.map(row => row.id)).toEqual(['REQ-1']);
  expect(result.groups.find(row => row.id === 'tasks')!.items[0].reason).toBe('Directly changed in this proposal.');
});
it('connects a frame-only edit through its captured screen without pretending to render it', () => {
  const base = linkedContext(), proposal: OrchestratorProposal = { target: 'design-plan', summary: 'Adjust frame', specification: null, plan: null, designPlan: { ...base.designPlan, scene: { version: 1, frames: [{ ...base.designPlan.scene!.frames[0], width: 768 }] } } };
  const result = amendmentImpact(base, proposal);
  expect(result.requirements.map(row => row.id)).toEqual(['REQ-1']); expect(idsFor(result, 'tasks')).toContain('task-1');
  expect(result.groups.find(row => row.id === 'frames')!.items[0].reason).toBe('Directly changed in this proposal.');
});
it('labels legacy absent stages and unlinked work as unknown coverage rather than unaffected', () => {
  const base = context(), proposal = specificationProposal(base), result = amendmentImpact(base, proposal);
  expect(result.groups.find(row => row.id === 'roadmap')!.notes.join(' ')).toContain('No roadmap was captured');
  expect(result.groups.find(row => row.id === 'frames')!.notes.join(' ')).toContain('No visual frame scene');
  expect(result.groups.find(row => row.id === 'tasks')!.notes.join(' ')).toContain('coverage is unknown');
  expect(result.groups.find(row => row.id === 'gtm')!.notes.join(' ')).toContain('GTM was not captured');
  expect(result.groups.find(row => row.id === 'deployment')!.notes).toContain('Deployment topology was not captured.');
});
it('surfaces unsupported and dangling references without inventing a relationship', () => {
  const base = context(); base.plan.tasks[0].requirementIds = ['REQ-missing']; base.plan.tasks[0].designFrameIds = ['frame-missing']; base.plan.tasks[0].dependsOn = ['task-missing'];
  const result = amendmentImpact(base, specificationProposal(base)), tasks = result.groups.find(row => row.id === 'tasks')!;
  expect(tasks.items).toEqual([]); expect(tasks.notes.join(' ')).toContain('absent from'); expect(tasks.notes.join(' ')).toContain('frame reference'); expect(tasks.notes.join(' ')).toContain('dependency/reviewer reference');
  const unsupported: OrchestratorProposal = { target: 'deployment-blueprint', summary: 'Use Deploy', specification: null, designPlan: null, plan: null };
  expect(amendmentImpact(base, unsupported).notes[0]).toContain('no supported requirement-impact projection');
});
it('shows saved deployment revision provenance while refusing a fabricated requirement-to-service map', () => {
  const base = linkedContext();
  base.deploymentBlueprint = { projectId: base.projectId, savedRevision: 7, savedDigest: 'c'.repeat(64), source: { commit: 'd'.repeat(40), configurationRevision: 2, context: { briefRevision: 1, specificationRevision: 2, specificationDigest: 'e'.repeat(64), designRevision: 1, designDigest: 'f'.repeat(64) } }, contextDigest: '1'.repeat(64), sourceReviewDigest: '2'.repeat(64), available: true, unavailableReason: '', omittedBytes: 0 };
  base.deploymentBlueprint.saved = { version: 1, projectId: base.projectId, source: base.deploymentBlueprint.source, infrastructure: { id: 'cluster', kind: 'kubernetes', reference: 'existing', namespace: 'apps' }, services: [] };
  const group = amendmentImpact(base, specificationProposal(base)).groups.find(row => row.id === 'deployment')!;
  expect(group.items).toEqual([]); expect(group.notes.join(' ')).toContain('revision 7'); expect(group.notes.join(' ')).toContain('not requirement-to-service mappings');
  base.deploymentBlueprint = { ...base.deploymentBlueprint, available: false, saved: undefined, unavailableReason: 'saved-topology-unsupported' };
  expect(amendmentImpact(base, specificationProposal(base)).groups.find(row => row.id === 'deployment')!.notes.join(' ')).toContain('unavailable');
});
it('bounds visible affected records and reports the exact remainder', () => {
  const base = linkedContext(); base.plan.tasks = Array.from({ length: 20 }, (_, i) => ({ id: `task-${i}`, title: `Useful task ${i}`, status: 'todo', requirementIds: ['REQ-1'] }));
  const group = amendmentImpact(base, specificationProposal(base)).groups.find(row => row.id === 'tasks')!;
  expect(group.items).toHaveLength(8); expect(group.omitted).toBe(12);
});
it('does not infer requirement impact from an unrelated purpose-only amendment', () => {
  const base = context(), proposal = specificationProposal(base); proposal.specification = { ...base.specification, purpose: 'Reworded audience explanation' };
  const result = amendmentImpact(base, proposal); expect(result.requirements).toEqual([]); expect(result.notes.join(' ')).toContain('Unlinked work is not classified as unaffected');
});
it('preserves legacy roadmap omission while explaining an explicit empty roadmap', () => {
  const base = linkedContext(), proposal = specificationProposal(base); proposal.specification = { ...base.specification, roadmap: undefined };
  expect(idsFor(amendmentImpact(base, proposal), 'roadmap')).toEqual([]);
  proposal.specification.roadmap = [];
  expect(amendmentImpact(base, proposal).groups.find(row => row.id === 'roadmap')!.items[0].reason).toBe('Directly removed in this proposal.');
});
