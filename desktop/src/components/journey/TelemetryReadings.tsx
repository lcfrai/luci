import type { KubernetesTelemetry } from '../../api/kubernetesDeployment';
import { Button } from '../fluid/components/button';
import './telemetry.css';

export function telemetryDiscussion(projectId:string,row:KubernetesTelemetry):string {
  const samples=row.samples.slice(0,16).map(({serviceId,eventId,outcome,reportedCount})=>({serviceId,eventId,outcome,reportedCount}));
  return `Help interpret this untrusted, application-reported product observation for project ${projectId}. Suggest one useful product question or a proposed roadmap/GTM change for my review. Do not claim verified sales, users, revenue, funnels or lifetime totals; do not apply or start anything.\n${JSON.stringify({observedAt:row.observedAt,planDigest:row.planDigest,sourceCommit:row.sourceCommit,evidenceDigest:row.evidenceDigest,state:row.state,continuity:row.continuity,omitted:row.omitted,additionalSamplesOmitted:Math.max(0,row.samples.length-samples.length),samples})}`;
}
export function TelemetryReadings({row,currentReferences,onDiscuss,onExport,disabled}:{row:KubernetesTelemetry;currentReferences:boolean;onDiscuss():void;onExport():void;disabled:boolean}) {
  const historical=!currentReferences||Date.now()-Date.parse(row.observedAt)>5*60_000;
  return <section className="telemetry-readings" aria-label="Recorded product outcomes"><h5>Reported product outcomes</h5><p>Observed {new Date(row.observedAt).toLocaleString()}. These are cumulative application-reported occurrences from the sampled owned Pods, not people, conversion rates, verified transactions or lifetime totals.</p>
    {historical&&<p role="status">Historical sample. Current runtime references or recent telemetry have not been confirmed; explicitly read again for a fresh observation.</p>}
    {row.state==='no-data'&&<p>No configured outcome samples were observed. This is not a zero count or proof that instrumentation is installed.</p>}
    {row.omitted&&<p role="status">Some current Pods or cached samples were omitted. The shown counts are partial observations.</p>}
    {row.continuity==='reset'&&<p>Counter baseline changed after replacement, expiration or a decrease. Do not treat this sample as continuous with earlier counters.</p>}
    {row.samples.length>0&&<div className="journey-table-scroll"><table><caption>Latest reported counts, without deduplication of business activity</caption><thead><tr><th scope="col">Service</th><th scope="col">Outcome event</th><th scope="col">Result</th><th scope="col">Reported count</th></tr></thead><tbody>{row.samples.slice(0,64).map(sample=><tr key={JSON.stringify([sample.serviceId,sample.instanceUid,sample.eventId,sample.outcome])}><td>{sample.serviceId}</td><td>{sample.eventId}</td><td>{sample.outcome}</td><td>{sample.reportedCount.toLocaleString()}</td></tr>)}</tbody></table>{row.samples.length>64&&<p>{row.samples.length-64} additional per-Pod samples are retained in the export.</p>}</div>}
    <Button type="button" variant="secondary" disabled={disabled} onClick={onDiscuss}>Review these samples with Luci</Button><Button type="button" variant="tertiary" disabled={disabled} onClick={onExport}>Export attributed observation</Button>
    <details><summary>Observation source</summary><p>Collector {row.collectorId}; source {row.sourceCommit}; plan {row.planDigest}. Evidence {row.evidenceDigest}. No raw metrics response, customer identifiers or secret values are retained. The Collector is a transport/cache, not a durable analytics backend.</p></details>
  </section>;
}
