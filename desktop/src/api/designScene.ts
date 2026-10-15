import type { DesignPlan } from './designPlan';

export const DESIGN_SCENE_BYTES = 128 * 1024;
export const DESIGN_NODE_KINDS = ['rectangle', 'text', 'button', 'input', 'image', 'divider'] as const;
export type DesignNodeKind = typeof DESIGN_NODE_KINDS[number];
export interface DesignNodeStyle {
  fill: string; stroke: string; strokeWidth: number; radius: number; color: string; fontSize: number;
  fontWeight: number; fontFamily: 'system' | 'serif' | 'mono'; textAlign: 'left' | 'center' | 'right'; padding: number; opacity: number;
}
export interface DesignSceneNode {
  id: string; kind: DesignNodeKind; name: string; x: number; y: number; width: number; height: number;
  text: string; assetId: string; hidden: boolean; locked: boolean; style: DesignNodeStyle;
  behavior: {kind: 'none' | 'navigate'; targetFrameId: string};
}
export interface DesignSceneFrame {
  id: string; name: string; screenId: string; state: string; width: number; height: number; background: string; nodes: DesignSceneNode[];
}
export interface EditableDesignScene { version: 1; frames: DesignSceneFrame[] }
export interface DesignRasterPin {
  sourceCommit: string; gitBlob: string; digest: string; mimeType: 'image/png' | 'image/jpeg'; bytes: number; width: number; height: number;
}
const fields = (value: object, names: string[]) => Object.keys(value).every(name => names.includes(name));
const integer = (value: unknown, min: number, max: number): value is number => Number.isSafeInteger(value) && (value as number) >= min && (value as number) <= max;
const text = (value: unknown, max: number): value is string => typeof value === 'string' && new TextEncoder().encode(value).length <= max && !value.includes('\0');
const id = (value: unknown): value is string => typeof value === 'string' && /^[a-zA-Z0-9_-]{1,80}$/.test(value);
const color = (value: unknown): value is string => typeof value === 'string' && /^(#[a-fA-F0-9]{6}|transparent)$/.test(value);
export function designRasterPinValid(pin: unknown): pin is DesignRasterPin {
  if (!pin || typeof pin !== 'object' || Array.isArray(pin)) return false;
  const value = pin as DesignRasterPin;
  return fields(value, ['sourceCommit', 'gitBlob', 'digest', 'mimeType', 'bytes', 'width', 'height']) &&
    /^[a-f0-9]{40}$/.test(value.sourceCommit) && /^[a-f0-9]{40}$/.test(value.gitBlob) && /^[a-f0-9]{64}$/.test(value.digest) &&
    ['image/png', 'image/jpeg'].includes(value.mimeType) && integer(value.bytes, 1, 8 * 1024 * 1024) &&
    integer(value.width, 1, 4096) && integer(value.height, 1, 4096) && value.width * value.height <= 16 * 1024 * 1024;
}
/** Shared document validation; only approved scalar render properties cross the canvas boundary. */
export function designSceneErrors(scene: unknown, plan?: Pick<DesignPlan, 'screens' | 'assets'>): string[] {
  const invalid = (message: string) => [message];
  if (!scene || typeof scene !== 'object' || Array.isArray(scene)) return invalid('The design scene is not a versioned document.');
  const value = scene as EditableDesignScene;
  if (!fields(value, ['version', 'frames']) || value.version !== 1 || !Array.isArray(value.frames) || value.frames.length > 100 || new TextEncoder().encode(JSON.stringify(value)).length > DESIGN_SCENE_BYTES) return invalid('Use a version 1 scene with at most 100 frames, within 128 KiB.');
  const ids = new Set<string>(), frameIds = new Set(value.frames.map(frame => frame?.id));
  let count = 0;
  for (const frame of value.frames) {
    if (!frame || !fields(frame, ['id', 'name', 'screenId', 'state', 'width', 'height', 'background', 'nodes']) || !id(frame.id) || ids.has(frame.id) || !text(frame.name, 200) || !frame.name.trim() || !id(frame.screenId) || !text(frame.state, 2000) ||
        !integer(frame.width, 320, 3840) || !integer(frame.height, 1, 8192) || !color(frame.background) || !Array.isArray(frame.nodes) || frame.nodes.length > 200) return invalid('Frames need unique IDs, a screen, valid integer dimensions and at most 200 components.');
    const screen = plan?.screens.find(item => item.id === frame.screenId);
    if (plan && (!screen || frame.state !== '' && !screen.states.includes(frame.state))) return invalid(`Frame ${frame.name} references a removed screen or state. Update the scene and screen plan together.`);
    ids.add(frame.id); count += frame.nodes.length;
    for (const node of frame.nodes) {
      if (!node || !fields(node, ['id', 'kind', 'name', 'x', 'y', 'width', 'height', 'text', 'assetId', 'hidden', 'locked', 'style', 'behavior']) || !id(node.id) || ids.has(node.id) || !DESIGN_NODE_KINDS.includes(node.kind) || !text(node.name, 200) || !node.name.trim() ||
          !integer(node.x, 0, frame.width) || !integer(node.y, 0, frame.height) || !integer(node.width, 1, frame.width) || !integer(node.height, 1, frame.height) || node.x + node.width > frame.width || node.y + node.height > frame.height ||
          !text(node.text, 4000) || !text(node.assetId, 80) || typeof node.hidden !== 'boolean' || typeof node.locked !== 'boolean') return invalid('Components need unique IDs, plain text and integer geometry within their frame.');
      ids.add(node.id);
      const style = node.style;
      if (!style || !fields(style, ['fill', 'stroke', 'strokeWidth', 'radius', 'color', 'fontSize', 'fontWeight', 'fontFamily', 'textAlign', 'padding', 'opacity']) ||
          !color(style.fill) || !color(style.stroke) || !color(style.color) || !integer(style.strokeWidth, 0, 16) || !integer(style.radius, 0, 64) || !integer(style.fontSize, 8, 96) ||
          ![400, 500, 600, 700].includes(style.fontWeight) || !['system', 'serif', 'mono'].includes(style.fontFamily) || !['left', 'center', 'right'].includes(style.textAlign) || !integer(style.padding, 0, 128) || !integer(style.opacity, 0, 100)) return invalid('Component properties must use supported colours and bounded integer values.');
      if (!node.behavior || !fields(node.behavior, ['kind', 'targetFrameId']) || !['none', 'navigate'].includes(node.behavior.kind) ||
          node.behavior.kind === 'none' && node.behavior.targetFrameId !== '' || node.behavior.kind === 'navigate' && !frameIds.has(node.behavior.targetFrameId)) return invalid('A design interaction must refer to a saved frame in this scene.');
      if (node.kind !== 'image' && node.assetId !== '' || node.kind === 'image' && !id(node.assetId)) return invalid('Image components need a pinned design asset; other components cannot load assets.');
      if (plan && node.kind === 'image' && !designRasterPinValid(plan.assets.find(asset => asset.id === node.assetId)?.content)) return invalid('This image has no immutable raster binding. Pin the asset before using it.');
    }
  }
  return count > 1000 ? invalid('Use at most 1,000 components across this scene.') : [];
}
export const editableDesignSceneValid = (value: unknown): value is EditableDesignScene => designSceneErrors(value).length === 0;
export const newDesignId = (prefix: string) => `${prefix}-${crypto.randomUUID().slice(0, 12)}`;
export function newDesignFrame(screen: DesignPlan['screens'][number]): DesignSceneFrame {
  return {id: newDesignId('frame'), name: screen.name || 'Untitled screen', screenId: screen.id, state: '', width: screen.widths[0] || 390, height: 844, background: '#ffffff', nodes: []};
}
export function newDesignNode(kind: Exclude<DesignNodeKind, 'image'>, frame: DesignSceneFrame): DesignSceneNode {
  const width = Math.min(kind === 'text' ? 240 : 160, frame.width), height = Math.min(kind === 'divider' ? 2 : kind === 'rectangle' ? 120 : 48, frame.height);
  return {id: newDesignId('node'), kind, name: kind === 'text' ? 'Text' : kind[0].toUpperCase() + kind.slice(1), x: Math.min(24, frame.width - width), y: Math.min(24, frame.height - height), width, height,
    text: kind === 'text' ? 'Edit this text' : kind === 'button' ? 'Button' : kind === 'input' ? 'Input label' : '', assetId: '', hidden: false, locked: false,
    style: {fill: kind === 'button' ? '#18181b' : kind === 'rectangle' || kind === 'divider' ? '#e4e4e7' : 'transparent', stroke: kind === 'input' ? '#a1a1aa' : 'transparent', strokeWidth: kind === 'input' ? 1 : 0, radius: kind === 'button' || kind === 'input' ? 8 : 0,
      color: kind === 'button' ? '#ffffff' : '#18181b', fontSize: kind === 'text' ? 24 : 16, fontWeight: 400, fontFamily: 'system', textAlign: 'left', padding: kind === 'button' || kind === 'input' ? 12 : 0, opacity: 100}, behavior: {kind: 'none', targetFrameId: ''}};
}
export function constrainDesignNode(node: DesignSceneNode, frame: DesignSceneFrame): DesignSceneNode {
  const width = Math.max(1, Math.min(frame.width, Math.round(node.width))), height = Math.max(1, Math.min(frame.height, Math.round(node.height)));
  return {...node, width, height, x: Math.max(0, Math.min(frame.width - width, Math.round(node.x))), y: Math.max(0, Math.min(frame.height - height, Math.round(node.y)))};
}
