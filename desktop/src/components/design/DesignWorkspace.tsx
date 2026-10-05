import { lazy, Suspense, useEffect, useRef, useState } from "react";
import { designPlanApi, type DesignPlan, type DesignScreen } from "../../api/designPlan";
import { definitionApi, type ProjectSpecification } from "../../api/definition";
import { productApi, errorText, type Release } from "../../api/product";
import { isGatewayHostname } from "../../api/platform";
import { useCockpit } from "../../store/store";
import { useSetup } from "../setup/SetupProvider";
import { JourneyHeader } from "../journey/shared";
import "../definition/definition.css";
import "./design.css";
const Sketch = lazy(() => import("../journey/DesignEditor"));
type View = "screens" | "sketch" | "prototype" | "handoff";
const id = (prefix: string) => `${prefix}-${crypto.randomUUID().slice(0, 12)}`;

export default function DesignWorkspace({ projectId }: { projectId: string }) {
  const [plan, setPlan] = useState<DesignPlan | null>(null);
  const [spec, setSpec] = useState<ProjectSpecification | null>(null);
  const [baseRevision, setBaseRevision] = useState(0);
  const [view, setView] = useState<View>("screens");
  const [dirty, setDirty] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const [notice, setNotice] = useState("");
  const [releases, setReleases] = useState<Release[]>([]);
  const [preview, setPreview] = useState("");
  const [width, setWidth] = useState(1440);
  const alive = useRef(true), working = useRef(false), draftVersion = useRef(0), generation = useRef(0);
  const setup = useSetup();
  const setLens = useCockpit((s) => s.setLens);
  async function load() {
    const requestGeneration = generation.current;
    const [saved, definition, history] = await Promise.all([designPlanApi.get(projectId), definitionApi.get(projectId), productApi.releases(projectId)]);
    if (!alive.current || requestGeneration !== generation.current) return;
    setPlan(saved); setBaseRevision(saved.revision); setSpec(definition); setReleases(history.releases); setDirty(false); setError("");
  }
  useEffect(() => { alive.current = true; generation.current++; void load().catch((e) => { if (alive.current) setError(errorText(e)); }); return () => { alive.current = false; generation.current++; }; }, [projectId]);
  const patch = (value: Partial<DesignPlan>) => { draftVersion.current++; if (plan) setPlan({ ...plan, ...value, status: "draft", digest: "" }); setDirty(true); setNotice(""); };
  const screenChange = (screenId: string, value: Partial<DesignScreen>) => plan && patch({ screens: plan.screens.map((s) => s.id === screenId ? { ...s, ...value } : s) });
  async function save() {
    if (!plan || !spec || working.current) return;
    working.current = true; setBusy(true); setError("");
    try { const saved = await designPlanApi.save(projectId, plan, baseRevision, spec.briefRevision, spec.revision); if (alive.current) { setPlan(saved); setBaseRevision(saved.revision); setDirty(false); setNotice(`Design plan ${saved.revision} saved. Implementation context updated.`); } }
    catch (e) { if (alive.current) setError(errorText(e)); }
    finally { working.current = false; if (alive.current) setBusy(false); }
  }
  async function approve() {
    if (!plan || dirty || working.current) return;
    working.current = true; setBusy(true); setError("");
    try { const saved = await designPlanApi.approve(projectId, plan.revision); if (alive.current) { setPlan(saved); setBaseRevision(saved.revision); setNotice(`Design handoff ${saved.revision} approved. Its linked definition and sketch are recorded.`); } }
    catch (e) { if (alive.current) setError(errorText(e)); }
    finally { working.current = false; if (alive.current) setBusy(false); }
  }
  async function openPrototype() {
    const release = releases.find((r) => r.id === plan?.prototypeReleaseId && r.status === "ready");
    if (!release || working.current) return;
    working.current = true; setBusy(true); setError("");
    try {
      let target = release.url;
      if (setup?.workspace?.mode === "cloud" && release.target === "local") target = (await productApi.prepareWorkspacePreview(projectId, release.id)).url;
      const url = new URL(target);
      if (url.origin === window.location.origin || !((url.protocol === "http:" && (["127.0.0.1", "localhost"].includes(url.hostname) || isGatewayHostname(url.hostname))) || url.protocol === "https:") || url.username || url.password) throw new Error("The prototype requires a verified separate application origin.");
      if (alive.current) setPreview(url.toString());
    } catch (e) { if (alive.current) setError(errorText(e)); }
    finally { working.current = false; if (alive.current) setBusy(false); }
  }
  const selectView = (next: View) => {
    setView(next);
    // Leaving the independent sketch adapter refreshes its saved revision;
    // edited screen/asset/token drafts stay intact.
    if (view === "sketch") {
      const version = draftVersion.current, requestGeneration = generation.current;
      void designPlanApi.get(projectId).then((current) => { if (alive.current && requestGeneration === generation.current && !dirty && version === draftVersion.current) { setPlan(current); setBaseRevision(current.revision); } }).catch((e) => { if (alive.current && requestGeneration === generation.current) setError(errorText(e)); });
    }
    if (next === "prototype") void productApi.releases(projectId).then((r) => { if (alive.current) setReleases(r.releases); }).catch((e) => setError(errorText(e)));
  };
  const linked = new Set(plan?.screens.flatMap((s) => s.requirementIds) || []);
  return <section className="journey-page design-workspace">
    <JourneyHeader title="Design the experience" subtitle="Plan the screens and their states, sketch ideas, inspect a working prototype and carry a precise handoff into development.">
      {plan && <span className="journey-revision" role="status">{dirty ? "Unsaved changes" : `${plan.status} · Revision ${plan.revision}`}</span>}
    </JourneyHeader>
    {error && <div role="alert" className="journey-error">{error} <button className="luci-button" disabled={busy} onClick={() => void load().catch((e) => setError(errorText(e)))}>Reload saved design</button>{dirty && <p>Your draft is retained. Reloading replaces it with saved work.</p>}</div>}
    {!plan && !error && <p role="status">Loading your design workspace…</p>}
    {plan && spec && <>
      <nav className="definition-tabs" aria-label="Design views">{([['screens','Screens and states'],['sketch','Sketch'],['prototype','Prototype'],['handoff','Implementation handoff']] as [View,string][]).map(([key,label]) => <button key={key} aria-current={view === key ? "page" : undefined} onClick={() => selectView(key)}>{label}</button>)}</nav>
      {plan.status === "outdated" && <div className="journey-error" role="status">The definition or sketch has changed. Review the screen plan and save a new handoff before approving.</div>}
      {view === "screens" && <div className="design-plan-layout"><div className="design-plan-content"><fieldset className="definition-fields" disabled={busy}>
        {!plan.screens.length && <div className="design-empty"><h2>Give the solution a screen plan</h2><p>Capture the primary screens, empty/loading/error states and responsive targets before developing the interface.</p><button className="luci-button luci-primary" onClick={() => patch({ screens: [{ id: id("screen"), name: "Home", route: "/", purpose: spec.purpose, states: ["Initial visit", "Loading", "No saved content", "Successful use", "Error and recovery"], widths: [390, 1440], requirementIds: spec.requirements.map((r) => r.id), accessibility: "Keyboard navigation, labelled controls, visible focus and readable contrast" }] })}>Start a screen plan from my definition</button><p className="journey-muted">This creates an editable planning outline, not a generated interface or verified implementation.</p></div>}
        {plan.screens.map((s,index) => <fieldset className="definition-record" key={s.id}><legend>Screen {index + 1}</legend>
          <div className="design-pair"><label className="luci-field">Screen name<input value={s.name} onChange={(e) => screenChange(s.id,{ name:e.target.value })} /></label><label className="luci-field">Route<input value={s.route} placeholder="/dashboard" onChange={(e) => screenChange(s.id,{ route:e.target.value })} /></label></div>
          <label className="luci-field">What this screen helps someone do<textarea rows={2} value={s.purpose} onChange={(e) => screenChange(s.id,{ purpose:e.target.value })} /></label>
          <label className="luci-field">States and recovery<textarea rows={4} aria-label="States and recovery" aria-describedby={`design-${s.id}-states-help`} value={s.states.join("\n")} onChange={(e) => screenChange(s.id,{ states:e.target.value.split("\n") })} /><small id={`design-${s.id}-states-help`}>One state per line. Include first use, loading, empty, successful, error and permission states that apply.</small></label>
          <label className="luci-field">Responsive widths<input aria-label="Responsive widths" aria-describedby={`design-${s.id}-widths-help`} value={s.widths.join(", ")} onChange={(e) => screenChange(s.id,{ widths:e.target.value.split(",").map((v) => Number(v.trim())).filter((v) => v > 0) })} /><small id={`design-${s.id}-widths-help`}>Widths in pixels, such as 390, 768, 1440.</small></label>
          <label className="luci-field">Accessibility and interaction<textarea rows={2} value={s.accessibility} onChange={(e) => screenChange(s.id,{ accessibility:e.target.value })} /></label>
          <fieldset className="definition-links"><legend>Requirements carried by this screen</legend>{spec.requirements.map((r) => <label key={r.id}><input type="checkbox" checked={s.requirementIds.includes(r.id)} onChange={(e) => screenChange(s.id,{ requirementIds:e.target.checked ? [...s.requirementIds,r.id] : s.requirementIds.filter((id) => id !== r.id) })} />{r.id} · {r.title}</label>)}</fieldset>
          <button className="luci-button" onClick={() => patch({ screens:plan.screens.filter((old) => old.id !== s.id) })}>Remove screen {index + 1}</button>
        </fieldset>)}
        <button className="luci-button" onClick={() => patch({ screens:[...plan.screens,{ id:id("screen"),name:"",route:"",purpose:"",states:[],widths:[390,1440],requirementIds:[],accessibility:"" }] })}>Add screen</button>
      </fieldset></div><aside className="design-definition"><h2>Definition coverage</h2><p className="journey-muted">Definition {spec.revision} · {spec.status}</p><ul>{spec.requirements.map((r) => <li key={r.id}><strong>{r.id}</strong><p>{r.title}</p><small>{linked.has(r.id) ? "Linked to a screen" : "No screen link yet"}</small></li>)}</ul><p className="journey-muted">Some backend requirements do not need a screen. Coverage records intention; checks record actual behavior.</p></aside></div>}
      {view === "sketch" && <div className="design-sketch"><p className="journey-muted">Sketch is a local drawing surface. Save the drawing here; screen plans and reviewed handoffs remain separate.</p><Suspense fallback={<p role="status">Opening sketch tools…</p>}><Sketch projectId={projectId} /></Suspense></div>}
      {view === "prototype" && <section className="design-prototype"><h2>Inspect an actual running prototype</h2><p>Choose a verified application release built from your project. Its source commit will be recorded in the design handoff.</p>
        <div className="design-prototype-tools"><label className="luci-field">Prototype release<select value={plan.prototypeReleaseId} disabled={busy} onChange={(e) => { patch({ prototypeReleaseId:e.target.value });setPreview(""); }}><option value="">Choose a verified release</option>{releases.filter((r) => r.status === "ready").map((r) => <option key={r.id} value={r.id}>{r.target} · {r.commit.slice(0,8)} · {r.id}</option>)}</select></label><label className="luci-field">Preview width<select value={width} onChange={(e) => setWidth(Number(e.target.value))}><option value={390}>Phone · 390 px</option><option value={768}>Tablet · 768 px</option><option value={1440}>Desktop · 1440 px</option></select></label><button className="luci-button" disabled={busy || !plan.prototypeReleaseId} onClick={() => void openPrototype()}>Open prototype</button></div>
        {!releases.some((r) => r.status === "ready") && <div className="design-empty"><h3>No verified prototype yet</h3><p>Build and preview the project's interface in Develop, then return here to inspect its real behavior.</p><button className="luci-button" onClick={() => setLens("develop")}>Go to Develop</button></div>}
        {preview && <div className="design-prototype-viewport"><iframe title="Design prototype" src={preview} style={{ width }} sandbox="allow-scripts allow-forms allow-same-origin" /><a href={preview} target="_blank" rel="noreferrer">Open prototype in a separate tab</a></div>}
      </section>}
      {view === "handoff" && <section className="design-handoff"><h2>A usable implementation handoff</h2><p>Agents read the screen plan, states, tokens, assets and notes from the project's versioned context. Approval binds the saved definition and sketch revisions.</p><fieldset disabled={busy} className="definition-fields">
        <label className="luci-field">Implementation notes<textarea rows={5} value={plan.implementationNotes} onChange={(e) => patch({ implementationNotes:e.target.value })} placeholder="Navigation, reusable components, interaction rules, animation and anything a builder must not invent." /></label>
        <h3>Design tokens</h3>{plan.tokens.map((token,i) => <div className="design-token" key={i}><label className="luci-field">Token name<input value={token.name} onChange={(e) => patch({ tokens:plan.tokens.map((t,n) => n === i ? {...t,name:e.target.value} : t) })} /></label><label className="luci-field">Value<input value={token.value} onChange={(e) => patch({ tokens:plan.tokens.map((t,n) => n === i ? {...t,value:e.target.value} : t) })} /></label><label className="luci-field">Purpose<input value={token.purpose} onChange={(e) => patch({ tokens:plan.tokens.map((t,n) => n === i ? {...t,purpose:e.target.value} : t) })} /></label><button className="luci-button" onClick={() => patch({ tokens:plan.tokens.filter((_,n) => n !== i) })}>Remove token {i + 1}</button></div>)}<button className="luci-button" onClick={() => patch({ tokens:[...plan.tokens,{name:"",value:"",purpose:""}] })}>Add token</button>
        <h3>Assets and provenance</h3>{plan.assets.map((asset,i) => <fieldset className="definition-record" key={asset.id}><legend>Asset {i + 1}</legend>{([['path','Project file path'],['purpose','Purpose'],['licence','Licence and source']] as const).map(([key,label]) => <label className="luci-field" key={key}>{label}<input value={asset[key]} onChange={(e) => patch({ assets:plan.assets.map((a,n) => n === i ? {...a,[key]:e.target.value} : a) })} /></label>)}<button className="luci-button" onClick={() => patch({ assets:plan.assets.filter((a) => a.id !== asset.id) })}>Remove asset {i + 1}</button></fieldset>)}<button className="luci-button" onClick={() => patch({ assets:[...plan.assets,{id:id("asset"),path:"",purpose:"",licence:""}] })}>Add asset</button>
      </fieldset><dl className="design-provenance"><dt>Definition</dt><dd>{plan.specificationRevision}</dd><dt>Brief</dt><dd>{plan.briefRevision}</dd><dt>Sketch</dt><dd>{plan.sketchRevision}</dd><dt>Prototype source</dt><dd>{plan.prototypeCommit || "No prototype attached"}</dd></dl></section>}
      {view !== "sketch" && <footer className="definition-footer"><div className="journey-actions"><button className="luci-button luci-primary" disabled={busy || (!dirty && plan.status !== "outdated")} onClick={() => void save()}>{busy ? "Working…" : "Save design plan"}</button><button className="luci-button" disabled={busy || dirty || !plan.revision || plan.status === "approved"} onClick={() => void approve()}>Approve design handoff</button>{plan.status === "approved" && !dirty && <button className="luci-button" onClick={() => setLens("develop")}>Continue to Develop</button>}</div><p role="status" className="journey-muted">{notice || `${plan.screens.length} screens · ${linked.size} linked requirements`}</p></footer>}
    </>}
  </section>;
}
