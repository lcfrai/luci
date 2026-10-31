import type { OrchestratorContext, OrchestratorProposal } from '../../api/orchestrator';

type RecordValue = Record<string, unknown>;
export interface AmendmentImpactItem { id: string; label: string; reason: string }
export interface AmendmentImpactGroup { id: string; title: string; items: AmendmentImpactItem[]; omitted: number; notes: string[] }
export interface AmendmentImpactProjection { requirements: AmendmentImpactItem[]; requirementsOmitted: number; groups: AmendmentImpactGroup[]; notes: string[] }
const record = (value: unknown): value is RecordValue => !!value && typeof value === 'object' && !Array.isArray(value);
const text = (value: unknown, fallback = '') => typeof value === 'string' ? value.slice(0, 180) : fallback;
const ids = (value: unknown): string[] => Array.isArray(value) ? value.slice(0, 200).filter((id): id is string => typeof id === 'string' && id.length > 0 && id.length <= 128) : [];
const rows = (value: unknown): RecordValue[] => Array.isArray(value) ? value.slice(0, 200).filter((row): row is RecordValue => record(row) && !!text(row.id)) : [];
const keyed = (value: unknown) => new Map(rows(value).map(row => [text(row.id), row]));
const valueKey = (value: unknown) => JSON.stringify(value, (_key, item: unknown) => record(item) ? Object.fromEntries(Object.entries(item).sort(([a], [b]) => a < b ? -1 : a > b ? 1 : 0)) : item);
function changed(before: unknown, after: unknown): Map<string, 'added' | 'changed' | 'removed'> {
  const a = keyed(before), b = keyed(after), result = new Map<string, 'added' | 'changed' | 'removed'>();
  for (const [id, row] of a) if (!b.has(id)) result.set(id, 'removed'); else if (valueKey(row) !== valueKey(b.get(id))) result.set(id, 'changed');
  for (const id of b.keys()) if (!a.has(id)) result.set(id, 'added');
  return result;
}
function unionRows(before: unknown, after: unknown): RecordValue[] {
  return [...new Map([...rows(before), ...rows(after)].map(row => [text(row.id), row])).values()];
}
const describe = (row: RecordValue) => text(row.title, text(row.name, text(row.id)));

/** Presentation only: exact captured references, never execution selection or
 * an approval/currentness decision. No fetch, state mutation or model inference.
 * Missing links are unknown coverage, not evidence that work is unaffected. */
export function amendmentImpact(context: OrchestratorContext, proposal: OrchestratorProposal): AmendmentImpactProjection {
  const notes: string[] = [], groups: AmendmentImpactGroup[] = [];
  if (!['specification', 'design-plan', 'plan', 'gtm'].includes(proposal.target)) return { requirements: [], requirementsOmitted: 0, groups, notes: ['This proposal target has no supported requirement-impact projection. Review it in its existing workflow.'] };
  const baseSpec = context.specification, nextSpec = proposal.target === 'specification' ? proposal.specification : null;
  const baseDesign = context.designPlan, nextDesign = proposal.target === 'design-plan' ? proposal.designPlan : null;
  const basePlan = context.plan, nextPlan = proposal.target === 'plan' ? proposal.plan : null;
  const baseGTM = context.gtm?.content, nextGTM = proposal.target === 'gtm' ? proposal.gtm : null;
  const requirements = keyed(baseSpec.requirements), proposedRequirements = keyed(nextSpec?.requirements);
  const requirementChanges = nextSpec ? changed(baseSpec.requirements, nextSpec.requirements) : new Map<string, 'added' | 'changed' | 'removed'>();
  const affected = new Set(requirementChanges.keys());
  const screenChanges = nextDesign ? changed(baseDesign.screens, nextDesign.screens) : new Map<string, 'added' | 'changed' | 'removed'>();
  const frameChanges = nextDesign ? changed(baseDesign.scene?.frames, nextDesign.scene?.frames) : new Map<string, 'added' | 'changed' | 'removed'>();
  const taskChanges = nextPlan ? changed(basePlan.tasks, nextPlan.tasks) : new Map<string, 'added' | 'changed' | 'removed'>();
  const segmentChanges = nextGTM ? changed(baseGTM?.segments, nextGTM.segments) : new Map<string, 'added' | 'changed' | 'removed'>();
  // Legacy omission preserves optional roadmap; an explicit [] may clear it.
  const phaseChanges = nextSpec?.roadmap !== undefined ? changed(baseSpec.roadmap, nextSpec.roadmap) : new Map<string, 'added' | 'changed' | 'removed'>();
  const screens = unionRows(baseDesign.screens, nextDesign?.screens), frames = unionRows(baseDesign.scene?.frames, nextDesign?.scene?.frames);
  const tasks = unionRows(basePlan.tasks, nextPlan?.tasks), segments = unionRows(baseGTM?.segments, nextGTM?.segments);
  const phases = unionRows(baseSpec.roadmap, nextSpec?.roadmap);
  function seed(before: unknown, after: unknown, direct: Map<string, string>) {
    // Both sides matter when an amendment removes an existing link.
    for (const row of [...rows(before), ...rows(after)]) if (direct.has(text(row.id))) for (const id of ids(row.requirementIds)) affected.add(id);
  }
  seed(baseDesign.screens, nextDesign?.screens, screenChanges); seed(basePlan.tasks, nextPlan?.tasks, taskChanges);
  seed(baseGTM?.segments, nextGTM?.segments, segmentChanges); seed(baseSpec.roadmap, nextSpec?.roadmap, phaseChanges);
  const selectedScreens = new Set(screenChanges.keys());
  for (const frame of [...rows(baseDesign.scene?.frames), ...rows(nextDesign?.scene?.frames)]) if (frameChanges.has(text(frame.id))) selectedScreens.add(text(frame.screenId));
  for (const task of [...rows(basePlan.tasks), ...rows(nextPlan?.tasks)]) if (taskChanges.has(text(task.id))) for (const id of ids(task.designFrameIds)) {
    const frame = frames.find(row => row.id === id); if (frame) selectedScreens.add(text(frame.screenId));
  }
  for (const screen of screens) if (selectedScreens.has(text(screen.id))) for (const id of ids(screen.requirementIds)) affected.add(id);
  const knownRequirement = (id: string) => requirements.has(id) || proposedRequirements.has(id);
  const linked = (row: RecordValue) => ids(row.requirementIds).filter(id => affected.has(id));
  const affectedScreens = new Set(screens.filter(row => selectedScreens.has(text(row.id)) || linked(row).length > 0).map(row => text(row.id)));
  const affectedFrames = new Set(frames.filter(row => frameChanges.has(text(row.id)) || affectedScreens.has(text(row.screenId))).map(row => text(row.id)));
  const affectedTasks = new Set(tasks.filter(row => taskChanges.has(text(row.id)) || linked(row).length > 0 || ids(row.designFrameIds).some(id => affectedFrames.has(id))).map(row => text(row.id)));
  // Explain saved dependencies/reviewer references; this does NOT choose or
  // renew agents. The existing coordinator independently owns those decisions.
  for (let pass = 0; pass < tasks.length; pass++) {
    let grew = false;
    for (const task of tasks) if (!affectedTasks.has(text(task.id)) && (ids(task.dependsOn).some(id => affectedTasks.has(id)) || typeof task.reviewOfTaskId === 'string' && affectedTasks.has(task.reviewOfTaskId))) { affectedTasks.add(text(task.id)); grew = true; }
    if (!grew) break;
  }
  function group(id: string, title: string, source: RecordValue[], include: (row: RecordValue) => boolean, reason: (row: RecordValue) => string, extra: string[] = []) {
    const selected = source.filter(include), missing = source.filter(row => ids(row.requirementIds).length === 0).length;
    const unknown = new Set(source.flatMap(row => ids(row.requirementIds)).filter(id => !knownRequirement(id)));
    groups.push({ id, title, items: selected.slice(0, 8).map(row => ({ id: text(row.id), label: describe(row), reason: reason(row) })), omitted: Math.max(0, selected.length - 8), notes: [
      ...(missing ? [`${missing} captured record${missing === 1 ? ' has' : 's have'} no explicit requirement link; coverage is unknown.`] : []),
      ...(unknown.size ? [`${unknown.size} requirement reference${unknown.size === 1 ? ' is' : 's are'} absent from the captured/proposed definition. Review the links.`] : []), ...extra,
    ] });
  }
  const directReason = (row: RecordValue, direct: Map<string, string>) => direct.has(text(row.id)) ? `Directly ${direct.get(text(row.id))} in this proposal.` : `Linked requirement${linked(row).length === 1 ? '' : 's'}: ${linked(row).join(', ')}.`;
  group('roadmap', 'Roadmap phases', phases, row => phaseChanges.has(text(row.id)) || linked(row).length > 0, row => directReason(row, phaseChanges), baseSpec.roadmap === undefined && !nextSpec?.roadmap ? ['No roadmap was captured in this definition.'] : []);
  group('design', 'Design screens', screens, row => affectedScreens.has(text(row.id)), row => screenChanges.has(text(row.id)) || linked(row).length ? directReason(row, screenChanges) : 'Linked through an explicitly changed or task-referenced frame.');
  const unknownScreenLinks = frames.filter(frame => !screens.some(screen => screen.id === frame.screenId)).length;
  groups.push({ id: 'frames', title: 'Design frames', items: frames.filter(row => affectedFrames.has(text(row.id))).slice(0, 8).map(row => ({ id: text(row.id), label: describe(row), reason: frameChanges.has(text(row.id)) ? `Directly ${frameChanges.get(text(row.id))} in this proposal.` : `Captured screen: ${text(row.screenId)}.` })), omitted: Math.max(0, affectedFrames.size - 8), notes: [
    ...(!baseDesign.scene && !nextDesign?.scene ? ['No visual frame scene was captured; planned screens are not rendered frames.'] : []),
    ...(unknownScreenLinks ? [`${unknownScreenLinks} frame screen link${unknownScreenLinks === 1 ? ' is' : 's are'} not present in this snapshot.`] : []),
  ] });
  const danglingFrames = new Set(tasks.flatMap(row => ids(row.designFrameIds)).filter(id => !frames.some(row => row.id === id)));
  const danglingTasks = new Set(tasks.flatMap(row => [...ids(row.dependsOn), ...(typeof row.reviewOfTaskId === 'string' && row.reviewOfTaskId ? [row.reviewOfTaskId] : [])]).filter(id => !tasks.some(row => row.id === id)));
  group('tasks', 'Implementation tasks', tasks, row => affectedTasks.has(text(row.id)), row => taskChanges.has(text(row.id)) || linked(row).length ? directReason(row, taskChanges) : ids(row.designFrameIds).some(id => affectedFrames.has(id)) ? 'References an affected captured design frame.' : 'Saved dependency or reviewer reference points to affected work.', [
    ...(danglingFrames.size ? [`${danglingFrames.size} task frame reference${danglingFrames.size === 1 ? ' is' : 's are'} unavailable in the captured scene.`] : []),
    ...(danglingTasks.size ? [`${danglingTasks.size} dependency/reviewer reference${danglingTasks.size === 1 ? ' is' : 's are'} unavailable in the captured/proposed plan.`] : []),
  ]);
  group('gtm', 'GTM customer segments', segments, row => segmentChanges.has(text(row.id)) || linked(row).length > 0, row => directReason(row, segmentChanges), !context.gtm ? ['GTM was not captured. No market, channel or pricing impact is inferred.'] : ['Only explicit segment-to-requirement links are mapped; other market assumptions need human review.']);
  const deployment = context.deploymentBlueprint, saved = deployment?.saved, provenance = saved?.source.context;
  groups.push({ id: 'deployment', title: 'Saved deployment provenance', items: [], omitted: 0, notes: !deployment ? ['Deployment topology was not captured.'] : deployment.available && deployment.savedRevision === 0 && !saved ? ['No saved deployment topology existed in the captured snapshot.'] : !deployment.available || !saved ? [`Saved deployment detail is unavailable${deployment.unavailableReason ? ` (${text(deployment.unavailableReason)})` : ''}; no service impact is inferred.`] : [
    `Captured saved topology revision ${deployment.savedRevision}, source ${text(saved.source.commit, 'unconfirmed') || 'unconfirmed'}.`,
    ...(provenance ? [`Its recorded definition revision is ${provenance.specificationRevision}; design revision is ${provenance.designRevision}. These are whole-document provenance links, not requirement-to-service mappings.`] : ['No definition/design revision provenance was captured.']),
    'A topology review may be needed. This explanation does not change its saved revision, checks or deployment authority.',
  ] });
  if (affected.size === 0) notes.push('No changed requirement or explicit requirement link was identified. Unlinked work is not classified as unaffected.');
  const collections = [baseSpec.requirements, nextSpec?.requirements, baseSpec.roadmap, nextSpec?.roadmap, baseDesign.screens, nextDesign?.screens, baseDesign.scene?.frames, nextDesign?.scene?.frames, basePlan.tasks, nextPlan?.tasks, baseGTM?.segments, nextGTM?.segments];
  if (collections.some(value => Array.isArray(value) && value.length > 200)) notes.push('The impact view is bounded to 200 records per collection; omitted records are not assessed.');
  const impacted = [...affected].sort().map(id => ({ id, label: text((proposedRequirements.get(id) ?? requirements.get(id))?.title, id), reason: requirementChanges.has(id) ? `Requirement ${requirementChanges.get(id)}.` : knownRequirement(id) ? 'Explicitly linked from directly changed work.' : 'Reference not present in the captured/proposed definition.' }));
  return { requirements: impacted.slice(0, 12), requirementsOmitted: Math.max(0, impacted.length - 12), groups, notes };
}
