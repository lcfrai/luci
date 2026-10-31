import { expect, it } from 'vitest';
import { designScenePlanFixture } from '../../test/designSceneFixtures';
import { designSceneErrors } from '../../api/designScene';
import { createFrameVariant, plannedFrameCoverage } from './frameVariants';

const ids = () => { let value = 0; return (prefix: string) => `${prefix}-variant-${++value}`; };

it('copies and adapts a planned width with unique identities while preserving the original, assets and interaction meaning', () => {
  const plan = designScenePlanFixture(), source = plan.scene!.frames[0];
  plan.screens[0].widths.push(1440);
  const external = { ...source, id: 'frame-external', state: 'Saved', nodes: [] };
  source.nodes[0].behavior = { kind: 'navigate', targetFrameId: source.id };
  source.nodes.push({ ...source.nodes[0], id: 'node-external-link', behavior: { kind: 'navigate', targetFrameId: external.id } });
  const image = { id: 'hero', path: 'hero.png', purpose: 'Owned illustration', licence: 'Owned', content: { sourceCommit: 'a'.repeat(40), gitBlob: 'b'.repeat(40), digest: 'c'.repeat(64), mimeType: 'image/png' as const, bytes: 12, width: 200, height: 100 } };
  plan.assets.push(image);
  source.nodes.push({ ...source.nodes[0], id: 'node-image', kind: 'image', assetId: 'hero', behavior: { kind: 'none', targetFrameId: '' } });
  plan.scene!.frames.push(external);
  const before = JSON.stringify(plan), next = createFrameVariant(plan, source.id, 1440, 'Empty', ids());
  const copy = next.frames[2], allIds = next.frames.flatMap(frame => [frame.id, ...frame.nodes.map(node => node.id)]);
  expect(new Set(allIds).size).toBe(allIds.length);
  expect(JSON.stringify(plan)).toBe(before);
  expect(next.frames[0]).toEqual(source); expect(next.frames[1]).toEqual(external);
  expect(copy.nodes[0].behavior.targetFrameId).toBe(copy.id);
  expect(copy.nodes[1].behavior.targetFrameId).toBe(external.id);
  expect(copy.nodes[2].assetId).toBe('hero');
  expect(copy.nodes[0].style).toEqual(source.nodes[0].style);
  expect(copy.nodes[0].style).not.toBe(source.nodes[0].style);
  expect(copy.nodes[0].y).toBe(source.nodes[0].y);
  expect(copy.nodes[0].width).toBe(Math.round(source.nodes[0].width * 1440 / 390));
  expect(designSceneErrors(next, plan)).toEqual([]);
});

it('keeps narrow copies at integer nonoverflow geometry without scaling text into unreadable sizes', () => {
  const plan = designScenePlanFixture(), source = plan.scene!.frames[0];
  plan.screens[0].widths = [320, 390];
  source.nodes[0] = { ...source.nodes[0], x: 389, width: 1 };
  const copy = createFrameVariant(plan, source.id, 320, 'Empty', ids()).frames[1];
  for (const node of copy.nodes) {
    expect([node.x, node.y, node.width, node.height].every(Number.isSafeInteger)).toBe(true);
    expect(node.x + node.width).toBeLessThanOrEqual(copy.width);
    expect(node.y + node.height).toBeLessThanOrEqual(copy.height);
  }
  expect(copy.nodes[0].style.fontSize).toBe(source.nodes[0].style.fontSize);
});

it('refuses duplicate combinations, unavailable targets, exhausted identity generation and scene limits without changing the source', () => {
  const plan = designScenePlanFixture(), source = plan.scene!.frames[0], before = JSON.stringify(plan);
  plan.screens[0].widths.push(1440);
  expect(() => createFrameVariant(plan, source.id, 390, 'Empty', ids())).toThrow(/already represents/);
  expect(() => createFrameVariant(plan, source.id, 768, 'Empty', ids())).toThrow(/screen plan/);
  expect(() => createFrameVariant(plan, source.id, 1440, 'Removed', ids())).toThrow(/screen plan/);
  expect(() => createFrameVariant(plan, source.id, 1440, 'Empty', () => source.id)).toThrow(/unique frame identity/);
  plan.screens[0].widths.pop();
  expect(JSON.stringify(plan)).toBe(before);
  plan.screens[0].widths.push(1440);
  plan.scene!.frames.push(...Array.from({ length: 99 }, (_, index) => ({ ...source, id: `frame-extra-${index}`, state: 'Saved', nodes: [] })));
  const full = JSON.stringify(plan);
  expect(() => createFrameVariant(plan, source.id, 1440, 'Empty', ids())).toThrow(/100 frames/);
  expect(JSON.stringify(plan)).toBe(full);
});

it('reports planned width and state presence while keeping missing combinations navigable and separate from runtime proof', () => {
  const plan = designScenePlanFixture(); plan.screens[0].widths.push(1440);
  const rows = plannedFrameCoverage(plan.scene!, plan.screens[0]);
  expect(rows).toEqual([
    { state: 'Empty', width: 390, frameIds: ['frame-home'] }, { state: 'Empty', width: 1440, frameIds: [] },
    { state: 'Saved', width: 390, frameIds: [] }, { state: 'Saved', width: 1440, frameIds: [] },
  ]);
});
it('requires an explicit planned state and only offers an unspecified state for a plan without states', () => {
  const plan = designScenePlanFixture(); plan.screens[0].widths.push(1440);
  expect(() => createFrameVariant(plan, 'frame-home', 1440, '', ids())).toThrow(/screen plan/);
  plan.screens[0].states = []; plan.scene!.frames[0].state = '';
  expect(createFrameVariant(plan, 'frame-home', 1440, '', ids()).frames[1].state).toBe('');
  expect(plannedFrameCoverage(plan.scene!, plan.screens[0]).map(row => row.state)).toEqual(['', '']);
});
