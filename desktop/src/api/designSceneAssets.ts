import { authHeaders, authToken, expireAuth } from './authSession';
import { productApi } from './product';
import { teamScopeGeneration } from './teamScope';
import { designRasterPinValid, type DesignRasterPin } from './designScene';

const path = (projectId: string) => `/v1/product/projects/${encodeURIComponent(projectId)}/design-plan`;
export async function bindDesignRaster(projectId: string, assetPath: string, commit: string, signal: AbortSignal): Promise<DesignRasterPin> {
  if (!/^[a-f0-9]{40}$/.test(commit)) throw new Error('Enter the full 40-character project commit.');
  const response = await productApi.request<{projectId: string; path: string; content: DesignRasterPin}>(`${path(projectId)}/asset-binding?${new URLSearchParams({path: assetPath, commit})}`, undefined, 'GET', 30_000, signal);
  if (response.projectId !== projectId || response.path !== assetPath || !designRasterPinValid(response.content) || response.content.sourceCommit !== commit) throw new Error('The image binding did not match this project, path and commit.');
  return response.content;
}
/** Verified bytes only. The view owns and retires any object URL it creates. */
export async function readDesignRaster(projectId: string, revision: number, assetId: string, pin: DesignRasterPin, signal: AbortSignal): Promise<Blob> {
  if (!designRasterPinValid(pin) || !Number.isSafeInteger(revision) || revision < 1) throw new Error('Save this pinned asset before rendering it.');
  const generation = teamScopeGeneration(), token = authToken(), base = productApi.baseUrl;
  const current = () => { signal.throwIfAborted(); if (generation !== teamScopeGeneration() || token !== authToken() || base !== productApi.baseUrl) throw new Error('Project access changed while the image was loading.'); };
  const response = await fetch(`${base}${path(projectId)}/${revision}/assets/${encodeURIComponent(assetId)}?digest=${pin.digest}`, {signal, headers: authHeaders()});
  current();
  if (response.status === 401) expireAuth();
  if (!response.ok) throw new Error(response.status === 403 || response.status === 401 ? 'Current project read access is required.' : 'This saved image revision is unavailable.');
  if (response.headers.get('Content-Type') !== pin.mimeType || response.headers.get('Content-Length') !== String(pin.bytes) || !response.body) throw new Error('The image response did not match its saved binding.');
  const reader = response.body.getReader(), chunks: Uint8Array[] = [];
  let total = 0;
  try {
    for (;;) {
      const {done, value} = await reader.read(); current(); if (done) break;
      total += value.byteLength;
      if (total > pin.bytes || total >= 2 * 1024 * 1024) throw new Error('The image exceeded its saved size.');
      chunks.push(value);
    }
  } finally { await reader.cancel().catch(() => {}); reader.releaseLock(); }
  if (total !== pin.bytes) throw new Error('The image response was incomplete.');
  const bytes = new Uint8Array(total); let offset = 0;
  for (const chunk of chunks) { bytes.set(chunk, offset); offset += chunk.length; }
  const digest = Array.from(new Uint8Array(await crypto.subtle.digest('SHA-256', bytes))).map(value => value.toString(16).padStart(2, '0')).join('');
  current(); if (digest !== pin.digest) throw new Error('The saved image digest did not match.');
  return new Blob([bytes], {type: pin.mimeType});
}
