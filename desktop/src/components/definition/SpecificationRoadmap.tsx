import type { ProjectSpecification, Requirement, RoadmapPhase } from '../../api/definition';
import { definitionDraftValid } from '../../hooks/projectDraftValidation';
import { Button } from '../fluid/components/button';

// Retained browser drafts may be unfinished, but their new optional field must
// still have the closed shape the editor can safely read. Never coerce it.
export function specificationRoadmapDraftValid(value: unknown): value is ProjectSpecification {
  if (!definitionDraftValid(value)) return false;
  const phases = value.roadmap;
  const text = (item: unknown): item is string => typeof item === 'string' && item.length <= 12000;
  const texts = (item: unknown, max: number): item is string[] => Array.isArray(item) && item.length <= max && item.every(text);
  return phases === undefined || Array.isArray(phases) && phases.length <= 100 && phases.every(phase =>
    !!phase && typeof phase === 'object' && Object.keys(phase).sort().join(',') === 'completionCriteria,dependsOn,id,outcome,requirementIds,title' &&
    text(phase.id) && text(phase.title) && text(phase.outcome) && texts(phase.requirementIds, 100) && texts(phase.dependsOn, 100) && texts(phase.completionCriteria, 30));
}

export function roadmapProblems(phases: RoadmapPhase[], requirements: Requirement[]): string[] {
  const problems: string[] = [], ids = new Set(phases.map(phase => phase.id)), requirementIds = new Set(requirements.map(item => item.id));
  for (const phase of phases) {
    const title = phase.title.trim() || 'Untitled phase';
    if (!phase.title.trim()) problems.push('Name every roadmap phase before saving.');
    if (phase.requirementIds.some(id => !requirementIds.has(id))) problems.push(`${title} refers to a removed requirement. Remove its unavailable link or restore the requirement.`);
    if (phase.dependsOn.some(id => !ids.has(id))) problems.push(`${title} refers to a removed phase. Remove its unavailable dependency or restore that phase.`);
    if (phase.dependsOn.includes(phase.id)) problems.push(`${title} cannot depend on itself.`);
  }
  const visiting = new Set<string>(), visited = new Set<string>(), byId = new Map(phases.map(phase => [phase.id, phase]));
  const visit = (id: string): boolean => {
    if (visiting.has(id)) return false;
    if (visited.has(id)) return true;
    visiting.add(id);
    for (const dependency of byId.get(id)?.dependsOn ?? []) if (ids.has(dependency) && !visit(dependency)) return false;
    visiting.delete(id); visited.add(id); return true;
  };
  if (phases.some(phase => !visit(phase.id))) problems.push('Roadmap dependencies form a cycle. Remove a dependency before saving.');
  return [...new Set(problems)];
}

export function SpecificationRoadmap({phases, requirements, onChange}: {
  phases: RoadmapPhase[]; requirements: Requirement[]; onChange(phases: RoadmapPhase[]): void;
}) {
  const change = (id: string, patch: Partial<RoadmapPhase>) => onChange(phases.map(phase => phase.id === id ? {...phase, ...patch} : phase));
  const toggle = (values: string[], id: string, checked: boolean) => checked ? [...values, id] : values.filter(value => value !== id);
  const move = (index: number, offset: number) => {
    const next = [...phases], target = index + offset;
    if (target < 0 || target >= next.length) return;
    [next[index], next[target]] = [next[target], next[index]]; onChange(next);
  };
  const problems = roadmapProblems(phases, requirements);
  return <section aria-labelledby="specification-roadmap-title">
    <h2 id="specification-roadmap-title">Roadmap and milestones</h2>
    <p className="journey-muted">Plan useful outcomes in phases. Link the requirements each phase carries, its prerequisites and how you will check completion. Saving this plan does not schedule work or prove it is finished.</p>
    {problems.length > 0 && <div role="alert" className="journey-warning">{problems.map(problem => <p key={problem}>{problem}</p>)}</div>}
    {!phases.length && <p>Start with the first useful version, then add the outcomes that follow. You can leave the roadmap empty while defining the idea.</p>}
    {phases.map((phase, index) => <fieldset className="definition-record" key={phase.id}>
      <legend>Phase {index + 1}</legend>
      <small className="journey-muted">{phase.id}</small>
      <label className="luci-field">Phase title<input value={phase.title} maxLength={200} onChange={event => change(phase.id, {title: event.target.value})}/></label>
      <label className="luci-field">Outcome<textarea aria-label="Outcome" rows={3} value={phase.outcome} maxLength={12000} onChange={event => change(phase.id, {outcome: event.target.value})}/><small>What useful result will people have at this milestone?</small></label>
      <fieldset className="definition-links"><legend>Requirements delivered by this phase</legend>
        {requirements.map(requirement => <label key={requirement.id}><input type="checkbox" checked={phase.requirementIds.includes(requirement.id)} onChange={event => change(phase.id, {requirementIds: toggle(phase.requirementIds, requirement.id, event.target.checked)})}/>{requirement.id} · {requirement.title}</label>)}
        {!requirements.length && <p>Add requirements in Specification, then link them here.</p>}
        {phase.requirementIds.filter(id => !requirements.some(item => item.id === id)).map(id => <label key={id}><input type="checkbox" checked onChange={() => change(phase.id, {requirementIds: phase.requirementIds.filter(value => value !== id)})}/>{id} · removed requirement. Uncheck to remove this unavailable link.</label>)}
      </fieldset>
      <fieldset className="definition-links"><legend>Depends on phases</legend>
        {phases.filter(item => item.id !== phase.id).map(dependency => <label key={dependency.id}><input type="checkbox" checked={phase.dependsOn.includes(dependency.id)} onChange={event => change(phase.id, {dependsOn: toggle(phase.dependsOn, dependency.id, event.target.checked)})}/>{dependency.title || dependency.id}</label>)}
        {phase.dependsOn.filter(id => id === phase.id || !phases.some(item => item.id === id)).map(id => <label key={id}><input type="checkbox" checked onChange={() => change(phase.id, {dependsOn: phase.dependsOn.filter(value => value !== id)})}/>{id} · unavailable dependency. Uncheck to remove it.</label>)}
      </fieldset>
      <label className="luci-field">Completion criteria<textarea aria-label="Completion criteria" rows={4} value={phase.completionCriteria.join('\n')} onChange={event => change(phase.id, {completionCriteria: event.target.value ? event.target.value.split('\n') : []})}/><small>One observable check per line. Record an outcome and linked requirements before approving this roadmap with the definition.</small></label>
      <div className="journey-actions">
        <Button type="button" variant="secondary" disabled={index === 0} onClick={() => move(index, -1)}>Move phase {index + 1} up</Button>
        <Button type="button" variant="secondary" disabled={index === phases.length - 1} onClick={() => move(index, 1)}>Move phase {index + 1} down</Button>
        <Button type="button" variant="secondary" onClick={() => onChange(phases.filter(item => item.id !== phase.id))}>Remove phase {index + 1}</Button>
      </div>
    </fieldset>)}
    <Button type="button" variant="secondary" disabled={phases.length >= 100} onClick={() => onChange([...phases, {id: `phase-${crypto.randomUUID().slice(0, 12)}`, title: '', outcome: '', requirementIds: [], dependsOn: [], completionCriteria: []}])}>Add roadmap phase</Button>
  </section>;
}
