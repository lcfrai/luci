import { useEffect, useRef, useState } from 'react';
import type { KeyboardEvent } from 'react';
import { useCockpit } from '../../store/store';
import { selectFocusedSessionId } from '../../store/selectors';
import type { SessionId } from '../../store/types';
import { useProjectDraftScope } from '../../hooks/useProjectDraft';
import { Button } from '../fluid/components/button';
import { StatusDot } from '../common/StatusDot';
import { statusLabel } from './SessionPaneHeader';
import { CanvasLayout } from './CanvasLayout';
import { ColumnsLayout } from './ColumnsLayout';
import { DEFAULT_WORKSPACE_LAYOUT, readWorkspaceLayout, workspaceLayoutKey, type WorkspaceLayout } from './workspaceLayout';

function SessionChoice({ id, selected, onChoose }: { id: SessionId; selected: boolean; onChoose(id: SessionId): void }) {
  const session = useCockpit(state => state.sessions[id]);
  if (!session) return null;
  const attention = session.startFailure ? 'Start needs attention' : session.implementationOutcome?.state === 'failed' ? 'Failed; inspect output' : '';
  return <button type="button" className="develop-session-choice" aria-pressed={selected}
    title={`${session.title} · ${statusLabel(session)}${attention ? ` · ${attention}` : ''}`}
    onClick={() => onChoose(id)}>
    <StatusDot status={session.status} live={session.status === 'running'} label={statusLabel(session)} />
    <span className="develop-session-name">{session.title}</span>
    <span className="develop-session-state">{attention || statusLabel(session)}</span>
  </button>;
}

export function DevelopWorkspace({ projectId, sessionIds, canvasIds }: { projectId: string | null; sessionIds: SessionId[]; canvasIds: SessionId[] }) {
  const scope = useProjectDraftScope(projectId ?? '');
  const branchId = useCockpit(state => state.selection.branchId);
  const key = workspaceLayoutKey(scope, branchId);
  // Changing account, workspace or branch remounts only layout chrome. Actual
  // sessions and their drafts remain in the existing registry/store.
  return <Workspace key={key ?? JSON.stringify([projectId, branchId])} storageKey={key} sessionIds={sessionIds} canvasIds={canvasIds} />;
}

function Workspace({ storageKey, sessionIds, canvasIds }: { storageKey: string | null; sessionIds: SessionId[]; canvasIds: SessionId[] }) {
  const [initial] = useState(() => {
    try { return { value: readWorkspaceLayout(storageKey), error: '' }; }
    catch { return { value: { ...DEFAULT_WORKSPACE_LAYOUT }, error: 'The saved layout could not be read. Changes apply here; adjust the layout to try saving again.' }; }
  });
  const [prefs, setPrefs] = useState(initial.value);
  const [storageError, setStorageError] = useState(initial.error);
  const [compact, setCompact] = useState(false);
  const host = useRef<HTMLDivElement>(null);
  const focusedId = useCockpit(selectFocusedSessionId);
  const activeId = focusedId && sessionIds.includes(focusedId) ? focusedId : sessionIds[0];
  const layout = useCockpit(state => state.layout);
  const setLayout = useCockpit(state => state.setLayout);
  const setLens = useCockpit(state => state.setLens);
  const focusSession = useCockpit(state => state.focusSession);
  const single = compact || prefs.mode === 'focus';

  useEffect(() => {
    const element = host.current;
    if (!element || typeof ResizeObserver === 'undefined') return;
    const observer = new ResizeObserver(entries => {
      const width = entries[0]?.contentRect.width;
      if (width > 0) setCompact(width < 760);
    });
    observer.observe(element);
    return () => observer.disconnect();
  }, []);

  function update(next: WorkspaceLayout) {
    setPrefs(next);
    if (!storageKey) return;
    try { localStorage.setItem(storageKey, JSON.stringify(next)); setStorageError(''); }
    catch { setStorageError('This layout could not be saved. It still applies until you leave this workspace.'); }
  }

  function chooseByKeyboard(event: KeyboardEvent<HTMLElement>) {
    if (!['ArrowLeft', 'ArrowRight', 'Home', 'End'].includes(event.key)) return;
    const buttons = Array.from(event.currentTarget.querySelectorAll<HTMLButtonElement>('.develop-session-choice'));
    const current = buttons.indexOf(document.activeElement as HTMLButtonElement);
    if (current < 0 || !buttons.length) return;
    const next = event.key === 'Home' ? 0 : event.key === 'End' ? buttons.length - 1 : (current + (event.key === 'ArrowRight' ? 1 : -1) + buttons.length) % buttons.length;
    event.preventDefault();
    // Arrows move through choices; Enter/Space activates the existing session.
    buttons[next].focus();
    buttons[next].scrollIntoView({ block: 'nearest', inline: 'nearest' });
  }

  function focusOutput() {
    if (!activeId) return;
    chooseSession(activeId);
    const pane = Array.from(host.current?.querySelectorAll<HTMLElement>('[data-session-id]') ?? [])
      .find(element => element.dataset.sessionId === activeId);
    (pane?.querySelector<HTMLElement>('textarea.xterm-helper-textarea') ?? pane?.querySelector<HTMLElement>('.pane-body'))?.focus();
  }

  function chooseSession(id: string) {
    const state = useCockpit.getState();
    if (layout === 'canvas' && !canvasIds.includes(id)) {
      state.pinSession(id, sessionIds);
      if (!useCockpit.getState().pinned.includes(id)) return;
    }
    focusSession(id);
  }

  return <div className="develop-live-workspace" ref={host} data-single-pane={single}>
    <div className="develop-workspace-toolbar" aria-label="Develop workspace controls">
      <div className="develop-workspace-views" role="group" aria-label="Session arrangement">
        <Button variant="ghost" active={prefs.mode === 'tiles'} aria-pressed={prefs.mode === 'tiles'} onClick={() => update({ ...prefs, mode: 'tiles' })}>Tiles</Button>
        <Button variant="ghost" active={prefs.mode === 'focus'} aria-pressed={prefs.mode === 'focus'} onClick={() => update({ ...prefs, mode: 'focus' })}>Focus</Button>
        {!single && <Button variant="ghost" active={layout === 'canvas'} aria-pressed={layout === 'canvas'} onClick={() => setLayout(layout === 'canvas' ? 'columns' : 'canvas')}>Arrange canvas</Button>}
        <Button variant="ghost" onClick={focusOutput}>Focus terminal</Button>
      </div>
      <div className="develop-workspace-tools" role="group" aria-label="Project work">
        <Button variant="ghost" onClick={() => setLens('files')}>Files</Button>
        <Button variant="ghost" onClick={() => setLens('merge')}>Diff and delivery</Button>
        <Button variant="ghost" onClick={() => setLens('deploy')}>Preview and deploy</Button>
      </div>
      {!single && layout === 'columns' && <label className="develop-column-size">Pane width
        <input aria-label="Session pane width" type="range" min={320} max={960} step={20} value={prefs.columnWidth}
          onChange={event => update({ ...prefs, columnWidth: Number(event.target.value) })} />
      </label>}
    </div>
    <div className="develop-session-choices" role="group" aria-label="Open sessions" onKeyDown={chooseByKeyboard}>
      {sessionIds.map(id => <SessionChoice key={id} id={id} selected={id === activeId} onChoose={chooseSession} />)}
    </div>
    {storageError && <p className="develop-layout-note" role="status">{storageError}</p>}
    {compact && <p className="develop-layout-note">One pane at a time in this space. Select a session to inspect its real output.</p>}
    <div className="develop-workspace-output">
      {/* Focus/narrow switching hides sibling panes within the chosen layout.
          It does not rebuild that layout or change any task's runtime status. */}
      <WorkspaceOutput layout={layout} single={single} activeId={activeId} sessionIds={sessionIds} canvasIds={canvasIds} paneWidth={prefs.columnWidth} />
    </div>
  </div>;
}

function WorkspaceOutput({ layout, single, activeId, sessionIds, canvasIds, paneWidth }: {
  layout: 'columns' | 'canvas'; single: boolean; activeId: string | undefined; sessionIds: string[]; canvasIds: string[]; paneWidth: number;
}) {
  // Canvas and columns already own bounded working sets. Never mount duplicate
  // SessionPane trees for the same session (the attach registry has one owner).
  return layout === 'canvas' ? <CanvasLayout sessionIds={canvasIds} singleSessionId={single ? activeId : undefined} />
    : <ColumnsLayout sessionIds={sessionIds} singleSessionId={single ? activeId : undefined} paneWidth={paneWidth} />;
}
