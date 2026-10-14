import {
  useEffect,
  useId,
  useLayoutEffect,
  useMemo,
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
  type SetupMode,
} from "../../api/product";
import { useDismissable } from "../../hooks/useDismissable";
import { Button } from "../fluid/components/button";
import { useCockpit } from "../../store/store";
import { CloudManager } from "./CloudManager";
import { LocalTools } from "./LocalTools";
import { ProviderSignIn } from "./ProviderSignIn";
import { changeWorkspace } from "./workspace";
import { ProjectRestore } from "../journey/ProjectBackup";
import { useAuth } from "../auth/AuthProvider";
import { AppearancePanel } from "./AppearancePanel";
import { RuntimePreparation } from "./RuntimePreparation";
import { DockerStorageConfirmation } from "./DockerStorageConfirmation";
import { createExecutionStorageClient, storagePreparationBlock } from "../../api/executionStorage";
import { AgentConnections } from "./AgentConnections";
import { DependencySetup } from "./DependencySetup";
import { SetupModeChoices } from './SetupModeChoices';
import { AzureSetup } from './AzureSetup';
import { LuciMark } from '../common/LuciMark';
import { createDependencyClient } from "../../api/dependencies";
import { projectNameError, projectNameHelp } from "../../lib/projectName";
import "./setup.css";

interface Props {
  initialStatus: SetupStatus | null;
  initialSection?: "local" | "project" | "cloud" | "appearance" | "agents" | "setup";
  presentation?: "dialog" | "portal";
  onOpenProject?(): void;
  onSessionStarted?(): void;
  onStatus(value: SetupStatus): void;
  onClose(): void;
  onProjectsChanged(): void;
  client?: ProductClient;
}
type SetupRoute = "local" | "managed" | "existing" | "byo" | "hosted";

function SetupRoutes({
  includeLocal,
  manageBacking,
  allowProvision,
  onChoose,
}: {
  includeLocal: boolean;
  manageBacking: boolean;
  allowProvision: boolean;
  onChoose(route: SetupRoute, provider?: "aws" | "azure"): void;
}) {
  const routes: {
    id: SetupRoute;
    title: string;
    description: string;
    note: string;
    disabled: boolean;
    provider?: "aws" | "azure";
  }[] = [
    ...(includeLocal
      ? [
          {
            id: "local" as const,
            title: "On this computer",
            description:
              "Keep your projects here and use Docker for agents and previews. You can connect a cloud workspace later.",
            note: "Recommended · lowest ongoing cost. No cloud hosting bill; model usage is separate.",
            disabled: false,
          },
        ]
      : []),
    ...(["aws", "azure"] as const).map(provider => ({
      id: "managed",
      provider,
      title: provider === "aws" ? "AWS" : "Azure",
      description:
        provider === "aws"
          ? "Run your workspace in your own AWS account. Luci guides the sign-in, machine setup and domain connection."
          : "Run your workspace in your own Azure account. Luci guides the sign-in, machine setup and domain connection.",
      note: !manageBacking
        ? "An Admin manages cloud connections for this workspace."
        : !allowProvision
          ? "Cloud provisioning is disabled by workspace policy."
          : "Have your cloud account and a domain you control ready. Cloud, domain and model charges are separate.",
      disabled: !manageBacking || !allowProvision,
    } as const)),
    {
      id: "existing",
      title: "Connect to an existing workspace",
      description:
        "Use a Luci workspace that you or your team already runs. Connect to its server, then sign in with your workspace account.",
      note: !manageBacking
        ? "Ask your Admin to connect the workspace."
        : "Ask its administrator for the SSH connection details, trusted server fingerprint and your Luci account.",
      disabled: !manageBacking,
    },
  ];
  return (
    <div
      className="setup-route-choices"
      role="group"
      aria-label="Workspace setup options"
    >
      {routes.map((route) => (
        <button
          key={route.provider || route.id}
          type="button"
          className="setup-route-choice"
          disabled={route.disabled}
          aria-labelledby={`setup-route-${route.provider || route.id}`}
          aria-describedby={`setup-route-description-${route.provider || route.id} setup-route-note-${route.provider || route.id}`}
          onClick={() => onChoose(route.id, route.provider)}
        >
          <span className="setup-route-title" id={`setup-route-${route.provider || route.id}`}>
            {route.title}
          </span>
          <span
            className="setup-route-description"
            id={`setup-route-description-${route.provider || route.id}`}
          >
            {route.description}
          </span>
          <span
            className={`setup-route-note${route.id === "local" ? " setup-route-recommended" : ""}`}
            id={`setup-route-note-${route.provider || route.id}`}
          >
            {route.note}
          </span>
        </button>
      ))}
    </div>
  );
}

export function SetupPanel({
  initialStatus,
  initialSection = "local",
  presentation = "dialog",
  onOpenProject,
  onSessionStarted,
  onStatus,
  onClose,
  onProjectsChanged,
  client = productApi,
}: Props) {
  const auth = useAuth();
  const localManageBacking = !auth || auth.profile.permissions.manageBacking;
  const allowProvision = !auth || auth.profile.policy.allowCloudProvisioning;
  const [status, setStatus] = useState(initialStatus);
  const [loading, setLoading] = useState(!initialStatus);
  const [config, setConfig] = useState<SetupConfig | null>(
    initialStatus?.config ?? null,
  );
  const [section, setSection] = useState(initialSection === 'local' && (presentation === 'portal' || initialStatus?.configured === false) && (initialStatus?.setupChoice?.mode === 'bring-your-own-cloud' || initialStatus?.setupChoice?.mode === 'luci-cloud') ? 'setup' : initialSection);
  const [route, setRoute] = useState<SetupRoute | null>(
    initialSection === "project" ? "local" : (presentation === 'portal' || initialStatus?.configured === false) ? initialStatus?.setupChoice?.mode === 'local' ? 'local' : initialStatus?.setupChoice?.mode === 'bring-your-own-cloud' ? 'byo' : initialStatus?.setupChoice?.mode === 'luci-cloud' ? 'hosted' : null : null,
  );
  const [choiceNeedsRefresh, setChoiceNeedsRefresh] = useState(false);
  const cloudWorkspace = status?.workspace?.mode === "cloud";
  const manageBacking =
    status?.access?.manageBacking ?? (!cloudWorkspace && localManageBacking);
  const projectsDirManaged = status?.access?.projectsDirManaged === true;
  const canChooseProjectsDir = manageBacking && !projectsDirManaged;
  const portal = presentation === "portal";
  const onboarding =
    (status?.configured === false || portal) &&
    !cloudWorkspace;
  const dependencies = useMemo(() => createDependencyClient(client), [client]);
  const executionStorage = useMemo(() => createExecutionStorageClient(client), [client]);
  const [toolsRevision, setToolsRevision] = useState(0);
  const [storageRevision, setStorageRevision] = useState(0);
  const storageInvalidation = useRef({revision: 0, needsCheck: false});
  const latestStatus = useRef(status); latestStatus.current = status;
  const [error, setError] = useState("");
  const [notice, setNotice] = useState("");
  const [busy, setBusy] = useState("");
  const [name, setName] = useState("");
  const nameHelpId = useId();
  const nameError = name.trim() ? projectNameError(name) : null;
  const [remote, setRemote] = useState("");
  const [cloudName, setCloudName] = useState("luci");
  const [size, setSize] = useState<"small" | "standard" | "full">("small");
  const [sshPublicKey, setSshPublicKey] = useState("");
  const [privateKeyPath, setPrivateKeyPath] = useState("");
  const [plan, setPlan] = useState<CloudPlan | null>(null);
  const [runtimePending, setRuntimePending] = useState(true);
  const runtimeActive =
    portal && section === "local" && (!onboarding || route === "local") &&
    busy !== "Preparing projects folder";
  const runtimeHandoffPending = runtimeActive && runtimePending;
  const navigationLocked = !!busy || runtimeHandoffPending;
  const runtimePendingRef = useRef(runtimeHandoffPending);
  runtimePendingRef.current = runtimeHandoffPending;
  const alive = useRef(true);
  const inFlight = useRef(false);
  const heading = useRef<HTMLHeadingElement>(null);
  const panel = useDismissable<HTMLDivElement>(!portal, () => {
    if (!inFlight.current && !runtimePendingRef.current) onClose();
  });
  useLayoutEffect(() => {
    heading.current?.focus();
  }, []);
  useLayoutEffect(() => {
    if (onboarding) heading.current?.focus();
  }, [onboarding, route, section]);
  useLayoutEffect(() => {
    if (busy && !panel.current?.contains(document.activeElement))
      heading.current?.focus();
  }, [busy, panel]);
  useEffect(() => {
    alive.current = true;
    return () => {
      alive.current = false;
    };
  }, []);
  const staleStorageReadiness = (value: SetupStatus): SetupStatus => {
    if (value.platform !== "windows" || value.workspace?.mode === "cloud") return value;
    const previous = value.checks.find(check => check.id === "docker-backing-storage");
    const check = {...previous, id: "docker-backing-storage", label: previous?.label || "Docker storage readiness", status: "unknown" as const,
      message: "Docker storage changed. Choose Check readiness to refresh workspace readiness.", action: ""};
    return {...value, checks: [...value.checks.filter(item => item.id !== check.id), check]};
  };
  const receive = (value: SetupStatus, checkedStorageRevision?: number, updateConfig = true) => {
    if (checkedStorageRevision !== undefined && checkedStorageRevision !== storageInvalidation.current.revision) return false;
    if (checkedStorageRevision === storageInvalidation.current.revision) storageInvalidation.current.needsCheck = false;
    const next = storageInvalidation.current.needsCheck ? staleStorageReadiness(value) : value;
    latestStatus.current = next; setStatus(next);
    if (updateConfig) setConfig(value.config);
    onStatus(next);
    return true;
  };
  const invalidateStorageReadiness = () => {
    storageInvalidation.current = {revision: storageInvalidation.current.revision + 1, needsCheck: true};
    setStorageRevision(storageInvalidation.current.revision);
    const previous = latestStatus.current;
    if (previous) {
      const next = staleStorageReadiness(previous);
      latestStatus.current = next; setStatus(next); onStatus(next);
    }
    setNotice("Storage status changed. Check readiness when you want to refresh the complete workspace checks.");
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
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
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
  const chooseRoute = (value: SetupRoute, provider?: "aws" | "azure") => {
    if (value === "local") setRuntimePending(true);
    if (provider) update({ cloudProvider: provider,
      region: (config?.cloudProvider || "aws") === provider ? config?.region || "" : "",
    });
    setRoute(value);
    setSection(value === "local" ? "local" : "cloud");
    setError("");
    setNotice("");
    if (
      portal &&
      value === "local" &&
      config &&
      !status?.configured &&
      canChooseProjectsDir
    ) {
      void run("Preparing projects folder", async () => {
        receive(await client.configure(config));
        onProjectsChanged();
      });
    }
  };
  const chooseSetupMode = (mode: SetupMode, nextRoute?: 'existing') => {
    if (choiceNeedsRefresh) return;
    void run('Saving setup choice', async () => {
      try {
        const choice = await client.chooseSetup(latestStatus.current?.setupChoice?.revision ?? 1, mode);
        if (!alive.current) return;
        const previous = latestStatus.current;
        if (previous) receive({ ...previous, setupChoice: choice }, undefined, false);
        if (mode === 'local' && portal && config && !previous?.configured && canChooseProjectsDir) {
          receive(await client.configure(config)); onProjectsChanged();
        }
        if (!alive.current) return;
        if (mode === 'local') setRuntimePending(true);
        setRoute(nextRoute ?? (mode === 'local' ? 'local' : mode === 'bring-your-own-cloud' ? 'byo' : 'hosted'));
        setSection(mode === 'local' ? 'local' : 'setup');
        setNotice('Setup preference saved. Your active connection and existing projects are unchanged.');
      } catch (cause) { if (alive.current) setChoiceNeedsRefresh(true); throw cause; }
    });
  };
  const refreshSetupChoice = () => void run('Refreshing setup choice', async () => {
    const value = await client.status();
    if (!value.setupChoice || !Number.isSafeInteger(value.setupChoice.revision) || value.setupChoice.revision < 1) throw new Error('This service does not report the saved setup choice. Update Luci before saving a mode.');
    if (alive.current) { receive(value, undefined, false); setChoiceNeedsRefresh(false); }
  });
  const createProject = () => {
    if (inFlight.current || runtimePendingRef.current) return;
    if (onOpenProject) onOpenProject();
    else setSection("project");
  };
  const chooseAgain = () => {
    setRoute(null);
    setPlan(null);
    setError("");
    setNotice("");
    if (onboarding) setSection("local");
    else setSection('setup');
    heading.current?.focus();
  };
  function trapFocus(event: KeyboardEvent<HTMLDivElement>) {
    if (event.key !== "Tab") return;
    const items = Array.from(
      event.currentTarget.querySelectorAll<HTMLElement>(
        "button:not(:disabled), input:not(:disabled), select:not(:disabled), textarea:not(:disabled), a[href], summary",
      ),
    ).filter((item) => {
      const closed = item.closest("details:not([open])");
      return !closed || item === closed.querySelector(":scope > summary");
    });
    const first = items[0],
      last = items[items.length - 1];
    if (!first) {
      event.preventDefault();
      heading.current?.focus();
      return;
    }
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
    if (!config || !canChooseProjectsDir) return;
    void run("Saving", async () => {
      receive(await client.configure(config));
      onProjectsChanged();
      setNotice("Settings saved. Your projects folder is ready.");
    });
  };
  return (
    <div className={portal ? "onboarding-portal" : "setup-backdrop"}>
      {portal && (
        <aside className="onboarding-intro">
          <strong>Luci</strong>
          <h2>A workspace for the whole idea.</h2>
          <p>
            Discover what matters, design the experience, develop with agents and
            deploy a working solution.
          </p>
          <ol aria-label="Getting started">
            <li aria-current={!route && onboarding ? "step" : undefined}>
              Choose where you work
            </li>
            <li aria-current={route || !onboarding ? "step" : undefined}>
              Prepare your workspace
            </li>
            <li>Create your first project</li>
          </ol>
          <p className="journey-muted">
            Your account is ready. These steps connect the tools your projects
            will use. Cloud resources and model use are separate choices.
          </p>
        </aside>
      )}
      <div
        className="setup-window"
        ref={panel}
        role={portal ? "main" : "dialog"}
        aria-modal={portal ? undefined : true}
        aria-labelledby="setup-title"
        onKeyDown={portal ? undefined : trapFocus}
      >
        <header className="setup-header">
          <div>
            <span className="setup-brand">
              <LuciMark />
              Luci <span>by lcfr.ai</span>
            </span>
            <h1 id="setup-title" ref={heading} tabIndex={-1}>
              {cloudWorkspace
                ? section === "project"
                  ? "Create a cloud project"
                  : "Your cloud workspace"
                : !onboarding && status?.configured
                  ? portal
                    ? "Prepare your workspace"
                    : "Settings"
                  : route === "local"
                    ? section === "project"
                      ? "Create your first project"
                      : "Set up this computer"
                    : route === 'byo'
                      ? 'Set up your own cloud'
                    : route === 'hosted'
                      ? 'Luci Cloud · not available yet'
                    : route === "managed"
                      ? "Set up a cloud workspace"
                      : route === "existing"
                        ? "Connect to an existing workspace"
                        : "Choose your workspace"}
            </h1>
            <p>
              {cloudWorkspace
                ? `Cloud workspace · ${status?.workspace?.host}${status?.access ? ` · Signed in as ${status.access.user.username}` : ""}`
                : onboarding && !route
                  ? "Start locally, let Luci guide your own cloud setup, or join a workspace you already have."
                  : "Your projects, your machine. Move your workspace to your cloud when you choose."}
            </p>
          </div>
          {!portal && (
            <button
              className="setup-close"
              type="button"
              onClick={onClose}
              disabled={navigationLocked}
              aria-label="Close settings"
            >
              ×
            </button>
          )}
        </header>
        <nav className="setup-tabs" aria-label="Settings sections">
          {onboarding
            ? route && (
                <button type="button" disabled={navigationLocked} onClick={chooseAgain}>
                  Choose another setup option
                </button>
              )
            : config &&
              (
                [
                  ["local", cloudWorkspace ? "This workspace" : "This machine"],
                  ["setup", "Workspace setup"],
                  ["project", "New project"],
                  [
                    "cloud",
                    cloudWorkspace ? "Connection and account" : "Your cloud",
                  ],
                  ["appearance", "Appearance"],
                  ["agents", "Agent connection"],
                ] as const
              )
                .filter(([id]) => id !== "cloud" || localManageBacking)
                .filter(([id]) => id !== "project" || !onOpenProject)
                .map(([id, label]) => (
                  <button
                    key={id}
                    type="button"
                    disabled={navigationLocked}
                    aria-current={section === id ? "page" : undefined}
                    onClick={() => {
                      if (id === "local" && portal) setRuntimePending(true);
                      if (id === 'setup') setRoute(null);
                      setSection(id);
                    }}
                  >
                    {label}
                  </button>
                ))}
        </nav>
        <div className="setup-content" aria-busy={loading || !!busy}>
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
              {loading ? (
                <p role="status">Loading your workspace settings…</p>
              ) : (
                <>
                  <p>Connect to Luci’s local service to load your settings.</p>
                  <Button type="button"
                    variant="secondary" loading={busy === "Connecting"}
                    disabled={!!busy}
                    onClick={() =>
                      void run("Connecting", async () => {
                        receive(await client.status());
                      })
                    }
                  >
                    Retry connection
                  </Button>
                </>
              )}
            </div>
          ) : (
            <>
              {choiceNeedsRefresh && <p role="status">The saved choice is unconfirmed. Refresh before choosing again. <Button type="button" variant="secondary" disabled={!!busy} onClick={refreshSetupChoice}>Refresh saved setup choice</Button></p>}
              {(onboarding && !route || section === 'setup' && !route) && (
                <SetupModeChoices
                  selected={status?.setupChoice?.mode}
                  busy={!!busy || choiceNeedsRefresh}
                  manageBacking={localManageBacking}
                  allowProvision={allowProvision}
                  onChoose={chooseSetupMode}
                  onExisting={() => chooseSetupMode('bring-your-own-cloud', 'existing')}
                />
              )}
              {section === 'setup' && route === 'hosted' && <section className="setup-mode-detail" aria-label="Luci Cloud availability"><h2>Luci Cloud is not available yet</h2><p>The Luci-hosted service is being developed before launch. This choice does not create a hosted account, take payment, provision resources or connect a runtime.</p><p>Your existing projects remain accessible. Choose Local or Bring your own cloud to work now.</p><SetupModeChoices selected="luci-cloud" busy={!!busy || choiceNeedsRefresh} manageBacking={localManageBacking} allowProvision={allowProvision} onChoose={chooseSetupMode} onExisting={() => chooseSetupMode('bring-your-own-cloud', 'existing')} /></section>}
              {section === 'setup' && route === 'byo' && <section className="setup-mode-detail"><h2>Bring your own cloud</h2><AzureSetup client={client} disabled={!!busy || choiceNeedsRefresh || !localManageBacking || !manageBacking || !allowProvision} budget={config?.monthlyBudgetUsd} /><Button type="button" variant="secondary" disabled={!!busy || !localManageBacking} onClick={() => { setRoute('existing'); setSection('cloud'); }}>Connect an existing workspace</Button><Button type="button" variant="ghost" disabled={!!busy} onClick={chooseAgain}>Choose another setup option</Button></section>}
              {section === 'setup' && route === 'existing' && <CloudManager client={client} plan={null} mode="existing" />}
              {section === "local" && (!onboarding || route === "local") && (
                <>
                  {cloudWorkspace && (
                    <div className="workspace-current">
                      <h2>Working on {status?.workspace?.host}</h2>
                      <p className="journey-muted">
                        Projects and agents use this cloud machine through your
                        private connection. Switching detaches terminal views;
                        existing sessions keep running on their original
                        machine. Save your brief and design before switching.
                      </p>
                      <Button
                        variant="primary"
                        type="button"
                        disabled={navigationLocked || !status?.workspace?.ready}
                        onClick={createProject}
                      >
                        Create your first project
                      </Button>
                      {localManageBacking && (
                        <Button type="button"
                          variant="secondary" loading={busy === "Returning locally"}
                          disabled={navigationLocked}
                          onClick={() =>
                            void run("Returning locally", () =>
                              changeWorkspace(() => client.localWorkspace()),
                            )
                          }
                        >
                          Return to this machine
                        </Button>
                      )}
                    </div>
                  )}
                  {runtimeActive && (
                    <>
                      <DependencySetup
                        client={dependencies}
                        canManage={localManageBacking}
                        automatic
                        cloudWorkspace={cloudWorkspace}
                        onChecked={() => {
                          setRuntimePending(true);
                          setToolsRevision(value => value + 1);
                        }}
                      />
                      <RuntimePreparation
                        refreshToken={toolsRevision}
                        storageRevision={storageRevision}
                        statusSnapshot={status}
                        client={client}
                        manageBacking={manageBacking}
                        automatic
                        onPendingChange={setRuntimePending}
                        onStatus={(value, checkedStorageRevision) => receive(value, checkedStorageRevision, false)}
                      />
                    </>
                  )}
                  {!portal && <DependencySetup client={dependencies} canManage={localManageBacking} cloudWorkspace={cloudWorkspace} onChecked={() => setToolsRevision(value => value + 1)} />}
                  <form onSubmit={submitConfig}>
                    <label className="luci-field">
                      Projects folder
                      <input
                        value={config.projectsDir}
                        disabled={!!busy}
                        readOnly={!canChooseProjectsDir}
                        onChange={(event) =>
                          update({ projectsDir: event.target.value })
                        }
                        required
                        placeholder={
                          cloudWorkspace
                            ? "An absolute folder path on this workspace"
                            : "An absolute folder path on this machine"
                        }
                      />
                      <small>
                        {projectsDirManaged
                          ? "Your private projects folder is assigned by this workspace. You can create a project without changing server settings."
                          : "New projects live here. Existing Git repositories appear in the project rail."}
                      </small>
                    </label>
                    <div className="setup-actions">
                      {canChooseProjectsDir && (
                        <Button type="submit"
                          variant="primary" loading={busy === "Saving"}
                          disabled={!!busy}
                        >
                          {busy === "Saving" ? "Saving…" : "Save settings"}
                        </Button>
                      )}
                      <Button
                        variant="secondary" loading={busy === "Checking"}
                        type="button"
                        disabled={!!busy}
                        onClick={() =>
                          void run("Checking", async () => {
                            const checkedStorageRevision = storageInvalidation.current.revision;
                            receive(await client.check(), checkedStorageRevision);
                            setNotice("Readiness check finished.");
                          })
                        }
                      >
                        {busy === "Checking"
                          ? cloudWorkspace
                            ? "Checking this workspace…"
                            : "Checking this machine…"
                          : "Check readiness"}
                      </Button>
                    </div>
                  </form>
                  {!portal && (
                    <div className="setup-checks">
                      <h2>Ready when you are</h2>
                      <p className="journey-muted">
                        Git supports your projects. Docker and the sandbox image
                        are needed for isolated agent sessions.
                        {cloudWorkspace
                          ? " These checks describe the selected cloud workspace."
                          : " AWS and Azure are optional."}
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
                  )}
                  <DockerStorageConfirmation
                    client={executionStorage}
                    enabled={status?.platform === "windows" && !cloudWorkspace}
                    onChanged={invalidateStorageReadiness}
                  />
                  <LocalTools
                    runtimeBlockedReason={storagePreparationBlock(status)}
                    client={client}
                    identityOnly={portal}
                    cloud={cloudWorkspace}
                    manageBacking={manageBacking}
                    onSessionStarted={onSessionStarted ?? onClose}
                  />
                  {portal && (
                    <section className="onboarding-next">
                      <h2>Connect your coding agent</h2>
                      <p className="journey-muted">
                        Use your Codex account, provider API credits or a local
                        model. You can connect now or come back after defining
                        your project.
                      </p>
                      <Button
                        variant="secondary"
                        type="button"
                        disabled={navigationLocked}
                        onClick={() => setSection("agents")}
                      >
                        Connect a coding agent
                      </Button>
                    </section>
                  )}
                  {!cloudWorkspace && (
                    <Button
                      variant="secondary"
                      type="button"
                      disabled={navigationLocked}
                      onClick={createProject}
                    >
                      Create your first project →
                    </Button>
                  )}
                </>
              )}
              {section === "project" && (
                <>
                  <form
                    onSubmit={(event) => {
                      event.preventDefault();
                      if (projectNameError(name)) return;
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
                      {cloudWorkspace
                        ? "Create a Git project in your private cloud workspace, or bring an existing one from an HTTPS repository."
                        : "Create a local Git project, or bring an existing one from an HTTPS repository."}
                    </p>
                    <label className="luci-field">
                      Project name
                      <input
                        value={name}
                        disabled={!!busy}
                        onChange={(event) => setName(event.target.value)}
                        required
                        autoComplete="off"
                        placeholder="my-next-app"
                        aria-describedby={nameHelpId}
                        aria-invalid={!!nameError}
                      />
                    </label>
                    <p id={nameHelpId} className={nameError ? "journey-error" : "journey-muted"}>
                      {nameError ?? projectNameHelp}
                    </p>
                    <label className="luci-field">
                      Repository URL{" "}
                      <span className="journey-muted">optional</span>
                      <input
                        value={remote}
                        disabled={!!busy}
                        onChange={(event) => setRemote(event.target.value)}
                        type="url"
                        placeholder="https://github.com/you/project.git"
                      />
                    </label>
                    <p className="journey-muted">
                      Saved inside <code>{config.projectsDir}</code>
                    </p>
                    <Button type="submit"
                      variant="primary" loading={busy === "Creating project"}
                      disabled={!!busy || !!projectNameError(name)}
                    >
                      {busy === "Creating project"
                        ? "Creating project…"
                        : "Create project"}
                    </Button>
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
              {section === "appearance" && <AppearancePanel />}
              {section === "agents" && (
                <>
                  <AgentConnections
                    client={client}
                    cloud={cloudWorkspace}
                    initialProvider="openai"
                    onSessionStarted={onSessionStarted ?? onClose}
                  />
                  {portal && (
                    <Button type="button"
                      variant="ghost"
                      onClick={() => setSection("local")}
                    >
                      Back to workspace preparation
                    </Button>
                  )}
                </>
              )}
              {section === "cloud" && !localManageBacking && (
                <p className="journey-muted">
                  Your administrator manages cloud backing and deployment
                  connections for this workspace.
                </p>
              )}
              {section === "cloud" && localManageBacking && cloudWorkspace && (
                <CloudManager
                  client={client}
                  plan={null}
                  mode="existing"
                  activeConnectionId={status?.workspace?.connectionId}
                  manageTargets={manageBacking}
                />
              )}
              {section === "cloud" && localManageBacking && !cloudWorkspace && (
                <>
                  {!onboarding && route && (
                    <Button
                      type="button"
                      variant="ghost"
                      disabled={!!busy}
                      onClick={chooseAgain}
                    >
                      Cloud setup options
                    </Button>
                  )}
                  {!route && !onboarding && (
                    <SetupRoutes
                      includeLocal={false}
                      manageBacking={localManageBacking}
                      allowProvision={allowProvision}
                      onChoose={chooseRoute}
                    />
                  )}
                  {route === "existing" && (
                    <CloudManager client={client} plan={null} mode="existing" />
                  )}
                  {route === "managed" && (
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
                          <h2>Plan your cloud workspace</h2>
                          <p className="journey-muted">
                            Sign in to your AWS or Azure account, choose a
                            region and a domain you control, then preview the
                            resources and costs before approving creation. Your
                            budget is a planning limit, not a billing cap.
                          </p>
                          <DependencySetup client={dependencies} canManage={localManageBacking} automatic target={config.cloudProvider || "aws"} />
                          <label className="luci-field">
                            Provider
                            <select
                              disabled={!!busy}
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
                          <ProviderSignIn
                            key={config.cloudProvider || "aws"}
                            client={client}
                            provider={config.cloudProvider || "aws"}
                          />
                          <div className="setup-field-grid">
                            <label className="luci-field">
                              Region
                              <input
                                required
                                disabled={!!busy}
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
                                disabled={!!busy}
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
                                disabled={!!busy}
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
                                disabled={!!busy}
                                value={size}
                                onChange={(event) => {
                                  setSize(event.target.value as typeof size);
                                  setPlan(null);
                                }}
                              >
                                <option value="small">
                                  Small · 4 GB / 2 CPU · lightweight apps
                                </option>
                                <option value="standard">
                                  Standard · 8 GB / 2 CPU
                                </option>
                                <option value="full">Full data stack · 4 CPU · {config.cloudProvider === "azure" ? "8 GB" : "16 GB"}</option>
                              </select>
                              <small>Choose Full data stack to run authentication, storage and realtime alongside an app and coding agent. The smaller machines suit lightweight projects.</small>
                            </label>
                            <label className="luci-field">
                              Monthly budget · USD
                              <input
                                type="number"
                                disabled={!!busy}
                                min="1"
                                required
                                value={config.monthlyBudgetUsd}
                                onChange={(event) =>
                                  update({
                                    monthlyBudgetUsd: Number(
                                      event.target.value,
                                    ),
                                  })
                                }
                              />
                            </label>
                          </div>
                          <label className="luci-field">
                            SSH public key
                            <textarea
                              disabled={!!busy}
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
                          <Button
                            variant="secondary" loading={busy === "Creating key"}
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
                          </Button>
                          <Button type="submit"
                            variant="primary" loading={busy === "Planning"}
                            disabled={!!busy}
                          >
                            {busy === "Planning"
                              ? "Preparing plan…"
                              : "Preview cloud plan"}
                          </Button>
                        </form>
                      )}
                      {allowProvision && plan && (
                        <div className="setup-plan">
                          <h2>Your deployment plan</h2>
                          <p className="journey-muted">
                            Plan generated. No cloud resources have been
                            created.
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
                              <Button type="button"
                                variant="secondary"
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
                              </Button>
                            </details>
                          ))}
                        </div>
                      )}
                      <CloudManager
                        client={client}
                        plan={allowProvision ? plan : null}
                        privateKeyPath={privateKeyPath}
                        mode="managed"
                      />
                    </>
                  )}
                </>
              )}
            </>
          )}
        </div>
        <footer className="setup-footer">
          <span role="status">
            {busy
              ? `${busy}…`
              : runtimeHandoffPending
                ? "Checking your workspace and starting runtime preparation…"
              : cloudWorkspace
                ? `Projects and agents run on ${status?.workspace?.host}`
                : portal
                  ? "You can connect tools later. Continuing does not mark them ready."
                  : "Local by default · Your account, when you choose"}
          </span>
          <Button type="button" variant="secondary" onClick={onClose} disabled={navigationLocked}>
            {portal ? "Continue to projects" : "Back to project"}
          </Button>
        </footer>
      </div>
    </div>
  );
}
