import { expect, it } from 'vitest';
import { designScenePlanFixture } from '../../test/designSceneFixtures';
import { editScene, redoScene, sceneHistory, undoScene } from './designSceneHistory';
it('coalesces one content/gesture edit and preserves exact undo/redo without mutating its base', () => {
  const original = designScenePlanFixture().scene!, changed = structuredClone(original); changed.frames[0].nodes[0].text = 'One';
  let history = editScene(sceneHistory(original), changed, 'text'); const next = structuredClone(changed); next.frames[0].nodes[0].text = 'Two'; history = editScene(history, next, 'text');
  expect(history.past).toHaveLength(1); expect(undoScene(history).current).toEqual(original); expect(redoScene(undoScene(history)).current).toEqual(next);
  expect(original.frames[0].nodes[0].text).toBe('Saved work');
});
it('bounds retained snapshots and clears redo on a new edit', () => {
  let history = sceneHistory(designScenePlanFixture().scene!);
  for (let index = 0; index < 100; index++) { const next = structuredClone(history.current); next.frames[0].nodes[0].text = String(index); history = editScene(history, next); }
  expect(history.past.length).toBeLessThanOrEqual(50);
  const undone = undoScene(history), next = structuredClone(undone.current); next.frames[0].nodes[0].text = 'New branch'; expect(editScene(undone, next).future).toEqual([]);
});
