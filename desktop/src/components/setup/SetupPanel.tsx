import {
  useEffect,
  useLayoutEffect,
  useRef,
  useState,
  type FormEvent,
  type KeyboardEvent,
} from "react";
import {
  productApi,
  errorText,
  type CloudPlan,
  type SetupConfig,
  type SetupStatus,
  type ProductClient,
} from "../../api/product";
import { useDismissable } from "../../hooks/useDismissable";
import { useCockpit } from "../../store/store";
import { CloudManager } from "./CloudManager";
import { LocalTools } from "./LocalTools";
import { ProviderSignIn } from "./ProviderSignIn";
import { changeWorkspace } from "./workspace";
import { ProjectRestore } from "../journey/ProjectBackup";
import { useAuth } from "../auth/AuthProvider";
import "./setup.css";

interface Props {
  initialStatus: SetupStatus | null;
  initialSection?: "local" | "project" | "cloud";
  onStatus(value: SetupStatus): void;
  onClose(): void;
  onProjectsChanged(): void;
  client?: ProductClient;
}

export function SetupPanel({
  initialStatus,
  initialSection = "local",
  onStatus,
  onClose,
  onProjectsChanged,
  client = productApi,
}: Props) {
  const auth = useAuth();
  const manageBacking = !auth || auth.profile.permissions.manageBacking;
  const allowProvision = !auth || auth.profile.policy.allowCloudProvisioning;
  const [status, setStatus] = useState(initialStatus);
  const [config, setConfig] = useState<SetupConfig | null>(
    initialStatus?.config ?? null,
  );
  const [section, setSection] = useState(initialSection);
  const [error, setError] = useState("");
  const [notice, setNotice] = useState("");
  const [busy, setBusy] = useState("");
  const [name, setName] = useState("");
  const [remote, setRemote] = useState("");
  const [cloudName, setCloudName] = useState("luci");
  const [size, setSize] = useState<"small" | "standard">("small");
  const [sshPublicKey, setSshPublicKey] = useState("");
  const [privateKeyPath, setPrivateKeyPath] = useState("");
  const [plan, setPlan] = useState<CloudPlan | null>(null);
  const alive = useRef(true);
  const inFlight = useRef(false);
  const heading = useRef<HTMLHeadingElement>(null);
  const panel = useDismissable<HTMLDivElement>(true, () => {
    if (!inFlight.current) onClose();
  });
  useLayoutEffect(() => {
    heading.current?.focus();
  }, []);
  useEffect(() => {
    alive.current = true;
    return () => {
      alive.current = false;
    };
  }, []);
  const receive = (value: SetupStatus) => {
    setStatus(value);
    setConfig(value.config);
    onStatus(value);
  };
  async function run(label: string, action: () => Promise<void>) {
    if (inFlight.current) return;
    inFlight.current = true;
    setBusy(label);
    setError("");
    setNotice("");
    try {
      await action();
    } catch (cause) {
      if (alive.current) setError(errorText(cause));
    } finally {
      inFlight.current = false;
      if (alive.current) setBusy("");
    }
  }
  useEffect(() => {
    if (initialStatus) return;
    let cancelled = false;
    client
      .status()
      .then((value) => {
        if (!cancelled) receive(value);
      })
      .catch((cause) => {
        if (!cancelled) setError(errorText(cause));
      });
    return () => {
      cancelled = true;
    };
    // The initial snapshot belongs to this opening of Settings.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [client]);
  const update = (patch: Partial<SetupConfig>) => {
    if (config) setConfig({ ...config, ...patch });
    setPlan(null);
  };
  function trapFocus(event: KeyboardEvent<HTMLDivElement>) {
    if (event.key !== "Tab") return;
    const items = Array.from(
      event.currentTarget.querySelectorAll<HTMLElement>(
        "button:not(:disabled), input:not(:disabled), select:not(:disabled), textarea:not(:disabled), a[href]",
      ),
    );
    const first = items[0],
      last = items[items.length - 1];
    if (
      event.shiftKey &&
      (document.activeElement === first ||
        document.activeElement === heading.current)
    ) {
      event.preventDefault();
      last?.focus();
    } else if (!event.shiftKey && document.activeElement === last) {
      event.preventDefault();
      first?.focus();
    }
  }
  const submitConfig = (event: FormEvent) => {
    event.preventDefault();
    if (!config || !manageBacking) return;
    void run("Saving", async () => {
      receive(await client.configure(config));
      onProjectsChanged();
      setNotice("Settings saved. Your projects folder is ready.");
    });
  };
  return (
    <div className="setup-backdrop">
      <div
        className="setup-window"
        ref={panel}
        role="dialog"
        aria-modal="true"
        aria-labelledby="setup-title"
        onKeyDown={trapFocus}
      >
        <header className="setup-header">
          <div>
            <span className="setup-brand">
              Luci <span>by lcfr.ai</span>
            </span>
            <h1 id="setup-title" ref={heading} tabIndex={-1}>
              {status?.configured
                ? "Settings"
                : "Make room for your next idea."}
            </h1>
            <p>
              {status?.workspace?.mode === "cloud"
                ? `Cloud workspace · ${status.workspace.host}`
                : "Your projects, your machine. Move your workspace to your cloud when you choose."}
            </p>
          </div>
          <button
            className="setup-close"
            type="button"
            onClick={onClose}
            disabled={!!busy}
            aria-label="Close settings"
          >
            ×
          </button>
        </header>
        <nav className="setup-tabs" aria-label="Settings sections">
          {(
            [
              [
                "local",
                status?.workspace?.mode === "cloud"
                  ? "This workspace"
                  : "This machine",
              ],
              ["project", "New project"],
              ["cloud", "Your cloud"],
            ] as const
          )
            .filter(([id]) => id !== "cloud" || manageBacking)
            .map(([id, label]) => (
              <button
                key={id}
                type="button"
                aria-current={section === id ? "page" : undefined}
                onClick={() => setSection(id)}
              >
                {label}
              </button>
            ))}
        </nav>
        <div className="setup-content">
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
          {!config ? (
            <div>
              <p>Connect to Luci’s local service to load your settings.</p>
              <button
                className="luci-button"
                disabled={!!busy}
                onClick={() =>
                  void run("Connecting", async () =>
                    receive(await client.status()),
                  )
                }
              >
                Retry connection
              </button>
            </div>
          ) : (
            <>
              {section === "local" && (
                <>
                  {manageBacking && status?.workspace?.mode === "cloud" && (
                    <div className="workspace-current">
                      <h2>Working on {status.workspace.host}</h2>
                      <p className="journey-muted">
                        Projects and agents use this cloud machine through your
                        private connection. Switching detaches terminal views;
                        existing sessions keep running on their original
                        machine. Save your brief and design before switching.
                      </p>
                      <button
                        className="luci-button"
                        disabled={!!busy}
                        onClick={() =>
                          void run("Returning locally", () =>
                            changeWorkspace(() => client.localWorkspace()),
                          )
                        }
                      >
                        Return to this machine
                      </button>
                    </div>
                  )}
                  <form onSubmit={submitConfig}>
                    <label className="luci-field">
                      Projects folder
                      <input
                        value={config.projectsDir}
                        readOnly={!manageBacking}
                        onChange={(event) =>
                          update({ projectsDir: event.target.value })
                        }
                        required
                        placeholder="An absolute folder path on this machine"
                      />
                      <small>
                        New projects live here. Existing Git repositories appear
                        in the project rail.
                      </small>
                    </label>
                    <div className="setup-actions">
                      <button
                        className="luci-button luci-primary"
                        disabled={!!busy || !manageBacking}
                      >
                        {busy === "Saving"
                          ? "Saving…"
                          : manageBacking
                            ? "Save settings"
                            : "Projects folder managed by administrator"}
                      </button>
                      <button
                        className="luci-button"
                        type="button"
                        disabled={!!busy}
                        onClick={() =>
                          void run("Checking", async () => {
                            receive(await client.check());
                            setNotice("Readiness check finished.");
                          })
                        }
                      >
                        {busy === "Checking"
                          ? "Checking this machine…"
                          : "Check readiness"}
                      </button>
                    </div>
                  </form>
                  <div className="setup-checks">
                    <h2>Ready when you are</h2>
                    <p className="journey-muted">
                      Git supports your projects. Docker and the sandbox image
                      are needed for isolated agent sessions. AWS and Azure are
                      optional.
                    </p>
                    {status?.checks.map((check) => (
                      <div className="setup-check" key={check.id}>
                        <div>
                          <strong>{check.label}</strong>
                          <span
                            className={`setup-status setup-status-${check.status}`}
                          >
                            {check.status}
                          </span>
                        </div>
                        <p>{check.message}</p>
                        {check.action && <pre>{check.action}</pre>}
                      </div>
                    ))}
                  </div>
                  <LocalTools
                    client={client}
                    cloud={status?.workspace?.mode === "cloud"}
                    onSessionStarted={onClose}
                  />
                  <button
                    className="luci-button"
                    type="button"
                    onClick={() => setSection("project")}
                  >
                    Create your first project →
                  </button>
                </>
              )}
              {section === "project" && (
                <>
                  <form
                    onSubmit={(event) => {
                      event.preventDefault();
                      void run("Creating project", async () => {
                        const project = await client.createProject(
                          name.trim(),
                          remote.trim() || undefined,
                        );
                        onProjectsChanged();
                        useCockpit.getState().selectProject(project.id);
                        useCockpit.getState().setLens("discover");
                        onClose();
                      });
                    }}
                  >
                    <h2>Start with a name.</h2>
                    <p className="journey-muted">
                      Create a local Git project, or bring an existing one from
                      an HTTPS repository.
                    </p>
                    <label className="luci-field">
                      Project name
                      <input
                        value={name}
                        onChange={(event) => setName(event.target.value)}
                        required
                        autoComplete="off"
                        placeholder="my-next-app"
                      />
                    </label>
                    <label className="luci-field">
                      Repository URL{" "}
                      <span className="journey-muted">optional</span>
                      <input
                        value={remote}
                        onChange={(event) => setRemote(event.target.value)}
                        type="url"
                        placeholder="https://github.com/you/project.git"
                      />
                    </label>
                    <p className="journey-muted">
                      Saved inside <code>{config.projectsDir}</code>
                    </p>
                    <button
                      className="luci-button luci-primary"
                      disabled={!!busy || !name.trim()}
                    >
                      {busy === "Creating project"
                        ? "Creating project…"
                        : "Create project"}
                    </button>
                  </form>
                  <ProjectRestore
                    client={client}
                    onRestored={(project) => {
                      onProjectsChanged();
                      useCockpit.getState().selectProject(project.id);
                      useCockpit.getState().setLens("discover");
                      onClose();
                    }}
                  />
                </>
              )}
              {section === "cloud" && !manageBacking && (
                <p className="journey-muted">
                  Your administrator manages cloud backing and deployment
                  connections for this workspace.
                </p>
              )}
              {section === "cloud" && manageBacking && (
                <>
                  {!allowProvision && (
                    <p className="journey-muted">
                      Cloud provisioning is disabled by workspace policy.
                      Existing connections can still be managed below.
                    </p>
                  )}
                  {allowProvision && (
                    <form
                      onSubmit={(event) => {
                        event.preventDefault();
                        void run("Planning", async () => {
                          const value = await client.cloudPlan({
                            provider: config.cloudProvider || "aws",
                            name: cloudName,
                            region: config.region,
                            domain: config.domain,
                            size,
                            monthlyBudgetUsd: config.monthlyBudgetUsd,
                            sshPublicKey,
                          });
                          setPlan(value);
                        });
                      }}
                    >
                      <h2>A small home for what you build.</h2>
                      <p className="journey-muted">
                        Use your own AWS or Azure account. Preview a
                        single-machine deployment before creating any resources.
                      </p>
                      <div className="setup-field-grid">
                        <label className="luci-field">
                          Provider
                          <select
                            value={config.cloudProvider || "aws"}
                            onChange={(event) =>
                              update({
                                cloudProvider: event.target.value as
                                  "aws" | "azure",
                                region: "",
                              })
                            }
                          >
                            <option value="aws">AWS · Lightsail</option>
                            <option value="azure">
                              Azure · Virtual machine
                            </option>
                          </select>
                        </label>
                        <label className="luci-field">
                          Region
                          <input
                            required
                            value={config.region}
                            onChange={(event) =>
                              update({ region: event.target.value })
                            }
                            placeholder={
                              config.cloudProvider === "azure"
                                ? "australiaeast"
                                : "ap-southeast-2"
                            }
                          />
                        </label>
                        <label className="luci-field">
                          Deployment name
                          <input
                            required
                            pattern="[a-z][a-z0-9-]*"
                            value={cloudName}
                            onChange={(event) => {
                              setCloudName(event.target.value);
                              setPlan(null);
                            }}
                          />
                        </label>
                        <label className="luci-field">
                          Domain
                          <input
                            required
                            value={config.domain}
                            onChange={(event) =>
                              update({ domain: event.target.value })
                            }
                            placeholder="app.example.com"
                          />
                        </label>
                        <label className="luci-field">
                          Machine size
                          <select
                            value={size}
                            onChange={(event) => {
                              setSize(event.target.value as typeof size);
                              setPlan(null);
                            }}
                          >
                            <option value="small">Small · start here</option>
                            <option value="standard">
                              Standard · more capacity
                            </option>
                          </select>
                        </label>
                        <label className="luci-field">
                          Monthly budget · USD
                          <input
                            type="number"
                            min="1"
                            required
                            value={config.monthlyBudgetUsd}
                            onChange={(event) =>
                              update({
                                monthlyBudgetUsd: Number(event.target.value),
                              })
                            }
                          />
                        </label>
                      </div>
                      <label className="luci-field">
                        SSH public key
                        <textarea
                          rows={3}
                          required
                          value={sshPublicKey}
                          onChange={(event) => {
                            setSshPublicKey(event.target.value);
                            setPlan(null);
                          }}
                          placeholder="ssh-ed25519 …"
                        />
                        <small>
                          Use the public .pub key. Your private key stays on
                          your machine.
                        </small>
                      </label>
                      <ProviderSignIn
                        client={client}
                        provider={config.cloudProvider || "aws"}
                      />
                      <button
                        className="luci-button"
                        type="button"
                        disabled={!!busy}
                        onClick={() =>
                          void run("Creating key", async () => {
                            const key = await client.createSSHKey();
                            setSshPublicKey(key.publicKey);
                            setPrivateKeyPath(key.privateKeyPath);
                            setPlan(null);
                            setNotice(
                              `Your Luci SSH key is ready. Fingerprint: ${key.fingerprint}`,
                            );
                          })
                        }
                      >
                        Create or use Luci SSH key
                      </button>
                      <button
                        className="luci-button luci-primary"
                        disabled={!!busy}
                      >
                        {busy === "Planning"
                          ? "Preparing plan…"
                          : "Preview cloud plan"}
                      </button>
                    </form>
                  )}
                  {allowProvision && plan && (
                    <div className="setup-plan">
                      <h2>Your deployment plan</h2>
                      <p className="journey-muted">
                        Plan generated. No cloud resources have been created.
                      </p>
                      <p>
                        <strong>
                          {plan.estimate.verified &&
                          plan.estimate.monthly !== null
                            ? `${plan.estimate.currency} ${plan.estimate.monthly}/month`
                            : "Monthly cost not verified"}
                        </strong>{" "}
                        · {plan.estimate.explanation}
                      </p>
                      <ul>
                        {plan.resources.map((resource) => (
                          <li key={resource}>{resource}</li>
                        ))}
                      </ul>
                      {plan.warnings.map((warning) => (
                        <p className="journey-warning" key={warning}>
                          {warning}
                        </p>
                      ))}
                      <ol>
                        {plan.steps.map((step) => (
                          <li key={step.id}>
                            <strong>{step.title}</strong>
                            <p>{step.detail}</p>
                          </li>
                        ))}
                      </ol>
                      {plan.files.map((file) => (
                        <details key={file.name}>
                          <summary>{file.name}</summary>
                          <pre>{file.content}</pre>
                          <button
                            className="luci-button"
                            onClick={() => {
                              const blob = new Blob([file.content], {
                                type: file.mediaType,
                              });
                              const url = URL.createObjectURL(blob);
                              const link = document.createElement("a");
                              link.href = url;
                              link.download = file.name;
                              link.click();
                              URL.revokeObjectURL(url);
                            }}
                          >
                            Save template
                          </button>
                        </details>
                      ))}
                    </div>
                  )}
                  <CloudManager
                    client={client}
                    plan={allowProvision ? plan : null}
                    privateKeyPath={privateKeyPath}
                  />
                </>
              )}
            </>
          )}
        </div>
        <footer className="setup-footer">
          <span>Local by default · Your account, when you choose</span>
          <button className="luci-button" onClick={onClose} disabled={!!busy}>
            Back to project
          </button>
        </footer>
      </div>
    </div>
  );
}
