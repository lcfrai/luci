import { useEffect, useRef, useState } from 'react';
import { metricCSVTemplate, metricImportsValid, metricValue, parseMetricCSV, type GTMMetricImport } from '../../api/gtmMetricImports';
import { errorText } from '../../api/product';
import { Button } from '../fluid/components/button';
import './gtm-metrics.css';

export function GTMMetricImports({ projectId, contextKey, value, editable, canDiscuss, onChange }: { projectId: string; contextKey: string; value: GTMMetricImport[]; editable: boolean; canDiscuss: boolean; onChange(value: GTMMetricImport[]): void }) {
 const [preview, setPreview] = useState<GTMMetricImport | null>(null), [error, setError] = useState(''), [busy, setBusy] = useState(false);
 const owner = useRef({ generation: 0, mounted: true, editable }); owner.current.editable = editable;
 useEffect(() => { owner.current.mounted = true; owner.current.generation++; setPreview(null); setBusy(false); setError(''); return () => { owner.current.mounted = false; owner.current.generation++; }; }, [projectId, contextKey, editable]);
 async function choose(file: File | undefined) {
  if (!file || !owner.current.editable) return;
  const generation = ++owner.current.generation; setBusy(true); setError(''); setPreview(null);
  try {
   if (file.size < 1 || file.size > 128 * 1024) throw new Error('Choose a CSV between 1 byte and 128 KiB.');
   const result = await parseMetricCSV(await file.arrayBuffer(), file.name);
   if (!owner.current.mounted || !owner.current.editable || generation !== owner.current.generation) return;
   if (!metricImportsValid([...value, result])) throw new Error('This source is already included, or the workspace would exceed 8 imports / 128 rows.');
   setPreview(result);
  } catch (cause) { if (owner.current.mounted && generation === owner.current.generation) setError(errorText(cause)); }
  finally { if (owner.current.mounted && generation === owner.current.generation) setBusy(false); }
 }
 function add() {
  if (!editable || !preview) return;
  const next = [...value, preview]; if (!metricImportsValid(next)) { setError('This import is already included or exceeds the current workspace limit.'); return; }
  onChange(next); setPreview(null); setError('');
 }
 function discuss(batch: GTMMetricImport) {
  if (!canDiscuss) return;
  window.dispatchEvent(new CustomEvent('luci:orchestrator-open', { detail: { projectId, message: 'Review the saved GTM aggregate import ' + batch.sourceSha256 + ' (' + batch.sourceName + '). These are user-reported results, not authenticated provider observations. Identify what the figures support, what they cannot establish, and one useful experiment or roadmap amendment. Preserve all imported results unchanged. Do not combine overlapping periods or currencies, infer revenue from event counts, or publish anything.' } }));
 }
 return <details className="gtm-metrics"><summary>Analytics and revenue results ({value.reduce((n, b) => n + b.rows.length, 0)} rows)</summary>
  <p>Bring in aggregate results from your analytics or billing exports. Review them here, then save with your GTM revision.</p>
  <p className="gtm-metrics__hint">Use the six-column template. Counts and money stay separate; Luci does not combine overlapping periods or different currencies. Include no customer names, emails or transaction-level data.</p>
  <a href={'data:text/csv;charset=utf-8,' + encodeURIComponent(metricCSVTemplate)} download="luci-aggregate-results-template.csv">Download CSV template with example values</a>
  <label>Preview aggregate CSV<input type="file" accept=".csv,text/csv" disabled={!editable || busy || value.length >= 8} onChange={event => { const file = event.target.files?.[0]; event.target.value = ''; void choose(file); }} /></label>
  {busy && <p role="status">Reading aggregate rows…</p>}{error && <p role="alert">{error}</p>}
  {preview && <section aria-label="Import preview"><h3>Review {preview.sourceName}</h3><MetricRows batch={preview} /><p>Only the displayed aggregates and source attribution are saved. The original file stays on your device.</p><div className="journey-actions"><Button disabled={!editable || busy} onClick={add}>Add these results to draft</Button><Button variant="secondary" onClick={() => setPreview(null)}>Discard import preview</Button></div></section>}
  {value.map(batch => <details key={batch.sourceSha256}><summary>{batch.sourceName} · {batch.rows.length} aggregate rows</summary><MetricRows batch={batch} /><p className="gtm-metrics__hint">User-reported import · source SHA-256 <code>{batch.sourceSha256}</code>. The file hash identifies the selected bytes; it does not verify the provider or the figures.</p><div className="journey-actions"><Button variant="secondary" disabled={!canDiscuss} onClick={() => discuss(batch)}>Discuss these results with Luci</Button><Button variant="secondary" disabled={!editable} onClick={() => onChange(value.filter(x => x.sourceSha256 !== batch.sourceSha256))}>Remove {batch.sourceName} from draft</Button></div></details>)}
  {!canDiscuss && value.length > 0 && <p className="gtm-metrics__hint">Save the current GTM draft before discussing its results with Luci.</p>}
 </details>;
}
function MetricRows({ batch }: { batch: GTMMetricImport }) {
 return <div className="gtm-metrics__table" role="region" aria-label={'Aggregate rows from ' + batch.sourceName} tabIndex={0}><table><caption>Imported figures · no totals inferred</caption><thead><tr><th scope="col">Metric</th><th scope="col">Period (inclusive)</th><th scope="col">Reported value</th><th scope="col">CSV row</th></tr></thead><tbody>{batch.rows.map(row => <tr key={row.row}><th scope="row">{row.metric.replace(/_/g, ' ')}</th><td>{row.start} to {row.end}</td><td>{metricValue(row)}</td><td>{row.row}</td></tr>)}</tbody></table></div>;
}
