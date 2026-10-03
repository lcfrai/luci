import { useCallback, useEffect, useState } from "react";
import { errorText, productApi, type GitDelivery } from "../../api/product";
import {
  platformApi,
  type PanaceaConnection,
  type PanaceaProject,
  type PlatformDeployment,
  type PlatformProject,
  type PlatformStatus,
} from "../../api/platform";

export function PlatformWorkspace({ projectId }: { projectId: string }) {
  return <ProjectPlatform key={projectId} projectId={projectId} />;
}

function ProjectPlatform({ projectId }: { projectId: string }) {
  const [status, setStatus] = useState<PlatformStatus | null>(null);
  const [project, setProject] = useState<PlatformProject | null>(null);
  const [delivery, setDelivery] = useState<GitDelivery | null>(null);
  const [connection, setConnection] = useState<PanaceaConnection | null>(null);
  const [remoteProjects, setRemoteProjects] = useState<PanaceaProject[]>([]);
  const [busy, setBusy] = useState("");
  const [serverBusy, setServerBusy] = useState(false);
  const [error, setError] = useState("");
  const [providerError, setProviderError] = useState("");
  const [view, setView] = useState<
    "Overview" | "Deployments" | "Data" | "Hosting connection"
  >("Overview");
  const [message, setMessage] = useState("");
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
  const reload = useCallback(async () => {
    const [nextProject, nextDelivery] = await Promise.all([
      platformApi.project(projectId),
      productApi.gitDelivery(projectId),
    ]);
    setProject(nextProject.project);
    setServerBusy(nextProject.busy);
    setDelivery(nextDelivery);
    setRemoteProject(nextProject.project.panaceaProject || "");
    setProviderError("");
    try {
      const nextConnection = await platformApi.connection();
      setConnection(nextConnection);
      if (nextConnection.configured)
        setRemoteProjects((await platformApi.projects()).projects);
      else setRemoteProjects([]);
    } catch {
      setRemoteProjects([]);
      setProviderError(
        "Panacea is unavailable. Your Docker services and database remain available.",
      );
    }
  }, [projectId]);
  useEffect(() => {
    let active = true;
    Promise.all([platformApi.status(), reload()])
      .then(([value]) => {
        if (active) setStatus(value);
      })
      .catch((cause) => {
        if (active) setError(errorText(cause));
      });
    return () => {
      active = false;
    };
  }, [reload]);
  useEffect(() => {
    if (!serverBusy) return;
    const timer = window.setInterval(() => {
      void platformApi
        .project(projectId)
        .then((value) => {
          setProject(value.project);
          setServerBusy(value.busy);
        })
        .catch((cause) => {
          setError(errorText(cause));
          setServerBusy(false);
        });
    }, 2500);
    return () => window.clearInterval(timer);
  }, [serverBusy, projectId]);
  async function run(label: string, task: () => Promise<unknown>) {
    if (busy || serverBusy) return;
    setBusy(label);
    setError("");
    setMessage("");
    try {
      const result = await task();
      await reload();
      const state =
        result && typeof result === "object" && "status" in result
          ? String(result.status)
          : "";
      setMessage(
        state === "building" || state === "deploying"
          ? "Deployment accepted and running. Readiness has not been verified yet."
          : `${label} completed.`,
      );
    } catch (cause) {
      setError(errorText(cause));
    } finally {
      setBusy("");
    }
  }
  const disabled = !!busy || serverBusy;
  const sourceReady = !!delivery?.head;
  const safeURL = (raw?: string) => {
    if (!raw) return undefined;
    try {
      const url = new URL(raw);
      return url.protocol === "https:" ||
        (url.protocol === "http:" && url.hostname === "127.0.0.1")
        ? url.href
        : undefined;
    } catch {
      return undefined;
    }
  };
  async function refreshDeployment(deployment: PlatformDeployment) {
    if (deployment.provider === "panacea")
      await platformApi.refreshPanacea(projectId, deployment.id);
    else await platformApi.action(projectId, deployment.id, "refresh");
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
            void run("Refresh platform", async () => {
              setStatus(await platformApi.status());
            })
          }
        >
          Refresh platform
        </button>
      </div>
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
                  {connection?.configured
                    ? `Connected to ${connection.baseUrl}`
                    : "Not connected"}
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
              className="cloud-deploy-form"
              onSubmit={(event) => {
                event.preventDefault();
                if (delivery)
                  void run("Start persistent deployment", () =>
                    platformApi.deploy(
                      projectId,
                      delivery.head,
                      Number(port),
                      healthPath,
                    ),
                  );
              }}
            >
              <h3>Persistent Docker deployment</h3>
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
              <label>
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
              <label>
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
                disabled={disabled || !status.dockerReady || !sourceReady}
              >
                Start persistent deployment
              </button>
            </form>
          )}
          {view === "Data" && (
            <section
              className="cloud-deploy-form"
              aria-label="Project database"
            >
              <h3>PostgreSQL database</h3>
              <p>
                {project?.database.message ||
                  "Create a private database on this Docker host."}
              </p>
              <p>
                A Docker volume retains the data. New deployments receive
                DATABASE_URL privately. This does not install Supabase
                authentication, object storage or realtime.
              </p>
              <p>
                Status:{" "}
                <strong>{project?.database.status || "not-created"}</strong>
              </p>
              <div className="journey-actions">
                <button
                  className="luci-button"
                  disabled={
                    disabled ||
                    !status.dockerReady ||
                    project?.database.status === "ready"
                  }
                  onClick={() =>
                    void run("Start PostgreSQL", () =>
                      platformApi.database(projectId, "start"),
                    )
                  }
                >
                  Start PostgreSQL
                </button>
                <button
                  className="luci-button"
                  disabled={disabled || project?.database.status !== "ready"}
                  onClick={() =>
                    void run("Stop PostgreSQL", () =>
                      platformApi.database(projectId, "stop"),
                    )
                  }
                >
                  Stop PostgreSQL
                </button>
                <button
                  className="luci-button"
                  disabled={disabled || project?.database.status !== "ready"}
                  onClick={() =>
                    void run("Create database backup", () =>
                      platformApi.database(projectId, "backup"),
                    )
                  }
                >
                  Create database backup
                </button>
              </div>
              <p>
                Backups are private PostgreSQL custom-format dumps, up to 64
                MiB. Restore is not automated here; use pg_restore into a
                separate database.
              </p>
              {project?.backups.map((backup) => (
                <p key={backup.id}>
                  <button
                    className="luci-button"
                    disabled={disabled}
                    onClick={() =>
                      void run("Download database backup", () =>
                        platformApi.downloadBackup(projectId, backup),
                      )
                    }
                  >
                    Download backup{" "}
                    {new Date(backup.createdAt).toLocaleString()}
                  </button>{" "}
                  {Math.ceil(backup.bytes / 1024)} KiB · SHA-256{" "}
                  <code>{backup.sha256.slice(0, 16)}…</code>
                </p>
              ))}
            </section>
          )}
          {view === "Hosting connection" && (
            <section className="cloud-deploy-form" aria-label="Panacea hosting">
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
                    onSubmit={(event) => {
                      event.preventDefault();
                      void run("Connect Panacea project", () =>
                        platformApi.mapProject(projectId, remoteProject),
                      );
                    }}
                  >
                    <label>
                      Panacea project
                      <select
                        value={remoteProject}
                        disabled={disabled}
                        required
                        onChange={(event) =>
                          setRemoteProject(event.target.value)
                        }
                      >
                        <option value="">Choose an accessible project</option>
                        {remoteProjects.map((item) => (
                          <option key={item.name} value={item.name}>
                            {item.displayName || item.name}
                          </option>
                        ))}
                      </select>
                    </label>
                    <button
                      className="luci-button"
                      disabled={disabled || !remoteProject}
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
                      disabled ||
                      !project?.panaceaProject ||
                      !sourceReady ||
                      !status.permissions.deployPanacea
                    }
                    onClick={() => {
                      if (delivery)
                        void run("Request Panacea deployment", () =>
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
                      disabled={disabled}
                      onClick={() =>
                        void run("Disconnect Panacea", platformApi.disconnect)
                      }
                    >
                      Disconnect Panacea
                    </button>
                  )}
                </>
              ) : connection?.canManage ? (
                <form
                  onSubmit={(event) => {
                    event.preventDefault();
                    const key = apiKey;
                    setApiKey("");
                    void run("Verify Panacea connection", () =>
                      platformApi.connect(baseUrl, key),
                    );
                  }}
                >
                  <label>
                    Panacea HTTPS origin
                    <input
                      type="url"
                      placeholder="https://panacea.example.com"
                      required
                      value={baseUrl}
                      disabled={disabled}
                      onChange={(event) => setBaseUrl(event.target.value)}
                    />
                  </label>
                  <label>
                    Scoped Panacea API key
                    <input
                      type="password"
                      autoComplete="off"
                      required
                      minLength={16}
                      value={apiKey}
                      disabled={disabled}
                      onChange={(event) => setApiKey(event.target.value)}
                    />
                  </label>
                  <button
                    className="luci-button"
                    disabled={disabled || !apiKey}
                  >
                    Verify and save connection
                  </button>
                  <p>
                    The key is kept in private workspace state and is never
                    returned to this form.
                  </p>
                </form>
              ) : (
                <p>An administrator can connect this workspace to Panacea.</p>
              )}
            </section>
          )}
          {view === "Deployments" && (
            <section aria-label="Persistent deployment history">
              <h3>Deployments</h3>
              {project?.deployments.length === 0 && (
                <p>No persistent deployments yet.</p>
              )}
              {project?.deployments.map((deployment) => (
                <article className="cloud-deploy-form" key={deployment.id}>
                  <h4>
                    {deployment.provider === "docker"
                      ? "Docker service"
                      : "Panacea hosting"}{" "}
                    · {deployment.commit.slice(0, 12)}
                  </h4>
                  <p>
                    <strong>{deployment.status}</strong> · {deployment.message}
                  </p>
                  {deployment.status === "ready" && safeURL(deployment.url) && (
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
                      disabled={disabled}
                      onClick={() =>
                        void run("Verify deployment health", () =>
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
                          disabled={disabled || deployment.status === "stopped"}
                          onClick={() =>
                            void run("Stop deployment", () =>
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
                          disabled={disabled}
                          onClick={() =>
                            void run("Restart deployment", () =>
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
                        <button
                          className="luci-button"
                          disabled={disabled}
                          onClick={() =>
                            void run("Read application logs", async () => {
                              setLogs({
                                id: deployment.id,
                                ...(await platformApi.logs(
                                  projectId,
                                  deployment.id,
                                )),
                              });
                            })
                          }
                        >
                          Read application logs
                        </button>
                      </>
                    )}
                  </div>
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
