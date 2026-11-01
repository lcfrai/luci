import { useEffect, useRef, useState } from 'react';
import { commercialSignalsApi, stripeChargeAmount, validStripeFilter, type CommercialConfiguration, type CommercialSnapshot, type CommercialSourcesInput, type CommercialView, type ReleaseChoice } from '../../api/commercialSignals';
import { ApiError } from '../../api/client';
import { errorText } from '../../api/product';
import { useProjectAccess } from '../../hooks/useProjectAccess';
import { useProjectDraftScope } from '../../hooks/useProjectDraft';
import { useUnsavedProjectDraft } from '../../hooks/useUnsavedProjectDraft';
import { useTeamWorkspace } from '../teams/TeamWorkspaceProvider';
import { Button } from '../fluid/components/button';
import { ObservationRouting } from '../orchestrator/ObservationRouting';
import './commercial-signals.css';

type SourceDraft = Omit<CommercialSourcesInput, 'expectedRevision' | 'acknowledgeSharing'>;
const sourceDraft = (c: CommercialConfiguration): SourceDraft => ({ stripeEnabled: c.stripeEnabled, stripeFilter: c.stripeFilter, windowDays: c.windowDays, publicationIds: c.releases.map(row => row.publicationId), automaticReads: c.automaticReads });
const when = (value: string) => value ? new Date(value).toLocaleString() : 'not yet recorded';
const number = (value: number) => value.toLocaleString();

export function CommercialSignals({ projectId }: { projectId: string }) {
 const scope = useProjectDraftScope(projectId), access = useProjectAccess(projectId), team = useTeamWorkspace();
 const canRead = !!scope && access.canRead && !access.loading && !access.error && !team?.accessBlocked;
 const canConnect = canRead && access.canEdit && !!access.permissions.secrets;
 if (!canRead || !scope) return null;
 return <CommercialBody key={JSON.stringify([scope, projectId, team?.teamId, canConnect, access.canAssist])} projectId={projectId} scope={scope} canConnect={canConnect} canAssist={access.canAssist}/>;
}
function CommercialBody({ projectId, scope, canConnect, canAssist }: { projectId: string; scope: string; canConnect: boolean; canAssist: boolean }) {
 const [view, setView] = useState<CommercialView | null>(null), [draft, setDraft] = useState<SourceDraft | null>(null), [choices, setChoices] = useState<ReleaseChoice[] | null>(null);
 const [name, setName] = useState('Stripe receipts'), [apiKey, setAPIKey] = useState(''), [privateConsent, setPrivateConsent] = useState(false), [sharing, setSharing] = useState(false);
 const [sourceDirty, setSourceDirty] = useState(false), [baselineReview, setBaselineReview] = useState(false), [uncertain, setUncertain] = useState(false), [accessLost, setAccessLost] = useState(false);
 const [busy, setBusy] = useState(false), [error, setError] = useState(''), [notice, setNotice] = useState(''), [history, setHistory] = useState(false), [findingSetup, setFindingSetup] = useState(false);
 const owner = useRef({ mounted: false, generation: 0, pending: false, controller: null as AbortController | null });
 const dirty = sourceDirty || !!apiKey || name !== (view?.configuration.stripe?.name ?? 'Stripe receipts');
 function discard() { if (view) { setDraft(sourceDraft(view.configuration)); setName(view.configuration.stripe?.name ?? 'Stripe receipts'); } setAPIKey(''); setPrivateConsent(false); setSharing(false); setSourceDirty(false); setBaselineReview(false); }
 useUnsavedProjectDraft(projectId, dirty, 'Commercial source settings', 'distribute', { retained: false, discard });
 useEffect(() => { owner.current.mounted = true; void load(); return () => { owner.current.mounted = false; owner.current.generation++; owner.current.controller?.abort(); owner.current.pending = false; }; }, []);
 async function request(work: (signal: AbortSignal, current: () => boolean) => Promise<void>, mutation = false) {
  if (owner.current.pending || accessLost) return;
  const ticket = ++owner.current.generation, controller = new AbortController(); owner.current.controller?.abort(); owner.current.controller = controller; owner.current.pending = true;
  setBusy(true); setError(''); const current = () => owner.current.mounted && !controller.signal.aborted && owner.current.generation === ticket;
  try { await work(controller.signal, current); }
  catch (cause) { if (!current()) return;
   if (cause instanceof ApiError && (cause.status === 401 || cause.status === 403)) { setView(null); setDraft(null); setChoices(null); setAPIKey(''); setPrivateConsent(false); setSharing(false); setAccessLost(true); }
   else if (mutation) setUncertain(true);
   setError(errorText(cause));
  } finally { if (current()) { owner.current.pending = false; owner.current.controller = null; setBusy(false); } }
 }
 function receive(value: CommercialView, resetDraft: boolean) {
  setView(value);
  if (resetDraft) { setDraft(sourceDraft(value.configuration)); setName(value.configuration.stripe?.name ?? 'Stripe receipts'); setSourceDirty(false); setBaselineReview(false); setSharing(false); }
 }
 async function load() { await request(async (signal, current) => {
  const value = await commercialSignalsApi.get(projectId, signal); if (!current()) return;
  if (sourceDirty && view && value.configuration.revision !== view.configuration.revision) setBaselineReview(true);
  receive(value, !dirty); setUncertain(false); setNotice(dirty ? 'Saved settings and results refreshed. Your draft remains; compare changed settings before saving.' : 'Saved settings and complete results loaded. No provider was contacted.');
 }); }
 async function connect(remove = false) {
  if (!view || !canConnect || uncertain || busy || !remove && (!privateConsent || !/^rk_(test|live)_[A-Za-z0-9_]{16,504}$/.test(apiKey) || !name.trim())) return;
  const input = { expectedRevision: view.configuration.revision, name: name.trim(), apiKey: remove ? '' : apiKey, remove, acknowledgePrivateStorage: privateConsent };
  await request(async (signal, current) => { const value = await commercialSignalsApi.stripe(projectId, input, signal); if (!current()) return;
   receive(value, true); setAPIKey(''); setPrivateConsent(false); setNotice(remove ? 'Stripe connection removed. Older aggregate snapshots remain in project history.' : 'Restricted key verified and saved privately. Check the account and test/live mode, then explicitly enable charge reads below.');
  }, true);
 }
 function update(patch: Partial<SourceDraft>) { setDraft(old => old ? { ...old, ...patch } : old); setSourceDirty(true); setSharing(false); }
 async function saveSources(pause = false) {
  if (!view || !draft || !canConnect || uncertain || baselineReview && !pause || !pause && (!sharing || !validDraft)) return;
  const input: CommercialSourcesInput = { ...(pause ? { ...sourceDraft(view.configuration), stripeEnabled: false, publicationIds: [], automaticReads: false } : draft), expectedRevision: view.configuration.revision, acknowledgeSharing: pause || sharing };
  await request(async (signal, current) => { const value = await commercialSignalsApi.sources(projectId, input, signal); if (!current()) return;
   receive(value, !pause || !sourceDirty); if (pause && sourceDirty) setBaselineReview(true); setNotice(pause ? 'New provider reads paused and current project evidence withdrawn. Historical snapshots and your unsaved source draft are retained.' : 'Sources saved. Enabled sources share completed aggregates with this project. Automatic reads use the existing observation route when configured below.');
  }, true);
 }
 async function readChoices() { if (!canConnect || uncertain) return; await request(async (signal, current) => { const value = await commercialSignalsApi.releases(projectId, signal); if (current()) setChoices(value); }); }
 async function refresh(sourceKey = '', resetCollection = false) {
  if (!canConnect || uncertain || !view || sourceDirty || baselineReview) return;
  await request(async (signal, current) => { const value = await commercialSignalsApi.refresh(projectId, sourceKey, resetCollection, signal); if (!current()) return; receive(value, false); setNotice('One bounded read batch settled. Only exhausted collections become complete snapshots; Continue resumes a retained partial collection.'); }, true);
 }
 function review() {
  if (!view || !canAssist || uncertain || sourceDirty || !currentSnapshots.length) return;
  window.dispatchEvent(new CustomEvent('luci:orchestrator-open', { detail: { scope, projectId, message: 'Review the completed commercialSignals snapshots in the captured project context. Keep exact source, currency/native minor units, CHARGE metadata attribution, completed UTC cohort and refund-to-date semantics. These are historical non-atomic provider reads, not accounting revenue, unique visitors or proof of causality. Account-wide unallocated totals are deliberately excluded. Identify a useful finding and propose a reviewed roadmap or go-to-market amendment only where justified; preserve unrelated context.' } }));
  setNotice('Findings are ready in Luci as an unsent message. Review and send using your chosen model.');
 }
 const config = view?.configuration;
 const validDraft = !!draft && Number.isInteger(draft.windowDays) && draft.windowDays >= 1 && draft.windowDays <= 90 && validStripeFilter(draft.stripeFilter) && (!draft.stripeEnabled || !!config?.stripe);
 const sourceIDs = new Set(view?.status.map(row => row.latestId).filter(Boolean));
 const currentSnapshots = view?.snapshots.filter(row => sourceIDs.has(row.id) && row.attribution !== 'account-unallocated') ?? [];
 const shownSnapshots = view?.snapshots.filter(row => history || sourceIDs.has(row.id)).slice().reverse() ?? [];
 const releaseChoices = [...(config?.releases.map(row => ({ publicationId: row.publicationId, repository: row.repository, tag: row.tag, label: row.label })) ?? []), ...(choices ?? []).filter(row => !config?.releases.some(old => old.publicationId === row.publicationId))];
 const enabled = !!config && (config.stripeEnabled || config.releases.length > 0);
 return <section className="commercial-signals" aria-label="Commercial results">
  <header><h2>Learn from commercial results</h2><p>Connect receipts and release downloads to this project's next decisions. Refreshes read aggregates; they never charge customers or publish a release.</p></header>
  <div className="journey-actions"><Button variant="secondary" disabled={busy || accessLost} onClick={() => void load()}>Read saved commercial results</Button>{view && <Button variant="secondary" disabled={busy || accessLost || !canConnect || uncertain || sourceDirty || !enabled} onClick={() => void refresh()}>Refresh or continue next source</Button>}</div>
  {busy && <p role="status">Reading or saving commercial results…</p>}{error && <p role="alert">{error}</p>}{notice && <p role="status">{notice}</p>}
  {accessLost && <p>Current access changed. Refresh project access before reopening this section.</p>}
  {uncertain && <p>A response was not confirmed. Read saved results before another change; a completed save or collection checkpoint may already be present.</p>}
  {!canConnect && <p>Saved aggregates are project-readable. Connecting, configuring or refreshing requires current Edit and Secrets access.</p>}
  {view && draft && <>
   <section aria-label="Complete commercial snapshots"><h3>Completed results</h3><label className="check"><input type="checkbox" checked={history} onChange={event => setHistory(event.target.checked)}/>Show previous snapshots, including changed or paused sources</label>
    {shownSnapshots.length === 0 && <p>No complete current collection yet. Choose sources and refresh; previous results remain in history after source settings change.</p>}
    {shownSnapshots.map(row => <Snapshot key={row.id} row={row} current={sourceIDs.has(row.id)}/>)}
    {view.prunedSnapshots > 0 && <p>{number(view.prunedSnapshots)} older snapshots were removed by bounded history retention. Every currently selected source keeps its latest result.</p>}
   </section>
   <details><summary>Connect and choose sources</summary>
    <fieldset disabled={busy || !canConnect || uncertain}><legend>Stripe restricted read key</legend>
     {config?.stripe && <p>Saved account: <strong>{config.stripe.accountId}</strong> · {config.stripe.live ? 'LIVE' : 'TEST'} · {config.stripe.name}. Charge reads are {config.stripeEnabled ? 'enabled' : 'paused'}.</p>}
     <p>Use a restricted rk_test_ or rk_live_ key with Account and Charges read permission. The workspace stores it privately for this project; it is never returned to the assistant or browser after saving. Unrestricted secret keys are refused.</p>
     <label>Connection name<input value={name} maxLength={120} onChange={event => setName(event.target.value)}/></label>
     <label>Restricted Stripe key<input type="password" autoComplete="off" spellCheck={false} value={apiKey} maxLength={512} onChange={event => setAPIKey(event.target.value)}/></label>
     <label className="check"><input type="checkbox" checked={privateConsent} onChange={event => setPrivateConsent(event.target.checked)}/>Store this restricted key in private workspace/project state for authorized read-only refreshes.</label>
     <div className="journey-actions"><Button disabled={!privateConsent || !/^rk_(test|live)_[A-Za-z0-9_]{16,504}$/.test(apiKey) || !name.trim() || sourceDirty} onClick={() => void connect()}>Verify and save restricted key</Button>{config?.stripe && <Button variant="secondary" disabled={sourceDirty || !!apiKey} onClick={() => void connect(true)}>Remove Stripe connection</Button>}</div>
     {sourceDirty && <p>Save or discard source changes before changing the connection.</p>}
    </fieldset>
    <fieldset disabled={busy || !canConnect || uncertain}><legend>Project attribution and reporting window</legend>
     <label className="check"><input type="checkbox" checked={draft.stripeEnabled} disabled={!config?.stripe} onChange={event => update({ stripeEnabled: event.target.checked })}/>Enable captured-charge aggregates from the saved Stripe account</label>
     <label className="check"><input type="checkbox" checked={draft.stripeFilter !== null} onChange={event => update({ stripeFilter: event.target.checked ? { key: 'luci_project', value: '' } : null })}/>Attribute this project using an exact CHARGE metadata match</label>
     {draft.stripeFilter && <div className="commercial-fields"><label>Charge metadata key<input maxLength={40} value={draft.stripeFilter.key} onChange={event => update({ stripeFilter: { key: event.target.value, value: draft.stripeFilter!.value } })}/></label><label>Exact metadata value<input maxLength={200} value={draft.stripeFilter.value} onChange={event => update({ stripeFilter: { key: draft.stripeFilter!.key, value: event.target.value } })}/></label></div>}
     <p>{draft.stripeFilter ? 'Only matching charge metadata is attributed. Metadata on a Checkout session, customer, product or subscription is not automatically inherited by a charge.' : 'Without a filter these are ACCOUNT-WIDE, UNALLOCATED totals. They are shown here, excluded from automatic project findings, and must not be called project sales.'}</p>
     <label>Previous complete UTC days<input type="number" min={1} max={90} value={draft.windowDays} onChange={event => update({ windowDays: Number(event.target.value) })}/></label>
     <p>The cohort ends at today's 00:00 UTC. Today's activity is excluded. Refunds are the amounts reported now for charges created in that cohort; this is not a refund-date statement or accounting revenue.</p>
    </fieldset>
    <fieldset disabled={busy || !canConnect || uncertain}><legend>Original published GitHub releases</legend>
     <Button variant="secondary" onClick={() => void readChoices()}>Load published release choices</Button>
     <p>Choices come from your recent retained original publication records (up to 64). Already configured releases stay selectable. Authentication uses the workspace GitHub CLI connection, not proof of your personal GitHub identity.</p>
     {choices?.length === 0 && !releaseChoices.length && <p>No original published releases are available. Publish a reviewed launch artifact with the release publisher above first.</p>}
     {releaseChoices.map(row => <label className="check" key={row.publicationId}><input type="checkbox" checked={draft.publicationIds.includes(row.publicationId)} onChange={event => update({ publicationIds: event.target.checked ? [...draft.publicationIds, row.publicationId] : draft.publicationIds.filter(id => id !== row.publicationId) })}/>{row.label} · {row.repository} · {row.tag}</label>)}
     <p>These counters are release-asset downloads and optional reactions. They do not count unique people, page views, conversions or revenue. Configuration is bounded by 128 KiB of saved source descriptors.</p>
    </fieldset>
    <fieldset disabled={busy || !canConnect || uncertain}><legend>Read consent and continued learning</legend>
     <label className="check"><input type="checkbox" checked={draft.automaticReads} onChange={event => update({ automaticReads: event.target.checked })}/>Allow automatic bounded read-only refreshes through this project's saved observation route</label>
     <p>Completed sources are due every {view.refreshMinutes} minutes by default. A pending collection can continue on later observation passes. The route needs Imported metrics and connected commercial results selected, ordinary finding discussions enabled, and a remaining model-turn budget. No second scheduler is created.</p>
     <label className="check"><input type="checkbox" checked={sharing} onChange={event => setSharing(event.target.checked)}/>Share complete source-labelled aggregates with this project and its model context. Authorized project members may read them. Never share raw customer or payment details.</label>
     <div className="journey-actions"><Button disabled={!sharing || !validDraft || baselineReview || !!apiKey} onClick={() => void saveSources()}>Save source consent</Button><Button variant="secondary" disabled={!enabled} onClick={() => void saveSources(true)}>Pause all provider reads</Button><Button variant="secondary" onClick={discard}>Discard source draft</Button></div>
    </fieldset>
    {baselineReview && <p role="alert">Saved settings changed while your draft was open. Compare the saved account and results, then discard this draft and start from the current settings before saving.</p>}
   </details>
   <div className="journey-actions"><Button variant="secondary" onClick={() => setFindingSetup(old => !old)} aria-expanded={findingSetup}>Configure bounded findings</Button><Button disabled={!canAssist || uncertain || sourceDirty || !currentSnapshots.length} onClick={review}>Review commercial findings with Luci</Button></div>
   {findingSetup && <ObservationRouting projectId={projectId}/>}
   <details><summary>Collection window and limits</summary><p>Collections scan at most {number(view.collectionRecordLimit)} account records per window, with up to ten pages per bounded invocation. Continue preserves cursor and accepted pages across restarts. If a window reaches this operational bound, choose fewer days and save, then refresh; no partial total is published. Reads across pages are not an atomic provider snapshot.</p></details>
   {view.status.length > 0 && <section aria-label="Collection progress"><h3>Collection progress</h3><ul>{view.status.map(row => <li key={row.sourceKey}><strong>{row.sourceKey.startsWith('stripe:') ? config?.stripe?.name ?? 'Stripe receipts' : config?.releases.find(source => row.sourceKey === `github:${source.repository}:${source.releaseId}`)?.label ?? 'Retained release'}</strong><p>{row.state} · {number(row.records)} records scanned. Latest successful refresh: {when(row.lastSuccessAt)}. Last attempt: {when(row.lastAttemptAt)}.</p><div className="journey-actions"><Button variant="secondary" disabled={busy || !canConnect || uncertain || sourceDirty} onClick={() => void refresh(row.sourceKey)}>Continue or refresh this source</Button>{row.state !== 'ready' && <Button variant="secondary" disabled={busy || !canConnect || uncertain || sourceDirty} onClick={() => void refresh(row.sourceKey, true)}>Start this collection again</Button>}</div></li>)}</ul></section>}

  </>}
 </section>;
}
function Snapshot({ row, current }: { row: CommercialSnapshot; current: boolean }) {
 return <article className="commercial-snapshot"><h4>{row.label} · {current ? 'Latest completed' : 'Previous snapshot'}</h4><p>Snapshot observed: {when(row.observedAt)}. The collection began {when(row.startedAt)}; see latest successful refresh separately above.</p>
  {row.provider === 'stripe' ? <><p>{row.accountId} · {row.live ? 'LIVE' : 'TEST'} · {row.periodStart} to {row.periodEndExclusive} exclusive (UTC). {row.stripeFilter ? `CHARGE metadata ${row.stripeFilter.key}=${row.stripeFilter.value}` : 'ACCOUNT-WIDE · UNALLOCATED · excluded from project findings'}.</p><p>{number(row.records)} account charge records scanned; {number(row.matchedCharges)} matched. {number(row.payingCustomerReferences)} distinct non-null customer references among captured payments; guests and duplicate customer identities across accounts are not inferred.</p><div className="commercial-table"><table><caption>Captured payments by currency</caption><thead><tr><th>Currency</th><th>Captured payments</th><th>Captured amount</th><th>Refunded amount</th><th>Disputed payments</th><th>Without customer</th></tr></thead><tbody>{row.currencies.map(c => <tr key={c.currency}><th scope="row">{c.currency}</th><td>{number(c.capturedPayments)}</td><td>{stripeChargeAmount(c.capturedMinor, c.currency)}</td><td>{stripeChargeAmount(c.refundedMinor, c.currency)}</td><td>{number(c.disputedPayments)}</td><td>{number(c.paymentsWithoutCustomer)}</td></tr>)}</tbody></table></div>{!row.currencies.length && <p>No paid, captured, succeeded charges contributed in this cohort; failed or pending charge rows are not revenue.</p>}</> : <p>{row.repository} · {row.tag} · original release {row.releaseId}. {number(row.assetDownloads)} downloads across {number(row.records)} returned assets; reactions: {row.reactions === null ? 'not provided' : number(row.reactions)}.</p>}
  <details><summary>Source and measurement limits</summary><p>{row.semantics}</p>{row.provider === 'stripe' && <p>Raw Stripe integer minor units: {row.currencies.map(c => `${c.currency} captured ${c.capturedMinor}; refunded ${c.refundedMinor}`).join(' · ') || 'no captured currency rows'}. Amount formatting follows Stripe Charges currency rules; currencies are never converted or summed.</p>}<p>Retained source digest: <code>{row.digest}</code></p>{row.artifactId && <p>Launch artifact: {row.artifactId} · {row.artifactDigest}</p>}</details>
 </article>;
}
