import { useEffect } from 'react';
import { act, fireEvent, render, screen } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { resetCockpit, useCockpit } from '../../store/store';
import { aBranch, aProject, aSession, aWorktree } from '../../test/fixtures';
import { DevelopWorkspace } from './DevelopWorkspace';
import { CanvasLayout } from './CanvasLayout';
import { workspaceLayoutKey } from './workspaceLayout';

const mocks = vi.hoisted(() => ({ scope: 'person-one/workspace/project', ended: vi.fn() }));
vi.mock('../../hooks/useProjectDraft', () => ({ useProjectDraftScope: () => mocks.scope }));
vi.mock('./SessionPane', () => ({ SessionPane: function MockSessionPane({ sessionId }: { sessionId: string }) {
  useEffect(() => () => mocks.ended(sessionId), [sessionId]);
  return <section data-session-id={sessionId}><textarea aria-label={`Terminal ${sessionId}`} className="xterm-helper-textarea" /></section>;
} }));

let resize: ResizeObserverCallback = () => {};
class FakeResizeObserver {
  constructor(callback: ResizeObserverCallback) { resize = callback; }
  observe() {} unobserve() {} disconnect() {}
}

function seed() {
  useCockpit.setState({
    projects: [aProject({ id: 'p' })],
    branchesByProject: { p: [aBranch({ id: 'b', projectId: 'p' })] },
    worktreesByBranch: { b: [aWorktree({ id: 'w', projectId: 'p', branchId: 'b' })] },
    sessions: {
      one: aSession({ id: 'one', title: 'Implement orchard', projectId: 'p', branchId: 'b', worktreeId: 'w', status: 'running' }),
      two: aSession({ id: 'two', title: 'Check orchard', projectId: 'p', branchId: 'b', worktreeId: 'w', status: 'exited', exitReason: 'interrupted' }),
    },
    sessionIdsByWorktree: { w: ['one', 'two'] },
    selection: { projectId: 'p', branchId: 'b', worktreeId: 'w', sessionId: 'one' },
    layout: 'columns', pinned: ['one', 'two'],
  });
}

beforeEach(() => {
  resetCockpit(); localStorage.clear(); seed(); mocks.ended.mockReset(); mocks.scope = 'person-one/workspace/project';
  vi.stubGlobal('ResizeObserver', FakeResizeObserver);
  vi.spyOn(Element.prototype, 'scrollIntoView').mockImplementation(() => {});
});
afterEach(() => { vi.restoreAllMocks(); vi.unstubAllGlobals(); });

describe('Develop workspace', () => {
  const workspace = () => <DevelopWorkspace projectId="p" sessionIds={['one', 'two']} canvasIds={['one', 'two']} />;

  it('selects real session identities and retains hidden terminal DOM and input', () => {
    const view = render(workspace());
    const terminal = screen.getByLabelText('Terminal one') as HTMLTextAreaElement;
    terminal.value = 'unsent input';
    fireEvent.click(screen.getByRole('button', { name: 'Focus' }));
    fireEvent.click(screen.getByRole('button', { name: /Check orchard/ }));
    expect(useCockpit.getState().selection.sessionId).toBe('two');
    expect(view.container.querySelector('[data-column-for="one"]')?.hasAttribute('hidden')).toBe(true);
    expect(screen.getByLabelText('Terminal one')).toBe(terminal);
    expect(terminal.value).toBe('unsent input');
    expect(mocks.ended).not.toHaveBeenCalled();
    expect(screen.getByRole('button', { name: /Check orchard/ }).textContent).toContain('interrupted');
  });

  it('switches a narrow container to one pane without changing the saved wide arrangement', () => {
    const view = render(workspace());
    act(() => resize([{ contentRect: { width: 600 } } as ResizeObserverEntry], {} as ResizeObserver));
    expect(view.container.querySelector('[data-single-pane="true"]')).not.toBeNull();
    expect(screen.queryByRole('slider')).toBeNull();
    act(() => resize([{ contentRect: { width: 1100 } } as ResizeObserverEntry], {} as ResizeObserver));
    expect(screen.getByRole('slider')).toBeTruthy();
    expect(mocks.ended).not.toHaveBeenCalled();
  });

  it('saves and restores width and mode only for the exact account/workspace/branch', () => {
    const view = render(workspace());
    fireEvent.change(screen.getByRole('slider'), { target: { value: '640' } });
    fireEvent.click(screen.getByRole('button', { name: 'Focus' }));
    const key = workspaceLayoutKey(mocks.scope, 'b')!;
    expect(JSON.parse(localStorage.getItem(key)!)).toEqual({ mode: 'focus', columnWidth: 640 });
    view.unmount();
    const again = render(workspace());
    expect(screen.getByRole('button', { name: 'Focus' }).getAttribute('aria-pressed')).toBe('true');
    again.unmount(); mocks.scope = 'person-two/workspace/project';
    render(workspace());
    expect((screen.getByRole('slider') as HTMLInputElement).value).toBe('520');
  });

  it('uses keyboard choice navigation without sending arrow keys to another terminal', () => {
    render(workspace());
    const first = screen.getByRole('button', { name: /Implement orchard/ });
    const second = screen.getByRole('button', { name: /Check orchard/ });
    first.focus(); fireEvent.keyDown(first, { key: 'ArrowRight' });
    expect(document.activeElement).toBe(second);
    expect(useCockpit.getState().selection.sessionId).toBe('one');
    fireEvent.click(second); fireEvent.click(screen.getByRole('button', { name: 'Focus terminal' }));
    expect(document.activeElement).toBe(screen.getByLabelText('Terminal two'));
  });

  it('hides canvas siblings in Focus without destroying their terminals or saved geometry', () => {
    useCockpit.setState({ layout: 'canvas', tiles: { one: { x: 20, y: 30, w: 460, h: 320 } } });
    const view = render(workspace());
    const terminal = screen.getByLabelText('Terminal one');
    fireEvent.click(screen.getByRole('button', { name: 'Focus' }));
    fireEvent.click(screen.getByRole('button', { name: /Check orchard/ }));
    expect(view.container.querySelector('[data-cv-tile="one"]')?.hasAttribute('hidden')).toBe(true);
    expect(screen.getByLabelText('Terminal one')).toBe(terminal);
    expect(useCockpit.getState().tiles.one).toEqual({ x: 20, y: 30, w: 460, h: 320 });
    expect(mocks.ended).not.toHaveBeenCalled();
  });

  it('reports storage failures while keeping layout controls usable', () => {
    const view = render(workspace());
    vi.spyOn(Storage.prototype, 'setItem').mockImplementation(() => { throw new Error('quota'); });
    fireEvent.change(screen.getByRole('slider'), { target: { value: '600' } });
    expect(screen.getByRole('status').textContent).toContain('could not be saved');
    expect(view.container.querySelector('.cols-scroll')?.getAttribute('style')).toContain('600px');
  });

  it('moves and resizes real canvas geometry with bounded keyboard steps', () => {
    useCockpit.setState({ tiles: { one: { x: 20, y: 30, w: 460, h: 320 } } });
    render(<CanvasLayout sessionIds={['one']} />);
    const move = screen.getByRole('button', { name: 'Move Implement orchard' });
    fireEvent.keyDown(move, { key: 'ArrowLeft', shiftKey: true });
    expect(useCockpit.getState().tiles.one.x).toBe(0);
    const size = screen.getByRole('button', { name: 'Resize Implement orchard' });
    fireEvent.keyDown(size, { key: 'ArrowRight' });
    expect(useCockpit.getState().tiles.one.w).toBe(470);
  });
});
