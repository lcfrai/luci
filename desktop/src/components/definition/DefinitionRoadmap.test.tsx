import {afterEach, beforeEach, expect, it, vi} from 'vitest';
import {cleanup, render, screen, waitFor, within} from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import {definitionApi, type ProjectSpecification, type RoadmapPhase} from '../../api/definition';
import {productApi} from '../../api/product';
import {resetCockpit} from '../../store/store';
import {DefinitionWorkspace} from './DefinitionWorkspace';
import {roadmapProblems, specificationRoadmapDraftValid} from './SpecificationRoadmap';

const access = vi.hoisted(() => ({canRead: true, canEdit: true, message: ''}));
vi.mock('../../hooks/useProjectAccess', () => ({useProjectAccess: () => access}));
vi.mock('../auth/AuthProvider', () => ({useAuth: () => ({profile: {user: {id: 'roadmap-owner'}}})}));
vi.mock('../setup/SetupProvider', () => ({useSetup: () => ({status: {access: {user: {username: 'roadmap-owner'}}, config: {projectsDir: '/roadmap-projects'}}})}));

const phase = (id: string, title: string, dependsOn: string[] = []): RoadmapPhase => ({id, title, outcome: `Useful outcome for ${title}`, requirementIds: ['AC-save'], dependsOn, completionCriteria: ['Saved work survives reopening']});
const fixture = (): ProjectSpecification => ({projectId: 'p-1', version: 1, revision: 2, briefRevision: 3, startingPoint: 'new', purpose: 'Repair requests', audience: 'Neighbours', constraints: '', outOfScope: [], requirements: [{id: 'AC-save', title: 'Save request', priority: 'must', acceptance: 'A saved request returns after reopening'}], journeys: [], roles: [], entities: [], integrations: [], glossary: [], decisions: [], questions: [], status: 'draft', digest: '', updatedAt: ''});

beforeEach(() => {
  localStorage.clear(); resetCockpit(); access.canRead = true; access.canEdit = true;
  vi.spyOn(definitionApi, 'listAssistance').mockResolvedValue({jobs: []});
  vi.spyOn(productApi, 'agentConnections').mockResolvedValue({revision: 0, activeConnectionId: '', connections: []});
  vi.spyOn(productApi, 'agents').mockResolvedValue({providers: []});
});
afterEach(() => {cleanup(); vi.restoreAllMocks();});

it('edits linked roadmap phases and reloads their stable dependency IDs through the canonical definition save', async () => {
  let saved = fixture();
  vi.spyOn(definitionApi, 'get').mockImplementation(async () => saved);
  const save = vi.spyOn(definitionApi, 'save').mockImplementation(async (_id, value) => {
    saved = {...value, revision: value.revision + 1, briefRevision: value.briefRevision + 1}; return saved;
  });
  const approve = vi.spyOn(definitionApi, 'approve'), assist = vi.spyOn(definitionApi, 'assist');
  const view = render(<DefinitionWorkspace projectId="p-1"/>);
  await screen.findByRole('textbox', {name: 'The idea'});
  await userEvent.click(screen.getByRole('tab', {name: 'Roadmap'}));
  await userEvent.click(screen.getByRole('button', {name: 'Add roadmap phase'}));
  const first = within(screen.getByRole('group', {name: 'Phase 1'}));
  await userEvent.type(first.getByRole('textbox', {name: 'Phase title'}), 'Request a repair');
  await userEvent.type(first.getByRole('textbox', {name: 'Outcome'}), 'Neighbours can request one visit');
  await userEvent.click(first.getByRole('checkbox', {name: /AC-save/}));
  await userEvent.type(first.getByRole('textbox', {name: 'Completion criteria'}), 'A submitted request returns after reopening');
  await userEvent.click(screen.getByRole('button', {name: 'Add roadmap phase'}));
  const second = within(screen.getByRole('group', {name: 'Phase 2'}));
  await userEvent.type(second.getByRole('textbox', {name: 'Phase title'}), 'Review requests');
  await userEvent.click(second.getByRole('checkbox', {name: 'Request a repair'}));
  await userEvent.click(second.getByRole('button', {name: 'Move phase 2 up'}));
  expect(save).not.toHaveBeenCalled(); expect(approve).not.toHaveBeenCalled(); expect(assist).not.toHaveBeenCalled();
  await userEvent.click(screen.getByRole('button', {name: 'Save definition'}));
  await waitFor(() => expect(save).toHaveBeenCalledOnce());
  const submitted = save.mock.calls[0][1];
  expect(submitted.roadmap?.[0].title).toBe('Review requests');
  expect(submitted.roadmap?.[0].dependsOn).toEqual([submitted.roadmap?.[1].id]);
  expect(submitted.roadmap?.[1].requirementIds).toEqual(['AC-save']);
  expect(save.mock.calls[0].slice(2)).toEqual([2, 3]);
  view.unmount(); render(<DefinitionWorkspace projectId="p-1"/>);
  await screen.findByRole('textbox', {name: 'The idea'});
  await userEvent.click(screen.getByRole('tab', {name: 'Roadmap'}));
  const reloaded = within(screen.getByRole('group', {name: 'Phase 1'}));
  expect((reloaded.getByRole('textbox', {name: 'Phase title'}) as HTMLInputElement).value).toBe('Review requests');
  expect((reloaded.getByRole('checkbox', {name: 'Request a repair'}) as HTMLInputElement).checked).toBe(true);
  expect(approve).not.toHaveBeenCalled(); expect(assist).not.toHaveBeenCalled();
});

it('preserves a removed phase dependency until explicit repair instead of silently changing the plan', async () => {
  vi.spyOn(definitionApi, 'get').mockResolvedValue({...fixture(), roadmap: [phase('phase-request', 'Request'), phase('phase-review', 'Review', ['phase-request'])]});
  const save = vi.spyOn(definitionApi, 'save').mockImplementation(async (_id, value) => ({...value, revision: 3, briefRevision: 4}));
  render(<DefinitionWorkspace projectId="p-1"/>);
  await screen.findByRole('textbox', {name: 'The idea'});
  await userEvent.click(screen.getByRole('tab', {name: 'Roadmap'}));
  await userEvent.click(screen.getByRole('button', {name: 'Remove phase 1'}));
  expect((screen.getByRole('button', {name: 'Save definition'}) as HTMLButtonElement).disabled).toBe(true);
  expect(screen.getByRole('alert').textContent).toContain('removed phase');
  await userEvent.click(screen.getByRole('checkbox', {name: /phase-request · unavailable dependency/}));
  await userEvent.click(screen.getByRole('button', {name: 'Save definition'}));
  await waitFor(() => expect(save).toHaveBeenCalledOnce());
  expect(save.mock.calls[0][1].roadmap?.[0]).toEqual({...phase('phase-review', 'Review'), dependsOn: []});
});

it('keeps roadmap edits and approval unavailable to a read-only project viewer', async () => {
  access.canEdit = false;
  vi.spyOn(definitionApi, 'get').mockResolvedValue({...fixture(), roadmap: [phase('phase-request', 'Request')]});
  const save = vi.spyOn(definitionApi, 'save'), approve = vi.spyOn(definitionApi, 'approve');
  render(<DefinitionWorkspace projectId="p-1"/>);
  await screen.findByRole('textbox', {name: 'The idea'});
  await userEvent.click(screen.getByRole('tab', {name: 'Roadmap'}));
  const title = screen.getByRole('textbox', {name: 'Phase title'});
  expect(title.closest('fieldset[disabled]')).not.toBeNull();
  await userEvent.type(title, ' forbidden');
  await userEvent.click(screen.getByRole('button', {name: 'Add roadmap phase'}));
  expect((title as HTMLInputElement).value).toBe('Request');
  expect(screen.queryByRole('group', {name: 'Phase 2'})).toBeNull();
  expect(save).not.toHaveBeenCalled(); expect(approve).not.toHaveBeenCalled();
});

it('accepts legacy and unfinished roadmap recovery but rejects malformed new retained fields', () => {
  expect(specificationRoadmapDraftValid(fixture())).toBe(true);
  expect(specificationRoadmapDraftValid({...fixture(), roadmap: [{...phase('phase-a', ''), outcome: '', completionCriteria: []}]})).toBe(true);
  expect(specificationRoadmapDraftValid({...fixture(), roadmap: null})).toBe(false);
  expect(specificationRoadmapDraftValid({...fixture(), roadmap: [{...phase('phase-a', 'A'), dependsOn: null}]})).toBe(false);
  expect(specificationRoadmapDraftValid({...fixture(), roadmap: [{...phase('phase-a', 'A'), completed: true}]})).toBe(false);
});

it('detects circular phase prerequisites without treating display order as execution or completion', () => {
  const phases = [phase('phase-b', 'Second', ['phase-a']), phase('phase-a', 'First')];
  expect(roadmapProblems(phases, fixture().requirements)).toEqual([]);
  expect(roadmapProblems([phases[0], {...phases[1], dependsOn: ['phase-b']}], fixture().requirements).join(' ')).toContain('cycle');
});
