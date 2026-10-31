import { useEffect, useRef, useState } from 'react';
import { designPlanApi, type DesignHistoryResponse } from '../../api/designPlan';
import { errorText } from '../../api/product';
import { Button } from '../fluid/components/button';

/** Read-only recent snapshots. It never restores a scene or changes the editor. */
export function DesignHistory({ projectId, scope, canRead, active, currentRevision }: {
  projectId: string; scope: string | null; canRead: boolean; active: boolean; currentRevision: number;
}) {
  const identity = JSON.stringify([scope, projectId, canRead, active]);
  const owner = useRef({ identity, generation: 0 });
  if (owner.current.identity !== identity) owner.current = { identity, generation: owner.current.generation + 1 };
  const key = JSON.stringify([identity, owner.current.generation]);
  const [read, setRead] = useState<{ key: string; value: DesignHistoryResponse | null; error: string; busy: boolean } | null>(null);
  const [refresh, setRefresh] = useState(0), [selected, setSelected] = useState<number | null>(null);
  useEffect(() => {
    if (!scope || !canRead || !active) return;
    const controller = new AbortController(); let alive = true;
    setRead(previous => ({ key, value: previous?.key === key ? previous.value : null, busy: true, error: '' }));
    void designPlanApi.history(projectId, controller.signal).then(value => {
      if (!alive || controller.signal.aborted) return;
      setRead({ key, value, busy: false, error: '' });
      setSelected(previous => value.revisions.some(row => row.revision === previous) ? previous : value.revisions.reduce((latest, row) => Math.max(latest, row.revision), 0) || null);
    }).catch(cause => {
      if (alive && !controller.signal.aborted) setRead(previous => ({ key, value: previous?.key === key ? previous.value : null, busy: false, error: errorText(cause) }));
    });
    return () => { alive = false; controller.abort(); };
  }, [projectId, scope, canRead, active, key, refresh]);
  if (!active) return null;
  if (!scope || !canRead) return <p role="status">Current project read access is required to inspect saved design revisions.</p>;
  const value = read?.key === key ? read.value : null, error = read?.key === key ? read.error : '', busy = read?.key !== key || !!read?.busy;
  const revisions = value?.revisions.slice().sort((a, b) => b.revision - a.revision) ?? [];
  const snapshot = revisions.find(row => row.revision === selected);
  return <section aria-labelledby="design-history-title">
    <h2 id="design-history-title">Saved design revisions</h2>
    <p className="journey-muted">Your editor uses saved revision {currentRevision}. Inspecting a snapshot does not replace your draft or confirm its current approval.</p>
    <Button type="button" variant="secondary" disabled={busy} onClick={() => setRefresh(value => value + 1)}>Refresh saved revisions</Button>
    {busy && <p role="status">Reading saved design revisions…</p>}
    {error && <p role="alert" className="journey-error">{error} <Button type="button" variant="secondary" onClick={() => setRefresh(value => value + 1)}>Retry saved revisions</Button></p>}
    {value && revisions.length === 0 && <p>No saved design snapshots are available yet. Save a design in Canvas or Screens and states to create one.</p>}
    {value?.archiveHead && <p className="journey-muted">This view lists recent snapshots. Older revisions may be held in Context storage.</p>}
    {revisions.length > 0 && <>
      <ol className="definition-history">{revisions.map(row => <li key={row.revision}>
        <Button type="button" variant="tertiary" aria-pressed={row.revision === snapshot?.revision} onClick={() => setSelected(row.revision)}>Read design revision {row.revision}</Button>
        <p>Saved status: {row.status} · {new Date(row.updatedAt).toLocaleString()}{row.revision === currentRevision && ' · Editor baseline'}</p>
      </li>)}</ol>
      {snapshot && <section aria-label={`Saved design revision ${snapshot.revision}`}>
        <h3>Saved revision {snapshot.revision}</h3>
        <p>Brief {snapshot.briefRevision} · Definition {snapshot.specificationRevision} · Sketch {snapshot.sketchRevision}. {snapshot.screens.length} screens, {snapshot.scene?.frames.length ?? 0} editable frames.</p>
        <ul>{snapshot.screens.map(screen => <li key={screen.id}>{screen.name} · {screen.states.join(', ') || 'No states recorded'} · {screen.widths.map(width => `${width} px`).join(', ') || 'No widths recorded'}</li>)}</ul>
        <details><summary>Read saved snapshot</summary><pre tabIndex={0}>{JSON.stringify(snapshot, null, 2)}</pre></details>
      </section>}
    </>}
  </section>;
}
