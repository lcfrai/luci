import { useCallback, useEffect, useId, useRef, useState } from "react";
import { dataEmailApi, type DataEmailConfiguration, type DataEmailRequest, type DataEmailStatus } from "../../api/dataEmail";
import { errorText } from "../../api/product";
import { useDataAccess } from "./useDataAccess";
import { Button } from "../fluid/components/button";

interface Props {
  projectId: string; ready: boolean; locked?: boolean; canManage?: boolean;
  onChange?(value: DataEmailStatus): void;
}
const empty: DataEmailConfiguration = { host: "", port: 465, username: "", fromAddress: "", senderName: "Luci", applicationOrigin: "", hasPassword: false };
const statusLabels: Record<string, string> = { unconfigured: "Not configured", saved: "Saved draft", configured: "Applied to Auth", attention: "Action needed" };

export function DataEmail(props: Props) {
  const access = useDataAccess(props.projectId);
  if (!access.showPrivate) return <p role="status">{access.message}</p>;
  return <ProjectDataEmail key={access.scope} {...props} />;
}
function ProjectDataEmail({ projectId, ready, locked = false, canManage = true, onChange }: Props) {
  const access = useDataAccess(projectId);
  const accessRef = useRef(access); accessRef.current = access;
  const active = useRef(true), submitting = useRef(false), generation = useRef(0), changed = useRef(false);
  const callback = useRef(onChange); callback.current = onChange;
  const id = useId();
  const [status, setStatus] = useState<DataEmailStatus | null>(null);
  const [draft, setDraft] = useState<DataEmailConfiguration>(empty);
  const [password, setPassword] = useState(""), [recipient, setRecipient] = useState("");
  const [sendConfirmed, setSendConfirmed] = useState(false), [removeConfirmed, setRemoveConfirmed] = useState(false);
  const [dirty, setDirty] = useState(false), [pending, setPending] = useState(""), [error, setError] = useState("");
  const accept = useCallback((value: DataEmailStatus, populate: boolean) => {
    if (value.projectId !== projectId || !Number.isInteger(value.revision) || value.revision < 0 ||
      !Number.isInteger(value.appliedRevision) || value.appliedRevision < 0 || value.appliedRevision > value.revision || !value.configuration)
      throw new Error("Luci could not verify this project's email settings. Refresh before retrying.");
    setStatus(value);
    if (populate && !changed.current) {
      setDraft({ ...value.configuration, applicationOrigin: value.originSource === "public-domain" ? value.applicationOrigin : value.configuration.applicationOrigin });
      setDirty(false);
    }
    callback.current?.(value);
  }, [projectId]);
  const load = useCallback(async () => {
    if (!accessRef.current.canRead) return;
    const current = accessRef.current.capture();
    const ticket = ++generation.current;
    try { const value = await dataEmailApi.status(projectId); if (active.current && generation.current === ticket && current()) { accept(value, true); setError(""); } }
    catch (cause) { if (active.current && generation.current === ticket && current()) setError(errorText(cause)); }
  }, [projectId, accept]);
  useEffect(() => { active.current = true; void load(); return () => { active.current = false; generation.current++; }; }, [load]);
  const running = status?.operation?.status === "running";
  useEffect(() => {
    if (!running || !access.canRead) return;
    let inFlight = false;
    const timer = window.setInterval(() => {
      if (inFlight) return;
      inFlight = true; void load().finally(() => { inFlight = false; });
    }, 2000);
    return () => window.clearInterval(timer);
  }, [running, load, access.canRead]);
  const busy = locked || !!pending || !!running || !access.canRead;
  const activeAction = pending || (running ? status?.operation?.action : "");
  const readOnly = busy || !canManage || !access.canWrite;
  const saved = !!status?.configuration.host;
  const applied = !!status && status.appliedRevision > 0 && status.appliedRevision === status.revision && status.status !== "attention";
  function field<K extends keyof DataEmailConfiguration>(name: K, value: DataEmailConfiguration[K]) {
    changed.current = true; setDirty(true); setDraft(current => ({ ...current, [name]: value })); setSendConfirmed(false);
  }
  async function action(kind: DataEmailRequest["action"]) {
    if (!status || readOnly || !accessRef.current.canWrite || submitting.current) return;
    const current = access.capture();
    if (kind === "test" && (!sendConfirmed || !recipient || dirty || !applied || !ready)) return;
    if (kind === "disable" && !removeConfirmed) return;
    submitting.current = true; generation.current++; setPending(kind); setError("");
    const input: DataEmailRequest = { action: kind, expectedRevision: status.revision };
    if (kind === "save") {
      const { hasPassword: _redacted, ...configuration } = draft;
      input.configuration = { ...configuration, ...(password ? { password } : {}) };
      setPassword("");
    }
    if (kind === "test") { input.recipient = recipient; input.acknowledgeSend = true; setSendConfirmed(false); }
    try {
      const result = await dataEmailApi.action(projectId, input);
      if (!active.current || !current()) return;
      if (kind === "save" || kind === "disable") { changed.current = false; setDirty(false); setRemoveConfirmed(false); }
      accept(result, true);
    } catch (cause) { if (active.current && current()) setError(errorText(cause)); }
    finally { submitting.current = false; if (active.current) setPending(""); }
  }
  return <section className="data-section" aria-label="Application email delivery">
    <div className="data-heading"><div><h4>Email delivery</h4><p>Connect your email provider for application signup confirmation and password recovery.</p></div><Button type="button" variant="secondary" disabled={busy} onClick={() => void load()}>Refresh email status</Button></div>
    {access.message && <p role="status">{access.message}</p>}
    {error && <p className="journey-error" role="alert">{error}</p>}
    {!status && !error && <p role="status">Loading email settings…</p>}
    {status && <><p role="status"><strong>{statusLabels[status.status] || status.status}.</strong> {pending ? `${pending === "save" ? "Saving" : "Preparing email operation"}…` : status.operation?.status === "running" ? status.operation.message : status.message}</p>
      <p>Saved revision {status.revision} · Applied revision {status.appliedRevision || "None"}. {dirty && "Save your changes before checking or applying."}</p></>}
    {!canManage && <p>An Admin manages SMTP credentials and test messages.</p>}
    <form className="data-form" onSubmit={event => { event.preventDefault(); void action("save"); }}>
      <label htmlFor={`${id}-host`}>SMTP server<input id={`${id}-host`} required maxLength={253} autoComplete="off" placeholder="smtp.your-provider.com" disabled={readOnly} value={draft.host} onChange={event => field("host", event.target.value)} /></label>
      <label htmlFor={`${id}-port`}>Secure SMTP port<input id={`${id}-port`} value="465" readOnly /></label>
      <p>Verified TLS on port 465. STARTTLS-only providers are not supported by this native configuration.</p>
      <label htmlFor={`${id}-user`}>SMTP username<input id={`${id}-user`} required maxLength={320} autoComplete="off" disabled={readOnly} value={draft.username} onChange={event => field("username", event.target.value)} /></label>
      <label htmlFor={`${id}-password`}>SMTP password<input id={`${id}-password`} type="password" maxLength={2048} autoComplete="new-password" required={!status?.configuration.hasPassword || draft.host !== status.configuration.host || draft.username !== status.configuration.username} disabled={readOnly} value={password} onChange={event => { changed.current = true; setDirty(true); setPassword(event.target.value); setSendConfirmed(false); }} /></label>
      <p>{status?.configuration.hasPassword ? "A password is saved privately. Leave blank to retain it for the same server and username." : "Use your provider's SMTP credential or app password."}</p>
      <label htmlFor={`${id}-from`}>Sender email address<input id={`${id}-from`} type="email" required maxLength={254} autoComplete="off" disabled={readOnly} value={draft.fromAddress} onChange={event => field("fromAddress", event.target.value)} /></label>
      <label htmlFor={`${id}-name`}>Sender name<input id={`${id}-name`} required maxLength={128} disabled={readOnly} value={draft.senderName} onChange={event => field("senderName", event.target.value)} /></label>
      <label htmlFor={`${id}-origin`}>Application redirect address<input id={`${id}-origin`} type="url" required disabled={readOnly || status?.originSource === "public-domain"} value={status?.originSource === "public-domain" ? status.applicationOrigin : draft.applicationOrigin} placeholder="http://localhost:3000" onChange={event => field("applicationOrigin", event.target.value)} /></label>
      <p>{status?.originSource === "public-domain" ? "Your public data connection controls this application origin." : "Use your application's HTTPS origin or local address. Confirmation and recovery return users there; the app must handle the Auth callback."}</p>
      <div className="journey-actions"><Button type="submit" variant="primary" loading={activeAction === "save"} disabled={readOnly || !status}>Save email settings</Button><Button type="button" variant="secondary" loading={activeAction === "check"} disabled={readOnly || !saved || dirty} onClick={() => void action("check")}>Check SMTP connection</Button><Button type="button" variant="secondary" loading={activeAction === "apply"} disabled={readOnly || !saved || dirty || !ready} onClick={() => void action("apply")}>Apply to Auth</Button></div>
    </form>
    {!ready && <p>Start the data stack before applying settings or sending a test. Saving and connection checks remain available.</p>}
    {status?.connection.status === "passed" && status.connection.revision === status.revision && <p>Connection verified: TLS and SMTP authentication succeeded. This check sent no email.</p>}
    {status?.connection.status === "failed" && <p>The SMTP connection check failed. Correct the saved credentials or provider settings, then check again.</p>}
    <details className="data-disclosure"><summary>Send a test email</summary>
      <form className="data-form" onSubmit={event => { event.preventDefault(); void action("test"); }}>
        <p>A test uses the applied SMTP settings. It sends a fixed message and does not create or change an application user.</p>
        <label htmlFor={`${id}-recipient`}>Test recipient<input id={`${id}-recipient`} type="email" required maxLength={254} disabled={readOnly} value={recipient} onChange={event => { setRecipient(event.target.value); setSendConfirmed(false); }} /></label>
        <label className="data-check"><input type="checkbox" checked={sendConfirmed} disabled={readOnly} onChange={event => setSendConfirmed(event.target.checked)} />Send one test email to this recipient.</label>
        <Button type="submit" variant="secondary" loading={activeAction === "test"} disabled={readOnly || !ready || !applied || dirty || !recipient || !sendConfirmed}>Send test email</Button>
      </form>
      {status?.test.status === "accepted" && <p>SMTP accepted the test email. Check the recipient's inbox; delivery has not been confirmed.</p>}
      {status?.test.status === "uncertain" && <p>The previous test may have been sent. Check the inbox before explicitly requesting another test.</p>}
      {status?.test.status === "failed" && <p>The test was not accepted. Check the operation result and provider sending restrictions.</p>}
    </details>
    {saved && <details className="data-disclosure"><summary>Remove SMTP configuration</summary><p>This removes saved SMTP credentials after Auth is verified with email delivery disabled. Existing application users and data remain.</p><label className="data-check"><input type="checkbox" checked={removeConfirmed} disabled={readOnly} onChange={event => setRemoveConfirmed(event.target.checked)} />Remove SMTP credentials and disable confirmation and recovery email.</label><Button type="button" variant="tertiary" loading={activeAction === "disable"} disabled={readOnly || !ready || !removeConfirmed} onClick={() => void action("disable")}>Remove SMTP configuration</Button></details>}
  </section>;
}
