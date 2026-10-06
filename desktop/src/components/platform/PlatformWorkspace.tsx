import { useCallback, useEffect, useRef, useState } from "react";
import { errorText, productApi, type GitDelivery } from "../../api/product";
import { prepareBrowserAccess } from "../../api/browserAccess";
import { selectedTeamId } from "../../api/teamScope";
import { notifyProjectContextChanged } from "../../api/projectContextEvents";
import { useProjectDraftScope } from "../../hooks/useProjectDraft";
import { useProjectAccess } from "../../hooks/useProjectAccess";
import {
  platformApi,
  isolatedBrowserURL,
  isGatewayHostname,
  type PanaceaConnection,
  type PanaceaProject,
  type PlatformDeployment,
  type PlatformProject,
  type PlatformStatus,
} from "../../api/platform";
import { DataWorkspace } from "./DataWorkspace";
import { StandaloneDatabase } from "./StandaloneDatabase";
import "./platform.css";

export function PlatformWorkspace({ projectId, inspectDeploymentId, inspectionRequest }: { projectId: string; inspectDeploymentId?: string; inspectionRequest?: number }) {
  const scope = useProjectDraftScope(projectId);
  return <ProjectPlatform key={scope ?? projectId} projectId={projectId} inspectDeploymentId={inspectDeploymentId} inspectionRequest={inspectionRequest} />;
}

function ProjectPlatform({ projectId, inspectDeploymentId, inspectionRequest }: { projectId: string; inspectDeploymentId?: string; inspectionRequest?: number }) {
  const projectAccess = useProjectAccess(projectId);
  const scope = useProjectDraftScope(projectId);
  const canDeploy = projectAccess.canEdit && projectAccess.permissions.deploy;
  const canReadLogs = !projectAccess.loading && !projectAccess.error && projectAccess.canRead && projectAccess.permissions.deploy && projectAccess.permissions.secrets;
  const logPermission = useRef(canReadLogs); logPermission.current = canReadLogs;
  const browserPermission = useRef(projectAccess.canExecute); browserPermission.current = projectAccess.canExecute;
  const active = useRef(true), connectingBrowser = useRef(false), operating = useRef(false);
  const requestGeneration = useRef(0);
  const providerGeneration = useRef(0);
  useEffect(() => { active.current = true; return () => { active.current = false; }; }, []);
  const [status, setStatus] = useState<PlatformStatus | null>(null);
  const [project, setProject] = useState<PlatformProject | null>(null);
  const [delivery, setDelivery] = useState<GitDelivery | null>(null);
  const [connection, setConnection] = useState<PanaceaConnection | null>(null);
  const [remoteProjects, setRemoteProjects] = useState<PanaceaProject[]>([]);
  const [busy, setBusy] = useState("");
  const [serverBusy, setServerBusy] = useState(false);
  const [error, setError] = useState("");
  const [providerError, setProviderError] = useState("");
  const [providerLoading, setProviderLoading] = useState<"" | "connection" | "projects" | "updating">("");
  const [view, setView] = useState<
    "Overview" | "Deployments" | "Data" | "Hosting connection"
  >(inspectDeploymentId ? "Deployments" : "Overview");
  const deploymentNodes = useRef(new Map<string, HTMLElement>());
  const inspected = useRef<string | undefined>(undefined);
  const [inspectionIssue, setInspectionIssue] = useState("");
  useEffect(() => { if (inspectDeploymentId) setView("Deployments"); }, [inspectDeploymentId, inspectionRequest]);
  useEffect(() => {
    if (!inspectDeploymentId || !project || !status || view !== "Deployments" || inspected.current === `${inspectDeploymentId}:${inspectionRequest ?? 0}`) return;
    inspected.current = `${inspectDeploymentId}:${inspectionRequest ?? 0}`;
    const node = deploymentNodes.current.get(inspectDeploymentId);
    if (node) { setInspectionIssue(""); node.scrollIntoView?.({ block: "nearest" }); node.focus(); }
    else setInspectionIssue("The selected application is no longer in this project's history. Refresh application history before choosing another deployment.");
  }, [inspectDeploymentId, inspectionRequest, project, status, view]);
  const [message, setMessage] = useState("");
  const [replacement, setReplacement] = useState<{ previous: PlatformDeployment; commit: string } | null>(null);
  const [replacementAccepted, setReplacementAccepted] = useState(false);
  const buildForm = useRef<HTMLFormElement>(null);
  const [port, setPort] = useState("3000");
  const [healthPath, setHealthPath] = useState("/");
  const [baseUrl, setBaseUrl] = useState("");
  const [apiKey, setApiKey] = useState("");
  const [remoteProject, setRemoteProject] = useState("");
  const [logs, setLogs] = useState<{
    id: string;
    text: string;
    notice: string;
  } | null>(null);
  const reloadProvider = useCallback(async () => {
    // Local operations can fail without refreshing. Their generation must not
    // invalidate a provider lookup that can still finish independently.
    const generation = ++providerGeneration.current;
    const current = () => active.current && generation === providerGeneration.current;
    setProviderLoading("connection");
    setProviderError("");
    setRemoteProjects([]);
    try {
      const nextConnection = await platformApi.connection();
      if (!current()) return;
      setConnection(nextConnection);
      if (nextConnection.configured) {
        setProviderLoading("projects");
        const remote = await platformApi.projects();
        if (current()) setRemoteProjects(remote.projects);
      }
    } catch {
      if (!current()) return;
      setRemoteProjects([]);
      setProviderError(
        "Panacea is unavailable. Your Docker services and database remain available.",
      );
    } finally {
      if (current()) setProviderLoading("");
    }
  }, []);
  const reload = useCallback(async () => {
    const generation = ++requestGeneration.current;
    const [nextProject, nextDelivery] = await Promise.all([
      platformApi.project(projectId),
      productApi.gitDelivery(projectId),
    ]);
    if (!active.current || generation !== requestGeneration.current) return null;
    if (nextProject.project.projectId !== projectId)
      throw new Error("Luci could not verify this project's application services. Refresh the platform and try again.");
    setProject(nextProject.project);
    setServerBusy(nextProject.busy || nextProject.project.databaseOperation?.status === "running");
    setDelivery(nextDelivery);
    setRemoteProject(nextProject.project.panaceaProject || "");
    // Hosting discovery is optional. Do not hold local capability admission or
    // a completed Docker operation behind the provider's network timeout.
    void reloadProvider();
    return generation;
  }, [projectId, reloadProvider]);
  useEffect(() => {
    let active = true;
    const local = reload();
    const generation = requestGeneration.current;
    Promise.all([platformApi.status(), local])
      .then(([value, loaded]) => {
        if (active && loaded === generation && generation === requestGeneration.current) setStatus(value);
      })
      .catch((cause) => {
        if (active && generation === requestGeneration.current) setError(errorText(cause));
      });
    return () => {
      active = false;
    };
  }, [reload]);
  useEffect(() => {
    if (!serverBusy) return;
    let current = true, inFlight = false;
    const timer = window.setInterval(() => {
      if (inFlight) return;
      inFlight = true;
      const generation = requestGeneration.current;
      void platformApi
        .project(projectId)
        .then((value) => {
          if (!current || !active.current || generation !== requestGeneration.current) return;
          if (value.project.projectId !== projectId)
            throw new Error("Luci could not verify this project's operation progress. Refresh the platform.");
          setProject(value.project);
          setServerBusy(value.busy || value.project.databaseOperation?.status === "running");
          setError("");
        })
        .catch((cause) => {
          if (current && active.current && generation === requestGeneration.current) setError(errorText(cause));
        })
        .finally(() => { inFlight = false; });
    }, 2500);
    return () => { current = false; window.clearInterval(timer); };
  }, [serverBusy, projectId]);
  async function run<T>(label: string, task: () => Promise<T>, permitted = true, options: { afterReload?: (result: T) => void; providerMutation?: boolean } = {}) {
    if (!permitted || busy || serverBusy || operating.current || !active.current) return;
    operating.current = true;
    ++requestGeneration.current;
    setBusy(label);
    setError("");
    setMessage("");
    let providerRefreshed = false;
    if (options.providerMutation) {
      ++providerGeneration.current;
      setProviderLoading("updating");
      setRemoteProjects([]);
      setProviderError("");
    }
    try {
      const result = await task();
      if (!active.current) return;
      const generation = await reload();
      providerRefreshed = generation !== null;
      if (!active.current || generation !== requestGeneration.current) return;
      options.afterReload?.(result);
      const state =
        result && typeof result === "object" && "status" in result
          ? String(result.status)
          : "";
      const observedMessage = result && typeof result === "object" && "message" in result ? String(result.message) : "";
      if (["failed", "unknown", "attention", "interrupted", "unhealthy"].includes(state)) {
        setError(`${label} needs attention. ${observedMessage || "A healthy outcome has not been verified."} Inspect the saved deployment and its logs before choosing recovery.`);
      } else setMessage(
        state === "running"
          ? "Database recovery accepted. Its verified progress and result appear below."
          : state === "building" || state === "deploying" ? "Deployment accepted and running. Readiness has not been verified yet."
          : ["queued", "preparing", "verifying", "stopping"].includes(state)
            ? "The operation is still in progress. Its final outcome has not been verified."
          : state === "ready" ? `The selected service was verified ready. ${observedMessage}`
          : state === "stopped" ? `The selected service is stopped. ${observedMessage}`
          : `${label} completed.`,
      );
    } catch (cause) {
      if (active.current) setError(errorText(cause));
    } finally {
      if (active.current && options.providerMutation && !providerRefreshed) void reloadProvider();
      operating.current = false;
      if (active.current) setBusy("");
    }
  }
  const runHosting = (label: string, task: () => Promise<unknown>, permitted: boolean) => run(label, task, permitted && !providerLoading, { providerMutation: true });
  const deploy = (label: string, task: () => Promise<unknown>) => run(label, async () => { const result = await task(); notifyProjectContextChanged(scope, projectId); return result; }, canDeploy);
  useEffect(() => { if (!canReadLogs) setLogs(null); }, [canReadLogs]);
  const disabled = !!busy || serverBusy;
  const hostingDisabled = disabled || !!providerLoading;
  const hostingProjectsReady = !providerLoading && !providerError && !!connection?.configured;
  const providerStatus = providerLoading === "connection" ? "Checking Panacea connection…"
    : providerLoading === "projects" ? "Loading Panacea projects…"
    : providerLoading === "updating" ? "Updating Panacea hosting…" : "";
  const sourceReady = !!delivery?.head;
  const safeURL = (raw?: string) => {
    if (!raw) return undefined;
    try {
      const url = new URL(raw);
      if (url.username || url.password || url.origin === window.location.origin) return undefined;
      return url.protocol === "https:" ||
        (url.protocol === "http:" && (url.hostname === "127.0.0.1" || isGatewayHostname(url.hostname)))
        ? url.href
        : undefined;
    } catch {
      return undefined;
    }
  };
  async function refreshDeployment(deployment: PlatformDeployment) {
    if (deployment.provider === "panacea")
      return await platformApi.refreshPanacea(projectId, deployment.id);
    else return await platformApi.action(projectId, deployment.id, "refresh");
  }
  async function openWorkspaceApplication(deployment: PlatformDeployment) {
    if (disabled || connectingBrowser.current || !projectAccess.canExecute) return;
    const popup = selectedTeamId() ? window.open('about:blank', '_blank') : null;
    if (popup) popup.opener = null;
    connectingBrowser.current = true;
    setBusy("Connecting application browser"); setError(""); setMessage("");
    try {
      const connection = deployment.browserConnectionRequired
        ? await platformApi.prepareBrowser(projectId, "application", deployment.id)
        : {url: deployment.url || '', projectId, kind: 'application', resourceId: deployment.id};
      if (!active.current || !browserPermission.current) { popup?.close(); return; }
      const address = isolatedBrowserURL(connection.url);
      if (!address || connection.projectId !== projectId || connection.kind !== "application" || connection.resourceId !== deployment.id)
        throw new Error("Luci could not verify this application's browser connection. Reconnect your workspace and try again.");
      setProject(current => current ? { ...current, deployments: current.deployments.map(row => row.id === deployment.id ? { ...row, url: address, browserConnectionRequired: false } : row) } : current);
      setMessage("Browser connection ready. You can open the application below.");
      const opened = await prepareBrowserAccess(projectId, address);
      if (active.current && browserPermission.current) {
        if (popup) popup.location.replace(opened);
        else window.open(opened, "_blank", "noopener,noreferrer");
      } else popup?.close();
    } catch (cause) { popup?.close(); if (active.current) setError(errorText(cause)); }
    finally { connectingBrowser.current = false; if (active.current) setBusy(""); }
  }
  return (
    <section
      className="journey-section platform-workspace"
      aria-label="Application platform"
    >
      <div className="journey-section-heading">
        <div>
          <h2>Application platform</h2>
          <p>Run your application and its database as persistent services.</p>
        </div>
        <button
          type="button"
          className="luci-button"
          disabled={disabled}
          onClick={() =>
            void run("Refresh platform", platformApi.status, true, { afterReload: setStatus })
          }
        >
          Refresh platform
        </button>
      </div>
      {projectAccess.loading ? <p role="status">Checking platform permissions…</p>
        : projectAccess.error ? <p role="status">Platform permissions could not be confirmed. {projectAccess.error} Luci must confirm access before you can operate applications or open private logs and previews.</p>
          : <>
            {!canDeploy && <p role="status">Deployment access is required to build, stop, restart or check deployment health. Ask a team owner for editing and deployment access.</p>}
            {!canReadLogs && <p role="status">Application logs require deployment and secrets access. Ask a team owner for those permissions.</p>}
            {!projectAccess.canExecute && <p role="status">Opening private application previews requires execution access.</p>}
          </>}
      {inspectionIssue && <p role="alert" className="journey-error">{inspectionIssue}</p>}
      {error && (
        <p className="journey-error" role="alert">
          {error}
        </p>
      )}
      {(busy || serverBusy || message) && (
        <p role="status">
          {busy
            ? `${busy}…`
            : serverBusy
              ? "A platform operation is running. Its real result will appear here."
              : message}
        </p>
      )}
      {providerStatus && <p role="status">{providerStatus}</p>}
      {!status && !error && (
        <p role="status">Checking platform capabilities…</p>
      )}
      {status && (
        <>
          <nav className="journey-actions" aria-label="Platform sections">
            {(
              ["Overview", "Deployments", "Data", "Hosting connection"] as const
            ).map((section) => (
              <button
                key={section}
                type="button"
                className="luci-button"
                aria-pressed={view === section}
                onClick={() => setView(section)}
              >
                {section}
              </button>
            ))}
          </nav>
          {view === "Overview" && (
            <section aria-label="Platform overview">
              <h3>Your running services</h3>
              <dl>
                <dt>Docker engine</dt>
                <dd>{status.dockerReady ? "Available" : "Not available"}</dd>
                <dt>Application deployments</dt>
                <dd>
                  {project?.deployments.filter(
                    (item) => item.status === "ready",
                  ).length || 0}{" "}
                  last verified healthy
                </dd>
                <dt>PostgreSQL</dt>
                <dd>{project?.database.status || "not-created"}</dd>
                <dt>Panacea</dt>
                <dd>
                  {providerLoading === "connection" || providerLoading === "updating"
                    ? "Checking connection"
                    : providerError ? "Unavailable"
                    : connection?.configured
                    ? `Connected to ${connection.baseUrl}`
                    : connection ? "Not connected" : "Connection not yet checked"}
                </dd>
              </dl>
              <p>{status.supabase.message}</p>
              <p>
                Deployments are checked when started or explicitly refreshed. A
                saved ready status is the last verified result.
              </p>
            </section>
          )}
          <p>
            {status.scope} Runtime limit per service: {status.limits.memoryMiB}{" "}
            MiB memory and {(status.limits.cpuMillis / 1000).toFixed(1)} CPU.
            Stopped services retain their data.
          </p>
          {view === "Deployments" && (
            <form
              ref={buildForm} tabIndex={-1}
              className="platform-form"
              onSubmit={(event) => {
                event.preventDefault();
                if (delivery && canDeploy && (!replacement || replacementAccepted))
                  void deploy(replacement ? "Start replacement deployment" : "Start persistent deployment", async () => {
                    const result = await platformApi.deploy(projectId, replacement?.commit ?? delivery.head, Number(port), healthPath);
                    if (active.current) { setReplacement(null); setReplacementAccepted(false); }
                    return result;
                  });
              }}
            >
              <h3>{replacement ? "Review replacement deployment" : "Persistent Docker deployment"}</h3>
              {replacement && <><p>Earlier deployment: <code>{replacement.previous.commit}</code>. Replacement commit: <code>{replacement.commit}</code>.</p>
                <p className="journey-warning">This creates a separate application using the selected commit and current runtime configuration and database bindings. The earlier deployment and its data are retained. Review the replacement's health and behavior before stopping the earlier service.</p>
                {replacement.previous.commit !== replacement.commit && <p>The source has changed. This replacement needs new checks and review; it is not a rollback to the earlier version.</p>}
                <label className="setup-check"><input type="checkbox" checked={replacementAccepted} disabled={disabled} onChange={event => setReplacementAccepted(event.target.checked)} />Create this replacement and retain the earlier deployment.</label>
                <button type="button" className="luci-button" disabled={disabled} onClick={() => { setReplacement(null); setReplacementAccepted(false); }}>Keep existing deployment</button></>}
              <p>
                Build the committed Dockerfile and run on this workspace’s
                Docker host. This is a local service, not public internet
                hosting. A root Dockerfile and committed approved project
                context are required.
              </p>
              <p>
                Source:{" "}
                {delivery?.head
                  ? `${delivery.branch} · ${delivery.head.slice(0, 12)}`
                  : "Commit your project in Develop first."}
              </p>
              <label className="platform-field">
                Application port
                <input
                  type="number"
                  min="1"
                  max="65535"
                  required
                  value={port}
                  disabled={disabled}
                  onChange={(event) => setPort(event.target.value)}
                />
              </label>
              <label className="platform-field">
                Health check path
                <input
                  required
                  value={healthPath}
                  disabled={disabled}
                  onChange={(event) => setHealthPath(event.target.value)}
                />
              </label>
              {!status.dockerReady && (
                <p role="status">
                  Start Docker to deploy an application or database.
                </p>
              )}
              <button
                className="luci-button luci-primary"
                disabled={disabled || !canDeploy || !status.dockerReady || !sourceReady || (!!replacement && !replacementAccepted)}
              >
                {replacement ? "Start replacement deployment" : "Start persistent deployment"}
              </button>
            </form>
          )}
          {view === "Data" && (
            <section aria-label="Project database">
              <DataWorkspace projectId={projectId} dockerReady={status.dockerReady} />
              <StandaloneDatabase projectId={projectId} project={project}
                dockerReady={status.dockerReady} disabled={disabled} run={run}
                onDeployments={() => setView("Deployments")} />
            </section>
          )}
          {view === "Hosting connection" && (
            <section className="platform-block" aria-label="Panacea hosting">
              <h3>Connect Panacea hosting</h3>
              {providerError && <p role="alert">{providerError}</p>}
              <p>
                Use an existing Panacea control plane and a scoped personal API
                key. Luci calls its real project and deployment APIs. Existing
                hosting charges and provider permissions apply.
              </p>
              {connection?.configured ? (
                <>
                  <p>
                    Connected to <strong>{connection.baseUrl}</strong>
                  </p>
                  <form
                    className="platform-form"
                    onSubmit={(event) => {
                      event.preventDefault();
                      void runHosting("Connect Panacea project", () =>
                        platformApi.mapProject(projectId, remoteProject),
                        hostingProjectsReady && remoteProjects.some(item => item.name === remoteProject),
                      );
                    }}
                  >
                    <label className="platform-field">
                      Panacea project
                      <select
                        value={remoteProject}
                        disabled={hostingDisabled || !hostingProjectsReady}
                        required
                        onChange={(event) =>
                          setRemoteProject(event.target.value)
                        }
                      >
                        <option value="">{providerLoading === "projects" ? "Loading accessible projects…" : providerError ? "Projects unavailable" : "Choose an accessible project"}</option>
                        {remoteProjects.map((item) => (
                          <option key={item.name} value={item.name}>
                            {item.displayName || item.name}
                          </option>
                        ))}
                      </select>
                    </label>
                    <button
                      className="luci-button"
                      disabled={hostingDisabled || !hostingProjectsReady || !remoteProjects.some(item => item.name === remoteProject)}
                    >
                      Connect Panacea project
                    </button>
                  </form>
                  <p>
                    Push the reviewed commit to this Panacea project’s
                    repository before deploying. Provider runtime configuration
                    remains managed by Panacea.
                  </p>
                  <button
                    className="luci-button luci-primary"
                    disabled={
                      hostingDisabled || !hostingProjectsReady ||
                      !project?.panaceaProject ||
                      !sourceReady ||
                      !status.permissions.deployPanacea || !canDeploy
                    }
                    onClick={() => {
                      if (delivery && hostingProjectsReady && project?.panaceaProject && canDeploy && status.permissions.deployPanacea)
                        void deploy("Request Panacea deployment", () =>
                          platformApi.deployPanacea(
                            projectId,
                            delivery.head,
                            delivery.branch,
                          ),
                        );
                    }}
                  >
                    Deploy through Panacea
                  </button>
                  {connection.canManage && (
                    <button
                      className="luci-button"
                      disabled={hostingDisabled}
                      onClick={() =>
                        void runHosting("Disconnect Panacea", platformApi.disconnect, !!connection?.canManage)
                      }
                    >
                      Disconnect Panacea
                    </button>
                  )}
                </>
              ) : connection?.canManage ? (
                <form
                  className="platform-form"
                  onSubmit={(event) => {
                    event.preventDefault();
                    const key = apiKey;
                    setApiKey("");
                    void runHosting("Verify Panacea connection", () =>
                      platformApi.connect(baseUrl, key),
                      !!connection?.canManage,
                    );
                  }}
                >
                  <label className="platform-field">
                    Panacea HTTPS origin
                    <input
                      type="url"
                      placeholder="https://panacea.example.com"
                      required
                      value={baseUrl}
                      disabled={hostingDisabled}
                      onChange={(event) => setBaseUrl(event.target.value)}
                    />
                  </label>
                  <label className="platform-field">
                    Scoped Panacea API key
                    <input
                      type="password"
                      autoComplete="off"
                      required
                      minLength={16}
                      value={apiKey}
                      disabled={hostingDisabled}
                      onChange={(event) => setApiKey(event.target.value)}
                    />
                  </label>
                  <button
                    className="luci-button"
                    disabled={hostingDisabled || !apiKey}
                  >
                    Verify and save connection
                  </button>
                  <p>
                    The key is kept in private workspace state and is never
                    returned to this form.
                  </p>
                </form>
              ) : connection && !providerLoading ? (
                <p>An administrator can connect this workspace to Panacea.</p>
              ) : null}
            </section>
          )}
          {view === "Deployments" && (
            <section aria-label="Persistent deployment history">
              <h3>Deployments</h3>
              {project?.deployments.length === 0 && (
                <p>No persistent deployments yet.</p>
              )}
              {project?.deployments.map((deployment) => (
                <article className="platform-block" key={deployment.id} tabIndex={-1} ref={node => { if (node) deploymentNodes.current.set(deployment.id, node); else deploymentNodes.current.delete(deployment.id); }} aria-label={`Deployment ${deployment.id}`}>
                  <h4>
                    {deployment.provider === "docker"
                      ? "Docker service"
                      : "Panacea hosting"}{" "}
                    · {deployment.commit.slice(0, 12)}
                  </h4>
                  <p>
                    <strong>{deployment.status}</strong> · {deployment.message}
                  </p>
                  {deployment.status === "ready" && deployment.provider === "docker" && (deployment.browserConnectionRequired || selectedTeamId()) && (
                    <button type="button" className="luci-button" disabled={disabled || !projectAccess.canExecute} onClick={() => void openWorkspaceApplication(deployment)}>
                      Open deployed application
                    </button>
                  )}
                  {deployment.status === "ready" && !deployment.browserConnectionRequired && !selectedTeamId() && safeURL(deployment.url) && (
                    <a
                      className="luci-button"
                      href={safeURL(deployment.url)}
                      target="_blank"
                      rel="noreferrer"
                    >
                      Open deployed application
                    </a>
                  )}
                  <div className="journey-actions">
                    <button
                      className="luci-button"
                      disabled={disabled || !canDeploy}
                      onClick={() =>
                        void deploy("Verify deployment health", () =>
                          refreshDeployment(deployment),
                        )
                      }
                    >
                      Verify deployment health
                    </button>
                    {deployment.provider === "docker" && (
                      <>
                        <button
                          className="luci-button"
                          disabled={disabled || !canDeploy || deployment.status === "stopped"}
                          onClick={() =>
                            void deploy("Stop deployment", () =>
                              platformApi.action(
                                projectId,
                                deployment.id,
                                "stop",
                              ),
                            )
                          }
                        >
                          Stop deployment
                        </button>
                        <button
                          className="luci-button"
                          disabled={disabled || !canDeploy}
                          onClick={() =>
                            void deploy("Restart deployment", () =>
                              platformApi.action(
                                projectId,
                                deployment.id,
                                "restart",
                              ),
                            )
                          }
                        >
                          Restart deployment
                        </button>
                        {["failed", "stopped", "unknown"].includes(deployment.status) && <button type="button" className="luci-button" disabled={disabled || !canDeploy || !sourceReady} onClick={() => {
                          if (!delivery || disabled || !canDeploy) return;
                          setReplacement({ previous: deployment, commit: delivery.head }); setReplacementAccepted(false);
                          buildForm.current?.scrollIntoView?.({ block: "nearest" }); buildForm.current?.focus();
                        }}>Prepare replacement deployment</button>}
                        <button
                          className="luci-button"
                          disabled={disabled || !canReadLogs}
                          onClick={() =>
                            void run("Read application logs", async () => {
                              const result = await platformApi.logs(projectId, deployment.id);
                              if (active.current && logPermission.current) setLogs({ id: deployment.id, ...result });
                            }, canReadLogs)
                          }
                        >
                          Read application logs
                        </button>
                      </>
                    )}
                  </div>
                  {["failed", "stopped", "unknown"].includes(deployment.status) && <p className="journey-muted">Inspect logs before recovery. Restart uses the retained container and image; if it is missing or its database binding changed, prepare a replacement from reviewed source. Neither action restores database contents.</p>}
                  {logs?.id === deployment.id && (
                    <>
                      <p>{logs.notice}</p>
                      <pre className="journey-output">
                        {logs.text || "No application output yet."}
                      </pre>
                    </>
                  )}
                </article>
              ))}
            </section>
          )}
        </>
      )}
    </section>
  );
}
