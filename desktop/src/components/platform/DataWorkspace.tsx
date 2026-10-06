import { useCallback, useEffect, useRef, useState } from "react";
import { dataApi, type DataStatus } from "../../api/data";
import { errorText } from "../../api/product";
import { platformApi, isolatedBrowserURL } from "../../api/platform";
import { DataTables, DataUsers, DataStorage, DataRealtime } from "./DataControls";
import { DataFunctions } from "./DataFunctions";
import { Button } from "../fluid/components/button";
import { DataPublicDomain } from "./DataPublicDomain";
import { DataEmail } from "./DataEmail";
import { DataClientSetup } from "./DataClientSetup";
import { useAuth } from "../auth/AuthProvider";
import "./platform.css";
import { useDataAccess } from "./useDataAccess";

const sections = ["Services", "Tables", "Authentication", "Storage", "Realtime", "Functions", "Backups", "Connect"] as const;
type Section = (typeof sections)[number];
export function DataWorkspace({ projectId, dockerReady }: { projectId: string; dockerReady: boolean }) {
  const access = useDataAccess(projectId);
  return <ProjectData key={access.scope} projectId={projectId} dockerReady={dockerReady} />;
}
function ProjectData({ projectId, dockerReady }: { projectId: string; dockerReady: boolean }) {
  const auth = useAuth();
  const access = useDataAccess(projectId);
  const accessRef = useRef(access); accessRef.current = access;
  const canManage = (!auth || auth.profile.permissions.manageBacking) && access.canWrite;
  const active = useRef(true), connectingBrowser = useRef(false), acting = useRef(false);
  useEffect(() => { active.current = true; return () => { active.current = false; }; }, []);
  const [data, setData] = useState<DataStatus | null>(null);
  const [section, setSection] = useState<Section>("Services");
  const [pending, setPending] = useState("");
  const [error, setError] = useState("");
  const [acknowledged, setAcknowledged] = useState(false);
  const [restoreId, setRestoreId] = useState("");
  const [restoreConfirmed, setRestoreConfirmed] = useState(false);
  const [freshInstallConfirmed, setFreshInstallConfirmed] = useState(false);
  const load = useCallback(() => dataApi.status(projectId), [projectId]);
  useEffect(() => {
    if (!access.canRead) return;
    let active = true;
    const current = access.capture();
    load().then(value => { if (active && current()) {
      if (value.projectId !== projectId) throw new Error("Luci could not verify this project's saved data services.");
      setData(value);
    } }).catch(cause => { if (active && current()) setError(errorText(cause)); });
    return () => { active = false; };
  }, [load, access.canRead, access.capture]);
  const running = data?.operation?.status === "running";
  useEffect(() => {
    if (!running || !access.canRead) return;
    const current = access.capture();
    let active = true, inFlight = false;
    const timer = window.setInterval(() => {
      if (inFlight) return;
      inFlight = true;
      load().then(value => { if (active && current()) {
        if (value.projectId !== projectId) throw new Error("Luci could not verify this project's data service progress.");
        setData(value); setError("");
      } }).catch(cause => { if (active && current()) setError(errorText(cause)); }).finally(() => { inFlight = false; });
    }, 2500);
    return () => { active = false; window.clearInterval(timer); };
  }, [running, load, projectId, access.canRead, access.capture]);
  useEffect(() => {
    if (!access.showPrivate) { setData(null); setError(""); setPending(""); setRestoreId(""); setRestoreConfirmed(false); setAcknowledged(false); }
  }, [access.showPrivate]);
  const readLocked = !!pending || running || !access.canRead;
  const locked = readLocked || !access.canWrite;
  const ready = data?.status === "ready" && !running;
  const installed = data && data.status !== "not-installed";
  const recoverable = !!data?.canRecoverInstall && data.initializationState !== "complete" && !data.backups.length && !ready;
  useEffect(() => { setFreshInstallConfirmed(false); }, [recoverable, data?.revision, data?.status]);
  async function action(kind: Parameters<typeof dataApi.action>[1]) {
    if (locked || acting.current || !accessRef.current.canWrite) return;
    const current = access.capture();
    if (["install", "start", "restore", "recover-install"].includes(kind) && (!dockerReady || !acknowledged)) return;
    if (kind === "recover-install" && (!recoverable || data?.status !== "stopped" || !freshInstallConfirmed)) return;
    if (kind === "restore" && (!restoreId || !restoreConfirmed)) return;
    acting.current = true;
    setPending(kind); setError("");
    try {
      const next = await dataApi.action(projectId, kind, { acknowledgeResources: acknowledged, ...(kind === "restore" ? { backupId: restoreId, confirmRestore: restoreConfirmed } : {}), ...(kind === "recover-install" ? { confirmFreshInstall: true } : {}) });
      if (!active.current || !current()) return;
      if (next.projectId !== projectId) throw new Error("Luci could not verify this project's data service operation. Refresh its saved status.");
      setData(next);
      if (kind === "restore") setRestoreConfirmed(false);
      if (kind === "recover-install") setFreshInstallConfirmed(false);
    } catch (cause) { if (active.current && current()) setError(errorText(cause)); }
    finally { acting.current = false; if (active.current) setPending(""); }
  }
  async function download(backup: NonNullable<DataStatus["backups"]>[number]) {
    if (readLocked || !accessRef.current.canRead) return;
    const current = access.capture();
    setPending("download"); setError("");
    try { await dataApi.downloadBackup(projectId, backup, current); }
    catch (cause) { if (current()) setError(errorText(cause)); }
    finally { if (active.current) setPending(""); }
  }
  async function connectBrowser() {
    if (locked || !accessRef.current.canExecute || !ready || connectingBrowser.current) return;
    const current = access.capture();
    connectingBrowser.current = true;
    setPending("browser"); setError("");
    try {
      const connection = await platformApi.prepareBrowser(projectId, "data");
      if (!active.current || !current()) return;
      const address = isolatedBrowserURL(connection.url);
      if (!address || connection.projectId !== projectId || connection.kind !== "data" || !connection.resourceId)
        throw new Error("Luci could not verify this project's browser API connection. Reconnect your workspace and try again.");
      setData(current => current ? { ...current, url: address, endpointScope: "browser-gateway" } : current);
    } catch (cause) { if (active.current && current()) setError(errorText(cause)); }
    finally { connectingBrowser.current = false; if (active.current) setPending(""); }
  }
  async function refreshSaved() {
    if (readLocked || !accessRef.current.canRead) return;
    const current = access.capture();
    setPending("read"); setError("");
    try {
      const next = await load();
      if (!current()) return;
      if (next.projectId !== projectId) throw new Error("Luci could not verify this project's saved data services.");
      setData(next);
    } catch (cause) { if (current()) setError(errorText(cause)); }
    finally { if (active.current) setPending(""); }
  }
  return <section className="data-workspace" aria-label="Project data platform">
    <header className="data-heading">
      <div><h3>Data &amp; authentication</h3><p>PostgreSQL, user accounts, files and live updates for this project.</p></div>
      <span className="platform-state" data-state={access.showPrivate ? data?.status : undefined}>{access.showPrivate ? data?.status?.split("-").join(" ") || "Loading" : "Access required"}</span>
    </header>
    {access.message && <p role="status">{access.message}</p>}
    {access.showPrivate && error && <p className="journey-error" role="alert">{error}</p>}
    {access.canRead && !data && !error && <p role="status">Loading saved service status…</p>}
    {access.showPrivate && data && <>
      <nav className="data-navigation" aria-label="Data sections">{sections.map(name => <Button type="button" variant="ghost" key={name} aria-current={section === name ? "page" : undefined} onClick={() => setSection(name)}>{name}</Button>)}</nav>
      <Button type="button" variant="secondary" disabled={readLocked} onClick={() => void refreshSaved()}>Refresh saved data status</Button>
      {data.operation && <div className="data-operation" role="status"><strong>{data.operation.action.split("-").join(" ")} · {data.operation.status}</strong><span>{data.operation.message}</span>{running && <small>Stage: {data.operation.stage}. You can leave this page and return to its saved progress.</small>}</div>}
      {section === "Services" && <>
        <p className="data-summary">{data.message}</p>
        <div className="journey-actions">
          {!installed ? <Button type="button" disabled={locked || !dockerReady || !acknowledged} onClick={() => void action("install")}>Install data services</Button> : <>
            <Button type="button" variant="secondary" disabled={locked || !dockerReady || !acknowledged || ready} onClick={() => void action("start")}>Start data services</Button>
            <Button type="button" variant="secondary" disabled={locked || !dockerReady || data.status === "stopped"} onClick={() => void action("stop")}>Stop data services</Button>
          </>}
          <Button type="button" variant="secondary" disabled={locked || !dockerReady} onClick={() => void action("refresh")}>Check service health</Button>
        </div>
        <div className="data-resource-note"><h4>Resources on your selected Docker host</h4><p>This optional Supabase stack needs {(data.requirements.memoryMiB / 1024).toFixed(0)} GiB RAM, {data.requirements.cpuMillis / 1000} CPU cores and {data.requirements.diskGiB} GiB disk in addition to your app and agent. Images download on first install. Existing PostgreSQL databases stay separate.</p><label className="data-check"><input type="checkbox" checked={acknowledged} disabled={locked} onChange={event => setAcknowledged(event.target.checked)} />I have capacity for these services on the selected Docker host.</label>{!dockerReady && <p>Start Docker or reconnect your cloud workspace, then refresh the platform.</p>}</div>
        {recoverable && <section className="data-resource-note" aria-label="Recover failed initial installation"><h4>Recover an incomplete first installation</h4><p>Luci can check whether this stack failed during its first database setup. Recovery starts a fresh candidate with this project's saved credentials and keeps the original volumes. It becomes the active stack only after database, authentication, API and storage checks pass.</p><p>This does not restore data from the incomplete stack. If the failure cannot be verified, Luci will leave it in place.</p>{data.status !== "stopped" ? <p>Stop the current data services before recovering this installation.</p> : <label className="data-check"><input type="checkbox" checked={freshInstallConfirmed} disabled={locked || !dockerReady} onChange={event => setFreshInstallConfirmed(event.target.checked)} />Start a fresh installation and retain the incomplete stack's volumes.</label>}<Button type="button" variant="secondary" disabled={locked || !dockerReady || !acknowledged || data.status !== "stopped" || !freshInstallConfirmed} onClick={() => void action("recover-install")}>{pending === "recover-install" ? "Checking recovery…" : "Recover initial installation"}</Button></section>}
        {section === "Services" && data.priorStackRetained && <p>Previous stack volumes are retained for recovery.</p>}
        {!!data.services.length && <details className="data-disclosure" open={!ready}><summary>Service health · {data.services.length} services</summary><div className="data-table-scroll"><table className="data-table"><caption>Last checked services {data.checkedAt ? `· ${new Date(data.checkedAt).toLocaleString()}` : ""}</caption><thead><tr><th>Service</th><th>Container</th><th>Health</th></tr></thead><tbody>{data.services.map(service => <tr key={service.name}><th scope="row">{service.name}</th><td>{service.status}</td><td>{service.health || "Not reported"}</td></tr>)}</tbody></table></div></details>}
      </>}
      {!["Services", "Backups", "Connect", "Functions"].includes(section) && !ready && <div className="data-empty"><h4>Start your data services</h4><p>These controls become available after the stack passes its real health checks.</p><Button type="button" variant="secondary" onClick={() => setSection("Services")}>Go to services</Button></div>}
      {ready && section === "Tables" && <DataTables projectId={projectId} />}
      {ready && section === "Authentication" && <DataUsers projectId={projectId} />}
      {installed && section === "Authentication" && <DataEmail projectId={projectId} ready={ready} locked={readLocked} canManage={canManage} />}
      {ready && section === "Storage" && <DataStorage projectId={projectId} />}
      {ready && section === "Realtime" && <DataRealtime projectId={projectId} />}
      {section === "Functions" && <DataFunctions projectId={projectId} ready={ready} locked={readLocked} />}
      {section === "Backups" && <section className="data-section" aria-label="Data backups"><div className="data-heading"><div><h4>Back up and recover</h4><p>A backup includes your application tables, accounts and stored files. Maximum archive size: 64 MiB.</p></div><Button type="button" variant="secondary" disabled={locked || !ready} onClick={() => void action("backup")}>Create data backup</Button></div>
        {!data.backups.length && <p>No backups yet. Create one while services are healthy.</p>}
        {!!data.backups.length && <ul className="data-records">{data.backups.map(backup => <li key={backup.id}><div><strong>{new Date(backup.createdAt).toLocaleString()}</strong><small>{Math.ceil(backup.bytes / 1024)} KiB · {backup.includesStorage ? "Database and files" : "Database"} · SHA-256 {backup.sha256.slice(0, 16)}…</small></div><Button type="button" variant="secondary" disabled={readLocked} onClick={() => void download(backup)}>Download archive</Button></li>)}</ul>}
        {!!data.backups.length && <form className="data-form data-recovery" onSubmit={event => { event.preventDefault(); void action("restore"); }}><h4>Restore into a fresh stack</h4><p>The selected backup replaces the active data stack. Luci keeps the previous volumes for recovery. Staging needs another 4 GiB RAM and 2 CPU cores while the previous stack exists.</p><label>Backup to restore<select required value={restoreId} disabled={locked} onChange={event => { setRestoreId(event.target.value); setRestoreConfirmed(false); }}><option value="">Choose a saved backup</option>{data.backups.map(backup => <option key={backup.id} value={backup.id}>{new Date(backup.createdAt).toLocaleString()} · {backup.id}</option>)}</select></label><label className="data-check"><input type="checkbox" checked={acknowledged} disabled={locked} onChange={event => setAcknowledged(event.target.checked)} />I have capacity for a fresh data stack.</label><label className="data-check"><input type="checkbox" checked={restoreConfirmed} disabled={locked || !restoreId} onChange={event => setRestoreConfirmed(event.target.checked)} />Restore this backup and replace the active stack.</label><Button type="submit" variant="secondary" disabled={locked || !dockerReady || !restoreId || !acknowledged || !restoreConfirmed}>Restore selected backup</Button></form>}
        {data.priorStackRetained && <p>Previous stack volumes are retained. They are not deleted by restoring.</p>}
      </section>}
      {section === "Connect" && <section className="data-section"><h4>Connect your application</h4><p>Apps running on this project’s Docker network receive server connection settings privately when deployed. Your browser client uses the public anonymous key with row-level security. New Docker deployments receive this browser address automatically. Redeploy existing applications after connecting or reconnecting the workspace.</p>{data.endpointScope === "workspace-browser-pending" ? <><p>Your data services are running on the connected workspace. Connect a private browser address to use their API from this computer.</p><Button type="button" variant="secondary" disabled={locked || !access.canExecute || !ready} onClick={() => void connectBrowser()}>{pending === "browser" ? "Connecting browser API…" : "Connect browser API"}</Button></> : data.url ? <><label className="data-readonly">{data.endpointScope === "browser-gateway" ? "API address for your browser" : "API address on the selected Docker host"}<input readOnly value={data.url} /></label><p>{data.endpointScope === "browser-gateway" ? "This local address reaches your project through your active Luci workspace connection. Keep Luci and its Docker host running. Public hosting needs an HTTPS proxy and domain." : "This loopback address works on that Docker host. A cloud workspace needs an HTTPS proxy and domain before a browser on another machine can use it."}</p></> : <p>An API address will appear after installation.</p>}{data.publishableKey && <label className="data-readonly">Public anonymous key<textarea readOnly rows={4} value={data.publishableKey} /><small>This key identifies the project; it is not an administrator credential. Access depends on your table and storage policies.</small></label>}<p>Supabase {data.version}. Configure confirmation and recovery email in Authentication. OAuth provider connections require separate setup.</p></section>}
      {section === "Connect" && ready && <DataClientSetup projectId={projectId} address={data.url} publishableKey={data.publishableKey} disabled={locked} />}
      {section === "Connect" && installed && <DataPublicDomain projectId={projectId} disabled={readLocked || !dockerReady} ready={ready} onEmailSettings={() => setSection("Authentication")} onChange={value => setData(current => current ? { ...current, publicDomain: value, publicUrl: value.publicUrl, ...(value.operation ? { operation: value.operation } : {}) } : current)} />}
    </>}
  </section>;
}
