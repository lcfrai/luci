import { act, cleanup, render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { useEffect } from 'react';
import { afterEach, beforeEach, expect, it, vi } from 'vitest';
import { definitionApi, type ProjectSpecification } from '../../api/definition';
import { productApi } from '../../api/product';
import { requestProjectContextEdit, takeProjectContextEdit } from '../../api/projectContextEvents';
import { resetCockpit, useCockpit } from '../../store/store';
import { DiscoverLens } from './DiscoverLens';

vi.mock('../auth/AuthProvider', () => ({ useAuth: () => ({ profile: { user: { id: 'discover-owner' } } }) }));
vi.mock('../setup/SetupProvider', () => ({ useSetup: () => ({ status: { access: { user: { username: 'discover-owner' } }, config: { projectsDir: '/projects' } } }) }));
vi.mock('../teams/TeamWorkspaceProvider', () => ({
  useTeamWorkspace: () => null,
  useTeamProjectPermissions: () => ({ loading: false, error: '', permissions: { read: true, edit: true, execute: false, secrets: false, deploy: false, admin: false } }),
}));
const sourceState = vi.hoisted(() => ({ empty: false as boolean | null, notes: false, report: undefined as undefined | ((empty: boolean | null) => void) }));
vi.mock('../discover/DiscoverDocuments', () => ({ DiscoverDocuments: ({ onEmptySources }: { onEmptySources?(empty: boolean | null): void }) => {
  useEffect(() => { sourceState.report = onEmptySources; onEmptySources?.(sourceState.empty); }, [onEmptySources]);
  return <section aria-label="Document evidence"><button>Refresh documents</button>{sourceState.notes && <label>Evidence question<textarea /></label>}</section>;
} }));

const scope = () => JSON.stringify([productApi.baseUrl, 'discover-owner', 'local', '', '', 'discover-owner', '/projects', 'p-1']);
const fixture = (): ProjectSpecification => ({ projectId: 'p-1', version: 1, revision: 2, briefRevision: 3, startingPoint: 'new', purpose: 'Resume saved projects', audience: 'Dylan', constraints: 'Local first', outOfScope: [], requirements: [], journeys: [], roles: [], entities: [], integrations: [], glossary: [], decisions: [], questions: [], status: 'draft', digest: '', updatedAt: '' });
const disclosure = () => screen.getByText('Edit the structured project definition').closest('details') as HTMLDetailsElement;
const sourcesDisclosure = () => screen.getByText('Sources').closest('details') as HTMLDetailsElement;
const emptyFixture = (): ProjectSpecification => ({ ...fixture(), revision: 0, purpose: '', audience: '', constraints: '' });
const sections = () => Array.from(screen.getByRole('region', { name: 'Discover workspace' }).children);

beforeEach(() => {
  localStorage.clear(); resetCockpit();
  sourceState.empty = false; sourceState.notes = false; sourceState.report = undefined;
  useCockpit.setState({ selection: { projectId: 'p-1', branchId: null, worktreeId: null, sessionId: null } });
  vi.spyOn(definitionApi, 'get').mockResolvedValue(fixture());
  vi.spyOn(definitionApi, 'listAssistance').mockResolvedValue({ jobs: [] });
  vi.spyOn(productApi, 'agentConnections').mockResolvedValue({ revision: 0, activeConnectionId: '', connections: [] });
  vi.spyOn(productApi, 'agents').mockResolvedValue({ providers: [] });
});
afterEach(() => { cleanup(); vi.restoreAllMocks(); });

it('keeps document evidence and the manually opened definition in the same keyboard-reachable workspace', async () => {
  const save = vi.spyOn(definitionApi, 'save'), assist = vi.spyOn(definitionApi, 'assist');
  render(<DiscoverLens />);
  const workspace = screen.getByRole('region', { name: 'Discover workspace' });
  await userEvent.tab(); expect(document.activeElement).toBe(workspace);
  await userEvent.tab(); expect(document.activeElement).toBe(screen.getByText('Sources'));
  await userEvent.tab(); expect(document.activeElement).toBe(screen.getByRole('button', { name: 'Refresh documents' }));
  await userEvent.tab(); expect(document.activeElement).toBe(screen.getByText('Edit the structured project definition'));
  expect(disclosure().open).toBe(false);
  await userEvent.click(screen.getByText('Edit the structured project definition'));
  expect(disclosure().open).toBe(true);
  expect(workspace.contains(await screen.findByRole('textbox', { name: 'The idea' }))).toBe(true);
  expect(save).not.toHaveBeenCalled(); expect(assist).not.toHaveBeenCalled();
});

it('reveals the definition only for the matching scoped edit request without saving or invoking an agent', async () => {
  const save = vi.spyOn(definitionApi, 'save'), assist = vi.spyOn(definitionApi, 'assist');
  render(<DiscoverLens />);
  await waitFor(() => expect(definitionApi.get).toHaveBeenCalled());
  let cancel = () => {};
  act(() => { cancel = requestProjectContextEdit(scope() + '-other', 'p-1', 'definition'); });
  expect(disclosure().open).toBe(false); cancel();
  act(() => { cancel = requestProjectContextEdit(scope(), 'p-1', 'design'); });
  expect(disclosure().open).toBe(false); cancel();
  act(() => { cancel = requestProjectContextEdit(scope(), 'p-1', 'definition'); });
  await waitFor(() => expect(disclosure().open).toBe(true));
  expect(document.activeElement).toBe(screen.getByText('Edit the structured project definition'));
  expect(screen.getByRole('tab', { name: 'Specification' }).getAttribute('aria-selected')).toBe('true');
  expect(takeProjectContextEdit(scope(), 'p-1', 'definition')).toBe(false);
  await userEvent.click(screen.getByRole('textbox', { name: 'The idea' }));
  await userEvent.type(screen.getByRole('textbox', { name: 'The idea' }), ' and notes');
  expect(document.activeElement).toBe(screen.getByRole('textbox', { name: 'The idea' }));
  expect(save).not.toHaveBeenCalled(); expect(assist).not.toHaveBeenCalled(); cancel();
});

it('opens the outer disclosure before a delayed exact-question edit focuses its answer', async () => {
  let finish!: (value: ProjectSpecification) => void;
  vi.mocked(definitionApi.get).mockReturnValue(new Promise(resolve => { finish = resolve; }));
  const save = vi.spyOn(definitionApi, 'save'), assist = vi.spyOn(definitionApi, 'assist');
  const cancel = requestProjectContextEdit(scope(), 'p-1', 'definition', 'storage');
  render(<DiscoverLens />);
  expect(disclosure().open).toBe(false);
  await act(async () => finish({ ...fixture(), questions: [{ id: 'storage', question: 'Where are notes saved?', answer: '' }] }));
  const group = await screen.findByRole('group', { name: 'Open question 1' });
  const answer = within(group).getByRole('textbox', { name: 'Answer' });
  await waitFor(() => expect(document.activeElement).toBe(answer));
  expect(disclosure().open).toBe(true);
  expect(save).not.toHaveBeenCalled(); expect(assist).not.toHaveBeenCalled(); cancel();
});

it('leads a verified blank project without sources with the idea and one guarded header save action', async () => {
  sourceState.empty = true;
  vi.mocked(definitionApi.get).mockResolvedValue(emptyFixture());
  const save = vi.spyOn(definitionApi, 'save').mockImplementation(async (_id, value) => ({ ...value, revision: 1 }));
  const assist = vi.spyOn(definitionApi, 'assist');
  render(<DiscoverLens />);
  const heading = await screen.findByRole('heading', { name: 'What are we building?' });
  expect(sections()).toEqual([disclosure(), sourcesDisclosure()]);
  expect(disclosure().open).toBe(true); expect(sourcesDisclosure().open).toBe(false);
  const saveButton = screen.getByRole('button', { name: 'Save definition' });
  expect(heading.closest('header')?.contains(saveButton)).toBe(true);
  expect(saveButton).toHaveProperty('disabled', true);
  const idea = screen.getByRole('textbox', { name: 'What should this solution do?' });
  await userEvent.type(idea, 'Help people resume their saved work');
  expect(saveButton).toHaveProperty('disabled', false);
  await userEvent.click(saveButton);
  await waitFor(() => expect(save).toHaveBeenCalledOnce());
  await screen.findByRole('button', { name: 'Approve definition' });
  expect(sections()).toEqual([disclosure(), sourcesDisclosure()]);
  expect(screen.getByRole('textbox', { name: 'What should this solution do?' })).toBe(idea);
  expect(idea).toHaveProperty('value', 'Help people resume their saved work');
  expect(assist).not.toHaveBeenCalled();
});

it('keeps existing definition content evidence-first even when the source list is empty', async () => {
  sourceState.empty = true;
  render(<DiscoverLens />);
  await waitFor(() => expect(definitionApi.get).toHaveBeenCalledOnce());
  expect(sections()).toEqual([sourcesDisclosure(), disclosure()]);
  expect(sourcesDisclosure().open).toBe(true); expect(disclosure().open).toBe(false);
  expect(screen.queryByRole('heading', { name: 'What are we building?' })).toBeNull();
});

it('keeps an otherwise blank project document-led when saved sources exist', async () => {
  vi.mocked(definitionApi.get).mockResolvedValue(emptyFixture());
  render(<DiscoverLens />);
  await waitFor(() => expect(definitionApi.get).toHaveBeenCalledOnce());
  expect(sections()).toEqual([sourcesDisclosure(), disclosure()]);
  expect(sourcesDisclosure().open).toBe(true); expect(disclosure().open).toBe(false);
  expect(screen.queryByRole('heading', { name: 'What are we building?' })).toBeNull();
});

it('does not infer a new definition from a failed saved-definition read', async () => {
  sourceState.empty = true;
  vi.mocked(definitionApi.get).mockRejectedValue(new Error('Saved definition unavailable'));
  render(<DiscoverLens />);
  await waitFor(() => expect(definitionApi.get).toHaveBeenCalledOnce());
  await userEvent.click(screen.getByText('Edit the structured project definition'));
  await screen.findByRole('alert');
  expect(sections()).toEqual([sourcesDisclosure(), disclosure()]);
  expect(sourcesDisclosure().open).toBe(true);
  expect(screen.queryByRole('heading', { name: 'What are we building?' })).toBeNull();
});

it('does not treat an unknown source read as an empty project or move an explicitly opened editor on late evidence', async () => {
  sourceState.empty = null;
  vi.mocked(definitionApi.get).mockResolvedValue(emptyFixture());
  render(<DiscoverLens />);
  await waitFor(() => expect(definitionApi.get).toHaveBeenCalledOnce());
  expect(disclosure().open).toBe(false); expect(sourcesDisclosure().open).toBe(true);
  await userEvent.click(screen.getByText('Edit the structured project definition'));
  const idea = screen.getByRole('textbox', { name: 'What should this solution do?' });
  await userEvent.type(idea, 'A manual draft before the source read returns');
  act(() => sourceState.report?.(true));
  expect(sections()).toEqual([sourcesDisclosure(), disclosure()]);
  expect(disclosure().open).toBe(true); expect(document.activeElement).toBe(idea);
  expect(idea).toHaveProperty('value', 'A manual draft before the source read returns');
});

it('preserves first-use drafts and explicit source disclosure choices through later source readbacks', async () => {
  sourceState.empty = true;
  vi.mocked(definitionApi.get).mockResolvedValue(emptyFixture());
  render(<DiscoverLens />);
  await screen.findByRole('heading', { name: 'What are we building?' });
  const idea = screen.getByRole('textbox', { name: 'What should this solution do?' });
  await userEvent.type(idea, 'Keep this unsaved idea');
  await userEvent.click(screen.getByText('Sources'));
  expect(sourcesDisclosure().open).toBe(true);
  act(() => sourceState.report?.(false));
  expect(sections()).toEqual([disclosure(), sourcesDisclosure()]);
  expect(sourcesDisclosure().open).toBe(true);
  expect(screen.getByRole('textbox', { name: 'What should this solution do?' })).toBe(idea);
  expect(idea).toHaveProperty('value', 'Keep this unsaved idea');
});

it('keeps the open Sources form and its focused draft when initial metadata arrives after interaction', async () => {
  sourceState.empty = null; sourceState.notes = true;
  vi.mocked(definitionApi.get).mockResolvedValue(emptyFixture());
  render(<DiscoverLens />);
  const question = screen.getByRole('textbox', { name: 'Evidence question' });
  await userEvent.type(question, 'Which source explains this constraint?');
  act(() => sourceState.report?.(true));
  expect(sections()).toEqual([sourcesDisclosure(), disclosure()]);
  expect(sourcesDisclosure().open).toBe(true); expect(disclosure().open).toBe(false);
  expect(document.activeElement).toBe(question);
  expect(screen.getByRole('textbox', { name: 'Evidence question' })).toBe(question);
  expect(question).toHaveProperty('value', 'Which source explains this constraint?');
});

it('does not carry the first-use layout into a different project scope', async () => {
  sourceState.empty = true;
  vi.mocked(definitionApi.get).mockResolvedValue(emptyFixture());
  render(<DiscoverLens />);
  await screen.findByRole('heading', { name: 'What are we building?' });
  sourceState.empty = false;
  vi.mocked(definitionApi.get).mockResolvedValue({ ...fixture(), projectId: 'p-2' });
  act(() => useCockpit.setState({ selection: { projectId: 'p-2', branchId: null, worktreeId: null, sessionId: null } }));
  await waitFor(() => expect(definitionApi.get).toHaveBeenLastCalledWith('p-2', expect.any(AbortSignal)));
  expect(sections()).toEqual([sourcesDisclosure(), disclosure()]);
  expect(disclosure().open).toBe(false); expect(sourcesDisclosure().open).toBe(true);
  expect(screen.queryByRole('heading', { name: 'What are we building?' })).toBeNull();
});
