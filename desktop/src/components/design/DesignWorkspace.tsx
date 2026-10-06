import { lazy, Suspense, useEffect, useId, useRef, useState } from "react";
import { notifyProjectContextChanged, PROJECT_CONTEXT_EDIT, takeProjectContextEdit } from "../../api/projectContextEvents";
import { designPlanApi, type DesignPlan, type DesignScreen } from "../../api/designPlan";
import { definitionApi, type ProjectSpecification } from "../../api/definition";
import { productApi, errorText, type Release } from "../../api/product";
import { isGatewayHostname, isolatedBrowserURL, platformApi } from "../../api/platform";
import { prepareBrowserAccess } from "../../api/browserAccess";
import { DEFAULT_BASE_URL } from "../../api/client";
import { useCockpit } from "../../store/store";
import { useProjectAccess } from "../../hooks/useProjectAccess";
import { useProjectDraft, useProjectDraftScope } from "../../hooks/useProjectDraft";
import { designPlanDraftValid } from "../../hooks/projectDraftValidation";
import { DraftRecovery } from "../common/DraftRecovery";
import { useUnsavedProjectDraft } from "../../hooks/useUnsavedProjectDraft";
import { useSetup } from "../setup/SetupProvider";
import { ContextStorage } from "../journey/ContextStorage";
import { JourneyHeader } from "../journey/shared";
import { Button } from "../fluid/components/button";
import { TabsSubtle, TabsSubtleItem, TabsSubtlePanel } from "../fluid/components/tabs-subtle";
import "../definition/definition.css";
import "./design.css";
const Sketch = lazy(() => import("../journey/DesignEditor"));
type View = "screens" | "sketch" | "prototype" | "handoff";
const views: [View, string][] = [["screens", "Screens and states"], ["sketch", "Sketch"], ["prototype", "Prototype"], ["handoff", "Implementation handoff"]];
const id = (prefix: string) => `${prefix}-${crypto.randomUUID().slice(0, 12)}`;

export default function DesignWorkspace({ projectId }: { projectId: string }) {
  const scope = useProjectDraftScope(projectId);
  const [view, setView] = useState<View>("screens");
  return <ProjectDesignWorkspace key={`${scope}:${projectId}`} projectId={projectId} scope={scope} view={view} setView={setView} />;
}
function ProjectDesignWorkspace({ projectId, scope, view, setView }: { projectId: string; scope: string | null; view: View; setView(view: View): void }) {
  const access = useProjectAccess(projectId);
  const { canRead, canEdit, canExecute } = access;
  const [plan, setPlan] = useState<DesignPlan | null>(null);
  const [spec, setSpec] = useState<ProjectSpecification | null>(null);
  const [baseRevision, setBaseRevision] = useState(0);
  const savedPlan = useRef<DesignPlan | null>(null);
  const [sketchDirty, setSketchDirty] = useState(false);
  const [sketchBusy, setSketchBusy] = useState(false);
  const [pendingView, setPendingView] = useState<View | null>(null);
  const [dirty, setDirty] = useState(false);
  const draftBase = plan && spec ? JSON.stringify({ revision: baseRevision, briefRevision: spec.briefRevision, specificationRevision: spec.revision, sketchRevision: plan.sketchRevision }) : null;
  const draft = useProjectDraft({ scope, kind: "design-plan", base: draftBase, value: plan, dirty, valid: designPlanDraftValid });
  useUnsavedProjectDraft(projectId, dirty, "Design", "design", scope ? { retained: draft.retained, discard: () => { draft.clear(); setPlan(savedPlan.current); setDirty(false); } } : undefined);
  const [busy, setBusy] = useState(false);
  const [refreshingSketch, setRefreshingSketch] = useState(false);
  const [pending, setPending] = useState<"" | "save" | "approve">("");
  const [error, setError] = useState("");
  const [notice, setNotice] = useState("");
  const [releases, setReleases] = useState<Release[]>([]);
  const [historyBusy, setHistoryBusy] = useState(false);
  const [historyError, setHistoryError] = useState("");
  const [prototypeBusy, setPrototypeBusy] = useState(false);
  const [prototypeError, setPrototypeError] = useState("");
  const [preview, setPreview] = useState("");
  const mayExecute = useRef(canExecute); mayExecute.current = canExecute;
  useEffect(() => { if (!access.loading && !canExecute) setPreview(""); }, [canExecute, access.loading]);
  const [width, setWidth] = useState(1440);
  const alive = useRef(true), working = useRef(false), draftVersion = useRef(0), generation = useRef(0);
  const sketchRead = useRef(0);
  const historyRead = useRef(0), prototypeRead = useRef(0), prototypeWorking = useRef(false);
  const tabsId = useId(), selectedIndex = views.findIndex(([key]) => key === view);
  const setup = useSetup();
  const setLens = useCockpit((s) => s.setLens);
  const sketchDrafts = useCockpit(state => state.unsavedProjectDrafts);
  const sketchGuards = Object.values(sketchDrafts).filter(item => item.projectId === projectId && item.label === "Sketch");
  const sketchRetained = sketchGuards.length > 0 && sketchGuards.every(item => item.retained);
  function leaveSketch(next: View, discard: boolean) {
    if (discard) sketchGuards.forEach(item => item.discard?.());
    setPendingView(null); setSketchDirty(false); activateView(next);
  }
  async function load() {
    if (!canRead) return;
    const requestGeneration = generation.current;
    sketchRead.current++;
    setRefreshingSketch(false);
    const [saved, definition] = await Promise.all([designPlanApi.get(projectId), definitionApi.get(projectId)]);
    if (!alive.current || requestGeneration !== generation.current) return;
    savedPlan.current = saved; setPlan(saved); setBaseRevision(saved.revision); setSpec(definition); setDirty(false); setError("");
  }
  async function loadPrototypeHistory() {
    const requestGeneration = generation.current, request = ++historyRead.current;
    setHistoryBusy(true); setHistoryError("");
    try {
      const history = await productApi.prototypeReleases(projectId);
      if (alive.current && requestGeneration === generation.current && request === historyRead.current) setReleases(history.releases);
    } catch (cause) {
      if (alive.current && requestGeneration === generation.current && request === historyRead.current) setHistoryError(errorText(cause));
    } finally {
      if (alive.current && requestGeneration === generation.current && request === historyRead.current) setHistoryBusy(false);
    }
  }
  useEffect(() => {
    alive.current = true; generation.current++; working.current = false;
    setPlan(null); setSpec(null); setDirty(false); setError(""); setNotice(""); setReleases([]);
    setBusy(false); setPending(""); setPreview(""); setPrototypeBusy(false); setPrototypeError(""); prototypeWorking.current = false;
    const requestGeneration = generation.current;
    void load().catch((e) => { if (alive.current && requestGeneration === generation.current) setError(errorText(e)); });
    if (canRead) void loadPrototypeHistory();
    return () => { alive.current = false; generation.current++; };
  }, [projectId, canRead || !!plan]);
  const patch = (value: Partial<DesignPlan>) => { draftVersion.current++; if (plan) setPlan({ ...plan, ...value, status: "draft", digest: "" }); setDirty(true); setNotice(""); };
  const screenChange = (screenId: string, value: Partial<DesignScreen>) => plan && patch({ screens: plan.screens.map((s) => s.id === screenId ? { ...s, ...value } : s) });
  async function save() {
    if (!canEdit || !plan || !spec || draft.pending || working.current || refreshingSketch) return;
    const requestGeneration = generation.current;
    working.current = true; setBusy(true); setPending("save"); setError("");
    try { const saved = await designPlanApi.save(projectId, plan, baseRevision, spec.briefRevision, spec.revision); if (alive.current && requestGeneration === generation.current) { draft.clear(); savedPlan.current = saved; setPlan(saved); setBaseRevision(saved.revision); setDirty(false); setNotice(`Design plan ${saved.revision} saved. Implementation context updated.`); notifyProjectContextChanged(scope, projectId); } }
    catch (e) { if (alive.current && requestGeneration === generation.current) setError(errorText(e)); }
    finally { if (alive.current && requestGeneration === generation.current) { working.current = false; setBusy(false); setPending(""); } }
  }
  async function approve() {
    if (!canEdit || !plan || dirty || draft.pending || working.current || refreshingSketch) return;
    const requestGeneration = generation.current;
    working.current = true; setBusy(true); setPending("approve"); setError("");
    try { const saved = await designPlanApi.approve(projectId, plan.revision); if (alive.current && requestGeneration === generation.current) { savedPlan.current = saved; setPlan(saved); setBaseRevision(saved.revision); setNotice(`Design handoff ${saved.revision} approved. Its linked definition and sketch are recorded.`); notifyProjectContextChanged(scope, projectId); } }
    catch (e) { if (alive.current && requestGeneration === generation.current) setError(errorText(e)); }
    finally { if (alive.current && requestGeneration === generation.current) { working.current = false; setBusy(false); setPending(""); } }
  }
  async function openPrototype(separateTab = false) {
    const release = selectedPrototype;
    if (!canExecute || !release || prototypeWorking.current || historyBusy || historyError) return;
    // Open synchronously from the click so popup blockers do not swallow an async handoff.
    const tab = separateTab ? window.open("about:blank", "_blank") : null;
    if (separateTab && !tab) { setPrototypeError("Allow a new tab in your browser, then try again."); return; }
    if (tab) tab.opener = null;
    let transferred = false;
    const requestGeneration = generation.current, request = ++prototypeRead.current;
    prototypeWorking.current = true; setPrototypeBusy(true); setPrototypeError("");
    try {
      let target = release.url;
      if (release.target === "application") {
        const current = await platformApi.project(projectId);
        if (!alive.current || requestGeneration !== generation.current) return;
        const rows = current.project.deployments.filter((row) => row.id === release.id);
        const deployment = rows[0];
        if (current.busy || current.project.projectId !== projectId || rows.length !== 1 ||
          deployment.projectId !== projectId || deployment.provider !== "docker" || deployment.status !== "ready" ||
          deployment.commit !== release.commit || deployment.configurationRevision !== (release.configurationRevision ?? 0))
          throw new Error("This prototype's deployment changed. Refresh it in Deploy and choose the current release.");
        if (deployment.browserConnectionRequired) {
          const connection = await platformApi.prepareBrowser(projectId, "application", release.id);
          if (connection.projectId !== projectId || connection.kind !== "application" || connection.resourceId !== release.id)
            throw new Error("Luci could not verify this prototype's browser connection. Refresh the application in Deploy and try again.");
          target = connection.url;
        } else target = deployment.url || "";
        const address = isolatedBrowserURL(target);
        if (!address)
          throw new Error("Luci could not verify this prototype's browser connection. Refresh the application in Deploy and try again.");
        target = address;
      } else if (setup?.workspace?.mode === "cloud" && release.target === "local") target = (await productApi.prepareWorkspacePreview(projectId, release.id)).url;
      const url = new URL(target);
      const controlOrigin = new URL(window.__LUCI_API__ || DEFAULT_BASE_URL).origin;
      if (url.origin === window.location.origin || url.origin === controlOrigin || !((url.protocol === "http:" && (["127.0.0.1", "localhost"].includes(url.hostname) || isGatewayHostname(url.hostname))) || url.protocol === "https:") || url.username || url.password) throw new Error("The prototype requires a verified separate application origin.");
      if (!alive.current || !mayExecute.current || requestGeneration !== generation.current || request !== prototypeRead.current) return;
      const browserURL = await prepareBrowserAccess(projectId, url.toString());
      if (alive.current && mayExecute.current && requestGeneration === generation.current && request === prototypeRead.current) {
        if (tab) { tab.location.href = browserURL; transferred = true; }
        else setPreview(browserURL);
      }
    } catch (e) { if (alive.current && requestGeneration === generation.current && request === prototypeRead.current) { setPreview(""); setPrototypeError(errorText(e)); } }
    finally { if (tab && !transferred) tab.close(); if (alive.current && requestGeneration === generation.current && request === prototypeRead.current) { prototypeWorking.current = false; setPrototypeBusy(false); } }
  }
  const activateView = (next: View) => {
    setView(next);
    // Leaving the independent sketch adapter refreshes its saved revision;
    // edited screen/asset/token drafts stay intact.
    if (view === "sketch" && next !== "sketch") {
      const version = draftVersion.current, requestGeneration = generation.current, request = ++sketchRead.current;
      setRefreshingSketch(true);
      void designPlanApi.get(projectId)
        .then((current) => { if (alive.current && requestGeneration === generation.current && request === sketchRead.current && !dirty && version === draftVersion.current) { savedPlan.current = current; setPlan(current); setBaseRevision(current.revision); } })
        .catch((e) => { if (alive.current && requestGeneration === generation.current && request === sketchRead.current) setError(errorText(e)); })
        .finally(() => { if (alive.current && requestGeneration === generation.current && request === sketchRead.current) setRefreshingSketch(false); });
    }
    if (next === "prototype") {
      void loadPrototypeHistory();
    }
  };
  const selectView = (next: View) => {
    if (sketchBusy) return;
    if (view === "sketch" && next !== "sketch" && sketchDirty) { setPendingView(next); return; }
    activateView(next);
  };
  useEffect(() => { if (!sketchDirty) setPendingView(null); }, [sketchDirty]);
  useEffect(() => {
    if (!plan || !canRead || sketchBusy) return;
    const edit = () => { if (takeProjectContextEdit(scope, projectId, "design")) selectView("screens"); };
    edit(); window.addEventListener(PROJECT_CONTEXT_EDIT, edit);
    return () => window.removeEventListener(PROJECT_CONTEXT_EDIT, edit);
  }, [scope, projectId, !!plan, canRead, view, sketchDirty, sketchBusy]);
  const linked = new Set(plan?.screens.flatMap((s) => s.requirementIds) || []);
  const prototypes = releases.filter((r) => r.projectId === projectId && r.status === "ready");
  const selectedPrototype = prototypes.find((r) => r.id === plan?.prototypeReleaseId);
  return <section className="journey-page design-workspace">
    <JourneyHeader title="Design the experience" subtitle="Plan the screens and their states, sketch ideas, inspect a working prototype and carry a precise handoff into development.">
      {plan && <span className="journey-revision" role="status">{dirty ? "Unsaved changes" : `${plan.status} · Revision ${plan.revision}`}</span>}
    </JourneyHeader>
    {access.message && <p role="status" className="journey-muted">{access.message}</p>}
    {error && <div role="alert" className="journey-error">{error} <Button type="button" variant="secondary" disabled={busy} onClick={() => { draft.offerRecovery(); void load().catch((e) => setError(errorText(e))); }}>Reload saved design</Button>{dirty && <p>Your draft is retained. Reload to compare it with saved work and choose what to recover.</p>}</div>}
    <DraftRecovery draft={draft} label="Design plan" base={draftBase} saved={savedPlan.current} busy={busy} readOnly={!canEdit} onDiscard={() => { setPlan(savedPlan.current); setDirty(false); }} onRestore={value => { if (plan) { draftVersion.current++; setPlan({ ...value, projectId, revision: plan.revision, briefRevision: plan.briefRevision, specificationRevision: plan.specificationRevision, sketchRevision: plan.sketchRevision, status: "draft", digest: "" }); setDirty(true); } }} />
    {!plan && !error && <p role="status">Loading your design workspace…</p>}
    {plan && spec && canRead && <>
      <TabsSubtle className="definition-tabs" aria-label="Design views" idPrefix={tabsId} selectedIndex={selectedIndex} onSelect={index => selectView(views[index][0])}>
        {views.map(([, label], index) => <TabsSubtleItem key={label} index={index} label={label} aria-disabled={sketchBusy} />)}
      </TabsSubtle>
      {pendingView && <div className="journey-error" role="alert"><p>Your sketch has unsaved changes. Save the drawing before leaving, keep its browser draft, or discard the edits.</p><div className="journey-actions"><Button type="button" variant="secondary" onClick={() => setPendingView(null)}>Keep editing</Button><Button type="button" variant="secondary" disabled={sketchBusy} onClick={() => leaveSketch(pendingView, true)}>Discard sketch and continue</Button>{sketchRetained && <Button type="button" disabled={sketchBusy} onClick={() => leaveSketch(pendingView, false)}>Keep sketch draft and continue</Button>}</div></div>}
      {plan.status === "outdated" && <div className="journey-error" role="status">The definition or sketch has changed. Review the screen plan and save a new handoff before approving.</div>}
      <TabsSubtlePanel idPrefix={tabsId} index={0} selectedIndex={selectedIndex}>
      {view === "screens" && <div className="design-plan-layout"><div className="design-plan-content"><fieldset className="definition-fields" disabled={!canEdit || busy || !!draft.pending}>
        {!plan.screens.length && <div className="design-empty"><h2>Give the solution a screen plan</h2><p>Capture the primary screens, empty/loading/error states and responsive targets before developing the interface.</p><Button type="button" variant="primary" onClick={() => patch({ screens: [{ id: id("screen"), name: "Home", route: "/", purpose: spec.purpose, states: ["Initial visit", "Loading", "No saved content", "Successful use", "Error and recovery"], widths: [390, 1440], requirementIds: spec.requirements.map((r) => r.id), accessibility: "Keyboard navigation, labelled controls, visible focus and readable contrast" }] })}>Start a screen plan from my definition</Button><p className="journey-muted">This creates an editable planning outline, not a generated interface or verified implementation.</p></div>}
        {plan.screens.map((s,index) => <fieldset className="definition-record" key={s.id}><legend>Screen {index + 1}</legend>
          <div className="design-pair"><label className="luci-field">Screen name<input value={s.name} onChange={(e) => screenChange(s.id,{ name:e.target.value })} /></label><label className="luci-field">Route<input value={s.route} placeholder="/dashboard" onChange={(e) => screenChange(s.id,{ route:e.target.value })} /></label></div>
          <label className="luci-field">What this screen helps someone do<textarea rows={2} value={s.purpose} onChange={(e) => screenChange(s.id,{ purpose:e.target.value })} /></label>
          <label className="luci-field">States and recovery<textarea rows={4} aria-label="States and recovery" aria-describedby={`design-${s.id}-states-help`} value={s.states.join("\n")} onChange={(e) => screenChange(s.id,{ states:e.target.value.split("\n") })} /><small id={`design-${s.id}-states-help`}>One state per line. Include first use, loading, empty, successful, error and permission states that apply.</small></label>
          <label className="luci-field">Responsive widths<input aria-label="Responsive widths" aria-describedby={`design-${s.id}-widths-help`} value={s.widths.join(", ")} onChange={(e) => screenChange(s.id,{ widths:e.target.value.split(",").map((v) => Number(v.trim())).filter((v) => v > 0) })} /><small id={`design-${s.id}-widths-help`}>Widths in pixels, such as 390, 768, 1440.</small></label>
          <label className="luci-field">Accessibility and interaction<textarea rows={2} value={s.accessibility} onChange={(e) => screenChange(s.id,{ accessibility:e.target.value })} /></label>
          <fieldset className="definition-links"><legend>Requirements carried by this screen</legend>{spec.requirements.map((r) => <label key={r.id}><input type="checkbox" checked={s.requirementIds.includes(r.id)} onChange={(e) => screenChange(s.id,{ requirementIds:e.target.checked ? [...s.requirementIds,r.id] : s.requirementIds.filter((id) => id !== r.id) })} />{r.id} · {r.title}</label>)}</fieldset>
          <Button type="button" variant="secondary" onClick={() => patch({ screens:plan.screens.filter((old) => old.id !== s.id) })}>Remove screen {index + 1}</Button>
        </fieldset>)}
        <Button type="button" variant="secondary" onClick={() => patch({ screens:[...plan.screens,{ id:id("screen"),name:"",route:"",purpose:"",states:[],widths:[390,1440],requirementIds:[],accessibility:"" }] })}>Add screen</Button>
      </fieldset></div><aside className="design-definition"><h2>Definition coverage</h2><p className="journey-muted">Definition {spec.revision} · {spec.status}</p><ul>{spec.requirements.map((r) => <li key={r.id}><strong>{r.id}</strong><p>{r.title}</p><small>{linked.has(r.id) ? "Linked to a screen" : "No screen link yet"}</small></li>)}</ul><p className="journey-muted">Some backend requirements do not need a screen. Coverage records intention; checks record actual behavior.</p></aside></div>}
      </TabsSubtlePanel>
      <TabsSubtlePanel idPrefix={tabsId} index={1} selectedIndex={selectedIndex}>
        {view === "sketch" && <div className="design-sketch"><p className="journey-muted">Sketch is a local drawing surface. Save the drawing here; screen plans and reviewed handoffs remain separate.</p><Suspense fallback={<p role="status">Opening sketch tools…</p>}><Sketch projectId={projectId} onDirtyChange={setSketchDirty} onBusyChange={setSketchBusy} /></Suspense></div>}
      </TabsSubtlePanel>
      <TabsSubtlePanel idPrefix={tabsId} index={2} selectedIndex={selectedIndex}>
      {view === "prototype" && <section className="design-prototype"><h2>Inspect an actual running prototype</h2><p>Choose a verified application release built from your project. Its source commit will be recorded in the design handoff.</p>
        {historyBusy && <p role="status">Refreshing prototype history… Your saved design remains available.</p>}
        {historyError && <div role="alert" className="journey-error">Prototype history could not be loaded: {historyError} <Button type="button" variant="secondary" disabled={historyBusy} onClick={() => void loadPrototypeHistory()}>Retry prototype history</Button><p>Your saved design and definition remain available.</p></div>}
        {prototypeError && <p role="alert" className="journey-error">{prototypeError}</p>}
        <div className="design-prototype-tools"><label className="luci-field">Prototype release<select value={plan.prototypeReleaseId} disabled={!canEdit || busy || !!draft.pending || prototypeBusy || historyBusy || !!historyError} onChange={(e) => { patch({ prototypeReleaseId:e.target.value });setPreview(""); setPrototypeError(""); }}><option value="">Choose a verified release</option>{plan.prototypeReleaseId && !selectedPrototype && <option value={plan.prototypeReleaseId} disabled>Saved prototype unavailable</option>}{prototypes.map((r) => <option key={r.id} value={r.id}>{r.target === "application" ? "Application" : r.target === "local" ? "Local preview" : r.target === "github-pages" ? "GitHub Pages" : "Cloud"} · {r.commit.slice(0,8)} · {r.id}</option>)}</select></label><label className="luci-field">Preview width<select value={width} onChange={(e) => setWidth(Number(e.target.value))}><option value={390}>Phone · 390 px</option><option value={768}>Tablet · 768 px</option><option value={1440}>Desktop · 1440 px</option></select></label><Button type="button" variant="secondary" loading={prototypeBusy} disabled={!canExecute || prototypeBusy || historyBusy || !!historyError || !selectedPrototype} onClick={() => void openPrototype()}>Open prototype</Button></div>
        {!historyBusy && !historyError && plan.prototypeReleaseId && !selectedPrototype && <div className="design-empty"><p role="status">This prototype has stopped or is no longer available. Choose a running release or open Deploy to refresh the application.</p><Button type="button" variant="secondary" onClick={() => setLens("deploy")}>Go to Deploy</Button></div>}
        {!historyBusy && !historyError && !prototypes.length && !plan.prototypeReleaseId && <div className="design-empty"><h3>No verified prototype yet</h3><p>Build and preview the project's interface in Develop, then return here to inspect its real behavior.</p><Button type="button" variant="secondary" onClick={() => setLens("develop")}>Go to Develop</Button></div>}
        {!canExecute && <p className="journey-muted">Opening a live prototype requires editing and execution access because the application can change project data.</p>}
        {preview && (canExecute || access.loading) && selectedPrototype && <div className="design-prototype-viewport"><iframe title="Design prototype" src={preview} style={{ width }} sandbox="allow-scripts allow-forms allow-same-origin" /><Button type="button" variant="secondary" disabled={!canExecute || prototypeBusy} onClick={() => void openPrototype(true)}>Open prototype in a separate tab</Button></div>}
      </section>}
      </TabsSubtlePanel>
      <TabsSubtlePanel idPrefix={tabsId} index={3} selectedIndex={selectedIndex}>
      {view === "handoff" && <section className="design-handoff"><h2>A usable implementation handoff</h2><p>Agents read the screen plan, states, tokens, assets and notes from the project's versioned context. Approval binds the saved definition and sketch revisions.</p><fieldset disabled={!canEdit || busy || !!draft.pending} className="definition-fields">
        <label className="luci-field">Implementation notes<textarea rows={5} value={plan.implementationNotes} onChange={(e) => patch({ implementationNotes:e.target.value })} placeholder="Navigation, reusable components, interaction rules, animation and anything a builder must not invent." /></label>
        <h3>Design tokens</h3>{plan.tokens.map((token,i) => <div className="design-token" key={i}><label className="luci-field">Token name<input value={token.name} onChange={(e) => patch({ tokens:plan.tokens.map((t,n) => n === i ? {...t,name:e.target.value} : t) })} /></label><label className="luci-field">Value<input value={token.value} onChange={(e) => patch({ tokens:plan.tokens.map((t,n) => n === i ? {...t,value:e.target.value} : t) })} /></label><label className="luci-field">Purpose<input value={token.purpose} onChange={(e) => patch({ tokens:plan.tokens.map((t,n) => n === i ? {...t,purpose:e.target.value} : t) })} /></label><Button type="button" variant="secondary" onClick={() => patch({ tokens:plan.tokens.filter((_,n) => n !== i) })}>Remove token {i + 1}</Button></div>)}<Button type="button" variant="secondary" onClick={() => patch({ tokens:[...plan.tokens,{name:"",value:"",purpose:""}] })}>Add token</Button>
        <h3>Assets and provenance</h3>{plan.assets.map((asset,i) => <fieldset className="definition-record" key={asset.id}><legend>Asset {i + 1}</legend>{([['path','Project file path'],['purpose','Purpose'],['licence','Licence and source']] as const).map(([key,label]) => <label className="luci-field" key={key}>{label}<input value={asset[key]} onChange={(e) => patch({ assets:plan.assets.map((a,n) => n === i ? {...a,[key]:e.target.value} : a) })} /></label>)}<Button type="button" variant="secondary" onClick={() => patch({ assets:plan.assets.filter((a) => a.id !== asset.id) })}>Remove asset {i + 1}</Button></fieldset>)}<Button type="button" variant="secondary" onClick={() => patch({ assets:[...plan.assets,{id:id("asset"),path:"",purpose:"",licence:""}] })}>Add asset</Button>
      </fieldset><dl className="design-provenance"><dt>Definition</dt><dd>{plan.specificationRevision}</dd><dt>Brief</dt><dd>{plan.briefRevision}</dd><dt>Sketch</dt><dd>{plan.sketchRevision}</dd><dt>Prototype source</dt><dd>{plan.prototypeCommit || "No prototype attached"}</dd></dl></section>}
      </TabsSubtlePanel>
      {view !== "sketch" && <footer className="definition-footer"><div className="journey-actions">
        {dirty || plan.status === "outdated" || !plan.revision
          ? <Button type="button" loading={pending === "save"} disabled={!canEdit || busy || !!draft.pending || refreshingSketch || (!dirty && plan.status !== "outdated")} onClick={() => void save()}>Save design plan</Button>
          : plan.status === "approved"
            ? <Button type="button" disabled={!canEdit || busy || !!draft.pending || refreshingSketch} onClick={() => setLens("develop")}>Continue to Develop</Button>
            : <Button type="button" loading={pending === "approve"} disabled={!canEdit || busy || !!draft.pending || refreshingSketch} onClick={() => void approve()}>Approve design handoff</Button>}
      </div><p role="status" className="journey-muted">{refreshingSketch ? "Checking the saved sketch revision…" : notice || `${plan.screens.length} screens · ${linked.size} linked requirements`}</p></footer>}
    </>}
    <ContextStorage projectId={projectId} />
  </section>;
}
