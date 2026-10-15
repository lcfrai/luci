import { useState } from 'react';
import { fireEvent, render, screen } from '@testing-library/react';
import { afterEach, beforeEach, expect, it, vi } from 'vitest';
import type { DesignPlan } from '../../api/designPlan';
import { designScenePlanFixture } from '../../test/designSceneFixtures';
import { DesignSceneCanvas } from './DesignSceneCanvas';
beforeEach(() => {
  vi.stubGlobal('PointerEvent', class extends MouseEvent { pointerId: number; constructor(type: string, init: PointerEventInit = {}) { super(type, init); this.pointerId = init.pointerId ?? 1; } });
});
afterEach(() => vi.unstubAllGlobals());
function Harness({initial = designScenePlanFixture(), edit = true, changed = vi.fn(), ask = vi.fn()}: {initial?: DesignPlan; edit?: boolean; changed?: (plan: DesignPlan) => void; ask?: (message: string) => void}) {
  const [plan, setPlan] = useState(initial);
  return <DesignSceneCanvas plan={plan} canRead canEdit={edit} busy={false} workScope={null} onChange={scene => { const next = {...plan, scene}; setPlan(next); changed(next); }} onAsk={ask} onPlan={vi.fn()} />;
}
it('selects real persisted components and edits escaped content through the same draft/history', () => {
  const changed = vi.fn(), initial = designScenePlanFixture(); render(<Harness initial={initial} changed={changed} />);
  fireEvent.click(screen.getByRole('button', {name: 'Select Page title (node-title)'}));
  fireEvent.change(screen.getByLabelText('Content / image description'), {target: {value: '<script>plain text</script>'}}); fireEvent.blur(screen.getByLabelText('Content / image description'));
  expect(changed.mock.calls[0][0].scene.frames[0].nodes[0].text).toBe('<script>plain text</script>'); expect(document.querySelector('script')).toBeNull();
  fireEvent.click(screen.getByRole('button', {name: 'Undo scene edit'})); expect(screen.getByLabelText('Content / image description')).toHaveProperty('value', 'Saved work');
  fireEvent.click(screen.getByRole('button', {name: 'Redo scene edit'})); expect(screen.getByLabelText('Content / image description')).toHaveProperty('value', '<script>plain text</script>');
  expect(initial.scene!.frames[0].nodes[0].text).toBe('Saved work');
});
it('commits one zoom-adjusted drag, cancels another and uses the same undo history', () => {
  const changed = vi.fn(); render(<Harness changed={changed} />); const node = screen.getByRole('button', {name: 'Select Page title (node-title)'});
  fireEvent.pointerDown(node, {pointerId: 1, button: 0, clientX: 0, clientY: 0}); fireEvent.pointerMove(node, {pointerId: 1, clientX: 30, clientY: 15});
  expect(changed).not.toHaveBeenCalled(); fireEvent.pointerUp(node, {pointerId: 1});
  expect(changed).toHaveBeenCalledOnce(); expect(changed.mock.calls[0][0].scene.frames[0].nodes[0]).toMatchObject({x: 64, y: 44});
  fireEvent.pointerDown(node, {pointerId: 2, button: 0, clientX: 0, clientY: 0}); fireEvent.pointerMove(node, {pointerId: 2, clientX: 10, clientY: 10}); fireEvent.pointerCancel(node, {pointerId: 2});
  expect(changed).toHaveBeenCalledOnce(); fireEvent.click(screen.getByRole('button', {name: 'Undo scene edit'})); expect(changed.mock.calls[1][0].scene.frames[0].nodes[0]).toMatchObject({x: 24, y: 24});
});
it('supports keyboard move/resize, integer property edits, lock and narrow pane switching', () => {
  const changed = vi.fn(); render(<Harness changed={changed} />); const node = screen.getByRole('button', {name: 'Select Page title (node-title)'});
  node.focus(); fireEvent.keyDown(node, {key: 'ArrowRight', shiftKey: true}); fireEvent.keyUp(node, {key: 'ArrowRight'});
  expect(changed.mock.calls[changed.mock.calls.length - 1]?.[0].scene.frames[0].nodes[0].x).toBe(34);
  fireEvent.keyDown(node, {key: 'ArrowDown', altKey: true}); fireEvent.keyUp(node, {key: 'ArrowDown'}); expect(changed.mock.calls[changed.mock.calls.length - 1]?.[0].scene.frames[0].nodes[0].height).toBe(49);
  fireEvent.click(screen.getByRole('button', {name: 'Properties', hidden: true}));
  fireEvent.change(screen.getByLabelText('Opacity (%)'), {target: {value: '50'}}); fireEvent.blur(screen.getByLabelText('Opacity (%)'));
  expect(changed.mock.calls[changed.mock.calls.length - 1]?.[0].scene.frames[0].nodes[0].style.opacity).toBe(50);
  fireEvent.click(screen.getByLabelText('Lock component')); const count = changed.mock.calls.length; fireEvent.keyDown(node, {key: 'ArrowRight'}); expect(changed).toHaveBeenCalledTimes(count);
});
it('read-only selection and frame preview never mutate or execute; Ask only hands off an unsent normal message', () => {
  const changed = vi.fn(), ask = vi.fn(); render(<Harness edit={false} changed={changed} ask={ask} />);
  const node = screen.getByRole('button', {name: 'Select Page title (node-title)'}); fireEvent.click(node); fireEvent.keyDown(node, {key: 'Delete'});
  fireEvent.click(screen.getByRole('button', {name: 'Try frame interactions'})); expect(changed).not.toHaveBeenCalled(); expect(ask).not.toHaveBeenCalled();
});
it('resets draft undo on a new saved revision while retaining a validated scene selection', () => {
  const plan = designScenePlanFixture(), changed = vi.fn(), props = {plan, canRead: true, canEdit: true, busy: false, workScope: null, onChange: changed, onAsk: vi.fn(), onPlan: vi.fn()};
  const page = render(<DesignSceneCanvas {...props} />); fireEvent.click(screen.getByRole('button', {name: 'Add button'})); expect(changed).toHaveBeenCalledOnce();
  page.rerender(<DesignSceneCanvas {...props} plan={{...plan, revision: 3, scene: changed.mock.calls[0][0]}} />);
  expect(screen.getByRole('button', {name: 'Undo scene edit'})).toHaveProperty('disabled', true);
});
