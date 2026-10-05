import { Button } from "../fluid/components/button";
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
  mode = "all",
  activeConnectionId,
  manageTargets = false,
}: {
  client: ProductClient;
  plan: CloudPlan | null;
  privateKeyPath?: string;
  mode?: "all" | "managed" | "existing";
  activeConnectionId?: string;
  manageTargets?: boolean;
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
    const [j, t, c] = await Promise.allSettled([
      mode === "existing" ? Promise.resolve({ jobs: [] }) : client.cloudJobs(),
      mode === "existing" && !manageTargets
        ? Promise.resolve({ targets: [] })
        : client.targets(),
      client.tunnels(),
    ]);
    if (alive.current) {
      if (j.status === "fulfilled") setJobs(j.value.jobs ?? []);
      if (t.status === "fulfilled") setTargets(t.value.targets ?? []);
      if (c.status === "fulfilled") setTunnels(c.value.connections ?? []);
    }
    // Account recovery must remain reachable when the remote target list rejects
    // expired credentials, while the local SSH connection is still available.
    for (const result of [c, j, t])
      if (result.status === "rejected") throw result.reason;
  };
  useEffect(() => {
    alive.current = true;
    void load().catch((e) => {
      if (alive.current) setError(errorText(e));
    });
    return () => {
      alive.current = false;
    };
  }, [client, mode, manageTargets]);
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
      {busy && <p role="status">{busy}…</p>}
      {activeConnectionId && (
        <WorkspaceInstaller
          client={client}
          connections={tunnels.filter(
            (connection) => connection.id === activeConnectionId,
          )}
          existing
        />
      )}
      {mode !== "existing" && plan && !created && (
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
          <Button type="button"
            variant="primary"
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
          </Button>
        </section>
      )}
      {mode !== "existing" && (
        <section className="setup-plan">
          <div className="cloud-section-head">
            <h2>Your cloud resources</h2>
            <Button type="button"
              variant="secondary"
              disabled={!!busy}
              onClick={() => void run("Refreshing", load)}
            >
              Refresh list
            </Button>
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
                <Button type="button"
                  variant="secondary"
                  disabled={!!busy || job.status === "deleted"}
                  onClick={() =>
                    void run("Checking cloud", async () =>
                      replaceJob(await client.refreshCloud(job.id)),
                    )
                  }
                >
                  Check progress
                </Button>
                {job.publicIp && job.status !== "deleted" && (
                  <>
                    <Button type="button"
                      variant="secondary"
                      disabled={!!busy}
                      onClick={() =>
                        void run("Checking DNS", async () => {
                          const result = await client.cloudDNS(job.id);
                          setDNS((v) => ({ ...v, [job.id]: result }));
                        })
                      }
                    >
                      Check DNS
                    </Button>
                    <Button type="button"
                      variant="secondary"
                      onClick={() => {
                        changeSSH({ host: job.publicIp });
                        document.getElementById("cloud-ssh-host")?.focus();
                      }}
                    >
                      Connect this server
                    </Button>
                  </>
                )}
                {!["deleted", "deleting", "creating"].includes(job.status) && (
                  <Button type="button"
                    variant="secondary"
                    disabled={!!busy}
                    onClick={() => {
                      setDeleting(job.id);
                      setConfirmName("");
                    }}
                  >
                    Remove resources…
                  </Button>
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
                    <strong>{job.name}</strong> to confirm. Billing ends only
                    when the provider has removed the resources.
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
                    <Button type="submit"
                      variant="secondary"
                      disabled={!!busy || confirmName !== job.name}
                    >
                      Delete these resources
                    </Button>
                    <Button
                      type="button"
                      variant="secondary"
                      onClick={() => setDeleting("")}
                    >
                      Keep resources
                    </Button>
                  </div>
                </form>
              )}
            </article>
          ))}
        </section>
      )}
      <section className="setup-plan">
        <h2>
          {activeConnectionId
            ? "Your private server connection"
            : mode === "existing"
              ? "Connect to your workspace server"
              : "Connect your server securely"}
        </h2>
        <p className="journey-muted">
          {activeConnectionId
            ? "Your selected workspace is already connected. Manage its sign-in above. Use the SSH controls below only if you need to reconnect the server."
            : mode === "existing"
              ? "Use the SSH details supplied by your workspace administrator. After verifying the server, sign in with your Luci workspace account below. Connecting does not create cloud resources."
              : "Luci opens a private SSH connection to your server. Public web traffic uses HTTPS; administration stays on this machine."}
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
                disabled={!!busy}
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
                disabled={!!busy}
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
                disabled={!!busy}
                value={ssh.user}
                onChange={(e) => changeSSH({ user: e.target.value })}
              />
            </label>
            <label className="luci-field">
              Private key path
              <input
                value={ssh.privateKeyPath}
                disabled={!!busy}
                onChange={(e) => changeSSH({ privateKeyPath: e.target.value })}
                placeholder="Absolute path, or use your SSH agent"
              />
            </label>
          </div>
          <Button type="submit" variant="secondary" disabled={!!busy}>
            Inspect server fingerprint
          </Button>
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
                disabled={!!busy}
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
                disabled={!!busy}
                checked={accepted}
                onChange={(e) => setAccepted(e.target.checked)}
              />
              I compared this fingerprint with my server’s trusted console.
            </label>
            <Button type="button"
              variant="primary"
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
            </Button>
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
              {mode !== "existing" &&
                t.status === "connected" &&
                localManagement(t.localUrl) && (
                  <Button type="button"
                    variant="secondary"
                    onClick={() => {
                      setManagement(localManagement(t.localUrl));
                      setBaseUrl(t.localUrl);
                    }}
                  >
                    Open server setup
                  </Button>
                )}
              {["connecting", "connected"].includes(t.status) && (
                <Button type="button"
                  variant="secondary"
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
                </Button>
              )}
            </div>
          </article>
        ))}
        {mode !== "existing" && management && (
          <div className="cloud-management">
            <div className="cloud-section-head">
              <strong>First account and access token</strong>
              <Button type="button" variant="secondary" onClick={() => setManagement("")}>
                Close server setup
              </Button>
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
      {!activeConnectionId && (
        <WorkspaceInstaller
          client={client}
          connections={tunnels}
          existing={mode === "existing"}
        />
      )}
      {(mode !== "existing" || manageTargets) && (
        <section className="setup-plan">
          <h2>Finish your deployment connection</h2>
          <p className="journey-muted">
            The access token stays in the selected workspace service. Luci
            verifies the server before adding it to Deploy.
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
            <Button type="submit" variant="primary" disabled={!!busy}>
              Verify and save connection
            </Button>
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
      )}
    </div>
  );
}
