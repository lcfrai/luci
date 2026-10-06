import { useEffect, useId, useRef, useState } from "react";
import { productApi, errorText, type Release } from "../../api/product";
import { isGatewayHostname, isPublicGatewayHostname } from "../../api/platform";
import { prepareBrowserAccess } from "../../api/browserAccess";
import { useCockpit } from "../../store/store";
import type { ProjectRunDeploymentIntent } from "../../store/slices/uiSlice";
import { useProjectDraftScope } from "../../hooks/useProjectDraft";
import { notifyProjectContextChanged } from "../../api/projectContextEvents";
import { projectRunsApi } from "../../api/projectRuns";
import { useRunDeployment } from "../journey/useRunDeployment";
import { useProjectAccess } from "../../hooks/useProjectAccess";
import { ProjectRequired, JourneyHeader } from "../journey/shared";
import { useSetup } from "../setup/SetupProvider";
import { CloudDeployForm } from "../journey/CloudDeployForm";
import { PagesDeployForm } from "../journey/PagesDeployForm";
import { ProjectConfiguration } from "../journey/ProjectConfiguration";
import { useAuth } from "../auth/AuthProvider";
import { PlatformWorkspace } from "../platform/PlatformWorkspace";
import { DEFAULT_BASE_URL } from "../../api/client";
import { Button } from "../fluid/components/button";
import { TabsSubtle, TabsSubtleItem, TabsSubtlePanel } from "../fluid/components/tabs-subtle";

export function DeployLens() {
  return (
    <ProjectRequired>
      {(id) => <DeploymentWorkspace key={id} projectId={id} />}
    </ProjectRequired>
  );
}
export function DeploymentWorkspace({ projectId }: { projectId: string }) {
  const scope = useProjectDraftScope(projectId);
  return <ScopedDeploymentWorkspace key={scope ?? projectId} projectId={projectId} scope={scope} />;
}
function ScopedDeploymentWorkspace({ projectId, scope }: { projectId: string; scope: string | null }) {
  const pending = useCockpit(state => state.projectRunDeployment);
  const matches = (value: ProjectRunDeploymentIntent | null) => !!scope && value?.scope === scope && value.projectId === projectId;
  const [intent, setIntent] = useState<ProjectRunDeploymentIntent | null>(() => matches(pending) ? pending : null);
  const [view, setView] = useState<"platform" | "publishing">(() => matches(pending) ? "publishing" : "platform");
  const [configurationBusy, setConfigurationBusy] = useState(false);
  const [publishingBusy, setPublishingBusy] = useState(false);
  const tabsId = useId(), selectedIndex = view === "platform" ? 0 : 1;
  const locked = configurationBusy || publishingBusy;
  useEffect(() => {
    if (!matches(pending) || locked) return;
    setIntent(pending); setView("publishing");
    useCockpit.setState(state => state.projectRunDeployment === pending ? { projectRunDeployment: null } : {});
  }, [pending, scope, projectId, locked]);
  return <div className="deployment-workspace">
    <TabsSubtle className="deployment-tabs" aria-label="Deployment workspace" idPrefix={tabsId} selectedIndex={selectedIndex} onSelect={index => { if (!locked) setView(index === 0 ? "platform" : "publishing"); }}>
      <TabsSubtleItem index={0} label="Application platform" aria-disabled={locked} />
      <TabsSubtleItem index={1} label="Previews and publishing" aria-disabled={locked} />
    </TabsSubtle>
    <TabsSubtlePanel idPrefix={tabsId} index={0} selectedIndex={selectedIndex}>
      <section className="journey-page"><JourneyHeader title="Deploy and operate" subtitle="Run the application and its backing services, inspect its health and maintain it after launch." /><fieldset className="definition-fields" disabled={configurationBusy}><PlatformWorkspace projectId={projectId} /></fieldset></section>
    </TabsSubtlePanel>
    <TabsSubtlePanel idPrefix={tabsId} index={1} selectedIndex={selectedIndex}>
      <ProjectDeploy projectId={projectId} deploymentIntent={intent} configurationManaged configurationBusy={configurationBusy} onBusyChange={setPublishingBusy} />
    </TabsSubtlePanel>
    <section className="journey-page deployment-configuration"><details className="deployment-details"><summary>Runtime configuration</summary><ProjectConfiguration projectId={projectId} disabled={publishingBusy} onBusyChange={setConfigurationBusy} /></details></section>
  </div>;
}
interface ProjectDeployProps { projectId: string; configurationManaged?: boolean; configurationBusy?: boolean; onBusyChange?(busy: boolean): void; deploymentIntent?: ProjectRunDeploymentIntent | null }
export function ProjectDeploy(props: ProjectDeployProps) {
  const scope = useProjectDraftScope(props.projectId);
  return <ScopedProjectDeploy key={JSON.stringify([scope ?? props.projectId, props.deploymentIntent?.runId ?? "", props.deploymentIntent?.checkId ?? ""])} {...props} scope={scope} />;
}
function ScopedProjectDeploy({ projectId, configurationManaged = false, configurationBusy: externalConfigurationBusy = false, onBusyChange, deploymentIntent, scope }: ProjectDeployProps & { scope: string | null }) {
  const intent = scope && deploymentIntent?.scope === scope && deploymentIntent.projectId === projectId ? deploymentIntent : null;
  const binding = useRunDeployment(projectId, intent);
  const projectAccess = useProjectAccess(projectId);
  const browserPermission = useRef(projectAccess.canExecute); browserPermission.current = projectAccess.canExecute;
  const canDeploy = projectAccess.canEdit && projectAccess.permissions.deploy;
  const permissionMessage = projectAccess.loading || projectAccess.error ? projectAccess.message
    : !canDeploy ? "Deployment access is required to build, stop, check health or roll back releases. Ask a team owner for editing and deployment access."
      : !projectAccess.canExecute ? "You can deploy this project. Ask a team owner for execution access to open its private application preview." : "";
  const [releases, setReleases] = useState<Release[]>([]);
  const [port, setPort] = useState(3000);
  const [dockerfile, setDockerfile] = useState("Dockerfile");
  const [busy, setBusy] = useState("");
  const [localConfigurationBusy, setConfigurationBusy] = useState(false);
  const configurationBusy = localConfigurationBusy || externalConfigurationBusy;
  const locked = !!busy || configurationBusy;
  const buildLocked = locked || !canDeploy || (!!intent && (!binding.run || binding.loading || !!binding.error));
  const [error, setError] = useState("");
  const [preview, setPreview] = useState<{ url: string; releaseId: string } | null>(null);
  const [mode, setMode] = useState<"local" | "cloud" | "pages">(intent?.outcome === "publish" ? "cloud" : "local");
  const tabsId = useId(), selectedIndex = mode === "local" ? 0 : mode === "cloud" ? 1 : 2;
  const alive = useRef(true),
    working = useRef(false);
  const setup = useSetup();
  const auth = useAuth();
  const manageBacking = !auth || auth.profile.permissions.manageBacking;
  useEffect(() => { onBusyChange?.(!!busy); return () => onBusyChange?.(false); }, [busy, onBusyChange]);
  const remotePreview = (release: Release) =>
    setup?.workspace?.mode === "cloud" &&
    release.target === "local" &&
    release.status === "ready";
  const refresh = async () => {
    const value = await productApi.releases(projectId);
    if (alive.current) setReleases(value.releases);
  };
  useEffect(() => {
    alive.current = true;
    void refresh().catch((cause) => {
      if (alive.current) setError(errorText(cause));
    });
    return () => {
      alive.current = false;
    };
  }, [projectId]);
  useEffect(() => { if (!projectAccess.canExecute) setPreview(null); }, [projectAccess.canExecute]);
  useEffect(() => {
    if (preview && !releases.some(release => release.id === preview.releaseId && release.status === "ready")) setPreview(null);
  }, [preview, releases]);
  useEffect(() => {
    if (!busy) return;
    const timer = setInterval(() => {
      void refresh().catch(() => {});
    }, 3000);
    return () => clearInterval(timer);
  }, [busy, projectId]);
  const pendingCloud = releases
    .filter(
      (release) =>
        release.target !== "local" &&
        [
          "preparing",
          "queued",
          "building",
          "deploying",
          "verifying",
          "stopping",
        ].includes(release.status),
    )
    .map((release) => release.id)
    .join(",");
  useEffect(() => {
    if (!pendingCloud || busy || !canDeploy) return;
    let cancelled = false,
      checking = false;
    const timer = setInterval(() => {
      if (checking) return;
      checking = true;
      void Promise.all(
        pendingCloud
          .split(",")
          .map((id) => productApi.refreshRelease(projectId, id)),
      )
        .then(() => {
          if (!cancelled) return refresh();
        })
        .catch((cause) => {
          if (!cancelled) setError(errorText(cause));
        })
        .finally(() => {
          checking = false;
        });
    }, 5000);
    return () => {
      cancelled = true;
      clearInterval(timer);
    };
  }, [pendingCloud, busy, projectId, canDeploy]);
  async function operate(label: string, action: () => Promise<Release>, refreshAfter = true, permitted = canDeploy) {
    if (working.current || configurationBusy || !permitted) return;
    working.current = true;
    setBusy(label);
    setError("");
    try {
      const release = await action();
      if (alive.current && refreshAfter) notifyProjectContextChanged(scope, projectId);
      if (alive.current && release.status === "stopped") setPreview(null);
    } catch (cause) {
      if (alive.current) setError(errorText(cause));
    } finally {
      try {
        if (refreshAfter) await refresh();
      } catch (cause) {
        if (alive.current) setError(errorText(cause));
      }
      working.current = false;
      if (alive.current) setBusy("");
    }
  }
  const returnToRun = () => {
    if (!alive.current || !intent || !scope) return;
    useCockpit.setState({ projectRunInspection: { scope, projectId, runId: intent.runId, refresh: true } });
    useCockpit.getState().setLens("develop");
  };
  const createRelease = async (build: () => Promise<Release>, commit?: string) => {
    if (intent) {
      await binding.validate();
      if (commit && commit !== intent.commit) throw new Error("The selected deployment source differs from this build's checked commit. Refresh its results before continuing.");
    }
    return build();
  };
  const localUrl = (release: Release): string | null => {
    if (release.status !== "ready") return null;
    try {
      const url = new URL(release.url);
      const controlOrigin = new URL(window.__LUCI_API__ || DEFAULT_BASE_URL).origin;
      if (url.username || url.password || url.origin === window.location.origin || url.origin === controlOrigin) return null;
      return ((release.target !== "local" || (!url.port && isPublicGatewayHostname(url.hostname))) && url.protocol === "https:") ||
        (url.protocol === "http:" &&
          (url.hostname === "127.0.0.1" ||
            url.hostname === "localhost" ||
            isGatewayHostname(url.hostname)))
        ? url.toString()
        : null;
    } catch {
      return null;
    }
  };
  return (
    <section className="journey-page">
      <JourneyHeader
        title="Previews and publishing"
        subtitle="Choose where this project runs. A release is ready after its deployment and health checks pass."
      >
        {manageBacking && (
          <Button type="button" variant="secondary" onClick={() => setup?.openCloud()}>
            Cloud settings
          </Button>
        )}
      </JourneyHeader>
      {permissionMessage && <p role="status" className="journey-muted">{permissionMessage}</p>}
      {intent && <section className="journey-section" aria-label="Checked build deployment">
        <h2>Deploy this checked build</h2>
        <p>Build {intent.runId} · Check {intent.checkId} · Commit <code>{intent.commit}</code></p>
        <p>{intent.outcome === "preview" ? "The agreed outcome is a local preview." : "The agreed outcome is publication."} Starting a release, attaching its verified evidence and accepting the outcome are separate decisions.</p>
        {binding.loading && <p role="status">Checking this build's saved evidence and current source…</p>}
        {binding.error && <p role="alert">{binding.error}</p>}
        <div className="journey-actions"><Button variant="secondary" disabled={locked || binding.loading} onClick={() => void binding.validate().catch(() => {})}>Refresh checked source</Button>
        <Button variant="secondary" disabled={locked} onClick={returnToRun}>Return to build results</Button></div>
      </section>}
      <TabsSubtle className="deployment-destinations" aria-label="Deployment destination" idPrefix={tabsId} selectedIndex={selectedIndex} onSelect={index => { if (!locked && (!intent || (intent.outcome === "preview" ? index === 0 : index !== 0))) setMode(index === 0 ? "local" : index === 1 ? "cloud" : "pages"); }}>
        <TabsSubtleItem index={0} label={setup?.workspace?.mode === "cloud" ? "This workspace" : "On this machine"} aria-disabled={locked || intent?.outcome === "publish"} />
        <TabsSubtleItem index={1} label="Your cloud" aria-disabled={locked || intent?.outcome === "preview"} />
        <TabsSubtleItem index={2} label="GitHub Pages" aria-disabled={locked || intent?.outcome === "preview"} />
      </TabsSubtle>
      {error && (
        <p className="journey-error" role="alert">
          {error}
        </p>
      )}
      <TabsSubtlePanel idPrefix={tabsId} index={0} selectedIndex={selectedIndex}>
      {mode === "local" && (
        <form
          className="deploy-form"
          onInvalidCapture={event => { const details = (event.target as HTMLElement).closest<HTMLDetailsElement>("details"); if (details) details.open = true; }}
          onSubmit={(event) => {
            event.preventDefault();
            if (buildLocked) return;
            void operate("Building and checking your app", () => createRelease(() =>
              intent ? productApi.release(projectId, port, dockerfile, intent.commit) : productApi.release(projectId, port, dockerfile),
            ));
          }}
        >
          <details className="deployment-details"><summary>Preview build options</summary><div className="setup-field-grid">
          <label className="luci-field">
            Dockerfile
            <input
              required
              disabled={buildLocked}
              value={dockerfile}
              onChange={(event) => setDockerfile(event.target.value)}
            />
          </label>
          <label className="luci-field">
            App port
            <input
              required
              disabled={buildLocked}
              type="number"
              min="1"
              max="65535"
              value={port}
              onChange={(event) => setPort(Number(event.target.value))}
            />
            <small>The port your app listens on inside its container.</small>
          </label>
          </div></details>
          <Button type="submit" variant="primary" loading={!!busy} disabled={buildLocked}>
            {busy || "Build local preview"}
          </Button>
        </form>
      )}
      </TabsSubtlePanel>
      <TabsSubtlePanel idPrefix={tabsId} index={2} selectedIndex={selectedIndex}>
      {mode === "pages" && (
        <PagesDeployForm
          projectId={projectId}
          disabled={buildLocked}
          busy={
            busy || (configurationBusy ? "Saving runtime configuration" : "")
          }
          onDeploy={(input) =>
            void operate("Submitting static deployment", () =>
              createRelease(() => productApi.pagesRelease(projectId, input), input.commit),
            )
          }
        />
      )}
      </TabsSubtlePanel>
      <TabsSubtlePanel idPrefix={tabsId} index={1} selectedIndex={selectedIndex}>
      {mode === "cloud" && canDeploy && (
        <CloudDeployForm
          projectId={projectId}
          disabled={buildLocked}
          busy={
            busy || (configurationBusy ? "Saving runtime configuration" : "")
          }
          canManageBacking={manageBacking}
          canPrepareAccess={projectAccess.permissions.secrets}
          onSetup={() => setup?.openCloud()}
          onDeploy={(input) =>
            void operate("Submitting cloud deployment", () =>
              createRelease(() => productApi.cloudRelease(projectId, input), input.commit),
            )
          }
        />
      )}
      </TabsSubtlePanel>
      {!configurationManaged && (
        <ProjectConfiguration
          projectId={projectId}
          disabled={!!busy}
          onBusyChange={setConfigurationBusy}
        />
      )}
      <div className="journey-header deploy-history-head">
        <div>
          <h2>Releases</h2>
          <p>
            Each release keeps the brief revision and Git commit it was built
            from.
          </p>
        </div>
        <Button type="button"
          variant="secondary"
          disabled={locked}
          onClick={() =>
            void refresh().catch((cause) => setError(errorText(cause)))
          }
        >
          Refresh
        </Button>
      </div>
      {!releases.length && (
        <p className="journey-muted">
          No releases yet. Your first preview will appear here.
        </p>
      )}
      <ul className="deploy-releases">
        {releases.map((release) => (
          <li key={release.id}>
            <div>
              <strong>
                {release.target === "local"
                  ? "Local preview"
                  : release.target === "github-pages"
                    ? "GitHub Pages site"
                    : "Cloud deployment"}
              </strong>
              <span
                className={`setup-status setup-status-${release.status === "failed" ? "error" : release.status === "ready" ? "ready" : "unknown"}`}
              >
                {release.status}
              </span>
            </div>
            <p>{release.message}</p>
            <small>
              Brief {release.briefRevision} ·{" "}
              {release.commit ? release.commit.slice(0, 8) : "No commit"} ·{" "}
              {new Date(release.createdAt).toLocaleString()}
            </small>
            <div className="journey-actions">
              {intent && release.status === "ready" && release.commit === intent.commit && (intent.outcome === "preview" ? release.target === "local" : release.target !== "local") && <Button variant="primary" disabled={locked || !projectAccess.canEdit || !binding.run || binding.loading || !!binding.error} onClick={() => {
                void operate("Verifying and attaching release evidence", async () => {
                  const current = await binding.validate();
                  if (!current) throw new Error("Reopen this build's results before attaching evidence.");
                  await projectRunsApi.joinRelease(projectId, current.id, current.revision, release.id, release.target);
                  notifyProjectContextChanged(scope, projectId);
                  returnToRun();
                  return release;
                }, false, projectAccess.canEdit);
              }}>Attach release to build results</Button>}
              {release.target === "github-pages" && localUrl(release) && (
                <Button asChild variant="secondary"><a
                  href={localUrl(release)!}
                  target="_blank"
                  rel="noopener noreferrer"
                >
                  Open deployed site
                </a></Button>
              )}
              {release.target !== "github-pages" &&
                (remotePreview(release) || localUrl(release)) && (
                  <Button type="button"
                    variant="secondary"
                    disabled={locked || !projectAccess.canExecute}
                    onClick={() => {
                      void operate("Opening workspace preview", async () => {
                        const prepared = remotePreview(release) ?
                          await productApi.prepareWorkspacePreview(
                            projectId,
                            release.id,
                          ) : release;
                        const url = localUrl({ ...release, url: prepared.url });
                        if (!url)
                          throw new Error(
                            "Luci did not return a valid local preview address.",
                          );
                        const opened = await prepareBrowserAccess(projectId, url);
                        if (alive.current && browserPermission.current) setPreview({ url: opened, releaseId: release.id });
                        return release;
                      }, false, projectAccess.canExecute);
                    }}
                  >
                    Open preview
                  </Button>
                )}
              {release.target !== "local" && (
                <Button type="button"
                  variant="secondary"
                  disabled={locked || !canDeploy}
                  onClick={() =>
                    void operate("Checking deployment health", () =>
                      productApi.refreshRelease(projectId, release.id),
                    )
                  }
                >
                  Check deployment health
                </Button>
              )}
              {!["stopped", "stopping", "failed", "cancelled"].includes(
                release.status,
              ) && (
                <Button type="button"
                  variant="secondary"
                  disabled={locked || !canDeploy}
                  onClick={() =>
                    void operate(
                      release.target === "local"
                        ? "Stopping preview"
                        : release.target === "github-pages"
                          ? "Unpublishing static site"
                          : "Stopping cloud application",
                      () => productApi.stopRelease(projectId, release.id),
                    )
                  }
                >
                  {release.target === "local"
                    ? "Stop preview"
                    : release.target === "github-pages"
                      ? "Unpublish site"
                      : "Stop cloud application"}
                </Button>
              )}
              {release.target !== "local" &&
                release.target !== "github-pages" &&
                ["ready", "failed", "stopped"].includes(release.status) && (
                  <Button type="button"
                    variant="secondary"
                    disabled={locked || !canDeploy}
                    onClick={() =>
                      void operate("Rebuilding the selected release", () =>
                        productApi.rollbackRelease(projectId, release.id),
                      )
                    }
                  >
                    Roll back to this release
                  </Button>
                )}
            </div>
            {release.target !== "local" &&
              release.target !== "github-pages" && (
                <small>
                  Rollback rebuilds this selected release’s exact commit and
                  historical brief. Provider progress and health must be checked
                  before it is ready.
                </small>
              )}
          </li>
        ))}
      </ul>
      {preview && (
        <div className="deploy-preview">
          <div className="journey-header">
            <strong>Verified release preview</strong>
            <Button type="button" variant="secondary" onClick={() => setPreview(null)}>
              Close preview
            </Button>
          </div>
          <iframe
            src={preview.url}
            title="Project preview"
            sandbox="allow-scripts allow-forms allow-same-origin"
          />
        </div>
      )}
    </section>
  );
}
