import { useCallback, useEffect, useId, useRef, useState } from "react";
import { dataApi, type DataDomainStatus } from "../../api/data";
import { errorText, productApi, type DeploymentTarget } from "../../api/product";

const labels: Record<string, string> = {
  "not-configured": "Private",
  pending: "Waiting for verification",
  uncertain: "Check this attempt",
  verifiedHTTPS: "HTTPS verified",
  stopped: "Public access stopped",
  attention: "Action needed",
  working: "Preparing public access",
  publishing: "Preparing public access",
};

// Readback must describe a complete HTTPS origin before the UI exposes a link.
export function publicDataAddress(value: DataDomainStatus): string | null {
  if (value.status !== "verifiedHTTPS" || !value.publicUrl) return null;
  try {
    const address = new URL(value.publicUrl);
    if (address.protocol !== "https:" || address.username || address.password ||
      address.search || address.hash || !["", "/"].includes(address.pathname) ||
      (address.port !== "" && address.port !== "443") || !address.hostname.includes(".") ||
      /^(\d{1,3}\.){3}\d{1,3}$/.test(address.hostname) ||
      [".localhost", ".local", ".internal"].some(suffix => address.hostname.endsWith(suffix))) return null;
    if (value.domain && address.hostname !== value.domain.replace(/^https:\/\//, "").replace(/\/$/, "")) return null;
    return address.origin;
  } catch { return null; }
}

export function DataPublicDomain(props: {
  projectId: string; disabled: boolean; ready: boolean; onChange(value: DataDomainStatus): void; onEmailSettings?(): void;
}) {
  return <ProjectPublicDomain key={props.projectId} {...props} />;
}
function ProjectPublicDomain({ projectId, disabled, ready, onChange, onEmailSettings }: {
  projectId: string; disabled: boolean; ready: boolean; onChange(value: DataDomainStatus): void; onEmailSettings?(): void;
}) {
  const active = useRef(true), submitting = useRef(false), generation = useRef(0);
  const fieldId = useId();
  const [binding, setBinding] = useState<DataDomainStatus | null>(null);
  const [targets, setTargets] = useState<DeploymentTarget[]>([]);
  const [targetId, setTargetId] = useState(""), [domain, setDomain] = useState("");
  const [applicationOrigin, setApplicationOrigin] = useState("");
  const [acknowledged, setAcknowledged] = useState(false), [stopConfirmed, setStopConfirmed] = useState(false);
  const [pending, setPending] = useState(""), [error, setError] = useState("");
  const accept = useCallback((value: DataDomainStatus) => {
    if (value.projectId !== projectId || !Number.isInteger(value.revision) || value.revision < 0)
      throw new Error("Luci could not verify this project's saved public connection. Refresh and try again.");
    setBinding(value); onChange(value);
  }, [projectId, onChange]);
  const load = useCallback(async () => {
    const ticket = ++generation.current;
    setError("");
    const [saved, available] = await Promise.allSettled([dataApi.domain(projectId), productApi.targets()]);
    if (!active.current || generation.current !== ticket) return;
    const failures: string[] = [];
    if (available.status === "fulfilled") setTargets(available.value.targets);
    else failures.push(errorText(available.reason));
    if (saved.status === "fulfilled") {
      const value = saved.value;
      if (value.projectId !== projectId || !Number.isInteger(value.revision) || value.revision < 0) {
        failures.push("This public connection does not belong to the selected project.");
        setBinding(null);
      } else {
        setBinding(value);
        setTargetId(value.targetId || ""); setDomain(value.domain || "");
        setApplicationOrigin(value.applicationOrigin || "");
      }
    } else failures.push(errorText(saved.reason));
    setAcknowledged(false); setStopConfirmed(false);
    setError(failures.join(" "));
  }, [projectId]);
  useEffect(() => {
    active.current = true;
    void load();
    return () => { active.current = false; generation.current++; };
  }, [load]);
  const operating = binding?.operation?.status === "running" || ["working", "publishing", "stopping", "checking"].includes(binding?.status || "");
  useEffect(() => {
    if (!operating) return;
    let inFlight = false;
    const ticket = generation.current;
    const timer = window.setInterval(() => {
      if (inFlight) return;
      inFlight = true;
      void dataApi.domain(projectId).then(value => {
        if (active.current && generation.current === ticket) accept(value);
      }).catch(cause => { if (active.current && generation.current === ticket) setError(errorText(cause)); })
        .finally(() => { inFlight = false; });
    }, 2500);
    return () => window.clearInterval(timer);
  }, [operating, projectId, accept]);
  const locked = disabled || !!pending || operating;
  const address = binding && publicDataAddress(binding);
  const uncertain = binding?.status === "uncertain";
  const configured = binding && binding.status !== "not-configured";
  function changed() { setAcknowledged(false); setStopConfirmed(false); }
  async function submit(action: "publish" | "refresh" | "stop") {
    if (locked || !binding || submitting.current) return;
    if (action === "publish" && (!ready || !acknowledged || uncertain || !targetId || !domain.trim() || !applicationOrigin.trim())) return;
    if (action === "stop" && !stopConfirmed) return;
    submitting.current = true; setPending(action); setError("");
    const ticket = generation.current;
    try {
      const value = await dataApi.configureDomain(projectId, {
        action, expectedRevision: binding.revision,
        ...(action === "publish" ? { targetId, domain: domain.trim(), applicationOrigin: applicationOrigin.trim(), acknowledgePublic: true } : {}),
      });
      if (active.current && generation.current === ticket) { accept(value); setAcknowledged(false); setStopConfirmed(false); }
    } catch (cause) { if (active.current && generation.current === ticket) setError(errorText(cause)); }
    finally { submitting.current = false; if (active.current && generation.current === ticket) setPending(""); }
  }
  return <section className="data-public-domain" aria-label="Public data connection">
    <div className="data-heading"><div><h4>Use data in your published app</h4><p>Give this project a secure API address on your cloud server. Table and file policies continue to control access.</p></div>
      <span className="platform-state" data-state={address ? "ready" : "pending"}>{binding ? binding.status === "verifiedHTTPS" && !address ? "Verification needed" : labels[binding.status] || "Checking connection" : "Loading"}</span></div>
    {error && <><p className="journey-error" role="alert">{error}</p><button type="button" className="luci-button" disabled={!!pending || operating} onClick={() => void load()}>Reload saved connection</button></>}
    {binding?.message && <p role="status">{binding.message}</p>}
    {binding?.checkedAt && Number.isFinite(Date.parse(binding.checkedAt)) && <p className="journey-muted">Last checked {new Date(binding.checkedAt).toLocaleString()}.</p>}
    {operating && <p role="status">You can leave this page. Luci keeps this attempt and its progress.</p>}
    {address && <label className="data-readonly">Public HTTPS API address<input readOnly value={address} /><small>Use this address with your public anonymous key. Redeploy your application when its connection variables change.</small></label>}
    {!targets.length && binding && <p>Connect your AWS, Azure or existing deployment server in Workspace setup first. Your data services must run on that same server.</p>}
    <form className="data-form" onSubmit={event => { event.preventDefault(); void submit("publish"); }}>
      <label>Data deployment server<select required value={targetId} disabled={locked || !targets.length || !!configured} onChange={event => { setTargetId(event.target.value); changed(); }}><option value="">Choose your connected server</option>{targets.map(target => <option key={target.id} value={target.id}>{target.name}</option>)}</select></label>
      <div className="data-field"><label htmlFor={`${fieldId}-domain`}>Data API domain</label><input id={`${fieldId}-domain`} aria-describedby={`${fieldId}-domain-help`} required autoComplete="off" spellCheck={false} placeholder="data.example.com" value={domain} disabled={locked || !targets.length} onChange={event => { setDomain(event.target.value); changed(); }} /><small id={`${fieldId}-domain-help`}>Point this domain's A or AAAA record at your cloud server's public address. Luci's deployment service requests and renews its HTTPS certificate.</small></div>
      <div className="data-field"><label htmlFor={`${fieldId}-origin`}>Published application address</label><input id={`${fieldId}-origin`} aria-describedby={`${fieldId}-origin-help`} required type="url" autoComplete="url" placeholder="https://app.example.com" value={applicationOrigin} disabled={locked || !targets.length} onChange={event => { setApplicationOrigin(event.target.value); changed(); }} /><small id={`${fieldId}-origin-help`}>Enter the exact HTTPS origin of the app allowed to use this API, without a path. This also sets its password sign-in redirect address.</small></div>
      <label className="data-check"><input type="checkbox" checked={acknowledged} disabled={locked || !targets.length || uncertain} onChange={event => setAcknowledged(event.target.checked)} />Make this project's client API available publicly at this domain.</label>
      <button className="luci-button luci-primary" disabled={locked || !ready || !binding || !targets.length || !acknowledged || uncertain || !targetId || !domain.trim() || !applicationOrigin.trim()}>{pending === "publish" ? "Preparing public access…" : configured ? "Update public connection" : "Publish data API"}</button>
    </form>
    {!!configured && <div className="journey-actions"><button type="button" className="luci-button" disabled={locked} onClick={() => void submit("refresh")}>{pending === "refresh" ? "Checking DNS and HTTPS…" : "Check DNS and HTTPS"}</button></div>}
    {uncertain && <p>Check this saved attempt before publishing again. Luci will reconcile it with the deployment service to avoid creating duplicates.</p>}
    {!ready && <p>Start and check the data services before publishing. You can still stop an existing public connection.</p>}
    {!!configured && binding.status !== "stopped" && <div className="data-domain-stop"><label className="data-check"><input type="checkbox" checked={stopConfirmed} disabled={locked} onChange={event => setStopConfirmed(event.target.checked)} />Stop public API access for applications using this address.</label><button type="button" className="luci-button" disabled={locked || !stopConfirmed} onClick={() => void submit("stop")}>{pending === "stop" ? "Stopping public access…" : "Stop public access"}</button></div>}
    <p>Email confirmation and password recovery use your SMTP connection in Authentication. Publishing this API does not configure email.</p>
    {onEmailSettings && <button type="button" className="luci-button" onClick={onEmailSettings}>Email settings</button>}
  </section>;
}
