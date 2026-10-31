import { expect, it } from 'vitest';
import { designScenePlanFixture } from '../test/designSceneFixtures';
import { designSceneErrors } from './designScene';
import { captureComponent, componentLibraryErrors, instantiateComponent, parseComponentPackage, prepareComponentImport, resolveComponentStyle } from './designComponents';

it('captures a plain component and instantiates a fresh bounded copy with explicit token resolution', () => {
  const plan = designScenePlanFixture(), frame = plan.scene!.frames[0], original = frame.nodes[0];
  const before = JSON.stringify(original), item = captureComponent(original);
  item.bindings = [{property: 'color', token: 'ink'}, {property: 'fontSize', token: 'body'}];
  plan.tokens = [{name: 'ink', value: '#123456', purpose: 'Text'}, {name: 'body', value: '18', purpose: 'Text size'}];
  plan.componentLibrary = {version: 1, items: [item]};
  const copy = instantiateComponent(item, {...frame, width: 320}, plan.tokens);
  expect(copy.id).not.toBe(original.id); expect(copy.componentId).toBe(item.id);
  expect(copy.style.color).toBe('#123456'); expect(copy.style.fontSize).toBe(18);
  expect(copy.x + copy.width).toBeLessThanOrEqual(320);
  expect(copy.behavior).toEqual({kind: 'none', targetFrameId: ''});
  expect(JSON.stringify(original)).toBe(before);
  expect(designSceneErrors({version: 1, frames: [{...frame, nodes: [copy]}]}, plan)).toEqual([]);
  expect(designSceneErrors({version: 1, frames: [{...frame, nodes: [copy]}]}, {...plan, componentLibrary: undefined})).not.toEqual([]);
});

it('renames conflicting imported tokens, rewrites bindings and leaves both input documents untouched', () => {
  const plan = designScenePlanFixture(), item = captureComponent(plan.scene!.frames[0].nodes[0]);
  plan.tokens = [{name: 'ink', value: '#000000', purpose: 'Existing ink'}];
  item.bindings = [{property: 'color', token: 'ink'}];
  const incoming = {format: 'luci-design-components' as const, version: 1 as const, components: {version: 1 as const, items: [item]}, tokens: [{name: 'ink', value: '#ffffff', purpose: 'Imported ink'}]};
  const before = JSON.stringify({plan, incoming}), next = prepareComponentImport(plan, incoming);
  expect(next.tokens[0]).toEqual(plan.tokens[0]); expect(next.renamed).toEqual(['ink → ink-import-2']);
  expect(next.componentLibrary.items[0].id).not.toBe(item.id);
  expect(resolveComponentStyle(next.componentLibrary.items[0], next.tokens).color).toBe('#ffffff');
  expect(JSON.stringify({plan, incoming})).toBe(before);
});

it('rejects executable properties, external asset components and malformed style tokens before import', () => {
  const plan = designScenePlanFixture(), item = captureComponent(plan.scene!.frames[0].nodes[0]);
  const packet = {format: 'luci-design-components', version: 1, components: {version: 1, items: [item]}, tokens: []};
  expect(() => parseComponentPackage(JSON.stringify({...packet, script: 'alert(1)'}))).toThrow();
  expect(() => parseComponentPackage(JSON.stringify({...packet, components: {version: 1, items: [{...item, kind: 'image', assetId: 'remote'}]}}))).toThrow();
  expect(componentLibraryErrors({version: 1, items: [{...item, bindings: [{property: 'fill', token: 'background'}]}]}, [{name: 'background', value: 'url(https://invalid.test)', purpose: ''}])).not.toEqual([]);
  expect(componentLibraryErrors({version: 1, items: [{...item, bindings: [{property: 'fontSize', token: 'body'}]}]}, [{name: 'body', value: '0', purpose: ''}])).not.toEqual([]);
});

it('retains legacy omission and rejects oversized merged libraries without changing the current design', () => {
  expect(componentLibraryErrors(undefined, [])).toEqual([]);
  const plan = designScenePlanFixture(), item = captureComponent(plan.scene!.frames[0].nodes[0]);
  plan.componentLibrary = {version: 1, items: Array.from({length: 64}, (_, i) => ({...item, id: `component-${i}`, name: `Component ${i}`}))};
  const before = JSON.stringify(plan);
  expect(() => prepareComponentImport(plan, {format: 'luci-design-components', version: 1, components: {version: 1, items: [item]}, tokens: []})).toThrow();
  expect(JSON.stringify(plan)).toBe(before);
});
