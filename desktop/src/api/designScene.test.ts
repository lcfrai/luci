import { expect, it } from 'vitest';
import { designScenePlanFixture } from '../test/designSceneFixtures';
import { constrainDesignNode, designSceneErrors, editableDesignSceneValid, designRasterPinValid } from './designScene';
it('validates the shared integer wire document and scalar/reference limits', () => {
  const plan = designScenePlanFixture(); expect(designSceneErrors(plan.scene, plan)).toEqual([]);
  const node = plan.scene!.frames[0].nodes[0]; node.style.opacity = 0.5; expect(editableDesignSceneValid(plan.scene)).toBe(false);
  node.style.opacity = 100; node.x = Number.NaN; expect(editableDesignSceneValid(plan.scene)).toBe(false);
  node.x = 24; node.style.fill = 'url(javascript:alert(1))'; expect(editableDesignSceneValid(plan.scene)).toBe(false);
  node.style.fill = 'transparent'; plan.scene!.frames[0].state = 'removed'; expect(designSceneErrors(plan.scene, plan)).not.toEqual([]);
});
it('keeps raster bytes strictly below the bounded Git stdout limit', () => {
  const pin = {sourceCommit:'a'.repeat(40),gitBlob:'b'.repeat(40),digest:'c'.repeat(64),mimeType:'image/png',bytes:2*1024*1024-1,width:4096,height:4096};
  expect(designRasterPinValid(pin)).toBe(true); expect(designRasterPinValid({...pin,bytes:2*1024*1024})).toBe(false);
  expect(designRasterPinValid({...pin,mimeType:'image/svg+xml'})).toBe(false); expect(designRasterPinValid({...pin,width:4097})).toBe(false);
});
it('rejects extra authority fields, duplicate IDs, unbound assets and dangling navigation', () => {
  let plan = designScenePlanFixture(); expect(editableDesignSceneValid({...plan.scene, execute: 'command'})).toBe(false);
  plan.scene!.frames[0].nodes[0].id = 'frame-home'; expect(editableDesignSceneValid(plan.scene)).toBe(false);
  plan = designScenePlanFixture(); plan.scene!.frames[0].nodes[0].behavior = {kind: 'navigate', targetFrameId: 'https://example.invalid'}; expect(editableDesignSceneValid(plan.scene)).toBe(false);
  plan = designScenePlanFixture(); const node = plan.scene!.frames[0].nodes[0]; node.kind = 'image'; node.assetId = 'asset-image'; expect(designSceneErrors(plan.scene, plan)).not.toEqual([]);
});
it('retains plain text without executable interpretation and bounds UTF-8 bytes', () => {
  const plan = designScenePlanFixture(), node = plan.scene!.frames[0].nodes[0]; node.text = '<script>not an instruction</script>';
  expect(editableDesignSceneValid(plan.scene)).toBe(true); node.text = 'é'.repeat(2001); expect(editableDesignSceneValid(plan.scene)).toBe(false);
});
it('rounds and clamps pointer geometry to the exact frame without negative or fractional output', () => {
  const frame = designScenePlanFixture().scene!.frames[0], node = frame.nodes[0];
  const next = constrainDesignNode({...node, x: 1000.5, y: -10, width: 450.1, height: 20.8}, frame);
  expect(next.x).toBe(0); expect(next.y).toBe(0); expect(next.width).toBe(390); expect(next.height).toBe(21);
});
