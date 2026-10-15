import { useEffect, useRef, useState } from 'react';
import type { DesignPlan } from '../../api/designPlan';
import { bindDesignRaster } from '../../api/designSceneAssets';
import { Button } from '../fluid/components/button';

export function DesignAssetBinding({projectId, asset, disabled, onChange}: {projectId: string; asset: DesignPlan['assets'][number]; disabled: boolean; onChange(asset: DesignPlan['assets'][number]): void}) {
  const [commit, setCommit] = useState(asset.content?.sourceCommit ?? ''), [busy, setBusy] = useState(false), [error, setError] = useState('');
  const controller = useRef<AbortController | null>(null), latest = useRef({asset, disabled}); latest.current = {asset, disabled};
  useEffect(() => { controller.current?.abort(); setBusy(false); setError(''); return () => controller.current?.abort(); }, [projectId, asset.path, disabled]);
  return <div className="design-asset-binding">
    {asset.content ? <p>Immutable {asset.content.mimeType} · {asset.content.width} × {asset.content.height} · {asset.content.bytes} bytes<br /><code>{asset.content.sourceCommit}</code><br />SHA-256 <code>{asset.content.digest}</code></p> : <p>Metadata only. Pin a committed PNG or JPEG to use it in the canvas.</p>}
    <label>Full project Git commit<input value={commit} onChange={event => setCommit(event.target.value)} disabled={disabled || busy} autoCapitalize="none" spellCheck={false} /></label>
    <Button variant="secondary" disabled={disabled || busy || !asset.path || !/^[a-f0-9]{40}$/.test(commit)} loading={busy} onClick={async () => {
      const request = new AbortController(); controller.current?.abort(); controller.current = request; const originalPath = asset.path;
      setBusy(true); setError('');
      try {
        const content = await bindDesignRaster(projectId, originalPath, commit, request.signal);
        if (!request.signal.aborted && !latest.current.disabled && latest.current.asset.path === originalPath) onChange({...latest.current.asset, content});
      } catch (cause) { if (!request.signal.aborted) setError(cause instanceof Error ? cause.message : 'Image binding is unavailable.'); }
      finally { if (controller.current === request && !request.signal.aborted) setBusy(false); }
    }}>Verify immutable image</Button>
    {asset.content && <Button variant="tertiary" disabled={disabled || busy} onClick={() => { const {content: _content, ...metadata} = asset; onChange(metadata); }}>Remove image binding</Button>}
    {error && <p role="alert">{error}</p>}
    <p className="journey-muted">Verification reads this project's exact commit without changing files or approving the design. Save the pinned asset before rendering it. Changed image bytes need a new draft binding.</p>
  </div>;
}
