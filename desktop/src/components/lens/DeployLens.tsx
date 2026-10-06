import { useEffect, useId, useRef, useState } from "react";
import { productApi, errorText, type Release } from "../../api/product";
import { isGatewayHostname, isPublicGatewayHostname } from "../../api/platform";
import { prepareBrowserAccess } from "../../api/browserAccess";
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
  const [view, setView] = useState<"platform" | "publishing">("platform");
  const [configurationBusy, setConfigurationBusy] = useState(false);
  const [publishingBusy, setPublishingBusy] = useState(false);
  const tabsId = useId(), selectedIndex = view === "platform" ? 0 : 1;
  const locked = configurationBusy || publishingBusy;
  return <div className="deployment-workspace">
    <TabsSubtle className="deployment-tabs" aria-label="Deployment workspace" idPrefix={tabsId} selectedIndex={selectedIndex} onSelect={index => { if (!locked) setView(index === 0 ? "platform" : "publishing"); }}>
      <TabsSubtleItem index={0} label="Application platform" aria-disabled={locked} />
      <TabsSubtleItem index={1} label="Previews and publishing" aria-disabled={locked} />
    </TabsSubtle>
    <TabsSubtlePanel idPrefix={tabsId} index={0} selectedIndex={selectedIndex}>
      <section className="journey-page"><JourneyHeader title="Deploy and operate" subtitle="Run the application and its backing services, inspect its health and maintain it after launch." /><fieldset className="definition-fields" disabled={configurationBusy}><PlatformWorkspace projectId={projectId} /></fieldset></section>
    </TabsSubtlePanel>
    <TabsSubtlePanel idPrefix={tabsId} index={1} selectedIndex={selectedIndex}>
      <ProjectDeploy projectId={projectId} configurationManaged configurationBusy={configurationBusy} onBusyChange={setPublishingBusy} />
    </TabsSubtlePanel>
    <section className="journey-page deployment-configuration"><details className="deployment-details"><summary>Runtime configuration</summary><ProjectConfiguration projectId={projectId} disabled={publishingBusy} onBusyChange={setConfigurationBusy} /></details></section>
  </div>;
}
export function ProjectDeploy({ projectId, configurationManaged = false, configurationBusy: externalConfigurationBusy = false, onBusyChange }: { projectId: string; configurationManaged?: boolean; configurationBusy?: boolean; onBusyChange?(busy: boolean): void }) {
  const projectAccess = useProjectAccess(projectId);
  const [releases, setReleases] = useState<Release[]>([]);
  const [port, setPort] = useState(3000);
  const [dockerfile, setDockerfile] = useState("Dockerfile");
  const [busy, setBusy] = useState("");
  const [localConfigurationBusy, setConfigurationBusy] = useState(false);
  const configurationBusy = localConfigurationBusy || externalConfigurationBusy;
  const locked = !!busy || configurationBusy;
  const [error, setError] = useState("");
  const [preview, setPreview] = useState<{ url: string; releaseId: string } | null>(null);
  const [mode, setMode] = useState<"local" | "cloud" | "pages">("local");
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
    if (!pendingCloud || busy) return;
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
  }, [pendingCloud, busy, projectId]);
  async function operate(label: string, action: () => Promise<Release>, refreshAfter = true) {
    if (working.current || configurationBusy) return;
    working.current = true;
    setBusy(label);
    setError("");
    try {
      const release = await action();
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
      <TabsSubtle className="deployment-destinations" aria-label="Deployment destination" idPrefix={tabsId} selectedIndex={selectedIndex} onSelect={index => { if (!locked) setMode(index === 0 ? "local" : index === 1 ? "cloud" : "pages"); }}>
        <TabsSubtleItem index={0} label={setup?.workspace?.mode === "cloud" ? "This workspace" : "On this machine"} aria-disabled={locked} />
        <TabsSubtleItem index={1} label="Your cloud" aria-disabled={locked} />
        <TabsSubtleItem index={2} label="GitHub Pages" aria-disabled={locked} />
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
            void operate("Building and checking your app", () =>
              productApi.release(projectId, port, dockerfile),
            );
          }}
        >
          <details className="deployment-details"><summary>Preview build options</summary><div className="setup-field-grid">
          <label className="luci-field">
            Dockerfile
            <input
              required
              disabled={locked}
              value={dockerfile}
              onChange={(event) => setDockerfile(event.target.value)}
            />
          </label>
          <label className="luci-field">
            App port
            <input
              required
              disabled={locked}
              type="number"
              min="1"
              max="65535"
              value={port}
              onChange={(event) => setPort(Number(event.target.value))}
            />
            <small>The port your app listens on inside its container.</small>
          </label>
          </div></details>
          <Button type="submit" variant="primary" loading={!!busy} disabled={locked}>
            {busy || "Build local preview"}
          </Button>
        </form>
      )}
      </TabsSubtlePanel>
      <TabsSubtlePanel idPrefix={tabsId} index={2} selectedIndex={selectedIndex}>
      {mode === "pages" && (
        <PagesDeployForm
          projectId={projectId}
          busy={
            busy || (configurationBusy ? "Saving runtime configuration" : "")
          }
          onDeploy={(input) =>
            void operate("Submitting static deployment", () =>
              productApi.pagesRelease(projectId, input),
            )
          }
        />
      )}
      </TabsSubtlePanel>
      <TabsSubtlePanel idPrefix={tabsId} index={1} selectedIndex={selectedIndex}>
      {mode === "cloud" && (
        <CloudDeployForm
          projectId={projectId}
          busy={
            busy || (configurationBusy ? "Saving runtime configuration" : "")
          }
          canManageBacking={manageBacking}
          onSetup={() => setup?.openCloud()}
          onDeploy={(input) =>
            void operate("Submitting cloud deployment", () =>
              productApi.cloudRelease(projectId, input),
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
                        if (alive.current) setPreview({ url: opened, releaseId: release.id });
                        return release;
                      }, false);
                    }}
                  >
                    Open preview
                  </Button>
                )}
              {release.target !== "local" && (
                <Button type="button"
                  variant="secondary"
                  disabled={locked}
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
                  disabled={locked}
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
                    disabled={locked}
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
