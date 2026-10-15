import type { DesignPlan } from '../api/designPlan';
import { newDesignFrame, newDesignNode } from '../api/designScene';
export function designScenePlanFixture(): DesignPlan {
  const screen = {id: 'screen-home', name: 'Home', route: '/', purpose: 'Save work', states: ['Empty', 'Saved'], widths: [390], requirementIds: [], accessibility: 'Keyboard'};
  const frame = {...newDesignFrame(screen), id: 'frame-home', name: 'Home phone', state: 'Empty'};
  frame.nodes = [{...newDesignNode('text', frame), id: 'node-title', name: 'Page title', text: 'Saved work'}];
  return {projectId: 'p-1', revision: 2, briefRevision: 1, specificationRevision: 2, sketchRevision: 0, screens: [screen], tokens: [], assets: [], implementationNotes: '', prototypeReleaseId: '', prototypeCommit: '', status: 'draft', digest: '', updatedAt: '', scene: {version: 1, frames: [frame]}};
}
