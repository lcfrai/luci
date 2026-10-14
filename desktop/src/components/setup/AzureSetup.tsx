import { useCallback, useEffect, useId, useLayoutEffect, useMemo, useRef, useState, type FormEvent } from "react";
import { errorText, type ProductClient } from "../../api/product";
import { azurePrerequisites, createAzureSetupClient, readAzureConnectionDescriptor, readAzureSetupProgress, type AzureConnectionDescriptor, type AzureSetupPlan, type AzureSetupProgress, type AzureSetupRequest } from "../../api/azureSetup";
import { TEAM_WORKSPACE_CHANGED } from "../../api/teamScope";
import { AUTH_EXPIRED } from "../../api/authSession";
import { Button } from "../fluid/components/button";
import { CloudManager } from "./CloudManager";
import "./azure-setup.css";

const stageNames: Record<string, string> = {
  credentials: "Secure credentials",
  images: "Coding environment image",
  "runtime-gallery": "Coding environment publication",
  "controller-gallery": "Controller image",
  infrastructure: "Cloud resources",
  connection: "Connection",
  retirement: "Cleanup",
};
function stageSummary(state: string, uncertain: boolean): string {
  if (uncertain) return "Outcome uncertain · reconcile before continuing";
  if (state === "not_started") return "Not started";
  return "Recorded by installer · review before continuing";
}

export function AzureSetup({ client, disabled, budget = 30 }: { client: ProductClient; disabled: boolean; budget?: number }) {
  const api = useMemo(() => createAzureSetupClient(client), [client]);
  const id = useId();
  const [input, setInput] = useState<AzureSetupRequest>({ name: "luci", region: "australiaeast", monthlyBudgetUsd: budget > 0 ? budget : 30, maxConcurrentVMs: 1 });
  const [plan, setPlan] = useState<AzureSetupPlan | null>(null);
  const [descriptor, setDescriptor] = useState<AzureConnectionDescriptor | null>(null);
  const [progress, setProgress] = useState<AzureSetupProgress | null>(null);
  const [verified, setVerified] = useState(false), [busy, setBusy] = useState("");
  const [error, setError] = useState(""), [notice, setNotice] = useState("");
  const alive = useRef(true), allowed = useRef(!disabled), generation = useRef(0), pending = useRef<AbortController | null>(null);
  allowed.current = !disabled;
  const invalidate = useCallback(() => {
    generation.current++; pending.current?.abort(); pending.current = null;
    setPlan(null); setDescriptor(null); setProgress(null); setVerified(false); setBusy(""); setError(""); setNotice("");
  }, []);
  useLayoutEffect(() => { invalidate(); }, [client, disabled, invalidate]);
  useEffect(() => {
    alive.current = true;
    window.addEventListener(TEAM_WORKSPACE_CHANGED, invalidate);
    window.addEventListener(AUTH_EXPIRED, invalidate);
    return () => {
      alive.current = false; generation.current++; pending.current?.abort();
      window.removeEventListener(TEAM_WORKSPACE_CHANGED, invalidate);
      window.removeEventListener(AUTH_EXPIRED, invalidate);
    };
  }, [invalidate]);
  const update = (patch: Partial<AzureSetupRequest>) => { invalidate(); setInput(value => ({ ...value, ...patch })); };
  async function run(label: string, action: (signal: AbortSignal, current: () => boolean) => Promise<void>) {
    if (!allowed.current || pending.current) return;
    const controller = new AbortController(), request = ++generation.current;
    pending.current = controller; setBusy(label); setError(""); setNotice("");
    const current = () => alive.current && allowed.current && generation.current === request && !controller.signal.aborted;
    try { await action(controller.signal, current); }
    catch (cause) { if (current()) setError(errorText(cause)); }
    finally { if (pending.current === controller) { pending.current = null; if (alive.current) setBusy(""); } }
  }
  const prepare = (event: FormEvent) => {
    event.preventDefault();
    setPlan(null); setDescriptor(null); setProgress(null); setVerified(false);
    void run("Preparing instructions…", async (signal, current) => {
      const result = await api.plan(input, signal);
      if (current()) setPlan(result);
    });
  };
  const download = () => {
    if (!plan) return;
    setVerified(false);
    void run("Verifying installer…", async (signal, current) => {
      const blob = await api.download(plan, signal);
      if (!current()) return;
      const url = URL.createObjectURL(blob), link = document.createElement("a");
      link.href = url; link.download = "luci-azure-setup.tar";
      document.body.appendChild(link); link.click(); link.remove();
      setTimeout(() => URL.revokeObjectURL(url), 30000);
      setVerified(true); setNotice("Verified installer download started. Your workspace is still unconnected.");
    });
  };
  const copy = () => {
    if (!plan || !verified) return;
    void run("Copying commands…", async (_signal, current) => {
      if (!navigator.clipboard?.writeText) throw new Error("Copy is unavailable here. Select the verified commands below and copy them manually.");
      await navigator.clipboard.writeText(plan.commands.join("\n"));
      if (current()) setNotice("Setup commands copied. Run them on your Linux or WSL2 setup computer after reviewing the installer guide.");
    });
  };
  const importProgress = (file?: File) => {
    if (!file || !plan) return;
    void run("Reading stage receipt…", async (_signal, current) => {
      const result = await readAzureSetupProgress(file, plan, descriptor);
      if (current()) { setProgress(result); setNotice("Installer stages loaded from a local receipt. Cloud access, cost and workspace connection are still unverified."); }
    });
  };
  const importConnection = (file?: File) => {
    if (!file || !plan) return;
    void run("Reading controller identity…", async (_signal, current) => {
      const result = await readAzureConnectionDescriptor(file, plan);
      if (!current()) return;
      setDescriptor(result);
      setProgress(value => value && (value.scope !== result.scope || value.selectionDigest !== result.selectionDigest) ? null : value);
      setNotice("Connection file loaded. Inspect the server and compare its key with your Azure setup readback before connecting.");
    });
  };
  return <section className="azure-setup" aria-label="Guided Azure setup">
    <p>Run setup, load its files, then connect your workspace. You will review Azure resources and costs before creating them.</p>
    {disabled && <p role="status" className="journey-warning">An administrator with cloud provisioning permission must prepare this setup.</p>}
    <form onSubmit={prepare}>
      <fieldset disabled={disabled || !!busy}>
        <legend>1. Run setup</legend>
        <div className="azure-setup-fields">
          <div className="luci-field"><label htmlFor={`${id}-name`}>Deployment name</label><input id={`${id}-name`} aria-describedby={`${id}-name-help`} required pattern={"[a-z][a-z0-9\\-]{1,39}"} maxLength={40} value={input.name} onChange={event => update({ name: event.target.value })} /><small id={`${id}-name-help`}>2–40 lowercase letters, numbers or hyphens; start with a letter.</small></div>
          <div className="luci-field"><label htmlFor={`${id}-region`}>Azure region</label><input id={`${id}-region`} aria-describedby={`${id}-region-help`} required pattern={"[a-z][a-z0-9\\-]{1,39}"} maxLength={40} value={input.region} onChange={event => update({ region: event.target.value })} /><small id={`${id}-region-help`}>Use an Azure region ID, such as australiaeast. Confirm the required VM capacity there.</small></div>
          <div className="luci-field"><label htmlFor={`${id}-budget`}>Monthly budget (USD)</label><input id={`${id}-budget`} aria-describedby={`${id}-budget-help`} type="number" required min="0.01" max="100000" step="0.01" value={input.monthlyBudgetUsd} onChange={event => update({ monthlyBudgetUsd: Number(event.target.value) })} /><small id={`${id}-budget-help`}>Review actual Azure charges before creating resources. This does not cap provider billing.</small></div>
          <div className="luci-field"><label htmlFor={`${id}-capacity`}>Concurrent task VMs</label><input id={`${id}-capacity`} aria-describedby={`${id}-capacity-help`} type="number" required min="1" max="1024" step="1" value={input.maxConcurrentVMs} onChange={event => update({ maxConcurrentVMs: Number(event.target.value) })} /><small id={`${id}-capacity-help`}>Each upstream task gets its own isolated VM, within account capacity and your policy.</small></div>
        </div>
        <Button type="submit">{busy === "Preparing instructions…" ? busy : plan ? "Prepare fresh Azure instructions" : "Prepare Azure instructions"}</Button>
      </fieldset>
    </form>
    {error && <p role="alert" className="journey-error">{error}</p>}
    {notice && <p role="status">{notice}</p>}
    {plan && <div className="setup-plan azure-setup-plan">
      <h3>Instructions prepared · not connected</h3>
      <p>Download the installer and run it on Linux or WSL2. Follow its guided steps and review costs before creating resources.</p>
      <div className="setup-actions">
        <Button type="button" disabled={disabled || !!busy} onClick={download}>{busy === "Verifying installer…" ? busy : "Download verified Azure installer"}</Button>
        <Button type="button" variant="secondary" disabled={disabled || !!busy || !verified} onClick={copy}>Copy setup commands</Button>
      </div>
      {!verified && <p className="journey-muted">Download and verify this exact installer before copying its commands.</p>}
      {verified && <div className="azure-setup-commands"><h4>Run in Linux or WSL2</h4><p>Start in a new private folder. Follow the bundled guide for secure sign-in, safe retries and cleanup.</p><pre tabIndex={0} aria-label="Verified Azure setup commands">{plan.commands.join("\n")}</pre></div>}
      <details><summary>Prerequisites and installer identity</summary>
        <p>{plan.setupHostRequirement}</p>
        {azurePrerequisites(plan.missingInputs).map(group => <section key={group.title}><h4>{group.title}</h4><ul>{group.items.map(item => <li key={item}>{item}</li>)}</ul></section>)}
        <dl><dt>Installer source commit</dt><dd><code>{plan.sourceCommit}</code></dd><dt>Download SHA-256</dt><dd><code>{plan.bundleSHA256}</code></dd></dl>
      </details>
      <section className="azure-setup-followup" aria-label="Azure installer progress and connection">
        <h3>2. Load setup files</h3>
        <p>Load the two files produced by your installer. They show progress and the private server address; you still need a private route and workspace sign-in.</p>
        <details><summary>Commands to save these files</summary><p>Run these from the private folder used for setup. Generate the connection file only after the installer finishes its controller setup. If any step has an uncertain outcome, reconcile its saved receipt before retrying.</p><pre tabIndex={0}>python3 luci-azure-setup/azure-setup/installer.py status --bundle luci-azure-setup --selection luci-azure-private/selection.json &gt; luci-azure-private/status.json{"\n"}python3 luci-azure-setup/azure-setup/installer.py connection-descriptor --bundle luci-azure-setup --selection luci-azure-private/selection.json</pre></details>
        <div className="azure-setup-receipts">
          <label className="luci-field">Setup progress file
            <input type="file" accept=".json,application/json" disabled={disabled || !!busy} onChange={event => { importProgress(event.target.files?.[0]); event.target.value = ""; }} />
            <small>Choose <code>status.json</code>.</small>
          </label>
          <label className="luci-field">Connection file
            <input type="file" accept=".json,application/json" disabled={disabled || !!busy} onChange={event => { importConnection(event.target.files?.[0]); event.target.value = ""; }} />
            <small>Choose <code>azure-connection.json</code>. No credential is imported.</small>
          </label>
        </div>
        {progress && <div role="status" className="azure-setup-progress"><h4>Setup progress · from your files</h4><ol>{progress.stages.map(stage => <li key={stage.id}><strong>{stageNames[stage.id]}</strong>: {stageSummary(stage.state, stage.uncertain)}</li>)}</ol><p>These files do not verify a live connection or final Azure charges.</p><details><summary>Exact stage states</summary><dl>{progress.stages.map(stage => <div key={stage.id}><dt>{stageNames[stage.id]}</dt><dd><code>{stage.state}</code></dd></div>)}</dl></details></div>}
        {descriptor && <div className="azure-setup-controller"><h3>3. Connect workspace</h3><p>Use your private route and SSH key, then sign in with your Luci workspace account.</p><details><summary>Server identity and receipt details</summary><p>Private address <code>{descriptor.host}</code>; controller VM <code>{descriptor.vmID}</code>; generation <code>{descriptor.generation}</code>. The imported file claims a host readback, but it does not independently prove Azure ownership. Compare its fingerprint with your authenticated Azure setup readback before trusting the server.</p></details><CloudManager key={`${descriptor.scope}:${descriptor.generation}:${descriptor.fingerprint}`} client={client} plan={null} mode="existing" azureDescriptor={descriptor} /></div>}
      </section>
    </div>}
  </section>;
}
