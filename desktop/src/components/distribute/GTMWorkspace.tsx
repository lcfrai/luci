import { useEffect, useRef, useState } from 'react';
import { notifyProjectContextChanged, PROJECT_CONTEXT_CHANGED, type ProjectContextIdentity } from '../../api/projectContextEvents';
import { emptyGTM, gtmDraftValid, gtmWorkspaceApi, type CommercialModelInput, type GTMContent, type GTMView } from '../../api/gtmWorkspace';
import { distributeArtifactsApi, type ArtifactSummary } from '../../api/distributeArtifacts';
import { errorText } from '../../api/product';
import { useProjectDraft, useProjectDraftScope } from '../../hooks/useProjectDraft';
import { useUnsavedProjectDraft } from '../../hooks/useUnsavedProjectDraft';
import { useTeamWorkspace } from '../teams/TeamWorkspaceProvider';
import { Button } from '../fluid/components/button';

const key = () => `gtm-${crypto.randomUUID()}`;
const newId = (kind: string) => `${kind}-${crypto.randomUUID()}`;
const lines = (value: string) => value.split('\n').map(x => x.trim()).filter(Boolean);

export function GTMWorkspace({ projectId, canRead, canEdit }: { projectId: string; canRead: boolean; canEdit: boolean }) {
  const scope = useProjectDraftScope(projectId), team = useTeamWorkspace();
  return <ScopedGTM key={JSON.stringify([scope, projectId, canRead, team?.teamId, team?.revision, team?.accessBlocked])} projectId={projectId} scope={scope} canRead={canRead && !team?.accessBlocked} canEdit={canEdit && !team?.accessBlocked} />;
}

function ScopedGTM({ projectId, scope, canRead, canEdit }: { projectId: string; scope: string | null; canRead: boolean; canEdit: boolean }) {
  const [view, setView] = useState<GTMView | null>(null), [content, setContent] = useState<GTMContent>(emptyGTM);
  const [dirty, setDirty] = useState(false), [busy, setBusy] = useState(false), [loading, setLoading] = useState(true);
  const [error, setError] = useState(''), [notice, setNotice] = useState(''), [relinked, setRelinked] = useState(false);
  const [needsReview, setNeedsReview] = useState(false);
  const [artifacts, setArtifacts] = useState<ArtifactSummary[]>([]);
  const owner = useRef({ mounted: true, request: 0 });
  const saveKey = useRef('');
  const publishingContext = useRef(false);
  const base = view ? `${view.workspace?.revision ?? 0}:${view.workspace?.digest ?? ''}:${view.sourceDigest}` : null;
  const draft = useProjectDraft({ scope, kind: 'go-to-market', base: needsReview && base ? `review-required:${base}` : base, baseValue: view?.workspace?.content ?? emptyGTM(), value: content, dirty, valid: gtmDraftValid });
  useUnsavedProjectDraft(projectId, dirty, 'Go-to-market workspace', 'distribute', { retained: draft.retained && gtmDraftValid(content), discard: () => { setContent(view?.workspace?.content ?? emptyGTM()); setDirty(false); draft.clear(); } });
  useEffect(() => {
    owner.current.mounted = true;
    const controller = new AbortController(), request = ++owner.current.request;
    if (!canRead || !scope) { setLoading(false); return () => { controller.abort(); owner.current.mounted = false; }; }
    void gtmWorkspaceApi.get(projectId, controller.signal).then(value => {
      if (controller.signal.aborted || request !== owner.current.request) return;
      setView(value); setContent(value.workspace?.content ?? emptyGTM());
    }).catch(cause => { if (!controller.signal.aborted) setError(errorText(cause)); }).finally(() => { if (!controller.signal.aborted) setLoading(false); });
    return () => { controller.abort(); owner.current.mounted = false; owner.current.request++; };
  }, [projectId, canRead, scope]);
  useEffect(() => {
    const changed = (event: Event) => {
      const identity = (event as CustomEvent<ProjectContextIdentity>).detail;
      if (publishingContext.current || !identity || identity.projectId !== projectId || identity.scope !== scope) return;
      // Retain unsaved content. The existing review-current action fetches a
      // fresh baseline without silently overwriting a person's draft.
      setNeedsReview(true); setRelinked(false);
      setNotice('Saved project context changed. Refresh or review the current context before saving.');
    };
    window.addEventListener(PROJECT_CONTEXT_CHANGED, changed);
    return () => window.removeEventListener(PROJECT_CONTEXT_CHANGED, changed);
  }, [projectId, scope]);
  function edit(next: GTMContent) { setContent(next); setDirty(true); setNotice(''); saveKey.current = ''; }
  const editable = !!scope && canEdit && !!view && view.freshness !== 'unavailable' && view.freshness !== 'readback' && !loading && !busy && !draft.pending;
  const validDraft = gtmDraftValid(content);
  async function refresh() {
    if (dirty || draft.pending || busy) return;
    const request = ++owner.current.request; setBusy(true); setError('');
    try { const value = await gtmWorkspaceApi.get(projectId); if (!owner.current.mounted || request !== owner.current.request) return; setView(value); setContent(value.workspace?.content ?? emptyGTM()); setRelinked(false); setNeedsReview(false); }
    catch (cause) { if (owner.current.mounted && request === owner.current.request) setError(errorText(cause)); }
    finally { if (owner.current.mounted && request === owner.current.request) setBusy(false); }
  }
  async function save() {
    if (!editable || !view || !dirty || !validDraft || (view.freshness === 'outdated' || needsReview) && !relinked) return;
    const request = ++owner.current.request; setBusy(true); setError(''); saveKey.current ||= key();
    try { const value = await gtmWorkspaceApi.save(projectId, view, content, saveKey.current); if (!owner.current.mounted || request !== owner.current.request) return;
      setView(value); setContent(value.workspace?.content ?? content); setDirty(false); setRelinked(false); setNeedsReview(false); draft.clear(); saveKey.current = ''; setNotice('GTM revision saved.');
      publishingContext.current = true;
      try { notifyProjectContextChanged(scope, projectId); } finally { publishingContext.current = false; }
    } catch (cause) { if (owner.current.mounted && request === owner.current.request) setError(errorText(cause)); }
    finally { if (owner.current.mounted && request === owner.current.request) setBusy(false); }
  }
  async function reviewCurrent() {
    if (!dirty || !canRead || !scope || busy || draft.pending) return;
    const request = ++owner.current.request; setBusy(true); setError('');
    try { const value = await gtmWorkspaceApi.get(projectId); if (!owner.current.mounted || request !== owner.current.request) return;
      setView(value); setNeedsReview(true); setRelinked(false); saveKey.current = ''; setNotice('Current revision loaded. Your edits are retained; review before saving over it.');
    } catch (cause) { if (owner.current.mounted && request === owner.current.request) setError(errorText(cause)); }
    finally { if (owner.current.mounted && request === owner.current.request) setBusy(false); }
  }
  async function chooseArtifacts() {
    if (!canRead || busy) return;
    const request = ++owner.current.request; setBusy(true); setError('');
    try { const value = await distributeArtifactsApi.list(projectId); if (owner.current.mounted && request === owner.current.request && value.projectId === projectId) setArtifacts(value.artifacts); }
    catch (cause) { if (owner.current.mounted && request === owner.current.request) setError(errorText(cause)); }
    finally { if (owner.current.mounted && request === owner.current.request) setBusy(false); }
  }
  function restore() { const retained = draft.take(); if (retained) { setContent(retained.value); setDirty(true); saveKey.current = ''; setRelinked(false); setNeedsReview(retained.base !== base); if (retained.base !== base) setError('This recovered draft has an older baseline. Review current context and all links before saving.'); } }
  const segmentOptions = content.segments.map(r => ({ id: r.id, title: r.name || r.id }));
  const channelOptions = content.channels.map(r => ({ id: r.id, title: r.name || r.id }));
  return <section className="distribute-card" aria-label="Go-to-market workspace"><h2>Go to market</h2><p>Plan your audience, offer and launch. Track what you learn as you test demand.</p>
    <Button variant="secondary" disabled={!editable || dirty || needsReview} onClick={() => window.dispatchEvent(new CustomEvent('luci:orchestrator-open', { detail: { projectId, message: 'Draft or amend the structured go-to-market workspace using the saved project definition and GTM strategy. Preserve unrelated content and stable IDs. Keep customer, competitor, pricing and channel claims as assumptions; do not invent experiment results. Propose changes for my explicit review, without publishing or spending.' } }))}>Draft or amend with Luci</Button>
    {!canRead || !scope ? <p role="status">Current account and project read access are required.</p> : <>
      {loading && <p role="status">Loading GTM workspace…</p>}{error && <p role="alert" className="journey-error">{error}</p>}{notice && <p role="status">{notice}</p>}{draft.error && <p role="alert">{draft.error}</p>}
      {draft.pending && <div className="distribute-evidence"><p>Unsent GTM edits are available from this account and workspace.</p><Button disabled={busy || loading || !canEdit} onClick={restore}>Restore GTM draft</Button><Button variant="secondary" disabled={busy} onClick={draft.clear}>Discard recovered draft</Button></div>}
      {view && <><p>Saved revision {view.workspace?.revision ?? 0} · definition {view.source.specificationRevision} · Design {view.source.designRevision} · plan {view.source.planRevision}.</p>
        {(view.freshness === 'outdated' || needsReview) && <label><input type="checkbox" checked={relinked} disabled={!editable} onChange={e => { setRelinked(e.target.checked); setDirty(true); saveKey.current = ''; }} /> I reviewed the current saved revision and project context; save my draft against them.</label>}
        {view.freshness === 'readback' && <p>Saved response recovered. Refresh before making further changes.</p>}
        {view.freshness === 'unavailable' && <p role="status">Current project context is unavailable. Your saved strategy remains readable; refresh before editing.</p>}
        {!validDraft && <p role="status">Repair invalid or out-of-range values before saving. Whole minor-unit amounts and explicit currency are required for a commercial scenario.</p>}
        <fieldset disabled={!editable || view.freshness === 'readback'}><legend>Strategy and assumptions</legend><Area label="Positioning" value={content.positioning} onChange={positioning => edit({ ...content, positioning })} /><Area label="Offer" value={content.offer} onChange={offer => edit({ ...content, offer })} /><LinesArea label="Pricing assumptions, one per line" value={content.pricingAssumptions} onChange={pricingAssumptions => edit({ ...content, pricingAssumptions })} /></fieldset>
        <details><summary>Commercial scenario</summary><p>Compare the price, cost and demand you assume for one interval.</p><fieldset disabled={!editable || view.freshness === 'readback'}><label><input type="checkbox" checked={!!content.commercialModel} onChange={e => { const { commercialModel: _removed, ...rest } = content; edit(e.target.checked ? { ...content, commercialModel: { currency: 'USD', minorUnitDigits: 2, interval: 'month', pricePerUnitMinor: 0, variableCostPerUnitMinor: 0, fixedCostMinor: 0, units: 0, assumptions: [] } } : rest); }} /> Include a commercial scenario</label>{content.commercialModel && <CommercialInputs value={content.commercialModel} onChange={commercialModel => edit({ ...content, commercialModel })} />}</fieldset>{view.commercialModel && <div className="distribute-evidence"><h3>Saved scenario calculation</h3><p>{dirty ? 'These figures describe the saved revision, not your unsaved edits.' : 'Derived from the saved inputs.'}</p><p>Revenue: {money(view.commercialModel.revenueMinor, view.commercialModel.input)}. Total cost: {money(view.commercialModel.totalCostMinor, view.commercialModel.input)}. Operating profit: {money(view.commercialModel.operatingProfitMinor, view.commercialModel.input)}.</p><p>Break-even units: {view.commercialModel.breakEvenUnits ?? 'Not established'}.</p>{view.commercialModel.decisions.map((decision, i) => <p key={i}>{decision}</p>)}</div>}</details>
        <details open><summary>Customer segments ({content.segments.length})</summary>{content.segments.map((row, i) => <fieldset key={row.id} disabled={!editable}><legend>Customer segment {i + 1}</legend><Text label="Segment name" value={row.name} onChange={name => edit({ ...content, segments: content.segments.map(r => r.id === row.id ? { ...r, name } : r) })} /><Area label="Customer problem" value={row.problem} onChange={problem => edit({ ...content, segments: content.segments.map(r => r.id === row.id ? { ...r, problem } : r) })} /><Area label="Value for this segment" value={row.value} onChange={value => edit({ ...content, segments: content.segments.map(r => r.id === row.id ? { ...r, value } : r) })} /><Links label="Linked requirements" selected={row.requirementIds} options={view.requirements} onChange={requirementIds => edit({ ...content, segments: content.segments.map(r => r.id === row.id ? { ...r, requirementIds } : r) })} /><Button variant="secondary" onClick={() => edit({ ...content, segments: content.segments.filter(r => r.id !== row.id) })}>Remove segment {i + 1}</Button></fieldset>)}<Button disabled={!editable || content.segments.length >= 50} onClick={() => edit({ ...content, segments: [...content.segments, { id: newId('segment'), name: '', problem: '', value: '', requirementIds: [] }] })}>Add customer segment</Button></details>
        <details><summary>Competitors and alternatives ({content.competitors.length})</summary>{content.competitors.map((row, i) => <fieldset key={row.id} disabled={!editable}><legend>Alternative {i + 1}</legend>{(['name', 'alternative', 'difference'] as const).map(field => <Area key={field} label={{ name: 'Competitor or alternative', alternative: 'What customers use today', difference: 'Our intended difference' }[field]} value={row[field]} onChange={value => edit({ ...content, competitors: content.competitors.map(r => r.id === row.id ? { ...r, [field]: value } : r) })} />)}<Button variant="secondary" onClick={() => edit({ ...content, competitors: content.competitors.filter(r => r.id !== row.id) })}>Remove alternative {i + 1}</Button></fieldset>)}<Button disabled={!editable || content.competitors.length >= 50} onClick={() => edit({ ...content, competitors: [...content.competitors, { id: newId('competitor'), name: '', alternative: '', difference: '' }] })}>Add alternative</Button></details>
        <details><summary>Channels ({content.channels.length})</summary>{content.channels.map((row, i) => <fieldset key={row.id} disabled={!editable}><legend>Channel {i + 1}</legend><Text label="Channel name" value={row.name} onChange={name => edit({ ...content, channels: content.channels.map(r => r.id === row.id ? { ...r, name } : r) })} /><Area label="Channel hypothesis" value={row.hypothesis} onChange={hypothesis => edit({ ...content, channels: content.channels.map(r => r.id === row.id ? { ...r, hypothesis } : r) })} /><Links label="Channel customer segments" selected={row.segmentIds} options={segmentOptions} onChange={segmentIds => edit({ ...content, channels: content.channels.map(r => r.id === row.id ? { ...r, segmentIds } : r) })} /><Button variant="secondary" onClick={() => edit({ ...content, channels: content.channels.filter(r => r.id !== row.id) })}>Remove channel {i + 1}</Button></fieldset>)}<Button disabled={!editable || content.channels.length >= 50} onClick={() => edit({ ...content, channels: [...content.channels, { id: newId('channel'), name: '', hypothesis: '', segmentIds: [] }] })}>Add channel</Button></details>
        <details><summary>Launch milestones ({content.milestones.length})</summary>{content.milestones.map((row, i) => <fieldset key={row.id} disabled={!editable}><legend>Launch milestone {i + 1}</legend><Text label="Milestone title" value={row.title} onChange={title => edit({ ...content, milestones: content.milestones.map(r => r.id === row.id ? { ...r, title } : r) })} /><Area label="Milestone outcome" value={row.outcome} onChange={outcome => edit({ ...content, milestones: content.milestones.map(r => r.id === row.id ? { ...r, outcome } : r) })} /><label>Target date<input type="date" value={row.targetDate} onChange={e => edit({ ...content, milestones: content.milestones.map(r => r.id === row.id ? { ...r, targetDate: e.target.value } : r) })} /></label><Links label="Milestone prerequisites" selected={row.dependsOn} options={content.milestones.filter(r => r.id !== row.id).map(r => ({ id: r.id, title: r.title || r.id }))} onChange={dependsOn => edit({ ...content, milestones: content.milestones.map(r => r.id === row.id ? { ...r, dependsOn } : r) })} /><Button variant="secondary" onClick={() => edit({ ...content, milestones: content.milestones.filter(r => r.id !== row.id) })}>Remove milestone {i + 1}</Button></fieldset>)}<Button disabled={!editable || content.milestones.length >= 50} onClick={() => edit({ ...content, milestones: [...content.milestones, { id: newId('milestone'), title: '', outcome: '', targetDate: '', dependsOn: [] }] })}>Add launch milestone</Button></details>
        <details><summary>Experiments ({content.experiments.length})</summary>{content.experiments.map((row, i) => <fieldset key={row.id} disabled={!editable}><legend>Experiment {i + 1}</legend>{(['title', 'hypothesis', 'method', 'successCriterion', 'result'] as const).map(field => <Area key={field} label={{ title: 'Experiment title', hypothesis: 'Hypothesis', method: 'Method', successCriterion: 'Success criterion', result: 'Reported result' }[field]} value={row[field]} onChange={value => edit({ ...content, experiments: content.experiments.map(r => r.id === row.id ? { ...r, [field]: value } : r) })} />)}<Choice label="Experiment segment" value={row.segmentId} options={segmentOptions} onChange={segmentId => edit({ ...content, experiments: content.experiments.map(r => r.id === row.id ? { ...r, segmentId } : r) })} /><Choice label="Experiment channel" value={row.channelId} options={channelOptions} onChange={channelId => edit({ ...content, experiments: content.experiments.map(r => r.id === row.id ? { ...r, channelId } : r) })} /><label>Reported experiment status<select value={row.status} onChange={e => edit({ ...content, experiments: content.experiments.map(r => r.id === row.id ? { ...r, status: e.target.value as typeof row.status } : r) })}>{['planned', 'in-progress', 'recorded', 'retired'].map(x => <option key={x}>{x}</option>)}</select></label><Button variant="secondary" onClick={() => edit({ ...content, experiments: content.experiments.filter(r => r.id !== row.id) })}>Remove experiment {i + 1}</Button></fieldset>)}<Button disabled={!editable || content.experiments.length >= 50} onClick={() => edit({ ...content, experiments: [...content.experiments, { id: newId('experiment'), title: '', hypothesis: '', method: '', successCriterion: '', segmentId: '', channelId: '', status: 'planned', result: '' }] })}>Add experiment</Button></details>
        <details><summary>Linked project artifacts ({content.artifactLinks.length})</summary><Button variant="secondary" disabled={!canRead || busy} onClick={() => void chooseArtifacts()}>Choose saved artifacts</Button>{content.artifactLinks.map(link => <p key={link.id}>{link.id} · revision {link.revision}<Button variant="secondary" disabled={!editable} onClick={() => edit({ ...content, artifactLinks: content.artifactLinks.filter(r => r.id !== link.id) })}>Unlink {link.id}</Button></p>)}{artifacts.map(row => <label key={row.id}><input type="checkbox" disabled={!editable} checked={content.artifactLinks.some(r => r.id === row.id && r.revision === row.revision && r.digest === row.digest)} onChange={e => edit({ ...content, artifactLinks: [...content.artifactLinks.filter(r => r.id !== row.id), ...(e.target.checked ? [{ id: row.id, revision: row.revision, digest: row.digest }] : [])] })} />{row.title} · revision {row.revision}</label>)}</details>
        <div className="journey-actions"><Button disabled={!editable || !dirty || !validDraft || (view.freshness === 'outdated' || needsReview) && !relinked} onClick={() => void save()}>Save GTM revision</Button><Button variant="secondary" disabled={busy || !dirty} onClick={() => { setContent(view.workspace?.content ?? emptyGTM()); setDirty(false); setRelinked(false); setNeedsReview(false); draft.clear(); }}>Discard GTM edits</Button><Button variant="secondary" disabled={!dirty || busy || !canRead || !!draft.pending} onClick={() => void reviewCurrent()}>Review current context without replacing edits</Button><Button variant="secondary" disabled={busy || dirty || !!draft.pending} onClick={() => void refresh()}>Refresh GTM context</Button></div>
        <details><summary>Saved GTM revisions ({view.history.length})</summary>{view.history.map(row => <p key={row.revision}>Revision {row.revision} · {row.at}</p>)}</details>
      </>}
    </>}
  </section>;
}

function Text({ label, value, onChange }: { label: string; value: string; onChange(value: string): void }) { return <label>{label}<input value={value} maxLength={4000} onChange={e => onChange(e.target.value)} /></label>; }
function Area({ label, value, onChange }: { label: string; value: string; onChange(value: string): void }) { return <label>{label}<textarea value={value} maxLength={4000} rows={3} onChange={e => onChange(e.target.value)} /></label>; }
function LinesArea({ label, value, onChange }: { label: string; value: string[]; onChange(value: string[]): void }) {
  const [raw, setRaw] = useState(() => value.join('\n'));
  useEffect(() => { if (JSON.stringify(lines(raw)) !== JSON.stringify(value)) setRaw(value.join('\n')); }, [value, raw]);
  return <label>{label}<textarea value={raw} maxLength={4000} rows={3} onChange={e => { setRaw(e.target.value); onChange(lines(e.target.value)); }} /></label>;
}
function Links({ label, selected, options, onChange }: { label: string; selected: string[]; options: { id: string; title: string }[]; onChange(value: string[]): void }) {
  const all = [...options, ...selected.filter(id => !options.some(o => o.id === id)).map(id => ({ id, title: `Missing link: ${id}` }))];
  return <label>{label}<select multiple value={selected} onChange={e => onChange(Array.from(e.target.selectedOptions, option => option.value))}>{all.map(o => <option key={o.id} value={o.id}>{o.title}</option>)}</select></label>;
}
function Choice({ label, value, options, onChange }: { label: string; value: string; options: { id: string; title: string }[]; onChange(value: string): void }) { return <label>{label}<select value={value} onChange={e => onChange(e.target.value)}><option value="">Not linked</option>{value && !options.some(o => o.id === value) && <option value={value}>Missing link: {value}</option>}{options.map(o => <option key={o.id} value={o.id}>{o.title}</option>)}</select></label>; }
function CommercialInputs({ value, onChange }: { value: CommercialModelInput; onChange(value: CommercialModelInput): void }) {
  return <><Text label="Scenario currency (3-letter code)" value={value.currency} onChange={currency => onChange({ ...value, currency: currency.toUpperCase() })} />
    <label>Scenario interval<select value={value.interval} onChange={e => onChange({ ...value, interval: e.target.value as CommercialModelInput['interval'] })}><option value="month">Month</option><option value="year">Year</option><option value="one_time">One time</option></select></label>
    <MoneyInput label="Price per customer" value={value.pricePerUnitMinor} digits={value.minorUnitDigits} onChange={pricePerUnitMinor => onChange({ ...value, pricePerUnitMinor })} />
    <MoneyInput label="Variable cost per customer" value={value.variableCostPerUnitMinor} digits={value.minorUnitDigits} onChange={variableCostPerUnitMinor => onChange({ ...value, variableCostPerUnitMinor })} />
    <MoneyInput label="Fixed cost per period" value={value.fixedCostMinor} digits={value.minorUnitDigits} onChange={fixedCostMinor => onChange({ ...value, fixedCostMinor })} />
    <label>Customers or units per period<input type="number" min={0} max={1e6} step={1} value={Number.isFinite(value.units) ? value.units : ''} onChange={e => onChange({ ...value, units: e.target.value === '' ? NaN : Number(e.target.value) })} /></label>
    <details><summary>Advanced currency settings</summary><label>Currency decimal digits<input type="number" min={0} max={6} step={1} value={value.minorUnitDigits} onChange={e => onChange({ ...value, minorUnitDigits: e.target.value === '' ? NaN : Number(e.target.value) })} /></label><p>Choose the currency's decimal places before entering amounts.</p></details>
    <LinesArea label="Scenario assumptions, one per line" value={value.assumptions} onChange={assumptions => onChange({ ...value, assumptions })} />
  </>;
}

export function parseMoneyMinor(raw: string, digits: number): number | null {
  if (!Number.isInteger(digits) || digits < 0 || digits > 6 || raw.length > 32 || !/^\d+(?:\.\d*)?$/.test(raw)) return null;
  const [whole, fraction = ''] = raw.split('.');
  if (fraction.length > digits) return null;
  const amount = BigInt(whole) * (10n ** BigInt(digits)) + BigInt(fraction.padEnd(digits, '0') || '0');
  return amount <= 1_000_000_000n ? Number(amount) : null;
}
function decimalMoney(value: number, digits: number): string {
  if (!Number.isSafeInteger(value) || !Number.isInteger(digits) || digits < 0 || digits > 6) return '';
  const raw = String(Math.abs(value)).padStart(digits + 1, '0');
  return `${value < 0 ? '-' : ''}${digits === 0 ? raw : `${raw.slice(0, -digits)}.${raw.slice(-digits)}`}`;
}
function money(value: number, input: CommercialModelInput): string { return `${input.currency} ${decimalMoney(value, input.minorUnitDigits)}`; }
function MoneyInput({ label, value, digits, onChange }: { label: string; value: number; digits: number; onChange(value: number): void }) {
  const [raw, setRaw] = useState(() => decimalMoney(value, digits));
  const lastWritten = useRef<{ value: number; digits: number } | null>(null);
  useEffect(() => { if (lastWritten.current?.digits === digits && Object.is(lastWritten.current.value, value)) return; setRaw(decimalMoney(value, digits)); }, [value, digits]);
  return <label>{label}<input inputMode="decimal" value={raw} maxLength={32} onChange={e => {
    const parsed = parseMoneyMinor(e.target.value, digits), next = parsed ?? NaN;
    lastWritten.current = { value: next, digits }; setRaw(e.target.value); onChange(next);
  }} onBlur={() => { const parsed = parseMoneyMinor(raw, digits); if (parsed !== null) setRaw(decimalMoney(parsed, digits)); }} />{parseMoneyMinor(raw, digits) === null && <span>Enter a nonnegative amount with at most {Number.isInteger(digits) ? digits : 6} decimal places.</span>}</label>;
}
