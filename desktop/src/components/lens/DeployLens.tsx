import { useEffect, useRef, useState } from "react";
import { productApi, errorText, type Release } from "../../api/product";
import { ProjectRequired, JourneyHeader } from "../journey/shared";
import { useSetup } from "../setup/SetupProvider";
import { CloudDeployForm } from "../journey/CloudDeployForm";
import { PagesDeployForm } from "../journey/PagesDeployForm";
import { ProjectConfiguration } from "../journey/ProjectConfiguration";
import { useAuth } from "../auth/AuthProvider";

export function DeployLens() {
  return (
    <ProjectRequired>
      {(id) => <ProjectDeploy key={id} projectId={id} />}
    </ProjectRequired>
  );
}
export function ProjectDeploy({ projectId }: { projectId: string }) {
  const [releases, setReleases] = useState<Release[]>([]);
  const [port, setPort] = useState(3000);
  const [dockerfile, setDockerfile] = useState("Dockerfile");
  const [busy, setBusy] = useState("");
  const [configurationBusy, setConfigurationBusy] = useState(false);
  const locked = !!busy || configurationBusy;
  const [error, setError] = useState("");
  const [preview, setPreview] = useState<string | null>(null);
  const [mode, setMode] = useState<"local" | "cloud" | "pages">("local");
  const alive = useRef(true),
    working = useRef(false);
  const setup = useSetup();
  const auth = useAuth();
  const manageBacking = !auth || auth.profile.permissions.manageBacking;
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
  async function operate(label: string, action: () => Promise<Release>) {
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
        await refresh();
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
      return (release.target !== "local" && url.protocol === "https:") ||
        (url.protocol === "http:" &&
          (url.hostname === "127.0.0.1" || url.hostname === "localhost"))
        ? url.toString()
        : null;
    } catch {
      return null;
    }
  };
  return (
    <section className="journey-page">
      <JourneyHeader
        title="Bring it to life."
        subtitle="Preview on this machine, publish a static site, or deploy to your cloud. Readiness requires a verified release and health check."
      >
        {manageBacking && (
          <button className="luci-button" onClick={() => setup?.openCloud()}>
            Cloud settings
          </button>
        )}
      </JourneyHeader>
      <div
        className="journey-actions"
        role="group"
        aria-label="Deployment destination"
      >
        <button
          className="luci-button"
          aria-pressed={mode === "local"}
          disabled={locked}
          onClick={() => setMode("local")}
        >
          {setup?.workspace?.mode === "cloud"
            ? "This workspace"
            : "On this machine"}
        </button>
        <button
          className="luci-button"
          aria-pressed={mode === "cloud"}
          disabled={locked}
          onClick={() => setMode("cloud")}
        >
          Your cloud
        </button>
        <button
          className="luci-button"
          aria-pressed={mode === "pages"}
          disabled={locked}
          onClick={() => setMode("pages")}
        >
          GitHub Pages
        </button>
      </div>
      {error && (
        <p className="journey-error" role="alert">
          {error}
        </p>
      )}
      {mode === "local" ? (
        <form
          className="deploy-form"
          onSubmit={(event) => {
            event.preventDefault();
            void operate("Building and checking your app", () =>
              productApi.release(projectId, port, dockerfile),
            );
          }}
        >
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
          <button className="luci-button luci-primary" disabled={locked}>
            {busy || "Build local preview"}
          </button>
        </form>
      ) : mode === "pages" ? (
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
      ) : (
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
      {mode !== "pages" && (
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
        <button
          className="luci-button"
          disabled={locked}
          onClick={() =>
            void refresh().catch((cause) => setError(errorText(cause)))
          }
        >
          Refresh
        </button>
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
                <a
                  className="luci-button"
                  href={localUrl(release)!}
                  target="_blank"
                  rel="noopener noreferrer"
                >
                  Open deployed site
                </a>
              )}
              {release.target !== "github-pages" &&
                (remotePreview(release) || localUrl(release)) && (
                  <button
                    className="luci-button"
                    disabled={locked}
                    onClick={() => {
                      if (!remotePreview(release)) {
                        setPreview(localUrl(release));
                        return;
                      }
                      void operate("Opening workspace preview", async () => {
                        const prepared =
                          await productApi.prepareWorkspacePreview(
                            projectId,
                            release.id,
                          );
                        const url = localUrl({ ...release, url: prepared.url });
                        if (!url)
                          throw new Error(
                            "Luci did not return a valid local preview address.",
                          );
                        if (alive.current) setPreview(url);
                        return release;
                      });
                    }}
                  >
                    Open preview
                  </button>
                )}
              {release.target !== "local" && (
                <button
                  className="luci-button"
                  disabled={locked}
                  onClick={() =>
                    void operate("Checking deployment health", () =>
                      productApi.refreshRelease(projectId, release.id),
                    )
                  }
                >
                  Check deployment health
                </button>
              )}
              {!["stopped", "stopping", "failed", "cancelled"].includes(
                release.status,
              ) && (
                <button
                  className="luci-button"
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
                </button>
              )}
              {release.target !== "local" &&
                release.target !== "github-pages" &&
                ["ready", "failed", "stopped"].includes(release.status) && (
                  <button
                    className="luci-button"
                    disabled={locked}
                    onClick={() =>
                      void operate("Rebuilding the selected release", () =>
                        productApi.rollbackRelease(projectId, release.id),
                      )
                    }
                  >
                    Roll back to this release
                  </button>
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
            <button className="luci-button" onClick={() => setPreview(null)}>
              Close preview
            </button>
          </div>
          <iframe
            src={preview}
            title="Project preview"
            sandbox={`allow-scripts allow-forms${new URL(preview).origin !== window.location.origin ? " allow-same-origin" : ""}`}
          />
        </div>
      )}
    </section>
  );
}
