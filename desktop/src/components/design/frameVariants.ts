import type { DesignPlan, DesignScreen } from '../../api/designPlan';
import { constrainDesignNode, designNodeLabel, designSceneErrors, newDesignId, type EditableDesignScene } from '../../api/designScene';

export interface FrameCoverage { state: string; width: number; frameIds: string[] }

/** Presence in the draft only; neither layout quality nor runtime verification. */
export function plannedFrameCoverage(scene: EditableDesignScene, screen: DesignScreen): FrameCoverage[] {
  if (screen.widths.length > 12 || screen.states.length > 30) return [];
  const widths = [...new Set(screen.widths.filter(width => Number.isSafeInteger(width) && width >= 320 && width <= 3840))];
  const states = screen.states.length ? [...new Set(screen.states.filter(state => state === '' || state.trim()))] : [''];
  return states.flatMap(state => widths.map(width => ({ state, width,
    frameIds: scene.frames.filter(frame => frame.screenId === screen.id && frame.state === state && frame.width === width).map(frame => frame.id),
  })));
}

/** Explicitly copy one frame. Horizontal positions/sizes scale to the chosen
 * width; typography, vertical layout and asset bindings stay editable and intact.
 * This is a starting point to adapt, not an automatic responsive layout engine. */
export function createFrameVariant(plan: DesignPlan, sourceId: string, width: number, state: string,
  makeId: (prefix: string) => string = newDesignId): EditableDesignScene {
  const scene = plan.scene, source = scene?.frames.find(frame => frame.id === sourceId);
  const screen = plan.screens.find(row => row.id === source?.screenId);
  if (!scene || !source || !screen || !Number.isSafeInteger(width) || width < 320 || width > 3840 ||
      screen.widths.length > 12 || screen.states.length > 30 || !screen.widths.includes(width) ||
      !(screen.states.length ? screen.states.includes(state) : state === '')) {
    throw new Error('Choose an existing frame and a width and state from its screen plan.');
  }
  if (scene.frames.some(frame => frame.screenId === screen.id && frame.width === width && frame.state === state)) {
    throw new Error('A frame already represents that width and state. Open it instead of creating a duplicate.');
  }
  const used = new Set(scene.frames.flatMap(frame => [frame.id, ...frame.nodes.map(node => node.id)]));
  const allocate = (prefix: string) => {
    for (let attempt = 0; attempt < 8; attempt++) {
      const next = makeId(prefix);
      if (/^[a-zA-Z0-9_-]{1,80}$/.test(next) && !used.has(next)) { used.add(next); return next; }
    }
    throw new Error('A unique frame identity could not be created. Your existing scene is unchanged.');
  };
  const frameId = allocate('frame');
  const copy = { ...source, id: frameId, width,
    name: designNodeLabel(`${source.name} · ${width} px${state ? ` · ${state}` : ''}`), state,
    nodes: source.nodes.map(node => constrainDesignNode({ ...node, id: allocate('node'),
      x: Math.round(node.x * width / source.width), width: Math.max(1, Math.round(node.width * width / source.width)),
      style: { ...node.style }, behavior: { ...node.behavior,
        targetFrameId: node.behavior.targetFrameId === source.id ? frameId : node.behavior.targetFrameId,
      },
    }, { ...source, width })),
  };
  const next: EditableDesignScene = { ...scene, frames: [...scene.frames, copy] };
  const problems = designSceneErrors(next, plan);
  if (problems.length) throw new Error(problems[0]);
  return next;
}
