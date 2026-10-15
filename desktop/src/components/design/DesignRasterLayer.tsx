import { useEffect, useRef, useState } from 'react';
import type { DesignPlan } from '../../api/designPlan';
import { readDesignRaster } from '../../api/designSceneAssets';
import { AUTH_EXPIRED } from '../../api/authSession';
import { TEAM_WORKSPACE_CHANGED, TEAM_WORKSPACE_ACCESS_DENIED } from '../../api/teamScope';
import type { DesignRasterPin } from '../../api/designScene';
import { productApi } from '../../api/product';
import { teamScopeGeneration } from '../../api/teamScope';

type ImageState = {url?: string; error?: string; key?: string};
/** Intersecting nodes share verified image URLs; scope owners retire their references. */
export function DesignRasterLayer({projectId, revision, asset, alt, canRead, scope, saved = true}: {projectId: string; revision: number; asset?: DesignPlan['assets'][number]; alt: string; canRead: boolean; scope: string | null; saved?: boolean}) {
  const element = useRef<HTMLSpanElement>(null), [visible, setVisible] = useState(false), [state, setState] = useState<ImageState>({}), [retry, setRetry] = useState(0);
  const pinKey = JSON.stringify(asset?.content), assetId = asset?.id ?? '';
  const viewKey = JSON.stringify([scope, projectId, revision, assetId, pinKey]);
  useEffect(() => {
    const target = element.current; if (!target) return;
    if (typeof IntersectionObserver === 'undefined') { setVisible(true); return; }
    const observer = new IntersectionObserver(entries => setVisible(entries.some(entry => entry.isIntersecting)));
    observer.observe(target); return () => observer.disconnect();
  }, []);
  useEffect(() => {
    const controller = new AbortController(); let release = () => {}; const timer = setTimeout(() => { controller.abort(); release(); setState({error: 'Image loading timed out. Retry when the project reconnects.'}); }, 30_000);
    const retire = () => { clearTimeout(timer); controller.abort(); release(); release = () => {}; setState({}); };
    window.addEventListener(AUTH_EXPIRED, retire); window.addEventListener(TEAM_WORKSPACE_CHANGED, retire); window.addEventListener(TEAM_WORKSPACE_ACCESS_DENIED, retire);
    setState({});
    if (canRead && visible && saved && asset?.content) {
      const pin = JSON.parse(pinKey!) as DesignRasterPin;
      const lease = shareRaster(JSON.stringify([scope, productApi.baseUrl, teamScopeGeneration(), projectId, revision, assetId, pin]), projectId, revision, assetId, pin); release = lease.release;
      void lease.ready.then(url => {
        if (controller.signal.aborted) return; clearTimeout(timer); setState({url, key: viewKey});
      }).catch(cause => { clearTimeout(timer); release(); if (!controller.signal.aborted) setState({error: cause instanceof Error ? cause.message : 'Saved image unavailable.'}); });
    } else clearTimeout(timer);
    return () => { clearTimeout(timer); controller.abort(); release(); window.removeEventListener(AUTH_EXPIRED, retire); window.removeEventListener(TEAM_WORKSPACE_CHANGED, retire); window.removeEventListener(TEAM_WORKSPACE_ACCESS_DENIED, retire); };
  }, [projectId, revision, assetId, pinKey, canRead, visible, scope, retry, viewKey, saved]);
  return <span ref={element} className="design-raster-layer">{state.url && state.key === viewKey && canRead && saved ? <img src={state.url} alt={alt} draggable={false} /> : <span role={state.error ? 'status' : undefined}>{!canRead ? 'Read access required' : !saved ? 'Save this pinned asset before rendering it' : state.error || (!asset?.content ? 'Pin this image in Assets and provenance' : 'Saved image loading')}{state.error && <button type="button" onClick={event => { event.stopPropagation(); setRetry(value => value + 1); }}>Retry image</button>}</span>}</span>;
}

// Bounded URL storage, keyed by exact scope/revision/pin. Duplicate nodes share one raster.
type RasterEntry = {refs: number; controller: AbortController; ready: Promise<string>; url: string; pixels: number; bytes: number};
const images = new Map<string, RasterEntry>();
let retainedPixels = 0, retainedBytes = 0;
function shareRaster(key: string, projectId: string, revision: number, assetId: string, pin: DesignRasterPin): {ready: Promise<string>; release(): void} {
  let entry = images.get(key);
  if (!entry) {
    const pixels = pin.width * pin.height;
    if (retainedPixels + pixels > 32 * 1024 * 1024 || retainedBytes + pin.bytes > 16 * 1024 * 1024) return {ready: Promise.reject(new Error('Visible image memory limit reached. Scroll other images out of view, then retry this image.')), release() {}};
    retainedPixels += pixels; retainedBytes += pin.bytes;
    entry = {refs: 0, controller: new AbortController(), ready: Promise.resolve(''), url: '', pixels, bytes: pin.bytes};
    const owned = entry;
    entry.ready = acquireRaster(projectId, revision, assetId, pin, entry.controller.signal).then(blob => {
      owned.controller.signal.throwIfAborted(); owned.url = URL.createObjectURL(blob); return owned.url;
    });
    images.set(key, entry);
  }
  entry.refs++; const owned = entry; let released = false;
  return {ready: entry.ready, release() {
    if (released) return; released = true; owned.refs--;
    if (owned.refs === 0) {
      owned.controller.abort(); if (owned.url) URL.revokeObjectURL(owned.url);
      images.delete(key); retainedPixels -= owned.pixels; retainedBytes -= owned.bytes;
    }
  }};
}

// Bound active downloads across canvases; queued work is cancelled with its last view owner.
let active = 0;
const queue: (() => void)[] = [];
async function acquireRaster(projectId: string, revision: number, assetId: string, pin: DesignRasterPin, signal: AbortSignal): Promise<Blob> {
  await new Promise<void>((resolve, reject) => {
    let queued = false;
    const start = () => { signal.removeEventListener('abort', abort); if (signal.aborted) { reject(signal.reason); return; } active++; resolve(); };
    const abort = () => { if (queued) { const at = queue.indexOf(start); if (at >= 0) queue.splice(at, 1); } reject(signal.reason); };
    if (signal.aborted) { reject(signal.reason); return; }
    if (active < 2) start(); else { queued = true; signal.addEventListener('abort', abort, {once: true}); queue.push(start); }
  });
  try { return await readDesignRaster(projectId, revision, assetId, pin, signal); }
  finally { active--; queue.shift()?.(); }
}
