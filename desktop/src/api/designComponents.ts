import type { DesignPlan } from './designPlan';
import { designSceneErrors, newDesignId, type DesignNodeStyle, type DesignSceneFrame, type DesignSceneNode } from './designScene';

export const componentTokenProperties = ['fill', 'stroke', 'color', 'fontSize', 'radius', 'padding', 'strokeWidth'] as const;
export type ComponentTokenProperty = typeof componentTokenProperties[number];
export interface DesignComponentTemplate {
  id: string; name: string; description: string; kind: Exclude<DesignSceneNode['kind'], 'image'>;
  width: number; height: number; text: string; style: DesignNodeStyle;
  bindings: { property: ComponentTokenProperty; token: string }[];
}
export interface DesignComponentLibrary { version: 1; items: DesignComponentTemplate[] }
export interface DesignComponentPackage {
  format: 'luci-design-components'; version: 1;
  components: DesignComponentLibrary; tokens: DesignPlan['tokens'];
}
const record = (value: unknown): value is Record<string, unknown> => !!value && typeof value === 'object' && !Array.isArray(value);
const fields = (value: Record<string, unknown>, names: string[]) => Object.keys(value).every(name => names.includes(name));
const text = (value: unknown, limit: number): value is string => typeof value === 'string' && !value.includes('\0') && new TextEncoder().encode(value).length <= limit;
const identifier = (value: unknown): value is string => typeof value === 'string' && /^[a-zA-Z0-9_-]{1,80}$/.test(value);
const bytes = (value: unknown) => new TextEncoder().encode(JSON.stringify(value)).length;
const emptyLibrary = (): DesignComponentLibrary => ({ version: 1, items: [] });

export function resolveComponentStyle(item: DesignComponentTemplate, tokens: DesignPlan['tokens']): DesignNodeStyle {
  const values = new Map(tokens.map(token => [token.name, token.value]));
  const used = new Set<string>(), style = { ...item.style };
  for (const binding of item.bindings) {
    const value = values.get(binding.token);
    if (value === undefined || used.has(binding.property) || !componentTokenProperties.includes(binding.property)) throw new Error('Choose an existing token once per component property.');
    used.add(binding.property);
    if (binding.property === 'fill' || binding.property === 'stroke' || binding.property === 'color') {
      if (!/^(#[a-fA-F0-9]{6}|transparent)$/.test(value)) throw new Error('Colour tokens need six-digit hex colours or transparent.');
      style[binding.property] = value;
    } else {
      if (!/^(0|[1-9][0-9]*)$/.test(value) || !Number.isSafeInteger(Number(value))) throw new Error('Size tokens need whole numbers without units.');
      style[binding.property] = Number(value);
    }
  }
  return style;
}

export function componentLibraryErrors(value: unknown, tokens: DesignPlan['tokens']): string[] {
  if (value === undefined) return [];
  if (!record(value) || !fields(value, ['version', 'items']) || value.version !== 1 || !Array.isArray(value.items) || value.items.length > 64 || bytes(value) > 64 * 1024) return ['Use a version 1 library with at most 64 components within 64 KiB.'];
  const ids = new Set<string>(), names = new Set<string>();
  for (const item of value.items) {
    if (!record(item) || !fields(item, ['id', 'name', 'description', 'kind', 'width', 'height', 'text', 'style', 'bindings']) || !identifier(item.id) || ids.has(item.id) || !text(item.name, 200) || !item.name.trim() || names.has(item.name) || !text(item.description, 2000) || !text(item.text, 4000) || !['rectangle', 'text', 'button', 'input', 'divider'].includes(item.kind as string) || !Array.isArray(item.bindings) || item.bindings.length > 7 || !item.bindings.every(binding => record(binding) && fields(binding, ['property', 'token']) && componentTokenProperties.includes(binding.property as ComponentTokenProperty) && text(binding.token, 100))) return ['Components need unique IDs and names, supported shapes, text and bounded token bindings.'];
    ids.add(item.id); names.add(item.name);
    try {
      const template = item as unknown as DesignComponentTemplate;
      for (const style of [template.style, resolveComponentStyle(template, tokens)]) {
        const node: DesignSceneNode = {id: 'sample-node', kind: template.kind, name: template.name, x: 0, y: 0, width: template.width, height: template.height, text: template.text, style, hidden: false, locked: false, assetId: '', behavior: {kind: 'none', targetFrameId: ''}};
        const errors = designSceneErrors({version: 1, frames: [{id: 'sample-frame', name: 'Component', screenId: 'sample-screen', state: '', width: 3840, height: 8192, background: 'transparent', nodes: [node]}]});
        if (errors.length) return [`${template.name}: ${errors[0]}`];
      }
    } catch (cause) { return [cause instanceof Error ? cause.message : 'The component token bindings are invalid.']; }
  }
  return [];
}

export function captureComponent(node: DesignSceneNode, library: DesignComponentLibrary = emptyLibrary()): DesignComponentTemplate {
  if (node.kind === 'image') throw new Error('Reuse pinned images through Assets; component libraries contain shapes and text.');
  let name = node.name; let suffix = 2;
  while (library.items.some(item => item.name === name)) name = `${node.name.slice(0, 70)} ${suffix++}`;
  return {id: newDesignId('component'), name, description: '', kind: node.kind, width: node.width, height: node.height, text: node.text, style: {...node.style}, bindings: []};
}

export function instantiateComponent(item: DesignComponentTemplate, frame: DesignSceneFrame, tokens: DesignPlan['tokens']): DesignSceneNode {
  const errors = componentLibraryErrors({version: 1, items: [item]}, tokens); if (errors.length) throw new Error(errors[0]);
  const width = Math.min(item.width, frame.width), height = Math.min(item.height, frame.height);
  return {id: newDesignId('node'), componentId: item.id, kind: item.kind, name: item.name, x: Math.min(24, frame.width - width), y: Math.min(24, frame.height - height), width, height, text: item.text, style: resolveComponentStyle(item, tokens), hidden: false, locked: false, assetId: '', behavior: {kind: 'none', targetFrameId: ''}};
}

export function parseComponentPackage(raw: string): DesignComponentPackage {
  if (new TextEncoder().encode(raw).length > 128 * 1024) throw new Error('Choose a component package smaller than 128 KiB.');
  let value: unknown; try { value = JSON.parse(raw); } catch { throw new Error('This file is not a JSON component package.'); }
  if (!record(value) || !fields(value, ['format', 'version', 'components', 'tokens']) || value.format !== 'luci-design-components' || value.version !== 1 || !Array.isArray(value.tokens) || value.tokens.length > 200) throw new Error('Choose a version 1 Luci component package.');
  const names = new Set<string>();
  for (const token of value.tokens) {
    if (!record(token) || !fields(token, ['name', 'value', 'purpose']) || !text(token.name, 100) || !token.name.trim() || names.has(token.name) || !text(token.value, 2000) || !text(token.purpose, 2000)) throw new Error('Package tokens need unique names and bounded values.');
    names.add(token.name);
  }
  const errors = componentLibraryErrors(value.components, value.tokens as DesignPlan['tokens']);
  if (value.components === undefined || errors.length) throw new Error(errors[0] ?? 'The component package has no library.');
  return value as unknown as DesignComponentPackage;
}

export function prepareComponentImport(plan: Pick<DesignPlan, 'componentLibrary' | 'tokens'>, incoming: DesignComponentPackage) {
  // Imports append. Conflicting token names are renamed, never overwritten.
  const checked = parseComponentPackage(JSON.stringify(incoming));
  const tokens = plan.tokens.map(token => ({...token})), remap = new Map<string, string>(), renamed: string[] = [];
  for (const token of checked.tokens) {
    const same = tokens.find(item => item.name === token.name);
    if (same?.value === token.value) { remap.set(token.name, same.name); continue; }
    let name = token.name, suffix = 2;
    while (tokens.some(item => item.name === name)) name = `${token.name.slice(0, 70)}-import-${suffix++}`;
    if (name !== token.name) renamed.push(`${token.name} → ${name}`);
    remap.set(token.name, name); tokens.push({...token, name});
  }
  const items = (plan.componentLibrary?.items ?? []).map(item => structuredClone(item));
  for (const item of checked.components.items) {
    let name = item.name, suffix = 2;
    while (items.some(current => current.name === name)) name = `${item.name.slice(0, 70)} ${suffix++}`;
    items.push({...structuredClone(item), id: newDesignId('component'), name, bindings: item.bindings.map(binding => ({...binding, token: remap.get(binding.token)!}))});
  }
  const componentLibrary: DesignComponentLibrary = {version: 1, items};
  const errors = componentLibraryErrors(componentLibrary, tokens);
  if (tokens.length > 200 || errors.length) throw new Error(errors[0] ?? 'The combined design would exceed 200 tokens.');
  return {componentLibrary, tokens, renamed, addedComponents: checked.components.items.length, addedTokens: tokens.length - plan.tokens.length};
}
