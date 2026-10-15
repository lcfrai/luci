import { useEffect, useId, useRef, useState, type CSSProperties, type KeyboardEvent, type PointerEvent, type ReactNode } from 'react';
import { constrainDesignNode, designSceneErrors, newDesignFrame, newDesignId, newDesignNode, type DesignNodeKind, type DesignNodeStyle, type DesignSceneFrame, type DesignSceneNode, type EditableDesignScene } from '../../api/designScene';
import type { DesignPlan } from '../../api/designPlan';
import { Button } from '../fluid/components/button';
import { DesignWorkIndicator } from './DesignWorkIndicator';
import { editScene, redoScene, sceneHistory, undoScene } from './designSceneHistory';
import './design-scene.css';

const emptyScene = (): EditableDesignScene => ({version: 1, frames: []});
interface Props {
  plan: DesignPlan; canRead: boolean; canEdit: boolean; busy: boolean; workScope: string | null; active?: boolean; canAsk?: boolean;
  onChange(scene: EditableDesignScene): void; onAsk(message: string): void; onPlan(): void;
  renderImage?(assetId: string, alt: string): ReactNode;
}
type Gesture = {pointerId: number; frameId: string; node: DesignSceneNode; startX: number; startY: number; resize: boolean; scale: number; target: HTMLElement};
export function DesignSceneCanvas({plan, canRead, canEdit, busy, workScope, active = true, canAsk = true, onChange, onAsk, onPlan, renderImage}: Props) {
  const scene = plan.scene ?? emptyScene(), fingerprint = JSON.stringify(scene);
  const history = useRef(sceneHistory(scene)), [historyVersion, setHistoryVersion] = useState(0);
  const [frameId, setFrameId] = useState(''), [nodeId, setNodeId] = useState(''), [screenId, setScreenId] = useState('');
  const [scale, setScale] = useState(0.75), [pane, setPane] = useState<'canvas' | 'layers' | 'properties'>('canvas'), [preview, setPreview] = useState(false);
  const [gestureScene, setGestureScene] = useState<EditableDesignScene | null>(null), [notice, setNotice] = useState('');
  const gesture = useRef<Gesture | null>(null), stage = useRef<HTMLDivElement>(null), textInput = useRef<HTMLTextAreaElement>(null);
  const gesturePreview = useRef<EditableDesignScene | null>(null);
  const live = useRef({canRead, canEdit, busy, scene}); live.current = {canRead, canEdit, busy, scene};
  const nodeElements = useRef(new Map<string, HTMLDivElement>()), pendingFocus = useRef<string | null>(null);
  const sceneValue = gestureScene ?? scene, frame = sceneValue.frames.find(item => item.id === frameId) ?? sceneValue.frames[0];
  const node = frame?.nodes.find(item => item.id === nodeId), screen = plan.screens.find(item => item.id === frame?.screenId);
  const readOnly = !canEdit || busy || preview || !active, canModify = canRead && canEdit && !busy && active;
  const structuralErrors = designSceneErrors(scene), errors = structuralErrors.length ? structuralErrors : designSceneErrors(scene, plan);
  const editing = useRef(false); editing.current = canModify;
  useEffect(() => {
    if (fingerprint !== JSON.stringify(history.current.current)) { history.current = sceneHistory(scene); setHistoryVersion(value => value + 1); gesture.current = null; setGestureScene(null); }
  }, [fingerprint]);
  useEffect(() => { history.current = sceneHistory(scene); setHistoryVersion(value => value + 1); }, [plan.revision]);
  useEffect(() => { if (!canModify) { gesture.current = null; setGestureScene(null); } }, [canModify]);
  useEffect(() => {
    const id = pendingFocus.current;
    if (id && nodeElements.current.has(id)) { nodeElements.current.get(id)?.focus(); pendingFocus.current = null; }
  }, [fingerprint, pane]);
  function commit(next: EditableDesignScene, group = '') {
    if (!editing.current) return;
    const invalid = designSceneErrors(next);
    if (invalid.length) { setNotice(invalid[0]); return; }
    const current = JSON.stringify(history.current.current) === JSON.stringify(live.current.scene) ? history.current : sceneHistory(live.current.scene);
    history.current = editScene(current, next, group); setHistoryVersion(value => value + 1); onChange(next); setNotice('');
  }
  function endGroup() { history.current.group = ''; }
  function patchFrame(value: Partial<DesignSceneFrame>, group = '') {
    if (!frame) return;
    const next = {...frame, ...value};
    if (value.width !== undefined || value.height !== undefined) next.nodes = next.nodes.map(item => constrainDesignNode(item, next));
    commit({...scene, frames: scene.frames.map(item => item.id === frame.id ? next : item)}, group);
  }
  function patchNode(value: Partial<DesignSceneNode>, group = '') {
    if (!frame || !node || node.locked && value.locked !== false) return;
    const next = constrainDesignNode({...node, ...value}, frame);
    patchFrame({nodes: frame.nodes.map(item => item.id === node.id ? next : item)}, group);
  }
  function patchStyle(value: Partial<DesignNodeStyle>, group = '') { if (node) patchNode({style: {...node.style, ...value}}, group); }
  function moveHistory(redo: boolean) {
    if (!canModify || gesture.current) return;
    const next = redo ? redoScene(history.current) : undoScene(history.current);
    if (next !== history.current) { history.current = next; setHistoryVersion(value => value + 1); onChange(next.current); setNotice(redo ? 'Redid the draft scene edit.' : 'Undid the draft scene edit. Saved revisions stay unchanged.'); }
  }
  function add(kind: Exclude<DesignNodeKind, 'image'>) {
    if (!frame || !canModify) return;
    const value = newDesignNode(kind, frame); pendingFocus.current = value.id; setNodeId(value.id); setPreview(false); setPane('canvas'); patchFrame({nodes: [...frame.nodes, value]});
  }
  function start(event: PointerEvent<HTMLElement>, selected: DesignSceneNode, resize = false) {
    setNodeId(selected.id);
    if (!canModify || preview || selected.locked || !frame || event.button !== 0) return;
    event.preventDefault(); event.stopPropagation(); event.currentTarget.setPointerCapture?.(event.pointerId);
    gesture.current = {pointerId: event.pointerId, frameId: frame.id, node: selected, startX: event.clientX, startY: event.clientY, resize, scale, target: event.currentTarget};
    gesturePreview.current = null;
  }
  function move(event: PointerEvent<HTMLDivElement>) {
    const current = gesture.current;
    if (!current || event.pointerId !== current.pointerId || !editing.current) return;
    const savedFrame = scene.frames.find(item => item.id === current.frameId); if (!savedFrame) return;
    const dx = Math.round((event.clientX - current.startX) / current.scale), dy = Math.round((event.clientY - current.startY) / current.scale);
    const next = constrainDesignNode({...current.node, ...(current.resize ? {width: current.node.width + dx, height: current.node.height + dy} : {x: current.node.x + dx, y: current.node.y + dy})}, savedFrame);
    const nextScene = {...scene, frames: scene.frames.map(item => item.id === savedFrame.id ? {...item, nodes: item.nodes.map(value => value.id === next.id ? next : value)} : item)};
    gesturePreview.current = nextScene; setGestureScene(nextScene);
  }
  function finish(event: PointerEvent<HTMLDivElement>, cancel = false) {
    if (!gesture.current || gesture.current.pointerId !== event.pointerId) return;
    const target = gesture.current.target; gesture.current = null;
    if (target.hasPointerCapture?.(event.pointerId)) target.releasePointerCapture(event.pointerId);
    if (!cancel && gesturePreview.current) commit(gesturePreview.current);
    gesturePreview.current = null;
    setGestureScene(null);
  }
  function keys(event: KeyboardEvent<HTMLDivElement>, selected: DesignSceneNode) {
    if (event.key === 'Enter' || event.key === ' ') { event.preventDefault(); setNodeId(selected.id); if (preview && selected.behavior.kind === 'navigate') { setFrameId(selected.behavior.targetFrameId); setNodeId(''); } return; }
    if (!canModify || preview || selected.locked || !frame) return;
    if (event.key === 'Delete' || event.key === 'Backspace') { event.preventDefault(); patchFrame({nodes: frame.nodes.filter(item => item.id !== selected.id)}); setNodeId(''); return; }
    if (!['ArrowUp', 'ArrowDown', 'ArrowLeft', 'ArrowRight'].includes(event.key)) return;
    event.preventDefault(); const step = event.shiftKey ? 10 : 1, dx = event.key === 'ArrowLeft' ? -step : event.key === 'ArrowRight' ? step : 0, dy = event.key === 'ArrowUp' ? -step : event.key === 'ArrowDown' ? step : 0;
    const next = constrainDesignNode({...selected, ...(event.altKey ? {width: selected.width + dx, height: selected.height + dy} : {x: selected.x + dx, y: selected.y + dy})}, frame);
    patchFrame({nodes: frame.nodes.map(item => item.id === selected.id ? next : item)}, `${selected.id}:keyboard`);
  }
  function reorder(delta: number) {
    if (!node || !frame || node.locked) return;
    const values = [...frame.nodes], at = values.findIndex(item => item.id === node.id), target = Math.max(0, Math.min(values.length - 1, at + delta));
    values.splice(at, 1); values.splice(target, 0, node); patchFrame({nodes: values});
  }
  if (!canRead) return <p>Current project read access is required to view this scene.</p>;
  if (structuralErrors.length) return <div role="alert">The saved scene cannot be rendered safely: {structuralErrors[0]} Preserve its history and inspect the design plan before editing.</div>;
  return <section className="design-scene-editor" aria-label="Editable design canvas" onKeyDown={event => {
    if ((event.ctrlKey || event.metaKey) && event.key.toLowerCase() === 'z' && !(event.target instanceof HTMLInputElement || event.target instanceof HTMLTextAreaElement)) { event.preventDefault(); moveHistory(event.shiftKey); }
    if (event.key === 'Escape') { gesture.current = null; setGestureScene(null); setNodeId(''); }
  }}>
    <DesignWorkIndicator scope={workScope} active={active} />
    <div className="design-scene-toolbar">
      <label>Frame<select value={frame?.id ?? ''} onChange={event => { setFrameId(event.target.value); setNodeId(''); setGestureScene(null); gesture.current = null; }} disabled={!scene.frames.length}>
        {!scene.frames.length && <option value="">No scene frames yet</option>}{scene.frames.map(item => <option key={item.id} value={item.id}>{item.name} · {item.width} px{item.state ? ` · ${item.state}` : ''}</option>)}
      </select></label>
      <Button variant="secondary" disabled={!canModify || !plan.screens.length} onClick={() => {
        const chosen = plan.screens.find(item => item.id === screenId) ?? plan.screens[0]; if (!chosen) return;
        const created = newDesignFrame(chosen); commit({...scene, frames: [...scene.frames, created]}); setFrameId(created.id); setNodeId('');
      }}>Add frame</Button>
      <label>For screen<select value={screenId || plan.screens[0]?.id || ''} disabled={!canModify} onChange={event => setScreenId(event.target.value)}>{plan.screens.map(item => <option key={item.id} value={item.id}>{item.name || 'Untitled screen'}</option>)}</select></label>
      <Button variant="tertiary" disabled={!canModify || !history.current.past.length} onClick={() => moveHistory(false)}>Undo scene edit</Button>
      <Button variant="tertiary" disabled={!canModify || !history.current.future.length} onClick={() => moveHistory(true)}>Redo scene edit</Button>
      <Button variant="tertiary" disabled={!frame || busy} aria-pressed={preview} onClick={() => { gesture.current = null; setGestureScene(null); setPreview(!preview); }}>Try frame interactions</Button>
      <Button variant="tertiary" disabled={busy} onClick={onPlan}>Edit screen plan</Button>
    </div>
    {notice && <p role="status">{notice}</p>}{errors.length > 0 && <p role="alert">{errors[0]} Save remains blocked until these references are repaired.</p>}
    {!plan.scene && <p className="journey-muted">This saved design has no editable scene yet. Add a frame for a planned screen or ask Luci to propose one. Your sketch and existing screen plan stay available.</p>}
    {!plan.screens.length && <Button variant="secondary" disabled={!canModify} onClick={onPlan}>Plan the first screen</Button>}
    <div className="design-scene-pane-switch" aria-label="Canvas panes">{(['layers', 'canvas', 'properties'] as const).map(value => <Button key={value} variant="tertiary" aria-pressed={pane === value} onClick={() => setPane(value)}>{value[0].toUpperCase() + value.slice(1)}</Button>)}</div>
    {frame && <>
      <div className="design-scene-tools" aria-label="Add component">{(['text', 'rectangle', 'button', 'input', 'divider'] as const).map(kind => <Button key={kind} variant="tertiary" disabled={readOnly} onClick={() => add(kind)}>Add {kind}</Button>)}
        <label>Zoom<select value={scale} onChange={event => setScale(Number(event.target.value))}>{![0.25, 0.5, 0.75, 1, 1.5, 2].includes(scale) && <option value={scale}>Fit · {Math.round(scale * 100)}%</option>}{[0.25, 0.5, 0.75, 1, 1.5, 2].map(value => <option key={value} value={value}>{value * 100}%</option>)}</select></label>
      <Button variant="tertiary" onClick={() => { const available = stage.current?.clientWidth ?? 390; setScale(Math.max(0.1, Math.min(1, (available - 24) / frame.width))); }}>Fit frame</Button>
      </div>
      <div className="design-scene-layout" data-pane={pane}>
        <aside className="design-scene-layers" aria-label="Scene layers"><h3>Layers</h3><p className="journey-muted">Front layers appear first.</p>
          <ol>{[...frame.nodes].reverse().map(item => <li key={item.id}><button type="button" aria-pressed={node?.id === item.id} onClick={() => { setNodeId(item.id); pendingFocus.current = item.id; setPane('properties'); }}>{item.name} · {item.kind}{item.hidden ? ' · hidden' : ''}{item.locked ? ' · locked' : ''}</button></li>)}</ol>
          {!frame.nodes.length && <p>Add a component, then select it to edit.</p>}
        </aside>
        <div className="design-scene-stage" ref={stage}>
          <div className="design-scene-scale" style={{width: frame.width * scale, height: frame.height * scale}}>
            <div className="design-scene-frame" aria-label={`Design frame ${frame.name}`} style={{width: frame.width, height: frame.height, background: frame.background, transform: `scale(${scale})`}} onPointerDown={event => { if (event.target === event.currentTarget) setNodeId(''); }}>
              {frame.nodes.filter(item => !item.hidden).map(item => <div key={item.id} ref={element => { if (element) nodeElements.current.set(item.id, element); else nodeElements.current.delete(item.id); }}
                className="design-scene-node" data-selected={node?.id === item.id || undefined} data-preview={preview || undefined} role="button" tabIndex={0}
                aria-label={`${preview ? 'Try' : 'Select'} ${item.name} (${item.id})`} onFocus={() => setNodeId(item.id)}
                onPointerDown={event => start(event, item)} onPointerMove={move} onPointerUp={event => finish(event)} onPointerCancel={event => finish(event, true)}
                onLostPointerCapture={event => finish(event, true)} onKeyDown={event => keys(event, item)} onKeyUp={endGroup}
                onClick={() => { setNodeId(item.id); if (preview && item.behavior.kind === 'navigate') { setFrameId(item.behavior.targetFrameId); setNodeId(''); } }}
                onDoubleClick={() => { if (canModify && !preview) { setPane('properties'); textInput.current?.focus(); } }}
                style={nodeCSS(item)}>
                {item.kind === 'image' ? renderImage?.(item.assetId, item.text || item.name) : <span>{item.text}</span>}
                {node?.id === item.id && !item.locked && !readOnly && <span role="button" tabIndex={-1} aria-label={`Resize ${item.name}`} className="design-scene-resize" onPointerDown={event => start(event, item, true)} />}
              </div>)}
            </div>
          </div>
        </div>
        <aside className="design-scene-properties" aria-label="Selected component properties"><h3>{node ? node.name : 'Frame properties'}</h3>
          <fieldset disabled={!canModify || preview}>
            {!node ? <>
              <TextProperty label="Frame name" value={frame.name} onCommit={value => patchFrame({name: value})} />
              <label>Linked screen<select value={frame.screenId} onChange={event => patchFrame({screenId: event.target.value, state: ''})}>{!screen && <option value={frame.screenId}>Unavailable screen</option>}{plan.screens.map(item => <option key={item.id} value={item.id}>{item.name}</option>)}</select></label>
              <label>Screen state<select value={frame.state} onChange={event => patchFrame({state: event.target.value})}><option value="">Default state</option>{frame.state && !screen?.states.includes(frame.state) && <option value={frame.state}>Unavailable: {frame.state}</option>}{screen?.states.map(value => <option key={value} value={value}>{value}</option>)}</select></label>
              <NumberProperty label="Frame width" value={frame.width} min={320} max={3840} onCommit={value => patchFrame({width: value})} />
              <NumberProperty label="Frame height" value={frame.height} min={1} max={8192} onCommit={value => patchFrame({height: value})} />
              <TextProperty label="Frame background" value={frame.background} onCommit={value => patchFrame({background: value})} />
              <Button variant="tertiary" onClick={() => { const next = {...scene, frames: scene.frames.filter(item => item.id !== frame.id).map(item => ({...item, nodes: item.nodes.map(component => component.behavior.targetFrameId === frame.id ? {...component, behavior: {kind: 'none' as const, targetFrameId: ''}} : component)}))}; commit(next); setFrameId(''); }}>Remove frame</Button>
            </> : <>
              <label><input type="checkbox" checked={node.locked} onChange={event => patchNode({locked: event.target.checked})} />Lock component</label>
              <fieldset disabled={node.locked}>
              <TextProperty label="Component name" value={node.name} onCommit={value => patchNode({name: value})} />
              <label>Content / image description<textarea ref={textInput} rows={3} value={node.text} maxLength={4000} onChange={event => patchNode({text: event.target.value}, `${node.id}:text`)} onBlur={endGroup} /></label>
              <div className="design-scene-numbers">{(['x', 'y', 'width', 'height'] as const).map(key => <NumberProperty key={key} label={key === 'x' || key === 'y' ? key.toUpperCase() : key[0].toUpperCase() + key.slice(1)} value={node[key]} min={key === 'x' || key === 'y' ? 0 : 1} max={key === 'x' || key === 'width' ? frame.width : frame.height} onCommit={value => patchNode({[key]: value})} />)}</div>
              {(['fill', 'stroke', 'color'] as const).map(key => <TextProperty key={key} label={key === 'color' ? 'Text colour' : key === 'fill' ? 'Fill colour' : 'Border colour'} value={node.style[key]} onCommit={value => patchStyle({[key]: value})} />)}
              {([{key: 'fontSize', label: 'Font size', min: 8, max: 96}, {key: 'padding', label: 'Padding', min: 0, max: 128}, {key: 'radius', label: 'Corner radius', min: 0, max: 64}, {key: 'strokeWidth', label: 'Border width', min: 0, max: 16}, {key: 'opacity', label: 'Opacity (%)', min: 0, max: 100}] as const).map(item => <NumberProperty key={item.key} label={item.label} value={node.style[item.key]} min={item.min} max={item.max} onCommit={value => patchStyle({[item.key]: value})} />)}
              <label>Font family<select value={node.style.fontFamily} onChange={event => patchStyle({fontFamily: event.target.value as DesignNodeStyle['fontFamily']})}>{['system', 'serif', 'mono'].map(value => <option key={value}>{value}</option>)}</select></label>
              <label>Font weight<select value={node.style.fontWeight} onChange={event => patchStyle({fontWeight: Number(event.target.value)})}>{[400, 500, 600, 700].map(value => <option key={value}>{value}</option>)}</select></label>
              <label>Text alignment<select value={node.style.textAlign} onChange={event => patchStyle({textAlign: event.target.value as DesignNodeStyle['textAlign']})}>{['left', 'center', 'right'].map(value => <option key={value}>{value}</option>)}</select></label>
              <label>Local frame interaction<select value={node.behavior.targetFrameId} onChange={event => patchNode({behavior: {kind: event.target.value ? 'navigate' : 'none', targetFrameId: event.target.value}})}><option value="">None</option>{scene.frames.map(item => <option key={item.id} value={item.id}>Show {item.name}</option>)}</select></label>
              <label><input type="checkbox" checked={node.hidden} onChange={event => patchNode({hidden: event.target.checked})} />Hide component</label>
              <div className="journey-actions"><Button variant="tertiary" onClick={() => reorder(1)}>Bring forward</Button><Button variant="tertiary" onClick={() => reorder(-1)}>Send backward</Button><Button variant="tertiary" onClick={() => { const copied = {...node, id: newDesignId('node'), name: `${node.name} copy`.slice(0, 100)}; pendingFocus.current = copied.id; patchFrame({nodes: [...frame.nodes, copied]}); setNodeId(copied.id); }}>Duplicate component</Button><Button variant="tertiary" onClick={() => { patchFrame({nodes: frame.nodes.filter(item => item.id !== node.id)}); setNodeId(''); }}>Remove component</Button></div>
              </fieldset>
            </>}
          </fieldset>
        </aside>
      </div>
      <p className="journey-muted">Select a layer or component. Drag to move; drag its corner to resize. Arrow keys move; Alt + arrows resize; Shift uses 10 px. Undo/redo edits this draft only. Frame interactions are design navigation, not a running application.</p>
    </>}
    <Button variant="secondary" disabled={busy || !canEdit || !canAsk} onClick={() => onAsk(`Amend the saved versioned design-plan scene${frame ? ` for frame ${frame.id} (screen ${frame.screenId}${frame.state ? `, state ${frame.state}` : ''})` : ''}${node ? `, selected component ${node.id}` : ''}. Use the same typed scene with integer geometry and opacity percent. Preserve unrelated components, screen/state/asset references and pinned image content. Propose a design-plan amendment for normal review; do not apply arbitrary scripts or change a running task's captured context.`)}>Ask Luci about this design</Button>
    {!canAsk && <p className="journey-muted">Save or resolve your design draft before asking Luci to amend its saved scene.</p>}
    <span className="sr-only">Draft history update {historyVersion}</span>
  </section>;
}
function nodeCSS(node: DesignSceneNode): CSSProperties {
  return {position: 'absolute', left: node.x, top: node.y, width: node.width, height: node.height, boxSizing: 'border-box', background: node.style.fill,
    border: `${node.style.strokeWidth}px solid ${node.style.stroke}`, borderRadius: node.style.radius, color: node.style.color, fontSize: node.style.fontSize, fontWeight: node.style.fontWeight,
    fontFamily: node.style.fontFamily === 'mono' ? 'monospace' : node.style.fontFamily === 'serif' ? 'serif' : 'system-ui, sans-serif', textAlign: node.style.textAlign, padding: node.style.padding, opacity: node.style.opacity / 100};
}
function NumberProperty({label, value, min, max, onCommit}: {label: string; value: number; min: number; max: number; onCommit(value: number): void}) {
  const [raw, setRaw] = useState(String(value)), id = useId();
  useEffect(() => setRaw(String(value)), [value]);
  return <label htmlFor={id}>{label}<input id={id} type="number" min={min} max={max} step={1} value={raw} onChange={event => setRaw(event.target.value)} onBlur={() => {
    const next = Number(raw); if (raw.trim() && Number.isSafeInteger(next) && next >= min && next <= max) onCommit(next); else setRaw(String(value));
  }} /></label>;
}
function TextProperty({label, value, onCommit}: {label: string; value: string; onCommit(value: string): void}) {
  const [raw, setRaw] = useState(value), id = useId(); useEffect(() => setRaw(value), [value]);
  return <label htmlFor={id}>{label}<input id={id} value={raw} onChange={event => setRaw(event.target.value)} onBlur={() => { if (raw.trim()) onCommit(raw); else setRaw(value); }} /></label>;
}
