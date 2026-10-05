import { Button } from "../fluid/components/button";
import { useEffect, useId, useRef, useState } from "react";
import { dependencyApi, type Dependency, type DependencyAction, type DependencyClient, type DependencyId, type DependencyStatus } from "../../api/dependencies";
import { errorText } from "../../api/product";

const active = (dependency: Dependency) => dependency.job?.status === "queued" || dependency.job?.status === "installing";
const actions: Record<DependencyAction, string> = {
  none: "",
  check: "Check tools to see what is available.",
  install_winget: "Install Microsoft App Installer, then check tools again.",
  elevation: "Finish the Windows administrator approval, then check tools again.",
  restart_app: "Save your work, close Luci and reopen it, then check tools again.",
  restart_computer: "Save your work and restart this computer, then reopen Luci and check tools.",
  start_docker: "Open Docker Desktop and wait for its engine to start, then check workspace readiness.",
  manual_install: "Install this tool with its official installer, then check tools again.",
};
const purpose: Record<DependencyId, string> = {
  git: "Keeps project history and enables imports.",
  docker: "Runs isolated agents, previews and local application services.",
  github: "Connects GitHub repositories and static publishing when you choose them.",
  aws: "Connects your AWS account when you choose AWS hosting.",
  azure: "Connects your Azure account when you choose Azure hosting.",
};
const statusLabel = (dependency: Dependency) => {
  if (active(dependency)) return dependency.job!.status === "queued" ? "Queued" : "Installing";
  if (dependency.job?.status === "failed") return "Installation failed";
  if (dependency.job?.status === "interrupted") return "Installation interrupted";
  if (dependency.job?.status === "action_required" || dependency.check.status === "action_required") return "Action needed";
  return { ready: "Installed", unchecked: "Not checked", missing: "Not installed" }[dependency.check.status];
};

/** These jobs belong to the local control-plane host, even if a cloud workspace is selected. */
export function DependencySetup({ client = dependencyApi, canManage, automatic = false, cloudWorkspace = false, target, onChecked }: {
  client?: DependencyClient;
  canManage: boolean;
  automatic?: boolean;
  cloudWorkspace?: boolean;
  target?: "aws" | "azure";
  onChecked?(): void;
}) {
  const [status, setStatus] = useState<DependencyStatus | null>(null);
  const [busy, setBusy] = useState("");
  const [error, setError] = useState("");
  const [consent, setConsent] = useState<Partial<Record<DependencyId, boolean>>>({});
  const [attempt, setAttempt] = useState(0);
  const alive = useRef(false), running = useRef(false), autoChecked = useRef(false);
  const onCheckedRef = useRef(onChecked);
  onCheckedRef.current = onChecked;
  const heading = useId();
  useEffect(() => {
    alive.current = true;
    let cancelled = false;
    setBusy("Loading tools"); setError("");
    void client.get().then(async (value) => {
      if (cancelled) return;
      setStatus(value);
      if (automatic && canManage && !autoChecked.current && value.dependencies.some(item => item.check.status === "unchecked")) {
        autoChecked.current = true;
        setBusy("Checking tools");
        const checked = await client.check();
        if (!cancelled) { setStatus(checked); onCheckedRef.current?.(); }
      }
    }).catch(cause => { if (!cancelled) setError(errorText(cause)); })
      .finally(() => { if (!cancelled) setBusy(""); });
    return () => { cancelled = true; alive.current = false; };
  }, [client, automatic, canManage, attempt]);
  const hasActiveJob = status?.dependencies.some(active) === true;
  useEffect(() => {
    if (!hasActiveJob || busy || error) return;
    let cancelled = false;
    const timer = setTimeout(() => {
      void client.get().then(value => {
        if (cancelled) return;
        setStatus(value);
        if (!value.dependencies.some(active)) onCheckedRef.current?.();
      }).catch(cause => { if (!cancelled) setError(errorText(cause)); });
    }, 2500);
    return () => { cancelled = true; clearTimeout(timer); };
  }, [client, status, busy, error, hasActiveJob]);
  async function run(label: string, operation: () => Promise<void>) {
    if (running.current) return;
    running.current = true; setBusy(label); setError("");
    try { await operation(); }
    catch (cause) { if (alive.current) setError(errorText(cause)); }
    finally { running.current = false; if (alive.current) setBusy(""); }
  }
  function row(dependency: Dependency) {
    const installing = active(dependency);
    const needsFreshCheck = !!dependency.job && !installing;
    const action = dependency.job?.status === "action_required" ? dependency.job.actionRequired : dependency.actionRequired;
    const installable = canManage && status?.canInstall && status.installer.status === "ready" && dependency.supported && !installing && !needsFreshCheck && dependency.check.status === "missing" && !["restart_app", "restart_computer", "elevation"].includes(action);
    return <li key={dependency.id} className="dependency-row">
      <div className="dependency-row-heading"><h3>{dependency.id === "docker" && status?.platform !== "windows" ? "Docker engine" : dependency.name}</h3><span className="setup-status" role={installing ? "status" : undefined}>{statusLabel(dependency)}</span></div>
      <p className="journey-muted">{purpose[dependency.id]}</p>
      <p>{dependency.job?.message || dependency.check.message || dependency.reason}</p>
      {dependency.check.version && <p className="journey-muted">Detected version: {dependency.check.version}</p>}
      {actions[action] && <p className="preparation-action">{actions[action]}</p>}
      {needsFreshCheck && <p className="preparation-action">Choose “Check tools again” to confirm the result before trying another installation.</p>}
      {!dependency.supported && dependency.check.status !== "ready" && dependency.reason && <p className="journey-muted">{dependency.reason}</p>}
      {dependency.id === "docker" && dependency.check.status !== "ready" && <p className="journey-muted">Installing Docker does not start its engine. Workspace readiness checks the engine separately.</p>}
      {installable && <div className="dependency-install">
        <label className="luci-check"><input type="checkbox" checked={consent[dependency.id] === true} disabled={!!busy || hasActiveJob} onChange={event => setConsent(value => ({ ...value, [dependency.id]: event.target.checked }))} />I accept the publisher and package source agreements for {dependency.name}.</label>
        <p className="journey-muted">Windows Package Manager installs {dependency.packageId}. Windows may request administrator approval or a restart.</p>
        <Button type="button" variant="secondary" disabled={!consent[dependency.id] || !!busy || hasActiveJob} onClick={() => void run(`Starting ${dependency.name} installation`, async () => {
          const job = await client.install(dependency.id, true);
          if (!alive.current) return;
          setConsent(value => ({ ...value, [dependency.id]: false }));
          setStatus(value => value ? { ...value, dependencies: value.dependencies.map(item => item.id === dependency.id ? { ...item, job } : item) } : value);
          if (!["queued", "installing"].includes(job.status)) onCheckedRef.current?.();
        })}>Install {dependency.name}</Button>
      </div>}
    </li>;
  }
  return <section className="dependency-setup" aria-labelledby={heading}>
    <h2 id={heading}>{status?.platform === "linux" ? "Workspace tools" : "Tools on this computer"}</h2>
    <p className="journey-muted">Luci checks the computer running this connection service. Installations download the selected publisher’s software; no model is called.{cloudWorkspace && " These installations do not change your selected cloud workspace."}</p>
    {busy && <p role="status">{busy}…</p>}
    {error && <div className="journey-error" role="alert">{error} <Button type="button" variant="secondary" disabled={!!busy} onClick={() => setAttempt(value => value + 1)}>Retry loading tools</Button></div>}
    {status && <>
      {status.installer.status !== "ready" && status.dependencies.some(item => item.check.status !== "ready") && <p className="journey-muted">{status.installer.message}</p>}
      {!canManage && <p className="journey-muted">Your administrator installs tools on this computer. You can keep defining your project.</p>}
      <ul className="dependency-list">{status.dependencies.filter(item => target ? item.id === target : item.id === "git" || item.id === "docker").map(row)}</ul>
      <details className="dependency-optional"><summary>Optional publishing and cloud tools</summary><p className="journey-muted">Install only the tools for services you choose. Installing a CLI does not sign you in or create paid resources.</p><ul className="dependency-list">{status.dependencies.filter(item => target ? item.id !== target : !["git", "docker"].includes(item.id)).map(row)}</ul></details>
      {canManage && <Button type="button" variant="secondary" disabled={!!busy || hasActiveJob} onClick={() => void run("Checking tools", async () => {
        const checked = await client.check();
        if (alive.current) { setStatus(checked); setConsent({}); onCheckedRef.current?.(); }
      })}>Check tools again</Button>}
    </>}
  </section>;
}
