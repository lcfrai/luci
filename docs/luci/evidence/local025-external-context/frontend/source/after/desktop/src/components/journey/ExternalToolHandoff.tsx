import { useEffect, useLayoutEffect, useRef, useState } from 'react';
import { productApi, errorText, type Brief, type ImplementationPlan } from '../../api/product';
import { definitionApi, type ProjectSpecification } from '../../api/definition';
import { designPlanApi, type DesignPlan } from '../../api/designPlan';
import { externalHandoffsApi, parseExternalReport, type ExternalHandoff, type ExternalHandoffSummary, type ExternalReturnReview } from '../../api/externalHandoffs';
import { useProjectAccess } from '../../hooks/useProjectAccess';
import { useProjectDraft, useProjectDraftScope } from '../../hooks/useProjectDraft';
import { useUnsavedProjectDraft } from '../../hooks/useUnsavedProjectDraft';
import { useCockpit } from '../../store/store';
import { Button } from '../ui/button';
import { WorkflowDialog } from '../common/WorkflowDialog';
import './external-tool-handoff.css';

type Facts = { brief: Brief; specification: ProjectSpecification; design: DesignPlan; plan: ImplementationPlan; sketchRevision: number };
const emptyReport = '{\n  "summary": "",\n  "checks": [],\n  "notes": ""\n}';
const reportTemplate = (handoff: ExternalHandoffSummary) => JSON.stringify({ projectId: handoff.projectId, handoffId: handoff.id, handoffDigest: handoff.digest, summary: '', checks: [], notes: '' }, null, 2);
type ReportDraft = { handoffId: string; handoffDigest: string; text: string; idempotencyKey: string; submittedReviewDigest?: string };
function reportDraftValid(value: unknown): value is ReportDraft {
  const draft = value as ReportDraft | null;
  return !!draft && typeof draft.handoffId === 'string' && draft.handoffId.length < 200 && /^[a-f0-9]{64}$/.test(draft.handoffDigest) &&
    typeof draft.text === 'string' && new TextEncoder().encode(draft.text).byteLength <= 128 * 1024 && typeof draft.idempotencyKey === 'string' && /^[A-Za-z0-9_-]{16,128}$/.test(draft.idempotencyKey) &&
    (draft.submittedReviewDigest === undefined || /^[a-f0-9]{64}$/.test(draft.submittedReviewDigest));
}
function download(blob: Blob, filename: string) {
  const url = URL.createObjectURL(blob), link = document.createElement('a');
  link.href = url; link.download = filename; document.body.appendChild(link); link.click(); link.remove();
  setTimeout(() => URL.revokeObjectURL(url), 30_000);
}
type Props = { projectId: string; onReviewPlan(): void };
export function ExternalToolHandoff({ projectId, onReviewPlan }: Props) {
  const scope = useProjectDraftScope(projectId);
  return <ProjectExternalToolHandoff key={JSON.stringify([productApi.baseUrl, scope, projectId])} projectId={projectId} onReviewPlan={onReviewPlan} scope={scope} />;
}
function ProjectExternalToolHandoff({ projectId, onReviewPlan, scope }: Props & { scope: string | null }) {
  const access = useProjectAccess(projectId), setLens = useCockpit(state => state.setLens);
  const [open, setOpen] = useState(false), [busy, setBusy] = useState(''), [error, setError] = useState(''), [notice, setNotice] = useState('');
  const [facts, setFacts] = useState<Facts | null>(null), [handoffs, setHandoffs] = useState<ExternalHandoffSummary[]>([]);
  const [taskIds, setTaskIds] = useState<string[]>([]), [selected, setSelected] = useState<ExternalHandoff | null>(null);
  const [review, setReview] = useState<ExternalReturnReview | null>(null), [reportText, setReportText] = useState(emptyReport), [consent, setConsent] = useState(false);
  const [reportKey, setReportKey] = useState<string>(() => crypto.randomUUID());
  const [submittedReviewDigest, setSubmittedReviewDigest] = useState<string>();
  const [uncertain, setUncertain] = useState(false), [loaded, setLoaded] = useState(false);
  const alive = useRef(true), generation = useRef(0), operation = useRef<object | null>(null);
  const freshRead = access.canRead && !access.loading && !access.error;
  const permissions = useRef({ read: freshRead, edit: access.canEdit });
  if (permissions.current.read && !freshRead || permissions.current.edit && !access.canEdit) generation.current++;
  permissions.current = { read: freshRead, edit: access.canEdit };
  const dirty = reportText !== (selected ? reportTemplate(selected) : emptyReport);
  const draft = useProjectDraft<ReportDraft>({ scope, kind: 'external-report', base: selected?.digest ?? null,
    value: selected ? { handoffId: selected.id, handoffDigest: selected.digest, text: reportText, idempotencyKey: reportKey, submittedReviewDigest } : null, dirty, valid: reportDraftValid });
  const discardDraft = () => { draft.clear(); setReportText(selected ? reportTemplate(selected) : emptyReport); setReview(null); setConsent(false); setReportKey(crypto.randomUUID()); setSubmittedReviewDigest(undefined); };
  useUnsavedProjectDraft(projectId, dirty, 'External tool report', 'develop', scope ? { retained: draft.retained, discard: discardDraft } : undefined);
  useLayoutEffect(() => { alive.current = true; return () => { alive.current = false; generation.current++; }; }, []);
  useEffect(() => {
    if (!access.canRead) { setFacts(null); setHandoffs([]); setSelected(null); setReview(null); setLoaded(false); setError(''); setNotice(''); }
    if (!access.canEdit) setConsent(false);
  }, [access.canRead, access.canEdit]);
  async function run(label: string, mutation: boolean, action: (current: () => boolean) => Promise<void>) {
    if (operation.current || !permissions.current.read || mutation && !permissions.current.edit) return;
    const owner = {}, ticket = generation.current;
    operation.current = owner; setBusy(label); setError(''); setNotice('');
    const current = () => alive.current && generation.current === ticket && permissions.current.read && (!mutation || permissions.current.edit);
    try { await action(current); }
    catch (cause) { if (current()) setError(errorText(cause)); if (mutation && alive.current) setUncertain(true); }
    finally { if (operation.current === owner) operation.current = null; if (alive.current) { setBusy(''); if (mutation && !current()) { setUncertain(true); setConsent(false); } } }
  }
  async function refresh() {
    await run('Reading saved context…', false, async current => {
      const [brief, specification, design, plan, sketch, list] = await Promise.all([
        productApi.brief(projectId), definitionApi.get(projectId), designPlanApi.get(projectId), productApi.plan(projectId), productApi.design(projectId), externalHandoffsApi.list(projectId),
      ]);
      if (!current()) return;
      if ([brief, specification, design, plan, sketch].some(value => value.projectId !== projectId)) throw new Error('The saved context belongs to another project. Refresh this project.');
      setFacts({ brief, specification, design, plan, sketchRevision: sketch.revision }); setHandoffs(list); setLoaded(true);
      setTaskIds(previous => previous.filter(id => plan.tasks.some(task => task.id === id)));
      setConsent(false); setReview(null);
      if (selected) {
        const saved = await externalHandoffsApi.get(projectId, selected.id);
        if (!current()) return;
        setSelected(saved);
        if (saved.returns.some(result => result.idempotencyKey === reportKey)) {
          draft.clear(); setReportText(reportTemplate(saved)); setReportKey(crypto.randomUUID()); setSubmittedReviewDigest(undefined);
          setNotice('The original report was already recorded as unverified. Its saved receipt is available below.');
        }
      }
      setUncertain(false);
    });
  }
  useEffect(() => { if (open && access.canRead && !loaded) void refresh(); }, [open, access.canRead, loaded]);
  const specificationReady = !!facts && facts.specification.status === 'approved' && facts.specification.briefRevision === facts.brief.revision;
  const designReady = !!facts && specificationReady && facts.design.status === 'approved' && facts.design.briefRevision === facts.brief.revision && facts.design.specificationRevision === facts.specification.revision && facts.design.sketchRevision === facts.sketchRevision;
  const planReady = !!facts && facts.plan.revision > 0 && facts.plan.briefRevision === facts.brief.revision && facts.plan.specificationRevision === facts.specification.revision;
  const ready = specificationReady && designReady && planReady && taskIds.length > 0;
  async function create() {
    if (!facts || !ready || uncertain || draft.pending) return;
    const chosen = { projectId, briefRevision: facts.brief.revision, specificationRevision: facts.specification.revision, designPlanRevision: facts.design.revision, planRevision: facts.plan.revision, taskIds };
    const key = crypto.randomUUID();
    await run('Capturing approved context…', true, async current => {
      const value = await externalHandoffsApi.create(projectId, chosen, key);
      if (!current()) return;
      setSelected(value); setReportText(reportTemplate(value)); setReportKey(crypto.randomUUID()); setReview(null); setHandoffs(list => [value, ...list.filter(row => row.id !== value.id)]);
      setNotice('Context captured. Download the bundle and open its HANDOFF.md in your coding tool.');
    });
  }
  async function choose(id: string) {
    if (dirty || draft.pending) return;
    await run('Reading handoff…', false, async current => {
      const value = await externalHandoffsApi.get(projectId, id);
      if (current()) { setSelected(value); setReportText(reportTemplate(value)); setReportKey(crypto.randomUUID()); setReview(null); setConsent(false); }
    });
  }
  async function reviewReturn() {
    if (!selected) return;
    // Validate the report before reading source, but never execute its commands.
    try { parseExternalReport(reportText, selected); } catch (cause) { setError(errorText(cause)); return; }
    await run('Comparing returned work…', false, async current => {
      const value = await externalHandoffsApi.review(projectId, selected);
      if (current()) { setReview(value); setConsent(false); setUncertain(false); }
    });
  }
  async function recordReturn() {
    if (!selected || !review?.contextCurrent || !consent || uncertain) return;
    const report = parseExternalReport(reportText, selected), captured = selected;
    const key = reportKey, exactReviewDigest = submittedReviewDigest ?? review.reviewDigest;
    setSubmittedReviewDigest(exactReviewDigest);
    await run('Recording reviewed report…', true, async current => {
      const value = await externalHandoffsApi.record(projectId, captured, exactReviewDigest, report, key);
      if (!current()) return;
      const retained = [...captured.returns.filter(row => row.id !== value.id), value];
      setSelected({ ...captured, returns: retained, returnCount: retained.length });
      setReportText(reportTemplate(captured)); setReview(null); setConsent(false);
      draft.clear(); setReportKey(crypto.randomUUID()); setSubmittedReviewDigest(undefined);
      setNotice('Report recorded as unverified. Review the source changes and run project checks before preview or deployment.');
    });
  }
  return <section className="external-handoff-entry" aria-label="External coding tools">
    <div><h2>Share context with another tool</h2><p>For work outside Luci, export approved context and bring the result back for review. Luci’s integrated build controls remain above.</p></div>
    <Button variant="outline" disabled={!freshRead} onClick={() => setOpen(true)}>Context export and returns</Button>
    {open && <WorkflowDialog readAccess={access} title="Context export and returns" description="An optional handoff for work in another tool. Capture the saved scope, then review returned code and evidence in Luci." busy={!!busy} onClose={() => setOpen(false)}>
      <div className="external-handoff-body">
        {access.message && <p role="status" className="journey-muted">{access.message}</p>}
        {busy && <p role="status">{busy}</p>}
        {error && <p role="alert" className="journey-error">{error}</p>}
        {notice && <p role="status" className="journey-success">{notice}</p>}
        {uncertain && <p role="alert" className="journey-warning">The last action needs reconciliation. Refresh saved handoffs and review the current source before submitting another action.</p>}
        {draft.error && <p role="alert" className="journey-warning">{draft.error}</p>}
        {dirty && draft.retained && <p className="journey-muted">Report draft saved in this browser for this project. It is not yet recorded as evidence.</p>}
        {draft.pending && <section aria-label="Saved external report draft"><p>A report draft is saved in this browser for this project. Restore it with its original handoff before reviewing the result.</p><div className="journey-actions"><Button variant="outline" disabled={!access.canEdit || !!busy} onClick={() => void run('Restoring report draft…', false, async current => {
          const saved = draft.pending!;
          const value = await externalHandoffsApi.get(projectId, saved.value.handoffId);
          if (!current() || !permissions.current.edit) return;
          if (value.digest !== saved.value.handoffDigest) throw new Error('The saved report belongs to a different captured context. Keep the draft and refresh the handoff.');
          draft.take(); setSelected(value); setReportText(saved.value.text); setReportKey(saved.value.idempotencyKey); setSubmittedReviewDigest(saved.value.submittedReviewDigest); setReview(null); setConsent(false); setUncertain(true);
        })}>Restore report draft</Button><Button variant="ghost" disabled={!access.canEdit || !!busy} onClick={discardDraft}>Discard saved report draft</Button></div></section>}
        <div className="journey-actions"><Button variant="outline" disabled={!!busy || !access.canRead} onClick={() => void refresh()}>Refresh saved handoffs</Button><Button variant="ghost" disabled={!!busy} onClick={() => setOpen(false)}>Close handoffs</Button></div>
        <section aria-label="Capture approved context">
          <h3>1. Choose the approved work</h3><p className="journey-muted">Only saved, approved context is captured. Save any browser drafts first. Creating a handoff does not start a tool or change project files.</p>
          {facts && <>
            <ul className="external-handoff-readiness">
              <li>Definition {facts.specification.revision} · {specificationReady ? 'Approved and current' : 'Save and approve the current definition'}</li>
              <li>Design {facts.design.revision} · {designReady ? 'Approved and current' : 'Save and approve the current design'}</li>
              <li>Plan {facts.plan.revision} · {planReady ? 'Saved against current requirements' : 'Save the plan against current requirements'}</li>
            </ul>
            {(!specificationReady || !designReady || !planReady) && <div className="journey-actions">
              {!specificationReady && <Button variant="outline" onClick={() => { setOpen(false); setLens('discover'); }}>Review definition</Button>}
              {!designReady && <Button variant="outline" onClick={() => { setOpen(false); setLens('design'); }}>Review design</Button>}
              {!planReady && <Button variant="outline" onClick={() => { setOpen(false); onReviewPlan(); }}>Review project plan below</Button>}
            </div>}
            <fieldset className="external-handoff-tasks" disabled={!access.canEdit || !!busy || dirty}>
              <legend>Tasks to hand over</legend>
              {facts.plan.tasks.map(task => <label key={task.id}><input type="checkbox" checked={taskIds.includes(task.id)} onChange={event => setTaskIds(ids => event.target.checked ? [...ids, task.id] : ids.filter(id => id !== task.id))} /><span>{task.title}<small>{task.acceptance || 'Acceptance comes from its linked requirements.'}</small></span></label>)}
              {!facts.plan.tasks.length && <p>Add and save implementation tasks in the project plan.</p>}
            </fieldset>
            <Button disabled={!access.canEdit || !!busy || !ready || uncertain || dirty || !!draft.pending} onClick={() => void create()}>Capture selected context</Button>
          </>}
        </section>
        {handoffs.length > 0 && <label className="luci-field">Saved handoff<select value={selected?.id ?? ''} disabled={!!busy || dirty || !!draft.pending} onChange={event => void choose(event.target.value)}><option value="" disabled>Choose captured context</option>{handoffs.map(row => <option key={row.id} value={row.id}>{new Date(row.createdAt).toLocaleString()} · {row.implementation.taskTitles.join(', ')} · {row.id}</option>)}</select></label>}
        {selected && <>
          <section aria-label="Captured handoff"><h3>2. Take this context to your tool</h3>
            <p>{selected.implementation.taskTitles.join(' · ')}</p>
            <p className="journey-muted">Definition {selected.implementation.specificationRevision} · Design {selected.implementation.designPlanRevision} · Plan {selected.implementation.planRevision}. Source {selected.implementation.sourceCommit ? selected.implementation.sourceCommit.slice(0, 12) : 'without a commit'}{selected.implementation.sourceDirty ? ' with uncommitted changes at capture' : ''}.</p>
            <p>Extract the ZIP beside your working checkout. Read HANDOFF.md with your tool before building. It includes approved requirements, decisions, design, tasks and the return report format. Keep the manifest with the result.</p>
            <div className="journey-actions"><Button disabled={!!busy || !access.canRead} onClick={() => void run('Preparing download…', false, async current => { const value = await externalHandoffsApi.bundle(projectId, selected.id); if (current()) { download(value, `luci-${selected.id}.zip`); setNotice('Bundle download started. Check that the ZIP was saved before handing it to your tool.'); } })}>Download context ZIP</Button>
              <Button variant="outline" disabled={!!busy || !access.canRead} onClick={() => void run('Copying instructions…', false, async current => { if (!navigator.clipboard?.writeText) throw new Error('Clipboard access is unavailable. Select the instructions below and copy them.'); await navigator.clipboard.writeText(selected.instructions); if (current()) setNotice('Handoff instructions copied.'); })}>Copy handoff instructions</Button></div>
            <details><summary>Instructions and captured files</summary><pre tabIndex={0}>{selected.instructions}</pre><ul>{selected.files.map(file => <li key={file.path}><code>{file.path}</code> · {file.bytes.toLocaleString()} bytes</li>)}</ul><p className="journey-muted">Manifest identity <code>{selected.digest}</code>. A newer amendment needs a new captured handoff.</p></details>
          </section>
          <section aria-label="Return work for review"><h3>3. Bring the result back</h3>
            <p>First bring the changed code into this Luci project using your normal Git workflow. Import the tool’s JSON report, then compare it with the source Luci can observe. The report does not apply code or mark checks as passed.</p>
            <label className="luci-field">Import report JSON<input type="file" accept="application/json,.json" disabled={!access.canEdit || !!busy || !!draft.pending} onChange={event => { const file = event.target.files?.[0]; event.target.value = ''; if (!file) return; void run('Reading report file…', false, async current => { if (file.size > 128 * 1024) throw new Error('Choose a report smaller than 128 KiB.'); const text = await file.text(); parseExternalReport(text, selected); if (current() && permissions.current.edit) { setReportText(text); setReportKey(crypto.randomUUID()); setSubmittedReviewDigest(undefined); setReview(null); setConsent(false); } }); }} /></label>
            <label className="luci-field">External report JSON<textarea rows={8} spellCheck={false} value={reportText} disabled={!access.canEdit || !!busy || !!draft.pending} onChange={event => { setReportText(event.target.value); setReportKey(crypto.randomUUID()); setSubmittedReviewDigest(undefined); setReview(null); setConsent(false); }} /></label>
            <div className="journey-actions"><Button variant="outline" disabled={!access.canRead || !!busy || !!draft.pending} onClick={() => void reviewReturn()}>Compare returned work</Button>{dirty && <Button variant="ghost" disabled={!!busy} onClick={discardDraft}>Discard report draft</Button>}</div>
            {review && <div className="external-handoff-review"><h4>Observed source changes</h4><p role="status" className={review.contextCurrent ? 'journey-muted' : 'journey-warning'}>{review.contextMessage || (review.contextCurrent ? 'Captured context still matches the current approved project.' : 'Context changed. Review the amendment and capture a new handoff before recording this result.')}</p>
              <p>{review.changedFiles.length} changed files · source {review.source.commit ? review.source.commit.slice(0, 12) : 'without a commit'}{review.source.dirty ? ' · uncommitted changes' : ''}</p>
              <ul>{review.changedFiles.map(file => <li key={file.path}><code>{file.path}</code> · {file.status}</li>)}</ul>
              <p className="journey-muted">The tool’s check statements remain unverified claims. Luci has not run these commands.</p>
              <label className="external-handoff-consent"><input type="checkbox" checked={consent} disabled={!access.canEdit || !!busy || !review.contextCurrent || uncertain} onChange={event => setConsent(event.target.checked)} />I reviewed these changes and want to retain this report as unverified evidence.</label>
              <Button disabled={!access.canEdit || !!busy || !review.contextCurrent || !consent || uncertain} onClick={() => void recordReturn()}>Record reviewed report</Button>
            </div>}
            {!!selected.returns.length && <details><summary>Retained reports · {selected.returns.length}</summary>{selected.returns.map(report => <article key={report.id}><h4>{report.report.summary}</h4><p>Unverified report · {new Date(report.createdAt).toLocaleString()}</p>{report.report.checks.map((check, index) => <p key={index}><code>{check.command}</code> · Reported: {check.outcome}</p>)}<p>{report.report.notes}</p></article>)}</details>}
            <div className="journey-actions"><Button variant="outline" disabled={!!busy} onClick={() => { setOpen(false); onReviewPlan(); }}>Review source and project checks</Button><Button variant="ghost" disabled={!!busy} onClick={() => { setOpen(false); setLens('deploy'); }}>Preview and deployment</Button></div>
          </section>
        </>}
      </div>
    </WorkflowDialog>}
  </section>;
}
