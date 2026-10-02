import { useEffect, useRef, useState } from "react";
import {
  errorText,
  ProductApiError,
  type ProductClient,
  type CloudPlan,
  type CloudJob,
  type DNSResult,
  type DeploymentTarget,
  type SSHScan,
  type SSHRequest,
  type Tunnel,
} from "../../api/product";
import { WorkspaceInstaller } from "./WorkspaceInstaller";

export function CloudManager({
  client,
  plan,
  privateKeyPath = "",
}: {
  client: ProductClient;
  plan: CloudPlan | null;
  privateKeyPath?: string;
}) {
  const [jobs, setJobs] = useState<CloudJob[]>([]);
  const [targets, setTargets] = useState<DeploymentTarget[]>([]);
  const [tunnels, setTunnels] = useState<Tunnel[]>([]);
  const [cost, setCost] = useState(false),
    [unknownPrice, setUnknownPrice] = useState(false);
  const [busy, setBusy] = useState(""),
    [error, setError] = useState(""),
    [notice, setNotice] = useState("");
  const [deleting, setDeleting] = useState(""),
    [confirmName, setConfirmName] = useState("");
  const [dns, setDNS] = useState<Record<string, DNSResult>>({});
  const [ssh, setSSH] = useState<SSHRequest>({
    host: "",
    port: 22,
    user: "luci",
    privateKeyPath: "",
  });
  const [scan, setScan] = useState<SSHScan | null>(null),
    [fingerprint, setFingerprint] = useState("");
  const [accepted, setAccepted] = useState(false),
    [management, setManagement] = useState("");
  const [targetId, setTargetId] = useState(""),
    [targetName, setTargetName] = useState("My cloud");
  const [baseUrl, setBaseUrl] = useState(""),
    [token, setToken] = useState(""),
    [serverUuid, setServerUuid] = useState("");
  const [servers, setServers] = useState<{ uuid: string; name: string }[]>([]);
  const working = useRef(false),
    alive = useRef(true);
  const load = async () => {
    const [j, t, c] = await Promise.all([
      client.cloudJobs(),
      client.targets(),
      client.tunnels(),
    ]);
    if (alive.current) {
      setJobs(j.jobs ?? []);
      setTargets(t.targets ?? []);
      setTunnels(c.connections ?? []);
    }
  };
  useEffect(() => {
    alive.current = true;
    void load().catch((e) => {
      if (alive.current) setError(errorText(e));
    });
    return () => {
      alive.current = false;
    };
  }, [client]);
  useEffect(() => {
    setCost(false);
    setUnknownPrice(false);
  }, [plan?.id]);
  useEffect(() => {
    if (privateKeyPath) setSSH((value) => ({ ...value, privateKeyPath }));
  }, [privateKeyPath]);
  useEffect(() => {
    if (!tunnels.some((t) => t.status === "connecting")) return;
    const timer = setInterval(() => {
      void client
        .tunnels()
        .then((v) => {
          if (alive.current) setTunnels(v.connections);
        })
        .catch(() => {});
    }, 3000);
    return () => clearInterval(timer);
  }, [client, tunnels]);
  async function run(label: string, action: () => Promise<void>) {
    if (working.current) return;
    working.current = true;
    setBusy(label);
    setError("");
    setNotice("");
    try {
      await action();
    } catch (e) {
      if (alive.current) setError(errorText(e));
    } finally {
      working.current = false;
      if (alive.current) setBusy("");
    }
  }
  const replaceJob = (job: CloudJob) =>
    setJobs((items) => [job, ...items.filter((item) => item.id !== job.id)]);
  const changeSSH = (patch: Partial<SSHRequest>) => {
    setSSH({ ...ssh, ...patch });
    setScan(null);
    setAccepted(false);
    setFingerprint("");
  };
  const localManagement = (value: string) => {
    try {
      const u = new URL(value);
      return u.protocol === "http:" &&
        ["127.0.0.1", "localhost"].includes(u.hostname)
        ? u.toString()
        : "";
    } catch {
      return "";
    }
  };
  const created = plan && jobs.some((job) => job.planId === plan.id);
  return (
    <div className="cloud-manager">
      {error && (
        <p className="journey-error" role="alert">
          {error}
        </p>
      )}
      {notice && (
        <p className="journey-success" role="status">
          {notice}
        </p>
      )}
      {plan && !created && (
        <section className="setup-plan">
          <h2>Create this cloud home</h2>
          <p>
            Provision <strong>{plan.name}</strong> in{" "}
            {plan.provider.toUpperCase()} · {plan.region}. Charges continue
            while its resources exist.
          </p>
          <label className="luci-check">
            <input
              type="checkbox"
              checked={cost}
              onChange={(e) => setCost(e.target.checked)}
            />
            I accept charges in my own cloud account for this plan.
          </label>
          {!plan.estimate.verified && (
            <label className="luci-check">
              <input
                type="checkbox"
                checked={unknownPrice}
                onChange={(e) => setUnknownPrice(e.target.checked)}
              />
              I understand the monthly price is unverified and my budget is not
              a spending cap.
            </label>
          )}
          <button
            className="luci-button luci-primary"
            disabled={
              !!busy || !cost || (!plan.estimate.verified && !unknownPrice)
            }
            onClick={() =>
              void run("Creating resources", async () => {
                replaceJob(
                  await client.createCloud(plan.id, cost, unknownPrice),
                );
                setNotice(
                  "Provisioning requested. Follow its status below; your app is not deployed yet.",
                );
              })
            }
          >
            {busy === "Creating resources"
              ? "Requesting resources…"
              : "Create cloud resources"}
          </button>
        </section>
      )}
      <section className="setup-plan">
        <div className="cloud-section-head">
          <h2>Your cloud resources</h2>
          <button
            className="luci-button"
            disabled={!!busy}
            onClick={() => void run("Refreshing", load)}
          >
            Refresh list
          </button>
        </div>
        {!jobs.length && (
          <p className="journey-muted">
            Your cloud jobs will appear here. You can also connect an existing
            server below.
          </p>
        )}
        {jobs.map((job) => (
          <article className="cloud-job" key={job.id}>
            <div className="cloud-section-head">
              <strong>{job.name}</strong>
              <span
                className={`setup-status setup-status-${job.status === "failed" ? "error" : "unknown"}`}
              >
                {job.status.replace(/_/g, " ")}
              </span>
            </div>
            <p>{job.message}</p>
            <small>
              {job.provider.toUpperCase()} · {job.region} · {job.domain}
            </small>
            {job.publicIp && (
              <p>
                DNS A record: <code>{job.domain}</code> →{" "}
                <code>{job.publicIp}</code>
              </p>
            )}
            <div className="setup-actions">
              <button
                className="luci-button"
                disabled={!!busy || job.status === "deleted"}
                onClick={() =>
                  void run("Checking cloud", async () =>
                    replaceJob(await client.refreshCloud(job.id)),
                  )
                }
              >
                Check progress
              </button>
              {job.publicIp && job.status !== "deleted" && (
                <>
                  <button
                    className="luci-button"
                    disabled={!!busy}
                    onClick={() =>
                      void run("Checking DNS", async () => {
                        const result = await client.cloudDNS(job.id);
                        setDNS((v) => ({ ...v, [job.id]: result }));
                      })
                    }
                  >
                    Check DNS
                  </button>
                  <button
                    className="luci-button"
                    onClick={() => {
                      changeSSH({ host: job.publicIp });
                      document.getElementById("cloud-ssh-host")?.focus();
                    }}
                  >
                    Connect this server
                  </button>
                </>
              )}
              {!["deleted", "deleting", "creating"].includes(job.status) && (
                <button
                  className="luci-button"
                  disabled={!!busy}
                  onClick={() => {
                    setDeleting(job.id);
                    setConfirmName("");
                  }}
                >
                  Remove resources…
                </button>
              )}
            </div>
            {dns[job.id] && (
              <p
                className={
                  dns[job.id].ready ? "journey-success" : "journey-warning"
                }
                role="status"
              >
                {dns[job.id].message}
              </p>
            )}
            {deleting === job.id && (
              <form
                className="cloud-confirm"
                onSubmit={(e) => {
                  e.preventDefault();
                  void run("Removing resources", async () => {
                    replaceJob(await client.deleteCloud(job.id, confirmName));
                    setDeleting("");
                  });
                }}
              >
                <p>
                  Delete the machine and its data. Type{" "}
                  <strong>{job.name}</strong> to confirm. Billing ends only when
                  the provider has removed the resources.
                </p>
                <label className="luci-field">
                  Resource name to remove
                  <input
                    required
                    value={confirmName}
                    onChange={(e) => setConfirmName(e.target.value)}
                  />
                </label>
                <div className="setup-actions">
                  <button
                    className="luci-button"
                    disabled={!!busy || confirmName !== job.name}
                  >
                    Delete these resources
                  </button>
                  <button
                    type="button"
                    className="luci-button"
                    onClick={() => setDeleting("")}
                  >
                    Keep resources
                  </button>
                </div>
              </form>
            )}
          </article>
        ))}
      </section>
      <section className="setup-plan">
        <h2>Connect your server securely</h2>
        <p className="journey-muted">
          Luci opens a private SSH connection to the deployment service. Public
          web traffic uses HTTPS; administration stays on this machine.
        </p>
        <form
          onSubmit={(e) => {
            e.preventDefault();
            void run("Inspecting host", async () => {
              setScan(await client.inspectSSH(ssh));
              setAccepted(false);
              setFingerprint("");
            });
          }}
        >
          <div className="setup-field-grid">
            <label className="luci-field">
              Server address
              <input
                id="cloud-ssh-host"
                required
                value={ssh.host}
                onChange={(e) => changeSSH({ host: e.target.value })}
                placeholder="Your server’s public IP"
              />
            </label>
            <label className="luci-field">
              SSH port
              <input
                type="number"
                min="1"
                max="65535"
                required
                value={ssh.port}
                onChange={(e) => changeSSH({ port: Number(e.target.value) })}
              />
            </label>
            <label className="luci-field">
              SSH user
              <input
                required
                value={ssh.user}
                onChange={(e) => changeSSH({ user: e.target.value })}
              />
            </label>
            <label className="luci-field">
              Private key path
              <input
                value={ssh.privateKeyPath}
                onChange={(e) => changeSSH({ privateKeyPath: e.target.value })}
                placeholder="Absolute path, or use your SSH agent"
              />
            </label>
          </div>
          <button className="luci-button" disabled={!!busy}>
            Inspect server fingerprint
          </button>
        </form>
        {scan && (
          <div className="cloud-confirm">
            <p>
              Compare this host key with your provider’s console before trusting{" "}
              <strong>{scan.host}</strong>. A key shown here alone does not
              prove ownership.
            </p>
            <label className="luci-field">
              Server fingerprint
              <select
                value={fingerprint}
                onChange={(e) => {
                  setFingerprint(e.target.value);
                  setAccepted(false);
                }}
              >
                <option value="">Choose the matching fingerprint</option>
                {scan.fingerprints.map((f) => (
                  <option key={f} value={f}>
                    {f}
                  </option>
                ))}
              </select>
            </label>
            <label className="luci-check">
              <input
                type="checkbox"
                checked={accepted}
                onChange={(e) => setAccepted(e.target.checked)}
              />
              I compared this fingerprint with my server’s trusted console.
            </label>
            <button
              className="luci-button luci-primary"
              disabled={!!busy || !fingerprint || !accepted}
              onClick={() =>
                void run("Connecting SSH", async () => {
                  const tunnel = await client.connectSSH(scan.id, fingerprint);
                  setTunnels((v) => [tunnel, ...v]);
                  setBaseUrl(tunnel.localUrl);
                  setScan(null);
                })
              }
            >
              Trust and connect
            </button>
          </div>
        )}
        {tunnels.map((t) => (
          <article className="cloud-job" key={t.id}>
            <div className="cloud-section-head">
              <strong>{t.host}</strong>
              <span className="setup-status">{t.status}</span>
            </div>
            <p>{t.message}</p>
            <div className="setup-actions">
              {t.status === "connected" && localManagement(t.localUrl) && (
                <button
                  className="luci-button"
                  onClick={() => {
                    setManagement(localManagement(t.localUrl));
                    setBaseUrl(t.localUrl);
                  }}
                >
                  Open server setup
                </button>
              )}
              {["connecting", "connected"].includes(t.status) && (
                <button
                  className="luci-button"
                  disabled={!!busy}
                  onClick={() =>
                    void run("Disconnecting", async () => {
                      await client.stopTunnel(t.id);
                      if (management === t.localUrl) setManagement("");
                      await load();
                    })
                  }
                >
                  Disconnect
                </button>
              )}
            </div>
          </article>
        ))}
        {management && (
          <div className="cloud-management">
            <div className="cloud-section-head">
              <strong>First account and access token</strong>
              <button className="luci-button" onClick={() => setManagement("")}>
                Close server setup
              </button>
            </div>
            <p>
              Create the server’s first account, then an API token in its
              security settings. Paste the token below to finish connecting it
              to Luci.
            </p>
            <iframe
              title="Private server setup"
              src={management}
              sandbox="allow-scripts allow-forms allow-same-origin"
            />
          </div>
        )}
      </section>
      <WorkspaceInstaller client={client} connections={tunnels} />
      <section className="setup-plan">
        <h2>Finish your deployment connection</h2>
        <p className="journey-muted">
          The access token stays in Luci’s local service. Luci verifies the
          server before adding it to Deploy.
        </p>
        <form
          onSubmit={(e) => {
            e.preventDefault();
            void run("Verifying connection", async () => {
              try {
                const t = await client.connectTarget({
                  ...(targetId ? { id: targetId } : {}),
                  name: targetName,
                  baseUrl,
                  token,
                  ...(serverUuid ? { serverUuid } : {}),
                });
                setTargets((v) => [t, ...v.filter((x) => x.id !== t.id)]);
                setToken("");
                setServers([]);
                setNotice(
                  "Deployment server verified. Choose it in Deploy when your project is ready.",
                );
              } catch (cause) {
                if (cause instanceof ProductApiError) {
                  const body = cause.body as {
                    servers?: { uuid: string; name: string }[];
                  };
                  if (Array.isArray(body?.servers)) setServers(body.servers);
                }
                throw cause;
              }
            });
          }}
        >
          {targets.length > 0 && (
            <label className="luci-field">
              Connection to save
              <select
                value={targetId}
                onChange={(e) => {
                  const id = e.target.value,
                    t = targets.find((x) => x.id === id);
                  setTargetId(id);
                  if (t) {
                    setTargetName(t.name);
                    setBaseUrl(t.baseUrl);
                    setServerUuid(t.serverUuid);
                  } else {
                    setServerUuid("");
                  }
                  setToken("");
                  setServers([]);
                }}
              >
                <option value="">Add a new connection</option>
                {targets.map((t) => (
                  <option key={t.id} value={t.id}>
                    Update {t.name}
                  </option>
                ))}
              </select>
              <small>
                When reconnecting SSH, update the existing connection with its
                new local address.
              </small>
            </label>
          )}
          <div className="setup-field-grid">
            <label className="luci-field">
              Connection name
              <input
                required
                value={targetName}
                onChange={(e) => setTargetName(e.target.value)}
              />
            </label>
            <label className="luci-field">
              Management address
              <input
                type="url"
                required
                value={baseUrl}
                onChange={(e) => {
                  setBaseUrl(e.target.value);
                  setServerUuid("");
                  setServers([]);
                }}
                placeholder="http://127.0.0.1:…"
              />
            </label>
          </div>
          <label className="luci-field">
            Server API token
            <input
              type="password"
              required
              value={token}
              autoComplete="off"
              onChange={(e) => setToken(e.target.value)}
            />
          </label>
          {servers.length > 0 && (
            <label className="luci-field">
              Application server
              <select
                required
                value={serverUuid}
                onChange={(e) => setServerUuid(e.target.value)}
              >
                <option value="">Choose a server</option>
                {servers.map((s) => (
                  <option key={s.uuid} value={s.uuid}>
                    {s.name || s.uuid}
                  </option>
                ))}
              </select>
            </label>
          )}
          <button className="luci-button luci-primary" disabled={!!busy}>
            Verify and save connection
          </button>
        </form>
        {targets.length > 0 && (
          <ul className="cloud-targets">
            {targets.map((t) => (
              <li key={t.id}>
                <strong>{t.name}</strong>
                <span>{t.baseUrl}</span>
              </li>
            ))}
          </ul>
        )}
      </section>
    </div>
  );
}
