import { useCallback, useEffect, useId, useRef, useState } from "react";
import { dataFunctionsApi, type FunctionAction, type FunctionAuth, type FunctionCatalog, type FunctionFile, type FunctionResponse, type FunctionSource, type FunctionSummary } from "../../api/dataFunctions";
import { errorText } from "../../api/product";
import { useCockpit } from "../../store/store";
import { Button } from "../fluid/components/button";
import { TabsSubtle, TabsSubtleItem, TabsSubtlePanel } from "../fluid/components/tabs-subtle";
import "./data-functions.css";

const starter = `Deno.serve(async (request: Request) => {
  const input = await request.json();
  return Response.json({ message: "Hello from your function", input });
});
`;
const validName = (name: string) => /^[a-z][a-z0-9-]{0,62}$/.test(name) && !["main", "hello"].includes(name);
const authLabel = (mode: FunctionAuth) => mode === "authenticated" ? "Signed-in app users" : "Project key";

export function DataFunctions(props: { projectId: string; ready: boolean; locked?: boolean }) {
  return <ProjectFunctions key={props.projectId} {...props} />;
}
function ProjectFunctions({ projectId, ready, locked = false }: { projectId: string; ready: boolean; locked?: boolean }) {
  const alive = useRef(true), flight = useRef(false), generation = useRef(0);
  const [catalog, setCatalog] = useState<FunctionCatalog | null>(null);
  const [selected, setSelected] = useState("");
  const [creating, setCreating] = useState(false), [name, setName] = useState("");
  const [pending, setPending] = useState(""), [error, setError] = useState("");
  const [dirty, setDirty] = useState(false), [destination, setDestination] = useState<string | null>(null);
  const accept = useCallback((value: FunctionCatalog) => {
    if (value.projectId !== projectId || !Number.isInteger(value.revision) || value.revision < 0 || !Array.isArray(value.functions))
      throw new Error("Luci could not verify this project's functions. Refresh before retrying.");
    setCatalog(value);
    setSelected(current => current || value.functions.find(item => !item.builtin)?.name || value.functions[0]?.name || "");
  }, [projectId]);
  const load = useCallback(async () => {
    const ticket = ++generation.current;
    try { const value = await dataFunctionsApi.catalog(projectId); if (alive.current && ticket === generation.current) { accept(value); setError(""); } }
    catch (cause) { if (alive.current && ticket === generation.current) setError(errorText(cause)); }
  }, [projectId, accept]);
  useEffect(() => { alive.current = true; void load(); return () => { alive.current = false; generation.current++; }; }, [load]);
  const running = catalog?.operation?.status === "running";
  useEffect(() => {
    if (!running) return;
    let inFlight = false;
    const timer = window.setInterval(() => { if (inFlight) return; inFlight = true; void load().finally(() => { inFlight = false; }); }, 2000);
    return () => window.clearInterval(timer);
  }, [running, load]);
  const busy = locked || !!pending || !!running;
  const canManage = !!catalog?.canManage && !!catalog.customPublishing;
  const summary = catalog?.functions.find(item => item.name === selected);
  function choose(next: string) {
    if (busy || next === selected) return;
    if (dirty) { setDestination(next); return; }
    setSelected(next); setCreating(false); setError("");
  }
  async function create() {
    if (!canManage || busy || flight.current || !validName(name)) return;
    flight.current = true; generation.current++; setPending("create"); setError("");
    try {
      const source = await dataFunctionsApi.save(projectId, name, { expectedDigest: "", authMode: "authenticated", files: [{ path: "index.ts", content: starter }] });
      if (!alive.current) return;
      if (source.name !== name || !source.sourceDigest) throw new Error("Luci could not verify the saved function. Refresh its source.");
      setSelected(name); setName(""); setCreating(false); setDirty(false); await load();
    } catch (cause) { if (alive.current) setError(errorText(cause)); }
    finally { flight.current = false; if (alive.current) setPending(""); }
  }
  async function action(name: string, input: Omit<FunctionAction, "expectedRevision">) {
    if (!catalog || !canManage || busy || flight.current || (input.action !== "refresh" && !ready)) return;
    flight.current = true; generation.current++; setPending(input.action); setError("");
    try {
      const value = await dataFunctionsApi.action(projectId, name, { ...input, expectedRevision: catalog.revision });
      if (alive.current) accept(value);
    } catch (cause) { if (alive.current) setError(errorText(cause)); }
    finally { flight.current = false; if (alive.current) setPending(""); }
  }
  return <section className="data-section functions-workspace" aria-label="Data functions">
    <header className="data-heading"><div><h4>Functions</h4><p>Small server actions, kept with your project and published to your data services.</p></div><div className="journey-actions">
      <Button variant="ghost" disabled={busy} onClick={() => void load()}>Refresh functions</Button>
      {canManage && catalog?.operation?.status === "failed" && <Button variant="secondary" disabled={busy || !ready} onClick={() => { if (catalog.operation) void action(catalog.operation.name, { action: "refresh" }); }}>Check function state</Button>}
      {canManage && <Button disabled={busy || dirty} onClick={() => { setCreating(true); setError(""); }}>New function</Button>}
    </div></header>
    {error && <p role="alert" className="journey-error">{error}</p>}
    {!catalog && !error && <p role="status">Loading saved functions…</p>}
    {catalog && !canManage && <p>An Admin manages function source and releases.</p>}
    {!ready && <p className="functions-note">You can prepare function source now. Start your data services when you are ready to publish or test.</p>}
    {catalog?.operation && <p role="status" className="functions-note"><strong>{catalog.operation.status === "running" ? "Updating function…" : catalog.operation.status === "failed" ? "Function needs attention." : "Function updated."}</strong> {catalog.operation.message}</p>}
    {creating && <form className="functions-new" onSubmit={event => { event.preventDefault(); void create(); }}>
      <label>Function name<input required maxLength={63} pattern="[a-z][a-z0-9-]{0,62}" value={name} disabled={busy} placeholder="welcome-message" onChange={event => setName(event.target.value)} /><small>Use lowercase letters, numbers and hyphens. You can edit the starting code next.</small></label>
      <div className="journey-actions"><Button type="submit" loading={pending === "create"} disabled={busy || !validName(name) || !!catalog?.functions.some(item => item.name === name)}>Create function</Button><Button variant="ghost" disabled={busy} onClick={() => setCreating(false)}>Cancel</Button></div>
    </form>}
    {!!catalog?.functions.length && <div className="functions-layout"><nav aria-label="Project functions" className="functions-list">{catalog.functions.map(item => <button key={item.name} type="button" aria-current={selected === item.name ? "page" : undefined} disabled={busy} onClick={() => choose(item.name)}><strong>{item.name}</strong><span>{item.builtin ? "Built-in example" : item.active ? "Published" : "Draft"}{item.workingChanged && !item.builtin ? " · Uncommitted changes" : ""}</span></button>)}</nav>
      <div className="functions-detail">
        {destination && <div className="functions-note" role="alert"><p>You have unsaved edits. Save them first, or discard them to open another function.</p><div className="journey-actions"><Button variant="secondary" onClick={() => { setSelected(destination); setDestination(null); setDirty(false); }}>Discard edits and switch</Button><Button variant="ghost" onClick={() => setDestination(null)}>Keep editing</Button></div></div>}
        {summary && <FunctionEditor key={selected} projectId={projectId} summary={summary} ready={ready} busy={busy} canManage={canManage} onDirty={setDirty} onSaved={load} onAction={action} />}
      </div>
    </div>}
    <details className="data-disclosure"><summary>How functions run</summary><p>Use local TypeScript or JavaScript in supabase/functions. Your CLI can create additional local modules. Publishing uses a saved Git commit and retains earlier releases for rollback. The sandbox has limited access to this project's data API and no server administrator credentials. External imports and arbitrary packages are not supported.</p><p>Function source and publication history stay with this workspace. Data backups contain database records and stored files; keep your source in Git as well.</p></details>
  </section>;
}

interface EditorProps {
  projectId: string; summary: FunctionSummary; ready: boolean; busy: boolean; canManage: boolean;
  onDirty(value: boolean): void; onSaved(): Promise<void>;
  onAction(name: string, input: Omit<FunctionAction, "expectedRevision">): Promise<void>;
}
function FunctionEditor({ projectId, summary, ready, busy, canManage, onDirty, onSaved, onAction }: EditorProps) {
  const alive = useRef(true), flight = useRef(false), changed = useRef(false), generation = useRef(0);
  const [source, setSource] = useState<FunctionSource | null>(null), [files, setFiles] = useState<FunctionFile[]>([]);
  const [authMode, setAuthMode] = useState<FunctionAuth>("authenticated"), [file, setFile] = useState("index.ts");
  const [dirty, setDirty] = useState(false), [pending, setPending] = useState(""), [error, setError] = useState("");
  const [tab, setTab] = useState(0), [execute, setExecute] = useState(false), [remove, setRemove] = useState(false);
  const [rollback, setRollback] = useState(""), [rollbackAck, setRollbackAck] = useState(false);
  const [request, setRequest] = useState('{"name":"Luci"}'), [token, setToken] = useState(""), [response, setResponse] = useState<FunctionResponse | null>(null);
  const id = useId(), setLens = useCockpit(state => state.setLens);
  const dirtyCallback = useRef(onDirty); dirtyCallback.current = onDirty;
  const accept = useCallback((value: FunctionSource) => {
    if (value.name !== summary.name || !Array.isArray(value.files) || !["authenticated", "project-key"].includes(value.authMode)) throw new Error("Luci could not verify this function's source. Reload before saving.");
    setSource(value); setFiles(value.files); setAuthMode(value.authMode); setFile(current => value.files.some(item => item.path === current) ? current : value.files[0]?.path || "index.ts");
    changed.current = false; setDirty(false); dirtyCallback.current(false);
  }, [summary.name]);
  const load = useCallback(async () => {
    const ticket = ++generation.current;
    try { const value = await dataFunctionsApi.source(projectId, summary.name); if (alive.current && ticket === generation.current) { if (!changed.current) accept(value); setError(""); } }
    catch (cause) { if (alive.current && ticket === generation.current) setError(errorText(cause)); }
  }, [projectId, summary.name, accept]);
  useEffect(() => { alive.current = true; void load(); return () => { alive.current = false; generation.current++; }; }, [load]);
  useEffect(() => { setExecute(false); setRollbackAck(false); setRemove(false); setResponse(null); setToken(""); }, [summary.active?.releaseId]);
  useEffect(() => {
    if (!dirty) return;
    const warn = (event: BeforeUnloadEvent) => { event.preventDefault(); event.returnValue = ""; };
    window.addEventListener("beforeunload", warn); return () => window.removeEventListener("beforeunload", warn);
  }, [dirty]);
  const locked = busy || !!pending;
  const editable = canManage && !summary.builtin && !locked;
  const published = summary.builtin || !!summary.active;
  const committed = !!source?.commit && !!source.committedDigest && source.committedDigest === source.sourceDigest && !source.workingChanged;
  const unchangedRelease = !!summary.active && summary.active.sourceDigest === source?.sourceDigest;
  const testAuth = summary.active?.authMode || (summary.builtin ? "project-key" : source?.authMode || "authenticated");
  function edit() { changed.current = true; setDirty(true); onDirty(true); setExecute(false); }
  async function save() {
    if (!source || !editable || flight.current || !dirty) return;
    flight.current = true; generation.current++; setPending("save"); setError("");
    try {
      const value = await dataFunctionsApi.save(projectId, summary.name, { expectedDigest: source.sourceDigest, authMode, files: files.filter(item => item.path !== "luci.function.json") });
      if (!alive.current) return; accept(value); await onSaved();
    } catch (cause) { if (alive.current) setError(errorText(cause)); }
    finally { flight.current = false; if (alive.current) setPending(""); }
  }
  async function publish() {
    if (!editable || !ready || !source?.commit || !committed || dirty || !execute || unchangedRelease || flight.current) return;
    flight.current = true; setPending("publish"); setExecute(false); await onAction(summary.name, { action: "publish", commit: source.commit, acknowledgeExecute: true });
    flight.current = false; if (alive.current) setPending("");
  }
  async function invoke() {
    if (!ready || locked || !published || flight.current || (testAuth === "authenticated" && !token)) return;
    let input: unknown;
    try { input = JSON.parse(request); if (!input || typeof input !== "object" || Array.isArray(input)) throw new Error("Enter a JSON object for the function request."); }
    catch (cause) { setError(errorText(cause)); return; }
    flight.current = true; setPending("invoke"); setError(""); setResponse(null);
    const submittedToken = token; setToken("");
    try {
      const value = await dataFunctionsApi.invoke(projectId, summary.name, input as Record<string, unknown>, submittedToken || undefined);
      if (!alive.current) return;
      if (value.function !== summary.name || !Number.isInteger(value.statusCode)) throw new Error("Luci could not verify the function response. Check its outcome before retrying.");
      setResponse(value);
    } catch (cause) { if (alive.current) setError(errorText(cause)); }
    finally { flight.current = false; if (alive.current) setPending(""); }
  }
  async function releaseAction(action: "rollback" | "remove") {
    if (!editable || !ready || flight.current || (action === "rollback" ? !rollback || !rollbackAck : !remove)) return;
    flight.current = true; setPending(action); setRollbackAck(false); setRemove(false);
    await onAction(summary.name, { action, ...(action === "rollback" ? { releaseId: rollback, acknowledgeExecute: true } : { acknowledgeRemove: true }) });
    flight.current = false; if (alive.current) setPending("");
  }
  return <section aria-label={`Function ${summary.name}`}>
    <header className="functions-title"><h4>{summary.name}</h4><span>{summary.builtin ? "Built-in example" : summary.active ? "Published" : "Draft"}</span></header>
    <TabsSubtle aria-label="Function tools" idPrefix={id} selectedIndex={tab} onSelect={setTab}>{["Code", "Test", "Releases"].map((label, index) => <TabsSubtleItem key={label} label={label} index={index} />)}</TabsSubtle>
    {error && <p role="alert" className="journey-error">{error}</p>}
    {!source && !error && <p role="status">Loading function source…</p>}
    <TabsSubtlePanel idPrefix={id} index={0} selectedIndex={tab}>
      {source && <><p>{summary.builtin ? "Use this example to check your running service. Create a function for your own code." : "Edit here or use your CLI in Develop. Save source before committing it."}</p>
        <form onSubmit={event => { event.preventDefault(); void save(); }}>
          {files.length > 1 && <label className="functions-file-picker">Source file<select value={file} disabled={locked} onChange={event => setFile(event.target.value)}>{files.map(item => <option key={item.path} value={item.path}>{item.path}</option>)}</select></label>}
          <label className="functions-code-label">{file}<textarea className="functions-code" rows={12} spellCheck={false} value={files.find(item => item.path === file)?.content || ""} readOnly={!editable || file === "luci.function.json"} disabled={locked} maxLength={65536} onChange={event => { edit(); setFiles(current => current.map(item => item.path === file ? { ...item, content: event.target.value } : item)); }} /></label>
          {!summary.builtin && <><label className="functions-auth">Who can call this function?<select value={authMode} disabled={!editable} onChange={event => { edit(); setAuthMode(event.target.value as FunctionAuth); }}><option value="authenticated">Signed-in app users (recommended)</option><option value="project-key">Anyone with the project's public key</option></select><small>{authMode === "authenticated" ? "The service verifies an application user's session before running your code." : "The public project key is sufficient. Your code must enforce access to its data."}</small></label>
            <div className="journey-actions"><Button type="submit" loading={pending === "save"} disabled={!editable || !dirty}>Save source</Button><Button variant="ghost" disabled={locked || dirty} onClick={() => void load()}>Reload source</Button></div>
          </>}
        </form>
        {!summary.builtin && <div className="functions-publish">
          {dirty ? <p>Save your edits before publishing.</p> : !committed ? <p>Commit the saved function in Develop → Git, then reload its source here.</p> : unchangedRelease ? <p>This saved source is already published.</p> : <p>Ready to publish the saved commit.</p>}
          {!committed && <Button variant="secondary" disabled={locked || dirty} onClick={() => setLens("develop")}>Open Develop</Button>}
          {committed && !unchangedRelease && <><label className="data-check"><input type="checkbox" checked={execute} disabled={!editable || !ready || dirty} onChange={event => setExecute(event.target.checked)} />Run this committed code while publishing and expose its endpoint.</label><Button loading={pending === "publish"} disabled={!editable || !ready || dirty || !execute} onClick={() => void publish()}>Publish function</Button></>}
          <details className="data-disclosure"><summary>Source details</summary><p><code>{source.path}</code></p><p>Saved commit: <code>{source.commit?.slice(0, 12) || "Not committed"}</code>. {source.workingChanged ? "Source has uncommitted changes." : "Source matches its saved commit."}</p></details>
        </div>}
      </>}
    </TabsSubtlePanel>
    <TabsSubtlePanel idPrefix={id} index={1} selectedIndex={tab}>
      {!published ? <p>Publish a committed function before testing its endpoint.</p> : <form className="data-form" onSubmit={event => { event.preventDefault(); void invoke(); }}>
        <p>Calls the actual published function. Its code may change application data.</p>
        {dirty && <p>Your editor has unsaved changes. This test uses the published version.</p>}
        <label>Function request<textarea rows={4} maxLength={65536} value={request} disabled={locked} onChange={event => setRequest(event.target.value)} /></label>
        {testAuth === "authenticated" && <label>Application user session token<input type="password" required autoComplete="off" maxLength={4096} value={token} disabled={locked} onChange={event => setToken(event.target.value)} /><small>Use an access token from a signed-in user of this app. It clears after the request. Luci login and server administrator keys are not accepted.</small></label>}
        <Button type="submit" loading={pending === "invoke"} disabled={!ready || locked || (testAuth === "authenticated" && !token)}>Invoke {summary.name}</Button>
      </form>}
      {response && <div className="functions-response" role="status"><p>HTTP {response.statusCode} · {response.durationMs} ms</p>{response.statusCode === 401 && <p>Sign in to your application and use its current session token, then explicitly test again.</p>}<pre className="journey-code" aria-label="Function response">{typeof response.result === "string" ? response.result : JSON.stringify(response.result, null, 2)}</pre></div>}
    </TabsSubtlePanel>
    <TabsSubtlePanel idPrefix={id} index={2} selectedIndex={tab}>
      {summary.builtin ? <p>The built-in example ships with your installed data services.</p> : <>
        {!summary.history.length && <p>No published releases yet.</p>}
        <ul className="functions-releases">{summary.history.map(item => <li key={item.releaseId}><div><strong>{item.status === "active" ? "Current release" : item.status === "failed" ? "Failed release" : item.status === "candidate" ? "Unverified candidate" : item.status === "stopped" ? "Removed endpoint" : "Earlier release"}</strong><span>{new Date(item.publishedAt).toLocaleString()} · {authLabel(item.authMode)}</span><code>{item.commit.slice(0, 12)}</code><p>{item.message}</p></div>{["active", "superseded", "stopped"].includes(item.status) && item.releaseId !== summary.active?.releaseId && <Button variant="secondary" disabled={!editable || !ready} onClick={() => { setRollback(item.releaseId); setRollbackAck(false); }}>Choose release {item.commit.slice(0, 8)}</Button>}</li>)}</ul>
        {rollback && <div className="functions-note"><p>Restore the selected release's code and access policy. This does not undo database changes.</p><label className="data-check"><input type="checkbox" checked={rollbackAck} disabled={!editable || !ready} onChange={event => setRollbackAck(event.target.checked)} />Run and activate this earlier release.</label><div className="journey-actions"><Button loading={pending === "rollback"} disabled={!editable || !ready || !rollbackAck} onClick={() => void releaseAction("rollback")}>Restore selected release</Button><Button variant="ghost" disabled={locked} onClick={() => setRollback("")}>Cancel restore</Button></div></div>}
        {summary.active && <details className="data-disclosure"><summary>Remove published endpoint</summary><p>Stop new calls to this function. Your source and release history remain available.</p><label className="data-check"><input type="checkbox" checked={remove} disabled={!editable || !ready} onChange={event => setRemove(event.target.checked)} />Remove this function's active endpoint.</label><Button variant="secondary" loading={pending === "remove"} disabled={!editable || !ready || !remove} onClick={() => void releaseAction("remove")}>Remove endpoint</Button></details>}
      </>}
    </TabsSubtlePanel>
  </section>;
}
