import type { EditableDesignScene } from '../../api/designScene';
export interface SceneHistory { past: EditableDesignScene[]; current: EditableDesignScene; future: EditableDesignScene[]; group: string }
export const sceneHistory = (current: EditableDesignScene): SceneHistory => ({past: [], current, future: [], group: ''});
const MAX_HISTORY_BYTES = 4 * 1024 * 1024;
function bound(values: EditableDesignScene[]) {
  const kept = values.slice(-50); let bytes = kept.reduce((total, scene) => total + JSON.stringify(scene).length * 2, 0);
  while (kept.length > 1 && bytes > MAX_HISTORY_BYTES) bytes -= JSON.stringify(kept.shift()).length * 2;
  return kept;
}
export function editScene(history: SceneHistory, current: EditableDesignScene, group = ''): SceneHistory {
  if (JSON.stringify(current) === JSON.stringify(history.current)) return history;
  return {past: group && group === history.group ? history.past : bound([...history.past, history.current]), current, future: [], group};
}
export function undoScene(history: SceneHistory): SceneHistory {
  if (!history.past.length) return history;
  return {past: history.past.slice(0, -1), current: history.past[history.past.length - 1], future: bound([...history.future, history.current]), group: ''};
}
export function redoScene(history: SceneHistory): SceneHistory {
  if (!history.future.length) return history;
  return {past: bound([...history.past, history.current]), current: history.future[history.future.length - 1], future: history.future.slice(0, -1), group: ''};
}
